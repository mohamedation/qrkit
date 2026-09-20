package qrkit

import (
	"image"
	"image/color"
	"math"
)

// prim is a filled shape (rounded rectangle or diamond, optionally with a
// rounded-rectangle hole) used as the common intermediate representation
// for both the raster and vector renderers.
type prim struct {
	diamond bool
	cx, cy  float64
	hx, hy  float64    // half extents (half diagonal for diamonds)
	r       [4]float64 // corner radii: top-left, top-right, bottom-right, bottom-left
	hole    *prim
	fill    color.NRGBA
}

func (p *prim) ownSDF(x, y float64) float64 {
	dx, dy := x-p.cx, y-p.cy
	if p.diamond {
		return (math.Abs(dx) + math.Abs(dy) - p.hx) * math.Sqrt2 / 2
	}
	var r float64
	switch {
	case dx < 0 && dy < 0:
		r = p.r[0]
	case dx >= 0 && dy < 0:
		r = p.r[1]
	case dx >= 0:
		r = p.r[2]
	default:
		r = p.r[3]
	}
	qx, qy := math.Abs(dx)-p.hx+r, math.Abs(dy)-p.hy+r
	return math.Min(math.Max(qx, qy), 0) + math.Hypot(math.Max(qx, 0), math.Max(qy, 0)) - r
}

// sdf is the signed distance to the shape edge (negative inside).
func (p *prim) sdf(x, y float64) float64 {
	d := p.ownSDF(x, y)
	if p.hole != nil {
		if h := -p.hole.ownSDF(x, y); h > d {
			d = h
		}
	}
	return d
}

func (p *prim) plain() bool {
	return !p.diamond && p.hole == nil && p.r == [4]float64{}
}

func (p prim) transform(ox, oy, s float64) prim {
	n := p
	n.cx, n.cy = ox+p.cx*s, oy+p.cy*s
	n.hx, n.hy = p.hx*s, p.hy*s
	for i := range n.r {
		n.r[i] *= s
	}
	if p.hole != nil {
		h := p.hole.transform(ox, oy, s)
		n.hole = &h
	}
	return n
}

// blend composites colour c with coverage cov onto premultiplied dst pixel.
func blend(dst *image.RGBA, x, y int, c color.NRGBA, cov float64) {
	a := float64(c.A) / 255 * cov
	if a <= 0 {
		return
	}
	i := dst.PixOffset(x, y)
	inv := 1 - a
	p := dst.Pix[i : i+4 : i+4]
	p[0] = clampByte(float64(c.R)*a + float64(p[0])*inv)
	p[1] = clampByte(float64(c.G)*a + float64(p[1])*inv)
	p[2] = clampByte(float64(c.B)*a + float64(p[2])*inv)
	p[3] = clampByte(255*a + float64(p[3])*inv)
}

func clampByte(v float64) uint8 {
	v += 0.5
	if v >= 255 {
		return 255
	}
	return uint8(v)
}

// rasterize draws a pixel-space primitive with anti-aliased edges.
func (p *prim) rasterize(dst *image.RGBA) {
	b := dst.Bounds()
	x0 := maxInt(b.Min.X, int(math.Floor(p.cx-p.hx-1)))
	x1 := minInt(b.Max.X, int(math.Ceil(p.cx+p.hx+1)))
	y0 := maxInt(b.Min.Y, int(math.Floor(p.cy-p.hy-1)))
	y1 := minInt(b.Max.Y, int(math.Ceil(p.cy+p.hy+1)))
	whole := p.plain() && isWhole(p.cx-p.hx) && isWhole(p.cx+p.hx) && isWhole(p.cy-p.hy) && isWhole(p.cy+p.hy)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if whole {
				if float64(x) >= p.cx-p.hx && float64(x) < p.cx+p.hx && float64(y) >= p.cy-p.hy && float64(y) < p.cy+p.hy {
					blend(dst, x, y, p.fill, 1)
				}
				continue
			}
			if cov := clamp01(0.5 - p.sdf(float64(x)+0.5, float64(y)+0.5)); cov > 0 {
				blend(dst, x, y, p.fill, cov)
			}
		}
	}
}

func isWhole(v float64) bool { return math.Abs(v-math.Round(v)) < 1e-9 }

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

const barThickness = 0.8 // bar width as a fraction of the module

// prims converts the symbol into shapes in module units (origin at the
// symbol's top-left corner, quiet zone excluded).
func (q *QRCode) prims() []prim {
	size, cfg := q.size, &q.cfg
	scale := cfg.moduleScale
	half := scale / 2
	customFinder := cfg.finder.shape != FinderModules
	inFinder := func(x, y int) bool {
		return customFinder && ((x < 7 && y < 7) || (x >= size-7 && y < 7) || (x < 7 && y >= size-7))
	}
	drawn := func(x, y int) bool {
		if x < 0 || y < 0 || x >= size || y >= size || !q.modules[y*size+x] || inFinder(x, y) {
			return false
		}
		return q.zone == nil || !q.zone.clears(x, y)
	}
	connected := scale > 0.999

	var out []prim
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			if !drawn(x, y) {
				continue
			}
			p := prim{cx: float64(x) + 0.5, cy: float64(y) + 0.5, hx: half, hy: half, fill: cfg.fg}
			up := connected && drawn(x, y-1)
			down := connected && drawn(x, y+1)
			left := connected && drawn(x-1, y)
			right := connected && drawn(x+1, y)
			switch cfg.shape {
			case ShapeCircle:
				p.r = [4]float64{half, half, half, half}
			case ShapeRounded:
				r := cfg.cornerRadius * scale
				if !up && !left {
					p.r[0] = r
				}
				if !up && !right {
					p.r[1] = r
				}
				if !down && !right {
					p.r[2] = r
				}
				if !down && !left {
					p.r[3] = r
				}
			case ShapeDiamond:
				p.diamond = true
			case ShapeVerticalBars:
				p.hx = half * barThickness
				if !up {
					p.r[0], p.r[1] = p.hx, p.hx
				}
				if !down {
					p.r[2], p.r[3] = p.hx, p.hx
				}
			case ShapeHorizontalBars:
				p.hy = half * barThickness
				if !left {
					p.r[0], p.r[3] = p.hy, p.hy
				}
				if !right {
					p.r[1], p.r[2] = p.hy, p.hy
				}
			}
			out = append(out, p)
		}
	}

	if customFinder {
		outerCol, innerCol := cfg.fg, cfg.fg
		if cfg.finder.outer != nil {
			outerCol, innerCol = *cfg.finder.outer, *cfg.finder.outer
		}
		if cfg.finder.inner != nil {
			innerCol = *cfg.finder.inner
		}
		var ro, rh, ri float64
		switch cfg.finder.shape {
		case FinderRounded:
			ro, rh, ri = 2, 1, 1
		case FinderCircle:
			ro, rh, ri = 3.5, 2.5, 1.5
		}
		for _, o := range [][2]int{{0, 0}, {size - 7, 0}, {0, size - 7}} {
			cx, cy := float64(o[0])+3.5, float64(o[1])+3.5
			hole := prim{cx: cx, cy: cy, hx: 2.5, hy: 2.5, r: [4]float64{rh, rh, rh, rh}}
			out = append(out,
				prim{cx: cx, cy: cy, hx: 3.5, hy: 3.5, r: [4]float64{ro, ro, ro, ro}, hole: &hole, fill: outerCol},
				prim{cx: cx, cy: cy, hx: 1.5, hy: 1.5, r: [4]float64{ri, ri, ri, ri}, fill: innerCol},
			)
		}
	}
	return out
}
