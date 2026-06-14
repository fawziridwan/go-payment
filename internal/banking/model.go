package banking

import (
	"time"

	"github.com/shopspring/decimal"
)

// ============================================
// Domain Models
// ============================================

// Account represents a bank account
type Account struct {
	ID            string     `json:"id"`
	AccountNumber string     `json:"accountNumber"`
	AccountName   string     `json:"accountName"`
	BankCode      string     `json:"bankCode"`
	Currency      string     `json:"currency"`
	Status        string     `json:"status"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
	DeletedAt     *time.Time `json:"deletedAt,omitempty"`
}

// Balance represents account balance
type Balance struct {
	ID               string          `json:"id"`
	AccountID        string          `json:"accountId"`
	AvailableBalance decimal.Decimal `json:"availableBalance"`
	LedgerBalance    decimal.Decimal `json:"ledgerBalance"`
	UpdatedAt        time.Time       `json:"updatedAt"`
}

// BankingTransaction represents a transfer transaction
type BankingTransaction struct {
	ID                 string          `json:"id"`
	ReferenceNumber    string          `json:"referenceNumber"`
	SourceAccountID    string          `json:"sourceAccountId"`
	DestinationAccount string          `json:"destinationAccount"`
	DestinationBank    string          `json:"destinationBank"`
	Amount             decimal.Decimal `json:"amount"`
	Fee                decimal.Decimal `json:"fee"`
	Currency           string          `json:"currency"`
	Status             string          `json:"status"`
	Remark             string          `json:"remark"`
	CreatedAt          time.Time       `json:"createdAt"`
}

// Mutation represents an account mutation record
type Mutation struct {
	ID            string          `json:"id"`
	TransactionID string          `json:"transactionId"`
	AccountID     string          `json:"accountId"`
	Type          string          `json:"type"`
	Amount        decimal.Decimal `json:"amount"`
	BalanceAfter  decimal.Decimal `json:"balanceAfter"`
	Description   string          `json:"description"`
	CreatedAt     time.Time       `json:"createdAt"`
}

// IdempotencyKey represents a stored idempotency key
type IdempotencyKey struct {
	ID             string    `json:"id"`
	IdempotencyKey string    `json:"idempotencyKey"`
	RequestHash    string    `json:"requestHash"`
	Response       []byte    `json:"response"`
	CreatedAt      time.Time `json:"createdAt"`
}

// AuditLog represents an audit log entry
type AuditLog struct {
	ID              string    `json:"id"`
	RequestID       string    `json:"requestId"`
	UserID          string    `json:"userId"`
	Action          string    `json:"action"`
	Method          string    `json:"method"`
	Path            string    `json:"path"`
	RequestPayload  []byte    `json:"requestPayload,omitempty"`
	ResponsePayload []byte    `json:"responsePayload,omitempty"`
	ResponseCode    int       `json:"responseCode"`
	IPAddress       string    `json:"ipAddress"`
	UserAgent       string    `json:"userAgent"`
	Timestamp       time.Time `json:"timestamp"`
}

// ============================================
// Request DTOs
// ============================================

// AddAccountRequest represents the request body for adding an account
type AddAccountRequest struct {
	AccountNumber string `json:"accountNumber"`
	AccountName   string `json:"accountName"`
	BankCode      string `json:"bankCode"`
	Currency      string `json:"currency"`
}

// TransferRequest represents the request body for a transfer
type TransferRequest struct {
	SourceAccountID        string          `json:"sourceAccountId"`
	DestinationAccountNumber string        `json:"destinationAccountNumber"`
	DestinationBankCode    string          `json:"destinationBankCode"`
	Amount                 decimal.Decimal `json:"amount"`
	Currency               string          `json:"currency"`
	Remark                 string          `json:"remark"`
}

// ============================================
// Response DTOs
// ============================================

// StandardResponse is the standard API response envelope
type StandardResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse is the standard error response
type ErrorResponse struct {
	Success   bool   `json:"success"`
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

// AddAccountResponse is returned after adding an account
type AddAccountResponse struct {
	AccountID string `json:"accountId"`
}

// RemoveAccountResponse is returned after removing an account
type RemoveAccountMessageResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// BalanceResponse is returned for balance inquiry
type BalanceResponse struct {
	AccountID        string          `json:"accountId"`
	AvailableBalance decimal.Decimal `json:"availableBalance"`
	LedgerBalance    decimal.Decimal `json:"ledgerBalance"`
	Currency         string          `json:"currency"`
	LastUpdated      time.Time       `json:"lastUpdated"`
}

// TransferResponse is returned after a successful transfer
type TransferResponseData struct {
	TransactionID   string          `json:"transactionId"`
	ReferenceNumber string          `json:"referenceNumber"`
	Status          string          `json:"status"`
	Amount          decimal.Decimal `json:"amount"`
	Fee             decimal.Decimal `json:"fee"`
	Timestamp       time.Time       `json:"timestamp"`
}

// MutationRecord is a single mutation in the mutation list
type MutationRecord struct {
	TransactionID string          `json:"transactionId"`
	Type          string          `json:"type"`
	Amount        decimal.Decimal `json:"amount"`
	BalanceAfter  decimal.Decimal `json:"balanceAfter"`
	Description   string          `json:"description"`
	CreatedAt     time.Time       `json:"createdAt"`
}

// MutationResponse is the paginated mutation response
type MutationResponse struct {
	Page    int              `json:"page"`
	Size    int              `json:"size"`
	Total   int              `json:"total"`
	Records []MutationRecord `json:"records"`
}

// HealthResponse is the health check response
type HealthResponse struct {
	Status string `json:"status"`
}

// ============================================
// Constants
// ============================================

// Account statuses
const (
	AccountStatusActive  = "ACTIVE"
	AccountStatusDeleted = "DELETED"
)

// Transaction statuses
const (
	TransactionStatusPending    = "PENDING"
	TransactionStatusProcessing = "PROCESSING"
	TransactionStatusSuccess    = "SUCCESS"
	TransactionStatusFailed     = "FAILED"
	TransactionStatusReversed   = "REVERSED"
)

// Mutation types
const (
	MutationTypeCredit      = "CREDIT"
	MutationTypeDebit       = "DEBIT"
	MutationTypeTransferIn  = "TRANSFER_IN"
	MutationTypeTransferOut = "TRANSFER_OUT"
)

// Default fee percentage (2.5%)
var DefaultTransferFee = decimal.NewFromFloat(2500)
