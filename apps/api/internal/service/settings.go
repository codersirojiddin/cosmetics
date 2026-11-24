package service

import (
	"context"
	"regexp"
	"strings"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/apperr"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/store"
)

type Settings struct{ s *Services }

type SettingsView struct {
	ShopName    string `json:"shopName"`
	ShopPhone   string `json:"shopPhone"`
	CardNumber  string `json:"cardNumber"`
	CardHolder  string `json:"cardHolder"`
	PaymentNote string `json:"paymentNote"`
}

type PublicSettings struct {
	ShopName  string `json:"shopName"`
	ShopPhone string `json:"shopPhone"`
}

type PaymentInfo struct {
	CardNumber  string `json:"cardNumber"`
	CardHolder  string `json:"cardHolder"`
	PaymentNote string `json:"paymentNote"`
	ShopPhone   string `json:"shopPhone"`
}

type SettingsInput struct {
	ShopName    *string `json:"shopName" validate:"omitempty,max=100"`
	ShopPhone   *string `json:"shopPhone" validate:"omitempty,max=30"`
	CardNumber  *string `json:"cardNumber" validate:"omitempty,max=32"`
	CardHolder  *string `json:"cardHolder" validate:"omitempty,max=100"`
	PaymentNote *string `json:"paymentNote" validate:"omitempty,max=500"`
}

var cardRe = regexp.MustCompile(`^[0-9 \-]{8,32}$`)

func (s *Settings) all(ctx context.Context) (*SettingsView, error) {
	m, err := s.s.d.Store.GetSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &SettingsView{ShopName: m["shop_name"], ShopPhone: m["shop_phone"], CardNumber: m["card_number"], CardHolder: m["card_holder"], PaymentNote: m["payment_note"]}, nil
}

// Full is for staff (card details are shown, but only the owner may change them).
func (s *Settings) Full(ctx context.Context) (*SettingsView, error) { return s.all(ctx) }

func (s *Settings) Public(ctx context.Context) (*PublicSettings, error) {
	v, err := s.all(ctx)
	if err != nil {
		return nil, err
	}
	return &PublicSettings{ShopName: v.ShopName, ShopPhone: v.ShopPhone}, nil
}

// Payment is shown to logged-in customers on checkout only.
func (s *Settings) Payment(ctx context.Context) (*PaymentInfo, error) {
	v, err := s.all(ctx)
	if err != nil {
		return nil, err
	}
	return &PaymentInfo{CardNumber: v.CardNumber, CardHolder: v.CardHolder, PaymentNote: v.PaymentNote, ShopPhone: v.ShopPhone}, nil
}

func (s *Settings) Update(ctx context.Context, actor *store.User, in SettingsInput) (*SettingsView, error) {
	if err := requireOwner(actor); err != nil {
		return nil, err
	}
	kv := map[string]string{}
	set := func(key string, v *string) {
		if v != nil {
			kv[key] = strings.TrimSpace(*v)
		}
	}
	set("shop_name", in.ShopName)
	set("shop_phone", in.ShopPhone)
	set("card_number", in.CardNumber)
	set("card_holder", in.CardHolder)
	set("payment_note", in.PaymentNote)
	if c, ok := kv["card_number"]; ok && c != "" && !cardRe.MatchString(c) {
		return nil, apperr.Field("cardNumber", "Karta raqami noto'g'ri")
	}
	if err := s.s.d.Store.SetSettings(ctx, kv, actor.ID); err != nil {
		return nil, err
	}
	changed := make([]string, 0, len(kv))
	for k := range kv {
		changed = append(changed, k)
	}
	meta := map[string]any{"keys": changed}
	if c, ok := kv["card_number"]; ok {
		meta["cardLast4"] = last4(c)
	}
	s.s.audit(ctx, actor, "settings.updated", "settings", "", meta)
	return s.all(ctx)
}

func last4(s string) string {
	d := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
	if len(d) > 4 {
		return d[len(d)-4:]
	}
	return d
}
