package banking

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

// RunMigrations creates all required database tables
func RunMigrations(ctx context.Context, db *sql.DB) error {
	log.Println("Running banking API database migrations...")

	migrations := []string{
		// 1. banking_accounts table
		`CREATE TABLE IF NOT EXISTS banking_accounts (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			account_number VARCHAR(50) NOT NULL UNIQUE,
			account_name VARCHAR(255) NOT NULL,
			bank_code VARCHAR(20) NOT NULL,
			currency VARCHAR(10) NOT NULL DEFAULT 'IDR',
			status VARCHAR(20) NOT NULL DEFAULT 'ACTIVE',
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
			deleted_at TIMESTAMP WITH TIME ZONE NULL
		)`,

		// 2. balances table
		`CREATE TABLE IF NOT EXISTS balances (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			account_id UUID NOT NULL REFERENCES banking_accounts(id),
			available_balance DECIMAL(18,2) NOT NULL DEFAULT 0,
			ledger_balance DECIMAL(18,2) NOT NULL DEFAULT 0,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
			UNIQUE(account_id)
		)`,

		// 3. banking_transactions table
		`CREATE TABLE IF NOT EXISTS banking_transactions (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			reference_number VARCHAR(100) UNIQUE,
			source_account_id UUID NOT NULL REFERENCES banking_accounts(id),
			destination_account VARCHAR(50) NOT NULL,
			destination_bank VARCHAR(20) NOT NULL,
			amount DECIMAL(18,2) NOT NULL CHECK (amount > 0),
			fee DECIMAL(18,2) NOT NULL DEFAULT 0,
			currency VARCHAR(10) NOT NULL DEFAULT 'IDR',
			status VARCHAR(20) NOT NULL DEFAULT 'PENDING',
			remark VARCHAR(255),
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
		)`,

		// 4. mutations table
		`CREATE TABLE IF NOT EXISTS mutations (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			transaction_id UUID NOT NULL REFERENCES banking_transactions(id),
			account_id UUID NOT NULL REFERENCES banking_accounts(id),
			type VARCHAR(20) NOT NULL,
			amount DECIMAL(18,2) NOT NULL,
			balance_after DECIMAL(18,2) NOT NULL,
			description VARCHAR(255),
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
		)`,

		// 5. idempotency_keys table
		`CREATE TABLE IF NOT EXISTS idempotency_keys (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			idempotency_key VARCHAR(255) NOT NULL UNIQUE,
			request_hash VARCHAR(255) NOT NULL,
			response JSONB NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
		)`,

		// 6. audit_logs table
		`CREATE TABLE IF NOT EXISTS audit_logs (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			request_id VARCHAR(100) NOT NULL,
			user_id VARCHAR(100),
			action VARCHAR(50) NOT NULL,
			method VARCHAR(10) NOT NULL,
			path VARCHAR(255) NOT NULL,
			request_payload JSONB,
			response_payload JSONB,
			response_code INT,
			ip_address VARCHAR(50),
			user_agent VARCHAR(500),
			timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
		)`,

		// Indexes
		`CREATE INDEX IF NOT EXISTS idx_banking_accounts_number ON banking_accounts(account_number)`,
		`CREATE INDEX IF NOT EXISTS idx_banking_accounts_status ON banking_accounts(status)`,
		`CREATE INDEX IF NOT EXISTS idx_balances_account_id ON balances(account_id)`,
		`CREATE INDEX IF NOT EXISTS idx_banking_txn_reference ON banking_transactions(reference_number)`,
		`CREATE INDEX IF NOT EXISTS idx_banking_txn_source ON banking_transactions(source_account_id)`,
		`CREATE INDEX IF NOT EXISTS idx_banking_txn_status ON banking_transactions(status)`,
		`CREATE INDEX IF NOT EXISTS idx_banking_txn_created ON banking_transactions(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_mutations_txn_id ON mutations(transaction_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mutations_account_id ON mutations(account_id)`,
		`CREATE INDEX IF NOT EXISTS idx_mutations_type ON mutations(type)`,
		`CREATE INDEX IF NOT EXISTS idx_mutations_created ON mutations(created_at)`,
		`CREATE INDEX IF NOT EXISTS idx_idempotency_key ON idempotency_keys(idempotency_key)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id ON audit_logs(user_id)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_action ON audit_logs(action)`,
		`CREATE INDEX IF NOT EXISTS idx_audit_logs_timestamp ON audit_logs(timestamp)`,
	}

	for i, migration := range migrations {
		_, err := db.ExecContext(ctx, migration)
		if err != nil {
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}
	}

	log.Println("Banking API database migrations completed successfully")
	return nil
}
