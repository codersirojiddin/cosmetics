// Package imgx validates and re-encodes uploaded images in pure Go (no cgo).
// Re-encoding strips metadata (EXIF/GPS) and neutralises malformed or polyglot files.
package imgx

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math/bits"

	"github.com/disintegration/imaging"
	_ "golang.org/x/image/webp" // register WebP decoder
)

const MaxPixels = 40_000_000

var (
	ErrInvalid  = errors.New("imgx: not a valid image")
	ErrTooLarge = errors.New("imgx: image dimensions too large")
)

// Spec describes one output rendition (images are only ever scaled down).
type Spec struct {
	Name      string
	MaxWidth  int
	MaxHeight int
	Quality   int
}

type Result struct {
	Width, Height int // size of the decoded (auto-oriented) original
	SHA256        string
	DHash         uint64
	Renditions    map[string][]byte // JPEG bytes
}

func Process(data []byte, specs []Spec) (*Result, error) {
	cfg, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, ErrInvalid
	}
	switch format {
	case "jpeg", "png", "gif", "webp":
	default:
		return nil, ErrInvalid
	}
	if cfg.Width < 1 || cfg.Height < 1 {
		return nil, ErrInvalid
	}
	if cfg.Width*cfg.Height > MaxPixels {
		return nil, ErrTooLarge
	}
	img, err := imaging.Decode(bytes.NewReader(data), imaging.AutoOrientation(true))
	if err != nil {
		return nil, ErrInvalid
	}
	img = flatten(img)

	sum := sha256.Sum256(data)
	res := &Result{
		Width:      img.Bounds().Dx(),
		Height:     img.Bounds().Dy(),
		SHA256:     hex.EncodeToString(sum[:]),
		DHash:      DHash(img),
		Renditions: make(map[string][]byte, len(specs)),
	}
	for _, s := range specs {
		out := imaging.Fit(img, s.MaxWidth, s.MaxHeight, imaging.Lanczos)
		var buf bytes.Buffer
		if err := jpeg.Encode(&buf, out, &jpeg.Options{Quality: s.Quality}); err != nil {
			return nil, err
		}
		res.Renditions[s.Name] = buf.Bytes()
	}
	return res, nil
}

// flatten composites transparent images over white (JPEG has no alpha channel).
func flatten(img image.Image) image.Image {
	if o, ok := img.(interface{ Opaque() bool }); ok && o.Opaque() {
		return img
	}
	dst := imaging.New(img.Bounds().Dx(), img.Bounds().Dy(), color.White)
	draw.Draw(dst, dst.Bounds(), img, img.Bounds().Min, draw.Over)
	return dst
}

// DHash computes a 64-bit difference hash: robust to rescaling and recompression,
// so near-identical screenshots of the same receipt end up a few bits apart.
func DHash(img image.Image) uint64 {
	g := imaging.Grayscale(imaging.Resize(img, 9, 8, imaging.Box))
	var h uint64
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			h <<= 1
			if g.NRGBAAt(x, y).R > g.NRGBAAt(x+1, y).R {
				h |= 1
			}
		}
	}
	return h
}

func Hamming(a, b uint64) int { return bits.OnesCount64(a ^ b) }
