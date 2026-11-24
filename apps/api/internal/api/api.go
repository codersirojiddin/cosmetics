// Package api is the HTTP layer: routing, middleware, request binding and handlers.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/go-playground/validator/v10"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/apperr"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/config"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/service"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/store"
)

type API struct {
	cfg      *config.Config
	svc      *service.Services
	log      *slog.Logger
	validate *validator.Validate
	ping     func(context.Context) error
	localDir string       // set when files are served from local disk
	webhook  http.Handler // Telegram webhook (nil unless BOT_MODE=webhook)
}

type Options struct {
	Ping     func(context.Context) error
	LocalDir string
	Webhook  http.Handler
}

type ctxKey struct{}

func userFrom(ctx context.Context) *store.User { u, _ := ctx.Value(ctxKey{}).(*store.User); return u }

func New(cfg *config.Config, svc *service.Services, log *slog.Logger, o Options) http.Handler {
	v := validator.New(validator.WithRequiredStructEnabled())
	v.RegisterTagNameFunc(func(f reflect.StructField) string {
		if n := strings.Split(f.Tag.Get("json"), ",")[0]; n != "" && n != "-" {
			return n
		}
		return f.Name
	})
	a := &API{cfg: cfg, svc: svc, log: log, validate: v, ping: o.Ping, localDir: o.LocalDir, webhook: o.Webhook}
	return a.routes()
}

// limit returns a per-IP rate limiter (disabled in tests).
func (a *API) limit(n int) func(http.Handler) http.Handler {
	if a.cfg.Env == "test" {
		return func(h http.Handler) http.Handler { return h }
	}
	return httprate.LimitByRealIP(n, time.Minute)
}

func (a *API) corsOrigins() []string {
	if len(a.cfg.CORSOrigins) > 0 {
		return a.cfg.CORSOrigins
	}
	return []string{"http://localhost:3000", "http://127.0.0.1:3000"}
}

func (a *API) routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, a.logRequests, middleware.Recoverer, secureHeaders)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: a.corsOrigins(),
		AllowedMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Authorization", "Content-Type"},
		MaxAge:         600,
	}))
	r.NotFound(func(w http.ResponseWriter, r *http.Request) { a.fail(w, r, apperr.NotFound("Manzil topilmadi")) })
	r.MethodNotAllowed(func(w http.ResponseWriter, r *http.Request) {
		a.fail(w, r, apperr.New(http.StatusMethodNotAllowed, "method_not_allowed", "Metod ruxsat etilmagan"))
	})
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "ok") })
	if a.localDir != "" {
		fs := http.StripPrefix("/files/", http.FileServer(noListFS{http.Dir(a.localDir)}))
		r.Get("/files/*", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			fs.ServeHTTP(w, r)
		})
	}
	if a.webhook != nil {
		r.Method(http.MethodPost, "/telegram/webhook", a.webhook)
	}

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(a.limit(300))
		r.Get("/health", a.health)
		r.Get("/settings/public", a.publicSettings)
		r.Get("/categories", a.listCategories)
		r.Get("/products", a.listProducts)
		r.Get("/products/{slug}", a.getProduct)

		r.Route("/auth", func(r chi.Router) {
			r.With(a.limit(20)).Post("/telegram/start", a.loginStart)
			r.With(a.limit(120)).Post("/telegram/poll", a.loginPoll)
			r.With(a.limit(30)).Post("/refresh", a.refresh)
			r.Post("/logout", a.logout)
		})

		r.Group(func(r chi.Router) {
			r.Use(a.requireAuth)
			r.Get("/me", a.getMe)
			r.Patch("/me", a.updateMe)
			r.Get("/payment-info", a.paymentInfo)
			r.With(a.limit(20)).Post("/uploads/receipt", a.uploadReceipt)
			r.With(a.limit(10)).Post("/orders", a.createOrder)
			r.Get("/orders", a.listMyOrders)
			r.Get("/orders/{id}", a.getMyOrder)
			r.Get("/files/receipts/{id}", a.getReceipt)

			r.Route("/admin", func(r chi.Router) {
				r.Use(a.requireStaff)
				r.Get("/stats", a.adminStats)
				r.Get("/orders", a.adminListOrders)
				r.Get("/orders/{id}", a.adminGetOrder)
				r.Post("/orders/{id}/claim", a.adminClaim)
				r.Post("/orders/{id}/status", a.adminChangeStatus)
				r.Post("/orders/{id}/notes", a.adminAddNote)

				r.Get("/categories", a.adminListCategories)
				r.Post("/categories", a.adminCreateCategory)
				r.Put("/categories/{id}", a.adminUpdateCategory)
				r.Delete("/categories/{id}", a.adminDeleteCategory)

				r.Get("/products", a.adminListProducts)
				r.Post("/products", a.adminCreateProduct)
				r.Get("/products/{id}", a.adminGetProduct)
				r.Put("/products/{id}", a.adminUpdateProduct)
				r.Delete("/products/{id}", a.adminArchiveProduct)
				r.With(a.limit(60)).Post("/uploads/image", a.adminUploadImage)

				r.Get("/settings", a.adminGetSettings)

				r.Group(func(r chi.Router) {
					r.Use(a.requireOwner)
					r.Put("/settings", a.adminUpdateSettings)
					r.Get("/admins", a.listAdmins)
					r.Post("/admins/invites", a.createInvite)
					r.Patch("/admins/{id}", a.updateAdmin)
					r.Delete("/admins/{id}", a.removeAdmin)
					r.Get("/audit", a.listAudit)
				})
			})
		})
	})
	return r
}

