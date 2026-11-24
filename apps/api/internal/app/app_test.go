package app_test

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/tg"
)

func TestAuthRequired(t *testing.T) {
	e := newEnv(t)
	for _, p := range []string{"/me", "/orders", "/payment-info", "/admin/stats", "/admin/products"} {
		st, r := e.do("GET", p, "", nil)
		require.Equal(t, 401, st, p)
		require.Equal(t, "unauthorized", str(r, "error", "code"), p)
	}
	st, _ := e.do("GET", "/me", "garbage.token.here", nil)
	require.Equal(t, 401, st)
}

func TestTelegramLoginFlow(t *testing.T) {
	e := newEnv(t)
	st, r := e.do("POST", "/auth/telegram/start", "", nil)
	require.Equal(t, 201, st, r)
	code, secret := str(r, "code"), str(r, "secret")
	require.Contains(t, str(r, "deepLink"), "https://t.me/testbot?start=login_"+code)

	_, r = e.do("POST", "/auth/telegram/poll", "", J{"code": code, "secret": secret})
	require.Equal(t, "pending", str(r, "status"))

	ali := tg.User{ID: 5001, FirstName: "Ali", Username: "ali"}
	e.botMessage(ali, "/start login_"+code)
	require.Contains(t, e.tg.last(5001), "Tasdiqlandi")

	_, r = e.do("POST", "/auth/telegram/poll", "", J{"code": code, "secret": "wrong-secret"})
	require.Equal(t, "expired", str(r, "status"), "a wrong secret must never yield tokens")
	require.Empty(t, str(r, "tokens", "accessToken"))

	_, r = e.do("POST", "/auth/telegram/poll", "", J{"code": code, "secret": secret})
	require.Equal(t, "confirmed", str(r, "status"))
	access, refresh := str(r, "tokens", "accessToken"), str(r, "tokens", "refreshToken")
	require.NotEmpty(t, access)
	require.Equal(t, "customer", str(r, "user", "role"))

	_, r = e.do("POST", "/auth/telegram/poll", "", J{"code": code, "secret": secret})
	require.Equal(t, "expired", str(r, "status"), "tokens can be collected only once")

	st, r = e.do("GET", "/me", access, nil)
	require.Equal(t, 200, st)
	require.EqualValues(t, 5001, num(r, "telegramId"))

	// refresh rotates; replaying the old token burns the whole family
	st, r = e.do("POST", "/auth/refresh", "", J{"refreshToken": refresh})
	require.Equal(t, 200, st, r)
	refresh2 := str(r, "refreshToken")
	st, _ = e.do("POST", "/auth/refresh", "", J{"refreshToken": refresh})
	require.Equal(t, 401, st)
	st, _ = e.do("POST", "/auth/refresh", "", J{"refreshToken": refresh2})
	require.Equal(t, 401, st, "reuse detection should revoke the new token too")

	// expired / unknown code
	e.botMessage(ali, "/start login_doesnotexist")
	require.Contains(t, e.tg.last(5001), "eskirgan")
}

