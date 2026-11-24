package app_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"log/slog"
	"math/rand"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/app"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/config"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/db"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/tg"
)

var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		url = "postgres://cosmetics:cosmetics@127.0.0.1:5432/cosmetics_test?sslmode=disable"
	}
	ctx := context.Background()
	// The suite drops the whole schema: never let it near anything but a throwaway "*_test" database.
	if !strings.Contains(strings.SplitN(url, "?", 2)[0], "_test") {
		fmt.Fprintln(os.Stderr, "refusing to run: TEST_DATABASE_URL must point to a database whose name contains _test")
		os.Exit(1)
	}
	if err := db.Reset(ctx, url); err != nil {
		fmt.Fprintln(os.Stderr, "reset test db:", err)
		os.Exit(1)
	}
	if err := db.Migrate(ctx, url); err != nil {
		fmt.Fprintln(os.Stderr, "migrate test db:", err)
		os.Exit(1)
	}
	var err error
	if testPool, err = db.Connect(ctx, url); err != nil {
		fmt.Fprintln(os.Stderr, "connect test db:", err)
		os.Exit(1)
	}
	code := m.Run()
	testPool.Close()
	os.Exit(code)
}

// ---------- fake Telegram ----------

type sentMsg struct {
	chat int64
	id   int64
	text string
	kb   tg.Keyboard
}

type fakeTG struct {
	mu      sync.Mutex
	seq     int64
	sent    []sentMsg
	edits   []sentMsg
	answers []string
}

func (f *fakeTG) GetMe(context.Context) (*tg.User, error) {
	return &tg.User{ID: 1, Username: "testbot"}, nil
}
func (f *fakeTG) SendMessage(_ context.Context, chat int64, text string, o *tg.SendOpts) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.seq++
	m := sentMsg{chat: chat, id: f.seq, text: text}
	if o != nil {
		m.kb = o.Keyboard
	}
	f.sent = append(f.sent, m)
	return f.seq, nil
}
func (f *fakeTG) EditMessage(_ context.Context, chat, id int64, text string, o *tg.SendOpts) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := sentMsg{chat: chat, id: id, text: text}
	if o != nil {
		m.kb = o.Keyboard
	}
	f.edits = append(f.edits, m)
	return nil
}
func (f *fakeTG) AnswerCallback(_ context.Context, _ string, text string, alert bool) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answers = append(f.answers, fmt.Sprintf("%v|%s", alert, text))
	return nil
}
func (f *fakeTG) GetUpdates(context.Context, int64, int) ([]tg.Update, error) { return nil, nil }
func (f *fakeTG) SetWebhook(context.Context, string, string) error            { return nil }
func (f *fakeTG) DeleteWebhook(context.Context) error                         { return nil }

func (f *fakeTG) to(chat int64) []sentMsg {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []sentMsg
	for _, m := range f.sent {
		if m.chat == chat {
			out = append(out, m)
		}
	}
	return out
}

func (f *fakeTG) last(chat int64) string {
	m := f.to(chat)
	if len(m) == 0 {
		return ""
	}
	return m[len(m)-1].text
}

func (f *fakeTG) editsFor(chat int64) []sentMsg {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []sentMsg
	for _, m := range f.edits {
		if m.chat == chat {
			out = append(out, m)
		}
	}
	return out
}

// ---------- environment ----------

type J = map[string]any

type env struct {
	t   *testing.T
	app *app.App
	tg  *fakeTG
}

const ownerTG = 1001

func newEnv(t *testing.T) *env {
	t.Helper()
	_, err := testPool.Exec(context.Background(), `TRUNCATE users, login_requests, refresh_tokens, admin_invites, files, categories, products,
		product_variants, product_images, orders, order_items, order_status_history, order_notes, order_admin_messages, settings, audit_log
		RESTART IDENTITY CASCADE`)
	require.NoError(t, err)
	cfg := &config.Config{Env: "test", JWTSecret: strings.Repeat("s", 40), AccessTTL: 15 * time.Minute, RefreshTTL: 24 * time.Hour,
		BotMode: "off", BotUsername: "testbot", OwnerTelegramIDs: []int64{ownerTG}, StorageDriver: "local", UploadsDir: t.TempDir(),
		MaxUploadBytes: 5 << 20, PublicAPIURL: "http://test", WebURL: "https://shop.example"}
	fake := &fakeTG{}
	a, err := app.New(cfg, testPool, app.Options{TG: fake, NotifySync: true, Log: slog.New(slog.NewTextHandler(io.Discard, nil))})
	require.NoError(t, err)
	return &env{t: t, app: a, tg: fake}
}

type actor struct {
	id    string
	tg    int64
	token string
}

// user creates a user directly in the database and signs them in. role: customer|admin|owner.
func (e *env) user(tgID int64, name, role string) actor {
	e.t.Helper()
	ctx := context.Background()
	u, err := e.app.Store.UpsertTelegramUser(ctx, tgID, name, "Test", strings.ToLower(name), role == "owner")
	require.NoError(e.t, err)
	if role == "admin" {
		require.NoError(e.t, e.app.Store.SetUserRole(ctx, u.ID, "admin"))
	}
	tok, err := e.app.Services.Auth.IssueTokens(ctx, u.ID)
	require.NoError(e.t, err)
	return actor{id: u.ID, tg: tgID, token: tok.AccessToken}
}

