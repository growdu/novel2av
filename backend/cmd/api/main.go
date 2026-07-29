// Package main is the HTTP API entrypoint of novel2av-backend.
//
// Responsibilities:
//   - serve REST + WebSocket under /api/v1
//   - own the project/chapter/character/shot/job state machine in Postgres
//   - enqueue pipeline tasks to Redis (asynq) for ai-engine workers
//
// This process MUST NOT call LLM/image/TTS/FFmpeg directly.
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

	"github.com/novel2av/backend/internal/config"
	"github.com/novel2av/backend/internal/httpapi"
	"github.com/novel2av/backend/internal/infra/db"
	"github.com/novel2av/backend/internal/infra/observability"
	"github.com/novel2av/backend/internal/infra/queue"
	"github.com/novel2av/backend/internal/infra/storage"
	"github.com/novel2av/backend/internal/service"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}
	if err := httpapi.EnsureInternalSecurity(cfg.AppEnv); err != nil {
		slog.Error("internal security check", "err", err)
		os.Exit(1)
	}
	observability.Setup(cfg.LogLevel)
	// Allocate the Prometheus registry up-front so /metrics is reachable
	// from the first scrape. SetupMetrics is idempotent.
	observability.SetupMetrics()
	service.EnsureSvcMetrics()

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DBURL)
	if err != nil {
		slog.Error("db connect", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	mc, err := storage.NewMinIO(ctx, cfg.S3)
	if err != nil {
		slog.Error("minio connect", "err", err)
		os.Exit(1)
	}

	qc, bus, err := queue.NewAsynqClient(cfg.RedisAIURL, cfg.RedisURL)
	if err != nil {
		slog.Error("queue connect", "err", err)
		os.Exit(1)
	}
	defer bus.Close()

	svcs := service.New(pool, mc, qc, bus)
	router := httpapi.NewRouter(svcs)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancelShutdown()
		_ = srv.Shutdown(shutdownCtx)
	}()

	slog.Info("api listening", "addr", cfg.HTTPAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}
}
