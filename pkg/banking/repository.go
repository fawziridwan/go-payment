package banking

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// Repository handles all database operations for the banking API
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new banking repository
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// ============================================
// Account Operations
// ============================================

// CreateAccount inserts a new account and its initial balance
func (r *Repository) CreateAccount(ctx context.Context, req AddAccountRequest) (string, error) {
	// Check if account number already exists
	var exists bool
	err := r.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM banking_accounts WHERE account_number = $1 AND deleted_at IS NULL)`,
		req.AccountNumber,
	).Scan(&exists)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	if exists {
		return "", ErrAccountAlreadyExists
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	defer tx.Rollback()

	accountID := uuid.New().String()
	now := time.Now()

	_, err = tx.ExecContext(ctx, `
		INSERT INTO banking_accounts (id, account_number, account_name, bank_code, currency, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, accountID, req.AccountNumber, req.AccountName, req.BankCode, req.Currency, AccountStatusActive, now, now)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInternalError, err)
	}

	// Create initial balance record
	balanceID := uuid.New().String()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO balances (id, account_id, available_balance, ledger_balance, updated_at)
		VALUES ($1, $2, 0, 0, $3)
	`, balanceID, accountID, now)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInternalError, err)
	}

	if err = tx.Commit(); err != nil {
		return "", fmt.Errorf("%w: %v", ErrInternalError, err)
	}

	return accountID, nil
}

// GetAccountByID retrieves an account by ID (excluding soft-deleted)
func (r *Repository) GetAccountByID(ctx context.Context, accountID string) (*Account, error) {
	var a Account
	err := r.db.QueryRowContext(ctx, `
		SELECT id, account_number, account_name, bank_code, currency, status, created_at, updated_at, deleted_at
		FROM banking_accounts
		WHERE id = $1
	`, accountID).Scan(
		&a.ID, &a.AccountNumber, &a.AccountName, &a.BankCode,
		&a.Currency, &a.Status, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	return &a, nil
}

// GetAccountByNumber retrieves an account by account number
func (r *Repository) GetAccountByNumber(ctx context.Context, accountNumber string) (*Account, error) {
	var a Account
	err := r.db.QueryRowContext(ctx, `
		SELECT id, account_number, account_name, bank_code, currency, status, created_at, updated_at, deleted_at
		FROM banking_accounts
		WHERE account_number = $1 AND deleted_at IS NULL
	`, accountNumber).Scan(
		&a.ID, &a.AccountNumber, &a.AccountName, &a.BankCode,
		&a.Currency, &a.Status, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	return &a, nil
}

// SoftDeleteAccount performs a soft delete on an account
func (r *Repository) SoftDeleteAccount(ctx context.Context, accountID string) error {
	now := time.Now()
	result, err := r.db.ExecContext(ctx, `
		UPDATE banking_accounts
		SET status = $1, deleted_at = $2, updated_at = $2
		WHERE id = $3 AND deleted_at IS NULL
	`, AccountStatusDeleted, now, accountID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInternalError, err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	if rows == 0 {
		return ErrAccountNotFound
	}
	return nil
}

// ============================================
// Balance Operations
// ============================================

// GetBalance retrieves the balance for an account
func (r *Repository) GetBalance(ctx context.Context, accountID string) (*Balance, error) {
	var b Balance
	err := r.db.QueryRowContext(ctx, `
		SELECT id, account_id, available_balance, ledger_balance, updated_at
		FROM balances
		WHERE account_id = $1
	`, accountID).Scan(&b.ID, &b.AccountID, &b.AvailableBalance, &b.LedgerBalance, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	return &b, nil
}

// GetAccountWithBalance retrieves both account and balance in a single query
func (r *Repository) GetAccountWithBalance(ctx context.Context, accountID string) (*Account, *Balance, error) {
	var a Account
	var b Balance
	err := r.db.QueryRowContext(ctx, `
		SELECT a.id, a.account_number, a.account_name, a.bank_code, a.currency, a.status, a.created_at, a.updated_at, a.deleted_at,
		       b.id, b.available_balance, b.ledger_balance, b.updated_at
		FROM banking_accounts a
		JOIN balances b ON a.id = b.account_id
		WHERE a.id = $1 AND a.deleted_at IS NULL
	`, accountID).Scan(
		&a.ID, &a.AccountNumber, &a.AccountName, &a.BankCode, &a.Currency, &a.Status, &a.CreatedAt, &a.UpdatedAt, &a.DeletedAt,
		&b.ID, &b.AvailableBalance, &b.LedgerBalance, &b.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil, ErrAccountNotFound
		}
		return nil, nil, fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	b.AccountID = a.ID
	return &a, &b, nil
}

// GetBalanceForUpdate locks the balance row for update within a transaction
func (r *Repository) GetBalanceForUpdate(ctx context.Context, tx *sql.Tx, accountID string) (*Balance, error) {
	var b Balance
	err := tx.QueryRowContext(ctx, `
		SELECT id, account_id, available_balance, ledger_balance, updated_at
		FROM balances
		WHERE account_id = $1
		FOR UPDATE
	`, accountID).Scan(&b.ID, &b.AccountID, &b.AvailableBalance, &b.LedgerBalance, &b.UpdatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	return &b, nil
}

// UpdateBalance updates balance amounts within a transaction
func (r *Repository) UpdateBalance(ctx context.Context, tx *sql.Tx, accountID string, available, ledger decimal.Decimal) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE balances
		SET available_balance = $1, ledger_balance = $2, updated_at = $3
		WHERE account_id = $4
	`, available, ledger, time.Now(), accountID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	return nil
}

