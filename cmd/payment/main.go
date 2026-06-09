package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/example/go-payment/internal/transfer"
	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv" // Add this import
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

	// Initialize authentication client
	loginServiceURL := os.Getenv("NODE_LOGIN_SERVICE_URL")
	if loginServiceURL == "" {
		log.Fatal("NODE_LOGIN_SERVICE_URL environment variable is required")
	}

	cacheTTLStr := os.Getenv("AUTH_CACHE_TTL")
	cacheTTL := 1 * time.Hour
	if cacheTTLStr != "" {
		if minutes, err := strconv.Atoi(cacheTTLStr); err == nil && minutes > 0 {
			cacheTTL = time.Duration(minutes) * time.Minute
		}
	}

	authClient := transfer.NewAuthClient(loginServiceURL, cacheTTL)
	log.Printf("authentication configured with login service: %s (cache TTL: %v)", loginServiceURL, cacheTTL)

	repo := transfer.NewSQLRepository(db)
	service := transfer.NewService(repo)
	handler := transfer.NewHandler(service)

	r := chi.NewRouter()

	// Health endpoint - public (no authentication required)
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// w.Write([]byte("ok"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"statusCode": "200",
			"message":    "Healthy",
			"timestamp":  time.Now().Format(time.RFC3339),
		})
	})

	// Protected endpoints with authentication middleware
	r.Post("/transfer", transfer.AuthMiddleware(authClient)(http.HandlerFunc(handler.Transfer)).ServeHTTP)
	r.Post("/balance", transfer.AuthMiddleware(authClient)(http.HandlerFunc(handler.CheckBalance)).ServeHTTP)
	r.Get("/transaction-history", transfer.AuthMiddleware(authClient)(http.HandlerFunc(handler.TransactionHistory)).ServeHTTP)

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

	log.Printf("starting payment service on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
