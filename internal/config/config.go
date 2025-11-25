package config

import "os"

type Config struct {
	Port        string
	DatabaseURL string
}

func FromEnv() Config {
	return Config{
		Port:        getenv("PORT", "8080"),
		DatabaseURL: getenv("DATABASE_URL", "postgres://postgres:postgres@db:5432/reviewer?sslmode=disable"),
	}
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
