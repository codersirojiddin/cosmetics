package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/apperr"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/store"
)

type Catalog struct{ s *Services }

// ---------- storefront views ----------

type CategoryRef struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type CategoryView struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	Slug         string     `json:"slug"`
	Position     int        `json:"position"`
	IsActive     bool       `json:"isActive"`
	ProductCount int        `json:"productCount"`
	Image        *ImageURLs `json:"image"`
}

type ProductCard struct {
	ID           string      `json:"id"`
	Slug         string      `json:"slug"`
	Name         string      `json:"name"`
	Brand        string      `json:"brand"`
	Category     CategoryRef `json:"category"`
	IsNew        bool        `json:"isNew"`
	IsFeatured   bool        `json:"isFeatured"`
	PriceFrom    int64       `json:"priceFrom"`
	OldPriceFrom *int64      `json:"oldPriceFrom"`
	InStock      bool        `json:"inStock"`
	HasVariants  bool        `json:"hasVariants"`
	Image        *ImageURLs  `json:"image"`
}

type PublicVariant struct {
	ID       string     `json:"id"`
	Label    string     `json:"label"`
	Price    int64      `json:"price"`
	OldPrice *int64     `json:"oldPrice"`
	InStock  bool       `json:"inStock"`
	Image    *ImageURLs `json:"image"`
}

type ProductDetail struct {
	ID          string          `json:"id"`
	Slug        string          `json:"slug"`
	Name        string          `json:"name"`
	Brand       string          `json:"brand"`
	Description string          `json:"description"`
	Ingredients string          `json:"ingredients"`
	HowToUse    string          `json:"howToUse"`
	OptionLabel string          `json:"optionLabel"`
	Category    CategoryRef     `json:"category"`
	IsNew       bool            `json:"isNew"`
	PriceFrom   int64           `json:"priceFrom"`
	InStock     bool            `json:"inStock"`
	Images      []ImageURLs     `json:"images"`
	Variants    []PublicVariant `json:"variants"`
}

type ListParams struct {
	Q, Category, Sort string
	Featured, New     bool
	Page, Limit       int
}

func (c *Catalog) Categories(ctx context.Context, onlyActive bool) ([]CategoryView, error) {
	cats, err := c.s.d.Store.ListCategories(ctx, onlyActive)
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, ct := range cats {
		if ct.ImageFileID != nil {
			ids = append(ids, *ct.ImageFileID)
		}
	}
	files := map[string]store.File{}
	if len(ids) > 0 {
		if files, err = c.s.d.Store.GetFiles(ctx, ids); err != nil {
			return nil, err
		}
	}
	out := make([]CategoryView, 0, len(cats))
	for _, ct := range cats {
		v := CategoryView{ID: ct.ID, Name: ct.Name, Slug: ct.Slug, Position: ct.Position, IsActive: ct.IsActive, ProductCount: ct.ProductCount}
		if ct.ImageFileID != nil {
			if f, ok := files[*ct.ImageFileID]; ok {
				v.Image = c.s.imageURLs(f.Variants)
			}
		}
		out = append(out, v)
	}
	return out, nil
}

func (c *Catalog) ListPublic(ctx context.Context, p ListParams) (*Page[ProductCard], error) {
	page, limit, off := Paging(p.Page, p.Limit, 12, 48)
	rows, err := c.s.d.Store.ListProducts(ctx, store.ProductFilter{Q: strings.TrimSpace(p.Q), CategorySlug: p.Category, Public: true,
		Featured: p.Featured, New: p.New, Sort: p.Sort, Limit: limit, Offset: off})
	if err != nil {
		return nil, err
	}
	out := &Page[ProductCard]{Items: make([]ProductCard, 0, len(rows)), Page: page, Limit: limit}
	for _, r := range rows {
		out.Items = append(out.Items, ProductCard{ID: r.ID, Slug: r.Slug, Name: r.Name, Brand: r.Brand,
			Category: CategoryRef{Name: r.CategoryName, Slug: r.CategorySlug}, IsNew: r.IsNew, IsFeatured: r.IsFeatured,
			PriceFrom: r.PriceFrom, OldPriceFrom: r.OldPriceFrom, InStock: r.InStock, HasVariants: r.VariantCount > 1,
			Image: c.s.imageURLs(r.ImageVariant)})
	}
	if len(rows) > 0 {
		out.Total = rows[0].Total
	}
	return out, nil
}