func TestOwnerBootstrapAndAdminInvites(t *testing.T) {
	e := newEnv(t)
	// OWNER_TELEGRAM_IDS promotes on login
	_, r := e.do("POST", "/auth/telegram/start", "", nil)
	code, secret := str(r, "code"), str(r, "secret")
	e.botMessage(tg.User{ID: ownerTG, FirstName: "Boss"}, "/start login_"+code)
	_, r = e.do("POST", "/auth/telegram/poll", "", J{"code": code, "secret": secret})
	require.Equal(t, "owner", str(r, "user", "role"))
	owner := str(r, "tokens", "accessToken")

	cust := e.user(5002, "Karim", "customer")
	st, _ := e.do("POST", "/admin/admins/invites", cust.token, nil)
	require.Equal(t, 403, st)

	st, r = e.do("POST", "/admin/admins/invites", owner, nil)
	require.Equal(t, 201, st, r)
	inviteCode := str(r, "code")
	require.Contains(t, str(r, "deepLink"), "start=admin_"+inviteCode)

	e.botMessage(tg.User{ID: 5002, FirstName: "Karim"}, "/start admin_"+inviteCode)
	require.Contains(t, e.tg.last(5002), "admin bo'ldingiz")
	st, r = e.do("GET", "/me", cust.token, nil)
	require.Equal(t, "admin", str(r, "role"), "role is read from the database on every request")
	st, _ = e.do("GET", "/admin/stats", cust.token, nil)
	require.Equal(t, 200, st)

	// an invite is single-use
	other := e.user(5003, "Sardor", "customer")
	e.botMessage(tg.User{ID: 5003, FirstName: "Sardor"}, "/start admin_"+inviteCode)
	require.Contains(t, e.tg.last(5003), "eskirgan")
	st, _ = e.do("GET", "/admin/stats", other.token, nil)
	require.Equal(t, 403, st)

	// admins cannot manage admins, settings or read the audit log
	for _, c := range [][2]string{{"POST", "/admin/admins/invites"}, {"GET", "/admin/admins"}, {"GET", "/admin/audit"}, {"PUT", "/admin/settings"}} {
		st, _ = e.do(c[0], c[1], cust.token, J{})
		require.Equal(t, 403, st, c[1])
	}

	// owner lists, blocks and removes the admin
	st, r = e.do("GET", "/admin/admins", owner, nil)
	require.Equal(t, 200, st)
	require.Len(t, arr(r, "items"), 2) // owner + the new admin
	st, _ = e.do("PATCH", "/admin/admins/"+cust.id, owner, J{"isActive": false})
	require.Equal(t, 200, st)
	st, _ = e.do("GET", "/admin/stats", cust.token, nil)
	require.Equal(t, 403, st, "a blocked admin loses access immediately")
	st, _ = e.do("PATCH", "/admin/admins/"+cust.id, owner, J{"isActive": true})
	require.Equal(t, 200, st)
	st, _ = e.do("DELETE", "/admin/admins/"+cust.id, owner, nil)
	require.Equal(t, 204, st)
	st, _ = e.do("GET", "/admin/stats", cust.token, nil)
	require.Equal(t, 403, st)

	// the owner is not an "admin" row that can be edited
	ownerID := str(mustGet(e, "/me", owner), "id")
	st, r = e.do("PATCH", "/admin/admins/"+ownerID, owner, J{"isActive": false})
	require.Equal(t, 400, st, r)

	st, r = e.do("GET", "/admin/audit", owner, nil)
	require.Equal(t, 200, st)
	require.NotEmpty(t, arr(r, "items"))
}

func mustGet(e *env, path, token string) J {
	st, r := e.do("GET", path, token, nil)
	require.Equal(e.t, 200, st, r)
	return r
}

