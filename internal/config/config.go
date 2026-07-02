package config

import (
	"errors"
	"os"
	"strconv"
	"time"
)

type Config struct {
	BaysePublicKey  string
	BayseBaseUrl    string
	DatabaseUrl     string
	HttpPort        string
	PollInterval    time.Duration
	PollTickTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		BaysePublicKey:  os.Getenv("BAYSE_PUBLIC_KEY"),
		BayseBaseUrl:    envOrDefault("BAYSE_BASE_URL", "https://relay.bayse.markets"),
		DatabaseUrl:     os.Getenv("DATABASE_URL"),
		HttpPort:        envOrDefault("HTTP_PORT", "8080"),
		PollInterval:    secondsEnv("POLL_INTERVAL_SECONDS", 20),
		PollTickTimeout: secondsEnv("POLL_TICK_TIMEOUT_SECONDS", 10),
	}

	if cfg.DatabaseUrl == "" {
		return Config{}, errors.New("DATABASE_URL is required")
	}
	if cfg.BaysePublicKey == "" {
		return Config{}, errors.New("BAYSE_PUBLIC_KEY is required")
	}
	if cfg.PollInterval <= 0 {
		return Config{}, errors.New("POLL_INTERVAL_SECONDS must be positive")
	}
	if cfg.PollTickTimeout <= 0 {
		return Config{}, errors.New("POLL_TICK_TIMEOUT_SECONDS must be positive")
	}

	return cfg, nil
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func secondsEnv(key string, fallback int) time.Duration {
	raw := os.Getenv(key)
	if raw == "" {
		return time.Duration(fallback) * time.Second
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return time.Duration(fallback) * time.Second
	}
	return time.Duration(value) * time.Second
}
