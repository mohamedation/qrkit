package qrkit_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/mohamedation/qrkit"
)

func testLogo() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, 120, 80))
	for y := 0; y < 80; y++ {
		for x := 0; x < 120; x++ {
			img.SetNRGBA(x, y, color.NRGBA{200, uint8(x), uint8(y), 255})
		}
	}
	return img
}

func TestVersionSelection(t *testing.T) {
	cases := []struct {
		content string
		opts    []qrkit.Option
		version int
	}{
		{"HELLO WORLD", nil, 1},
		{"0123456789", nil, 1},
		{strings.Repeat("a", 14), nil, 1}, // 14 bytes fit 1-M
		{strings.Repeat("a", 15), nil, 2},
		{"HELLO WORLD", []qrkit.Option{qrkit.WithVersion(5)}, 5},
		{"HELLO WORLD", []qrkit.Option{qrkit.WithVersionRange(3, 9)}, 3},
		{strings.Repeat("9", 7089), []qrkit.Option{qrkit.WithRecoveryLevel(qrkit.LevelLow)}, 40}, // max numeric capacity
	}
	for _, c := range cases {
		q, err := qrkit.New(c.content, append(c.opts, qrkit.WithModuleSize(1))...)
		if err != nil {
			t.Fatalf("%.20q: %v", c.content, err)
		}
		if q.Version() != c.version {
			t.Errorf("%.20q: version %d, want %d", c.content, q.Version(), c.version)
		}
		if q.Size() != 17+4*q.Version() {
			t.Errorf("size %d for version %d", q.Size(), q.Version())
		}
	}
}

func TestErrors(t *testing.T) {
	_, err := qrkit.New("")
	if !errors.Is(err, qrkit.ErrEmptyData) {
		t.Errorf("empty: %v", err)
	}
	_, err = qrkit.New(strings.Repeat("9", 7090), qrkit.WithRecoveryLevel(qrkit.LevelLow))
	if !errors.Is(err, qrkit.ErrDataTooLong) {
		t.Errorf("too long: %v", err)
	}
	_, err = qrkit.New(strings.Repeat("a", 30), qrkit.WithVersion(1))
	if !errors.Is(err, qrkit.ErrDataTooLong) {
		t.Errorf("forced version: %v", err)
	}
	bad := []qrkit.Option{
		qrkit.WithVersion(0), qrkit.WithVersion(41), qrkit.WithVersionRange(5, 3),
		qrkit.WithMask(8), qrkit.WithQuietZone(-1), qrkit.WithSize(-5), qrkit.WithModuleSize(-1),
		qrkit.WithModuleScale(0), qrkit.WithModuleScale(1.5), qrkit.WithCornerRadius(0.9),
		qrkit.WithRecoveryLevel(9), qrkit.WithModuleShape(99), qrkit.WithLogo(nil),
		qrkit.WithLogo(testLogo(), qrkit.LogoScale(0.9)), qrkit.WithLogo(testLogo(), qrkit.LogoPadding(-1)),
	}
	for i, o := range bad {
		if _, err := qrkit.New("x", o); !errors.Is(err, qrkit.ErrInvalidOption) {
			t.Errorf("bad option #%d: got %v", i, err)
		}
	}
	// Too small to give each module a pixel.
	if _, err := qrkit.New("x", qrkit.WithSize(10)); !errors.Is(err, qrkit.ErrInvalidOption) {
		t.Errorf("tiny size: %v", err)
	}
}

func TestDeterministic(t *testing.T) {
	a, _ := qrkit.New("https://example.com/path?q=1")
	b, _ := qrkit.New("https://example.com/path?q=1")
	if fmt.Sprint(a.Matrix()) != fmt.Sprint(b.Matrix()) {
		t.Error("same input produced different matrices")
	}
}

// Golden regression test: the matrix for a fixed input must not change.
func TestGoldenMatrix(t *testing.T) {
	q, err := qrkit.New("HELLO WORLD", qrkit.WithRecoveryLevel(qrkit.LevelMedium))
	if err != nil {
		t.Fatal(err)
	}
	var sb strings.Builder
	for _, row := range q.Matrix() {
		for _, d := range row {
			if d {
				sb.WriteByte('1')
			} else {
				sb.WriteByte('0')
			}
		}
	}
	got := fmt.Sprintf("%x", sha256.Sum256([]byte(sb.String())))
	const want = "9f15327b648fdbb6e3878c9a72183cc1f76f880c1b3e4f53fd7859bb5b4f2421"
	if got != want {
		t.Errorf("golden mismatch: %s", got)
	}
}