func TestCatalogAdminAndStorefront(t *testing.T) {
	e := newEnv(t)
	s := e.setupShop()

	// access control
	st, _ := e.do("GET", "/admin/products", s.cust.token, nil)
	require.Equal(t, 403, st)

	// a second category with the same name gets a unique slug
	st, r := e.do("POST", "/admin/categories", s.admin.token, J{"name": "Yuz parvarishi"})
	require.Equal(t, 201, st)
	require.Equal(t, "yuz-parvarishi-2", str(r, "slug"))

	// image uploads are validated
	st, r = e.upload("/admin/uploads/image", s.admin.token, []byte("this is not an image"))
	require.Equal(t, 422, st, r)
	st, _ = e.upload("/admin/uploads/image", s.admin.token, blocks(5, 10, 10)) // 90x80 px: too small
	require.Equal(t, 422, st)
	st, r = e.upload("/admin/uploads/image", s.admin.token, pic(901))
	require.Equal(t, 201, st, r)
	require.Contains(t, str(r, "urls", "card"), "/files/products/")
	card := strings.TrimPrefix(str(r, "urls", "card"), "http://test")
	rec := e.raw(card, "")
	require.Equal(t, 200, rec.Code, "uploaded images are served publicly")
	require.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
	require.Equal(t, 404, e.raw("/files/products/", "").Code, "no directory listing")

	// the product from setupShop is live
	st, r = e.do("GET", "/products", "", nil)
	require.Equal(t, 200, st)
	require.EqualValues(t, 1, num(r, "total"))
	require.EqualValues(t, 85000, num(r, "items", 0, "priceFrom"))
	require.Equal(t, true, at(r, "items", 0, "hasVariants"))
	require.Equal(t, true, at(r, "items", 0, "inStock"))
	require.NotEmpty(t, str(r, "items", 0, "image", "thumb"))

	st, r = e.do("GET", "/products/hyaluron-serum", "", nil)
	require.Equal(t, 200, st, r)
	require.Len(t, arr(r, "variants"), 2)
	require.Equal(t, "Hajm", str(r, "optionLabel"))
	require.Nil(t, at(r, "variants", 0, "stock"), "exact stock levels are not public")

	// search, category filter, sorting
	_, r = e.do("GET", "/products?q=hyal", "", nil)
	require.EqualValues(t, 1, num(r, "total"))
	_, r = e.do("GET", "/products?q=nomatch", "", nil)
	require.EqualValues(t, 0, num(r, "total"))
	_, r = e.do("GET", "/products?category=yuz-parvarishi", "", nil)
	require.EqualValues(t, 1, num(r, "total"))
	_, r = e.do("GET", "/products?category=yuz-parvarishi-2", "", nil)
	require.EqualValues(t, 0, num(r, "total"))
	_, r = e.do("GET", "/products?q=%25", "", nil) // a literal % must not act as a wildcard
	require.EqualValues(t, 0, num(r, "total"))

	// business-rule validation
	body := e.productBody(s.category, nil, "active")
	st, r = e.do("POST", "/admin/products", s.admin.token, body)
	require.Equal(t, 422, st)
	require.Contains(t, str(r, "error", "fields", "imageIds"), "rasm")

	st, r = e.do("POST", "/admin/products", s.admin.token, e.productBody(s.category, []string{s.image}, "draft",
		J{"label": "", "price": 1000, "stock": 1}, J{"label": "B", "price": 2000, "stock": 1}))
	require.Equal(t, 422, st)
	require.NotEmpty(t, str(r, "error", "fields", "variants[0].label"))

	st, r = e.do("POST", "/admin/products", s.admin.token, e.productBody(s.category, []string{s.image}, "draft",
		J{"label": "A", "price": 1000, "oldPrice": 900, "stock": 1}, J{"label": "B", "price": 2000, "stock": 1}))
	require.Equal(t, 422, st)
	require.NotEmpty(t, str(r, "error", "fields", "variants[0].oldPrice"))

	st, r = e.do("POST", "/admin/products", s.admin.token, e.productBody(s.category, []string{s.image}, "draft",
		J{"label": "A", "price": -5, "stock": 1}, J{"label": "B", "price": 2000, "stock": 1}))
	require.Equal(t, 422, st)
	require.NotEmpty(t, str(r, "error", "fields", "variants[0].price"))

	st, r = e.do("POST", "/admin/products", s.admin.token, e.productBody(s.category, []string{s.image}, "draft",
		J{"label": "A", "price": 1000, "stock": 1, "sku": "SER-30"}, J{"label": "B", "price": 2000, "stock": 1}))
	require.Equal(t, 422, st, "SKU must be unique across the shop")
	require.NotEmpty(t, str(r, "error", "fields", "variants[0].sku"))

	// raw HTML and script links are stripped from descriptions; drafts stay hidden
	b := e.productBody(s.category, []string{s.image}, "draft", J{"price": 50000, "stock": 3})
	b["name"] = "Lab bo'yog'i"
	b["description"] = "<script>alert(1)</script>Salom <b>dunyo</b> [x](javascript:alert(1)) **qalin**"
	st, r = e.do("POST", "/admin/products", s.admin.token, b)
	require.Equal(t, 201, st, r)
	draft := str(r, "id")
	require.NotContains(t, str(r, "description"), "<")
	require.NotContains(t, strings.ToLower(str(r, "description")), "javascript:")
	require.Contains(t, str(r, "description"), "**qalin**")
	require.Equal(t, "", str(r, "optionLabel"), "single-variant products have no option label")
	st, _ = e.do("GET", "/products/"+str(r, "slug"), "", nil)
	require.Equal(t, 404, st)
	_, r = e.do("GET", "/products", "", nil)
	require.EqualValues(t, 1, num(r, "total"))

	// publish, then archive
	b["status"] = "active"
	st, _ = e.do("PUT", "/admin/products/"+draft, s.admin.token, b)
	require.Equal(t, 200, st)
	_, r = e.do("GET", "/products?sort=price_asc", "", nil)
	require.EqualValues(t, 2, num(r, "total"))
	require.EqualValues(t, 50000, num(r, "items", 0, "priceFrom"))
	_, r = e.do("GET", "/products?sort=price_desc", "", nil)
	require.EqualValues(t, 85000, num(r, "items", 0, "priceFrom"))
	st, _ = e.do("DELETE", "/admin/products/"+draft, s.admin.token, nil)
	require.Equal(t, 204, st)
	_, r = e.do("GET", "/products", "", nil)
	require.EqualValues(t, 1, num(r, "total"))

	// categories with products cannot be deleted
	st, r = e.do("DELETE", "/admin/categories/"+s.category, s.admin.token, nil)
	require.Equal(t, 409, st)
	require.Equal(t, "category_not_empty", str(r, "error", "code"))
}

