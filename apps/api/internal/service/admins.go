package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/apperr"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/store"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/tg"
)

const inviteTTL = 24 * time.Hour

type Admins struct{ s *Services }

type Invite struct {
	Code      string    `json:"code"`
	DeepLink  string    `json:"deepLink"`
	ExpiresAt time.Time `json:"expiresAt"`
}

func requireOwner(actor *store.User) error {
	if actor == nil || actor.Role != "owner" {
		return apperr.Forbidden("Bu amal faqat owner uchun")
	}
	return nil
}

// CreateInvite makes a one-time link; whoever opens it in Telegram becomes an admin.
func (a *Admins) CreateInvite(ctx context.Context, actor *store.User) (*Invite, error) {
	if err := requireOwner(actor); err != nil {
		return nil, err
	}
	bot := a.s.BotUsername()
	if bot == "" {
		return nil, apperr.New(503, "bot_unavailable", "Telegram bot hozircha sozlanmagan")
	}
	code := randToken(16)
	exp := time.Now().Add(inviteTTL)
	if err := a.s.d.Store.CreateInvite(ctx, code, actor.ID, exp); err != nil {
		return nil, err
	}
	a.s.audit(ctx, actor, "admin.invite_created", "admin_invite", "", nil)
	return &Invite{Code: code, DeepLink: fmt.Sprintf("https://t.me/%s?start=admin_%s", bot, code), ExpiresAt: exp}, nil
}

// AcceptInvite is called by the bot when someone opens an admin invite link.
func (a *Admins) AcceptInvite(ctx context.Context, code string, u tg.User) (*store.User, error) {
	var user *store.User
	err := a.s.d.Store.InTx(ctx, func(q *store.Queries) error {
		var err error
		if user, err = (&Auth{a.s}).upsertTelegramUser(ctx, q, u); err != nil {
			return err
		}
		if user.IsStaff() {
			return apperr.Conflict("already_staff", "Siz allaqachon adminsiz")
		}
		ok, err := q.UseInvite(ctx, code, user.ID)
		if err != nil {
			return err
		}
		if !ok {
			return apperr.BadRequest("invite_invalid", "Taklif havolasi eskirgan yoki allaqachon ishlatilgan")
		}
		user.Role = "admin"
		return q.SetUserRole(ctx, user.ID, "admin")
	})
	if err == nil {
		a.s.audit(ctx, user, "admin.joined", "user", user.ID, nil)
	}
	return user, err
}

func (a *Admins) List(ctx context.Context, actor *store.User) ([]store.User, error) {
	if err := requireOwner(actor); err != nil {
		return nil, err
	}
	return a.s.d.Store.ListStaff(ctx)
}

func (a *Admins) target(ctx context.Context, actor *store.User, id string) (*store.User, error) {
	if err := requireOwner(actor); err != nil {
		return nil, err
	}
	t, err := a.s.d.Store.GetUser(ctx, id)
	if errors.Is(err, store.ErrNotFound) || store.IsInvalidText(err) {
		return nil, apperr.NotFound("Admin topilmadi")
	}
	if err != nil {
		return nil, err
	}
	if t.Role != "admin" {
		return nil, apperr.BadRequest("not_admin", "Faqat adminlarni o'zgartirish mumkin (owner'ni emas)")
	}
	return t, nil
}

// SetActive blocks or unblocks an admin. A blocked admin loses access immediately.
func (a *Admins) SetActive(ctx context.Context, actor *store.User, id string, active bool) (*store.User, error) {
	t, err := a.target(ctx, actor, id)
	if err != nil {
		return nil, err
	}
	if err := a.s.d.Store.SetUserActive(ctx, t.ID, active); err != nil {
		return nil, err
	}
	if !active {
		_ = a.s.d.Store.RevokeUserRefreshTokens(ctx, t.ID)
	}
	t.IsActive = active
	a.s.audit(ctx, actor, "admin.set_active", "user", t.ID, map[string]any{"active": active})
	return t, nil
}

// Remove demotes an admin back to a regular customer.
func (a *Admins) Remove(ctx context.Context, actor *store.User, id string) error {
	t, err := a.target(ctx, actor, id)
	if err != nil {
		return err
	}
	if err := a.s.d.Store.SetUserRole(ctx, t.ID, "customer"); err != nil {
		return err
	}
	a.s.audit(ctx, actor, "admin.removed", "user", t.ID, nil)
	return nil
}
