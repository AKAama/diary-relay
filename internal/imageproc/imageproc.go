// Package imageproc validates, orients, resizes, and re-encodes uploaded images.
package imageproc

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"

	xdraw "golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // register webp decoder
)

const (
	MaxInputBytes = 10 << 20
	MaxWidth      = 1920
	JPEGQuality   = 82
	maxPixels     = 60_000_000 // decompression-bomb guard
)

var (
	ErrTooLarge    = errors.New("图片超过 10MB 限制")
	ErrUnsupported = errors.New("不支持的图片格式（仅支持 JPEG/PNG/GIF/WebP）")
	ErrHEIC        = errors.New("暂不支持 HEIC/HEIF，请在 iPhone 设置→相机→格式中选择“兼容性最佳”，或先转换为 JPEG")
)

// Result is a processed image ready to store.
type Result struct {
	Data []byte
	Ext  string // "jpg", "png", "gif"
	Mime string
}

// Detect sniffs the real format from content (not filename / client header).
func Detect(b []byte) (string, error) {
	if len(b) >= 12 && string(b[4:8]) == "ftyp" {
		brand := string(b[8:12])
		switch brand {
		case "heic", "heix", "hevc", "hevx", "heim", "heis", "mif1", "msf1", "avif":
			return "", ErrHEIC
		}
	}
	switch http.DetectContentType(b) {
	case "image/jpeg":
		return "jpeg", nil
	case "image/png":
		return "png", nil
	case "image/gif":
		return "gif", nil
	case "image/webp":
		return "webp", nil
	}
	return "", ErrUnsupported
}

// Process validates and compresses an upload.
func Process(b []byte) (*Result, error) {
	if len(b) > MaxInputBytes {
		return nil, ErrTooLarge
	}
	format, err := Detect(b)
	if err != nil {
		return nil, err
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("无法解析图片: %w", err)
	}
	if cfg.Width*cfg.Height > maxPixels {
		return nil, fmt.Errorf("图片像素过大 (%dx%d)", cfg.Width, cfg.Height)
	}
	if format == "gif" {
		// Keep animation untouched.
		if _, err := gif.DecodeConfig(bytes.NewReader(b)); err != nil {
			return nil, fmt.Errorf("无法解析 GIF: %w", err)
		}
		return &Result{Data: b, Ext: "gif", Mime: "image/gif"}, nil
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("无法解析图片: %w", err)
	}
	if format == "jpeg" {
		img = applyOrientation(img, jpegOrientation(b))
	}
	img = resize(img, MaxWidth)

	var buf bytes.Buffer
	if format != "jpeg" && hasTransparency(img) {
		enc := png.Encoder{CompressionLevel: png.BestCompression}
		if err := enc.Encode(&buf, img); err != nil {
			return nil, err
		}
		return &Result{Data: buf.Bytes(), Ext: "png", Mime: "image/png"}, nil
	}
	// Flatten onto white (in case of partially opaque edge) and encode JPEG.
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), image.White, image.Point{}, draw.Src)
	draw.Draw(rgba, rgba.Bounds(), img, img.Bounds().Min, draw.Over)
	if err := jpeg.Encode(&buf, rgba, &jpeg.Options{Quality: JPEGQuality}); err != nil {
		return nil, err
	}
	return &Result{Data: buf.Bytes(), Ext: "jpg", Mime: "image/jpeg"}, nil
}

func resize(img image.Image, maxW int) image.Image {
	b := img.Bounds()
	if b.Dx() <= maxW {
		return img
	}
	h := b.Dy() * maxW / b.Dx()
	if h < 1 {
		h = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, maxW, h))
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Src, nil)
	return dst
}

func hasTransparency(img image.Image) bool {
	if o, ok := img.(interface{ Opaque() bool }); ok {
		return !o.Opaque()
	}
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a < 0xffff {
				return true
			}
		}
	}
	return false
}

// jpegOrientation returns the EXIF orientation tag (1..8), or 1 if absent.
func jpegOrientation(b []byte) int {
	if len(b) < 4 || b[0] != 0xFF || b[1] != 0xD8 {
		return 1
	}
	i := 2
	for i+4 <= len(b) {
		if b[i] != 0xFF {
			return 1
		}
		marker := b[i+1]
		if marker == 0xDA || marker == 0xD9 {
			return 1
		}
		segLen := int(binary.BigEndian.Uint16(b[i+2:]))
		if segLen < 2 || i+2+segLen > len(b) {
			return 1
		}
		seg := b[i+4 : i+2+segLen]
		if marker == 0xE1 && len(seg) > 14 && string(seg[:6]) == "Exif\x00\x00" {
			return parseTIFFOrientation(seg[6:])
		}
		i += 2 + segLen
	}
	return 1
}

func parseTIFFOrientation(t []byte) int {
	if len(t) < 8 {
		return 1
	}
	var bo binary.ByteOrder
	switch string(t[:2]) {
	case "II":
		bo = binary.LittleEndian
	case "MM":
		bo = binary.BigEndian
	default:
		return 1
	}
	off := int(bo.Uint32(t[4:]))
	if off+2 > len(t) {
		return 1
	}
	n := int(bo.Uint16(t[off:]))
	for k := 0; k < n; k++ {
		e := off + 2 + k*12
		if e+12 > len(t) {
			return 1
		}
		if bo.Uint16(t[e:]) == 0x0112 {
			v := int(bo.Uint16(t[e+8:]))
			if v >= 1 && v <= 8 {
				return v
			}
			return 1
		}
	}
	return 1
}

// applyOrientation transforms img so it displays upright.
func applyOrientation(img image.Image, o int) image.Image {
	if o <= 1 || o > 8 {
		return img
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	dw, dh := w, h
	if o >= 5 {
		dw, dh = h, w
	}
	dst := image.NewRGBA(image.Rect(0, 0, dw, dh))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var nx, ny int
			switch o {
			case 2:
				nx, ny = w-1-x, y
			case 3:
				nx, ny = w-1-x, h-1-y
			case 4:
				nx, ny = x, h-1-y
			case 5:
				nx, ny = y, x
			case 6:
				nx, ny = h-1-y, x
			case 7:
				nx, ny = h-1-y, w-1-x
			case 8:
				nx, ny = y, w-1-x
			}
			dst.Set(nx, ny, img.At(b.Min.X+x, b.Min.Y+y))
		}
	}
	return dst
}