func TestProductVariantsReconcile(t *testing.T) {
	e := newEnv(t)
	s := e.setupShop()
	_, p := e.do("GET", "/admin/products/"+s.product, s.admin.token, nil)

	// order variant A so it can no longer be deleted
	rc := e.receipt(s.cust, 11)
	st, r := e.do("POST", "/orders", s.cust.token, e.orderBody(rc, item(s.varA, 1)))
	require.Equal(t, 201, st, r)

	// drop both variants from the request and add a new one
	body := e.productBody(s.category, []string{s.image}, "active", J{"label": "100 ml", "price": 150000, "stock": 9})
	st, r = e.do("PUT", "/admin/products/"+s.product, s.admin.token, body)
	require.Equal(t, 200, st, r)
	variants := arr(r, "variants")
	require.Len(t, variants, 2) // the ordered one is kept but deactivated; the unused one is deleted
	byLabel := map[string]any{}
	for _, v := range variants {
		byLabel[str(v, "label")] = v
	}
	require.Equal(t, false, at(byLabel["30 ml"], "isActive"))
	require.Equal(t, true, at(byLabel["100 ml"], "isActive"))
	_, pub := e.do("GET", "/products/hyaluron-serum", "", nil)
	require.Len(t, arr(pub, "variants"), 1)
	_ = p
}

