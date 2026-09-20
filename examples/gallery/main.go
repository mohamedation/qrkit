// Command gallery renders a set of sample QR codes demonstrating the
// styling options of qrkit. It is used to produce the images in the README.
//
//	go run ./examples/gallery docs/images
package main

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"github.com/mohamedation/qrkit"
)

const content = "https://q.mohamedation.com"

func main() {
	dir := "docs/images"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatal(err)
	}
	logo, err := loadSquareLogo(dir)
	if err != nil {
		log.Fatal(err)
	}
	navy := color.NRGBA{0x1b, 0x2a, 0x49, 255}
	coral := color.NRGBA{0xe8, 0x4a, 0x5f, 255}
	cream := color.NRGBA{0xff, 0xf8, 0xec, 255}
	sage := color.NRGBA{0x6b, 0x7d, 0x7d, 255}
	mint := color.NRGBA{0x8e, 0xaf, 0x9d, 255}

	samples := []struct {
		name string
		opts []qrkit.Option
	}{
		{"default", nil},
		{"colors", []qrkit.Option{qrkit.WithForeground(navy), qrkit.WithBackground(cream)}},
		{"rounded", []qrkit.Option{qrkit.WithModuleShape(qrkit.ShapeRounded), qrkit.WithForeground(navy),
			qrkit.WithFinderStyle(qrkit.FinderStyle{Shape: qrkit.FinderRounded, OuterColor: coral, InnerColor: navy})}},
		{"dots", []qrkit.Option{qrkit.WithModuleShape(qrkit.ShapeCircle), qrkit.WithModuleScale(0.9), qrkit.WithForeground(navy),
			qrkit.WithFinderStyle(qrkit.FinderStyle{Shape: qrkit.FinderCircle, OuterColor: coral})}},
		{"diamonds", []qrkit.Option{qrkit.WithModuleShape(qrkit.ShapeDiamond), qrkit.WithForeground(coral)}},
		{"vbars", []qrkit.Option{qrkit.WithModuleShape(qrkit.ShapeVerticalBars), qrkit.WithForeground(navy),
			qrkit.WithFinderStyle(qrkit.FinderStyle{Shape: qrkit.FinderRounded})}},
		{"hbars", []qrkit.Option{qrkit.WithModuleShape(qrkit.ShapeHorizontalBars), qrkit.WithForeground(coral),
			qrkit.WithFinderStyle(qrkit.FinderStyle{Shape: qrkit.FinderRounded})}},
		{"gaps", []qrkit.Option{qrkit.WithModuleScale(0.8), qrkit.WithModuleShape(qrkit.ShapeRounded), qrkit.WithForeground(navy)}},
		{"transparent", []qrkit.Option{qrkit.WithTransparentBackground(), qrkit.WithForeground(navy), qrkit.WithModuleShape(qrkit.ShapeRounded)}},
		{"logo", []qrkit.Option{qrkit.WithLogo(logo, qrkit.LogoPlate(color.White), qrkit.LogoScale(0.24)), qrkit.WithForeground(sage)}},
		{"logo-circle", []qrkit.Option{qrkit.WithLogo(logo, qrkit.LogoClip(qrkit.LogoCircle), qrkit.LogoScale(0.24), qrkit.LogoPlate(color.White)),
			qrkit.WithModuleShape(qrkit.ShapeCircle), qrkit.WithModuleScale(0.9), qrkit.WithForeground(sage),
			qrkit.WithFinderStyle(qrkit.FinderStyle{Shape: qrkit.FinderCircle, OuterColor: mint})}},
		{"logo-plate", []qrkit.Option{qrkit.WithLogo(logo, qrkit.LogoClip(qrkit.LogoRounded), qrkit.LogoPlate(color.White), qrkit.LogoScale(0.24)),
			qrkit.WithTransparentBackground(), qrkit.WithModuleShape(qrkit.ShapeRounded), qrkit.WithForeground(sage)}},
	}
	for _, s := range samples {
		opts := append([]qrkit.Option{qrkit.WithSize(400)}, s.opts...)
		q, err := qrkit.New(content, opts...)
		if err != nil {
			log.Fatalf("%s: %v", s.name, err)
		}
		must(q.Save(filepath.Join(dir, s.name+".png")))
		fmt.Printf("%-12s version %-2d level %s\n", s.name, q.Version(), q.RecoveryLevel())
		if s.name == "rounded" || s.name == "logo-circle" {
			must(q.Save(filepath.Join(dir, s.name+".svg")))
		}
	}

	// Transparent QR code composited over a gradient poster.
	q, err := qrkit.New(content, qrkit.WithSize(400), qrkit.WithTransparentBackground(),
		qrkit.WithForeground(color.White), qrkit.WithModuleShape(qrkit.ShapeRounded), qrkit.WithQuietZone(2))
	if err != nil {
		log.Fatal(err)
	}
	bg := image.NewRGBA(image.Rect(0, 0, 400, 400))
	for y := 0; y < 400; y++ {
		for x := 0; x < 400; x++ {
			t := float64(x+y) / 800
			bg.Set(x, y, color.RGBA{uint8(40 + 190*t), uint8(60 + 20*t), uint8(180 - 90*t), 255})
		}
	}
	draw.Draw(bg, bg.Bounds(), q.Image(), image.Point{}, draw.Over)
	f, err := os.Create(filepath.Join(dir, "on-gradient.png"))
	must(err)
	must(png.Encode(f, bg))
	must(f.Close())
}

func loadSquareLogo(dir string) (image.Image, error) {
	for _, p := range []string{
		filepath.Join(dir, "qrkit_logo_sq.png"),
		"docs/images/qrkit_logo_sq.png",
	} {
		f, err := os.Open(p)
		if err != nil {
			continue
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			return nil, err
		}
		return padLogo(img, 0.12), nil
	}
	return nil, fmt.Errorf("qrkit_logo_sq.png not found (looked in %s and docs/images)", dir)
}

// padLogo adds a transparent margin so circular/rounded clips do not crop the wordmark.
func padLogo(src image.Image, frac float64) image.Image {
	b := src.Bounds()
	pad := int(float64(b.Dx()) * frac)
	dst := image.NewNRGBA(image.Rect(0, 0, b.Dx()+2*pad, b.Dy()+2*pad))
	draw.Draw(dst, image.Rect(pad, pad, pad+b.Dx(), pad+b.Dy()), src, b.Min, draw.Over)
	return dst
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
