package api

import (
	"context"
	"io"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/apperr"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/service"
)

func (a *API) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if a.ping != nil {
		if err := a.ping(ctx); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, map[string]any{"status": "degraded", "db": false})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "db": true})
}

// ---------- public ----------

func (a *API) publicSettings(w http.ResponseWriter, r *http.Request) {
	s, err := a.svc.Settings.Public(r.Context())
	a.respond(w, r, http.StatusOK, s, err)
}

func (a *API) respond(w http.ResponseWriter, r *http.Request, status int, v any, err error) {
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, status, v)
}

func (a *API) listCategories(w http.ResponseWriter, r *http.Request) {
	items, err := a.svc.Catalog.Categories(r.Context(), true)
	a.respond(w, r, http.StatusOK, map[string]any{"items": items}, err)
}

func (a *API) listProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p, err := a.svc.Catalog.ListPublic(r.Context(), service.ListParams{Q: q.Get("q"), Category: q.Get("category"), Sort: q.Get("sort"),
		Featured: qBool(r, "featured"), New: qBool(r, "new"), Page: qInt(r, "page"), Limit: qInt(r, "limit")})
	a.respond(w, r, http.StatusOK, p, err)
}

func (a *API) getProduct(w http.ResponseWriter, r *http.Request) {
	p, err := a.svc.Catalog.GetPublic(r.Context(), chi.URLParam(r, "slug"))
	a.respond(w, r, http.StatusOK, p, err)
}

// ---------- auth ----------

func (a *API) loginStart(w http.ResponseWriter, r *http.Request) {
	s, err := a.svc.Auth.StartLogin(r.Context())
	a.respond(w, r, http.StatusCreated, s, err)
}

type pollInput struct {
	Code   string `json:"code" validate:"required,max=64"`
	Secret string `json:"secret" validate:"required,max=128"`
}

