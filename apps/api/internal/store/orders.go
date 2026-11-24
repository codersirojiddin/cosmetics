package store

import (
	"context"
	"strconv"
	"strings"
	"time"
)

type Order struct {
	ID                   string    `db:"id" json:"id"`
	Number               int64     `db:"number" json:"number"`
	UserID               string    `db:"user_id" json:"-"`
	Status               string    `db:"status" json:"status"`
	CustomerName         string    `db:"customer_name" json:"customerName"`
	Phone                string    `db:"phone" json:"phone"`
	AddressText          string    `db:"address_text" json:"addressText"`
	Lat                  float64   `db:"lat" json:"lat"`
	Lng                  float64   `db:"lng" json:"lng"`
	CustomerComment      string    `db:"customer_comment" json:"comment"`
	TotalAmount          int64     `db:"total_amount" json:"totalAmount"`
	ReceiptFileID        string    `db:"receipt_file_id" json:"-"`
	ReceiptDuplicateOf   *string   `db:"receipt_duplicate_of" json:"-"`
	ReceiptDuplicateKind *string   `db:"receipt_duplicate_kind" json:"-"`
	AssignedAdminID      *string   `db:"assigned_admin_id" json:"-"`
	RejectReason         string    `db:"reject_reason" json:"rejectReason"`
	CreatedAt            time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt            time.Time `db:"updated_at" json:"updatedAt"`

	// populated by the select helpers below
	AssignedAdminName string `db:"assigned_admin_name" json:"-"`
	CustomerUsername  string `db:"customer_username" json:"-"`
	CustomerTelegram  int64  `db:"customer_telegram_id" json:"-"`
	ItemCount         int    `db:"item_count" json:"-"`
	TotalRows         int    `db:"total_rows" json:"-"`
}

const orderSelect = `SELECT o.id, o.number, o.user_id, o.status, o.customer_name, o.phone, o.address_text, o.lat, o.lng, o.customer_comment,
	o.total_amount, o.receipt_file_id, o.receipt_duplicate_of, o.receipt_duplicate_kind, o.assigned_admin_id, o.reject_reason, o.created_at, o.updated_at,
	` + "COALESCE(NULLIF(btrim(aa.first_name || ' ' || aa.last_name), ''), NULLIF(aa.username, ''), '')" + ` AS assigned_admin_name,
	u.username AS customer_username, u.telegram_id AS customer_telegram_id,
	(SELECT count(*) FROM order_items i WHERE i.order_id = o.id)::int AS item_count,
	(count(*) OVER())::int AS total_rows
	FROM orders o JOIN users u ON u.id = o.user_id LEFT JOIN users aa ON aa.id = o.assigned_admin_id`

func (q *Queries) GetOrder(ctx context.Context, id string) (*Order, error) {
	return one[Order](q.db.Query(ctx, orderSelect+` WHERE o.id = $1`, id))
}

// GetOrderForUpdate locks the order row for the duration of the transaction.
func (q *Queries) GetOrderForUpdate(ctx context.Context, id string) (*Order, error) {
	return one[Order](q.db.Query(ctx, `SELECT id, number, user_id, status, customer_name, phone, address_text, lat, lng, customer_comment,
		total_amount, receipt_file_id, receipt_duplicate_of, receipt_duplicate_kind, assigned_admin_id, reject_reason, created_at, updated_at
		FROM orders WHERE id = $1 FOR UPDATE`, id))
}

func (q *Queries) InsertOrder(ctx context.Context, o *Order) error {
	return q.db.QueryRow(ctx, `INSERT INTO orders (user_id, status, customer_name, phone, address_text, lat, lng, customer_comment, total_amount,
		receipt_file_id, receipt_duplicate_of, receipt_duplicate_kind) VALUES ($1,'waiting',$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, number, status, created_at, updated_at`,
		o.UserID, o.CustomerName, o.Phone, o.AddressText, o.Lat, o.Lng, o.CustomerComment, o.TotalAmount,
		o.ReceiptFileID, o.ReceiptDuplicateOf, o.ReceiptDuplicateKind).Scan(&o.ID, &o.Number, &o.Status, &o.CreatedAt, &o.UpdatedAt)
}

