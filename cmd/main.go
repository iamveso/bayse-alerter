package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/iamveso/internal/bayse"
	"github.com/iamveso/internal/config"
	"github.com/iamveso/internal/handler"
	"github.com/iamveso/internal/repository"
	"github.com/iamveso/internal/services"
	"github.com/joho/godotenv"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	err := godotenv.Load()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		logger.Error("Error loading .env file", "error", err)
		os.Exit(1)
	}

	cfg, err := config.Load()
	if err != nil {
		logger.Error("invalid config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	store, err := repository.Open(ctx, cfg.DatabaseUrl)
	logger.Info("database connected")
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer store.Close()

	bayseClient := bayse.NewClient(cfg.BayseBaseUrl, cfg.BaysePublicKey)
	ruleHandler := handler.NewRulesHandler(store, bayseClient, logger)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	mux.HandleFunc("POST /rules", ruleHandler.CreateRules)

	server := &http.Server{
		Addr:              ":" + cfg.HttpPort,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	poller := services.NewPoller(store, bayseClient, cfg.PollInterval, cfg.PollTickTimeout, logger)
	go poller.Run(ctx)

	go func() {
		logger.Info("http server starting...", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server failed to start", "error", err)
			stop()
		}
	}()

	<-ctx.Done()
	logger.Info("shutdown requested")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown failed", "error", err)
	}
}
