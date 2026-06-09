package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/example/go-payment/internal/transfer"
	"github.com/go-chi/chi/v5"
	_ "github.com/lib/pq"
)

func main() {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/payment?sslmode=disable"
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

	// Initialize authentication client
	loginServiceURL := os.Getenv("NODE_LOGIN_SERVICE_URL")
	if loginServiceURL == "" {
		loginServiceURL = "https://backend-course-gamma.vercel.app"
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
		w.Write([]byte("ok"))
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
