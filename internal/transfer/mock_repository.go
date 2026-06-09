package transfer

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

type MockRepository struct {
	mu           sync.Mutex
	accounts     map[int64]*Account
	transactions map[string]*Transaction
}

func NewMockRepository() *MockRepository {
	return &MockRepository{
		accounts: map[int64]*Account{
			1: {ID: 9036091190, Balance: 1000000000},
			2: {ID: 9036091192, Balance: 500000000},
			3: {ID: 9036091193, Balance: 0},
		},
		transactions: make(map[string]*Transaction),
	}
}

func (r *MockRepository) Transfer(ctx context.Context, fromAccountID, toAccountID, amount int64) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if amount <= 0 {
		return "", errors.New("amount must be greater than zero")
	}

	from, ok := r.accounts[fromAccountID]
	if !ok {
		return "", ErrAccountNotFound
	}

	to, ok := r.accounts[toAccountID]
	if !ok {
		return "", ErrAccountNotFound
	}

	if from.Balance < amount {
		return "", ErrInsufficientFunds
	}

	from.Balance -= amount
	to.Balance += amount

	txnUUID := uuid.New().String()
	transaction := &Transaction{
		UUID:          txnUUID,
		FromAccountID: fromAccountID,
		ToAccountID:   toAccountID,
		Amount:        amount,
		CreatedAt:     time.Now(),
	}
	r.transactions[txnUUID] = transaction

	return txnUUID, nil
}

func (r *MockRepository) GetTransactionByUUID(ctx context.Context, txnUUID string) (*Transaction, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	txn, ok := r.transactions[txnUUID]
	if !ok {
		return nil, ErrTransactionNotFound
	}

	return txn, nil
}

func (r *MockRepository) GetBalance(ctx context.Context, accountID int64) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	account, ok := r.accounts[accountID]
	if !ok {
		return 0, ErrAccountNotFound
	}

	return account.Balance, nil
}
