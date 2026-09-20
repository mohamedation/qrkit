package qrkit_test

import (
	"fmt"
	"image"
	"image/color"

	"github.com/mohamedation/qrkit"
)

func Example() {
	qr, err := qrkit.New("https://q.mohamedation.com")
	if err != nil {
		panic(err)
	}
	fmt.Printf("version %d, %dx%d modules, level %s\n", qr.Version(), qr.Size(), qr.Size(), qr.RecoveryLevel())
	// qr.Save("code.png") writes a PNG; "code.svg" writes an SVG.
	// Output: version 2, 25x25 modules, level M
}

func ExampleNew_styled() {
	qr, err := qrkit.New("https://q.mohamedation.com",
		qrkit.WithSize(600),
		qrkit.WithForeground(color.NRGBA{0x1b, 0x2a, 0x49, 0xff}),
		qrkit.WithTransparentBackground(),
		qrkit.WithModuleShape(qrkit.ShapeRounded),
		qrkit.WithFinderStyle(qrkit.FinderStyle{
			Shape:      qrkit.FinderRounded,
			OuterColor: color.NRGBA{0xe8, 0x4a, 0x5f, 0xff},
		}),
	)
	if err != nil {
		panic(err)
	}
	b := qr.Image().Bounds()
	fmt.Println(b.Dx(), b.Dy())
	// Output: 600 600
}

func ExampleWithLogo() {
	// Any image.Image works: decode a PNG/JPEG with image.Decode, or draw one.
	logo := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for i := range logo.Pix {
		logo.Pix[i] = 0xff
	}
	qr, err := qrkit.New("https://q.mohamedation.com",
		qrkit.WithLogo(logo, qrkit.LogoScale(0.2), qrkit.LogoClip(qrkit.LogoCircle), qrkit.LogoPlate(color.White)),
	)
	if err != nil {
		panic(err)
	}
	// With a logo the recovery level defaults to H.
	fmt.Println(qr.RecoveryLevel())
	// Output: H
}

func ExampleQRCode_Matrix() {
	qr, _ := qrkit.New("HELLO WORLD")
	m := qr.Matrix()
	// Print the top-left finder pattern's first row.
	for x := 0; x < 7; x++ {
		if m[0][x] {
			fmt.Print("#")
		} else {
			fmt.Print(".")
		}
	}
	fmt.Println()
	// Output: #######
}
