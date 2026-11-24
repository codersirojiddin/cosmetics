package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/store"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/tg"
)

// Notifier sends Telegram messages. It is best-effort: failures are logged and never
// affect the order flow.
type Notifier struct{ s *Services }

func (n *Notifier) run(name string, fn func(ctx context.Context) error) {
	if n.s.d.TG == nil {
		return
	}
	do := func() {
		defer func() {
			if r := recover(); r != nil {
				n.s.d.Log.Error("notifier panic", "job", name, "panic", r)
			}
		}()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := fn(ctx); err != nil {
			n.s.d.Log.Warn("notification failed", "job", name, "err", err)
		}
	}
	if n.s.d.NotifySync {
		do()
	} else {
		go do()
	}
}

// esc escapes the only three characters Telegram's HTML parse mode cares about.
var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

func esc(s string) string { return escaper.Replace(s) }

func (n *Notifier) orderData(ctx context.Context, orderID string) (*store.Order, []store.OrderItem, error) {
	st := n.s.d.Store
	o, err := st.GetOrder(ctx, orderID)
	if err != nil {
		return nil, nil, err
	}
	items, err := st.ListOrderItems(ctx, []string{orderID})
	return o, items, err
}

func (n *Notifier) adminText(ctx context.Context, o *store.Order, items []store.OrderItem, claimedBy string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "🆕 <b>Yangi buyurtma #%d</b>\n\n", o.Number)
	fmt.Fprintf(&b, "👤 %s\n📞 %s\n💰 <b>%s so'm</b>\n", esc(o.CustomerName), esc(o.Phone), formatMoney(o.TotalAmount))
	b.WriteString("\n📦 Mahsulotlar:\n")
	for _, it := range items {
		name := it.ProductName
		if it.VariantLabel != "" {
			name += " (" + it.VariantLabel + ")"
		}
		fmt.Fprintf(&b, " • %s × %d\n", esc(name), it.Quantity)
	}
	fmt.Fprintf(&b, "\n📍 %s\n", esc(o.AddressText))
	if o.ReceiptDuplicateOf != nil {
		kind := "o'xshash"
		if o.ReceiptDuplicateKind != nil && *o.ReceiptDuplicateKind == "exact" {
			kind = "aynan bir xil"
		}
		if d, err := n.s.d.Store.GetOrder(ctx, *o.ReceiptDuplicateOf); err == nil {
			fmt.Fprintf(&b, "\n⚠️ <b>Diqqat:</b> bu chek #%d buyurtmadagi chek bilan %s!\n", d.Number, kind)
		}
	}
	if claimedBy != "" {
		fmt.Fprintf(&b, "\n✅ <b>Olgan:</b> %s", esc(claimedBy))
	}
	return b.String()
}

func (n *Notifier) adminKeyboard(o *store.Order, claimed bool) tg.Keyboard {
	var kb tg.Keyboard
	if !claimed {
		kb = append(kb, []tg.Button{{Text: "✋ Men olaman", CallbackData: "claim:" + o.ID}})
	}
	// Telegram only accepts public https URLs in buttons.
	if web := n.s.d.Cfg.WebURL; strings.HasPrefix(web, "https://") {
		kb = append(kb, []tg.Button{{Text: "🔗 Panelda ochish", URL: web + "/admin/orders/" + o.ID}})
	}
	return kb
}

// NewOrder tells every notifiable admin about a new order and confirms receipt to the customer.
func (n *Notifier) NewOrder(orderID string) {
	n.run("new_order", func(ctx context.Context) error {
		o, items, err := n.orderData(ctx, orderID)
		if err != nil {
			return err
		}
		staff, err := n.s.d.Store.ListNotifiableStaff(ctx)
		if err != nil {
			return err
		}
		text := n.adminText(ctx, o, items, "")
		kb := &tg.SendOpts{Keyboard: n.adminKeyboard(o, false)}
		for _, u := range staff {
			mid, err := n.s.d.TG.SendMessage(ctx, u.TelegramID, text, kb)
			if err != nil {
				n.s.d.Log.Warn("admin notification failed", "admin", u.ID, "err", err)
				continue
			}
			if err := n.s.d.Store.SaveAdminMessage(ctx, o.ID, u.TelegramID, mid); err != nil {
				n.s.d.Log.Warn("save admin message failed", "err", err)
			}
		}
		_, err = n.s.d.TG.SendMessage(ctx, o.CustomerTelegram,
			fmt.Sprintf("✅ <b>Buyurtmangiz qabul qilindi #%d</b>\n\nAdmin chekni tekshirib, tez orada siz bilan bog'lanadi.", o.Number), nil)
		return err
	})
}

// OrderClaimed updates the admins' messages so everyone sees who took the order.
func (n *Notifier) OrderClaimed(orderID string) {
	n.run("order_claimed", func(ctx context.Context) error {
		o, items, err := n.orderData(ctx, orderID)
		if err != nil || o.AssignedAdminID == nil {
			return err
		}
		msgs, err := n.s.d.Store.ListAdminMessages(ctx, orderID)
		if err != nil {
			return err
		}
		text := n.adminText(ctx, o, items, o.AssignedAdminName)
		opts := &tg.SendOpts{Keyboard: n.adminKeyboard(o, true)}
		for _, m := range msgs {
			if err := n.s.d.TG.EditMessage(ctx, m.ChatID, m.MessageID, text, opts); err != nil {
				var ae *tg.APIError
				if errors.As(err, &ae) && ae.Code == 400 { // "message is not modified" etc.
					continue
				}
				n.s.d.Log.Warn("edit admin message failed", "err", err)
			}
		}
		return nil
	})
}

// StatusChanged notifies the customer about progress.
func (n *Notifier) StatusChanged(orderID string) {
	n.run("status_changed", func(ctx context.Context) error {
		o, err := n.s.d.Store.GetOrder(ctx, orderID)
		if err != nil {
			return err
		}
		var text string
		switch o.Status {
		case "confirmed":
			text = fmt.Sprintf("✅ <b>Buyurtma #%d tasdiqlandi</b>\n\nTo'lovingiz qabul qilindi. Tez orada jo'natiladi.", o.Number)
		case "shipping":
			text = fmt.Sprintf("🚚 <b>Buyurtma #%d jo'natildi</b>\n\nMahsulot yo'lda, tez orada yetkaziladi.", o.Number)
		case "delivered":
			text = fmt.Sprintf("🎉 <b>Buyurtma #%d yetkazib berildi</b>\n\nXaridingiz uchun rahmat!", o.Number)
		case "rejected":
			text = fmt.Sprintf("❌ <b>Buyurtma #%d rad etildi</b>\n\nSabab: %s", o.Number, esc(o.RejectReason))
			if p, err := n.s.Settings.Public(ctx); err == nil && p.ShopPhone != "" {
				text += "\n\nSavollar uchun: " + esc(p.ShopPhone)
			}
		default:
			return nil
		}
		_, err = n.s.d.TG.SendMessage(ctx, o.CustomerTelegram, text, nil)
		return err
	})
}
