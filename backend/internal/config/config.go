package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Addr, DatabaseURL, Env string
	AllowRegistration      bool
	CorsOrigins            []string
	BootstrapAdminEmail    string
	BootstrapAdminPassword string
	BootstrapAdminName     string
}

func Load() Config {
	_ = godotenv.Load() // optional .env for local dev; real environment variables win
	c := Config{
		Addr:                   get("ADDR", ":8080"),
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		Env:                    get("APP_ENV", "development"),
		BootstrapAdminEmail:    os.Getenv("BOOTSTRAP_ADMIN_EMAIL"),
		BootstrapAdminPassword: os.Getenv("BOOTSTRAP_ADMIN_PASSWORD"),
		BootstrapAdminName:     get("BOOTSTRAP_ADMIN_NAME", "Admin"),
	}
	// Registration is closed unless explicitly opened. Prefer admin-created accounts.
	reg := os.Getenv("ALLOW_REGISTRATION")
	c.AllowRegistration = reg == "true"
	for _, o := range strings.Split(get("CORS_ORIGINS", "http://localhost:5173,http://localhost:4173"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.CorsOrigins = append(c.CorsOrigins, o)
		}
	}
	if c.DatabaseURL == "" {
		log.Fatal("DATABASE_URL is required")
	}
	return c
}

func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