func (c *Catalog) GetPublic(ctx context.Context, slug string) (*ProductDetail, error) {
	st := c.s.d.Store
	notFound := apperr.NotFound("Mahsulot topilmadi")
	p, err := st.GetProductBySlug(ctx, slug)
	if errors.Is(err, store.ErrNotFound) {
		return nil, notFound
	}
	if err != nil {
		return nil, err
	}
	cat, err := st.GetCategory(ctx, p.CategoryID)
	if err != nil {
		return nil, err
	}
	if p.Status != "active" || !cat.IsActive {
		return nil, notFound
	}
	vars, err := st.ListVariants(ctx, p.ID, true)
	if err != nil {
		return nil, err
	}
	if len(vars) == 0 {
		return nil, notFound
	}
	imgs, err := st.ListProductImages(ctx, p.ID)
	if err != nil {
		return nil, err
	}
	d := &ProductDetail{ID: p.ID, Slug: p.Slug, Name: p.Name, Brand: p.Brand, Description: p.Description, Ingredients: p.Ingredients,
		HowToUse: p.HowToUse, OptionLabel: p.OptionLabel, Category: CategoryRef{Name: cat.Name, Slug: cat.Slug}, IsNew: p.IsNew,
		Images: []ImageURLs{}, Variants: make([]PublicVariant, 0, len(vars))}
	byFile := map[string]*ImageURLs{}
	for _, im := range imgs {
		u := c.s.imageURLs(im.Variants)
		byFile[im.FileID] = u
		d.Images = append(d.Images, *u)
	}
	for i, v := range vars {
		pv := PublicVariant{ID: v.ID, Label: v.Label, Price: v.Price, OldPrice: v.OldPrice, InStock: v.Stock > 0}
		if v.ImageFileID != nil {
			pv.Image = byFile[*v.ImageFileID]
		}
		if i == 0 || v.Price < d.PriceFrom {
			d.PriceFrom = v.Price
		}
		d.InStock = d.InStock || v.Stock > 0
		d.Variants = append(d.Variants, pv)
	}
	return d, nil
}

// ---------- admin: categories ----------

type CategoryInput struct {
	Name     string `json:"name" validate:"required,min=2,max=100"`
	ImageID  string `json:"imageId" validate:"omitempty,uuid"`
	Position int    `json:"position" validate:"gte=0,lte=10000"`
	IsActive *bool  `json:"isActive"`
}

