package render

// tiny3d renders a Minecraft skin into a "tiny" (big-head) 3D PNG bust using a
// tiny software rasterizer: pure standard library, no GPU. It is a Go port of
// the original Node/pngjs render-skin.js. Capes are not supported here because
// the Mojang client only fetches the skin texture.

import (
	"image"
	"math"
)

// Fixed camera and quality settings for the 3D render. These are intentionally
// not configurable: every avatar is rendered from the same angle so results are
// consistent across players.
const (
	camElev  = 33  // camera height above the horizon, in degrees
	camAzim  = -38 // camera rotation around the figure, in degrees
	camFov   = 32  // vertical field of view, in degrees
	ssFactor = 3   // supersampling factor for anti-aliasing
)

// Tiny3D renders skin as a stylized big-head 3D bust, size×size pixels, on a
// transparent background, from a fixed camera angle. slim selects the 3px-arm
// (Alex) model.
func Tiny3D(skin image.Image, size int, slim bool) ([]byte, error) {
	out := size
	if out < 64 {
		out = 64
	}
	if out > 2048 {
		out = 2048
	}
	ss := ssFactor
	w, h := out*ss, out*ss

	elev := deg(camElev)
	azim := deg(camAzim)
	fov := deg(camFov)

	tex := newTexture(skin)
	parts := buildParts(slim)

	dist := (radius / math.Sin(fov/2)) * 1.05
	eye := vec3{
		target[0] + dist*math.Cos(elev)*math.Sin(azim),
		target[1] + dist*math.Sin(elev),
		target[2] + dist*math.Cos(elev)*math.Cos(azim),
	}
	view := lookAt(eye, target, vec3{0, 1, 0})
	proj := perspective(fov, float64(w)/float64(h), 0.1, 1000)

	color := make([]uint8, w*h*4) // transparent RGBA
	zbuf := make([]float64, w*h)
	for i := range zbuf {
		zbuf[i] = math.Inf(1)
	}
	light := norm(vec3{-0.45, 0.85, 0.55})
	const amb, dif = 0.5, 0.55

	for _, part := range parts {
		g := part.geom()
		wp := make([]vec4, len(g.positions)/3)
		wn := make([]vec3, len(g.positions)/3)
		for i := 0; i < len(g.positions); i += 3 {
			p := vec3{g.positions[i], g.positions[i+1], g.positions[i+2]}
			n := vec3{g.normals[i], g.normals[i+1], g.normals[i+2]}
			wp[i/3] = tPoint(part.matrix, p)
			wn[i/3] = norm(tDir(part.matrix, n))
		}
		for t := 0; t < len(g.indices); t += 3 {
			tri := [3]int{g.indices[t], g.indices[t+1], g.indices[t+2]}
			var verts [3]vertex
			for j, vi := range tri {
				cv := tPoint(view, vec3{wp[vi][0], wp[vi][1], wp[vi][2]})
				cp := tPoint(proj, vec3{cv[0], cv[1], cv[2]})
				cw := cp[3]
				if cw == 0 {
					cw = 1e-6
				}
				verts[j] = vertex{
					sx: (cp[0]/cw*0.5 + 0.5) * float64(w),
					sy: (1 - (cp[1]/cw*0.5 + 0.5)) * float64(h),
					z:  cp[2] / cw,
					iw: 1 / cw,
					u:  g.uvs[vi*2],
					v:  g.uvs[vi*2+1],
				}
			}
			nrm := wn[tri[0]]
			te := norm(vec3{eye[0] - wp[tri[0]][0], eye[1] - wp[tri[0]][1], eye[2] - wp[tri[0]][2]})
			if dot(nrm, te) < 0 {
				nrm = vec3{-nrm[0], -nrm[1], -nrm[2]}
			}
			shade := amb + dif*math.Max(0, dot(nrm, light))
			raster(verts, color, zbuf, w, h, tex, shade, part.mask)
		}
	}

	// Downsample the supersampled buffer into the final image.
	img := image.NewNRGBA(image.Rect(0, 0, out, out))
	n := ss * ss
	for y := 0; y < out; y++ {
		for x := 0; x < out; x++ {
			var r, gn, b, a int
			for dy := 0; dy < ss; dy++ {
				for dx := 0; dx < ss; dx++ {
					si := ((y*ss+dy)*w + (x*ss + dx)) * 4
					r += int(color[si])
					gn += int(color[si+1])
					b += int(color[si+2])
					a += int(color[si+3])
				}
			}
			di := img.PixOffset(x, y)
			img.Pix[di+0] = uint8(r / n)
			img.Pix[di+1] = uint8(gn / n)
			img.Pix[di+2] = uint8(b / n)
			img.Pix[di+3] = uint8(a / n)
		}
	}
	return encode(img)
}

