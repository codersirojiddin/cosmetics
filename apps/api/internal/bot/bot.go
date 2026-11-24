// Package bot wires Telegram updates (login, admin invites, "Men olaman" buttons) to the services.
package bot

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/apperr"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/config"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/service"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/tg"
)

type Bot struct {
	api tg.API
	svc *service.Services
	cfg *config.Config
	log *slog.Logger
}

func New(api tg.API, svc *service.Services, cfg *config.Config, log *slog.Logger) *Bot {
	return &Bot{api: api, svc: svc, cfg: cfg, log: log}
}

// Init learns the bot username (needed for deep links) and prepares the delivery mode.
func (b *Bot) Init(ctx context.Context) {
	if me, err := b.api.GetMe(ctx); err != nil {
		b.log.Warn("telegram getMe failed (set BOT_USERNAME as a fallback)", "err", err)
	} else {
		b.svc.SetBotUsername(me.Username)
		b.log.Info("telegram bot ready", "username", me.Username)
	}
	switch b.cfg.BotMode {
	case "webhook":
		if err := b.api.SetWebhook(ctx, b.cfg.PublicAPIURL+"/telegram/webhook", b.cfg.TelegramWebhookSecret); err != nil {
			b.log.Error("setWebhook failed", "err", err)
		}
	case "polling":
		if err := b.api.DeleteWebhook(ctx); err != nil {
			b.log.Warn("deleteWebhook failed", "err", err)
		}
	}
}

// Poll long-polls Telegram until ctx is cancelled (development mode).
func (b *Bot) Poll(ctx context.Context) {
	var offset int64
	for ctx.Err() == nil {
		ups, err := b.api.GetUpdates(ctx, offset, 30)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			b.log.Warn("getUpdates failed", "err", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(3 * time.Second):
			}
			continue
		}
		for _, u := range ups {
			offset = u.UpdateID + 1
			b.safeHandle(ctx, u)
		}
	}
}

// WebhookHandler receives updates from Telegram (production mode).
func (b *Bot) WebhookHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Telegram-Bot-Api-Secret-Token")
		if subtle.ConstantTimeCompare([]byte(got), []byte(b.cfg.TelegramWebhookSecret)) != 1 || b.cfg.TelegramWebhookSecret == "" {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		var u tg.Update
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&u); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		go b.safeHandle(context.Background(), u) // answer fast, Telegram retries slow webhooks
		w.WriteHeader(http.StatusOK)
	})
}

func (b *Bot) safeHandle(ctx context.Context, u tg.Update) {
	defer func() {
		if r := recover(); r != nil {
			b.log.Error("bot handler panic", "panic", r)
		}
	}()
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	b.HandleUpdate(ctx, u)
}

func (b *Bot) HandleUpdate(ctx context.Context, u tg.Update) {
	switch {
	case u.Message != nil && u.Message.From != nil && u.Message.Chat.Type == "private":
		b.onMessage(ctx, u.Message)
	case u.CallbackQuery != nil:
		b.onCallback(ctx, u.CallbackQuery)
	}
}

func (b *Bot) reply(ctx context.Context, chatID int64, text string) {
	if _, err := b.api.SendMessage(ctx, chatID, text, nil); err != nil {
		b.log.Warn("bot reply failed", "err", err)
	}
}

// failText turns an error into a user-facing message (internal errors stay generic).
func (b *Bot) failText(err error) string {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		return "⚠️ " + ae.Message
	}
	b.log.Error("bot action failed", "err", err)
	return "⚠️ Xatolik yuz berdi, keyinroq qayta urinib ko'ring."
}

func (b *Bot) onMessage(ctx context.Context, m *tg.Message) {
	fields := strings.Fields(strings.TrimSpace(m.Text))
	if len(fields) == 0 {
		return
	}
	cmd := fields[0]
	if i := strings.Index(cmd, "@"); i > 0 {
		cmd = cmd[:i]
	}
	switch cmd {
	case "/id":
		b.reply(ctx, m.Chat.ID, fmt.Sprintf("Sizning Telegram ID: <code>%d</code>", m.From.ID))
	case "/start":
		payload := ""
		if len(fields) > 1 {
			payload = fields[1]
		}
		switch {
		case strings.HasPrefix(payload, "login_"):
			b.onLogin(ctx, m, strings.TrimPrefix(payload, "login_"))
		case strings.HasPrefix(payload, "admin_"):
			b.onAdminInvite(ctx, m, strings.TrimPrefix(payload, "admin_"))
		default:
			text := "Salom! 👋 Buyurtma berish uchun do'kon saytidan foydalaning."
			if b.cfg.WebURL != "" {
				text += "\n\n" + b.cfg.WebURL
			}
			b.reply(ctx, m.Chat.ID, text)
		}
	default:
		b.reply(ctx, m.Chat.ID, "Buyurtma berish uchun do'kon saytidan foydalaning.")
	}
}

func (b *Bot) onLogin(ctx context.Context, m *tg.Message, code string) {
	if _, err := b.svc.Auth.ConfirmLogin(ctx, code, *m.From); err != nil {
		b.reply(ctx, m.Chat.ID, b.failText(err)+"\nSaytda «Telegram orqali kirish» tugmasini qaytadan bosing.")
		return
	}
	b.reply(ctx, m.Chat.ID, "✅ <b>Tasdiqlandi!</b>\n\nSaytga qayting, avtomatik kirasiz.")
}

func (b *Bot) onAdminInvite(ctx context.Context, m *tg.Message, code string) {
	if _, err := b.svc.Admins.AcceptInvite(ctx, code, *m.From); err != nil {
		b.reply(ctx, m.Chat.ID, b.failText(err))
		return
	}
	b.reply(ctx, m.Chat.ID, "🎉 <b>Siz admin bo'ldingiz!</b>\n\nSaytga kirib admin panelni oching. Yangi buyurtmalar haqida shu yerga xabar keladi.")
}

func (b *Bot) onCallback(ctx context.Context, cb *tg.CallbackQuery) {
	answer := func(text string, alert bool) {
		if err := b.api.AnswerCallback(ctx, cb.ID, text, alert); err != nil {
			b.log.Warn("answerCallback failed", "err", err)
		}
	}
	orderID, ok := strings.CutPrefix(cb.Data, "claim:")
	if !ok {
		answer("", false)
		return
	}
	admin, err := b.svc.Auth.StaffByTelegramID(ctx, cb.From.ID)
	if err != nil {
		answer("Bu amal faqat adminlar uchun", true)
		return
	}
	if _, err := b.svc.Orders.Claim(ctx, admin, orderID, false); err != nil {
		var ae *apperr.Error
		if errors.As(err, &ae) {
			answer(ae.Message, true)
		} else {
			b.log.Error("claim via bot failed", "err", err)
			answer("Xatolik yuz berdi", true)
		}
		return
	}
	answer("✅ Buyurtma sizga biriktirildi", false)
}
