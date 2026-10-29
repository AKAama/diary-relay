package imageproc

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

func solid(w, h int, c color.Color) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	return img
}

func TestJPEGResized(t *testing.T) {
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, solid(3000, 1500, color.RGBA{200, 10, 10, 255}), nil)
	r, err := Process(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	cfg, f, _ := image.DecodeConfig(bytes.NewReader(r.Data))
	if f != "jpeg" || r.Ext != "jpg" || cfg.Width != 1920 || cfg.Height != 960 {
		t.Fatalf("got %s %dx%d", f, cfg.Width, cfg.Height)
	}
}

func TestOpaquePNGBecomesJPEG(t *testing.T) {
	var buf bytes.Buffer
	_ = png.Encode(&buf, solid(100, 50, color.NRGBA{1, 2, 3, 255}))
	r, err := Process(buf.Bytes())
	if err != nil || r.Ext != "jpg" {
		t.Fatalf("%v %v", r, err)
	}
}

func TestTransparentPNGStaysPNG(t *testing.T) {
	var buf bytes.Buffer
	_ = png.Encode(&buf, solid(100, 50, color.NRGBA{1, 2, 3, 0}))
	r, err := Process(buf.Bytes())
	if err != nil || r.Ext != "png" {
		t.Fatalf("%v %v", r, err)
	}
}

func TestGIFPassthrough(t *testing.T) {
	var buf bytes.Buffer
	p := image.NewPaletted(image.Rect(0, 0, 10, 10), color.Palette{color.Black, color.White})
	_ = gif.Encode(&buf, p, nil)
	r, err := Process(buf.Bytes())
	if err != nil || r.Ext != "gif" || !bytes.Equal(r.Data, buf.Bytes()) {
		t.Fatalf("%v", err)
	}
}

func TestRejects(t *testing.T) {
	heic := append([]byte{0, 0, 0, 24}, []byte("ftypheic0000")...)
	if _, err := Process(heic); !errors.Is(err, ErrHEIC) {
		t.Fatalf("heic: %v", err)
	}
	if _, err := Process([]byte("hello world, not an image")); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("text: %v", err)
	}
	if _, err := Process(make([]byte, MaxInputBytes+1)); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("big: %v", err)
	}
}

func TestOrientation6Rotates(t *testing.T) {
	img := solid(4, 2, color.White)
	out := applyOrientation(img, 6)
	if out.Bounds().Dx() != 2 || out.Bounds().Dy() != 4 {
		t.Fatal(out.Bounds())
	}
	// Build minimal EXIF APP1 with orientation=6 (big endian).
	tiff := []byte{'M', 'M', 0, 42, 0, 0, 0, 8, 0, 1, 0x01, 0x12, 0, 3, 0, 0, 0, 1, 0, 6, 0, 0, 0, 0, 0, 0, 0, 0}
	seg := append([]byte("Exif\x00\x00"), tiff...)
	l := len(seg) + 2
	b := []byte{0xFF, 0xD8, 0xFF, 0xE1, byte(l >> 8), byte(l)}
	b = append(b, seg...)
	b = append(b, 0xFF, 0xD9)
	if o := jpegOrientation(b); o != 6 {
		t.Fatalf("orientation %d", o)
	}
}
