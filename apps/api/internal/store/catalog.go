package store

import (
	"context"
	"strconv"
	"strings"
	"time"
)

// ---------- categories ----------

type Category struct {
	ID           string    `db:"id" json:"id"`
	Name         string    `db:"name" json:"name"`
	Slug         string    `db:"slug" json:"slug"`
	ImageFileID  *string   `db:"image_file_id" json:"-"`
	Position     int       `db:"position" json:"position"`
	IsActive     bool      `db:"is_active" json:"isActive"`
	ProductCount int       `db:"product_count" json:"productCount"`
	CreatedAt    time.Time `db:"created_at" json:"createdAt"`
}

const categorySelect = `SELECT c.id, c.name, c.slug, c.image_file_id, c.position, c.is_active, c.created_at,
	(SELECT count(*) FROM products p WHERE p.category_id = c.id AND p.status = 'active') AS product_count
	FROM categories c`

func (q *Queries) ListCategories(ctx context.Context, onlyActive bool) ([]Category, error) {
	return list[Category](q.db.Query(ctx, categorySelect+` WHERE ($1::boolean = false OR c.is_active) ORDER BY c.position, c.name`, onlyActive))
}

func (q *Queries) GetCategory(ctx context.Context, id string) (*Category, error) {
	return one[Category](q.db.Query(ctx, categorySelect+` WHERE c.id = $1`, id))
}

func (q *Queries) InsertCategory(ctx context.Context, c *Category) error {
	return q.db.QueryRow(ctx, `INSERT INTO categories (name, slug, image_file_id, position, is_active) VALUES ($1,$2,$3,$4,$5)
		RETURNING id, created_at`, c.Name, c.Slug, c.ImageFileID, c.Position, c.IsActive).Scan(&c.ID, &c.CreatedAt)
}

