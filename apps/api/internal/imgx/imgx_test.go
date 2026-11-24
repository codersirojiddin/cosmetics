package imgx

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/require"
)

func blocks(seed int64, cw, ch int) []byte {
	rng := rand.New(rand.NewSource(seed))
	img := image.NewGray(image.Rect(0, 0, 9*cw, 8*ch))
	for by := 0; by < 8; by++ {
		for bx := 0; bx < 9; bx++ {
			g := color.Gray{Y: uint8(rng.Intn(256))}
			for y := by * ch; y < (by+1)*ch; y++ {
				for x := bx * cw; x < (bx+1)*cw; x++ {
					img.SetGray(x, y, g)
				}
			}
		}
	}
	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}

func TestProcessRenditionsAndHash(t *testing.T) {
	specs := []Spec{{Name: "thumb", MaxWidth: 100, MaxHeight: 100, Quality: 80}, {Name: "full", MaxWidth: 2000, MaxHeight: 2000, Quality: 85}}
	a, err := Process(blocks(1, 48, 48), specs)
	require.NoError(t, err)
	require.Equal(t, 432, a.Width)
	require.Len(t, a.Renditions, 2)

	cfg, format, err := image.DecodeConfig(bytes.NewReader(a.Renditions["thumb"]))
	require.NoError(t, err)
	require.Equal(t, "jpeg", format)
	require.LessOrEqual(t, cfg.Width, 100) // scaled down
	cfg, _, _ = image.DecodeConfig(bytes.NewReader(a.Renditions["full"]))
	require.Equal(t, 432, cfg.Width) // never upscaled

	same, _ := Process(blocks(1, 60, 60), specs) // same picture, different resolution
	other, _ := Process(blocks(2, 48, 48), specs)
	require.LessOrEqual(t, Hamming(a.DHash, same.DHash), 4)
	require.Greater(t, Hamming(a.DHash, other.DHash), 8)
	require.NotEqual(t, a.SHA256, same.SHA256)
}

func TestProcessRejectsGarbage(t *testing.T) {
	_, err := Process([]byte("not an image at all"), nil)
	require.ErrorIs(t, err, ErrInvalid)
	_, err = Process(nil, nil)
	require.ErrorIs(t, err, ErrInvalid)
}
