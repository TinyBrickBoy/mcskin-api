package mojang

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// defaultSkin builds a deterministic procedural fallback skin so requests for
// players without a custom skin still succeed. The hue is derived from the
// UUID, the model alternates classic/slim, mirroring Mojang's behaviour.
func defaultSkin(id string) *Skin {
	slim := defaultIsSlim(id)
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))

	skin := hueColor(id)
	shirt := color.NRGBA{R: 60, G: 60, B: 70, A: 255}
	pants := color.NRGBA{R: 40, G: 40, B: 50, A: 255}
	eye := color.NRGBA{R: 255, G: 255, B: 255, A: 255}

	fill := func(x0, y0, x1, y1 int, c color.NRGBA) {
		for y := y0; y < y1; y++ {
			for x := x0; x < x1; x++ {
				img.SetNRGBA(x, y, c)
			}
		}
	}

	// Head (front face at 8,8..16,16) plus the whole head region for texture
	// validity.
	fill(8, 8, 16, 16, skin)
	fill(10, 12, 11, 13, eye)
	fill(13, 12, 14, 13, eye)
	// Body
	fill(20, 20, 28, 32, shirt)
	// Arms
	fill(44, 20, 48, 32, skin)
	fill(36, 52, 40, 64, skin)
	// Legs
	fill(4, 20, 8, 32, pants)
	fill(20, 52, 24, 64, pants)

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return &Skin{PNG: buf.Bytes(), Slim: slim, Model: model(slim)}
}

func defaultIsSlim(id string) bool {
	if id == "" {
		return false
	}
	// Mojang selects the model from the hashed UUID; a simple parity over the
	// hex characters is a fine deterministic stand-in for the fallback.
	var sum int
	for _, r := range id {
		sum += int(r)
	}
	return sum%2 == 0
}

func hueColor(id string) color.NRGBA {
	var sum uint32
	for _, r := range id {
		sum = sum*31 + uint32(r)
	}
	return color.NRGBA{
		R: uint8(120 + sum%100),
		G: uint8(90 + (sum/100)%120),
		B: uint8(80 + (sum/10000)%120),
		A: 255,
	}
}
