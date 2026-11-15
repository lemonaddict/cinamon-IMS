# SOHO Inventory Management System (CIMOL)

Go + PostgreSQL backend, Svelte + Vite + TypeScript frontend. Runs on Windows without Docker.

```
ims/
  start-dev.cmd           starts backend + frontend in two windows (Windows)
  docker-compose.yml      optional: Postgres + migrations + API
  backend/
    cmd/api/main.go       entry point, routing, admin bootstrap
    internal/auth/        login, sessions, rate limit, roles
    internal/admin/       admin-only user management
    internal/catalog/     categories, locations, products
    internal/inventory/   stock ledger, balances, opname, dashboard (+ tests)
    internal/httpx/       JSON helpers, error mapping, security headers, same-origin
    internal/config/      environment config
    migrations/           SQL migrations (golang-migrate)
  frontend/src/           Svelte app (sign-in, dashboard, products, stock, finance, admin)
```

## Accounts

- **Self-registration is disabled.** Only an admin can create users (page **Pengguna**).
- First admin is created once via env vars when the `users` table is empty:
  - `BOOTSTRAP_ADMIN_EMAIL`
  - `BOOTSTRAP_ADMIN_PASSWORD` (10–72 chars)
  - `BOOTSTRAP_ADMIN_NAME` (optional, default `Admin`)
- After the first successful login, **remove** the bootstrap env vars.
- Do **not** put passwords in SQL migrations (they end up in git history and every clone).

`ALLOW_REGISTRATION=true` remains as an emergency hatch only.

## Prerequisites

1. PostgreSQL 14+ and a database (e.g. `ims`).
2. Go 1.22+ and Node 18+.
3. Install the migration tool: `go install -tags postgres github.com/golang-migrate/migrate/v4/cmd/migrate@latest`
4. Apply migrations: `migrate -path migrations -database "postgres://ims:change-me@localhost:5432/ims?sslmode=disable" up`
5. In `backend`: `go mod tidy`. In `frontend`: `npm install`.

## Run

Double-click `start-dev.cmd`, or use two terminals:

- `cd backend` then set env (see `.env.example`) and `go run ./cmd/api` (http://localhost:8080, health at `/healthz`)
- `cd frontend` then `npm run dev` (http://localhost:5173, proxies `/api` to the backend)

Local example `.env` in `backend/`:

```
APP_ENV=development
DATABASE_URL=postgres://ims:change-me@localhost:5432/ims?sslmode=disable
ALLOW_REGISTRATION=false
BOOTSTRAP_ADMIN_EMAIL=admin@example.com
BOOTSTRAP_ADMIN_PASSWORD=change-me-now-12
CORS_ORIGINS=http://localhost:5173,http://localhost:4173
```

## Tests

- `cd backend` then `go test ./...` runs the unit tests.
- Integration tests need a THROWAWAY database with migrations applied:
  `set TEST_DATABASE_URL=postgres://ims:change-me@localhost:5432/ims_test?sslmode=disable` then `go test ./...`

## Security notes

- Sessions: random 32-byte token, only SHA-256 hash stored, HttpOnly cookie, Secure in production, SameSite=Lax.
- Tenant isolation: every data query is scoped by `user_id` from the session (never from the client body). Composite FKs enforce the same boundary in the database.
- RLS policies are enabled on tenant tables as defense-in-depth. They are not FORCE'd so a typical free-tier owner role still works; app-level filters remain authoritative with a connection pool.
- Login/register rate-limited per client IP (honours `X-Forwarded-For` / `X-Real-IP` behind a proxy).
- Security headers: nosniff, DENY framing, Referrer-Policy, Permissions-Policy, basic CSP on API.
- Same-origin guard on state-changing requests.

## Docker (optional)

`set DB_PASSWORD=...` then `docker compose up --build`. Run `go mod tidy` first so `go.sum` exists. For production, run `npm run build` and serve `frontend/dist` from Caddy/nginx, proxying `/api` to the backend.
