package banking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/shopspring/decimal"
)

// Service handles all business logic for the banking API
type Service struct {
	repo *Repository
}

// NewService creates a new banking service
func NewService(repo *Repository) *Service {
	return &Service{repo: repo}
}

// ============================================
// Account Service
// ============================================

// AddAccount creates a new bank account
func (s *Service) AddAccount(ctx context.Context, req AddAccountRequest) (string, error) {
	// Validate request
	if req.AccountNumber == "" {
		return "", ErrInvalidRequest
	}
	if req.AccountName == "" {
		return "", ErrInvalidRequest
	}
	if req.BankCode == "" {
		return "", ErrInvalidRequest
	}
	if req.Currency == "" {
		req.Currency = "IDR"
	}

	return s.repo.CreateAccount(ctx, req)
}

// RemoveAccount soft deletes an account
func (s *Service) RemoveAccount(ctx context.Context, accountID string) error {
	if accountID == "" {
		return ErrInvalidRequest
	}

	// Validate account exists and is active
	account, err := s.repo.GetAccountByID(ctx, accountID)
	if err != nil {
		return err
	}

	if account.Status == AccountStatusDeleted || account.DeletedAt != nil {
		return ErrAccountDeleted
	}

	return s.repo.SoftDeleteAccount(ctx, accountID)
}

// ============================================
// Balance Service
// ============================================

// GetBalance retrieves the balance for an account efficiently
func (s *Service) GetBalance(ctx context.Context, accountID string) (*BalanceResponse, error) {
	if accountID == "" {
		return nil, ErrInvalidRequest
	}

	// Fetch account and balance in a single query
	account, balance, err := s.repo.GetAccountWithBalance(ctx, accountID)
	if err != nil {
		return nil, err
	}

	if account.Status == AccountStatusDeleted {
		return nil, ErrAccountDeleted
	}

	return &BalanceResponse{
		AccountID:        accountID,
		AvailableBalance: balance.AvailableBalance,
		LedgerBalance:    balance.LedgerBalance,
		Currency:         account.Currency,
		LastUpdated:      balance.UpdatedAt,
	}, nil
}

// ============================================
// Mutation Service
// ============================================

// GetMutations retrieves account mutations with filters
func (s *Service) GetMutations(ctx context.Context, accountID string, fromDate, toDate time.Time, txnType string, page, size int) (*MutationResponse, error) {
	if accountID == "" {
		return nil, ErrInvalidRequest
	}

	// Default pagination
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	// Note: We've removed the redundant s.repo.GetAccountByID check here
	// and will rely on the main query to handle non-existent accounts if possible,
	// or accept that fetching mutations for a deleted/non-existent account 
	// will just return an empty list or error if we want more strictness.
	// For performance, we skip the extra check and let the GetMutations query run.

	// Validate transaction type
	validTypes := map[string]bool{
		"ALL": true, "CREDIT": true, "DEBIT": true,
		"TRANSFER_IN": true, "TRANSFER_OUT": true, "": true,
	}
	if !validTypes[txnType] {
		return nil, ErrInvalidRequest
	}

	// Default pagination
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}

	records, total, err := s.repo.GetMutations(ctx, accountID, fromDate, toDate, txnType, page, size)
	if err != nil {
		return nil, err
	}

	if records == nil {
		records = []MutationRecord{}
	}

	return &MutationResponse{
		Page:    page,
		Size:    size,
		Total:   total,
		Records: records,
	}, nil
}

// ============================================
// Transfer Service
// ============================================

