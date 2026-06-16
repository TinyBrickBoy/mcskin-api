package render

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func sampleSkin() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 4), G: uint8(y * 4), B: 128, A: 255})
		}
	}
	return img
}

func decode(t *testing.T, b []byte) image.Image {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return img
}

func TestFaceSize(t *testing.T) {
	out, err := Face(sampleSkin(), 64)
	if err != nil {
		t.Fatal(err)
	}
	if b := decode(t, out).Bounds(); b.Dx() != 64 || b.Dy() != 64 {
		t.Fatalf("got %v, want 64x64", b)
	}
}

func TestHeadComposites(t *testing.T) {
	out, err := Head(sampleSkin(), 32)
	if err != nil {
		t.Fatal(err)
	}
	if b := decode(t, out).Bounds(); b.Dx() != 32 {
		t.Fatalf("got width %d, want 32", b.Dx())
	}
}

func TestBodyDimensions(t *testing.T) {
	out, err := Body(sampleSkin(), 128)
	if err != nil {
		t.Fatal(err)
	}
	img := decode(t, out)
	b := img.Bounds()
	if b.Dx() != 128 || b.Dy() != 256 { // 16:32 aspect at scale 8
		t.Fatalf("got %v, want 128x256", b)
	}
	// Head must actually be rendered (centered near the top, not transparent).
	if _, _, _, a := img.At(64, 16).RGBA(); a == 0 {
		t.Fatal("head region is transparent; expected opaque pixels")
	}
}

func TestPfpDimensions(t *testing.T) {
	out, err := Pfp(sampleSkin(), 120)
	if err != nil {
		t.Fatal(err)
	}
	img := decode(t, out)
	b := img.Bounds()
	if b.Dx() != 120 || b.Dy() != 120 { // 20x20 tile at scale 6
		t.Fatalf("got %v, want 120x120", b)
	}
	// Background must be fully transparent (top-left corner is outside the bust).
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Fatalf("background not transparent: alpha=%d", a)
	}
}

func TestPfpLegacySkin(t *testing.T) {
	// A 64x32 (legacy) skin must render without panicking or erroring.
	legacy := image.NewNRGBA(image.Rect(0, 0, 64, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			legacy.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 4), G: uint8(y * 8), B: 64, A: 255})
		}
	}
	if _, err := Pfp(legacy, 40); err != nil {
		t.Fatalf("legacy pfp: %v", err)
	}
}

func TestSizeFloorIsOne(t *testing.T) {
	out, err := Face(sampleSkin(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if b := decode(t, out).Bounds(); b.Dx() < 1 {
		t.Fatalf("expected at least 1px, got %v", b)
	}
}
