package service

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/google/uuid"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/apperr"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/imgx"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/storage"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/store"
)

const maxReceiptUploadsPerHour = 20

type Files struct{ s *Services }

type ImageURLs struct {
	Thumb string `json:"thumb"`
	Card  string `json:"card"`
	Full  string `json:"full"`
}

type ImageView struct {
	ID     string    `json:"id"`
	URLs   ImageURLs `json:"urls"`
	Width  int       `json:"width"`
	Height int       `json:"height"`
}

var (
	productSpecs = []imgx.Spec{{Name: "thumb", MaxWidth: 400, MaxHeight: 400, Quality: 80}, {Name: "card", MaxWidth: 800, MaxHeight: 800, Quality: 82}, {Name: "full", MaxWidth: 1600, MaxHeight: 1600, Quality: 85}}
	receiptSpecs = []imgx.Spec{{Name: "full", MaxWidth: 2400, MaxHeight: 2400, Quality: 90}}
)

// imageURLs converts stored variant keys to public URLs (nil when there is no image).
func (s *Services) imageURLs(v map[string]string) *ImageURLs {
	if len(v) == 0 {
		return nil
	}
	u := &ImageURLs{
		Thumb: s.d.Storage.PublicURL(v["thumb"]),
		Card:  s.d.Storage.PublicURL(v["card"]),
		Full:  s.d.Storage.PublicURL(v["full"]),
	}
	return u
}

func processErr(err error) error {
	switch {
	case errors.Is(err, imgx.ErrInvalid):
		return apperr.Field("file", "Rasm fayli yaroqsiz (JPG, PNG yoki WebP yuklang)")
	case errors.Is(err, imgx.ErrTooLarge):
		return apperr.Field("file", "Rasm o'lchami juda katta")
	}
	return err
}

// UploadReceipt validates and stores a payment receipt (private storage).
func (f *Files) UploadReceipt(ctx context.Context, user *store.User, data []byte) (*store.File, error) {
	st := f.s.d.Store
	if n, err := st.CountRecentUploads(ctx, user.ID, "receipt", time.Now().Add(-time.Hour)); err != nil {
		return nil, err
	} else if n >= maxReceiptUploadsPerHour {
		return nil, apperr.New(429, "too_many_uploads", "Juda ko'p chek yuklandi, birozdan keyin urinib ko'ring")
	}
	res, err := imgx.Process(data, receiptSpecs)
	if err != nil {
		return nil, processErr(err)
	}
	if res.Width < 200 || res.Height < 200 {
		return nil, apperr.Field("file", "Rasm juda kichik, chek aniq ko'rinadigan rasm yuklang")
	}
	id := uuid.NewString()
	key := "receipts/" + id + ".jpg"
	if err := f.s.d.Storage.Put(ctx, storage.Private, key, res.Renditions["full"], "image/jpeg"); err != nil {
		return nil, err
	}
	dh := int64(res.DHash)
	file := &store.File{ID: id, Kind: "receipt", OwnerID: &user.ID, Variants: map[string]string{"full": key}, Mime: "image/jpeg",
		SizeBytes: int64(len(res.Renditions["full"])), Width: res.Width, Height: res.Height, SHA256: res.SHA256, DHash: &dh}
	if err := st.InsertFile(ctx, file); err != nil {
		_ = f.s.d.Storage.Delete(ctx, storage.Private, key)
		return nil, err
	}
	return file, nil
}

// UploadImage stores a product/category image in three sizes (public storage).
func (f *Files) UploadImage(ctx context.Context, actor *store.User, kind string, data []byte) (*ImageView, error) {
	prefix := "products/"
	if kind == "category_image" {
		prefix = "categories/"
	} else if kind != "product_image" {
		return nil, apperr.BadRequest("bad_kind", "Noto'g'ri rasm turi")
	}
	res, err := imgx.Process(data, productSpecs)
	if err != nil {
		return nil, processErr(err)
	}
	if res.Width < 300 || res.Height < 300 {
		return nil, apperr.Field("file", "Rasm kamida 300x300 piksel bo'lishi kerak")
	}
	id := uuid.NewString()
	variants := map[string]string{}
	var written []string
	cleanup := func() {
		for _, k := range written {
			_ = f.s.d.Storage.Delete(ctx, storage.Public, k)
		}
	}
	for name, b := range res.Renditions {
		key := prefix + id + "/" + name + ".jpg"
		if err := f.s.d.Storage.Put(ctx, storage.Public, key, b, "image/jpeg"); err != nil {
			cleanup()
			return nil, err
		}
		written = append(written, key)
		variants[name] = key
	}
	dh := int64(res.DHash)
	file := &store.File{ID: id, Kind: kind, OwnerID: &actor.ID, Variants: variants, Mime: "image/jpeg",
		SizeBytes: int64(len(res.Renditions["full"])), Width: res.Width, Height: res.Height, SHA256: res.SHA256, DHash: &dh}
	if err := f.s.d.Store.InsertFile(ctx, file); err != nil {
		cleanup()
		return nil, err
	}
	return &ImageView{ID: id, URLs: *f.s.imageURLs(variants), Width: res.Width, Height: res.Height}, nil
}

// OpenReceipt returns the receipt image if the user owns it or is staff.
func (f *Files) OpenReceipt(ctx context.Context, user *store.User, fileID string) (io.ReadCloser, error) {
	notFound := apperr.NotFound("Fayl topilmadi")
	file, err := f.s.d.Store.GetFile(ctx, fileID)
	if errors.Is(err, store.ErrNotFound) || store.IsInvalidText(err) {
		return nil, notFound
	}
	if err != nil {
		return nil, err
	}
	if file.Kind != "receipt" {
		return nil, notFound
	}
	if !user.IsStaff() && (file.OwnerID == nil || *file.OwnerID != user.ID) {
		return nil, notFound // do not reveal that the file exists
	}
	rc, err := f.s.d.Storage.Get(ctx, storage.Private, file.Variants["full"])
	if errors.Is(err, storage.ErrNotFound) {
		return nil, notFound
	}
	return rc, err
}
