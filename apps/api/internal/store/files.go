package store

import (
	"context"
	"time"
)

type File struct {
	ID        string            `db:"id" json:"id"`
	Kind      string            `db:"kind" json:"kind"`
	OwnerID   *string           `db:"owner_id" json:"-"`
	Variants  map[string]string `db:"variants" json:"-"`
	Mime      string            `db:"mime" json:"mime"`
	SizeBytes int64             `db:"size_bytes" json:"sizeBytes"`
	Width     int               `db:"width" json:"width"`
	Height    int               `db:"height" json:"height"`
	SHA256    string            `db:"sha256" json:"-"`
	DHash     *int64            `db:"dhash" json:"-"`
	CreatedAt time.Time         `db:"created_at" json:"createdAt"`
}

const fileCols = `id, kind, owner_id, variants, mime, size_bytes, width, height, sha256, dhash, created_at`

func (q *Queries) InsertFile(ctx context.Context, f *File) error {
	return q.db.QueryRow(ctx, `INSERT INTO files (id, kind, owner_id, variants, mime, size_bytes, width, height, sha256, dhash)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10) RETURNING created_at`,
		f.ID, f.Kind, f.OwnerID, f.Variants, f.Mime, f.SizeBytes, f.Width, f.Height, f.SHA256, f.DHash).Scan(&f.CreatedAt)
}

func (q *Queries) GetFile(ctx context.Context, id string) (*File, error) {
	return one[File](q.db.Query(ctx, `SELECT `+fileCols+` FROM files WHERE id = $1`, id))
}

func (q *Queries) GetFiles(ctx context.Context, ids []string) (map[string]File, error) {
	rows, err := list[File](q.db.Query(ctx, `SELECT `+fileCols+` FROM files WHERE id = ANY($1::text[]::uuid[])`, ids))
	if err != nil {
		return nil, err
	}
	m := make(map[string]File, len(rows))
	for _, f := range rows {
		m[f.ID] = f
	}
	return m, nil
}

// OrderUserByReceipt returns the owner (customer) of the order that uses this receipt file.
func (q *Queries) OrderUserByReceipt(ctx context.Context, fileID string) (string, error) {
	var uid string
	rows, err := q.db.Query(ctx, `SELECT user_id::text FROM orders WHERE receipt_file_id = $1`, fileID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	if rows.Next() {
		err = rows.Scan(&uid)
		return uid, err
	}
	return "", ErrNotFound
}

type ReceiptHash struct {
	OrderID string `db:"order_id"`
	SHA256  string `db:"sha256"`
	DHash   *int64 `db:"dhash"`
}

// RecentReceiptHashes lists receipts of orders created since the given time.
func (q *Queries) RecentReceiptHashes(ctx context.Context, since time.Time) ([]ReceiptHash, error) {
	return list[ReceiptHash](q.db.Query(ctx, `SELECT o.id AS order_id, f.sha256, f.dhash
		FROM orders o JOIN files f ON f.id = o.receipt_file_id WHERE o.created_at > $1 ORDER BY o.created_at DESC`, since))
}

// CountRecentUploads counts files of a kind uploaded by an owner since the given time (abuse limit).
func (q *Queries) CountRecentUploads(ctx context.Context, ownerID, kind string, since time.Time) (int, error) {
	var n int
	err := q.db.QueryRow(ctx, `SELECT count(*)::int FROM files WHERE owner_id = $1 AND kind = $2 AND created_at > $3`, ownerID, kind, since).Scan(&n)
	return n, err
}
