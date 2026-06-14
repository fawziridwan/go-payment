package banking

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ============================================
// Context Keys
// ============================================

type contextKey string

const (
	ContextKeyUser      contextKey = "authenticated_user"
	ContextKeyRequestID contextKey = "request_id"
	ContextKeyRequestBody contextKey = "request_body"
)

// ============================================
// Authenticated User
// ============================================

type AuthenticatedUser struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// GetUserFromContext retrieves the authenticated user from context
func GetUserFromContext(ctx context.Context) (*AuthenticatedUser, error) {
	user, ok := ctx.Value(ContextKeyUser).(*AuthenticatedUser)
	if !ok || user == nil {
		return nil, errors.New("user not authenticated")
	}
	return user, nil
}

// GetRequestIDFromContext retrieves the request ID from context
func GetRequestIDFromContext(ctx context.Context) string {
	if reqID, ok := ctx.Value(ContextKeyRequestID).(string); ok {
		return reqID
	}
	return ""
}

// ============================================
// Request ID Middleware
// ============================================

// RequestIDMiddleware adds a unique request ID to each request
func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		ctx := context.WithValue(r.Context(), ContextKeyRequestID, requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// ============================================
// JWT Authentication Middleware
// ============================================

// JWTAuthMiddleware validates JWT tokens from the Authorization header
func JWTAuthMiddleware(jwtSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				writeErrorResponse(w, r, http.StatusUnauthorized, ErrUnauthorized, "Authorization header is required")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				writeErrorResponse(w, r, http.StatusUnauthorized, ErrUnauthorized, "Invalid authorization header format. Expected: Bearer <token>")
				return
			}

			tokenString := parts[1]
			if tokenString == "" {
				writeErrorResponse(w, r, http.StatusUnauthorized, ErrUnauthorized, "Token is required")
				return
			}

			// Parse and validate JWT
			token, err := jwt.Parse(tokenString, func(t *jwt.Token) (interface{}, error) {
				if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
					return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
				}
				return []byte(jwtSecret), nil
			})

			if err != nil {
				writeErrorResponse(w, r, http.StatusUnauthorized, ErrUnauthorized, "Invalid or expired token")
				return
			}

			claims, ok := token.Claims.(jwt.MapClaims)
			if !ok || !token.Valid {
				writeErrorResponse(w, r, http.StatusUnauthorized, ErrUnauthorized, "Invalid token claims")
				return
			}

			user := &AuthenticatedUser{}

			// Extract user ID
			if id, ok := claims["id"].(float64); ok {
				user.ID = int(id)
			} else if idStr, ok := claims["id"].(string); ok {
				if id, err := strconv.Atoi(idStr); err == nil {
					user.ID = id
				}
			}

			// Extract email
			if email, ok := claims["email"].(string); ok {
				user.Email = email
			}

			// Extract name
			if name, ok := claims["name"].(string); ok {
				user.Name = name
			}

			ctx := context.WithValue(r.Context(), ContextKeyUser, user)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ============================================
// API Key Middleware
// ============================================

// APIKeyMiddleware validates the X-API-Key header
func APIKeyMiddleware(validAPIKeys map[string]bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			apiKey := r.Header.Get("X-API-Key")
			if apiKey == "" {
				writeErrorResponse(w, r, http.StatusUnauthorized, ErrUnauthorized, "X-API-Key header is required")
				return
			}

			if !validAPIKeys[apiKey] {
				writeErrorResponse(w, r, http.StatusForbidden, ErrForbidden, "Invalid API key")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ============================================
// HMAC Signature Middleware
// ============================================

// SignatureMiddleware validates HMAC SHA256 request signatures
func SignatureMiddleware(secretKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			signature := r.Header.Get("X-Signature")
			timestampStr := r.Header.Get("X-Timestamp")

			// If no signature headers present, skip validation (optional security layer)
			if signature == "" && timestampStr == "" {
				next.ServeHTTP(w, r)
				return
			}

			if signature == "" || timestampStr == "" {
				writeErrorResponse(w, r, http.StatusUnauthorized, ErrInvalidSignature, "Both X-Signature and X-Timestamp headers are required")
				return
			}

			// Validate timestamp (±5 minutes)
			timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
			if err != nil {
				writeErrorResponse(w, r, http.StatusUnauthorized, ErrTimestampExpired, "Invalid timestamp format")
				return
			}

			now := time.Now().Unix()
			skew := math.Abs(float64(now - timestamp))
			if skew > 300 { // 5 minutes
				writeErrorResponse(w, r, http.StatusUnauthorized, ErrTimestampExpired, "Request timestamp expired (max ±5 minutes)")
				return
			}

			// Read body for signature calculation
			var bodyBytes []byte
			if r.Body != nil {
				bodyBytes, err = io.ReadAll(r.Body)
				if err != nil {
					writeErrorResponse(w, r, http.StatusBadRequest, ErrInvalidRequest, "Failed to read request body")
					return
				}
				// Restore body for downstream handlers
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}

			// Calculate expected signature: HMAC_SHA256(secretKey, METHOD + PATH + BODY + TIMESTAMP)
			message := r.Method + r.URL.Path + string(bodyBytes) + timestampStr
			mac := hmac.New(sha256.New, []byte(secretKey))
			mac.Write([]byte(message))
			expectedSig := hex.EncodeToString(mac.Sum(nil))

			if !hmac.Equal([]byte(signature), []byte(expectedSig)) {
				writeErrorResponse(w, r, http.StatusUnauthorized, ErrInvalidSignature, "Invalid request signature")
				return
			}

			// Store body in context for downstream use
			ctx := context.WithValue(r.Context(), ContextKeyRequestBody, bodyBytes)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ============================================
// Audit Middleware
// ============================================

// AuditMiddleware logs all API requests and responses for audit trail
func AuditMiddleware(repo *Repository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Read request body
			var bodyBytes []byte
			if r.Body != nil {
				bodyBytes, _ = io.ReadAll(r.Body)
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}

			// Determine action from path
			action := determineAction(r.Method, r.URL.Path)

			// Capture response
			recorder := &responseRecorder{
				ResponseWriter: w,
				statusCode:     http.StatusOK,
				body:           &bytes.Buffer{},
			}

			next.ServeHTTP(recorder, r)

			// Save audit log asynchronously
			go func() {
				userID := ""
				if user, err := GetUserFromContext(r.Context()); err == nil {
					userID = fmt.Sprintf("%d", user.ID)
				}

				_ = repo.SaveAuditLog(context.Background(), AuditLog{
					RequestID:       GetRequestIDFromContext(r.Context()),
					UserID:          userID,
					Action:          action,
					Method:          r.Method,
					Path:            r.URL.Path,
					RequestPayload:  bodyBytes,
					ResponsePayload: recorder.body.Bytes(),
					ResponseCode:    recorder.statusCode,
					IPAddress:       getClientIP(r),
					UserAgent:       r.UserAgent(),
				})
			}()
		})
	}
}