func deg(d float64) float64 { return d * math.Pi / 180 }

/* ----------------------------------------------------------- linear algebra */

type vec3 [3]float64
type vec4 [4]float64

// mat4 is column-major: m[col*4 + row].
type mat4 [16]float64

func mul(a, b mat4) mat4 {
	var o mat4
	for c := 0; c < 4; c++ {
		for r := 0; r < 4; r++ {
			s := 0.0
			for k := 0; k < 4; k++ {
				s += a[k*4+r] * b[c*4+k]
			}
			o[c*4+r] = s
		}
	}
	return o
}

func translate(x, y, z float64) mat4 {
	return mat4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, x, y, z, 1}
}

func rotX(a float64) mat4 {
	c, s := math.Cos(a), math.Sin(a)
	return mat4{1, 0, 0, 0, 0, c, s, 0, 0, -s, c, 0, 0, 0, 0, 1}
}

func tPoint(m mat4, p vec3) vec4 {
	x, y, z := p[0], p[1], p[2]
	return vec4{
		m[0]*x + m[4]*y + m[8]*z + m[12],
		m[1]*x + m[5]*y + m[9]*z + m[13],
		m[2]*x + m[6]*y + m[10]*z + m[14],
		m[3]*x + m[7]*y + m[11]*z + m[15],
	}
}

func tDir(m mat4, d vec3) vec3 {
	x, y, z := d[0], d[1], d[2]
	return vec3{
		m[0]*x + m[4]*y + m[8]*z,
		m[1]*x + m[5]*y + m[9]*z,
		m[2]*x + m[6]*y + m[10]*z,
	}
}

func perspective(fovy, aspect, near, far float64) mat4 {
	f := 1 / math.Tan(fovy/2)
	nf := 1 / (near - far)
	return mat4{
		f / aspect, 0, 0, 0,
		0, f, 0, 0,
		0, 0, (far + near) * nf, -1,
		0, 0, 2 * far * near * nf, 0,
	}
}

func subv(a, b vec3) vec3 { return vec3{a[0] - b[0], a[1] - b[1], a[2] - b[2]} }
func cross(a, b vec3) vec3 {
	return vec3{a[1]*b[2] - a[2]*b[1], a[2]*b[0] - a[0]*b[2], a[0]*b[1] - a[1]*b[0]}
}
func dot(a, b vec3) float64 { return a[0]*b[0] + a[1]*b[1] + a[2]*b[2] }
func norm(a vec3) vec3 {
	l := math.Hypot(math.Hypot(a[0], a[1]), a[2])
	if l == 0 {
		l = 1
	}
	return vec3{a[0] / l, a[1] / l, a[2] / l}
}

func lookAt(eye, center, up vec3) mat4 {
	z := norm(subv(eye, center))
	x := norm(cross(up, z))
	y := cross(z, x)
	return mat4{
		x[0], y[0], z[0], 0,
		x[1], y[1], z[1], 0,
		x[2], y[2], z[2], 0,
		-dot(x, eye), -dot(y, eye), -dot(z, eye), 1,
	}
}

/* ------------------------------------------------------------------ geometry */

const atlas = 64

type geometry struct {
	positions []float64
	normals   []float64
	uvs       []float64
	indices   []int
}