func (c *Catalog) uniqueSlug(ctx context.Context, base string, taken func(context.Context, string) (bool, error)) (string, error) {
	slug := base
	for i := 2; i < 30; i++ {
		t, err := taken(ctx, slug)
		if err != nil {
			return "", err
		}
		if !t {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
	return base + "-" + sha256Hex(randToken(8))[:6], nil
}

func (c *Catalog) checkImage(ctx context.Context, id, kind, field string) (*string, error) {
	if id == "" {
		return nil, nil
	}
	f, err := c.s.d.Store.GetFile(ctx, id)
	if errors.Is(err, store.ErrNotFound) || (err == nil && f.Kind != kind) {
		return nil, apperr.Field(field, "Rasm topilmadi, qaytadan yuklang")
	}
	if err != nil {
		return nil, err
	}
	return &f.ID, nil
}

func (c *Catalog) CreateCategory(ctx context.Context, actor *store.User, in CategoryInput) (*CategoryView, error) {
	st := c.s.d.Store
	img, err := c.checkImage(ctx, in.ImageID, "category_image", "imageId")
	if err != nil {
		return nil, err
	}
	slug, err := c.uniqueSlug(ctx, slugify(in.Name), func(ctx context.Context, s string) (bool, error) { return st.CategorySlugTaken(ctx, s, "") })
	if err != nil {
		return nil, err
	}
	cat := &store.Category{Name: strings.TrimSpace(in.Name), Slug: slug, ImageFileID: img, Position: in.Position, IsActive: in.IsActive == nil || *in.IsActive}
	if err := st.InsertCategory(ctx, cat); err != nil {
		return nil, err
	}
	c.s.audit(ctx, actor, "category.created", "category", cat.ID, map[string]any{"name": cat.Name})
	return c.categoryView(ctx, cat.ID)
}

func (c *Catalog) categoryView(ctx context.Context, id string) (*CategoryView, error) {
	all, err := c.Categories(ctx, false)
	if err != nil {
		return nil, err
	}
	for i := range all {
		if all[i].ID == id {
			return &all[i], nil
		}
	}
	return nil, apperr.NotFound("Kategoriya topilmadi")
}

func (c *Catalog) UpdateCategory(ctx context.Context, actor *store.User, id string, in CategoryInput) (*CategoryView, error) {
	st := c.s.d.Store
	cat, err := st.GetCategory(ctx, id)
	if errors.Is(err, store.ErrNotFound) || store.IsInvalidText(err) {
		return nil, apperr.NotFound("Kategoriya topilmadi")
	}
	if err != nil {
		return nil, err
	}
	img, err := c.checkImage(ctx, in.ImageID, "category_image", "imageId")
	if err != nil {
		return nil, err
	}
	cat.Name, cat.ImageFileID, cat.Position = strings.TrimSpace(in.Name), img, in.Position
	if in.IsActive != nil {
		cat.IsActive = *in.IsActive
	}
	if err := st.UpdateCategory(ctx, cat); err != nil {
		return nil, err
	}
	c.s.audit(ctx, actor, "category.updated", "category", id, nil)
	return c.categoryView(ctx, id)
}

func (c *Catalog) DeleteCategory(ctx context.Context, actor *store.User, id string) error {
	err := c.s.d.Store.DeleteCategory(ctx, id)
	switch {
	case errors.Is(err, store.ErrNotFound) || store.IsInvalidText(err):
		return apperr.NotFound("Kategoriya topilmadi")
	case store.IsForeignKey(err):
		return apperr.Conflict("category_not_empty", "Kategoriyada mahsulotlar bor. Avval ularni boshqa kategoriyaga ko'chiring")
	case err != nil:
		return err
	}
	c.s.audit(ctx, actor, "category.deleted", "category", id, nil)
	return nil
}

// ---------- admin: products ----------

type VariantInput struct {
	ID       string `json:"id" validate:"omitempty,uuid"`
	Label    string `json:"label" validate:"max=80"`
	SKU      string `json:"sku" validate:"max=64"`
	Price    int64  `json:"price" validate:"required,gt=0,lte=1000000000"`
	OldPrice *int64 `json:"oldPrice" validate:"omitempty,gt=0,lte=1000000000"`
	Stock    int    `json:"stock" validate:"gte=0,lte=1000000"`
	ImageID  string `json:"imageId" validate:"omitempty,uuid"`
	IsActive *bool  `json:"isActive"`
}

type ProductInput struct {
	CategoryID  string         `json:"categoryId" validate:"required,uuid"`
	Name        string         `json:"name" validate:"required,min=2,max=200"`
	Brand       string         `json:"brand" validate:"max=100"`
	Description string         `json:"description" validate:"max=10000"`
	Ingredients string         `json:"ingredients" validate:"max=5000"`
	HowToUse    string         `json:"howToUse" validate:"max=5000"`
	OptionLabel string         `json:"optionLabel" validate:"max=40"`
	Status      string         `json:"status" validate:"required,oneof=draft active archived"`
	IsNew       bool           `json:"isNew"`
	IsFeatured  bool           `json:"isFeatured"`
	ImageIDs    []string       `json:"imageIds" validate:"max=8,dive,uuid"`
	Variants    []VariantInput `json:"variants" validate:"required,min=1,max=30,dive"`
}

type AdminVariant struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	SKU      string `json:"sku"`
	Price    int64  `json:"price"`
	OldPrice *int64 `json:"oldPrice"`
	Stock    int    `json:"stock"`
	ImageID  string `json:"imageId"`
	IsActive bool   `json:"isActive"`
}

type AdminProduct struct {
	ID          string         `json:"id"`
	Slug        string         `json:"slug"`
	CategoryID  string         `json:"categoryId"`
	Category    CategoryRef    `json:"category"`
	Name        string         `json:"name"`
	Brand       string         `json:"brand"`
	Description string         `json:"description"`
	Ingredients string         `json:"ingredients"`
	HowToUse    string         `json:"howToUse"`
	OptionLabel string         `json:"optionLabel"`
	Status      string         `json:"status"`
	IsNew       bool           `json:"isNew"`
	IsFeatured  bool           `json:"isFeatured"`
	Images      []ImageView    `json:"images"`
	Variants    []AdminVariant `json:"variants"`
	CreatedAt   time.Time      `json:"createdAt"`
	UpdatedAt   time.Time      `json:"updatedAt"`
}

type AdminProductRow struct {
	ID           string      `json:"id"`
	Slug         string      `json:"slug"`
	Name         string      `json:"name"`
	Brand        string      `json:"brand"`
	Status       string      `json:"status"`
	Category     CategoryRef `json:"category"`
	PriceFrom    int64       `json:"priceFrom"`
	StockTotal   int         `json:"stockTotal"`
	VariantCount int         `json:"variantCount"`
	IsNew        bool        `json:"isNew"`
	IsFeatured   bool        `json:"isFeatured"`
	Image        *ImageURLs  `json:"image"`
	UpdatedAt    time.Time   `json:"updatedAt"`
}

type AdminListParams struct {
	Q, Status, CategoryID string
	Page, Limit           int
}

func (c *Catalog) AdminList(ctx context.Context, p AdminListParams) (*Page[AdminProductRow], error) {
	page, limit, off := Paging(p.Page, p.Limit, 20, 100)
	rows, err := c.s.d.Store.ListProducts(ctx, store.ProductFilter{Q: strings.TrimSpace(p.Q), Status: p.Status, CategoryID: p.CategoryID, Limit: limit, Offset: off, Sort: "new"})
	if err != nil {
		return nil, err
	}
	out := &Page[AdminProductRow]{Items: make([]AdminProductRow, 0, len(rows)), Page: page, Limit: limit}
	for _, r := range rows {
		out.Items = append(out.Items, AdminProductRow{ID: r.ID, Slug: r.Slug, Name: r.Name, Brand: r.Brand, Status: r.Status,
			Category: CategoryRef{Name: r.CategoryName, Slug: r.CategorySlug}, PriceFrom: r.PriceFrom, StockTotal: r.StockTotal,
			VariantCount: r.VariantCount, IsNew: r.IsNew, IsFeatured: r.IsFeatured, Image: c.s.imageURLs(r.ImageVariant), UpdatedAt: r.UpdatedAt})
	}
	if len(rows) > 0 {
		out.Total = rows[0].Total
	}
	return out, nil
}

func (c *Catalog) AdminGet(ctx context.Context, id string) (*AdminProduct, error) {
	st := c.s.d.Store
	p, err := st.GetProductByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) || store.IsInvalidText(err) {
		return nil, apperr.NotFound("Mahsulot topilmadi")
	}
	if err != nil {
		return nil, err
	}
	cat, err := st.GetCategory(ctx, p.CategoryID)
	if err != nil {
		return nil, err
	}
	vars, err := st.ListVariants(ctx, id, false)
	if err != nil {
		return nil, err
	}
	imgs, err := st.ListProductImages(ctx, id)
	if err != nil {
		return nil, err
	}
	ap := &AdminProduct{ID: p.ID, Slug: p.Slug, CategoryID: p.CategoryID, Category: CategoryRef{Name: cat.Name, Slug: cat.Slug}, Name: p.Name,
		Brand: p.Brand, Description: p.Description, Ingredients: p.Ingredients, HowToUse: p.HowToUse, OptionLabel: p.OptionLabel,
		Status: p.Status, IsNew: p.IsNew, IsFeatured: p.IsFeatured, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
		Images: make([]ImageView, 0, len(imgs)), Variants: make([]AdminVariant, 0, len(vars))}
	for _, im := range imgs {
		ap.Images = append(ap.Images, ImageView{ID: im.FileID, URLs: *c.s.imageURLs(im.Variants)})
	}
	for _, v := range vars {
		av := AdminVariant{ID: v.ID, Label: v.Label, Price: v.Price, OldPrice: v.OldPrice, Stock: v.Stock, IsActive: v.IsActive}
		if v.SKU != nil {
			av.SKU = *v.SKU
		}
		if v.ImageFileID != nil {
			av.ImageID = *v.ImageFileID
		}
		ap.Variants = append(ap.Variants, av)
	}
	return ap, nil
}

