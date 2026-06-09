package transfer

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrAccountNotFound      = errors.New("account not found")
	ErrTransactionNotFound  = errors.New("transaction not found")
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrInvalidAccountNumber = errors.New("invalid account number")
)

type SQLRepository struct {
	db *sql.DB
}

func NewSQLRepository(db *sql.DB) *SQLRepository {
	return &SQLRepository{db: db}
}

func (r *SQLRepository) Transfer(ctx context.Context, fromAccountNumber, toAccountNumber string, amount int64) (string, error) {
	if err := validateAccountNumber(fromAccountNumber); err != nil {
		return "", err
	}
	if err := validateAccountNumber(toAccountNumber); err != nil {
		return "", err
	}

	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	fromAccount, err := r.getAccountByNumberForUpdate(ctx, tx, fromAccountNumber)
	if err != nil {
		return "", err
	}

	toAccount, err := r.getAccountByNumberForUpdate(ctx, tx, toAccountNumber)
	if err != nil {
		return "", err
	}

	if fromAccount.Balance < amount {
		return "", ErrInsufficientFunds
	}

	if err = r.updateBalance(ctx, tx, fromAccount.ID, fromAccount.Balance-amount); err != nil {
		return "", err
	}
	if err = r.updateBalance(ctx, tx, toAccount.ID, toAccount.Balance+amount); err != nil {
		return "", err
	}

	transactionUUID, err := r.insertTransaction(ctx, tx, Transaction{
		FromAccountID:   fromAccount.ID,
		ToAccountID:     toAccount.ID,
		Amount:          amount,
		TransactionType: "transfer",
	})
	if err != nil {
		return "", err
	}

	if err = tx.Commit(); err != nil {
		return "", err
	}

	return transactionUUID, nil
}

func (r *SQLRepository) GetTransactionByUUID(ctx context.Context, txnUUID string) (*Transaction, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, uuid, from_account_id, to_account_id, amount, transaction_type, notes, created_at
		FROM transactions
		WHERE uuid = $1
	`, txnUUID)

	var t Transaction
	if err := row.Scan(&t.ID, &t.UUID, &t.FromAccountID, &t.ToAccountID, &t.Amount, &t.TransactionType, &t.Notes, &t.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrTransactionNotFound
		}
		return nil, err
	}

	return &t, nil
}

func (r *SQLRepository) GetBalance(ctx context.Context, accountNumber string) (int64, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT balance
		FROM accounts
		WHERE account_number = $1
	`, accountNumber)

	var balance int64
	if err := row.Scan(&balance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrAccountNotFound
		}
		return 0, err
	}

	return balance, nil
}

func (r *SQLRepository) GetBalanceByID(ctx context.Context, accountID int64) (int64, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT balance
		FROM accounts
		WHERE id = $1
	`, accountID)

	var balance int64
	if err := row.Scan(&balance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, ErrAccountNotFound
		}
		return 0, err
	}

	return balance, nil
}

func (r *SQLRepository) GetTransactionHistory(ctx context.Context, accountNumber string, days int) ([]Transaction, error) {
	if err := validateAccountNumber(accountNumber); err != nil {
		return nil, err
	}

	account, err := r.getAccountByNumber(ctx, accountNumber)
	if err != nil {
		return nil, err
	}

	var timeFilter time.Duration
	switch days {
	case 7:
		timeFilter = 7 * 24 * time.Hour
	case 30:
		timeFilter = 30 * 24 * time.Hour
	case 90:
		timeFilter = 90 * 24 * time.Hour
	default:
		timeFilter = 30 * 24 * time.Hour
	}

	cutoffTime := time.Now().Add(-timeFilter)

	rows, err := r.db.QueryContext(ctx, `
		SELECT id, uuid, from_account_id, to_account_id, amount, transaction_type, notes, created_at
		FROM transactions
		WHERE (from_account_id = $1 OR to_account_id = $1)
		AND created_at >= $2
		ORDER BY created_at DESC
	`, account.ID, cutoffTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []Transaction
	for rows.Next() {
		var t Transaction
		if err := rows.Scan(&t.ID, &t.UUID, &t.FromAccountID, &t.ToAccountID, &t.Amount, &t.TransactionType, &t.Notes, &t.CreatedAt); err != nil {
			return nil, err
		}
		transactions = append(transactions, t)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return transactions, nil
}

func (r *SQLRepository) getAccountByNumber(ctx context.Context, accountNumber string) (*Account, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, account_number, balance
		FROM accounts
		WHERE account_number = $1
	`, accountNumber)

	var account Account
	if err := row.Scan(&account.ID, &account.AccountNumber, &account.Balance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}

	return &account, nil
}

func (r *SQLRepository) getAccountByNumberForUpdate(ctx context.Context, tx *sql.Tx, accountNumber string) (*Account, error) {
	row := tx.QueryRowContext(ctx, `
		SELECT id, account_number, balance
		FROM accounts
		WHERE account_number = $1
		FOR UPDATE
	`, accountNumber)

	var account Account
	if err := row.Scan(&account.ID, &account.AccountNumber, &account.Balance); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAccountNotFound
		}
		return nil, err
	}

	return &account, nil
}

func (r *SQLRepository) updateBalance(ctx context.Context, tx *sql.Tx, accountID int64, newBalance int64) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE accounts
		SET balance = $1
		WHERE id = $2
	`, newBalance, accountID)
	return err
}

func (r *SQLRepository) insertTransaction(ctx context.Context, tx *sql.Tx, t Transaction) (string, error) {
	txnUUID := uuid.New().String()

	row := tx.QueryRowContext(ctx, `
		INSERT INTO transactions (uuid, from_account_id, to_account_id, amount, transaction_type, notes)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id
	`, txnUUID, t.FromAccountID, t.ToAccountID, t.Amount, t.TransactionType, t.Notes)

	var id int64
	if err := row.Scan(&id); err != nil {
		return "", err
	}

	return txnUUID, nil
}

func validateAccountNumber(accountNumber string) error {
	if accountNumber == "" {
		return errors.New("account number is required")
	}

	if !(strings.HasPrefix(accountNumber, "003694") || strings.HasPrefix(accountNumber, "903694")) {
		return ErrInvalidAccountNumber
	}

	return nil
}
