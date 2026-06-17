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

// opaqueOverlaySkin builds a skin with a visible face and a FULLY OPAQUE head
// overlay (hat) region, mimicking legacy skins like Notch's where the unused
// 2nd layer is left filled (here black).
func opaqueOverlaySkin() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := headFace.y; y < headFace.y+headFace.h; y++ {
		for x := headFace.x; x < headFace.x+headFace.w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 160, B: 120, A: 255}) // skin tone
		}
	}
	for y := headOverlay.y; y < headOverlay.y+headOverlay.h; y++ {
		for x := headOverlay.x; x < headOverlay.x+headOverlay.w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 0, G: 0, B: 0, A: 255}) // opaque black hat
		}
	}
	return img
}

// A fully opaque head overlay must be ignored so the face stays visible rather
// than being blacked out.
func TestOpaqueOverlayDoesNotBlackOutHead(t *testing.T) {
	black := func(c color.Color) bool {
		r, g, b, a := c.RGBA()
		return r>>8 == 0 && g>>8 == 0 && b>>8 == 0 && a>>8 == 255
	}
	render := func(name string, b []byte, err error) image.Image {
		t.Helper()
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		return decode(t, b)
	}

	out, err := Head(opaqueOverlaySkin(), 8)
	if head := render("Head", out, err); black(head.At(4, 4)) {
		t.Fatal("Head: face blacked out by opaque overlay")
	}

	out, err = Body(opaqueOverlaySkin(), 16)
	if body := render("Body", out, err); black(body.At(8, 4)) { // head at top-center
		t.Fatal("Body: head blacked out by opaque overlay")
	}

	out, err = Pfp(opaqueOverlaySkin(), 20)
	if pfp := render("Pfp", out, err); black(pfp.At(11, 7)) { // inside the bust's head
		t.Fatal("Pfp: head blacked out by opaque overlay")
	}
}

func TestTiny3DDimensionsAndContent(t *testing.T) {
	out, err := Tiny3D(sampleSkin(), 128, Tiny3DOptions{SS: 2})
	if err != nil {
		t.Fatal(err)
	}
	img := decode(t, out)
	b := img.Bounds()
	if b.Dx() != 128 || b.Dy() != 128 {
		t.Fatalf("got %v, want 128x128", b)
	}
	// Background corner must stay transparent.
	if _, _, _, a := img.At(0, 0).RGBA(); a != 0 {
		t.Fatalf("background not transparent: alpha=%d", a)
	}
	// The figure must actually be drawn somewhere (some opaque pixel).
	opaque := false
	for y := 0; y < 128 && !opaque; y += 4 {
		for x := 0; x < 128; x += 4 {
			if _, _, _, a := img.At(x, y).RGBA(); a == 0xffff {
				opaque = true
				break
			}
		}
	}
	if !opaque {
		t.Fatal("rendered figure is fully transparent")
	}
}

func TestTiny3DSlimAndLegacy(t *testing.T) {
	if _, err := Tiny3D(sampleSkin(), 96, Tiny3DOptions{Slim: true}); err != nil {
		t.Fatalf("slim: %v", err)
	}
	legacy := image.NewNRGBA(image.Rect(0, 0, 64, 32))
	for y := 0; y < 32; y++ {
		for x := 0; x < 64; x++ {
			legacy.SetNRGBA(x, y, color.NRGBA{R: uint8(x * 4), G: uint8(y * 8), B: 64, A: 255})
		}
	}
	if _, err := Tiny3D(legacy, 80, Tiny3DOptions{}); err != nil {
		t.Fatalf("legacy: %v", err)
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