func TestStructure(t *testing.T) {
	q, _ := qrkit.New("structure", qrkit.WithVersion(7))
	n := q.Size()
	// Finder pattern rings.
	for _, o := range [][2]int{{0, 0}, {n - 7, 0}, {0, n - 7}} {
		for dy := 0; dy < 7; dy++ {
			for dx := 0; dx < 7; dx++ {
				want := dx == 0 || dx == 6 || dy == 0 || dy == 6 || (dx >= 2 && dx <= 4 && dy >= 2 && dy <= 4)
				if q.IsDark(o[0]+dx, o[1]+dy) != want {
					t.Fatalf("finder at %v mismatch (%d,%d)", o, dx, dy)
				}
			}
		}
	}
	// Timing patterns and the dark module.
	for i := 8; i < n-8; i++ {
		if q.IsDark(i, 6) != (i%2 == 0) || q.IsDark(6, i) != (i%2 == 0) {
			t.Fatalf("timing pattern wrong at %d", i)
		}
	}
	if !q.IsDark(8, n-8) {
		t.Error("dark module missing")
	}
	if q.IsDark(-1, 0) || q.IsDark(0, n) {
		t.Error("out-of-range modules must be light")
	}
	// Matrix returns a copy.
	m := q.Matrix()
	m[0][0] = !m[0][0]
	if m[0][0] == q.IsDark(0, 0) {
		t.Error("Matrix leaks internal state")
	}
}

func TestMaskOverride(t *testing.T) {
	for m := 0; m < 8; m++ {
		q, err := qrkit.New("mask", qrkit.WithMask(m))
		if err != nil || q.Mask() != m {
			t.Errorf("mask %d: got %v, %v", m, q, err)
		}
	}
}

func TestPNGSizes(t *testing.T) {
	q, err := qrkit.New("size", qrkit.WithSize(300))
	if err != nil {
		t.Fatal(err)
	}
	if b := q.Image().Bounds(); b.Dx() != 300 || b.Dy() != 300 {
		t.Errorf("got %v, want 300x300", b)
	}
	q, _ = qrkit.New("size", qrkit.WithModuleSize(6), qrkit.WithQuietZone(2))
	want := (q.Size() + 4) * 6
	if b := q.Image().Bounds(); b.Dx() != want {
		t.Errorf("got %d, want %d", b.Dx(), want)
	}
	data, err := q.PNG()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := png.Decode(bytes.NewReader(data)); err != nil {
		t.Errorf("PNG does not decode: %v", err)
	}
}

func TestColorsAndTransparency(t *testing.T) {
	red := color.NRGBA{200, 10, 20, 255}
	q, _ := qrkit.New("colors", qrkit.WithModuleSize(8), qrkit.WithForeground(red), qrkit.WithTransparentBackground())
	img := q.Image().(*image.NRGBA)
	if a := img.NRGBAAt(0, 0).A; a != 0 {
		t.Errorf("corner alpha = %d, want 0 (transparent)", a)
	}
	// Centre of the top-left finder's outer ring is a dark module.
	x, y := (4*8)+4, (4*8)+4
	if got := img.NRGBAAt(x, y); got != red {
		t.Errorf("dark pixel = %v, want %v", got, red)
	}
	q, _ = qrkit.New("colors", qrkit.WithModuleSize(8), qrkit.WithBackground(color.NRGBA{10, 20, 30, 255}))
	if got := q.Image().(*image.NRGBA).NRGBAAt(0, 0); got != (color.NRGBA{10, 20, 30, 255}) {
		t.Errorf("background = %v", got)
	}
}

func TestAllShapesRender(t *testing.T) {
	shapes := []qrkit.ModuleShape{qrkit.ShapeSquare, qrkit.ShapeRounded, qrkit.ShapeCircle, qrkit.ShapeDiamond, qrkit.ShapeVerticalBars, qrkit.ShapeHorizontalBars}
	finders := []qrkit.FinderShape{qrkit.FinderSquare, qrkit.FinderRounded, qrkit.FinderCircle, qrkit.FinderModules}
	for _, s := range shapes {
		for _, f := range finders {
			q, err := qrkit.New("shapes", qrkit.WithModuleShape(s), qrkit.WithFinderStyle(qrkit.FinderStyle{Shape: f}), qrkit.WithModuleSize(5))
			if err != nil {
				t.Fatal(err)
			}
			if _, err := q.PNG(); err != nil {
				t.Errorf("png %d/%d: %v", s, f, err)
			}
			if _, err := q.SVG(); err != nil {
				t.Errorf("svg %d/%d: %v", s, f, err)
			}
		}
	}
}