// box builds a unit-cube mesh of the given size, with UVs mapped from a
// Minecraft-style atlas region at texOrigin spanning texSize voxels.
func box(size vec3, texOrigin [2]float64, texSize vec3) geometry {
	w, h, d := size[0], size[1], size[2]
	a, b, c := w/2, h/2, d/2
	u0, v0 := texOrigin[0], texOrigin[1]
	tw, th, td := texSize[0], texSize[1], texSize[2]

	type face struct {
		n vec3
		v [4]vec3
		r [4]float64 // x, y, w, h within the atlas
	}
	faces := []face{
		{vec3{0, 0, 1}, [4]vec3{{-a, b, c}, {a, b, c}, {a, -b, c}, {-a, -b, c}}, [4]float64{u0 + td, v0 + td, tw, th}},             // front
		{vec3{0, 0, -1}, [4]vec3{{a, b, -c}, {-a, b, -c}, {-a, -b, -c}, {a, -b, -c}}, [4]float64{u0 + 2*td + tw, v0 + td, tw, th}}, // back
		{vec3{1, 0, 0}, [4]vec3{{a, b, c}, {a, b, -c}, {a, -b, -c}, {a, -b, c}}, [4]float64{u0, v0 + td, td, th}},                  // right
		{vec3{-1, 0, 0}, [4]vec3{{-a, b, -c}, {-a, b, c}, {-a, -b, c}, {-a, -b, -c}}, [4]float64{u0 + td + tw, v0 + td, td, th}},   // left
		{vec3{0, 1, 0}, [4]vec3{{-a, b, -c}, {a, b, -c}, {a, b, c}, {-a, b, c}}, [4]float64{u0 + td, v0, tw, td}},                  // top
		{vec3{0, -1, 0}, [4]vec3{{-a, -b, c}, {a, -b, c}, {a, -b, -c}, {-a, -b, -c}}, [4]float64{u0 + td + tw, v0, tw, td}},        // bottom
	}

	var g geometry
	k := 0
	for _, f := range faces {
		x, y, ww, hh := f.r[0], f.r[1], f.r[2], f.r[3]
		uL, uR := x/atlas, (x+ww)/atlas
		vT, vB := y/atlas, (y+hh)/atlas
		fuv := [4][2]float64{{uL, vT}, {uR, vT}, {uR, vB}, {uL, vB}}
		for i := 0; i < 4; i++ {
			g.positions = append(g.positions, f.v[i][0], f.v[i][1], f.v[i][2])
			g.normals = append(g.normals, f.n[0], f.n[1], f.n[2])
			g.uvs = append(g.uvs, fuv[i][0], fuv[i][1])
		}
		g.indices = append(g.indices, k, k+1, k+2, k, k+2, k+3)
		k += 4
	}
	return g
}

/* ------------------------------------------------------ character + pose */

// pose constants ported from DEFAULT_POSE in render-skin.js.
const (
	poseHeadPitch = -0.13
	poseLegR      = -0.28
	poseLegL      = 0.0
)

var (
	target = vec3{0, 9.5, 0}
	radius = 14.0
)

type part struct {
	g      geometry
	matrix mat4
	mask   bool // overlay layer: skip texels with alpha < 128
}

func (p part) geom() geometry { return p.g }

// buildParts assembles the big-head humanoid: base + overlay boxes for head,
// torso, arms and legs. Capes are intentionally omitted.
func buildParts(slim bool) []part {
	armW, armX, sleeveW := 3.0, 5.0, 3.5
	armTexW := 4.0
	if slim {
		armW, armX, sleeveW = 2.5, 4.75, 3.0
		armTexW = 3.0
	}

	headM := mul(mul(translate(0, 13, 0), rotX(poseHeadPitch)), translate(0, 4.25, 0))
	legRM := mul(mul(translate(-1.5, 6, 0), rotX(poseLegR)), translate(0, -3, 0))
	legLM := mul(mul(translate(1.5, 6, 0), rotX(poseLegL)), translate(0, -3, 0))

	mk := func(size vec3, origin [2]float64, texSize vec3, m mat4, mask bool) part {
		return part{g: box(size, origin, texSize), matrix: m, mask: mask}
	}

	return []part{
		mk(vec3{8.5, 8.5, 8.5}, [2]float64{0, 0}, vec3{8, 8, 8}, headM, false),
		mk(vec3{9.3, 9.3, 9.3}, [2]float64{32, 0}, vec3{8, 8, 8}, headM, true),
		mk(vec3{7, 7, 3.5}, [2]float64{16, 16}, vec3{8, 12, 4}, translate(0, 9.5, 0), false),
		mk(vec3{7.6, 7.6, 4.1}, [2]float64{16, 32}, vec3{8, 12, 4}, translate(0, 9.5, 0), true),
		mk(vec3{armW, 6.5, armW}, [2]float64{40, 16}, vec3{armTexW, 12, 4}, translate(-armX, 9.75, 0), false),
		mk(vec3{sleeveW, 7, sleeveW}, [2]float64{40, 32}, vec3{armTexW, 12, 4}, translate(-armX, 9.75, 0), true),
		mk(vec3{armW, 6.5, armW}, [2]float64{32, 48}, vec3{armTexW, 12, 4}, translate(armX, 9.75, 0), false),
		mk(vec3{sleeveW, 7, sleeveW}, [2]float64{48, 48}, vec3{armTexW, 12, 4}, translate(armX, 9.75, 0), true),
		mk(vec3{3, 6, 3}, [2]float64{0, 16}, vec3{4, 12, 4}, legRM, false),
		mk(vec3{3.3, 6.4, 3.3}, [2]float64{0, 32}, vec3{4, 12, 4}, legRM, true),
		mk(vec3{3, 6, 3}, [2]float64{16, 48}, vec3{4, 12, 4}, legLM, false),
		mk(vec3{3.3, 6.4, 3.3}, [2]float64{0, 48}, vec3{4, 12, 4}, legLM, true),
	}
}