// validate applies the business rules that struct tags cannot express.
func (c *Catalog) validate(ctx context.Context, in *ProductInput, existing map[string]bool) error {
	st := c.s.d.Store
	f := map[string]string{}
	in.Name, in.Brand, in.OptionLabel = strings.TrimSpace(in.Name), strings.TrimSpace(in.Brand), strings.TrimSpace(in.OptionLabel)
	in.Description, in.Ingredients, in.HowToUse = cleanMarkdown(in.Description), cleanMarkdown(in.Ingredients), cleanMarkdown(in.HowToUse)

	if _, err := st.GetCategory(ctx, in.CategoryID); errors.Is(err, store.ErrNotFound) {
		f["categoryId"] = "Kategoriya topilmadi"
	} else if err != nil {
		return err
	}

	gallery := map[string]bool{}
	if len(in.ImageIDs) > 0 {
		files, err := st.GetFiles(ctx, in.ImageIDs)
		if err != nil {
			return err
		}
		for _, id := range in.ImageIDs {
			if fl, ok := files[id]; !ok || fl.Kind != "product_image" {
				f["imageIds"] = "Rasm topilmadi, qaytadan yuklang"
			}
			if gallery[id] {
				f["imageIds"] = "Bir rasm ikki marta qo'shilgan"
			}
			gallery[id] = true
		}
	}

	labels, skus := map[string]bool{}, map[string]bool{}
	activeVariants := 0
	for i := range in.Variants {
		v := &in.Variants[i]
		key := func(n string) string { return fmt.Sprintf("variants[%d].%s", i, n) }
		v.Label, v.SKU = strings.TrimSpace(v.Label), strings.TrimSpace(v.SKU)
		if v.ID != "" && !existing[v.ID] {
			f[key("id")] = "Variant bu mahsulotga tegishli emas"
		}
		if v.OldPrice != nil && *v.OldPrice <= v.Price {
			f[key("oldPrice")] = "Eski narx joriy narxdan katta bo'lishi kerak"
		}
		if len(in.Variants) > 1 {
			l := strings.ToLower(v.Label)
			if l == "" {
				f[key("label")] = "Variant nomini yozing"
			} else if labels[l] {
				f[key("label")] = "Variant nomlari takrorlanmasligi kerak"
			}
			labels[l] = true
		}
		if v.SKU != "" {
			taken, err := st.VariantSKUTaken(ctx, v.SKU, v.ID)
			if err != nil {
				return err
			}
			if taken || skus[v.SKU] {
				f[key("sku")] = "Bu SKU band"
			}
			skus[v.SKU] = true
		}
		if v.ImageID != "" && !gallery[v.ImageID] {
			f[key("imageId")] = "Variant rasmi mahsulot rasmlaridan biri bo'lishi kerak"
		}
		if v.IsActive == nil || *v.IsActive {
			activeVariants++
		}
	}
	if len(in.Variants) > 1 && in.OptionLabel == "" {
		in.OptionLabel = "Variant"
	}
	if len(in.Variants) == 1 {
		in.OptionLabel = ""
	}
	if in.Status == "active" {
		if len(in.ImageIDs) == 0 {
			f["imageIds"] = "Faol mahsulotga kamida 1 ta rasm kerak"
		}
		if activeVariants == 0 {
			f["variants"] = "Faol mahsulotga kamida 1 ta faol variant kerak"
		}
	}
	if len(f) > 0 {
		return apperr.Validation(f)
	}
	return nil
}