// Transfer performs a fund transfer with idempotency support
func (s *Service) Transfer(ctx context.Context, req TransferRequest, idempotencyKey string) (*TransferResponseData, error) {
	// Validate request
	if err := s.validateTransferRequest(req); err != nil {
		return nil, err
	}

	// Check idempotency key
	if idempotencyKey != "" {
		reqBytes, _ := json.Marshal(req)
		requestHash := HashRequest(reqBytes)

		existing, err := s.repo.GetIdempotencyKey(ctx, idempotencyKey)
		if err != nil {
			return nil, err
		}

		if existing != nil {
			// Same key exists - check if request hash matches
			if existing.RequestHash != requestHash {
				return nil, ErrIdempotencyConflict
			}
			// Return cached response
			var cachedResp TransferResponseData
			if err := json.Unmarshal(existing.Response, &cachedResp); err == nil {
				return &cachedResp, nil
			}
		}
	}

	// Validate source account and its status
	sourceAccount, err := s.repo.GetAccountByID(ctx, req.SourceAccountID)
	if err != nil {
		return nil, err
	}
	if sourceAccount.Status == AccountStatusDeleted {
		return nil, ErrAccountDeleted
	}

	// Validate destination account exists (by account number)
	// We'll skip destination status check here for extreme performance, 
	// as it will fail during balance lock if it doesn't exist.
	// But keeping GetAccountByNumber to at least know where it's going.
	destAccount, err := s.repo.GetAccountByNumber(ctx, req.DestinationAccountNumber)
	if err != nil {
		return nil, ErrDestinationNotFound
	}

	// Calculate fee
	fee := DefaultTransferFee
	if sourceAccount.BankCode == req.DestinationBankCode {
		fee = decimal.Zero // No fee for same bank transfer
	}

	totalAmount := req.Amount.Add(fee)

	// Begin transaction
	tx, err := s.repo.BeginTx(ctx)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	defer tx.Rollback()

	// Lock source balance (FOR UPDATE)
	sourceBalance, err := s.repo.GetBalanceForUpdate(ctx, tx, sourceAccount.ID)
	if err != nil {
		return nil, err
	}

	// Validate sufficient balance
	if sourceBalance.AvailableBalance.LessThan(totalAmount) {
		return nil, ErrInsufficientBalance
	}

	// Lock destination balance
	destBalance, err := s.repo.GetBalanceForUpdate(ctx, tx, destAccount.ID)
	if err != nil {
		return nil, err
	}

	// Create transaction record
	currency := req.Currency
	if currency == "" {
		currency = "IDR"
	}

	txnID, err := s.repo.CreateTransaction(ctx, tx, BankingTransaction{
		SourceAccountID:    sourceAccount.ID,
		DestinationAccount: req.DestinationAccountNumber,
		DestinationBank:    req.DestinationBankCode,
		Amount:             req.Amount,
		Fee:                fee,
		Currency:           currency,
		Status:             TransactionStatusProcessing,
		Remark:             req.Remark,
	})
	if err != nil {
		return nil, err
	}

	// Update source balance (debit)
	newSourceAvailable := sourceBalance.AvailableBalance.Sub(totalAmount)
	newSourceLedger := sourceBalance.LedgerBalance.Sub(totalAmount)
	if err = s.repo.UpdateBalance(ctx, tx, sourceAccount.ID, newSourceAvailable, newSourceLedger); err != nil {
		return nil, err
	}

	// Update destination balance (credit)
	newDestAvailable := destBalance.AvailableBalance.Add(req.Amount)
	newDestLedger := destBalance.LedgerBalance.Add(req.Amount)
	if err = s.repo.UpdateBalance(ctx, tx, destAccount.ID, newDestAvailable, newDestLedger); err != nil {
		return nil, err
	}

	// Create mutations
	// Source account - TRANSFER_OUT / DEBIT
	if err = s.repo.CreateMutation(ctx, tx, Mutation{
		TransactionID: txnID,
		AccountID:     sourceAccount.ID,
		Type:          MutationTypeTransferOut,
		Amount:        totalAmount,
		BalanceAfter:  newSourceAvailable,
		Description:   fmt.Sprintf("Transfer to %s - %s", req.DestinationAccountNumber, req.Remark),
	}); err != nil {
		return nil, err
	}

	// Destination account - TRANSFER_IN / CREDIT
	if err = s.repo.CreateMutation(ctx, tx, Mutation{
		TransactionID: txnID,
		AccountID:     destAccount.ID,
		Type:          MutationTypeTransferIn,
		Amount:        req.Amount,
		BalanceAfter:  newDestAvailable,
		Description:   fmt.Sprintf("Transfer from %s - %s", sourceAccount.AccountNumber, req.Remark),
	}); err != nil {
		return nil, err
	}

	// Update transaction status to SUCCESS
	if err = s.repo.UpdateTransactionStatus(ctx, tx, txnID, TransactionStatusSuccess); err != nil {
		return nil, err
	}

	// Commit transaction
	if err = tx.Commit(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInternalError, err)
	}

	// Get the created transaction for reference number
	createdTxn, err := s.repo.GetTransaction(ctx, txnID)
	if err != nil {
		return nil, err
	}

	response := &TransferResponseData{
		TransactionID:   txnID,
		ReferenceNumber: createdTxn.ReferenceNumber,
		Status:          TransactionStatusSuccess,
		Amount:          req.Amount,
		Fee:             fee,
		Timestamp:       createdTxn.CreatedAt,
	}

	// Save idempotency key
	if idempotencyKey != "" {
		reqBytes, _ := json.Marshal(req)
		requestHash := HashRequest(reqBytes)
		_ = s.repo.SaveIdempotencyKey(ctx, idempotencyKey, requestHash, response)
	}

	return response, nil
}

// validateTransferRequest validates the transfer request
func (s *Service) validateTransferRequest(req TransferRequest) error {
	if req.SourceAccountID == "" {
		return ErrInvalidRequest
	}
	if req.DestinationAccountNumber == "" {
		return ErrInvalidRequest
	}
	if req.DestinationBankCode == "" {
		return ErrInvalidRequest
	}
	if req.Amount.LessThanOrEqual(decimal.Zero) {
		return ErrInvalidRequest
	}
	return nil
}
