package qrkit

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
)

// LogoShape is the shape of the logo's clipping mask and backing plate.
type LogoShape int

const (
	// LogoSquare leaves the logo unclipped (rectangular).
	LogoSquare LogoShape = iota
	// LogoRounded clips the logo to a rounded rectangle.
	LogoRounded
	// LogoCircle clips the logo to a circle.
	LogoCircle
)

const (
	logoRoundedRadius = 0.2 // corner radius as a fraction of the shorter side
	logoErrorBudget   = 0.6 // fraction of each block's correction capacity a logo may use
)

// LogoOption customises WithLogo.
type LogoOption func(*logoCfg)

type logoCfg struct {
	img     image.Image
	scale   float64
	padding float64
	shape   LogoShape
	plate   *color.NRGBA
	unsafe  bool
}

// LogoScale sets the logo's longer side as a fraction of the symbol width,
// in (0, 0.5]. Default 0.2.
func LogoScale(s float64) LogoOption { return func(c *logoCfg) { c.scale = s } }

// LogoPadding sets the empty margin around the logo, in modules. Default 1.
func LogoPadding(modules float64) LogoOption { return func(c *logoCfg) { c.padding = modules } }

// LogoClip sets the logo's clipping/plate shape. Default LogoSquare.
func LogoClip(s LogoShape) LogoOption { return func(c *logoCfg) { c.shape = s } }

// LogoPlate paints a solid plate of the given colour behind the logo, which
// helps logos with transparency stand out on busy or transparent backgrounds.
func LogoPlate(col color.Color) LogoOption {
	return func(c *logoCfg) {
		if col != nil {
			v := toNRGBA(col)
			c.plate = &v
		}
	}
}

// LogoAllowUnsafe disables the error-correction budget check. The code may
// then be impossible to scan; the finder/timing/format patterns are still
// protected. Use only if you test the result.
func LogoAllowUnsafe() LogoOption { return func(c *logoCfg) { c.unsafe = true } }

func (l *logoCfg) validate() error {
	switch {
	case l.img == nil || l.img.Bounds().Empty():
		return invalid("logo image is nil or empty")
	case l.scale <= 0 || l.scale > 0.5:
		return invalid("logo scale %v must be in (0, 0.5]", l.scale)
	case l.padding < 0:
		return invalid("logo padding %v must be >= 0", l.padding)
	case l.shape < LogoSquare || l.shape > LogoCircle:
		return invalid("logo shape %d", int(l.shape))
	}
	return nil
}

// logoZone is the placed logo: the block of cleared modules plus the exact
// image box, all in module units.
type logoZone struct {
	cfg            *logoCfg
	x0, y0, x1, y1 int     // cleared area, in whole modules (end exclusive)
	lw, lh         float64 // image size, in modules
	side           float64 // longer side, used for the circular clip
}

// clears reports whether module (x, y) is removed for the logo.
func (z *logoZone) clears(x, y int) bool {
	if x < z.x0 || x >= z.x1 || y < z.y0 || y >= z.y1 {
		return false
	}
	if z.cfg.shape != LogoCircle {
		return true
	}
	cx, cy := float64(z.x0+z.x1)/2, float64(z.y0+z.y1)/2
	r := float64(z.x1-z.x0) / 2
	dx := math.Max(math.Max(float64(x)-cx, cx-float64(x+1)), 0)
	dy := math.Max(math.Max(float64(y)-cy, cy-float64(y+1)), 0)
	return dx*dx+dy*dy < r*r
}

// plateShape returns the cleared area as a primitive in module units.
func (z *logoZone) plateShape(fill color.NRGBA) prim {
	p := prim{
		cx: float64(z.x0+z.x1) / 2, cy: float64(z.y0+z.y1) / 2,
		hx: float64(z.x1-z.x0) / 2, hy: float64(z.y1-z.y0) / 2,
		fill: fill,
	}
	switch z.cfg.shape {
	case LogoRounded:
		r := logoRoundedRadius * math.Min(p.hx, p.hy) * 2
		p.r = [4]float64{r, r, r, r}
	case LogoCircle:
		p.r = [4]float64{p.hx, p.hx, p.hx, p.hx}
	}
	return p
}

