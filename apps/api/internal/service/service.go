// Package service holds the business logic. HTTP handlers and the Telegram bot are thin
// layers on top of it.
package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/config"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/storage"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/store"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/tg"
)

type Deps struct {
	Cfg     *config.Config
	Store   *store.Store
	Storage storage.Storage
	TG      tg.API // nil disables Telegram notifications
	Log     *slog.Logger
	// NotifySync makes notifications run inline instead of in a goroutine (tests).
	NotifySync bool
}

type Services struct {
	d           *Deps
	botUsername atomic.Value // string

	Auth     *Auth
	Admins   *Admins
	Settings *Settings
	Files    *Files
	Catalog  *Catalog
	Orders   *Orders
	Notifier *Notifier
}

func New(d Deps) *Services {
	if d.Log == nil {
		d.Log = slog.Default()
	}
	s := &Services{d: &d}
	s.botUsername.Store(d.Cfg.BotUsername)
	s.Auth = &Auth{s}
	s.Admins = &Admins{s}
	s.Settings = &Settings{s}
	s.Files = &Files{s}
	s.Catalog = &Catalog{s}
	s.Orders = &Orders{s}
	s.Notifier = &Notifier{s}
	return s
}

// SetBotUsername records the bot's @username (learned via getMe) for deep links.
func (s *Services) SetBotUsername(u string) { s.botUsername.Store(strings.TrimPrefix(u, "@")) }
func (s *Services) BotUsername() string     { v, _ := s.botUsername.Load().(string); return v }

func (s *Services) audit(ctx contextLike, actor *store.User, action, entity, id string, meta map[string]any) {
	var aid *string
	if actor != nil {
		aid = &actor.ID
	}
	if err := s.d.Store.InsertAudit(ctx, aid, action, entity, id, meta); err != nil {
		s.d.Log.Error("audit log failed", "action", action, "err", err)
	}
}

// ---------- paging ----------

type Page[T any] struct {
	Items []T `json:"items"`
	Page  int `json:"page"`
	Limit int `json:"limit"`
	Total int `json:"total"`
}

// Paging normalises page/limit query values.
func Paging(page, limit, def, max int) (p, l, offset int) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = def
	}
	if limit > max {
		limit = max
	}
	return page, limit, (page - 1) * limit
}

// ---------- small helpers ----------

func randToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

var translit = strings.NewReplacer(
	"а", "a", "б", "b", "в", "v", "г", "g", "д", "d", "е", "e", "ё", "yo", "ж", "j", "з", "z", "и", "i", "й", "y",
	"к", "k", "л", "l", "м", "m", "н", "n", "о", "o", "п", "p", "р", "r", "с", "s", "т", "t", "у", "u", "ф", "f",
	"х", "x", "ц", "ts", "ч", "ch", "ш", "sh", "щ", "sh", "ъ", "", "ы", "i", "ь", "", "э", "e", "ю", "yu", "я", "ya",
	"ў", "o", "ғ", "g", "қ", "q", "ҳ", "h",
	"ʻ", "", "ʼ", "", "‘", "", "’", "", "'", "", "`", "",
)

func slugify(s string) string {
	s = nonSlug.ReplaceAllString(translit.Replace(strings.ToLower(s)), "-")
	s = strings.Trim(s, "-")
	if len(s) > 80 {
		s = strings.Trim(s[:80], "-")
	}
	if s == "" {
		s = "item"
	}
	return s
}

// normalizePhone turns common Uzbek phone formats into +998XXXXXXXXX.
func normalizePhone(s string) (string, bool) {
	var d strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			d.WriteRune(r)
		}
	}
	digits := d.String()
	switch {
	case len(digits) == 9:
		digits = "998" + digits
	case len(digits) == 10 && strings.HasPrefix(digits, "0"):
		digits = "998" + digits[1:]
	case len(digits) == 12 && strings.HasPrefix(digits, "998"):
	default:
		return "", false
	}
	return "+" + digits, true
}

var (
	htmlTag  = regexp.MustCompile(`(?is)<\s*(script|style)[^>]*>.*?<\s*/\s*(script|style)\s*>`)
	htmlAny  = regexp.MustCompile(`</?[a-zA-Z][^>]*>`)
	badLinks = regexp.MustCompile(`(?i)\]\(\s*(javascript|data|vbscript):`)
)

// cleanMarkdown strips raw HTML and script-ish link targets. Descriptions are stored as Markdown;
// clients must still render without enabling raw HTML.
func cleanMarkdown(s string) string {
	s = htmlTag.ReplaceAllString(s, "")
	s = htmlAny.ReplaceAllString(s, "")
	s = badLinks.ReplaceAllString(s, "](")
	return strings.TrimSpace(s)
}

// formatMoney renders 85000 as "85 000".
func formatMoney(n int64) string {
	s := strconv.FormatInt(n, 10)
	var out []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			out = append(out, ' ')
		}
		out = append(out, c)
	}
	return string(out)
}

// AuditList returns the latest audit log entries (owner only; enforced by the router).
func (s *Services) AuditList(ctx contextLike, limit, offset int) ([]store.AuditRow, error) {
	rows, err := s.d.Store.ListAudit(ctx, limit, offset)
	if rows == nil {
		rows = []store.AuditRow{}
	}
	return rows, err
}