/* ------------------------------------------------------------------ texture */

// texture is a flat RGBA copy of the skin for fast nearest-neighbor sampling.
type texture struct {
	w, h int
	data []uint8
}

func newTexture(img image.Image) *texture {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	t := &texture{w: w, h: h, data: make([]uint8, w*h*4)}
	if nr, ok := img.(*image.NRGBA); ok {
		copy(t.data, nr.Pix)
		return t
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := toNRGBA(img.At(b.Min.X+x, b.Min.Y+y))
			i := (y*w + x) * 4
			t.data[i+0], t.data[i+1], t.data[i+2], t.data[i+3] = c.R, c.G, c.B, c.A
		}
	}
	return t
}

func (t *texture) sample(u, v float64) (r, g, b, a uint8) {
	x := int(math.Floor(u * float64(t.w)))
	y := int(math.Floor(v * float64(t.h)))
	if x < 0 {
		x = 0
	} else if x >= t.w {
		x = t.w - 1
	}
	if y < 0 {
		y = 0
	} else if y >= t.h {
		y = t.h - 1
	}
	i := (y*t.w + x) * 4
	return t.data[i], t.data[i+1], t.data[i+2], t.data[i+3]
}

/* ---------------------------------------------------------------- rasterizer */

type vertex struct {
	sx, sy, z, iw, u, v float64
}

func raster(v [3]vertex, color []uint8, zbuf []float64, w, h int, tex *texture, shade float64, mask bool) {
	a, b, c := v[0], v[1], v[2]
	minX := clampi(int(math.Floor(min3(a.sx, b.sx, c.sx))), 0, w-1)
	maxX := clampi(int(math.Ceil(max3(a.sx, b.sx, c.sx))), 0, w-1)
	minY := clampi(int(math.Floor(min3(a.sy, b.sy, c.sy))), 0, h-1)
	maxY := clampi(int(math.Ceil(max3(a.sy, b.sy, c.sy))), 0, h-1)

	area := (b.sx-a.sx)*(c.sy-a.sy) - (b.sy-a.sy)*(c.sx-a.sx)
	if math.Abs(area) < 1e-9 {
		return
	}
	inv := 1 / area

	for y := minY; y <= maxY; y++ {
		py := float64(y) + 0.5
		for x := minX; x <= maxX; x++ {
			px := float64(x) + 0.5
			w0 := ((b.sx-px)*(c.sy-py) - (b.sy-py)*(c.sx-px)) * inv
			w1 := ((c.sx-px)*(a.sy-py) - (c.sy-py)*(a.sx-px)) * inv
			w2 := 1 - w0 - w1
			// Outside the triangle (signs differ) — skip. Matches the JS edge test.
			if (w0 < 0 || w1 < 0 || w2 < 0) && (w0 > 0 || w1 > 0 || w2 > 0) {
				continue
			}
			z := w0*a.z + w1*b.z + w2*c.z
			zi := y*w + x
			if z >= zbuf[zi] {
				continue
			}
			iw := w0*a.iw + w1*b.iw + w2*c.iw
			u := (w0*a.u*a.iw + w1*b.u*b.iw + w2*c.u*c.iw) / iw
			vv := (w0*a.v*a.iw + w1*b.v*b.iw + w2*c.v*c.iw) / iw
			tr, tg, tb, ta := tex.sample(u, vv)
			if mask && ta < 128 {
				continue
			}
			zbuf[zi] = z
			ci := zi * 4
			color[ci+0] = clamp8(float64(tr) * shade)
			color[ci+1] = clamp8(float64(tg) * shade)
			color[ci+2] = clamp8(float64(tb) * shade)
			color[ci+3] = 255
		}
	}
}

func clampi(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
func clamp8(f float64) uint8 {
	if f < 0 {
		return 0
	}
	if f > 255 {
		return 255
	}
	return uint8(f)
}
func min3(a, b, c float64) float64 { return math.Min(a, math.Min(b, c)) }
func max3(a, b, c float64) float64 { return math.Max(a, math.Max(b, c)) }