func TestSVG(t *testing.T) {
	q, _ := qrkit.New("svg", qrkit.WithModuleShape(qrkit.ShapeCircle), qrkit.WithLogo(testLogo(), qrkit.LogoClip(qrkit.LogoCircle), qrkit.LogoPlate(color.White)),
		qrkit.WithTransparentBackground(), qrkit.WithSize(300))
	s, err := q.SVG()
	if err != nil {
		t.Fatal(err)
	}
	dec := xml.NewDecoder(strings.NewReader(s))
	for {
		if _, err := dec.Token(); err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("SVG is not well-formed XML: %v", err)
		}
	}
	for _, want := range []string{`viewBox="0 0 `, `width="300"`, `<image `, "data:image/png;base64,"} {
		if !strings.Contains(s, want) {
			t.Errorf("SVG missing %q", want)
		}
	}
	if strings.Contains(s, "<rect") {
		t.Error("transparent background must not emit a background rect")
	}
}

func TestLogo(t *testing.T) {
	q, err := qrkit.New("https://example.com", qrkit.WithLogo(testLogo()))
	if err != nil {
		t.Fatal(err)
	}
	if q.RecoveryLevel() != qrkit.LevelHigh {
		t.Errorf("level = %v, want H by default with a logo", q.RecoveryLevel())
	}
	// Explicit level is respected.
	q, err = qrkit.New("https://example.com", qrkit.WithLogo(testLogo(), qrkit.LogoScale(0.1)), qrkit.WithRecoveryLevel(qrkit.LevelQuartile))
	if err != nil || q.RecoveryLevel() != qrkit.LevelQuartile {
		t.Errorf("explicit level: %v %v", q, err)
	}
	// Oversized logos are rejected...
	_, err = qrkit.New("https://example.com", qrkit.WithLogo(testLogo(), qrkit.LogoScale(0.5)))
	if !errors.Is(err, qrkit.ErrLogoTooLarge) {
		t.Errorf("large logo: %v", err)
	}
	// ...unless the caller opts out of the safety check.
	if _, err = qrkit.New("https://example.com", qrkit.WithLogo(testLogo(), qrkit.LogoScale(0.4), qrkit.LogoAllowUnsafe())); err != nil {
		t.Errorf("unsafe logo: %v", err)
	}
	// A logo forces a larger version when the small one cannot hold it safely.
	small, _ := qrkit.New("hi")
	withLogo, err := qrkit.New("hi", qrkit.WithLogo(testLogo()))
	if err != nil || withLogo.Version() <= small.Version() {
		t.Errorf("expected version bump, got %v (%v)", withLogo, err)
	}
	// Logo pixels must actually appear in the image.
	q, _ = qrkit.New("https://example.com", qrkit.WithLogo(testLogo()), qrkit.WithModuleSize(10))
	b := q.Image().Bounds()
	c := q.Image().(*image.NRGBA).NRGBAAt(b.Dx()/2, b.Dy()/2)
	if c.A != 255 || c.R < 150 {
		t.Errorf("centre pixel %v does not look like the logo", c)
	}
}

func TestSave(t *testing.T) {
	dir := t.TempDir()
	q, _ := qrkit.New("save")
	for _, name := range []string{"a.png", "b.SVG"} {
		p := filepath.Join(dir, name)
		if err := q.Save(p); err != nil {
			t.Fatal(err)
		}
		if st, err := os.Stat(p); err != nil || st.Size() == 0 {
			t.Errorf("%s not written", name)
		}
	}
	if err := q.Save(filepath.Join(dir, "c.jpg")); err == nil {
		t.Error("expected error for unsupported extension")
	}
}

func TestConcurrentUse(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			q, err := qrkit.New(fmt.Sprintf("concurrent-%d", i), qrkit.WithLogo(testLogo(), qrkit.LogoScale(0.15)))
			if err != nil {
				t.Error(err)
				return
			}
			_, _ = q.PNG()
			_, _ = q.SVG()
		}(i)
	}
	wg.Wait()
}

func TestNewFromBytesBinary(t *testing.T) {
	data := []byte{0x00, 0xFF, 0x10, 0x80, 0x7F}
	if _, err := qrkit.NewFromBytes(data); err != nil {
		t.Fatal(err)
	}
}

func BenchmarkEncodeSmall(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := qrkit.New("https://example.com/a/b?c=d"); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeLarge(b *testing.B) {
	s := strings.Repeat("abcdefghij", 200)
	for i := 0; i < b.N; i++ {
		if _, err := qrkit.New(s); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkPNG512(b *testing.B) {
	q, _ := qrkit.New("https://example.com/a/b?c=d", qrkit.WithModuleShape(qrkit.ShapeRounded))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := q.PNG(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSVG(b *testing.B) {
	q, _ := qrkit.New("https://example.com/a/b?c=d", qrkit.WithModuleShape(qrkit.ShapeRounded))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := q.SVG(); err != nil {
			b.Fatal(err)
		}
	}
}