// responseRecorder captures response status and body for audit logging
type responseRecorder struct {
	http.ResponseWriter
	statusCode int
	body       *bytes.Buffer
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.statusCode = code
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	rr.body.Write(b)
	return rr.ResponseWriter.Write(b)
}

// ============================================
// Rate Limiting Middleware
// ============================================

// RateLimiter implements a simple in-memory rate limiter
type RateLimiter struct {
	mu       sync.Mutex
	requests map[string][]time.Time
	limit    int
	window   time.Duration
}

// NewRateLimiter creates a rate limiter with specified requests per window
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		requests: make(map[string][]time.Time),
		limit:    limit,
		window:   window,
	}
}

// RateLimitMiddleware applies rate limiting per client IP
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			clientIP := getClientIP(r)

			limiter.mu.Lock()
			now := time.Now()
			windowStart := now.Add(-limiter.window)

			// Clean old entries
			var validRequests []time.Time
			for _, t := range limiter.requests[clientIP] {
				if t.After(windowStart) {
					validRequests = append(validRequests, t)
				}
			}
			limiter.requests[clientIP] = validRequests

			if len(validRequests) >= limiter.limit {
				limiter.mu.Unlock()
				writeErrorResponse(w, r, 429, ErrRateLimitExceeded, "Rate limit exceeded")
				return
			}

			limiter.requests[clientIP] = append(limiter.requests[clientIP], now)
			limiter.mu.Unlock()

			next.ServeHTTP(w, r)
		})
	}
}

// ============================================
// CORS Middleware
// ============================================

// CORSMiddleware adds CORS headers
func CORSMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Signature, X-Timestamp, Idempotency-Key, X-Request-ID")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ============================================
// Content-Type Middleware
// ============================================

// JSONContentTypeMiddleware sets JSON content type headers
func JSONContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// ============================================
// Logging Middleware
// ============================================

// LoggingMiddleware logs request details in structured format
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		recorder := &responseRecorder{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
			body:           &bytes.Buffer{},
		}

		next.ServeHTTP(recorder, r)

		duration := time.Since(start)
		log.Printf(`{"method":"%s","path":"%s","status":%d,"duration":"%s","ip":"%s","requestId":"%s"}`,
			r.Method, r.URL.Path, recorder.statusCode, duration,
			getClientIP(r), GetRequestIDFromContext(r.Context()))
	})
}

// ============================================
// Helper Functions
// ============================================

func writeErrorResponse(w http.ResponseWriter, r *http.Request, status int, errCode error, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(ErrorResponse{
		Success:   false,
		ErrorCode: errCode.Error(),
		Message:   message,
		RequestID: GetRequestIDFromContext(r.Context()),
	})
}

func getClientIP(r *http.Request) string {
	// Check X-Forwarded-For header first
	forwarded := r.Header.Get("X-Forwarded-For")
	if forwarded != "" {
		parts := strings.Split(forwarded, ",")
		return strings.TrimSpace(parts[0])
	}

	// Check X-Real-IP header
	realIP := r.Header.Get("X-Real-IP")
	if realIP != "" {
		return realIP
	}

	// Fall back to RemoteAddr
	addr := r.RemoteAddr
	if idx := strings.LastIndex(addr, ":"); idx != -1 {
		return addr[:idx]
	}
	return addr
}

func determineAction(method, path string) string {
	path = strings.TrimPrefix(path, "/v1")

	switch {
	case method == "POST" && strings.HasPrefix(path, "/accounts"):
		return "ADD_ACCOUNT"
	case method == "DELETE" && strings.HasPrefix(path, "/accounts"):
		return "REMOVE_ACCOUNT"
	case method == "GET" && strings.Contains(path, "/balance"):
		return "BALANCE_INQUIRY"
	case method == "GET" && strings.Contains(path, "/mutations"):
		return "MUTATION_INQUIRY"
	case method == "POST" && strings.HasPrefix(path, "/transfers"):
		return "TRANSFER"
	default:
		return "UNKNOWN"
	}
}
