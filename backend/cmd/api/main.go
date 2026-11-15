package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"ims/internal/admin"
	"ims/internal/auth"
	"ims/internal/catalog"
	"ims/internal/config"
	"ims/internal/export"
	"ims/internal/finance"
	"ims/internal/httpx"
	"ims/internal/inventory"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("database unreachable: %v", err)
	}

	if err := bootstrapAdmin(ctx, pool, cfg); err != nil {
		log.Fatalf("bootstrap admin: %v", err)
	}

	authH := &auth.Handler{DB: pool, Secure: cfg.Env == "production", AllowRegistration: cfg.AllowRegistration}
	router := chi.NewRouter()
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CorsOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))
	router.Use(middleware.Recoverer, middleware.Logger, httpx.SecurityHeaders, httpx.SameOrigin(cfg.CorsOrigins))
	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			httpx.Err(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
		_, _ = w.Write([]byte("ok"))
	})
	router.Route("/api", func(api chi.Router) {
		authH.Public(api)
		api.Group(func(priv chi.Router) {
			priv.Use(authH.Middleware)
			authH.Private(priv)
			(&catalog.Handler{DB: pool}).Routes(priv)
			(&inventory.Service{DB: pool}).Routes(priv)
			(&finance.Handler{DB: pool}).Routes(priv)
			(&export.Handler{DB: pool}).Routes(priv)
			(&admin.Handler{DB: pool}).Routes(priv)
		})
	})

	srv := &http.Server{Addr: cfg.Addr, Handler: router, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	log.Printf("listening on %s (%s) registration=%v", cfg.Addr, cfg.Env, cfg.AllowRegistration)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

// bootstrapAdmin creates the first admin when the users table is empty and
// BOOTSTRAP_ADMIN_EMAIL + BOOTSTRAP_ADMIN_PASSWORD are set. Safe for production:
// runs only once, never seeds a password into SQL migrations or git history.
func bootstrapAdmin(ctx context.Context, pool *pgxpool.Pool, cfg config.Config) error {
	email := strings.TrimSpace(strings.ToLower(cfg.BootstrapAdminEmail))
	pass := cfg.BootstrapAdminPassword
	if email == "" || pass == "" {
		return nil
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	if err := auth.ValidateNewUser("Admin", email, pass); err != nil {
		return err
	}
	hash, err := auth.HashPassword(pass)
	if err != nil {
		return err
	}
	name := strings.TrimSpace(cfg.BootstrapAdminName)
	if name == "" {
		name = "Admin"
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO users (name, email, password_hash, role) VALUES ($1,$2,$3,'admin')`,
		name, email, hash)
	if err != nil {
		return err
	}
	log.Printf("bootstrap: created first admin %s — remove BOOTSTRAP_ADMIN_* env vars after first login", email)
	return nil
}
