# SOHO Inventory Management System

Go + PostgreSQL backend, Svelte + Vite + TypeScript frontend. Runs on Windows without Docker.

```
ims/
  start-dev.cmd           starts backend + frontend in two windows (Windows)
  docker-compose.yml      optional: Postgres + migrations + API
  backend/
    cmd/api/main.go       entry point and routing
    internal/auth/        register, login, sessions, rate limit
    internal/catalog/     categories, locations, products
    internal/inventory/   stock ledger, balances, opname, dashboard (+ tests)
    internal/httpx/       JSON helpers, error mapping, same-origin guard
    internal/config/      environment config
    migrations/           SQL migrations (golang-migrate)
  frontend/src/           Svelte app (sign-in, dashboard, products, stock, history, setup)
```

## Prerequisites (installed directly on Windows)
Go 1.22+, PostgreSQL 14+, Node.js 20+.

## One-time setup
1. Create the database: `CREATE USER ims WITH PASSWORD 'change-me'; CREATE DATABASE ims OWNER ims;`
2. In `backend`: copy `.env.example` to `.env` and adjust `DATABASE_URL`.
3. Install the migration tool: `go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest`
4. Apply migrations: `migrate -path migrations -database "postgres://ims:change-me@localhost:5432/ims?sslmode=disable" up`
5. In `backend`: `go mod tidy`. In `frontend`: `npm install`.

## Run
Double-click `start-dev.cmd`, or use two terminals:
- `cd backend` then `go run ./cmd/api` (http://localhost:8080, health check at `/healthz`)
- `cd frontend` then `npm run dev` (http://localhost:5173, proxies `/api` to the backend)

## Tests
- `cd backend` then `go test ./...` runs the unit tests.
- Integration tests (tenant isolation, concurrent stock-out, idempotent replay, opname) need a THROWAWAY database with migrations applied, because the ledger is append-only:
  `set TEST_DATABASE_URL=postgres://ims:change-me@localhost:5432/ims_test?sslmode=disable` then `go test ./...`

## Configuration
`ALLOW_REGISTRATION` is `true` in development and `false` when `APP_ENV=production` unless you set it. Accounts will be provisioned by the ISP integration later.

## Docker (optional)
`set DB_PASSWORD=...` then `docker compose up --build`. Run `go mod tidy` first so `go.sum` exists. For production, run `npm run build` and serve `frontend/dist` from Caddy, proxying `/api` to the backend.