func (q *Queries) UpdateOrderStatus(ctx context.Context, id, status, rejectReason string) error {
	_, err := q.db.Exec(ctx, `UPDATE orders SET status = $2, reject_reason = $3, updated_at = now() WHERE id = $1`, id, status, rejectReason)
	return err
}

// ClaimOrder assigns the order to adminID only if nobody has claimed it yet.
func (q *Queries) ClaimOrder(ctx context.Context, orderID, adminID string) (bool, error) {
	tag, err := q.db.Exec(ctx, `UPDATE orders SET assigned_admin_id = $2, updated_at = now() WHERE id = $1 AND assigned_admin_id IS NULL`, orderID, adminID)
	return tag.RowsAffected() == 1, err
}

func (q *Queries) ForceAssign(ctx context.Context, orderID, adminID string) error {
	_, err := q.db.Exec(ctx, `UPDATE orders SET assigned_admin_id = $2, updated_at = now() WHERE id = $1`, orderID, adminID)
	return err
}

// ---------- items ----------

type OrderItem struct {
	ID           string  `db:"id" json:"id"`
	OrderID      string  `db:"order_id" json:"-"`
	ProductID    string  `db:"product_id" json:"productId"`
	VariantID    string  `db:"variant_id" json:"variantId"`
	ProductName  string  `db:"product_name" json:"productName"`
	VariantLabel string  `db:"variant_label" json:"variantLabel"`
	SKU          string  `db:"sku" json:"sku"`
	ImageFileID  *string `db:"image_file_id" json:"-"`
	UnitPrice    int64   `db:"unit_price" json:"unitPrice"`
	Quantity     int     `db:"quantity" json:"quantity"`
}

func (q *Queries) InsertOrderItem(ctx context.Context, it *OrderItem) error {
	return q.db.QueryRow(ctx, `INSERT INTO order_items (order_id, product_id, variant_id, product_name, variant_label, sku, image_file_id, unit_price, quantity)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		it.OrderID, it.ProductID, it.VariantID, it.ProductName, it.VariantLabel, it.SKU, it.ImageFileID, it.UnitPrice, it.Quantity).Scan(&it.ID)
}

func (q *Queries) ListOrderItems(ctx context.Context, orderIDs []string) ([]OrderItem, error) {
	return list[OrderItem](q.db.Query(ctx, `SELECT id, order_id, product_id, variant_id, product_name, variant_label, sku, image_file_id, unit_price, quantity
		FROM order_items WHERE order_id = ANY($1::text[]::uuid[]) ORDER BY product_name, variant_label`, orderIDs))
}

// ---------- lists ----------

func (q *Queries) ListUserOrders(ctx context.Context, userID string, limit, offset int) ([]Order, error) {
	return list[Order](q.db.Query(ctx, orderSelect+` WHERE o.user_id = $1 ORDER BY o.created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset))
}

type AdminOrderFilter struct {
	Status   string
	Assigned string // "" (any) | "me" | "none"
	AdminID  string
	Q        string
	Limit    int
	Offset   int
}

func (q *Queries) ListAdminOrders(ctx context.Context, f AdminOrderFilter) ([]Order, error) {
	var conds []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, strings.ReplaceAll(cond, "?", "$"+strconv.Itoa(len(args))))
	}
	if f.Status != "" {
		add(`o.status = ?`, f.Status)
	}
	switch f.Assigned {
	case "me":
		add(`o.assigned_admin_id = ?::uuid`, f.AdminID)
	case "none":
		conds = append(conds, `o.assigned_admin_id IS NULL`)
	}
	if qs := strings.TrimPrefix(strings.TrimSpace(f.Q), "#"); qs != "" {
		if n, err := strconv.ParseInt(qs, 10, 64); err == nil && len(qs) <= 9 {
			args = append(args, n, likePattern(qs))
			conds = append(conds, `(o.number = $`+strconv.Itoa(len(args)-1)+` OR o.phone ILIKE $`+strconv.Itoa(len(args))+`)`)
		} else {
			args = append(args, likePattern(qs))
			p := `$` + strconv.Itoa(len(args))
			conds = append(conds, `(o.customer_name ILIKE `+p+` OR o.phone ILIKE `+p+`)`)
		}
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	args = append(args, f.Limit, f.Offset)
	return list[Order](q.db.Query(ctx, orderSelect+where+` ORDER BY o.created_at DESC LIMIT $`+strconv.Itoa(len(args)-1)+` OFFSET $`+strconv.Itoa(len(args)), args...))
}

