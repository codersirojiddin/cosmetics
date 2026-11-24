package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/app"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/config"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/db"
)

func main() {
	_ = godotenv.Load() // optional .env for local development
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	logger := app.NewLogger(cfg)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if os.Getenv("AUTO_MIGRATE") == "true" {
		if err := db.Migrate(ctx, cfg.DirectURL); err != nil {
			log.Fatalf("migrate: %v", err)
		}
		logger.Info("migrations applied")
	}
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	a, err := app.New(cfg, pool, app.Options{Log: logger})
	if err != nil {
		log.Fatalf("app: %v", err)
	}
	a.StartBot(ctx)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           a.Handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		logger.Info("api listening", "addr", srv.Addr, "env", cfg.Env, "bot", cfg.BotMode, "storage", cfg.StorageDriver)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()
	<-ctx.Done()
	logger.Info("shutting down")
	sctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = srv.Shutdown(sctx)
}
