package qrkit

import (
	"fmt"
	"image"
	"image/color"
)

// ModuleShape selects how each dark module is drawn.
type ModuleShape int

const (
	// ShapeSquare draws square modules (the standard look).
	ShapeSquare ModuleShape = iota
	// ShapeRounded draws squares whose free corners are rounded; adjacent
	// modules merge into smooth, blob-like shapes.
	ShapeRounded
	// ShapeCircle draws each module as a dot.
	ShapeCircle
	// ShapeDiamond draws each module as a diamond.
	ShapeDiamond
	// ShapeVerticalBars merges vertically adjacent modules into
	// rounded-end bars.
	ShapeVerticalBars
	// ShapeHorizontalBars merges horizontally adjacent modules into
	// rounded-end bars.
	ShapeHorizontalBars
)

func (s ModuleShape) valid() bool { return s >= ShapeSquare && s <= ShapeHorizontalBars }

// FinderShape selects how the three large corner "eyes" are drawn.
type FinderShape int

const (
	// FinderSquare draws classic square finder patterns (the default).
	FinderSquare FinderShape = iota
	// FinderRounded draws finder patterns with rounded corners.
	FinderRounded
	// FinderCircle draws circular finder patterns.
	FinderCircle
	// FinderModules draws finder patterns module by module, using the
	// selected ModuleShape.
	FinderModules
)

func (s FinderShape) valid() bool { return s >= FinderSquare && s <= FinderModules }

// FinderStyle customises the finder patterns.
type FinderStyle struct {
	// Shape of the eyes.
	Shape FinderShape
	// OuterColor is the colour of the outer 7x7 ring. Nil means the
	// foreground colour.
	OuterColor color.Color
	// InnerColor is the colour of the central 3x3 block. Nil means
	// OuterColor if set, otherwise the foreground colour.
	InnerColor color.Color
}

type finderCfg struct {
	shape        FinderShape
	outer, inner *color.NRGBA
}

type config struct {
	level        RecoveryLevel
	levelSet     bool
	minVersion   int
	maxVersion   int
	mask         int
	quiet        int
	size         int
	moduleSize   int
	fg, bg       color.NRGBA
	shape        ModuleShape
	moduleScale  float64
	cornerRadius float64
	finder       finderCfg
	logo         *logoCfg
}

func defaultConfig() config {
	return config{
		level:        LevelMedium,
		minVersion:   MinVersion,
		maxVersion:   MaxVersion,
		mask:         -1,
		quiet:        4,
		size:         512,
		fg:           color.NRGBA{A: 255},
		bg:           color.NRGBA{255, 255, 255, 255},
		moduleScale:  1,
		cornerRadius: 0.4,
	}
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidOption, fmt.Sprintf(format, args...))
}

func (c *config) validate() error {
	switch {
	case !c.level.valid():
		return invalid("recovery level %d", int(c.level))
	case c.minVersion < MinVersion || c.maxVersion > MaxVersion || c.minVersion > c.maxVersion:
		return invalid("version range [%d, %d] must lie within [%d, %d]", c.minVersion, c.maxVersion, MinVersion, MaxVersion)
	case c.mask < -1 || c.mask > 7:
		return invalid("mask %d must be -1 (auto) or 0-7", c.mask)
	case c.quiet < 0:
		return invalid("quiet zone %d must be >= 0", c.quiet)
	case c.moduleSize == 0 && c.size <= 0:
		return invalid("size %d must be > 0", c.size)
	case c.moduleSize < 0:
		return invalid("module size %d must be > 0", c.moduleSize)
	case !c.shape.valid():
		return invalid("module shape %d", int(c.shape))
	case !c.finder.shape.valid():
		return invalid("finder shape %d", int(c.finder.shape))
	case c.moduleScale <= 0 || c.moduleScale > 1:
		return invalid("module scale %v must be in (0, 1]", c.moduleScale)
	case c.cornerRadius < 0 || c.cornerRadius > 0.5:
		return invalid("corner radius %v must be in [0, 0.5]", c.cornerRadius)
	}
	if c.logo != nil {
		return c.logo.validate()
	}
	return nil
}

