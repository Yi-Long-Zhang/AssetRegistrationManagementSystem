package config

import (
	"os"
	"time"
)

type Config struct {
	HTTPAddr       string
	DatabasePath   string
	JWTSecret      string
	TokenTTL       time.Duration
	AdminUsername  string
	AdminPassword  string
	AllowedOrigins string
}

func Load() Config {
	return Config{
		HTTPAddr:       env("HTTP_ADDR", ":8080"),
		DatabasePath:   env("DATABASE_PATH", "data/assets.db"),
		JWTSecret:      env("JWT_SECRET", "change-me-in-production"),
		TokenTTL:       24 * time.Hour,
		AdminUsername:  env("ADMIN_USERNAME", "admin"),
		AdminPassword:  env("ADMIN_PASSWORD", "admin123456"),
		AllowedOrigins: env("ALLOWED_ORIGINS", "*"),
	}
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
