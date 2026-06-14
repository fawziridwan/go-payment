# Go Payment Transfer Service

A Go microservice for account transfers and transaction management using PostgreSQL.

## Features

- `POST /transfer` - Transfer funds between accounts with validation
- `POST /balance` - Check balance after a transaction
- `GET /transaction-history` - Retrieve transaction history with date-range filtering
- PostgreSQL database backend
- Account number validation (prefix: 003694 or 903694)
- Transaction history filtering (7 days, 1 month, 3 months)

## Request Examples

### POST /transfer

Transfer funds between accounts. Both account numbers must have prefix 003694 or 903694.

```json
{
  "from_account": "00369400001",
  "to_account": "90369400001",
  "amount": 50000
}
```

**Response:**

```json
{
  "transaction_id": "uuid-string",
  "message": "transfer completed"
}
```

### POST /balance

Check balance after a transaction.

```json
{
  "transaction_id": "uuid-string"
}
```

**Response:**

```json
{
  "balance": 950000000,
  "account_id": 2,
  "transaction_id": "uuid-string",
  "amount": 50000,
  "message": "balance retrieved successfully"
}
```

### GET /transaction-history

Retrieve transaction history with optional date-range filtering.

**Query Parameters:**

- `source_account` (required): Account number (e.g., "00369400001")
- `filter_days` (optional): Filter range - 7, 30, or 90 days (default: 30)

```
GET /transaction-history?source_account=00369400001&filter_days=7
```

**Response:**

```json
{
  "transactions": [
    {
      "retrieval_reference_number": "uuid-string",
      "transaction_name": "Received Transfer",
      "balance_change": "+50000",
      "notes": "optional notes",
      "transaction_date": "2026-06-09T23:08:25.915Z"
    }
  ],
  "message": "transaction history retrieved successfully"
}
```

## Run locally

```bash
# Install dependencies
go mod download

# Ensure PostgreSQL is running and accessible
# Set environment variable for database connection
export DATABASE_URL="postgres://user:password@localhost:5432/payment"

# Run the application
go run ./cmd/main.go
```

## Deploy to Vercel

### Prerequisites

1. PostgreSQL database (e.g., AWS RDS, Neon, Supabase, or similar)
2. Vercel account
3. Git repository with this code

### Deployment Steps

1. **Create `vercel.json` in the root directory:**

```json
{
  "buildCommand": "go mod download && go build -o payment ./cmd",
  "outputDirectory": ".",
  "publicSource": "./",
  "framework": "go",
  "functions": {
    "cmd/**": {
      "memory": 1024,
      "maxDuration": 30
    }
  }
}
```

2. **Add environment variables in Vercel:**
   - Go to your Vercel project settings
   - Add `DATABASE_URL` with your PostgreSQL connection string
   - Add `PORT` (optional, defaults to 8080)

3. **Deploy:**

```bash
# Using Vercel CLI
vercel

# Or push to your main branch if connected to GitHub/GitLab
git push origin main
```

### Example PostgreSQL Providers

- **Neon**: https://neon.tech (Free tier available)
- **Supabase**: https://supabase.com (PostgreSQL included)
- **AWS RDS**: https://aws.amazon.com/rds/postgresql/
- **Railway**: https://railway.app

## Environment Variables

- `DATABASE_URL` - PostgreSQL connection string (required)
  - Format: `postgres://user:password@host:port/database?sslmode=require`
- `PORT` - Server port (optional, default: 8080)

## Database

The service uses PostgreSQL with the following tables:

- `accounts` - Account information with account numbers and balances
- `transactions` - Transaction records with UUID, amounts, and timestamps

### Database Setup

Run the migration manually on your PostgreSQL database:

```sql
-- From db/migrations/init.sql
CREATE TABLE IF NOT EXISTS accounts (
    id BIGSERIAL PRIMARY KEY,
    account_number VARCHAR(20) NOT NULL UNIQUE,
    balance BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS transactions (
    id BIGSERIAL PRIMARY KEY,
    uuid VARCHAR(36) NOT NULL UNIQUE,
    from_account_id BIGINT NOT NULL REFERENCES accounts(id),
    to_account_id BIGINT NOT NULL REFERENCES accounts(id),
    amount BIGINT NOT NULL CHECK (amount > 0),
    transaction_type VARCHAR(50),
    notes VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

CREATE INDEX idx_transactions_created_at ON transactions(created_at);
CREATE INDEX idx_transactions_from_account_id ON transactions(from_account_id);
CREATE INDEX idx_transactions_to_account_id ON transactions(to_account_id);
CREATE INDEX idx_transactions_uuid ON transactions(uuid);

INSERT INTO accounts (account_number, balance) VALUES
    ('00369400001', 1000000000),
    ('90369400001', 500000000),
    ('00369400002', 0);
```

## Validation

- Account numbers are required fields
- Account numbers must have prefix `003694` or `903694`
- Transfer amount must be greater than zero
- Source account must have sufficient balance
- Transaction history filtering supports 7, 30, or 90-day ranges

## API Health Check

```bash
curl http://localhost:8080/health
# Response: ok
```