func (a *API) loginPoll(w http.ResponseWriter, r *http.Request) {
	in, err := bind[pollInput](a, w, r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	res, err := a.svc.Auth.Poll(r.Context(), in.Code, in.Secret)
	a.respond(w, r, http.StatusOK, res, err)
}

type refreshInput struct {
	RefreshToken string `json:"refreshToken" validate:"required,max=256"`
}

func (a *API) refresh(w http.ResponseWriter, r *http.Request) {
	in, err := bind[refreshInput](a, w, r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	t, err := a.svc.Auth.Refresh(r.Context(), in.RefreshToken)
	a.respond(w, r, http.StatusOK, t, err)
}

func (a *API) logout(w http.ResponseWriter, r *http.Request) {
	in, err := bind[refreshInput](a, w, r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if err := a.svc.Auth.Logout(r.Context(), in.RefreshToken); err != nil {
		a.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------- me / customer ----------

func (a *API) getMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, userFrom(r.Context()))
}

func (a *API) updateMe(w http.ResponseWriter, r *http.Request) {
	in, err := bind[service.UpdateMeInput](a, w, r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	u, err := a.svc.Auth.UpdateMe(r.Context(), userFrom(r.Context()), *in)
	a.respond(w, r, http.StatusOK, u, err)
}

func (a *API) paymentInfo(w http.ResponseWriter, r *http.Request) {
	p, err := a.svc.Settings.Payment(r.Context())
	a.respond(w, r, http.StatusOK, p, err)
}

func (a *API) uploadReceipt(w http.ResponseWriter, r *http.Request) {
	data, err := readFilePart(w, r, a.cfg.MaxUploadBytes)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	f, err := a.svc.Files.UploadReceipt(r.Context(), userFrom(r.Context()), data)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": f.ID, "width": f.Width, "height": f.Height})
}

func (a *API) createOrder(w http.ResponseWriter, r *http.Request) {
	in, err := bind[service.CreateOrderInput](a, w, r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	o, err := a.svc.Orders.Create(r.Context(), userFrom(r.Context()), *in)
	a.respond(w, r, http.StatusCreated, o, err)
}

func (a *API) listMyOrders(w http.ResponseWriter, r *http.Request) {
	p, err := a.svc.Orders.ListMine(r.Context(), userFrom(r.Context()), qInt(r, "page"), qInt(r, "limit"))
	a.respond(w, r, http.StatusOK, p, err)
}

func (a *API) getMyOrder(w http.ResponseWriter, r *http.Request) {
	o, err := a.svc.Orders.GetMine(r.Context(), userFrom(r.Context()), chi.URLParam(r, "id"))
	a.respond(w, r, http.StatusOK, o, err)
}

func (a *API) getReceipt(w http.ResponseWriter, r *http.Request) {
	rc, err := a.svc.Files.OpenReceipt(r.Context(), userFrom(r.Context()), chi.URLParam(r, "id"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	defer rc.Close()
	h := w.Header()
	h.Set("Content-Type", "image/jpeg")
	h.Set("Cache-Control", "private, max-age=3600")
	h.Set("Content-Disposition", "inline")
	_, _ = io.Copy(w, rc)
}

// ---------- admin: orders ----------

func (a *API) adminStats(w http.ResponseWriter, r *http.Request) {
	s, err := a.svc.Orders.Stats(r.Context())
	a.respond(w, r, http.StatusOK, s, err)
}

func (a *API) adminListOrders(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p, err := a.svc.Orders.AdminList(r.Context(), userFrom(r.Context()), service.AdminOrderQuery{
		Status: q.Get("status"), Assigned: q.Get("assigned"), Q: q.Get("q"), Page: qInt(r, "page"), Limit: qInt(r, "limit")})
	a.respond(w, r, http.StatusOK, p, err)
}

func (a *API) adminGetOrder(w http.ResponseWriter, r *http.Request) {
	o, err := a.svc.Orders.AdminGet(r.Context(), chi.URLParam(r, "id"))
	a.respond(w, r, http.StatusOK, o, err)
}

type claimInput struct {
	Force bool `json:"force"`
}

func (a *API) adminClaim(w http.ResponseWriter, r *http.Request) {
	in := claimInput{}
	if r.ContentLength != 0 {
		p, err := bind[claimInput](a, w, r)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		in = *p
	}
	o, err := a.svc.Orders.Claim(r.Context(), userFrom(r.Context()), chi.URLParam(r, "id"), in.Force)
	a.respond(w, r, http.StatusOK, o, err)
}

func (a *API) adminChangeStatus(w http.ResponseWriter, r *http.Request) {
	in, err := bind[service.ChangeStatusInput](a, w, r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	o, err := a.svc.Orders.ChangeStatus(r.Context(), userFrom(r.Context()), chi.URLParam(r, "id"), *in)
	a.respond(w, r, http.StatusOK, o, err)
}

type noteInput struct {
	Body string `json:"body" validate:"required,max=1000"`
}

func (a *API) adminAddNote(w http.ResponseWriter, r *http.Request) {
	in, err := bind[noteInput](a, w, r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	o, err := a.svc.Orders.AddNote(r.Context(), userFrom(r.Context()), chi.URLParam(r, "id"), in.Body)
	a.respond(w, r, http.StatusCreated, o, err)
}

// ---------- admin: catalog ----------

func (a *API) adminListCategories(w http.ResponseWriter, r *http.Request) {
	items, err := a.svc.Catalog.Categories(r.Context(), false)
	a.respond(w, r, http.StatusOK, map[string]any{"items": items}, err)
}

func (a *API) adminCreateCategory(w http.ResponseWriter, r *http.Request) {
	in, err := bind[service.CategoryInput](a, w, r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	c, err := a.svc.Catalog.CreateCategory(r.Context(), userFrom(r.Context()), *in)
	a.respond(w, r, http.StatusCreated, c, err)
}

func (a *API) adminUpdateCategory(w http.ResponseWriter, r *http.Request) {
	in, err := bind[service.CategoryInput](a, w, r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	c, err := a.svc.Catalog.UpdateCategory(r.Context(), userFrom(r.Context()), chi.URLParam(r, "id"), *in)
	a.respond(w, r, http.StatusOK, c, err)
}

func (a *API) adminDeleteCategory(w http.ResponseWriter, r *http.Request) {
	if err := a.svc.Catalog.DeleteCategory(r.Context(), userFrom(r.Context()), chi.URLParam(r, "id")); err != nil {
		a.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) adminListProducts(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	p, err := a.svc.Catalog.AdminList(r.Context(), service.AdminListParams{Q: q.Get("q"), Status: q.Get("status"), CategoryID: q.Get("categoryId"),
		Page: qInt(r, "page"), Limit: qInt(r, "limit")})
	a.respond(w, r, http.StatusOK, p, err)
}

func (a *API) adminCreateProduct(w http.ResponseWriter, r *http.Request) {
	in, err := bind[service.ProductInput](a, w, r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	p, err := a.svc.Catalog.CreateProduct(r.Context(), userFrom(r.Context()), *in)
	a.respond(w, r, http.StatusCreated, p, err)
}

func (a *API) adminGetProduct(w http.ResponseWriter, r *http.Request) {
	p, err := a.svc.Catalog.AdminGet(r.Context(), chi.URLParam(r, "id"))
	a.respond(w, r, http.StatusOK, p, err)
}

func (a *API) adminUpdateProduct(w http.ResponseWriter, r *http.Request) {
	in, err := bind[service.ProductInput](a, w, r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	p, err := a.svc.Catalog.UpdateProduct(r.Context(), userFrom(r.Context()), chi.URLParam(r, "id"), *in)
	a.respond(w, r, http.StatusOK, p, err)
}

func (a *API) adminArchiveProduct(w http.ResponseWriter, r *http.Request) {
	if err := a.svc.Catalog.Archive(r.Context(), userFrom(r.Context()), chi.URLParam(r, "id")); err != nil {
		a.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// adminUploadImage accepts multipart field "file" and ?kind=product|category.
func (a *API) adminUploadImage(w http.ResponseWriter, r *http.Request) {
	kind := "product_image"
	switch r.URL.Query().Get("kind") {
	case "", "product":
	case "category":
		kind = "category_image"
	default:
		a.fail(w, r, apperr.BadRequest("bad_kind", "kind: product yoki category bo'lishi kerak"))
		return
	}
	data, err := readFilePart(w, r, a.cfg.MaxUploadBytes)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	img, err := a.svc.Files.UploadImage(r.Context(), userFrom(r.Context()), kind, data)
	a.respond(w, r, http.StatusCreated, img, err)
}

// ---------- admin: settings, admins, audit ----------

func (a *API) adminGetSettings(w http.ResponseWriter, r *http.Request) {
	s, err := a.svc.Settings.Full(r.Context())
	a.respond(w, r, http.StatusOK, s, err)
}

func (a *API) adminUpdateSettings(w http.ResponseWriter, r *http.Request) {
	in, err := bind[service.SettingsInput](a, w, r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	s, err := a.svc.Settings.Update(r.Context(), userFrom(r.Context()), *in)
	a.respond(w, r, http.StatusOK, s, err)
}

func (a *API) listAdmins(w http.ResponseWriter, r *http.Request) {
	items, err := a.svc.Admins.List(r.Context(), userFrom(r.Context()))
	a.respond(w, r, http.StatusOK, map[string]any{"items": items}, err)
}

func (a *API) createInvite(w http.ResponseWriter, r *http.Request) {
	inv, err := a.svc.Admins.CreateInvite(r.Context(), userFrom(r.Context()))
	a.respond(w, r, http.StatusCreated, inv, err)
}

type updateAdminInput struct {
	IsActive *bool `json:"isActive" validate:"required"`
}

func (a *API) updateAdmin(w http.ResponseWriter, r *http.Request) {
	in, err := bind[updateAdminInput](a, w, r)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	u, err := a.svc.Admins.SetActive(r.Context(), userFrom(r.Context()), chi.URLParam(r, "id"), *in.IsActive)
	a.respond(w, r, http.StatusOK, u, err)
}

func (a *API) removeAdmin(w http.ResponseWriter, r *http.Request) {
	if err := a.svc.Admins.Remove(r.Context(), userFrom(r.Context()), chi.URLParam(r, "id")); err != nil {
		a.fail(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) listAudit(w http.ResponseWriter, r *http.Request) {
	page, limit, off := service.Paging(qInt(r, "page"), qInt(r, "limit"), 50, 200)
	rows, err := a.svc.AuditList(r.Context(), limit, off)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	total := 0
	if len(rows) > 0 {
		total = rows[0].Total
	}
	writeJSON(w, http.StatusOK, service.Page[any]{Items: toAny(rows), Page: page, Limit: limit, Total: total})
}
