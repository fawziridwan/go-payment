package banking

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

var (
	DB     *sql.DB
	Once   sync.Once
	Router http.Handler
)

// NewAppRouter initializes the entire application router and handlers
func NewAppRouter(repo *Repository) *chi.Mux {
	bankingService := NewService(repo)
	bankingHandler := NewHandler(bankingService)

	// Configure API keys
	apiKeys := map[string]bool{}
	if key := os.Getenv("API_KEY"); key != "" {
		apiKeys[key] = true
	}
	// Development mode: allow without API key if explicitly set
	if os.Getenv("ENVIRONMENT") == "development" {
		apiKeys["dev-api-key"] = true
	}

	// JWT and HMAC secrets
	jwtSecret := os.Getenv("JWT_SECRET_KEY")
	hmacSecret := os.Getenv("HMAC_SECRET_KEY")
	if hmacSecret == "" {
		hmacSecret = jwtSecret // Fallback
	}

	// Rate limiter: 100 TPS
	rateLimiter := NewRateLimiter(100, 1*time.Second)

	r := chi.NewRouter()

	// Global middleware
	r.Use(CORSMiddleware)
	r.Use(RequestIDMiddleware)
	r.Use(LoggingMiddleware)

	// Banking API v1 Routes
	r.Route("/v1", func(r chi.Router) {
		r.Use(RateLimitMiddleware(rateLimiter))
		r.Use(SignatureMiddleware(hmacSecret))

		// Health check (v1 public)
		r.Get("/health", bankingHandler.HealthCheck)

		// Protected routes (JWT + API Key)
		r.Group(func(r chi.Router) {
			r.Use(JWTAuthMiddleware(jwtSecret))

			if len(apiKeys) > 0 {
				r.Use(APIKeyMiddleware(apiKeys))
			}

			r.Use(AuditMiddleware(repo))

			r.Post("/accounts", bankingHandler.AddAccount)
			r.Delete("/accounts/{accountId}", bankingHandler.RemoveAccount)
			r.Get("/accounts/{accountId}/balance", bankingHandler.GetBalance)
			r.Get("/accounts/{accountId}/mutations", bankingHandler.GetMutations)
			r.Post("/transfers", bankingHandler.Transfer)
		})
	})

	// Public Health endpoint
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"statusCode": "200",
			"message":    "Healthy",
			"timestamp":  time.Now().Format(time.RFC3339),
		})
	})

	// Root endpoint
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Welcome to the Banking Transaction API",
			"version": "1.0",
			"health":  "/health",
		})
	})

	return r
}