func (c *Catalog) CreateProduct(ctx context.Context, actor *store.User, in ProductInput) (*AdminProduct, error) {
	st := c.s.d.Store
	if err := c.validate(ctx, &in, map[string]bool{}); err != nil {
		return nil, err
	}
	slug, err := c.uniqueSlug(ctx, slugify(in.Name), func(ctx context.Context, s string) (bool, error) { return st.ProductSlugTaken(ctx, s, "") })
	if err != nil {
		return nil, err
	}
	p := &store.Product{CategoryID: in.CategoryID, Name: in.Name, Slug: slug, Brand: in.Brand, Description: in.Description, Ingredients: in.Ingredients,
		HowToUse: in.HowToUse, OptionLabel: in.OptionLabel, Status: in.Status, IsNew: in.IsNew, IsFeatured: in.IsFeatured, CreatedBy: &actor.ID}
	err = st.InTx(ctx, func(q *store.Queries) error {
		if err := q.InsertProduct(ctx, p); err != nil {
			return err
		}
		if err := q.SetProductImages(ctx, p.ID, in.ImageIDs); err != nil {
			return err
		}
		return c.writeVariants(ctx, q, p.ID, in.Variants, nil)
	})
	if err != nil {
		return nil, c.mapWriteErr(err)
	}
	c.s.audit(ctx, actor, "product.created", "product", p.ID, map[string]any{"name": p.Name})
	return c.AdminGet(ctx, p.ID)
}

