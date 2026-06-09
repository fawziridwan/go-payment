package transfer

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Transfer(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		h.writeErrorJSON(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorJSON(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	if err := h.validateTransferRequest(req); err != nil {
		h.writeErrorJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	transactionID, err := h.service.TransferFunds(r.Context(), req.FromAccountNumber, req.ToAccountNumber, req.Amount)
	if err != nil {
		status := http.StatusInternalServerError
		switch err {
		case ErrAccountNotFound:
			status = http.StatusNotFound
		case ErrInsufficientFunds:
			status = http.StatusUnprocessableEntity
		case ErrInvalidAccountNumber:
			status = http.StatusBadRequest
		default:
			status = http.StatusInternalServerError
		}
		h.writeErrorJSON(w, status, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, TransferResponse{
		TransactionID: transactionID,
		Message:       "transfer completed",
		UserID:        user.ID,
		UserEmail:     user.Email,
	})
}

func (h *Handler) CheckBalance(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		h.writeErrorJSON(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	var req CheckBalanceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeErrorJSON(w, http.StatusBadRequest, "invalid request payload")
		return
	}

	transaction, err := h.service.CheckBalance(r.Context(), req.TransactionID)
	if err != nil {
		status := http.StatusInternalServerError
		switch err {
		case ErrTransactionNotFound:
			status = http.StatusNotFound
		default:
			status = http.StatusInternalServerError
		}
		h.writeErrorJSON(w, status, err.Error())
		return
	}

	balance, err := h.service.GetBalanceByID(r.Context(), transaction.ToAccountID)
	if err != nil {
		h.writeErrorJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, CheckBalanceResponse{
		Balance:       balance,
		AccountID:     transaction.ToAccountID,
		TransactionID: transaction.UUID,
		Amount:        transaction.Amount,
		Message:       "balance retrieved successfully",
		UserID:        user.ID,
		UserEmail:     user.Email,
	})
}

func (h *Handler) TransactionHistory(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user from context
	user, err := GetUserFromContext(r.Context())
	if err != nil {
		h.writeErrorJSON(w, http.StatusUnauthorized, "user not authenticated")
		return
	}

	accountNumber := r.URL.Query().Get("source_account")
	if accountNumber == "" {
		h.writeErrorJSON(w, http.StatusBadRequest, "source_account is required")
		return
	}

	filterDaysStr := r.URL.Query().Get("filter_days")
	filterDays := 30
	if filterDaysStr != "" {
		days, err := strconv.Atoi(filterDaysStr)
		if err == nil && (days == 7 || days == 30 || days == 90) {
			filterDays = days
		}
	}

	transactions, err := h.service.GetTransactionHistory(r.Context(), accountNumber, filterDays)
	if err != nil {
		status := http.StatusInternalServerError
		if err == ErrAccountNotFound {
			status = http.StatusNotFound
		} else if err == ErrInvalidAccountNumber {
			status = http.StatusBadRequest
		}
		h.writeErrorJSON(w, status, err.Error())
		return
	}

	historyItems := h.buildTransactionHistoryItems(accountNumber, transactions)

	h.writeJSON(w, http.StatusOK, TransactionHistoryResponse{
		Transactions: historyItems,
		Message:      "transaction history retrieved successfully",
		UserID:       user.ID,
		UserEmail:    user.Email,
	})
}

func (h *Handler) buildTransactionHistoryItems(accountNumber string, transactions []Transaction) []TransactionHistoryItem {
	var items []TransactionHistoryItem
	for _, txn := range transactions {
		var balanceChange string
		var name string

		if strings.Contains(accountNumber, fmt.Sprintf("%d", txn.ToAccountID)) || txn.ToAccountID == txn.FromAccountID {
			balanceChange = fmt.Sprintf("+%d", txn.Amount)
			name = "Received Transfer"
		} else {
			balanceChange = fmt.Sprintf("-%d", txn.Amount)
			name = "Sent Transfer"
		}

		item := TransactionHistoryItem{
			RetrievalRefNumber: txn.UUID,
			TransactionName:    name,
			BalanceChange:      balanceChange,
			Notes:              txn.Notes,
			TransactionDate:    txn.CreatedAt,
		}
		items = append(items, item)
	}
	return items
}

func (h *Handler) validateTransferRequest(req TransferRequest) error {
	if req.FromAccountNumber == "" {
		return errors.New("from_account is required")
	}
	if req.ToAccountNumber == "" {
		return errors.New("to_account is required")
	}
	if req.Amount <= 0 {
		return errors.New("amount must be greater than zero")
	}

	if err := validateAccountNumber(req.FromAccountNumber); err != nil {
		return fmt.Errorf("invalid from_account: %w", err)
	}
	if err := validateAccountNumber(req.ToAccountNumber); err != nil {
		return fmt.Errorf("invalid to_account: %w", err)
	}

	return nil
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload)
}

func (h *Handler) writeErrorJSON(w http.ResponseWriter, status int, message string) {
	h.writeJSON(w, status, map[string]string{"error": message})
}
