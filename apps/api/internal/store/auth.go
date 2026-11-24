package store

import (
	"context"
	"time"
)

type LoginRequest struct {
	Code       string    `db:"code"`
	SecretHash string    `db:"secret_hash"`
	Status     string    `db:"status"`
	UserID     *string   `db:"user_id"`
	ExpiresAt  time.Time `db:"expires_at"`
}

func (q *Queries) CreateLoginRequest(ctx context.Context, code, secretHash string, expires time.Time) error {
	_, err := q.db.Exec(ctx, `INSERT INTO login_requests (code, secret_hash, expires_at) VALUES ($1, $2, $3)`, code, secretHash, expires)
	return err
}

func (q *Queries) GetLoginRequest(ctx context.Context, code string) (*LoginRequest, error) {
	return one[LoginRequest](q.db.Query(ctx, `SELECT code, secret_hash, status, user_id, expires_at FROM login_requests WHERE code = $1`, code))
}

// ConfirmLoginRequest marks a pending, unexpired request as confirmed by userID.
func (q *Queries) ConfirmLoginRequest(ctx context.Context, code, userID string) (bool, error) {
	tag, err := q.db.Exec(ctx, `UPDATE login_requests SET status = 'confirmed', user_id = $2
		WHERE code = $1 AND status = 'pending' AND expires_at > now()`, code, userID)
	return tag.RowsAffected() == 1, err
}

// ConsumeLoginRequest atomically turns a confirmed request into consumed and returns its user.
func (q *Queries) ConsumeLoginRequest(ctx context.Context, code string) (string, bool, error) {
	var uid string
	rows, err := q.db.Query(ctx, `UPDATE login_requests SET status = 'consumed'
		WHERE code = $1 AND status = 'confirmed' AND expires_at > now() - interval '10 minutes' RETURNING user_id::text`, code)
	if err != nil {
		return "", false, err
	}
	defer rows.Close()
	if rows.Next() {
		if err := rows.Scan(&uid); err != nil {
			return "", false, err
		}
		return uid, true, nil
	}
	return "", false, rows.Err()
}

type RefreshToken struct {
	ID        string     `db:"id"`
	UserID    string     `db:"user_id"`
	ExpiresAt time.Time  `db:"expires_at"`
	RevokedAt *time.Time `db:"revoked_at"`
}

func (q *Queries) InsertRefreshToken(ctx context.Context, userID, hash string, expires time.Time) error {
	_, err := q.db.Exec(ctx, `INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`, userID, hash, expires)
	return err
}

func (q *Queries) GetRefreshToken(ctx context.Context, hash string) (*RefreshToken, error) {
	return one[RefreshToken](q.db.Query(ctx, `SELECT id, user_id, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1`, hash))
}

// RevokeRefreshToken revokes a token; false means it was already revoked (reuse).
func (q *Queries) RevokeRefreshToken(ctx context.Context, id string) (bool, error) {
	tag, err := q.db.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE id = $1 AND revoked_at IS NULL`, id)
	return tag.RowsAffected() == 1, err
}

func (q *Queries) RevokeUserRefreshTokens(ctx context.Context, userID string) error {
	_, err := q.db.Exec(ctx, `UPDATE refresh_tokens SET revoked_at = now() WHERE user_id = $1 AND revoked_at IS NULL`, userID)
	return err
}

// ---- admin invites ----

func (q *Queries) CreateInvite(ctx context.Context, code, createdBy string, expires time.Time) error {
	_, err := q.db.Exec(ctx, `INSERT INTO admin_invites (code, created_by, expires_at) VALUES ($1, $2, $3)`, code, createdBy, expires)
	return err
}

// UseInvite consumes an invite atomically; false when it is unknown, used or expired.
func (q *Queries) UseInvite(ctx context.Context, code, userID string) (bool, error) {
	tag, err := q.db.Exec(ctx, `UPDATE admin_invites SET used_by = $2, used_at = now()
		WHERE code = $1 AND used_by IS NULL AND expires_at > now()`, code, userID)
	return tag.RowsAffected() == 1, err
}
