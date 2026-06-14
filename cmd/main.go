package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/example/go-payment/pkg/banking"
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
	
	// Initialize Router
	r := banking.NewAppRouter(bankingRepo)

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