// planLogo sizes and positions the logo for a version and verifies it is
// safe. It returns an error wrapping ErrLogoTooLarge if it is not.
func planLogo(lc *logoCfg, version int, level RecoveryLevel) (*logoZone, error) {
	size := symbolSize(version)
	b := lc.img.Bounds()
	iw, ih := float64(b.Dx()), float64(b.Dy())
	side := lc.scale * float64(size)
	lw, lh := side, side
	if iw >= ih {
		lh = side * ih / iw
	} else {
		lw = side * iw / ih
	}
	nw, nh := oddCeil(lw+2*lc.padding), oddCeil(lh+2*lc.padding)
	if lc.shape == LogoCircle {
		n := oddCeil(side + 2*lc.padding)
		nw, nh = n, n
	}
	if nw > size-16 || nh > size-16 {
		return nil, fmt.Errorf("%w: needs %dx%d modules in a version-%d symbol", ErrLogoTooLarge, nw, nh, version)
	}
	z := &logoZone{
		cfg: lc, lw: lw, lh: lh, side: side,
		x0: (size - nw) / 2, y0: (size - nh) / 2,
	}
	z.x1, z.y1 = z.x0+nw, z.y0+nh

	lay := getLayout(version)
	bi := newBlockInfo(version, level)
	blockOf, _ := bi.order()
	seen := make([]bool, bi.rawCW)
	counts := make([]int, bi.numBlocks)
	for y := z.y0; y < z.y1; y++ {
		for x := z.x0; x < z.x1; x++ {
			if !z.clears(x, y) {
				continue
			}
			i := y*size + x
			switch lay.kind[i] {
			case kindData:
				if bit := lay.bitIdx[i]; bit >= 0 && !seen[bit>>3] {
					seen[bit>>3] = true
					counts[blockOf[bit>>3]]++
				}
			case kindAlign:
				// Decoders tolerate a missing alignment pattern.
			default:
				return nil, fmt.Errorf("%w: it would cover structural patterns of a version-%d symbol", ErrLogoTooLarge, version)
			}
		}
	}
	if !lc.unsafe {
		allowed := int(float64(bi.eccLen/2) * logoErrorBudget)
		for _, c := range counts {
			if c > allowed {
				hint := "reduce LogoScale"
				if level != LevelHigh {
					hint += " or raise the recovery level"
				}
				return nil, fmt.Errorf("%w: it would damage more than the %s recovery level can safely correct in a version-%d symbol (%s)",
					ErrLogoTooLarge, level, version, hint)
			}
		}
	}
	return z, nil
}

// image renders the logo at pw x ph pixels with the clip mask applied,
// as a premultiplied RGBA image.
func (z *logoZone) image(pw, ph int) *image.RGBA {
	if pw < 1 {
		pw = 1
	}
	if ph < 1 {
		ph = 1
	}
	out := resizeRGBA(z.cfg.img, pw, ph)
	if z.cfg.shape == LogoSquare {
		return out
	}
	m := prim{cx: float64(pw) / 2, cy: float64(ph) / 2, hx: float64(pw) / 2, hy: float64(ph) / 2}
	switch z.cfg.shape {
	case LogoRounded:
		r := logoRoundedRadius * math.Min(float64(pw), float64(ph))
		m.r = [4]float64{r, r, r, r}
	case LogoCircle:
		d := math.Max(float64(pw), float64(ph)) / 2
		m.hx, m.hy = d, d
		m.r = [4]float64{d, d, d, d}
	}
	for y := 0; y < ph; y++ {
		for x := 0; x < pw; x++ {
			cov := clamp01(0.5 - m.sdf(float64(x)+0.5, float64(y)+0.5))
			if cov >= 1 {
				continue
			}
			i := out.PixOffset(x, y)
			for k := 0; k < 4; k++ {
				out.Pix[i+k] = uint8(float64(out.Pix[i+k])*cov + 0.5)
			}
		}
	}
	return out
}

// toRGBA copies any image into a premultiplied RGBA image at the origin.
func toRGBA(src image.Image) *image.RGBA {
	b := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(dst, dst.Bounds(), src, b.Min, draw.Src)
	return dst
}