func (c *Catalog) UpdateProduct(ctx context.Context, actor *store.User, id string, in ProductInput) (*AdminProduct, error) {
	st := c.s.d.Store
	p, err := st.GetProductByID(ctx, id)
	if errors.Is(err, store.ErrNotFound) || store.IsInvalidText(err) {
		return nil, apperr.NotFound("Mahsulot topilmadi")
	}
	if err != nil {
		return nil, err
	}
	existingVars, err := st.ListVariants(ctx, id, false)
	if err != nil {
		return nil, err
	}
	existing := map[string]bool{}
	for _, v := range existingVars {
		existing[v.ID] = true
	}
	if err := c.validate(ctx, &in, existing); err != nil {
		return nil, err
	}
	p.CategoryID, p.Name, p.Brand, p.Description, p.Ingredients, p.HowToUse = in.CategoryID, in.Name, in.Brand, in.Description, in.Ingredients, in.HowToUse
	p.OptionLabel, p.Status, p.IsNew, p.IsFeatured = in.OptionLabel, in.Status, in.IsNew, in.IsFeatured
	err = st.InTx(ctx, func(q *store.Queries) error {
		if err := q.UpdateProduct(ctx, p); err != nil {
			return err
		}
		if err := q.SetProductImages(ctx, id, in.ImageIDs); err != nil {
			return err
		}
		return c.writeVariants(ctx, q, id, in.Variants, existing)
	})
	if err != nil {
		return nil, c.mapWriteErr(err)
	}
	c.s.audit(ctx, actor, "product.updated", "product", id, map[string]any{"status": p.Status})
	return c.AdminGet(ctx, id)
}

// writeVariants reconciles the stored variants with the request: update by id, insert new,
// and remove the rest (deactivated instead when orders reference them).
func (c *Catalog) writeVariants(ctx context.Context, q *store.Queries, productID string, in []VariantInput, existing map[string]bool) error {
	keep := map[string]bool{}
	for i, vi := range in {
		v := &store.Variant{ID: vi.ID, ProductID: productID, Label: vi.Label, Price: vi.Price, OldPrice: vi.OldPrice, Stock: vi.Stock, Position: i,
			IsActive: vi.IsActive == nil || *vi.IsActive}
		if vi.SKU != "" {
			s := vi.SKU
			v.SKU = &s
		}
		if vi.ImageID != "" {
			s := vi.ImageID
			v.ImageFileID = &s
		}
		if vi.ID != "" {
			keep[vi.ID] = true
			if err := q.UpdateVariant(ctx, v); err != nil {
				return err
			}
		} else if err := q.InsertVariant(ctx, v); err != nil {
			return err
		}
	}
	for id := range existing {
		if !keep[id] {
			if err := q.RemoveVariant(ctx, id); err != nil {
				return err
			}
		}
	}
	return nil
}

func (c *Catalog) mapWriteErr(err error) error {
	switch {
	case store.IsUnique(err, "sku"):
		return apperr.Field("variants", "SKU band")
	case store.IsUnique(err, "slug"):
		return apperr.Conflict("slug_taken", "Bu nomdagi mahsulot allaqachon bor, nomni biroz o'zgartiring")
	}
	return err
}

// Archive hides a product from the storefront without deleting it (orders keep referencing it).
func (c *Catalog) Archive(ctx context.Context, actor *store.User, id string) error {
	err := c.s.d.Store.SetProductStatus(ctx, id, "archived")
	if errors.Is(err, store.ErrNotFound) || store.IsInvalidText(err) {
		return apperr.NotFound("Mahsulot topilmadi")
	}
	if err != nil {
		return err
	}
	c.s.audit(ctx, actor, "product.archived", "product", id, nil)
	return nil
}
