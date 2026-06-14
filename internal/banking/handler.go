package banking

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

// Handler handles HTTP requests for the banking API
type Handler struct {
	service *Service
}

// NewHandler creates a new banking handler
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// ============================================
// Health Check
// ============================================

// HealthCheck handles GET /v1/health
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.writeSuccess(w, HealthResponse{Status: "UP"})
}

// ============================================
// Account Handlers
// ============================================

// AddAccount handles POST /v1/accounts
func (h *Handler) AddAccount(w http.ResponseWriter, r *http.Request) {
	var req AddAccountRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, ErrInvalidRequest, "Invalid request payload")
		return
	}

	// Validate required fields
	if req.AccountNumber == "" || req.AccountName == "" || req.BankCode == "" {
		h.writeError(w, r, http.StatusBadRequest, ErrInvalidRequest,
			"accountNumber, accountName, and bankCode are required")
		return
	}

	if req.Currency == "" {
		req.Currency = "IDR"
	}

	accountID, err := h.service.AddAccount(r.Context(), req)
	if err != nil {
		status := GetHTTPStatus(err)
		h.writeError(w, r, status, err, err.Error())
		return
	}

	h.writeSuccess(w, AddAccountResponse{AccountID: accountID})
}

// RemoveAccount handles DELETE /v1/accounts/{accountId}
func (h *Handler) RemoveAccount(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountId")
	if accountID == "" {
		h.writeError(w, r, http.StatusBadRequest, ErrInvalidRequest, "accountId is required")
		return
	}

	err := h.service.RemoveAccount(r.Context(), accountID)
	if err != nil {
		status := GetHTTPStatus(err)
		h.writeError(w, r, status, err, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(RemoveAccountMessageResponse{
		Success: true,
		Message: "Account removed successfully",
	})
}

// ============================================
// Balance Handler
// ============================================

// GetBalance handles GET /v1/accounts/{accountId}/balance
func (h *Handler) GetBalance(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountId")
	if accountID == "" {
		h.writeError(w, r, http.StatusBadRequest, ErrInvalidRequest, "accountId is required")
		return
	}

	balance, err := h.service.GetBalance(r.Context(), accountID)
	if err != nil {
		status := GetHTTPStatus(err)
		h.writeError(w, r, status, err, err.Error())
		return
	}

	h.writeSuccess(w, balance)
}

// ============================================
// Mutation Handler
// ============================================

// GetMutations handles GET /v1/accounts/{accountId}/mutations
func (h *Handler) GetMutations(w http.ResponseWriter, r *http.Request) {
	accountID := chi.URLParam(r, "accountId")
	if accountID == "" {
		h.writeError(w, r, http.StatusBadRequest, ErrInvalidRequest, "accountId is required")
		return
	}

	// Parse query parameters
	fromDateStr := r.URL.Query().Get("fromDate")
	toDateStr := r.URL.Query().Get("toDate")

	if fromDateStr == "" || toDateStr == "" {
		h.writeError(w, r, http.StatusBadRequest, ErrInvalidRequest, "fromDate and toDate are required")
		return
	}

	fromDate, err := time.Parse("2006-01-02", fromDateStr)
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, ErrInvalidRequest, "Invalid fromDate format. Expected: YYYY-MM-DD")
		return
	}

	toDate, err := time.Parse("2006-01-02", toDateStr)
	if err != nil {
		h.writeError(w, r, http.StatusBadRequest, ErrInvalidRequest, "Invalid toDate format. Expected: YYYY-MM-DD")
		return
	}

	// End of toDate (include the whole day)
	toDate = toDate.Add(24*time.Hour - time.Second)

	txnType := r.URL.Query().Get("transactionType")

	page := 1
	if p := r.URL.Query().Get("page"); p != "" {
		if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
			page = parsed
		}
	}

	size := 20
	if s := r.URL.Query().Get("size"); s != "" {
		if parsed, err := strconv.Atoi(s); err == nil && parsed > 0 {
			size = parsed
		}
	}

	result, err := h.service.GetMutations(r.Context(), accountID, fromDate, toDate, txnType, page, size)
	if err != nil {
		status := GetHTTPStatus(err)
		h.writeError(w, r, status, err, err.Error())
		return
	}

	h.writeSuccess(w, result)
}

// ============================================
// Transfer Handler
// ============================================

// Transfer handles POST /v1/transfers
func (h *Handler) Transfer(w http.ResponseWriter, r *http.Request) {
	// Get idempotency key
	idempotencyKey := r.Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		h.writeError(w, r, http.StatusBadRequest, ErrInvalidRequest, "Idempotency-Key header is required")
		return
	}

	var req TransferRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, r, http.StatusBadRequest, ErrInvalidRequest, "Invalid request payload")
		return
	}

	result, err := h.service.Transfer(r.Context(), req, idempotencyKey)
	if err != nil {
		status := GetHTTPStatus(err)
		h.writeError(w, r, status, err, err.Error())
		return
	}

	h.writeSuccess(w, result)
}

// ============================================
// Response Helpers
// ============================================

func (h *Handler) writeSuccess(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(StandardResponse{
		Success: true,
		Data:    data,
	})
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, status int, errCode error, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Success:   false,
		ErrorCode: errCode.Error(),
		Message:   message,
		RequestID: GetRequestIDFromContext(r.Context()),
	})
}
