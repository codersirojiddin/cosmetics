// Package db opens the connection pool and runs migrations.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/codersirojiddin/cosmetics/apps/api/migrations"
)

// Connect opens a pgx pool. channel_binding is not supported by pgx and is stripped
// (Neon's default connection string includes it).
func Connect(ctx context.Context, url string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(sanitizeURL(url))
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnIdleTime = 5 * time.Minute
	cfg.MaxConnLifetime = 30 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	pctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := pool.Ping(pctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return pool, nil
}

// Migrate applies all pending migrations using the given (preferably direct, non-pooled) URL.
func Migrate(ctx context.Context, url string) error {
	cfg, err := pgxpool.ParseConfig(sanitizeURL(url))
	if err != nil {
		return err
	}
	sqlDB := stdlib.OpenDB(*cfg.ConnConfig)
	defer sqlDB.Close()
	return MigrateDB(ctx, sqlDB)
}

func MigrateDB(ctx context.Context, sqlDB *sql.DB) error {
	goose.SetBaseFS(migrations.FS)
	goose.SetLogger(goose.NopLogger())
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}
	return goose.UpContext(ctx, sqlDB, ".")
}

// Reset drops everything in the public schema. Test helper — never call in production.
func Reset(ctx context.Context, url string) error {
	cfg, err := pgxpool.ParseConfig(sanitizeURL(url))
	if err != nil {
		return err
	}
	sqlDB := stdlib.OpenDB(*cfg.ConnConfig)
	defer sqlDB.Close()
	_, err = sqlDB.ExecContext(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`)
	return err
}

func sanitizeURL(u string) string {
	i := strings.Index(u, "?")
	if i < 0 {
		return u
	}
	var keep []string
	for _, kv := range strings.Split(u[i+1:], "&") {
		if strings.HasPrefix(kv, "channel_binding=") {
			continue
		}
		keep = append(keep, kv)
	}
	if len(keep) == 0 {
		return u[:i]
	}
	return u[:i] + "?" + strings.Join(keep, "&")
}
