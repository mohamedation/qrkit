package qrkit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// QRCode is an encoded QR code together with its rendering style.
// A QRCode is immutable and safe for concurrent use.
type QRCode struct {
	version int
	level   RecoveryLevel
	mask    int
	size    int
	modules []bool // size*size, row-major, true = dark
	cfg     config
	zone    *logoZone
}

// New encodes content as a QR code. The most compact of the numeric,
// alphanumeric and byte (UTF-8) modes is chosen automatically, as is the
// smallest version that fits.
func New(content string, opts ...Option) (*QRCode, error) {
	return NewFromBytes([]byte(content), opts...)
}

// NewFromBytes is like New for arbitrary binary data.
func NewFromBytes(data []byte, opts ...Option) (*QRCode, error) {
	cfg := defaultConfig()
	for _, o := range opts {
		if o != nil {
			o(&cfg)
		}
	}
	if err := cfg.validate(); err != nil {
		return nil, err
	}
	if len(data) == 0 {
		return nil, ErrEmptyData
	}

	level := cfg.level
	if cfg.logo != nil && !cfg.levelSet {
		level = LevelHigh
	}
	m := chooseMode(data)

	var (
		version int
		fits    bool
		logoErr error
		zone    *logoZone
	)
	for v := cfg.minVersion; v <= cfg.maxVersion; v++ {
		if totalBits(m, len(data), v) > numDataCodewords(v, level)*8 {
			continue
		}
		fits = true
		if cfg.logo != nil {
			z, err := planLogo(cfg.logo, v, level)
			if err != nil {
				logoErr = err
				continue
			}
			zone = z
		}
		version = v
		break
	}
	if version == 0 {
		if fits && logoErr != nil {
			return nil, logoErr
		}
		return nil, fmt.Errorf("%w: %d bytes do not fit in versions %d-%d at level %s",
			ErrDataTooLong, len(data), cfg.minVersion, cfg.maxVersion, level)
	}

	all := interleaveWithECC(encodeDataCodewords(data, m, version, level), version, level)
	lay := getLayout(version)
	modules, mask := buildModules(lay, all, level, cfg.mask)

	q := &QRCode{version: version, level: level, mask: mask, size: lay.size, modules: modules, cfg: cfg, zone: zone}
	if cfg.moduleSize == 0 && cfg.size < q.totalModules() {
		return nil, invalid("size %dpx is too small for %d modules (need at least 1px per module)", cfg.size, q.totalModules())
	}
	return q, nil
}

// Version returns the symbol version (1-40).
func (q *QRCode) Version() int { return q.version }

// RecoveryLevel returns the error-correction level actually used.
func (q *QRCode) RecoveryLevel() RecoveryLevel { return q.level }

// Mask returns the mask pattern (0-7) that was applied.
func (q *QRCode) Mask() int { return q.mask }

// Size returns the side length in modules, excluding the quiet zone.
func (q *QRCode) Size() int { return q.size }

// IsDark reports whether the module at column x, row y is dark. Coordinates
// outside the symbol (the quiet zone) report false. The logo area is not
// taken into account; use Matrix for the raw symbol.
func (q *QRCode) IsDark(x, y int) bool {
	if x < 0 || y < 0 || x >= q.size || y >= q.size {
		return false
	}
	return q.modules[y*q.size+x]
}

// Matrix returns a copy of the module matrix, indexed [row][column], with
// true for dark modules and no quiet zone.
func (q *QRCode) Matrix() [][]bool {
	m := make([][]bool, q.size)
	for y := range m {
		m[y] = append([]bool(nil), q.modules[y*q.size:(y+1)*q.size]...)
	}
	return m
}

func (q *QRCode) totalModules() int { return q.size + 2*q.cfg.quiet }

// pixelLayout returns pixels per module, the pixel offset of the quiet-zone
// origin and the image width/height in pixels.
func (q *QRCode) pixelLayout() (ms, off, px int) {
	total := q.totalModules()
	if q.cfg.moduleSize > 0 {
		return q.cfg.moduleSize, 0, q.cfg.moduleSize * total
	}
	ms = q.cfg.size / total
	return ms, (q.cfg.size - ms*total) / 2, q.cfg.size
}

// Save writes the code to a file; the format is chosen by the extension
// (".png" or ".svg", case-insensitive). The destination is replaced only
// after a successful write, so a failure does not leave a partial file.
func (q *QRCode) Save(path string) error {
	var write func(f *os.File) error
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		write = func(f *os.File) error { return q.WritePNG(f) }
	case ".svg":
		write = func(f *os.File) error { return q.WriteSVG(f) }
	default:
		return fmt.Errorf("qrkit: unsupported file extension %q (use .png or .svg)", filepath.Ext(path))
	}
	return writeFileAtomic(path, write)
}

func writeFileAtomic(path string, write func(*os.File) error) error {
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	f, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	removeTmp := func() { _ = os.Remove(tmp) }

	if err := write(f); err != nil {
		f.Close()
		removeTmp()
		return err
	}
	if err := f.Close(); err != nil {
		removeTmp()
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			removeTmp()
			return err
		}
	}
	return nil
}
