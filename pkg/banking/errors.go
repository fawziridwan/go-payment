package banking

import "errors"

// ============================================
// Error Codes (matching TSD error table)
// ============================================

// Application-level errors
var (
	ErrInvalidRequest      = errors.New("INVALID_REQUEST")
	ErrUnauthorized        = errors.New("UNAUTHORIZED")
	ErrForbidden           = errors.New("FORBIDDEN")
	ErrAccountNotFound     = errors.New("ACCOUNT_NOT_FOUND")
	ErrDuplicateTransaction = errors.New("DUPLICATE_TRANSACTION")
	ErrInsufficientBalance = errors.New("INSUFFICIENT_BALANCE")
	ErrRateLimitExceeded   = errors.New("RATE_LIMIT_EXCEEDED")
	ErrInternalError       = errors.New("INTERNAL_ERROR")
	ErrAccountAlreadyExists = errors.New("ACCOUNT_ALREADY_EXISTS")
	ErrAccountDeleted      = errors.New("ACCOUNT_DELETED")
	ErrInvalidSignature    = errors.New("INVALID_SIGNATURE")
	ErrTimestampExpired    = errors.New("TIMESTAMP_EXPIRED")
	ErrIdempotencyConflict = errors.New("IDEMPOTENCY_CONFLICT")
	ErrDestinationNotFound = errors.New("DESTINATION_NOT_FOUND")
)

// ErrorHTTPStatus maps error codes to HTTP status codes
var ErrorHTTPStatus = map[string]int{
	"INVALID_REQUEST":       400,
	"UNAUTHORIZED":          401,
	"FORBIDDEN":             403,
	"ACCOUNT_NOT_FOUND":     404,
	"DESTINATION_NOT_FOUND": 404,
	"DUPLICATE_TRANSACTION": 409,
	"IDEMPOTENCY_CONFLICT":  409,
	"INSUFFICIENT_BALANCE":  422,
	"RATE_LIMIT_EXCEEDED":   429,
	"INTERNAL_ERROR":        500,
	"ACCOUNT_ALREADY_EXISTS": 409,
	"ACCOUNT_DELETED":       410,
	"INVALID_SIGNATURE":     401,
	"TIMESTAMP_EXPIRED":     401,
}

// GetHTTPStatus returns the HTTP status code for an error
func GetHTTPStatus(err error) int {
	if status, ok := ErrorHTTPStatus[err.Error()]; ok {
		return status
	}
	return 500
}
