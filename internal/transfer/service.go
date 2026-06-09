package transfer

import (
	"context"
	"errors"
)

type Repository interface {
	Transfer(ctx context.Context, fromAccountNumber, toAccountNumber string, amount int64) (string, error)
	GetTransactionByUUID(ctx context.Context, uuid string) (*Transaction, error)
	GetBalance(ctx context.Context, accountNumber string) (int64, error)
	GetBalanceByID(ctx context.Context, accountID int64) (int64, error)
	GetTransactionHistory(ctx context.Context, accountNumber string, days int) ([]Transaction, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) TransferFunds(ctx context.Context, fromAccountNumber, toAccountNumber string, amount int64) (string, error) {
	if amount <= 0 {
		return "", errors.New("amount must be greater than zero")
	}

	return s.repo.Transfer(ctx, fromAccountNumber, toAccountNumber, amount)
}

func (s *Service) CheckBalance(ctx context.Context, transactionUUID string) (*Transaction, error) {
	if transactionUUID == "" {
		return nil, errors.New("transaction_id is required")
	}

	return s.repo.GetTransactionByUUID(ctx, transactionUUID)
}

func (s *Service) GetBalance(ctx context.Context, accountNumber string) (int64, error) {
	return s.repo.GetBalance(ctx, accountNumber)
}

func (s *Service) GetBalanceByID(ctx context.Context, accountID int64) (int64, error) {
	return s.repo.GetBalanceByID(ctx, accountID)
}

func (s *Service) GetTransactionHistory(ctx context.Context, accountNumber string, days int) ([]Transaction, error) {
	if accountNumber == "" {
		return nil, errors.New("account number is required")
	}

	if days <= 0 {
		days = 30
	}

	return s.repo.GetTransactionHistory(ctx, accountNumber, days)
}
