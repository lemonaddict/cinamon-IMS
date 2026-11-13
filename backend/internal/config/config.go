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
}

func Load() Config {
	_ = godotenv.Load() // optional .env for local dev; real environment variables win
	c := Config{Addr: get("ADDR", ":8080"), DatabaseURL: os.Getenv("DATABASE_URL"), Env: get("APP_ENV", "development")}
	reg := os.Getenv("ALLOW_REGISTRATION") // open in development, closed in production unless set to true
	c.AllowRegistration = reg == "true" || (reg == "" && c.Env != "production")
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
