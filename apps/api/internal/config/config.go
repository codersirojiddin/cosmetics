// Package config loads runtime configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env         string // development | production | test
	Port        string
	DatabaseURL string // pooled URL used by the app
	DirectURL   string // direct URL used for migrations (falls back to DatabaseURL)

	JWTSecret  string
	AccessTTL  time.Duration
	RefreshTTL time.Duration

	TelegramToken         string
	BotMode               string // polling | webhook | off
	BotUsername           string // optional fallback when getMe is unreachable
	TelegramWebhookSecret string

	PublicAPIURL string // public base URL of this API (used for webhook + local file URLs)
	WebURL       string // public base URL of the website (admin links in bot messages)
	CORSOrigins  []string

	OwnerTelegramIDs []int64

	StorageDriver string // local | s3
	UploadsDir    string
	S3            S3Config

	MaxUploadBytes int64
}

type S3Config struct {
	Endpoint      string
	Region        string
	AccessKey     string
	SecretKey     string
	PublicBucket  string
	PrivateBucket string
	PublicBaseURL string // public domain of the public bucket (CDN / r2.dev)
}

func (c *Config) IsProd() bool { return c.Env == "production" }

func Load() (*Config, error) {
	c := &Config{
		Env:                   get("APP_ENV", "development"),
		Port:                  get("PORT", "8080"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		DirectURL:             os.Getenv("DIRECT_URL"),
		JWTSecret:             os.Getenv("JWT_SECRET"),
		AccessTTL:             getDur("ACCESS_TTL", 15*time.Minute),
		RefreshTTL:            getDur("REFRESH_TTL", 30*24*time.Hour),
		TelegramToken:         os.Getenv("TELEGRAM_BOT_TOKEN"),
		BotUsername:           strings.TrimPrefix(os.Getenv("BOT_USERNAME"), "@"),
		TelegramWebhookSecret: os.Getenv("TELEGRAM_WEBHOOK_SECRET"),
		PublicAPIURL:          strings.TrimRight(get("PUBLIC_API_URL", ""), "/"),
		WebURL:                strings.TrimRight(get("WEB_URL", ""), "/"),
		StorageDriver:         get("STORAGE_DRIVER", "local"),
		UploadsDir:            get("UPLOADS_DIR", "./uploads"),
		MaxUploadBytes:        getInt64("MAX_UPLOAD_BYTES", 10<<20),
		S3: S3Config{
			Endpoint:      os.Getenv("S3_ENDPOINT"),
			Region:        get("S3_REGION", "auto"),
			AccessKey:     os.Getenv("S3_ACCESS_KEY"),
			SecretKey:     os.Getenv("S3_SECRET_KEY"),
			PublicBucket:  os.Getenv("S3_BUCKET_PUBLIC"),
			PrivateBucket: os.Getenv("S3_BUCKET_PRIVATE"),
			PublicBaseURL: strings.TrimRight(os.Getenv("S3_PUBLIC_BASE_URL"), "/"),
		},
	}
	if c.DirectURL == "" {
		c.DirectURL = c.DatabaseURL
	}
	c.BotMode = get("BOT_MODE", "")
	if c.BotMode == "" {
		if c.TelegramToken == "" {
			c.BotMode = "off"
		} else {
			c.BotMode = "polling"
		}
	}
	for _, o := range strings.Split(os.Getenv("CORS_ORIGINS"), ",") {
		if o = strings.TrimSpace(o); o != "" {
			c.CORSOrigins = append(c.CORSOrigins, strings.TrimRight(o, "/"))
		}
	}
	for _, s := range strings.Split(os.Getenv("OWNER_TELEGRAM_IDS"), ",") {
		if s = strings.TrimSpace(s); s != "" {
			id, err := strconv.ParseInt(s, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("OWNER_TELEGRAM_IDS: %q is not a number", s)
			}
			c.OwnerTelegramIDs = append(c.OwnerTelegramIDs, id)
		}
	}
	return c, c.Validate()
}

func (c *Config) Validate() error {
	var errs []string
	if c.DatabaseURL == "" {
		errs = append(errs, "DATABASE_URL is required")
	}
	if len(c.JWTSecret) < 32 {
		errs = append(errs, "JWT_SECRET must be at least 32 characters")
	}
	if c.BotMode != "off" && c.TelegramToken == "" {
		errs = append(errs, "TELEGRAM_BOT_TOKEN is required unless BOT_MODE=off")
	}
	if c.BotMode == "webhook" && (c.PublicAPIURL == "" || c.TelegramWebhookSecret == "") {
		errs = append(errs, "BOT_MODE=webhook needs PUBLIC_API_URL and TELEGRAM_WEBHOOK_SECRET")
	}
	if c.StorageDriver == "s3" && (c.S3.Endpoint == "" || c.S3.PublicBucket == "" || c.S3.PrivateBucket == "" || c.S3.PublicBaseURL == "") {
		errs = append(errs, "STORAGE_DRIVER=s3 needs S3_ENDPOINT, S3_BUCKET_PUBLIC, S3_BUCKET_PRIVATE, S3_PUBLIC_BASE_URL")
	}
	if c.IsProd() && len(c.CORSOrigins) == 0 {
		errs = append(errs, "CORS_ORIGINS is required in production")
	}
	if len(errs) > 0 {
		return fmt.Errorf("invalid configuration:\n - %s", strings.Join(errs, "\n - "))
	}
	return nil
}

func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func getDur(k string, def time.Duration) time.Duration {
	if v := os.Getenv(k); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getInt64(k string, def int64) int64 {
	if v := os.Getenv(k); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			return n
		}
	}
	return def
}
