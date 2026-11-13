//backend/cmd/api/main.go
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"

	"ims/internal/auth"
	"ims/internal/catalog"
	"ims/internal/config"
	"ims/internal/finance"
	"ims/internal/httpx"
	"ims/internal/inventory"
	"ims/internal/export"
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
	router.Use(middleware.Recoverer, middleware.Logger, httpx.SameOrigin(cfg.CorsOrigins))
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
		})
	})

	srv := &http.Server{Addr: cfg.Addr, Handler: router, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(sctx)
	}()
	log.Printf("listening on %s (%s)", cfg.Addr, cfg.Env)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
