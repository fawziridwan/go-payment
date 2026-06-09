package transfer

import "time"

type Account struct {
	ID            int64  `json:"id"`
	AccountNumber string `json:"account_number"`
	Balance       int64  `json:"balance"`
}

type Transaction struct {
	ID              int64     `json:"id"`
	UUID            string    `json:"uuid"`
	FromAccountID   int64     `json:"from_account_id"`
	ToAccountID     int64     `json:"to_account_id"`
	Amount          int64     `json:"amount"`
	TransactionType string    `json:"transaction_type"`
	Notes           string    `json:"notes"`
	CreatedAt       time.Time `json:"created_at"`
}

type TransferRequest struct {
	FromAccountNumber string `json:"from_account"`
	ToAccountNumber   string `json:"to_account"`
	Amount            int64  `json:"amount"`
}

type TransferResponse struct {
	TransactionID string `json:"transaction_id"`
	Message       string `json:"message"`
	UserID        int    `json:"user_id,omitempty"`
	UserEmail     string `json:"user_email,omitempty"`
}

type CheckBalanceRequest struct {
	TransactionID string `json:"transaction_id"`
}

type CheckBalanceResponse struct {
	Balance       int64  `json:"balance"`
	AccountID     int64  `json:"account_id"`
	TransactionID string `json:"transaction_id"`
	Amount        int64  `json:"amount"`
	Message       string `json:"message"`
	UserID        int    `json:"user_id,omitempty"`
	UserEmail     string `json:"user_email,omitempty"`
}

type TransactionHistoryRequest struct {
	UserID        int64  `json:"user_id"`
	SourceAccount string `json:"source_account"`
	TransactionID string `json:"transaction_id"`
}

type TransactionHistoryItem struct {
	RetrievalRefNumber string    `json:"retrieval_reference_number"`
	TransactionName    string    `json:"transaction_name"`
	BalanceChange      string    `json:"balance_change"`
	Notes              string    `json:"notes,omitempty"`
	TransactionDate    time.Time `json:"transaction_date"`
}

type TransactionHistoryResponse struct {
	Transactions []TransactionHistoryItem `json:"transactions"`
	Message      string                   `json:"message"`
	UserID       int                      `json:"user_id,omitempty"`
	UserEmail    string                   `json:"user_email,omitempty"`
}
