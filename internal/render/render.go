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
	if !fullyOpaque(over) {
		draw.Draw(merged, merged.Bounds(), over, over.Bounds().Min, draw.Over)
	}
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
	if over := crop(skin, headOverlay); !fullyOpaque(over) {
		draw.Draw(head, head.Bounds(), over, image.Point{}, draw.Over)
	}

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
// several skin regions on a transparent background, with a light shading
// overlay, then scaled to size×size with nearest-neighbor. It supports both
// modern 64x64 and legacy 64x32 skins (the latter lack 2nd-layer arm/torso).
func Pfp(skin image.Image, size int) ([]byte, error) {
	const dim = 20
	canvas := image.NewNRGBA(image.Rect(0, 0, dim, dim))

	// blit copies a w×h region of the skin at (sx,sy) onto the canvas at
	// (dx,dy), compositing with alpha-over.
	blit := func(sx, sy, w, h, dx, dy int) {
		piece := crop(skin, region{sx, sy, w, h})
		draw.Draw(canvas, image.Rect(dx, dy, dx+w, dy+h), piece, image.Point{}, draw.Over)
	}

	legacy := skin.Bounds().Dy() <= 32

	// Bottom (base) layer.
	blit(8, 9, 7, 7, 8, 4) // head
	blit(5, 9, 3, 7, 5, 4) // head side
	if legacy {
		blit(44, 20, 3, 7, 12, 13) // right arm side (legacy texture position)
	} else {
		blit(36, 52, 3, 7, 12, 13) // right arm side
	}
	blit(21, 20, 6, 1, 7, 11) // chest neck line
	blit(20, 21, 8, 8, 6, 12) // chest
	blit(44, 20, 3, 7, 5, 13) // left arm side

	// Top (overlay) layer. Skip the head overlay when it is fully opaque: legacy
	// skins often leave the hat region filled (commonly black), which would
	// otherwise completely cover the face.
	if !fullyOpaque(crop(skin, headOverlay)) {
		blit(40, 9, 7, 7, 8, 4) // head overlay
		blit(33, 9, 3, 7, 5, 4) // head side overlay
	}
	if !legacy {
		blit(52, 52, 3, 7, 12, 13) // right arm side overlay
		blit(52, 36, 3, 7, 5, 13)  // left arm side overlay
		blit(20, 37, 8, 8, 6, 12)  // chest overlay
		blit(21, 36, 6, 1, 7, 11)  // chest neck line overlay
	}

	// Shading: darken opaque pixels toward the bottom-right corner for a touch
	// of depth, leaving the transparent background untouched. Equivalent to
	// compositing a translucent black gradient, but in a single in-place pass.
	for y := 0; y < dim; y++ {
		for x := 0; x < dim; x++ {
			i := canvas.PixOffset(x, y)
			if canvas.Pix[i+3] == 0 {
				continue
			}
			f := float64(x+y) * 60 / float64(2*(dim-1)) / 255 // 0..~0.24
			k := 1 - f
			canvas.Pix[i+0] = uint8(float64(canvas.Pix[i+0]) * k)
			canvas.Pix[i+1] = uint8(float64(canvas.Pix[i+1]) * k)
			canvas.Pix[i+2] = uint8(float64(canvas.Pix[i+2]) * k)
		}
	}

	scale := size / dim
	if scale < 1 {
		scale = 1
	}
	return encode(scaleNearest(canvas, dim*scale, dim*scale))
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

// fullyOpaque reports whether every pixel of img has alpha 255. A fully opaque
// head overlay is a legacy-skin artifact (the unused 2nd layer left filled,
// often black) rather than a real hat, so callers skip compositing it to avoid
// blacking out the face.
func fullyOpaque(img image.Image) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a != 0xffff {
				return false
			}
		}
	}
	return true
}

func scaleNearest(src image.Image, w, h int) image.Image {
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	out := image.NewNRGBA(image.Rect(0, 0, w, h))

	// Fast path: copy 4-byte pixels straight out of the source's Pix buffer,
	// skipping per-pixel interface dispatch and colour conversion. All callers
	// pass *image.NRGBA, so this is the hot path for every endpoint.
	if nr, ok := src.(*image.NRGBA); ok {
		xoff := make([]int, w) // source byte offset within a row per column
		for x := 0; x < w; x++ {
			xoff[x] = (x * sw / w) * 4
		}
		for y := 0; y < h; y++ {
			srow := nr.Pix[nr.PixOffset(b.Min.X, b.Min.Y+y*sh/h):]
			drow := out.Pix[y*out.Stride:]
			for x, di := 0, 0; x < w; x, di = x+1, di+4 {
				copy(drow[di:di+4], srow[xoff[x]:xoff[x]+4])
			}
		}
		return out
	}

	// Generic fallback for any other image type.
	for y := 0; y < h; y++ {
		sy := b.Min.Y + y*sh/h
		for x := 0; x < w; x++ {
			out.Set(x, y, src.At(b.Min.X+x*sw/w, sy))
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
