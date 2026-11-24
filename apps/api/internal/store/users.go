package store

import (
	"context"
	"strings"
	"time"
)

type User struct {
	ID                   string     `db:"id" json:"id"`
	TelegramID           int64      `db:"telegram_id" json:"telegramId"`
	FirstName            string     `db:"first_name" json:"firstName"`
	LastName             string     `db:"last_name" json:"lastName"`
	Username             string     `db:"username" json:"username"`
	Phone                *string    `db:"phone" json:"phone"`
	Role                 string     `db:"role" json:"role"`
	IsActive             bool       `db:"is_active" json:"isActive"`
	NotificationsEnabled bool       `db:"notifications_enabled" json:"notificationsEnabled"`
	CreatedAt            time.Time  `db:"created_at" json:"createdAt"`
	LastLoginAt          *time.Time `db:"last_login_at" json:"lastLoginAt"`
}

func (u *User) DisplayName() string {
	if n := strings.TrimSpace(u.FirstName + " " + u.LastName); n != "" {
		return n
	}
	if u.Username != "" {
		return "@" + u.Username
	}
	return "Foydalanuvchi"
}

func (u *User) IsStaff() bool { return u.Role == "admin" || u.Role == "owner" }

const userCols = `id, telegram_id, first_name, last_name, username, phone, role, is_active, notifications_enabled, created_at, last_login_at`

// UpsertTelegramUser creates or refreshes a user from Telegram profile data.
// makeOwner promotes the user to owner (used for OWNER_TELEGRAM_IDS).
func (q *Queries) UpsertTelegramUser(ctx context.Context, tgID int64, first, last, username string, makeOwner bool) (*User, error) {
	return one[User](q.db.Query(ctx, `
		INSERT INTO users (telegram_id, first_name, last_name, username, role, last_login_at)
		VALUES ($1, $2, $3, $4, CASE WHEN $5::boolean THEN 'owner' ELSE 'customer' END, now())
		ON CONFLICT (telegram_id) DO UPDATE SET
			first_name = EXCLUDED.first_name, last_name = EXCLUDED.last_name, username = EXCLUDED.username,
			role = CASE WHEN $5::boolean THEN 'owner' ELSE users.role END,
			last_login_at = now(), updated_at = now()
		RETURNING `+userCols, tgID, first, last, username, makeOwner))
}

func (q *Queries) GetUser(ctx context.Context, id string) (*User, error) {
	return one[User](q.db.Query(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id))
}

func (q *Queries) GetUserByTelegramID(ctx context.Context, tgID int64) (*User, error) {
	return one[User](q.db.Query(ctx, `SELECT `+userCols+` FROM users WHERE telegram_id = $1`, tgID))
}

func (q *Queries) ListStaff(ctx context.Context) ([]User, error) {
	return list[User](q.db.Query(ctx, `SELECT `+userCols+` FROM users WHERE role IN ('admin','owner') ORDER BY role DESC, created_at`))
}

// ListNotifiableStaff returns active staff who want Telegram notifications.
func (q *Queries) ListNotifiableStaff(ctx context.Context) ([]User, error) {
	return list[User](q.db.Query(ctx, `SELECT `+userCols+` FROM users
		WHERE role IN ('admin','owner') AND is_active AND notifications_enabled ORDER BY created_at`))
}

func (q *Queries) SetUserRole(ctx context.Context, id, role string) error {
	_, err := q.db.Exec(ctx, `UPDATE users SET role = $2, updated_at = now() WHERE id = $1`, id, role)
	return err
}

func (q *Queries) SetUserActive(ctx context.Context, id string, active bool) error {
	_, err := q.db.Exec(ctx, `UPDATE users SET is_active = $2, updated_at = now() WHERE id = $1`, id, active)
	return err
}

func (q *Queries) UpdateMe(ctx context.Context, id string, phone *string, notifications *bool) (*User, error) {
	return one[User](q.db.Query(ctx, `
		UPDATE users SET phone = COALESCE($2, phone), notifications_enabled = COALESCE($3, notifications_enabled), updated_at = now()
		WHERE id = $1 RETURNING `+userCols, id, phone, notifications))
}
