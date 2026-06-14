package handler

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/example/go-payment/pkg/banking"
	_ "github.com/lib/pq"
)

var (
	db     *sql.DB
	once   sync.Once
	router http.Handler
)

func Handler(w http.ResponseWriter, r *http.Request) {
	once.Do(func() {
		dsn := os.Getenv("DATABASE_URL")
		if dsn == "" {
			log.Println("DATABASE_URL environment variable is required")
			return
		}

		var err error
		db, err = sql.Open("postgres", dsn)
		if err != nil {
			log.Printf("failed to open database: %v", err)
			return
		}

		db.SetMaxOpenConns(10)
		db.SetMaxIdleConns(5)
		db.SetConnMaxLifetime(time.Minute * 5)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			log.Printf("failed to connect to database: %v", err)
		}

		if os.Getenv("RUN_MIGRATIONS") == "true" {
			if err := banking.RunMigrations(ctx, db); err != nil {
				log.Printf("failed to run migrations: %v", err)
			}
		}

		repo := banking.NewRepository(db)
		router = banking.NewAppRouter(repo)
	})

	if router == nil {
		http.Error(w, "Internal Server Error: Router not initialized", http.StatusInternalServerError)
		return
	}

	router.ServeHTTP(w, r)
}
