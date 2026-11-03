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

	"homebase/internal/api"
	"homebase/internal/config"
	"homebase/internal/store"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	cfg := config.Load()
	if err := cfg.ApplyTimezone(); err != nil {
		logger.Error("load application timezone", "timezone", cfg.Timezone, "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect database", "error", err)
		os.Exit(1)
	}
	defer func() { _ = st.Close() }()

	if err := st.Migrate(ctx); err != nil {
		logger.Error("migrate database", "error", err)
		os.Exit(1)
	}
	if cfg.BootstrapOwnerEmail != "" {
		if err := st.EnsureBootstrapOwner(ctx, cfg.BootstrapOwnerEmail, cfg.BootstrapOwnerName, cfg.BootstrapHousehold); err != nil {
			logger.Error("bootstrap owner", "error", err)
			os.Exit(1)
		}
	}
	apiServer := api.New(cfg, st, logger)
	go runRoutineScheduler(ctx, logger, apiServer, cfg.RoutineCheckInterval)
	go runReminderLoop(ctx, logger, apiServer, time.Minute)

	server := &http.Server{
		Addr:              cfg.APIAddr,
		Handler:           apiServer,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("api listening", "addr", cfg.APIAddr, "timezone", cfg.Timezone)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("api server", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("api shutdown", "error", err)
	}
}

func runRoutineScheduler(ctx context.Context, logger *slog.Logger, apiServer *api.Server, interval time.Duration) {
	if interval <= 0 {
		interval = 15 * time.Minute
	}

	run := func() {
		jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		if err := apiServer.GenerateDueTasks(jobCtx); err != nil {
			logger.Error("generate due tasks", "error", err)
		}
	}

	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

// runReminderLoop sends due Reminders every interval. It is separate from the
// Routine scheduler so that loop keeps its own cadence.
func runReminderLoop(ctx context.Context, logger *slog.Logger, server *api.Server, interval time.Duration) {
	run := func() {
		jobCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		sent, err := server.SendDueReminders(jobCtx, time.Now())
		if err != nil {
			logger.Error("send due reminders", "error", err)
		}
		if sent > 0 {
			logger.Info("sent due reminders", "count", sent)
		}
	}

	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
