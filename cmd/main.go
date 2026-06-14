package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/example/go-payment/internal/banking"
	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	// Load .env file if it exists (for local development)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	log.Println("Database connected successfully")

	// ============================================
	// Run Banking API Migrations
	// ============================================
	if err := banking.RunMigrations(ctx, db); err != nil {
		log.Fatalf("failed to run banking migrations: %v", err)
	}

	// ============================================
	// Banking API v1 Setup
	// ============================================
	bankingRepo := banking.NewRepository(db)
	bankingService := banking.NewService(bankingRepo)
	bankingHandler := banking.NewHandler(bankingService)

	// Configure API keys (from environment or default for development)
	apiKeys := map[string]bool{}
	if key := os.Getenv("API_KEY"); key != "" {
		apiKeys[key] = true
	}
	// Development mode: allow without API key
	if os.Getenv("ENVIRONMENT") == "development" {
		apiKeys["dev-api-key"] = true
	}

	// JWT and HMAC secrets
	jwtSecret := os.Getenv("JWT_SECRET_KEY")
	if jwtSecret == "" {
		log.Println("WARNING: JWT_SECRET_KEY environment variable is not set. Token verification will fail.")
	}

	hmacSecret := os.Getenv("HMAC_SECRET_KEY")
	if hmacSecret == "" {
		hmacSecret = jwtSecret // Fallback to JWT secret
	}

	// Rate limiter: 100 TPS
	rateLimiter := banking.NewRateLimiter(100, 1*time.Second)

	// ============================================
	// Router Setup
	// ============================================
	r := chi.NewRouter()

	// Global middleware - MUST be before any routes
	r.Use(banking.CORSMiddleware)
	r.Use(banking.RequestIDMiddleware)
	r.Use(banking.LoggingMiddleware)

	// ============================================
	// Banking API v1 Routes
	// ============================================
	r.Route("/v1", func(r chi.Router) {
		// Middlewares for v1 - MUST be before routes
		r.Use(banking.RateLimitMiddleware(rateLimiter))
		r.Use(banking.SignatureMiddleware(hmacSecret))

		// Health check (v1 public)
		r.Get("/health", bankingHandler.HealthCheck)

		// Protected routes (JWT + API Key)
		r.Group(func(r chi.Router) {
			// JWT Authentication
			r.Use(banking.JWTAuthMiddleware(jwtSecret))

			// API Key validation (skip in development if no keys configured)
			if len(apiKeys) > 0 {
				r.Use(banking.APIKeyMiddleware(apiKeys))
			}

			// Audit logging
			r.Use(banking.AuditMiddleware(bankingRepo))

			// Account Management
			r.Post("/accounts", bankingHandler.AddAccount)
			r.Delete("/accounts/{accountId}", bankingHandler.RemoveAccount)

			// Balance Service
			r.Get("/accounts/{accountId}/balance", bankingHandler.GetBalance)

			// Mutation Service
			r.Get("/accounts/{accountId}/mutations", bankingHandler.GetMutations)

			// Transfer Service
			r.Post("/transfers", bankingHandler.Transfer)
		})
	})

	// ============================================
	// Public Routes
	// ============================================

	// Health endpoint - public (no authentication)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"statusCode": "200",
			"message":    "Healthy",
			"timestamp":  time.Now().Format(time.RFC3339),
		})
	})

	// ============================================
	// Start Server
	// ============================================

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Printf("===========================================")
	log.Printf("Banking Transaction API v1.0")
	log.Printf("===========================================")
	log.Printf("Server starting on :%s", port)
	log.Printf("API v1 routes: /v1/accounts, /v1/transfers, /v1/health")
	log.Printf("===========================================")

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}

}
