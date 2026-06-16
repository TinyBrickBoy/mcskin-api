// Package render extracts and scales parts of a Minecraft skin texture into
// standalone PNG avatars.
package render

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

// Region is a rectangle within the 64x64 skin texture.
type region struct{ x, y, w, h int }

var (
	headFace    = region{8, 8, 8, 8}    // bare face
	headOverlay = region{40, 8, 8, 8}   // hat / 2nd layer
	bodyFront   = region{20, 20, 8, 12} // torso front
	armRight    = region{44, 20, 4, 12}
	legRight    = region{4, 20, 4, 12}
)

// Face returns the 8x8 face, scaled to size×size with nearest-neighbor.
func Face(skin image.Image, size int) ([]byte, error) {
	return encode(scaleNearest(crop(skin, headFace), size, size))
}

// Head returns the face with the hat/overlay layer composited on top.
func Head(skin image.Image, size int) ([]byte, error) {
	base := crop(skin, headFace)
	over := crop(skin, headOverlay)
	merged := image.NewNRGBA(base.Bounds())
	draw.Draw(merged, merged.Bounds(), base, base.Bounds().Min, draw.Src)
	draw.Draw(merged, merged.Bounds(), over, over.Bounds().Min, draw.Over)
	return encode(scaleNearest(merged, size, size))
}

// Body returns a flat front-facing render (head, torso, arms, legs) scaled so
// its width is roughly size pixels.
func Body(skin image.Image, size int) ([]byte, error) {
	// Layout in "skin pixels": 16 wide, 32 tall humanoid.
	canvas := image.NewNRGBA(image.Rect(0, 0, 16, 32))
	place := func(src image.Image, dx, dy int) {
		b := src.Bounds()
		draw.Draw(canvas, image.Rect(dx, dy, dx+b.Dx(), dy+b.Dy()), src, b.Min, draw.Over)
	}
	// crop returns origin-based images, so build the head at (0,0) too.
	head := image.NewNRGBA(image.Rect(0, 0, headFace.w, headFace.h))
	draw.Draw(head, head.Bounds(), crop(skin, headFace), image.Point{}, draw.Src)
	draw.Draw(head, head.Bounds(), crop(skin, headOverlay), image.Point{}, draw.Over)

	place(head, 4, 0)                  // head centered
	place(crop(skin, bodyFront), 4, 8) // torso
	place(crop(skin, armRight), 0, 8)  // left arm (viewer)
	place(crop(skin, armRight), 12, 8) // right arm (mirrored source reuse)
	place(crop(skin, legRight), 4, 20) // left leg
	place(crop(skin, legRight), 8, 20) // right leg

	scale := size / 16
	if scale < 1 {
		scale = 1
	}
	return encode(scaleNearest(canvas, 16*scale, 32*scale))
}

// Pfp renders a stylized 20x20 portrait bust (head + shoulders) composed from
// several skin regions, over a generated backdrop and under a light shading
// overlay, then scaled to size×size with nearest-neighbor. It supports both
// modern 64x64 and legacy 64x32 skins (the latter lack 2nd-layer arm/torso).
func Pfp(skin image.Image, size int) ([]byte, error) {
	const dim = 20
	canvas := image.NewNRGBA(image.Rect(0, 0, dim, dim))

	// Backdrop fills the whole tile.
	draw.Draw(canvas, canvas.Bounds(), pfpBackdrop(dim), image.Point{}, draw.Src)

	// blit copies a w×h region of the skin at (sx,sy) onto the canvas at
	// (dx,dy), compositing with alpha-over.
	blit := func(sx, sy, w, h, dx, dy int) {
		piece := crop(skin, region{sx, sy, w, h})
		draw.Draw(canvas, image.Rect(dx, dy, dx+w, dy+h), piece, image.Point{}, draw.Over)
	}

	legacy := skin.Bounds().Dy() <= 32

	// Bottom (base) layer.
	blit(8, 9, 7, 7, 8, 4)    // head
	blit(5, 9, 3, 7, 5, 4)    // head side
	if legacy {
		blit(44, 20, 3, 7, 12, 13) // right arm side (legacy texture position)
	} else {
		blit(36, 52, 3, 7, 12, 13) // right arm side
	}
	blit(21, 20, 6, 1, 7, 11) // chest neck line
	blit(20, 21, 8, 8, 6, 12) // chest
	blit(44, 20, 3, 7, 5, 13) // left arm side

	// Top (overlay) layer.
	blit(40, 9, 7, 7, 8, 4) // head overlay
	blit(33, 9, 3, 7, 5, 4) // head side overlay
	if !legacy {
		blit(52, 52, 3, 7, 12, 13) // right arm side overlay
		blit(52, 36, 3, 7, 5, 13)  // left arm side overlay
		blit(20, 37, 8, 8, 6, 12)  // chest overlay
		blit(21, 36, 6, 1, 7, 11)  // chest neck line overlay
	}

	// Shading overlay on top.
	draw.Draw(canvas, canvas.Bounds(), pfpShading(dim), image.Point{}, draw.Over)

	scale := size / dim
	if scale < 1 {
		scale = 1
	}
	return encode(scaleNearest(canvas, dim*scale, dim*scale))
}

// pfpBackdrop builds an n×n vertical gradient used behind the portrait.
func pfpBackdrop(n int) image.Image {
	top := color.NRGBA{R: 0x2b, G: 0x2f, B: 0x4a, A: 0xff}
	bot := color.NRGBA{R: 0x4a, G: 0x52, B: 0x80, A: 0xff}
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		t := float64(y) / float64(n-1)
		row := color.NRGBA{R: lerp(top.R, bot.R, t), G: lerp(top.G, bot.G, t), B: lerp(top.B, bot.B, t), A: 0xff}
		for x := 0; x < n; x++ {
			img.SetNRGBA(x, y, row)
		}
	}
	return img
}

// pfpShading builds a subtle translucent overlay that darkens toward the
// bottom-right corner for a touch of depth.
func pfpShading(n int) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			t := float64(x+y) / float64(2*(n-1))
			img.SetNRGBA(x, y, color.NRGBA{A: uint8(t * 60)})
		}
	}
	return img
}

func lerp(a, b uint8, t float64) uint8 {
	return uint8(float64(a) + (float64(b)-float64(a))*t)
}

func (r region) rect() image.Rectangle {
	return image.Rect(r.x, r.y, r.x+r.w, r.y+r.h)
}

// crop returns the region of skin, tolerating textures smaller than expected.
func crop(skin image.Image, r region) image.Image {
	out := image.NewNRGBA(image.Rect(0, 0, r.w, r.h))
	b := skin.Bounds()
	for y := 0; y < r.h; y++ {
		for x := 0; x < r.w; x++ {
			sx, sy := b.Min.X+r.x+x, b.Min.Y+r.y+y
			if sx < b.Max.X && sy < b.Max.Y {
				out.SetNRGBA(x, y, toNRGBA(skin.At(sx, sy)))
			}
		}
	}
	return out
}

func scaleNearest(src image.Image, w, h int) image.Image {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	b := src.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		sy := b.Min.Y + y*b.Dy()/h
		for x := 0; x < w; x++ {
			sx := b.Min.X + x*b.Dx()/w
			out.Set(x, y, src.At(sx, sy))
		}
	}
	return out
}

func toNRGBA(c color.Color) color.NRGBA {
	r, g, b, a := c.RGBA()
	return color.NRGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)}
}

func encode(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