// OrderStats returns order counts per status and the number of active orders nobody has claimed.
func (q *Queries) OrderStats(ctx context.Context) (map[string]int, int, error) {
	rows, err := q.db.Query(ctx, `SELECT status, count(*)::int FROM orders GROUP BY status`)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	m := map[string]int{"waiting": 0, "confirmed": 0, "shipping": 0, "delivered": 0, "rejected": 0}
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, 0, err
		}
		m[s] = n
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	var unassigned int
	err = q.db.QueryRow(ctx, `SELECT count(*)::int FROM orders WHERE assigned_admin_id IS NULL AND status IN ('waiting','confirmed','shipping')`).Scan(&unassigned)
	return m, unassigned, err
}

// ---------- history & notes ----------

type HistoryRow struct {
	ID            int64     `db:"id" json:"-"`
	FromStatus    *string   `db:"from_status" json:"from"`
	ToStatus      string    `db:"to_status" json:"to"`
	ChangedBy     *string   `db:"changed_by" json:"-"`
	ChangedByName string    `db:"changed_by_name" json:"changedBy"`
	Comment       string    `db:"comment" json:"comment"`
	CreatedAt     time.Time `db:"created_at" json:"at"`
}

func (q *Queries) InsertHistory(ctx context.Context, orderID string, from *string, to string, by *string, comment string) error {
	_, err := q.db.Exec(ctx, `INSERT INTO order_status_history (order_id, from_status, to_status, changed_by, comment) VALUES ($1,$2,$3,$4,$5)`,
		orderID, from, to, by, comment)
	return err
}

func (q *Queries) ListHistory(ctx context.Context, orderID string) ([]HistoryRow, error) {
	return list[HistoryRow](q.db.Query(ctx, `SELECT h.id, h.from_status, h.to_status, h.changed_by, `+nameExpr("u")+` AS changed_by_name, h.comment, h.created_at
		FROM order_status_history h LEFT JOIN users u ON u.id = h.changed_by WHERE h.order_id = $1 ORDER BY h.id`, orderID))
}

type NoteRow struct {
	ID         string    `db:"id" json:"id"`
	AuthorID   *string   `db:"author_id" json:"-"`
	AuthorName string    `db:"author_name" json:"author"`
	Body       string    `db:"body" json:"body"`
	CreatedAt  time.Time `db:"created_at" json:"createdAt"`
}

func (q *Queries) InsertNote(ctx context.Context, orderID, authorID, body string) error {
	_, err := q.db.Exec(ctx, `INSERT INTO order_notes (order_id, author_id, body) VALUES ($1,$2,$3)`, orderID, authorID, body)
	return err
}

func (q *Queries) ListNotes(ctx context.Context, orderID string) ([]NoteRow, error) {
	return list[NoteRow](q.db.Query(ctx, `SELECT n.id, n.author_id, `+nameExpr("u")+` AS author_name, n.body, n.created_at
		FROM order_notes n LEFT JOIN users u ON u.id = n.author_id WHERE n.order_id = $1 ORDER BY n.created_at`, orderID))
}

// ---------- telegram message refs ----------

type AdminMessage struct {
	ChatID    int64 `db:"chat_id"`
	MessageID int64 `db:"message_id"`
}

func (q *Queries) SaveAdminMessage(ctx context.Context, orderID string, chatID, messageID int64) error {
	_, err := q.db.Exec(ctx, `INSERT INTO order_admin_messages (order_id, chat_id, message_id) VALUES ($1,$2,$3)
		ON CONFLICT (order_id, chat_id) DO UPDATE SET message_id = EXCLUDED.message_id`, orderID, chatID, messageID)
	return err
}

func (q *Queries) ListAdminMessages(ctx context.Context, orderID string) ([]AdminMessage, error) {
	return list[AdminMessage](q.db.Query(ctx, `SELECT chat_id, message_id FROM order_admin_messages WHERE order_id = $1`, orderID))
}