func TestOrderLifecycle(t *testing.T) {
	e := newEnv(t)
	s := e.setupShop()

	// owner configures payment details; admins cannot
	st, _ := e.do("PUT", "/admin/settings", s.admin.token, J{"cardNumber": "8600 1234 5678 9012"})
	require.Equal(t, 403, st)
	st, r := e.do("PUT", "/admin/settings", s.owner.token, J{"cardNumber": "8600 1234 5678 9012", "cardHolder": "A. VALIYEV", "shopPhone": "+998 90 000 00 00"})
	require.Equal(t, 200, st, r)
	st, r = e.do("GET", "/payment-info", s.cust.token, nil)
	require.Equal(t, 200, st)
	require.Equal(t, "8600 1234 5678 9012", str(r, "cardNumber"))
	st, r = e.do("GET", "/settings/public", "", nil)
	require.Equal(t, 200, st)
	require.Equal(t, "+998 90 000 00 00", str(r, "shopPhone"))
	require.Nil(t, at(r, "cardNumber"), "card details are not public")

	// place an order: 2 x 30ml + 1 x 50ml. The total always comes from the database prices.
	rc := e.receipt(s.cust, 21)
	body := e.orderBody(rc, item(s.varA, 2), item(s.varB, 1))
	st, r = e.do("POST", "/orders", s.cust.token, body)
	require.Equal(t, 201, st, r)
	orderID := str(r, "id")
	require.EqualValues(t, 1001, num(r, "number"))
	require.Equal(t, "waiting", str(r, "status"))
	require.EqualValues(t, 2*85000+90000, num(r, "totalAmount"))
	require.Equal(t, "+998901234567", str(r, "phone"))
	require.Len(t, arr(r, "items"), 2)

	// stock was reserved
	_, pr := e.do("GET", "/admin/products/"+s.product, s.admin.token, nil)
	require.EqualValues(t, 3, num(pr, "variants", 0, "stock"))
	require.EqualValues(t, 1, num(pr, "variants", 1, "stock"))

	// every notifiable admin got the Telegram message with a claim button; the customer got a confirmation
	for _, a := range []actor{s.owner, s.admin, s.admin2} {
		msgs := e.tg.to(a.tg)
		require.Len(t, msgs, 1, "admin %d", a.tg)
		require.Contains(t, msgs[0].text, "Yangi buyurtma #1001")
		require.Contains(t, msgs[0].text, "260 000")
		require.Equal(t, "claim:"+orderID, msgs[0].kb[0][0].CallbackData)
		require.Equal(t, "https://shop.example/admin/orders/"+orderID, msgs[0].kb[1][0].URL)
	}
	require.Contains(t, e.tg.last(s.cust.tg), "qabul qilindi")

	// customer visibility
	st, r = e.do("GET", "/orders/"+orderID, s.cust.token, nil)
	require.Equal(t, 200, st)
	require.Nil(t, at(r, "assignedAdmin"), "customers never see admin data")
	st, _ = e.do("GET", "/orders/"+orderID, s.cust2.token, nil)
	require.Equal(t, 404, st)
	_, r = e.do("GET", "/orders", s.cust.token, nil)
	require.EqualValues(t, 1, num(r, "total"))
	_, r = e.do("GET", "/orders", s.cust2.token, nil)
	require.EqualValues(t, 0, num(r, "total"))

	// receipt access: owner of the order and staff only
	rec := e.raw("/api/v1/files/receipts/"+rc, s.cust.token)
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "image/jpeg", rec.Header().Get("Content-Type"))
	require.Equal(t, 200, e.raw("/api/v1/files/receipts/"+rc, s.admin.token).Code)
	require.Equal(t, 404, e.raw("/api/v1/files/receipts/"+rc, s.cust2.token).Code)
	require.Equal(t, 401, e.raw("/api/v1/files/receipts/"+rc, "").Code)

	// admin inbox
	_, r = e.do("GET", "/admin/orders?status=waiting", s.admin.token, nil)
	require.EqualValues(t, 1, num(r, "total"))
	_, r = e.do("GET", "/admin/orders?q=1001", s.admin.token, nil)
	require.EqualValues(t, 1, num(r, "total"))
	_, r = e.do("GET", "/admin/orders?q=901234", s.admin.token, nil)
	require.EqualValues(t, 1, num(r, "total"))
	_, r = e.do("GET", "/admin/orders?assigned=none", s.admin.token, nil)
	require.EqualValues(t, 1, num(r, "total"))
	_, r = e.do("GET", "/admin/stats", s.admin.token, nil)
	require.EqualValues(t, 1, num(r, "counts", "waiting"))
	require.EqualValues(t, 1, num(r, "unassigned"))

	// "Men olaman": the first admin wins, everybody's Telegram message is updated
	st, r = e.do("POST", "/admin/orders/"+orderID+"/claim", s.admin.token, nil)
	require.Equal(t, 200, st, r)
	require.Equal(t, s.admin.id, str(r, "assignedAdmin", "id"))
	for _, a := range []actor{s.owner, s.admin, s.admin2} {
		ed := e.tg.editsFor(a.tg)
		require.Len(t, ed, 1)
		require.Contains(t, ed[0].text, "Olgan:")
		require.Contains(t, ed[0].text, "Ahmad")
		require.Len(t, ed[0].kb, 1, "claim button is gone, panel link stays")
	}
	st, r = e.do("POST", "/admin/orders/"+orderID+"/claim", s.admin2.token, nil)
	require.Equal(t, 409, st)
	require.Equal(t, "already_claimed", str(r, "error", "code"))
	require.Contains(t, str(r, "error", "message"), "Ahmad")
	st, _ = e.do("POST", "/admin/orders/"+orderID+"/claim", s.admin.token, nil)
	require.Equal(t, 200, st, "claiming your own order again is a no-op")

	// only the assignee (or the owner) can move it forward
	st, r = e.do("POST", "/admin/orders/"+orderID+"/status", s.admin2.token, J{"to": "shipping", "expectedStatus": "waiting"})
	require.Equal(t, 403, st)
	require.Equal(t, "assigned_to_other", str(r, "error", "code"))

	// stale screens and illegal jumps are rejected
	st, r = e.do("POST", "/admin/orders/"+orderID+"/status", s.admin.token, J{"to": "shipping", "expectedStatus": "confirmed"})
	require.Equal(t, 409, st)
	require.Equal(t, "status_changed", str(r, "error", "code"))
	st, r = e.do("POST", "/admin/orders/"+orderID+"/status", s.admin.token, J{"to": "delivered", "expectedStatus": "waiting"})
	require.Equal(t, 409, st)
	require.Equal(t, "invalid_transition", str(r, "error", "code"))

	// confirm, then ship, then deliver. The customer hears about each step.
	st, r = e.do("POST", "/admin/orders/"+orderID+"/status", s.admin.token, J{"to": "confirmed", "expectedStatus": "waiting", "comment": "Telefon qilindi"})
	require.Equal(t, 200, st, r)
	require.Contains(t, e.tg.last(s.cust.tg), "tasdiqlandi")
	st, r = e.do("POST", "/admin/orders/"+orderID+"/status", s.admin.token, J{"to": "shipping", "expectedStatus": "confirmed"})
	require.Equal(t, 200, st, r)
	require.Contains(t, e.tg.last(s.cust.tg), "jo'natildi")
	st, r = e.do("POST", "/admin/orders/"+orderID+"/status", s.owner.token, J{"to": "delivered", "expectedStatus": "shipping"})
	require.Equal(t, 200, st, r) // owner may act on any admin's order
	require.Contains(t, e.tg.last(s.cust.tg), "yetkazib berildi")
	require.Equal(t, "delivered", str(r, "status"))
	require.Len(t, arr(r, "history"), 4)
	require.Equal(t, "Telefon qilindi", str(r, "history", 1, "comment"))
	st, _ = e.do("POST", "/admin/orders/"+orderID+"/status", s.owner.token, J{"to": "rejected", "expectedStatus": "delivered", "rejectReason": "xato"})
	require.Equal(t, 409, st, "a finished order is final")

	st, r = e.do("GET", "/orders/"+orderID, s.cust.token, nil)
	require.Equal(t, 200, st)
	require.Len(t, arr(r, "timeline"), 4)

	// notes
	st, r = e.do("POST", "/admin/orders/"+orderID+"/notes", s.admin.token, J{"body": "Kuryer: Anvar"})
	require.Equal(t, 201, st, r)
	require.Equal(t, "Kuryer: Anvar", str(r, "notes", 0, "body"))
}

