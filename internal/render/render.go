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