func (e *env) do(method, path, token string, body any) (int, J) {
	e.t.Helper()
	var rd io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(e.t, err)
		rd = bytes.NewReader(b)
	}
	req := httptest.NewRequest(method, "/api/v1"+path, rd)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.app.Handler.ServeHTTP(rec, req)
	var out J
	if rec.Body.Len() > 0 {
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
	}
	return rec.Code, out
}

func (e *env) upload(path, token string, data []byte) (int, J) {
	e.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "upload.png")
	require.NoError(e.t, err)
	_, _ = fw.Write(data)
	require.NoError(e.t, mw.Close())
	req := httptest.NewRequest(http.MethodPost, "/api/v1"+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.app.Handler.ServeHTTP(rec, req)
	var out J
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func (e *env) raw(path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	e.app.Handler.ServeHTTP(rec, req)
	return rec
}

// botMessage simulates a private Telegram message to the bot.
func (e *env) botMessage(from tg.User, text string) {
	e.app.Bot.HandleUpdate(context.Background(), tg.Update{Message: &tg.Message{From: &from, Chat: tg.Chat{ID: from.ID, Type: "private"}, Text: text}})
}

// ---------- json path helpers ----------

func at(v any, keys ...any) any {
	for _, k := range keys {
		switch kk := k.(type) {
		case string:
			m, ok := v.(map[string]any)
			if !ok {
				return nil
			}
			v = m[kk]
		case int:
			a, ok := v.([]any)
			if !ok || kk >= len(a) {
				return nil
			}
			v = a[kk]
		}
	}
	return v
}
func str(v any, keys ...any) string  { s, _ := at(v, keys...).(string); return s }
func num(v any, keys ...any) float64 { n, _ := at(v, keys...).(float64); return n }
func arr(v any, keys ...any) []any   { a, _ := at(v, keys...).([]any); return a }

// ---------- test images ----------

// blocks renders a 9x8 grid of random gray cells; the same seed gives the same picture at any cell size.
func blocks(seed int64, cw, ch int) []byte {
	rng := rand.New(rand.NewSource(seed))
	img := image.NewGray(image.Rect(0, 0, 9*cw, 8*ch))
	for by := 0; by < 8; by++ {
		for bx := 0; bx < 9; bx++ {
			g := color.Gray{Y: uint8(rng.Intn(256))}
			for y := by * ch; y < (by+1)*ch; y++ {
				for x := bx * cw; x < (bx+1)*cw; x++ {
					img.SetGray(x, y, g)
				}
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func pic(seed int64) []byte { return blocks(seed, 48, 48) }

// ---------- fixtures ----------

type shop struct {
	owner, admin, admin2, cust, cust2 actor
	category, product                 string
	varA, varB                        string // 30 ml (85 000, stock 5) and 50 ml (90 000, stock 2)
	image                             string
}

func (e *env) productBody(category string, images []string, status string, variants ...J) J {
	if len(variants) == 0 {
		variants = []J{{"label": "30 ml", "price": 85000, "stock": 5, "sku": "SER-30"}, {"label": "50 ml", "price": 90000, "stock": 2, "sku": "SER-50"}}
	}
	return J{"categoryId": category, "name": "Hyaluron Serum", "brand": "Lumi", "description": "Namlantiruvchi **serum**",
		"optionLabel": "Hajm", "status": status, "imageIds": images, "variants": variants}
}

func (e *env) setupShop() shop {
	e.t.Helper()
	s := shop{
		owner: e.user(ownerTG, "Owner", "owner"), admin: e.user(2001, "Ahmad", "admin"), admin2: e.user(2002, "Bobur", "admin"),
		cust: e.user(3001, "Ali", "customer"), cust2: e.user(3002, "Vali", "customer"),
	}
	st, r := e.do("POST", "/admin/categories", s.admin.token, J{"name": "Yuz parvarishi"})
	require.Equal(e.t, 201, st, r)
	s.category = str(r, "id")
	st, r = e.upload("/admin/uploads/image", s.admin.token, pic(900))
	require.Equal(e.t, 201, st, r)
	s.image = str(r, "id")
	st, r = e.do("POST", "/admin/products", s.admin.token, e.productBody(s.category, []string{s.image}, "active"))
	require.Equal(e.t, 201, st, r)
	s.product = str(r, "id")
	s.varA, s.varB = str(r, "variants", 0, "id"), str(r, "variants", 1, "id")
	return s
}

// receipt uploads a payment receipt as the given customer and returns its file id.
func (e *env) receipt(c actor, seed int64) string {
	e.t.Helper()
	st, r := e.upload("/uploads/receipt", c.token, pic(seed))
	require.Equal(e.t, 201, st, r)
	return str(r, "id")
}

func (e *env) orderBody(receipt string, items ...J) J {
	return J{"items": items, "customerName": "Ali Valiyev", "phone": "90 123 45 67", "addressText": "Toshkent, Chilonzor 5-kvartal",
		"lat": 41.31, "lng": 69.24, "receiptFileId": receipt}
}

func item(variant string, qty int) J { return J{"variantId": variant, "quantity": qty} }