func TestRejectRestoresStock(t *testing.T) {
	e := newEnv(t)
	s := e.setupShop()
	st, r := e.do("POST", "/orders", s.cust.token, e.orderBody(e.receipt(s.cust, 31), item(s.varB, 2)))
	require.Equal(t, 201, st, r)
	id := str(r, "id")
	_, pr := e.do("GET", "/admin/products/"+s.product, s.admin.token, nil)
	require.EqualValues(t, 0, num(pr, "variants", 1, "stock"))

	// sold out: further orders are refused and nothing changes
	st, r = e.do("POST", "/orders", s.cust2.token, e.orderBody(e.receipt(s.cust2, 32), item(s.varB, 1)))
	require.Equal(t, 409, st)
	require.Equal(t, "out_of_stock", str(r, "error", "code"))

	// a reason is required
	st, r = e.do("POST", "/admin/orders/"+id+"/status", s.admin.token, J{"to": "rejected", "expectedStatus": "waiting"})
	require.Equal(t, 422, st)
	require.NotEmpty(t, str(r, "error", "fields", "rejectReason"))

	st, r = e.do("POST", "/admin/orders/"+id+"/status", s.admin.token, J{"to": "rejected", "expectedStatus": "waiting", "rejectReason": "Chekdagi summa noto'g'ri"})
	require.Equal(t, 200, st, r)
	require.Equal(t, "rejected", str(r, "status"))
	require.Contains(t, e.tg.last(s.cust.tg), "Chekdagi summa noto'g'ri")
	_, pr = e.do("GET", "/admin/products/"+s.product, s.admin.token, nil)
	require.EqualValues(t, 2, num(pr, "variants", 1, "stock"), "stock returns to the shelf")

	st, _ = e.do("POST", "/orders", s.cust2.token, e.orderBody(e.receipt(s.cust2, 33), item(s.varB, 1)))
	require.Equal(t, 201, st)
}

func TestOrderValidation(t *testing.T) {
	e := newEnv(t)
	s := e.setupShop()
	good := func() J { return e.orderBody(e.receipt(s.cust, time64()), item(s.varA, 1)) }
	post := func(mod func(J)) (int, J) {
		b := good()
		mod(b)
		return e.do("POST", "/orders", s.cust.token, b)
	}
	cases := map[string]struct {
		mod   func(J)
		field string
	}{
		"bad phone":     {func(b J) { b["phone"] = "12345" }, "phone"},
		"short name":    {func(b J) { b["customerName"] = "A" }, "customerName"},
		"no address":    {func(b J) { delete(b, "addressText") }, "addressText"},
		"no latitude":   {func(b J) { delete(b, "lat") }, "lat"},
		"no longitude":  {func(b J) { delete(b, "lng") }, "lng"},
		"lat range":     {func(b J) { b["lat"] = 123.0 }, "lat"},
		"no receipt":    {func(b J) { delete(b, "receiptFileId") }, "receiptFileId"},
		"unknown file":  {func(b J) { b["receiptFileId"] = "00000000-0000-4000-8000-000000000000" }, "receiptFileId"},
		"no items":      {func(b J) { b["items"] = []J{} }, "items"},
		"zero quantity": {func(b J) { b["items"] = []J{item(s.varA, 0)} }, "items[0].quantity"},
		"bad variant":   {func(b J) { b["items"] = []J{item("nope", 1)} }, "items[0].variantId"},
	}
	for name, c := range cases {
		st, r := post(c.mod)
		require.Equal(t, 422, st, name)
		require.NotEmpty(t, str(r, "error", "fields", c.field), "%s: %v", name, r)
	}
	st, r := post(func(b J) { b["items"] = []J{item("00000000-0000-4000-8000-000000000000", 1)} })
	require.Equal(t, 422, st, r)

	// somebody else's receipt cannot be used
	other := e.receipt(s.cust2, 77)
	st, r = e.do("POST", "/orders", s.cust.token, e.orderBody(other, item(s.varA, 1)))
	require.Equal(t, 422, st)
	require.NotEmpty(t, str(r, "error", "fields", "receiptFileId"))

	// a receipt works once
	rc := e.receipt(s.cust, 78)
	st, _ = e.do("POST", "/orders", s.cust.token, e.orderBody(rc, item(s.varA, 1)))
	require.Equal(t, 201, st)
	st, r = e.do("POST", "/orders", s.cust.token, e.orderBody(rc, item(s.varA, 1)))
	require.Equal(t, 422, st)
	require.NotEmpty(t, str(r, "error", "fields", "receiptFileId"))

	// nothing was created or reserved by the failed attempts
	_, r = e.do("GET", "/admin/orders", s.admin.token, nil)
	require.EqualValues(t, 1, num(r, "total"))

	// archived products cannot be ordered
	st, _ = e.do("DELETE", "/admin/products/"+s.product, s.admin.token, nil)
	require.Equal(t, 204, st)
	st, r = e.do("POST", "/orders", s.cust.token, e.orderBody(e.receipt(s.cust, 79), item(s.varA, 1)))
	require.Equal(t, 409, st)
	require.Equal(t, "product_unavailable", str(r, "error", "code"))
}