// ============================================
// Transaction Operations
// ============================================

// CreateTransaction inserts a new transaction record
func (r *Repository) CreateTransaction(ctx context.Context, tx *sql.Tx, txn BankingTransaction) (string, error) {
	txnID := uuid.New().String()
	refNum := "REF-" + uuid.New().String()[:8]

	_, err := tx.ExecContext(ctx, `
		INSERT INTO banking_transactions (id, reference_number, source_account_id, destination_account, destination_bank, amount, fee, currency, status, remark, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, txnID, refNum, txn.SourceAccountID, txn.DestinationAccount, txn.DestinationBank,
		txn.Amount, txn.Fee, txn.Currency, txn.Status, txn.Remark, time.Now())
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInternalError, err)
	}

	return txnID, nil
}

// GetTransaction retrieves a transaction by ID
func (r *Repository) GetTransaction(ctx context.Context, txnID string) (*BankingTransaction, error) {
	var t BankingTransaction
	err := r.db.QueryRowContext(ctx, `
		SELECT id, reference_number, source_account_id, destination_account, destination_bank,
		       amount, fee, currency, status, remark, created_at
		FROM banking_transactions
		WHERE id = $1
	`, txnID).Scan(
		&t.ID, &t.ReferenceNumber, &t.SourceAccountID, &t.DestinationAccount,
		&t.DestinationBank, &t.Amount, &t.Fee, &t.Currency, &t.Status, &t.Remark, &t.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	return &t, nil
}

// UpdateTransactionStatus updates the status of a transaction
func (r *Repository) UpdateTransactionStatus(ctx context.Context, tx *sql.Tx, txnID, status string) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE banking_transactions SET status = $1 WHERE id = $2
	`, status, txnID)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	return nil
}

// ============================================
// Mutation Operations
// ============================================

