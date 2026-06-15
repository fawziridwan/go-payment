package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/example/go-payment/pkg/banking"
	_ "github.com/lib/pq"
)

func main() {
	// 1. Initial Setup
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Println("DATABASE_URL environment variable is required")
		// In a serverless environment, we don't want to log.Fatal here 
		// because it might crash the runtime before Vercel can handle it.
		// But for a root main.go, it's generally okay.
		os.Exit(1)
	}

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("failed to open database: %v", err)
	}
	defer db.Close()

	// 2. Database Connection Check & Config (Optimized for performance)
	db.SetMaxOpenConns(50)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(time.Minute * 5)
	db.SetConnMaxIdleTime(time.Minute * 2)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// 3. Optional: Run migrations
	if os.Getenv("RUN_MIGRATIONS") == "true" {
		if err := banking.RunMigrations(ctx, db); err != nil {
			log.Printf("failed to run migrations: %v", err)
		}
	}

	// 4. Initialize Application
	repo := banking.NewRepository(db)
	r := banking.NewAppRouter(repo)

	// 5. Start Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on :%s", port)
	
	// Vercel expects a standard ListenAndServe
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
