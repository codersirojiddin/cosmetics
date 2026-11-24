// Command devtoken signs a (fake) Telegram user in for local development and prints JSON tokens.
// It exists so the UI can be tested without Telegram. It refuses to run when APP_ENV=production.
//
//	go run ./cmd/devtoken -tg 3001 -name Ali -role customer
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/app"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/config"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/db"
)

func main() {
	tgID := flag.Int64("tg", 3001, "fake Telegram user id")
	name := flag.String("name", "Dev User", "first name")
	role := flag.String("role", "customer", "customer | admin | owner")
	flag.Parse()

	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if cfg.IsProd() {
		log.Fatal("refusing to mint tokens in production")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	a, err := app.New(cfg, pool, app.Options{})
	if err != nil {
		log.Fatal(err)
	}
	u, err := a.Store.UpsertTelegramUser(ctx, *tgID, *name, "", "", *role == "owner")
	if err != nil {
		log.Fatal(err)
	}
	if *role == "admin" {
		if err := a.Store.SetUserRole(ctx, u.ID, "admin"); err != nil {
			log.Fatal(err)
		}
	}
	tok, err := a.Services.Auth.IssueTokens(ctx, u.ID)
	if err != nil {
		log.Fatal(err)
	}
	out, _ := json.Marshal(map[string]any{"user": u, "tokens": tok})
	fmt.Fprintln(os.Stdout, string(out))
}