// Option configures New and NewFromBytes.
type Option func(*config)

func toNRGBA(c color.Color) color.NRGBA { return color.NRGBAModel.Convert(c).(color.NRGBA) }

// WithRecoveryLevel sets the error-correction level (default LevelMedium,
// or LevelHigh when a logo is used and no level was chosen).
func WithRecoveryLevel(l RecoveryLevel) Option {
	return func(c *config) { c.level, c.levelSet = l, true }
}

// WithVersion forces a specific symbol version (1-40). Encoding fails with
// ErrDataTooLong if the content does not fit.
func WithVersion(v int) Option {
	return func(c *config) { c.minVersion, c.maxVersion = v, v }
}

// WithVersionRange restricts automatic version selection to [min, max].
func WithVersionRange(min, max int) Option {
	return func(c *config) { c.minVersion, c.maxVersion = min, max }
}

// WithMask forces a mask pattern (0-7). The default, -1, evaluates all
// eight and picks the best, as the standard recommends.
func WithMask(m int) Option { return func(c *config) { c.mask = m } }

// WithQuietZone sets the blank border around the symbol, in modules.
// The standard requires 4 (the default); scanners may need it.
func WithQuietZone(modules int) Option { return func(c *config) { c.quiet = modules } }

// WithSize sets the output image size in pixels (default 512). The image is
// always exactly px x px; modules get the largest whole pixel size that
// fits and any remainder is added to the quiet zone. SVG output uses it
// as the width and height attributes.
func WithSize(px int) Option { return func(c *config) { c.size, c.moduleSize = px, 0 } }

// WithModuleSize sets the size of one module in pixels, so the image is
// (modules + 2*quiet zone) * px wide. Overrides WithSize (last one wins).
func WithModuleSize(px int) Option { return func(c *config) { c.moduleSize, c.size = px, 0 } }

// WithForeground sets the colour of the dark modules (default black).
func WithForeground(col color.Color) Option {
	return func(c *config) {
		if col != nil {
			c.fg = toNRGBA(col)
		}
	}
}

// WithBackground sets the background colour (default white). The colour
// may be translucent or fully transparent.
func WithBackground(col color.Color) Option {
	return func(c *config) {
		if col != nil {
			c.bg = toNRGBA(col)
		}
	}
}

// WithTransparentBackground makes the background fully transparent.
func WithTransparentBackground() Option {
	return func(c *config) { c.bg = color.NRGBA{} }
}

// WithModuleShape selects the module shape (default ShapeSquare).
func WithModuleShape(s ModuleShape) Option { return func(c *config) { c.shape = s } }

// WithModuleScale shrinks each module inside its cell; 1 (default) fills
// the cell completely, 0.8 leaves a small gap. Values in (0, 1].
func WithModuleScale(s float64) Option { return func(c *config) { c.moduleScale = s } }

// WithCornerRadius sets the corner radius of ShapeRounded as a fraction of
// the module size, in [0, 0.5] (default 0.4).
func WithCornerRadius(r float64) Option { return func(c *config) { c.cornerRadius = r } }

// WithFinderStyle customises the three finder patterns.
func WithFinderStyle(s FinderStyle) Option {
	return func(c *config) {
		c.finder.shape = s.Shape
		c.finder.outer, c.finder.inner = nil, nil
		if s.OuterColor != nil {
			v := toNRGBA(s.OuterColor)
			c.finder.outer = &v
		}
		if s.InnerColor != nil {
			v := toNRGBA(s.InnerColor)
			c.finder.inner = &v
		}
	}
}

// WithLogo places an image in the centre of the code. The modules beneath
// it are left out, and the recovery level defaults to LevelHigh. See the
// Logo* options for size, padding and shape. New fails with
// ErrLogoTooLarge if the logo cannot be placed safely.
func WithLogo(img image.Image, opts ...LogoOption) Option {
	return func(c *config) {
		lc := &logoCfg{img: img, scale: 0.2, padding: 1}
		for _, o := range opts {
			if o != nil {
				o(lc)
			}
		}
		c.logo = lc
	}
}
