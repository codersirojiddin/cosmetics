// Package app assembles the store, services, bot and HTTP handler. Used by main and by tests.
package app

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/api"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/bot"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/config"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/service"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/storage"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/store"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/tg"
)

type Options struct {
	TG         tg.API          // overrides the real Telegram client (tests)
	Storage    storage.Storage // overrides the configured storage (tests)
	NotifySync bool
	Log        *slog.Logger
}

type App struct {
	Handler  http.Handler
	Services *service.Services
	Store    *store.Store
	Bot      *bot.Bot // nil when the bot is disabled
	cfg      *config.Config
}

func NewLogger(cfg *config.Config) *slog.Logger {
	if cfg.IsProd() {
		return slog.New(slog.NewJSONHandler(os.Stdout, nil))
	}
	return slog.New(slog.NewTextHandler(os.Stdout, nil))
}

func New(cfg *config.Config, pool *pgxpool.Pool, o Options) (*App, error) {
	log := o.Log
	if log == nil {
		log = NewLogger(cfg)
	}
	st := store.New(pool)

	var localDir string
	stor := o.Storage
	if stor == nil {
		switch cfg.StorageDriver {
		case "s3":
			s3, err := storage.NewS3(cfg.S3)
			if err != nil {
				return nil, err
			}
			stor = s3
		default:
			base := cfg.PublicAPIURL
			if base == "" {
				base = "http://localhost:" + cfg.Port
			}
			l, err := storage.NewLocal(cfg.UploadsDir, base+"/files")
			if err != nil {
				return nil, err
			}
			stor, localDir = l, l.PublicDir()
		}
	} else if l, ok := stor.(*storage.Local); ok {
		localDir = l.PublicDir()
	}

	tgAPI := o.TG
	if tgAPI == nil && cfg.BotMode != "off" && cfg.TelegramToken != "" {
		tgAPI = tg.NewClient(cfg.TelegramToken)
	}

	svc := service.New(service.Deps{Cfg: cfg, Store: st, Storage: stor, TG: tgAPI, Log: log, NotifySync: o.NotifySync})
	a := &App{Services: svc, Store: st, cfg: cfg}

	var webhook http.Handler
	if tgAPI != nil {
		a.Bot = bot.New(tgAPI, svc, cfg, log)
		if cfg.BotMode == "webhook" {
			webhook = a.Bot.WebhookHandler()
		}
	}
	a.Handler = api.New(cfg, svc, log, api.Options{Ping: pool.Ping, LocalDir: localDir, Webhook: webhook})
	return a, nil
}

// StartBot resolves the bot identity and, in polling mode, starts the update loop.
func (a *App) StartBot(ctx context.Context) {
	if a.Bot == nil {
		return
	}
	a.Bot.Init(ctx)
	if a.cfg.BotMode == "polling" {
		go a.Bot.Poll(ctx)
	}
}