func (q *Queries) UpdateCategory(ctx context.Context, c *Category) error {
	tag, err := q.db.Exec(ctx, `UPDATE categories SET name=$2, slug=$3, image_file_id=$4, position=$5, is_active=$6, updated_at=now() WHERE id=$1`,
		c.ID, c.Name, c.Slug, c.ImageFileID, c.Position, c.IsActive)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (q *Queries) DeleteCategory(ctx context.Context, id string) error {
	tag, err := q.db.Exec(ctx, `DELETE FROM categories WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (q *Queries) CategorySlugTaken(ctx context.Context, slug, excludeID string) (bool, error) {
	var n int
	err := q.db.QueryRow(ctx, `SELECT count(*) FROM categories WHERE slug = $1 AND ($2 = '' OR id::text <> $2)`, slug, excludeID).Scan(&n)
	return n > 0, err
}

// ---------- products ----------

type Product struct {
	ID          string    `db:"id" json:"id"`
	CategoryID  string    `db:"category_id" json:"categoryId"`
	Name        string    `db:"name" json:"name"`
	Slug        string    `db:"slug" json:"slug"`
	Brand       string    `db:"brand" json:"brand"`
	Description string    `db:"description" json:"description"`
	Ingredients string    `db:"ingredients" json:"ingredients"`
	HowToUse    string    `db:"how_to_use" json:"howToUse"`
	OptionLabel string    `db:"option_label" json:"optionLabel"`
	Status      string    `db:"status" json:"status"`
	IsNew       bool      `db:"is_new" json:"isNew"`
	IsFeatured  bool      `db:"is_featured" json:"isFeatured"`
	CreatedBy   *string   `db:"created_by" json:"-"`
	CreatedAt   time.Time `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time `db:"updated_at" json:"updatedAt"`
}

const productCols = `id, category_id, name, slug, brand, description, ingredients, how_to_use, option_label, status, is_new, is_featured, created_by, created_at, updated_at`

func (q *Queries) GetProductByID(ctx context.Context, id string) (*Product, error) {
	return one[Product](q.db.Query(ctx, `SELECT `+productCols+` FROM products WHERE id = $1`, id))
}

func (q *Queries) GetProductBySlug(ctx context.Context, slug string) (*Product, error) {
	return one[Product](q.db.Query(ctx, `SELECT `+productCols+` FROM products WHERE slug = $1`, slug))
}

func (q *Queries) InsertProduct(ctx context.Context, p *Product) error {
	return q.db.QueryRow(ctx, `INSERT INTO products (category_id, name, slug, brand, description, ingredients, how_to_use, option_label, status, is_new, is_featured, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12) RETURNING id, created_at, updated_at`,
		p.CategoryID, p.Name, p.Slug, p.Brand, p.Description, p.Ingredients, p.HowToUse, p.OptionLabel, p.Status, p.IsNew, p.IsFeatured, p.CreatedBy,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (q *Queries) UpdateProduct(ctx context.Context, p *Product) error {
	tag, err := q.db.Exec(ctx, `UPDATE products SET category_id=$2, name=$3, slug=$4, brand=$5, description=$6, ingredients=$7, how_to_use=$8,
		option_label=$9, status=$10, is_new=$11, is_featured=$12, updated_at=now() WHERE id=$1`,
		p.ID, p.CategoryID, p.Name, p.Slug, p.Brand, p.Description, p.Ingredients, p.HowToUse, p.OptionLabel, p.Status, p.IsNew, p.IsFeatured)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (q *Queries) SetProductStatus(ctx context.Context, id, status string) error {
	tag, err := q.db.Exec(ctx, `UPDATE products SET status = $2, updated_at = now() WHERE id = $1`, id, status)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}

func (q *Queries) ProductSlugTaken(ctx context.Context, slug, excludeID string) (bool, error) {
	var n int
	err := q.db.QueryRow(ctx, `SELECT count(*) FROM products WHERE slug = $1 AND ($2 = '' OR id::text <> $2)`, slug, excludeID).Scan(&n)
	return n > 0, err
}

type ProductFilter struct {
	Q            string
	CategorySlug string
	CategoryID   string
	Status       string // admin filter
	Public       bool   // storefront rules: active product, active category, has an active variant
	Featured     bool
	New          bool
	Sort         string // new | price_asc | price_desc | name
	Limit        int
	Offset       int
}

type ProductRow struct {
	ID           string            `db:"id"`
	Slug         string            `db:"slug"`
	Name         string            `db:"name"`
	Brand        string            `db:"brand"`
	Status       string            `db:"status"`
	CategoryID   string            `db:"category_id"`
	CategoryName string            `db:"category_name"`
	CategorySlug string            `db:"category_slug"`
	IsNew        bool              `db:"is_new"`
	IsFeatured   bool              `db:"is_featured"`
	PriceFrom    int64             `db:"price_from"`
	OldPriceFrom *int64            `db:"old_price_from"`
	InStock      bool              `db:"in_stock"`
	StockTotal   int               `db:"stock_total"`
	VariantCount int               `db:"variant_count"`
	ImageVariant map[string]string `db:"image_variants"`
	CreatedAt    time.Time         `db:"created_at"`
	UpdatedAt    time.Time         `db:"updated_at"`
	Total        int               `db:"total"`
}

func (q *Queries) ListProducts(ctx context.Context, f ProductFilter) ([]ProductRow, error) {
	var conds []string
	var args []any
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, strings.ReplaceAll(cond, "?", "$"+strconv.Itoa(len(args))))
	}
	if f.Public {
		conds = append(conds, `p.status = 'active' AND c.is_active AND EXISTS (SELECT 1 FROM product_variants v WHERE v.product_id = p.id AND v.is_active)`)
	}
	if f.Status != "" {
		add(`p.status = ?`, f.Status)
	}
	if f.CategorySlug != "" {
		add(`c.slug = ?`, f.CategorySlug)
	}
	if f.CategoryID != "" {
		add(`p.category_id = ?::uuid`, f.CategoryID)
	}
	if f.Featured {
		conds = append(conds, `p.is_featured`)
	}
	if f.New {
		conds = append(conds, `p.is_new`)
	}
	if f.Q != "" {
		add(`(p.name ILIKE ? OR p.brand ILIKE ?)`, likePattern(f.Q))
	}
	where := ""
	if len(conds) > 0 {
		where = " WHERE " + strings.Join(conds, " AND ")
	}
	order := map[string]string{
		"price_asc":  "price_from ASC, p.created_at DESC",
		"price_desc": "price_from DESC, p.created_at DESC",
		"name":       "p.name ASC",
	}[f.Sort]
	if order == "" {
		order = "p.created_at DESC"
	}
	args = append(args, f.Limit, f.Offset)
	sql := `SELECT p.id, p.slug, p.name, p.brand, p.status, p.category_id, c.name AS category_name, c.slug AS category_slug,
		p.is_new, p.is_featured, COALESCE(mv.price, 0) AS price_from, mv.old_price AS old_price_from,
		EXISTS (SELECT 1 FROM product_variants v WHERE v.product_id = p.id AND v.is_active AND v.stock > 0) AS in_stock,
		(SELECT COALESCE(sum(v.stock), 0) FROM product_variants v WHERE v.product_id = p.id AND v.is_active)::int AS stock_total,
		(SELECT count(*) FROM product_variants v WHERE v.product_id = p.id AND v.is_active)::int AS variant_count,
		img.variants AS image_variants, p.created_at, p.updated_at, (count(*) OVER())::int AS total
		FROM products p JOIN categories c ON c.id = p.category_id
		LEFT JOIN LATERAL (SELECT price, old_price FROM product_variants v WHERE v.product_id = p.id AND v.is_active ORDER BY price ASC LIMIT 1) mv ON true
		LEFT JOIN LATERAL (SELECT f.variants FROM product_images pi JOIN files f ON f.id = pi.file_id WHERE pi.product_id = p.id ORDER BY pi.position, pi.id LIMIT 1) img ON true` +
		where + ` ORDER BY ` + order + ` LIMIT $` + strconv.Itoa(len(args)-1) + ` OFFSET $` + strconv.Itoa(len(args))
	return list[ProductRow](q.db.Query(ctx, sql, args...))
}

// ---------- variants ----------

type Variant struct {
	ID          string    `db:"id" json:"id"`
	ProductID   string    `db:"product_id" json:"-"`
	Label       string    `db:"label" json:"label"`
	SKU         *string   `db:"sku" json:"sku"`
	Price       int64     `db:"price" json:"price"`
	OldPrice    *int64    `db:"old_price" json:"oldPrice"`
	Stock       int       `db:"stock" json:"stock"`
	ImageFileID *string   `db:"image_file_id" json:"-"`
	Position    int       `db:"position" json:"position"`
	IsActive    bool      `db:"is_active" json:"isActive"`
	UpdatedAt   time.Time `db:"updated_at" json:"-"`
}

const variantCols = `id, product_id, label, sku, price, old_price, stock, image_file_id, position, is_active, updated_at`

func (q *Queries) ListVariants(ctx context.Context, productID string, onlyActive bool) ([]Variant, error) {
	return list[Variant](q.db.Query(ctx, `SELECT `+variantCols+` FROM product_variants
		WHERE product_id = $1 AND ($2::boolean = false OR is_active) ORDER BY position, created_at`, productID, onlyActive))
}

func (q *Queries) InsertVariant(ctx context.Context, v *Variant) error {
	return q.db.QueryRow(ctx, `INSERT INTO product_variants (product_id, label, sku, price, old_price, stock, image_file_id, position, is_active)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING id`,
		v.ProductID, v.Label, v.SKU, v.Price, v.OldPrice, v.Stock, v.ImageFileID, v.Position, v.IsActive).Scan(&v.ID)
}

func (q *Queries) UpdateVariant(ctx context.Context, v *Variant) error {
	_, err := q.db.Exec(ctx, `UPDATE product_variants SET label=$2, sku=$3, price=$4, old_price=$5, stock=$6, image_file_id=$7, position=$8, is_active=$9, updated_at=now()
		WHERE id=$1 AND product_id=$10`, v.ID, v.Label, v.SKU, v.Price, v.OldPrice, v.Stock, v.ImageFileID, v.Position, v.IsActive, v.ProductID)
	return err
}

// RemoveVariant deletes a variant, or only deactivates it when orders reference it.
func (q *Queries) RemoveVariant(ctx context.Context, id string) error {
	var used bool
	if err := q.db.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM order_items WHERE variant_id = $1)`, id).Scan(&used); err != nil {
		return err
	}
	if used {
		_, err := q.db.Exec(ctx, `UPDATE product_variants SET is_active = false, updated_at = now() WHERE id = $1`, id)
		return err
	}
	_, err := q.db.Exec(ctx, `DELETE FROM product_variants WHERE id = $1`, id)
	return err
}

// LockedVariant is a variant row locked for an order, joined with what we need to validate it.
type LockedVariant struct {
	ID             string  `db:"id"`
	ProductID      string  `db:"product_id"`
	Label          string  `db:"label"`
	SKU            *string `db:"sku"`
	Price          int64   `db:"price"`
	Stock          int     `db:"stock"`
	ImageFileID    *string `db:"image_file_id"`
	IsActive       bool    `db:"is_active"`
	ProductName    string  `db:"product_name"`
	ProductStatus  string  `db:"product_status"`
	CategoryActive bool    `db:"category_active"`
	ProductImage   *string `db:"product_image_file_id"`
}

// LockVariants locks variants (ordered by id to avoid deadlocks) for stock reservation.
func (q *Queries) LockVariants(ctx context.Context, ids []string) ([]LockedVariant, error) {
	return list[LockedVariant](q.db.Query(ctx, `
		SELECT v.id, v.product_id, v.label, v.sku, v.price, v.stock, v.image_file_id, v.is_active,
			p.name AS product_name, p.status AS product_status, c.is_active AS category_active,
			(SELECT pi.file_id FROM product_images pi WHERE pi.product_id = p.id ORDER BY pi.position, pi.id LIMIT 1) AS product_image_file_id
		FROM product_variants v JOIN products p ON p.id = v.product_id JOIN categories c ON c.id = p.category_id
		WHERE v.id = ANY($1::text[]::uuid[]) ORDER BY v.id FOR UPDATE OF v`, ids))
}

func (q *Queries) AdjustStock(ctx context.Context, variantID string, delta int) error {
	_, err := q.db.Exec(ctx, `UPDATE product_variants SET stock = stock + $2, updated_at = now() WHERE id = $1`, variantID, delta)
	return err
}

func (q *Queries) VariantSKUTaken(ctx context.Context, sku, excludeVariantID string) (bool, error) {
	var n int
	err := q.db.QueryRow(ctx, `SELECT count(*) FROM product_variants WHERE sku = $1 AND ($2 = '' OR id::text <> $2)`, sku, excludeVariantID).Scan(&n)
	return n > 0, err
}

// ---------- product images ----------

type ProductImage struct {
	ID       string            `db:"id"`
	FileID   string            `db:"file_id"`
	Position int               `db:"position"`
	Variants map[string]string `db:"variants"`
}

func (q *Queries) ListProductImages(ctx context.Context, productID string) ([]ProductImage, error) {
	return list[ProductImage](q.db.Query(ctx, `SELECT pi.id, pi.file_id, pi.position, f.variants
		FROM product_images pi JOIN files f ON f.id = pi.file_id WHERE pi.product_id = $1 ORDER BY pi.position, pi.id`, productID))
}

func (q *Queries) SetProductImages(ctx context.Context, productID string, fileIDs []string) error {
	if _, err := q.db.Exec(ctx, `DELETE FROM product_images WHERE product_id = $1`, productID); err != nil {
		return err
	}
	for i, id := range fileIDs {
		if _, err := q.db.Exec(ctx, `INSERT INTO product_images (product_id, file_id, position) VALUES ($1, $2, $3)`, productID, id, i); err != nil {
			return err
		}
	}
	return nil
}
