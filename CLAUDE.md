# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
go build -o server main.go   # Build the server binary
go test ./...                # Run tests
go vet ./...                 # Static analysis
go fmt ./...                 # Format code
```

The server runs on port 8080 (or `$PORT` env var) and requires a PostgreSQL connection (`DATABASE_URL` env var, falls back to local postgres).

## Architecture

Gin framework + PostgreSQL (`lib/pq`).

```
imposter-api/
├── main.go              # Bootstrap: init DB, register routes, start server
├── db/db.go             # DB connection and migrations
├── models/review.go     # Review struct + DB queries
├── handlers/review.go   # HTTP handler functions
└── routes/routes.go     # Route registration
```

**Endpoints**:
- `GET /ping` — health check
- `POST /reviews` — submit a review (description + star rating 1–5)