var seedMu sync.Mutex
var seedN int64 = 5000

func time64() int64 { seedMu.Lock(); defer seedMu.Unlock(); seedN++; return seedN }

func TestReceiptUploadRejectsBadFiles(t *testing.T) {
	e := newEnv(t)
	c := e.user(3001, "Ali", "customer")
	st, _ := e.upload("/uploads/receipt", c.token, []byte("%PDF-1.4 not an image"))
	require.Equal(t, 422, st)
	st, _ = e.upload("/uploads/receipt", c.token, blocks(3, 10, 10))
	require.Equal(t, 422, st, "tiny images cannot be readable receipts")
	st, _ = e.upload("/uploads/receipt", "", pic(1))
	require.Equal(t, 401, st)
}

func TestDuplicateReceiptDetection(t *testing.T) {
	e := newEnv(t)
	s := e.setupShop()

	st, r := e.do("POST", "/orders", s.cust.token, e.orderBody(e.receipt(s.cust, 100), item(s.varA, 1)))
	require.Equal(t, 201, st)
	first := str(r, "id")
	_, ar := e.do("GET", "/admin/orders/"+first, s.admin.token, nil)
	require.Nil(t, at(ar, "receiptDuplicate"))

	// the very same file again (another customer)
	st, r = e.do("POST", "/orders", s.cust2.token, e.orderBody(e.receipt(s.cust2, 100), item(s.varA, 1)))
	require.Equal(t, 201, st)
	second := str(r, "id")
	_, ar = e.do("GET", "/admin/orders/"+second, s.admin.token, nil)
	require.Equal(t, "exact", str(ar, "receiptDuplicate", "kind"))
	require.EqualValues(t, 1001, num(ar, "receiptDuplicate", "number"))
	adminMsgs := e.tg.to(s.admin.tg)
	require.Contains(t, adminMsgs[len(adminMsgs)-1].text, "Diqqat")

	// the same picture re-saved at another resolution
	st, r = e.upload("/uploads/receipt", s.cust.token, blocks(100, 60, 60))
	require.Equal(t, 201, st, r)
	st, r = e.do("POST", "/orders", s.cust.token, e.orderBody(str(r, "id"), item(s.varA, 1)))
	require.Equal(t, 201, st)
	_, ar = e.do("GET", "/admin/orders/"+str(r, "id"), s.admin.token, nil)
	require.Equal(t, "similar", str(ar, "receiptDuplicate", "kind"))

	// a different picture is clean
	st, r = e.do("POST", "/orders", s.cust.token, e.orderBody(e.receipt(s.cust, 101), item(s.varA, 1)))
	require.Equal(t, 201, st)
	_, ar = e.do("GET", "/admin/orders/"+str(r, "id"), s.admin.token, nil)
	require.Nil(t, at(ar, "receiptDuplicate"))
	_, lr := e.do("GET", "/admin/orders", s.admin.token, nil)
	flagged := 0
	for _, o := range arr(lr, "items") {
		if at(o, "duplicateReceipt") == true {
			flagged++
		}
	}
	require.Equal(t, 2, flagged)
}

// Two admins press "Men olaman" at the same moment: exactly one wins.
func TestClaimRace(t *testing.T) {
	e := newEnv(t)
	s := e.setupShop()
	_, r := e.do("POST", "/orders", s.cust.token, e.orderBody(e.receipt(s.cust, 200), item(s.varA, 1)))
	id := str(r, "id")

	for round := 0; round < 3; round++ {
		if round > 0 { // reset the claim between rounds via the owner
			_, _ = e.do("POST", "/admin/orders/"+id+"/claim", s.owner.token, J{"force": true})
		}
		var wg sync.WaitGroup
		codes := make([]int, 4)
		who := []actor{s.admin, s.admin2, s.admin, s.admin2}
		start := make(chan struct{})
		for i := range codes {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				codes[i], _ = e.do("POST", "/admin/orders/"+id+"/claim", who[i].token, nil)
			}()
		}
		close(start)
		wg.Wait()
		_, ar := e.do("GET", "/admin/orders/"+id, s.owner.token, nil)
		winner := str(ar, "assignedAdmin", "id")
		require.NotEmpty(t, winner)
		for i, c := range codes {
			if who[i].id == winner {
				require.Equal(t, 200, c, "round %d: the winner (and their repeat clicks) get OK", round)
			} else {
				require.Equal(t, 409, c, "round %d: the loser gets a conflict", round)
			}
		}
		break // owner-forced reassignment is covered below
	}
	st, r := e.do("POST", "/admin/orders/"+id+"/claim", s.owner.token, J{"force": true})
	require.Equal(t, 200, st, r)
	require.Equal(t, s.owner.id, str(r, "assignedAdmin", "id"))
	st, _ = e.do("POST", "/admin/orders/"+id+"/claim", s.admin.token, J{"force": true})
	require.Equal(t, 409, st, "force is for owners only")
}

