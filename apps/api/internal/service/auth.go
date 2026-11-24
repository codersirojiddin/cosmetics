package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/apperr"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/store"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/tg"
)

const loginTTL = 5 * time.Minute

type Auth struct{ s *Services }

type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ExpiresIn    int    `json:"expiresIn"` // seconds
}

type LoginStart struct {
	Code      string    `json:"code"`
	Secret    string    `json:"secret"`
	DeepLink  string    `json:"deepLink"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type PollResult struct {
	Status string      `json:"status"` // pending | expired | confirmed
	Tokens *TokenPair  `json:"tokens,omitempty"`
	User   *store.User `json:"user,omitempty"`
}

// StartLogin creates a login request. The browser/app keeps `secret`; only `code` goes to Telegram.
func (a *Auth) StartLogin(ctx context.Context) (*LoginStart, error) {
	bot := a.s.BotUsername()
	if bot == "" {
		return nil, apperr.New(503, "bot_unavailable", "Telegram bot hozircha sozlanmagan")
	}
	code, secret := randToken(18), randToken(24)
	exp := time.Now().Add(loginTTL)
	if err := a.s.d.Store.CreateLoginRequest(ctx, code, sha256Hex(secret), exp); err != nil {
		return nil, err
	}
	return &LoginStart{Code: code, Secret: secret, DeepLink: fmt.Sprintf("https://t.me/%s?start=login_%s", bot, code), ExpiresAt: exp}, nil
}

// upsertTelegramUser creates/refreshes the user and promotes configured owners.
func (a *Auth) upsertTelegramUser(ctx context.Context, q *store.Queries, u tg.User) (*store.User, error) {
	user, err := q.UpsertTelegramUser(ctx, u.ID, u.FirstName, u.LastName, u.Username, slices.Contains(a.s.d.Cfg.OwnerTelegramIDs, u.ID))
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, apperr.Forbidden("Akkaunt bloklangan")
	}
	return user, nil
}

// ConfirmLogin is called by the bot when a user taps Start on a login deep link.
func (a *Auth) ConfirmLogin(ctx context.Context, code string, u tg.User) (*store.User, error) {
	var user *store.User
	err := a.s.d.Store.InTx(ctx, func(q *store.Queries) error {
		var err error
		if user, err = a.upsertTelegramUser(ctx, q, u); err != nil {
			return err
		}
		ok, err := q.ConfirmLoginRequest(ctx, code, user.ID)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.BadRequest("login_expired", "Havola eskirgan yoki allaqachon ishlatilgan")
		}
		return nil
	})
	return user, err
}

// Poll lets the original client exchange a confirmed login for tokens (exactly once).
func (a *Auth) Poll(ctx context.Context, code, secret string) (*PollResult, error) {
	st := a.s.d.Store
	lr, err := st.GetLoginRequest(ctx, code)
	if errors.Is(err, store.ErrNotFound) {
		return &PollResult{Status: "expired"}, nil
	}
	if err != nil {
		return nil, err
	}
	if subtle.ConstantTimeCompare([]byte(sha256Hex(secret)), []byte(lr.SecretHash)) != 1 {
		return &PollResult{Status: "expired"}, nil
	}
	switch lr.Status {
	case "pending":
		if time.Now().After(lr.ExpiresAt) {
			return &PollResult{Status: "expired"}, nil
		}
		return &PollResult{Status: "pending"}, nil
	case "confirmed":
		uid, ok, err := st.ConsumeLoginRequest(ctx, code)
		if err != nil {
			return nil, err
		}
		if !ok {
			return &PollResult{Status: "expired"}, nil
		}
		user, err := st.GetUser(ctx, uid)
		if err != nil {
			return nil, err
		}
		if !user.IsActive {
			return nil, apperr.Forbidden("Akkaunt bloklangan")
		}
		tokens, err := a.IssueTokens(ctx, user.ID)
		if err != nil {
			return nil, err
		}
		return &PollResult{Status: "confirmed", Tokens: tokens, User: user}, nil
	default:
		return &PollResult{Status: "expired"}, nil
	}
}

func (a *Auth) IssueTokens(ctx context.Context, userID string) (*TokenPair, error) {
	cfg := a.s.d.Cfg
	now := time.Now()
	access, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(cfg.AccessTTL)),
	}).SignedString([]byte(cfg.JWTSecret))
	if err != nil {
		return nil, err
	}
	refresh := randToken(32)
	if err := a.s.d.Store.InsertRefreshToken(ctx, userID, sha256Hex(refresh), now.Add(cfg.RefreshTTL)); err != nil {
		return nil, err
	}
	return &TokenPair{AccessToken: access, RefreshToken: refresh, ExpiresIn: int(cfg.AccessTTL.Seconds())}, nil
}

// Refresh rotates a refresh token. Reusing an already-rotated token revokes the whole family.
func (a *Auth) Refresh(ctx context.Context, refresh string) (*TokenPair, error) {
	st := a.s.d.Store
	bad := apperr.Unauthorized("Sessiya tugagan, qaytadan kiring")
	rt, err := st.GetRefreshToken(ctx, sha256Hex(refresh))
	if errors.Is(err, store.ErrNotFound) {
		return nil, bad
	}
	if err != nil {
		return nil, err
	}
	if rt.RevokedAt != nil {
		_ = st.RevokeUserRefreshTokens(ctx, rt.UserID)
		return nil, bad
	}
	if time.Now().After(rt.ExpiresAt) {
		return nil, bad
	}
	user, err := st.GetUser(ctx, rt.UserID)
	if err != nil || !user.IsActive {
		return nil, bad
	}
	if ok, err := st.RevokeRefreshToken(ctx, rt.ID); err != nil {
		return nil, err
	} else if !ok {
		return nil, bad
	}
	return a.IssueTokens(ctx, user.ID)
}

func (a *Auth) Logout(ctx context.Context, refresh string) error {
	st := a.s.d.Store
	rt, err := st.GetRefreshToken(ctx, sha256Hex(refresh))
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	_, err = st.RevokeRefreshToken(ctx, rt.ID)
	return err
}

// Authenticate validates an access token and loads the current user from the database,
// so role changes and blocks take effect immediately.
func (a *Auth) Authenticate(ctx context.Context, token string) (*store.User, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(token, claims, func(*jwt.Token) (any, error) { return []byte(a.s.d.Cfg.JWTSecret), nil },
		jwt.WithValidMethods([]string{"HS256"}), jwt.WithExpirationRequired())
	if err != nil || claims.Subject == "" {
		return nil, apperr.Unauthorized("Token yaroqsiz yoki muddati tugagan")
	}
	user, err := a.s.d.Store.GetUser(ctx, claims.Subject)
	if errors.Is(err, store.ErrNotFound) || store.IsInvalidText(err) {
		return nil, apperr.Unauthorized("Foydalanuvchi topilmadi")
	}
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, apperr.Forbidden("Akkaunt bloklangan")
	}
	return user, nil
}

// StaffByTelegramID resolves an active admin/owner from a Telegram user id (bot callbacks).
func (a *Auth) StaffByTelegramID(ctx context.Context, tgID int64) (*store.User, error) {
	u, err := a.s.d.Store.GetUserByTelegramID(ctx, tgID)
	if errors.Is(err, store.ErrNotFound) || (err == nil && (!u.IsStaff() || !u.IsActive)) {
		return nil, apperr.Forbidden("Bu amal faqat adminlar uchun")
	}
	return u, err
}

type UpdateMeInput struct {
	Phone                *string `json:"phone" validate:"omitempty,max=30"`
	NotificationsEnabled *bool   `json:"notificationsEnabled"`
}

func (a *Auth) UpdateMe(ctx context.Context, u *store.User, in UpdateMeInput) (*store.User, error) {
	var phone *string
	if in.Phone != nil && *in.Phone != "" {
		p, ok := normalizePhone(*in.Phone)
		if !ok {
			return nil, apperr.Field("phone", "Telefon raqami noto'g'ri. Masalan: +998 90 123 45 67")
		}
		phone = &p
	}
	return a.s.d.Store.UpdateMe(ctx, u.ID, phone, in.NotificationsEnabled)
}
