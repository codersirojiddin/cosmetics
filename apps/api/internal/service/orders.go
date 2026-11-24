package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/apperr"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/imgx"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/store"
)

type Orders struct{ s *Services }

// ---------- state machine ----------

// Allowed transitions. waiting -> shipping is allowed on purpose: an admin may ship
// straight away without a separate "confirmed" step.
var transitions = map[string][]string{
	"waiting":   {"confirmed", "shipping", "rejected"},
	"confirmed": {"shipping", "rejected"},
	"shipping":  {"delivered"},
}

func CanTransition(from, to string) bool {
	for _, t := range transitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

func IsTerminal(status string) bool { return status == "delivered" || status == "rejected" }

// ---------- inputs ----------

type OrderItemInput struct {
	VariantID string `json:"variantId" validate:"required,uuid"`
	Quantity  int    `json:"quantity" validate:"required,min=1,max=20"`
}

type CreateOrderInput struct {
	Items         []OrderItemInput `json:"items" validate:"required,min=1,max=20,dive"`
	CustomerName  string           `json:"customerName" validate:"required,min=2,max=100"`
	Phone         string           `json:"phone" validate:"required,max=30"`
	AddressText   string           `json:"addressText" validate:"required,min=5,max=300"`
	Lat           *float64         `json:"lat" validate:"required,gte=-90,lte=90"`
	Lng           *float64         `json:"lng" validate:"required,gte=-180,lte=180"`
	ReceiptFileID string           `json:"receiptFileId" validate:"required,uuid"`
	Comment       string           `json:"comment" validate:"max=500"`
}

type ChangeStatusInput struct {
	To             string `json:"to" validate:"required,oneof=confirmed shipping delivered rejected"`
	ExpectedStatus string `json:"expectedStatus" validate:"required,oneof=waiting confirmed shipping delivered rejected"`
	Comment        string `json:"comment" validate:"max=500"`
	RejectReason   string `json:"rejectReason" validate:"max=500"`
}

type AdminOrderQuery struct {
	Status, Assigned, Q string
	Page, Limit         int
}

// ---------- views ----------

type ItemView struct {
	ProductID    string     `json:"productId"`
	VariantID    string     `json:"variantId"`
	ProductName  string     `json:"productName"`
	VariantLabel string     `json:"variantLabel"`
	UnitPrice    int64      `json:"unitPrice"`
	Quantity     int        `json:"quantity"`
	LineTotal    int64      `json:"lineTotal"`
	Image        *ImageURLs `json:"image"`
}

type TimelineEntry struct {
	Status string    `json:"status"`
	At     time.Time `json:"at"`
}

type OrderView struct {
	ID            string          `json:"id"`
	Number        int64           `json:"number"`
	Status        string          `json:"status"`
	CustomerName  string          `json:"customerName"`
	Phone         string          `json:"phone"`
	AddressText   string          `json:"addressText"`
	Lat           float64         `json:"lat"`
	Lng           float64         `json:"lng"`
	Comment       string          `json:"comment"`
	TotalAmount   int64           `json:"totalAmount"`
	Items         []ItemView      `json:"items"`
	ReceiptFileID string          `json:"receiptFileId"`
	RejectReason  string          `json:"rejectReason"`
	Timeline      []TimelineEntry `json:"timeline"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

type StaffRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type CustomerRef struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	TelegramID int64  `json:"telegramId"`
}

type DuplicateRef struct {
	OrderID string `json:"orderId"`
	Number  int64  `json:"number"`
	Kind    string `json:"kind"` // exact | similar
}

type AdminOrderView struct {
	OrderView
	Customer         CustomerRef        `json:"customer"`
	AssignedAdmin    *StaffRef          `json:"assignedAdmin"`
	ReceiptDuplicate *DuplicateRef      `json:"receiptDuplicate"`
	History          []store.HistoryRow `json:"history"`
	Notes            []store.NoteRow    `json:"notes"`
}

type OrderSummary struct {
	ID               string    `json:"id"`
	Number           int64     `json:"number"`
	Status           string    `json:"status"`
	CustomerName     string    `json:"customerName"`
	Phone            string    `json:"phone"`
	TotalAmount      int64     `json:"totalAmount"`
	ItemCount        int       `json:"itemCount"`
	AssignedAdmin    *StaffRef `json:"assignedAdmin"`
	DuplicateReceipt bool      `json:"duplicateReceipt"`
	CreatedAt        time.Time `json:"createdAt"`
}

type Stats struct {
	Counts     map[string]int `json:"counts"`
	Unassigned int            `json:"unassigned"`
}

func staffRef(id *string, name string) *StaffRef {
	if id == nil {
		return nil
	}
	return &StaffRef{ID: *id, Name: name}
}

func (o *Orders) view(ctx context.Context, ord *store.Order, items []store.OrderItem, hist []store.HistoryRow) (*OrderView, error) {
	var imgIDs []string
	for _, it := range items {
		if it.ImageFileID != nil {
			imgIDs = append(imgIDs, *it.ImageFileID)
		}
	}
	files := map[string]store.File{}
	if len(imgIDs) > 0 {
		var err error
		if files, err = o.s.d.Store.GetFiles(ctx, imgIDs); err != nil {
			return nil, err
		}
	}
	v := &OrderView{
		ID: ord.ID, Number: ord.Number, Status: ord.Status, CustomerName: ord.CustomerName, Phone: ord.Phone,
		AddressText: ord.AddressText, Lat: ord.Lat, Lng: ord.Lng, Comment: ord.CustomerComment, TotalAmount: ord.TotalAmount,
		ReceiptFileID: ord.ReceiptFileID, RejectReason: ord.RejectReason, CreatedAt: ord.CreatedAt, UpdatedAt: ord.UpdatedAt,
		Items: make([]ItemView, 0, len(items)), Timeline: make([]TimelineEntry, 0, len(hist)),
	}
	for _, it := range items {
		iv := ItemView{ProductID: it.ProductID, VariantID: it.VariantID, ProductName: it.ProductName, VariantLabel: it.VariantLabel,
			UnitPrice: it.UnitPrice, Quantity: it.Quantity, LineTotal: it.UnitPrice * int64(it.Quantity)}
		if it.ImageFileID != nil {
			if f, ok := files[*it.ImageFileID]; ok {
				iv.Image = o.s.imageURLs(f.Variants)
			}
		}
		v.Items = append(v.Items, iv)
	}
	for _, h := range hist {
		v.Timeline = append(v.Timeline, TimelineEntry{Status: h.ToStatus, At: h.CreatedAt})
	}
	return v, nil
}

func (o *Orders) loadView(ctx context.Context, q *store.Queries, id string) (*store.Order, *OrderView, []store.HistoryRow, error) {
	ord, err := q.GetOrder(ctx, id)
	if err != nil {
		return nil, nil, nil, err
	}
	items, err := q.ListOrderItems(ctx, []string{id})
	if err != nil {
		return nil, nil, nil, err
	}
	hist, err := q.ListHistory(ctx, id)
	if err != nil {
		return nil, nil, nil, err
	}
	v, err := o.view(ctx, ord, items, hist)
	return ord, v, hist, err
}

func (o *Orders) adminView(ctx context.Context, id string) (*AdminOrderView, error) {
	st := o.s.d.Store
	ord, v, hist, err := o.loadView(ctx, st.Queries, id)
	if errors.Is(err, store.ErrNotFound) || store.IsInvalidText(err) {
		return nil, apperr.NotFound("Buyurtma topilmadi")
	}
	if err != nil {
		return nil, err
	}
	notes, err := st.ListNotes(ctx, id)
	if err != nil {
		return nil, err
	}
	av := &AdminOrderView{OrderView: *v, History: hist, Notes: notes,
		Customer:      CustomerRef{ID: ord.UserID, Username: ord.CustomerUsername, TelegramID: ord.CustomerTelegram},
		AssignedAdmin: staffRef(ord.AssignedAdminID, ord.AssignedAdminName)}
	if ord.ReceiptDuplicateOf != nil && ord.ReceiptDuplicateKind != nil {
		if d, err := st.GetOrder(ctx, *ord.ReceiptDuplicateOf); err == nil {
			av.ReceiptDuplicate = &DuplicateRef{OrderID: d.ID, Number: d.Number, Kind: *ord.ReceiptDuplicateKind}
		}
	}
	return av, nil
}

// ---------- customer ----------

// findDuplicate looks for an earlier order with the same (or a near-identical) receipt image.
func (o *Orders) findDuplicate(ctx context.Context, file *store.File) (*string, *string, error) {
	rows, err := o.s.d.Store.RecentReceiptHashes(ctx, time.Now().AddDate(0, 0, -90))
	if err != nil {
		return nil, nil, err
	}
	var best string
	bestDist := 5 // anything within 4 bits counts as "similar"
	for _, r := range rows {
		if r.SHA256 == file.SHA256 {
			kind := "exact"
			return &r.OrderID, &kind, nil
		}
		if file.DHash != nil && r.DHash != nil {
			if d := imgx.Hamming(uint64(*file.DHash), uint64(*r.DHash)); d < bestDist {
				best, bestDist = r.OrderID, d
			}
		}
	}
	if best != "" {
		kind := "similar"
		return &best, &kind, nil
	}
	return nil, nil, nil
}

func (o *Orders) Create(ctx context.Context, user *store.User, in CreateOrderInput) (*OrderView, error) {
	st := o.s.d.Store
	fields := map[string]string{}
	phone, ok := normalizePhone(in.Phone)
	if !ok {
		fields["phone"] = "Telefon raqami noto'g'ri. Masalan: +998 90 123 45 67"
	}
	name, addr := strings.TrimSpace(in.CustomerName), strings.TrimSpace(in.AddressText)
	if len([]rune(name)) < 2 {
		fields["customerName"] = "Ismni kiriting"
	}
	if len([]rune(addr)) < 5 {
		fields["addressText"] = "Manzilni to'liqroq yozing"
	}
	qty := map[string]int{}
	var ids []string
	for _, it := range in.Items {
		if _, seen := qty[it.VariantID]; !seen {
			ids = append(ids, it.VariantID)
		}
		qty[it.VariantID] += it.Quantity
		if qty[it.VariantID] > 20 {
			fields["items"] = "Bitta mahsulotdan 20 tadan ko'p buyurtma qilib bo'lmaydi"
		}
	}
	file, err := st.GetFile(ctx, in.ReceiptFileID)
	receiptErr := "Chek rasmi topilmadi, qaytadan yuklang"
	switch {
	case errors.Is(err, store.ErrNotFound):
		fields["receiptFileId"] = receiptErr
	case err != nil:
		return nil, err
	case file.Kind != "receipt" || file.OwnerID == nil || *file.OwnerID != user.ID:
		fields["receiptFileId"] = receiptErr
	}
	if len(fields) > 0 {
		return nil, apperr.Validation(fields)
	}
	dupOf, dupKind, err := o.findDuplicate(ctx, file)
	if err != nil {
		return nil, err
	}

	var created *store.Order
	err = st.InTx(ctx, func(q *store.Queries) error {
		locked, err := q.LockVariants(ctx, ids)
		if err != nil {
			return err
		}
		if len(locked) != len(ids) {
			return apperr.Field("items", "Mahsulot topilmadi")
		}
		var total int64
		for _, v := range locked {
			label := v.ProductName
			if v.Label != "" {
				label += " (" + v.Label + ")"
			}
			if v.ProductStatus != "active" || !v.IsActive || !v.CategoryActive {
				return apperr.Conflict("product_unavailable", fmt.Sprintf("«%s» hozir sotuvda yo'q", label))
			}
			if v.Stock < qty[v.ID] {
				return apperr.Conflict("out_of_stock", fmt.Sprintf("«%s» yetarli emas (qolgani: %d ta)", label, v.Stock))
			}
			total += v.Price * int64(qty[v.ID])
		}
		ord := &store.Order{UserID: user.ID, CustomerName: name, Phone: phone, AddressText: addr, Lat: *in.Lat, Lng: *in.Lng,
			CustomerComment: strings.TrimSpace(in.Comment), TotalAmount: total, ReceiptFileID: file.ID,
			ReceiptDuplicateOf: dupOf, ReceiptDuplicateKind: dupKind}
		if err := q.InsertOrder(ctx, ord); err != nil {
			if store.IsUnique(err, "receipt_file_id") {
				return apperr.Field("receiptFileId", "Bu chek allaqachon ishlatilgan")
			}
			return err
		}
		for _, v := range locked {
			if err := q.AdjustStock(ctx, v.ID, -qty[v.ID]); err != nil {
				return err
			}
			img := v.ImageFileID
			if img == nil {
				img = v.ProductImage
			}
			sku := ""
			if v.SKU != nil {
				sku = *v.SKU
			}
			if err := q.InsertOrderItem(ctx, &store.OrderItem{OrderID: ord.ID, ProductID: v.ProductID, VariantID: v.ID, ProductName: v.ProductName,
				VariantLabel: v.Label, SKU: sku, ImageFileID: img, UnitPrice: v.Price, Quantity: qty[v.ID]}); err != nil {
				return err
			}
		}
		if err := q.InsertHistory(ctx, ord.ID, nil, "waiting", &user.ID, ""); err != nil {
			return err
		}
		created = ord
		return nil
	})
	if err != nil {
		return nil, err
	}
	o.s.Notifier.NewOrder(created.ID)
	_, v, _, err := o.loadView(ctx, st.Queries, created.ID)
	return v, err
}

func (o *Orders) ListMine(ctx context.Context, user *store.User, page, limit int) (*Page[OrderView], error) {
	page, limit, off := Paging(page, limit, 20, 50)
	rows, err := o.s.d.Store.ListUserOrders(ctx, user.ID, limit, off)
	if err != nil {
		return nil, err
	}
	out := &Page[OrderView]{Items: []OrderView{}, Page: page, Limit: limit}
	for i := range rows {
		_, v, _, err := o.loadView(ctx, o.s.d.Store.Queries, rows[i].ID)
		if err != nil {
			return nil, err
		}
		out.Items = append(out.Items, *v)
	}
	if len(rows) > 0 {
		out.Total = rows[0].TotalRows
	}
	return out, nil
}

func (o *Orders) GetMine(ctx context.Context, user *store.User, id string) (*OrderView, error) {
	ord, v, _, err := o.loadView(ctx, o.s.d.Store.Queries, id)
	if errors.Is(err, store.ErrNotFound) || store.IsInvalidText(err) || (err == nil && ord.UserID != user.ID) {
		return nil, apperr.NotFound("Buyurtma topilmadi")
	}
	return v, err
}

// ---------- admin ----------

func (o *Orders) AdminList(ctx context.Context, actor *store.User, q AdminOrderQuery) (*Page[OrderSummary], error) {
	page, limit, off := Paging(q.Page, q.Limit, 20, 100)
	rows, err := o.s.d.Store.ListAdminOrders(ctx, store.AdminOrderFilter{Status: q.Status, Assigned: q.Assigned, AdminID: actor.ID, Q: q.Q, Limit: limit, Offset: off})
	if err != nil {
		return nil, err
	}
	out := &Page[OrderSummary]{Items: make([]OrderSummary, 0, len(rows)), Page: page, Limit: limit}
	for _, r := range rows {
		out.Items = append(out.Items, OrderSummary{ID: r.ID, Number: r.Number, Status: r.Status, CustomerName: r.CustomerName, Phone: r.Phone,
			TotalAmount: r.TotalAmount, ItemCount: r.ItemCount, AssignedAdmin: staffRef(r.AssignedAdminID, r.AssignedAdminName),
			DuplicateReceipt: r.ReceiptDuplicateOf != nil, CreatedAt: r.CreatedAt})
	}
	if len(rows) > 0 {
		out.Total = rows[0].TotalRows
	}
	return out, nil
}

func (o *Orders) AdminGet(ctx context.Context, id string) (*AdminOrderView, error) {
	return o.adminView(ctx, id)
}

func (o *Orders) Stats(ctx context.Context) (*Stats, error) {
	counts, un, err := o.s.d.Store.OrderStats(ctx)
	if err != nil {
		return nil, err
	}
	return &Stats{Counts: counts, Unassigned: un}, nil
}

func (o *Orders) staffName(ctx context.Context, id string) string {
	if u, err := o.s.d.Store.GetUser(ctx, id); err == nil {
		return u.DisplayName()
	}
	return "boshqa admin"
}

// Claim is the "Men olaman" action: assigns the order to the admin unless someone else holds it.
func (o *Orders) Claim(ctx context.Context, actor *store.User, id string, force bool) (*AdminOrderView, error) {
	changed := false
	err := o.s.d.Store.InTx(ctx, func(q *store.Queries) error {
		ord, err := q.GetOrderForUpdate(ctx, id)
		if errors.Is(err, store.ErrNotFound) || store.IsInvalidText(err) {
			return apperr.NotFound("Buyurtma topilmadi")
		}
		if err != nil {
			return err
		}
		if IsTerminal(ord.Status) {
			return apperr.Conflict("order_closed", "Buyurtma yakunlangan")
		}
		switch {
		case ord.AssignedAdminID != nil && *ord.AssignedAdminID == actor.ID:
			return nil // already yours
		case ord.AssignedAdminID == nil:
			ok, err := q.ClaimOrder(ctx, id, actor.ID)
			if err != nil {
				return err
			}
			changed = ok
		case force && actor.Role == "owner":
			if err := q.ForceAssign(ctx, id, actor.ID); err != nil {
				return err
			}
			changed = true
		default:
			return apperr.Conflict("already_claimed", fmt.Sprintf("Bu buyurtmani %s allaqachon olgan", o.staffName(ctx, *ord.AssignedAdminID)))
		}
		if changed {
			o.s.audit(ctx, actor, "order.claimed", "order", id, map[string]any{"force": force})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if changed {
		o.s.Notifier.OrderClaimed(id)
	}
	return o.adminView(ctx, id)
}

// ChangeStatus moves an order along the state machine. expectedStatus guards against two admins
// acting on a stale screen; an unclaimed order is claimed by whoever changes it first.
func (o *Orders) ChangeStatus(ctx context.Context, actor *store.User, id string, in ChangeStatusInput) (*AdminOrderView, error) {
	reason := strings.TrimSpace(in.RejectReason)
	if reason == "" {
		reason = strings.TrimSpace(in.Comment)
	}
	if in.To == "rejected" && len([]rune(reason)) < 3 {
		return nil, apperr.Field("rejectReason", "Rad etish sababini yozing")
	}
	claimedNow := false
	err := o.s.d.Store.InTx(ctx, func(q *store.Queries) error {
		ord, err := q.GetOrderForUpdate(ctx, id)
		if errors.Is(err, store.ErrNotFound) || store.IsInvalidText(err) {
			return apperr.NotFound("Buyurtma topilmadi")
		}
		if err != nil {
			return err
		}
		if ord.Status != in.ExpectedStatus {
			return apperr.Conflict("status_changed", fmt.Sprintf("Buyurtma holati o'zgargan (hozir: %s). Sahifani yangilang", ord.Status))
		}
		if !CanTransition(ord.Status, in.To) {
			return apperr.Conflict("invalid_transition", fmt.Sprintf("%s holatidan %s holatiga o'tib bo'lmaydi", ord.Status, in.To))
		}
		if ord.AssignedAdminID != nil && *ord.AssignedAdminID != actor.ID && actor.Role != "owner" {
			return apperr.New(403, "assigned_to_other", fmt.Sprintf("Bu buyurtmani %s olgan", o.staffName(ctx, *ord.AssignedAdminID)))
		}
		if ord.AssignedAdminID == nil {
			if claimedNow, err = q.ClaimOrder(ctx, id, actor.ID); err != nil {
				return err
			}
		}
		rejectReason := ""
		if in.To == "rejected" {
			rejectReason = reason
			items, err := q.ListOrderItems(ctx, []string{id})
			if err != nil {
				return err
			}
			for _, it := range items { // give the reserved stock back
				if err := q.AdjustStock(ctx, it.VariantID, it.Quantity); err != nil {
					return err
				}
			}
		}
		if err := q.UpdateOrderStatus(ctx, id, in.To, rejectReason); err != nil {
			return err
		}
		from := ord.Status
		comment := strings.TrimSpace(in.Comment)
		if in.To == "rejected" {
			comment = reason
		}
		if err := q.InsertHistory(ctx, id, &from, in.To, &actor.ID, comment); err != nil {
			return err
		}
		o.s.audit(ctx, actor, "order.status", "order", id, map[string]any{"from": from, "to": in.To})
		return nil
	})
	if err != nil {
		return nil, err
	}
	if claimedNow {
		o.s.Notifier.OrderClaimed(id)
	}
	o.s.Notifier.StatusChanged(id)
	return o.adminView(ctx, id)
}

func (o *Orders) AddNote(ctx context.Context, actor *store.User, id, body string) (*AdminOrderView, error) {
	body = strings.TrimSpace(body)
	if body == "" || len([]rune(body)) > 1000 {
		return nil, apperr.Field("body", "Izoh 1 dan 1000 gacha belgi bo'lishi kerak")
	}
	if _, err := o.s.d.Store.GetOrder(ctx, id); errors.Is(err, store.ErrNotFound) || store.IsInvalidText(err) {
		return nil, apperr.NotFound("Buyurtma topilmadi")
	} else if err != nil {
		return nil, err
	}
	if err := o.s.d.Store.InsertNote(ctx, id, actor.ID, body); err != nil {
		return nil, err
	}
	return o.adminView(ctx, id)
}
