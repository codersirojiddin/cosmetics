// Command seed fills a DEVELOPMENT database with demo categories and products (generated images).
// It refuses to run when APP_ENV=production.
package main

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"time"

	"github.com/joho/godotenv"

	"github.com/codersirojiddin/cosmetics/apps/api/internal/app"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/config"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/db"
	"github.com/codersirojiddin/cosmetics/apps/api/internal/service"
)

// swatch renders a soft pastel gradient with a bottle-like shape, enough to design the UI against.
func swatch(c color.RGBA) []byte {
	const w, h = 900, 1100
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		t := float64(y) / h
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: mix(250, c.R, t*0.35), G: mix(246, c.G, t*0.35), B: mix(242, c.B, t*0.35), A: 255})
		}
	}
	cx, top := float64(w)/2, 260.0
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx := math.Abs(float64(x) - cx)
			body := float64(y) > top+110 && float64(y) < 900 && dx < 190
			neck := float64(y) > top && float64(y) <= top+110 && dx < 70
			if body || neck {
				shade := 1 - 0.18*(dx/190)
				img.SetRGBA(x, y, color.RGBA{R: uint8(float64(c.R) * shade), G: uint8(float64(c.G) * shade), B: uint8(float64(c.B) * shade), A: 255})
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func mix(a, b uint8, t float64) uint8 { return uint8(float64(a)*(1-t) + float64(b)*t) }

func main() {
	_ = godotenv.Load()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	if cfg.IsProd() {
		log.Fatal("refusing to seed a production database")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	a, err := app.New(cfg, pool, app.Options{})
	if err != nil {
		log.Fatal(err)
	}
	owner, err := a.Store.UpsertTelegramUser(ctx, 1, "Demo", "Owner", "demo_owner", true)
	if err != nil {
		log.Fatal(err)
	}
	svc := a.Services
	img := func(c color.RGBA, kind string) string {
		v, err := svc.Files.UploadImage(ctx, owner, kind, swatch(c))
		if err != nil {
			log.Fatal(err)
		}
		return v.ID
	}
	active := true
	cats := map[string]string{}
	for i, n := range []string{"Yuz parvarishi", "Makiyaj", "Soch parvarishi", "Parfyumeriya"} {
		c, err := svc.Catalog.CreateCategory(ctx, owner, service.CategoryInput{Name: n, Position: i, IsActive: &active, ImageID: img(color.RGBA{R: 214, G: 168, B: 160, A: 255}, "category_image")})
		if err != nil {
			log.Fatalf("category %s: %v", n, err)
		}
		cats[n] = c.ID
	}
	rose, nude, berry := color.RGBA{R: 205, G: 120, B: 130, A: 255}, color.RGBA{R: 214, G: 170, B: 140, A: 255}, color.RGBA{R: 150, G: 60, B: 90, A: 255}
	sage, sand, sky := color.RGBA{R: 160, G: 185, B: 160, A: 255}, color.RGBA{R: 225, G: 205, B: 170, A: 255}, color.RGBA{R: 150, G: 180, B: 205, A: 255}
	type v = service.VariantInput
	products := []struct {
		cat, name, brand, desc, option string
		color                          color.RGBA
		new, featured                  bool
		variants                       []v
	}{
		{"Yuz parvarishi", "Hyaluron Serum", "Lumi", "Chuqur namlantiruvchi **serum**.\n\n- Teriga yengil singadi\n- Kunduzi ham, kechqurun ham ishlatiladi", "Hajm", sky, true, true,
			[]v{{Label: "30 ml", Price: 185000, OldPrice: i64(220000), Stock: 12}, {Label: "50 ml", Price: 265000, Stock: 8}}},
		{"Yuz parvarishi", "Kunduzgi krem SPF 30", "Lumi", "Quyoshdan himoya qiluvchi yengil krem.", "", sand, false, true, []v{{Price: 149000, Stock: 20}}},
		{"Yuz parvarishi", "Tozalovchi gel", "Pure", "Yumshoq tozalovchi gel, har kuni uchun.", "", sage, false, false, []v{{Price: 89000, Stock: 30}}},
		{"Makiyaj", "Matte lab bo'yog'i", "Rouge", "Uzoq saqlanuvchi mat lab bo'yog'i.", "Tus", rose, true, true,
			[]v{{Label: "Rose", Price: 120000, Stock: 15}, {Label: "Nude", Price: 120000, Stock: 10}, {Label: "Berry", Price: 120000, Stock: 0}}},
		{"Makiyaj", "Tush (mascara)", "Rouge", "Hajm va uzunlik beruvchi tush.", "", color.RGBA{R: 60, G: 55, B: 60, A: 255}, false, false, []v{{Price: 98000, Stock: 25}}},
		{"Soch parvarishi", "Tiklovchi soch maskasi", "Silk", "Shikastlangan sochlar uchun chuqur parvarish.", "Hajm", nude, false, true,
			[]v{{Label: "250 ml", Price: 135000, Stock: 14}, {Label: "500 ml", Price: 215000, Stock: 6}}},
		{"Soch parvarishi", "Argan yog'i", "Silk", "Sochga yaltiroqlik beruvchi yog'.", "", sand, true, false, []v{{Price: 110000, Stock: 18}}},
		{"Parfyumeriya", "Eau de Parfum «Gul»", "Maison", "Gul va sandal daraxti hidi.", "Hajm", berry, false, true,
			[]v{{Label: "50 ml", Price: 420000, Stock: 7}, {Label: "100 ml", Price: 690000, Stock: 4}}},
	}
	for _, p := range products {
		_, err := svc.Catalog.CreateProduct(ctx, owner, service.ProductInput{CategoryID: cats[p.cat], Name: p.name, Brand: p.brand, Description: p.desc,
			OptionLabel: p.option, Status: "active", IsNew: p.new, IsFeatured: p.featured, ImageIDs: []string{img(p.color, "product_image"), img(p.color, "product_image")}, Variants: p.variants})
		if err != nil {
			log.Fatalf("product %s: %v", p.name, err)
		}
		log.Printf("created %s", p.name)
	}
	log.Printf("done: %d categories, %d products", len(cats), len(products))
}

func i64(n int64) *int64 { return &n }
