package config

import (
	"errors"
	"os"
)

const defaultHTTPAddress = ":18080"

type Config struct {
	HTTPAddress string
	DatabaseURL string
	APIToken    string
}

func Load() (Config, error) {
	cfg := Config{
		HTTPAddress: envOrDefault("HTTP_ADDRESS", defaultHTTPAddress),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		APIToken:    os.Getenv("API_TOKEN"),
	}

	switch {
	case cfg.DatabaseURL == "":
		return Config{}, errors.New("DATABASE_URL is required")
	case cfg.APIToken == "":
		return Config{}, errors.New("API_TOKEN is required")
	default:
		return cfg, nil
	}
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