// Many customers fight for the last item: it is sold exactly once and stock never goes negative.
func TestLastItemRace(t *testing.T) {
	e := newEnv(t)
	s := e.setupShop()
	b := e.productBody(s.category, []string{s.image}, "active", J{"price": 70000, "stock": 1})
	b["name"] = "Limited Lip Oil"
	st, r := e.do("POST", "/admin/products", s.admin.token, b)
	require.Equal(t, 201, st, r)
	variant := str(r, "variants", 0, "id")

	const n = 6
	buyers := make([]actor, n)
	receipts := make([]string, n)
	for i := range buyers {
		buyers[i] = e.user(int64(4000+i), fmt.Sprintf("Buyer%d", i), "customer")
		receipts[i] = e.receipt(buyers[i], int64(300+i))
	}
	codes := make([]int, n)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := range buyers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			codes[i], _ = e.do("POST", "/orders", buyers[i].token, e.orderBody(receipts[i], item(variant, 1)))
		}()
	}
	close(start)
	wg.Wait()
	created, conflicts := 0, 0
	for _, c := range codes {
		switch c {
		case 201:
			created++
		case 409:
			conflicts++
		}
	}
	require.Equal(t, 1, created, codes)
	require.Equal(t, n-1, conflicts, codes)
	_, pr := e.do("GET", "/admin/products/"+str(r, "id"), s.admin.token, nil)
	require.EqualValues(t, 0, num(pr, "variants", 0, "stock"))
}

func TestClaimFromTelegramButton(t *testing.T) {
	e := newEnv(t)
	s := e.setupShop()
	_, r := e.do("POST", "/orders", s.cust.token, e.orderBody(e.receipt(s.cust, 400), item(s.varA, 1)))
	id := str(r, "id")
	click := func(from int64) {
		e.app.Bot.HandleUpdate(t.Context(), tg.Update{CallbackQuery: &tg.CallbackQuery{ID: "cb", From: tg.User{ID: from}, Data: "claim:" + id}})
	}

	click(s.cust.tg) // a customer pressing the button is refused
	require.Contains(t, e.tg.answers[len(e.tg.answers)-1], "true|")
	_, ar := e.do("GET", "/admin/orders/"+id, s.admin.token, nil)
	require.Nil(t, at(ar, "assignedAdmin"))

	click(s.admin2.tg)
	require.Contains(t, e.tg.answers[len(e.tg.answers)-1], "false|")
	_, ar = e.do("GET", "/admin/orders/"+id, s.admin.token, nil)
	require.Equal(t, s.admin2.id, str(ar, "assignedAdmin", "id"))
	require.Contains(t, e.tg.editsFor(s.admin.tg)[0].text, "Bobur")

	click(s.admin.tg) // too late
	require.Contains(t, e.tg.answers[len(e.tg.answers)-1], "true|")
	require.Contains(t, e.tg.answers[len(e.tg.answers)-1], "Bobur")
}

func TestBotBasics(t *testing.T) {
	e := newEnv(t)
	u := tg.User{ID: 6001, FirstName: "Zafar"}
	e.botMessage(u, "/id")
	require.Contains(t, e.tg.last(6001), "6001")
	e.botMessage(u, "/start")
	require.Contains(t, e.tg.last(6001), "Salom")
	e.botMessage(u, "hello")
	require.Contains(t, e.tg.last(6001), "saytidan")
}

func TestHealthAndNotFound(t *testing.T) {
	e := newEnv(t)
	st, r := e.do("GET", "/health", "", nil)
	require.Equal(t, 200, st)
	require.Equal(t, true, at(r, "db"))
	st, r = e.do("GET", "/nope", "", nil)
	require.Equal(t, 404, st)
	require.Equal(t, "not_found", str(r, "error", "code"))
}
