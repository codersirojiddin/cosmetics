package store

import (
	"context"
	"time"
)

// ---------- settings ----------

func (q *Queries) GetSettings(ctx context.Context) (map[string]string, error) {
	rows, err := q.db.Query(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, rows.Err()
}

func (q *Queries) SetSettings(ctx context.Context, kv map[string]string, by string) error {
	for k, v := range kv {
		if _, err := q.db.Exec(ctx, `INSERT INTO settings (key, value, updated_by) VALUES ($1,$2,$3)
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_by = EXCLUDED.updated_by, updated_at = now()`, k, v, by); err != nil {
			return err
		}
	}
	return nil
}

// ---------- audit ----------

type AuditRow struct {
	ID        int64          `db:"id" json:"id"`
	ActorID   *string        `db:"actor_id" json:"actorId"`
	ActorName string         `db:"actor_name" json:"actor"`
	Action    string         `db:"action" json:"action"`
	Entity    string         `db:"entity" json:"entity"`
	EntityID  string         `db:"entity_id" json:"entityId"`
	Meta      map[string]any `db:"meta" json:"meta"`
	CreatedAt time.Time      `db:"created_at" json:"createdAt"`
	Total     int            `db:"total" json:"-"`
}

func (q *Queries) InsertAudit(ctx context.Context, actorID *string, action, entity, entityID string, meta map[string]any) error {
	if meta == nil {
		meta = map[string]any{}
	}
	_, err := q.db.Exec(ctx, `INSERT INTO audit_log (actor_id, action, entity, entity_id, meta) VALUES ($1,$2,$3,$4,$5)`, actorID, action, entity, entityID, meta)
	return err
}

func (q *Queries) ListAudit(ctx context.Context, limit, offset int) ([]AuditRow, error) {
	return list[AuditRow](q.db.Query(ctx, `SELECT a.id, a.actor_id, `+nameExpr("u")+` AS actor_name, a.action, a.entity, a.entity_id, a.meta, a.created_at, (count(*) OVER())::int AS total
		FROM audit_log a LEFT JOIN users u ON u.id = a.actor_id ORDER BY a.id DESC LIMIT $1 OFFSET $2`, limit, offset))
}