// CreateMutation inserts a mutation record
func (r *Repository) CreateMutation(ctx context.Context, tx *sql.Tx, m Mutation) error {
	mutationID := uuid.New().String()
	_, err := tx.ExecContext(ctx, `
		INSERT INTO mutations (id, transaction_id, account_id, type, amount, balance_after, description, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, mutationID, m.TransactionID, m.AccountID, m.Type, m.Amount, m.BalanceAfter, m.Description, time.Now())
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	return nil
}

// GetMutations retrieves mutations for an account with filters and pagination using a single query
func (r *Repository) GetMutations(ctx context.Context, accountID string, fromDate, toDate time.Time, txnType string, page, size int) ([]MutationRecord, int, error) {
	offset := (page - 1) * size
	
	// Base query with window function for total count
	query := `
		SELECT m.transaction_id, m.type, m.amount, m.balance_after, m.description, m.created_at,
		       COUNT(*) OVER() as total_count
		FROM mutations m
		WHERE m.account_id = $1 AND m.created_at >= $2 AND m.created_at <= $3
	`
	args := []interface{}{accountID, fromDate, toDate}
	argIdx := 4

	if txnType != "" && txnType != "ALL" {
		query += fmt.Sprintf(" AND m.type = $%d", argIdx)
		args = append(args, txnType)
		argIdx++
	}

	query += fmt.Sprintf(" ORDER BY m.created_at DESC LIMIT $%d OFFSET $%d", argIdx, argIdx+1)
	args = append(args, size, offset)

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	defer rows.Close()

	var records []MutationRecord
	total := 0
	for rows.Next() {
		var rec MutationRecord
		if err := rows.Scan(&rec.TransactionID, &rec.Type, &rec.Amount, &rec.BalanceAfter, &rec.Description, &rec.CreatedAt, &total); err != nil {
			return nil, 0, fmt.Errorf("%w: %v", ErrInternalError, err)
		}
		records = append(records, rec)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("%w: %v", ErrInternalError, err)
	}

	return records, total, nil
}

// ============================================
// Idempotency Operations
// ============================================

// GetIdempotencyKey checks if an idempotency key exists and returns cached response
func (r *Repository) GetIdempotencyKey(ctx context.Context, key string) (*IdempotencyKey, error) {
	var ik IdempotencyKey
	err := r.db.QueryRowContext(ctx, `
		SELECT id, idempotency_key, request_hash, response, created_at
		FROM idempotency_keys
		WHERE idempotency_key = $1 AND created_at > $2
	`, key, time.Now().Add(-24*time.Hour)).Scan(
		&ik.ID, &ik.IdempotencyKey, &ik.RequestHash, &ik.Response, &ik.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // Not found, not an error
		}
		return nil, fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	return &ik, nil
}

// SaveIdempotencyKey stores an idempotency key with its response
func (r *Repository) SaveIdempotencyKey(ctx context.Context, key, requestHash string, response interface{}) error {
	respJSON, err := json.Marshal(response)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInternalError, err)
	}

	id := uuid.New().String()
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO idempotency_keys (id, idempotency_key, request_hash, response, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, id, key, requestHash, respJSON, time.Now())
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInternalError, err)
	}
	return nil
}

// HashRequest creates a hash of the request for idempotency checking
func HashRequest(body []byte) string {
	hash := sha256.Sum256(body)
	return hex.EncodeToString(hash[:])
}

// ============================================
// Audit Operations
// ============================================

// SaveAuditLog stores an audit log entry
func (r *Repository) SaveAuditLog(ctx context.Context, log AuditLog) error {
	id := uuid.New().String()

	// Ensure empty payloads are handled as NULL for JSONB columns
	var reqPayload interface{} = log.RequestPayload
	if len(log.RequestPayload) == 0 {
		reqPayload = nil
	}

	var resPayload interface{} = log.ResponsePayload
	if len(log.ResponsePayload) == 0 {
		resPayload = nil
	}

	_, err := r.db.ExecContext(ctx, `
		INSERT INTO audit_logs (id, request_id, user_id, action, method, path, request_payload, response_payload, response_code, ip_address, user_agent, timestamp)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`, id, log.RequestID, log.UserID, log.Action, log.Method, log.Path,
		reqPayload, resPayload, log.ResponseCode,
		log.IPAddress, log.UserAgent, time.Now())
	if err != nil {
		// Don't fail the request because of audit log failure
		fmt.Printf("WARNING: Failed to save audit log: %v (RequestID: %s)\n", err, log.RequestID)
	}
	return nil
}


// BeginTx starts a new database transaction
func (r *Repository) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return r.db.BeginTx(ctx, nil)
}