// ---------- middleware ----------

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func (a *API) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		if r.URL.Path == "/healthz" {
			return
		}
		a.log.Info("request", "method", r.Method, "path", r.URL.Path, "status", ww.Status(), "ms", time.Since(start).Milliseconds(),
			"id", middleware.GetReqID(r.Context()))
	})
}

func (a *API) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		if !ok || tok == "" {
			a.fail(w, r, apperr.Unauthorized("Tizimga kiring"))
			return
		}
		u, err := a.svc.Auth.Authenticate(r.Context(), tok)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), ctxKey{}, u)))
	})
}

func (a *API) requireStaff(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u := userFrom(r.Context()); u == nil || !u.IsStaff() {
			a.fail(w, r, apperr.Forbidden("Bu bo'lim faqat adminlar uchun"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *API) requireOwner(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if u := userFrom(r.Context()); u == nil || u.Role != "owner" {
			a.fail(w, r, apperr.Forbidden("Bu amal faqat owner uchun"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

func (a *API) fail(w http.ResponseWriter, r *http.Request, err error) {
	var ae *apperr.Error
	if errors.As(err, &ae) {
		writeJSON(w, ae.Status, map[string]any{"error": ae})
		return
	}
	if errors.Is(err, context.Canceled) {
		return // client went away
	}
	a.log.Error("request failed", "err", err, "method", r.Method, "path", r.URL.Path)
	writeJSON(w, http.StatusInternalServerError, map[string]any{"error": map[string]string{"code": "internal", "message": "Ichki xatolik. Keyinroq qayta urinib ko'ring"}})
}

var fieldMsg = map[string]string{
	"required": "Majburiy maydon", "min": "Qiymat juda kichik yoki qisqa", "max": "Qiymat juda katta yoki uzun",
	"gte": "Qiymat juda kichik", "lte": "Qiymat juda katta", "gt": "Qiymat noldan katta bo'lishi kerak",
	"uuid": "Noto'g'ri identifikator", "oneof": "Ruxsat etilmagan qiymat",
}

func (a *API) validationErr(err error) error {
	var ves validator.ValidationErrors
	if !errors.As(err, &ves) {
		return apperr.BadRequest("invalid_request", "So'rov noto'g'ri")
	}
	fields := map[string]string{}
	for _, fe := range ves {
		name := fe.Namespace()
		if i := strings.Index(name, "."); i >= 0 {
			name = name[i+1:]
		}
		msg := fieldMsg[fe.Tag()]
		if msg == "" {
			msg = "Noto'g'ri qiymat"
		}
		fields[name] = msg
	}
	return apperr.Validation(fields)
}

// bind decodes a JSON body into T (unknown fields rejected) and validates its struct tags.
func bind[T any](a *API, w http.ResponseWriter, r *http.Request) (*T, error) {
	var v T
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&v); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			return nil, apperr.TooLarge("So'rov hajmi juda katta")
		}
		return nil, apperr.BadRequest("invalid_json", "So'rov formati noto'g'ri")
	}
	if err := a.validate.Struct(&v); err != nil {
		return nil, a.validationErr(err)
	}
	return &v, nil
}

func qInt(r *http.Request, key string) int {
	n, _ := strconv.Atoi(r.URL.Query().Get(key))
	return n
}

func qBool(r *http.Request, key string) bool {
	v := r.URL.Query().Get(key)
	return v == "1" || v == "true"
}

// readFilePart reads the multipart field "file" without temp files, enforcing a size limit.
func readFilePart(w http.ResponseWriter, r *http.Request, max int64) ([]byte, error) {
	r.Body = http.MaxBytesReader(w, r.Body, max+(1<<20))
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, apperr.BadRequest("bad_multipart", "Fayl yuborish formati noto'g'ri (multipart/form-data, maydon nomi: file)")
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var mbe *http.MaxBytesError
			if errors.As(err, &mbe) {
				return nil, apperr.TooLarge("Fayl hajmi juda katta")
			}
			return nil, apperr.BadRequest("bad_multipart", "Faylni o'qib bo'lmadi")
		}
		if part.FormName() != "file" {
			continue
		}
		data, err := io.ReadAll(io.LimitReader(part, max+1))
		if err != nil {
			return nil, apperr.BadRequest("bad_multipart", "Faylni o'qib bo'lmadi")
		}
		if int64(len(data)) > max {
			return nil, apperr.TooLarge("Fayl hajmi juda katta")
		}
		return data, nil
	}
	return nil, apperr.Field("file", "Fayl yuklanmagan")
}

// noListFS hides directory listings when serving local uploads.
type noListFS struct{ fs http.FileSystem }

func (n noListFS) Open(name string) (http.File, error) {
	f, err := n.fs.Open(name)
	if err != nil {
		return nil, err
	}
	if st, err := f.Stat(); err != nil || st.IsDir() {
		_ = f.Close()
		return nil, os.ErrNotExist
	}
	return f, nil
}

func toAny[T any](in []T) []any {
	out := make([]any, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}
