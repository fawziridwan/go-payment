package handler

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

func Handler(w http.ResponseWriter, r *http.Request) {
	banking.Once.Do(func() {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			log.Println("DATABASE_URL environment variable is required")
			return
		}

		var err error
		banking.DB, err = sql.Open("postgres", dsn)
		if err != nil {
			log.Printf("failed to open database: %v", err)
			return
		}

		// Configure pool for serverless environment
		banking.DB.SetMaxOpenConns(10)
		banking.DB.SetMaxIdleConns(5)
		banking.DB.SetConnMaxLifetime(time.Minute * 5)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		if os.Getenv("RUN_MIGRATIONS") == "true" {
			_ = banking.RunMigrations(ctx, banking.DB)
		}

		repo := banking.NewRepository(banking.DB)
		banking.Router = banking.NewAppRouter(repo)
	})

	if banking.Router == nil {
		http.Error(w, "Internal Server Error", 500)
		return
	}

	banking.Router.ServeHTTP(w, r)
}
