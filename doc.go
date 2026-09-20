// Package qrkit generates QR codes (ISO/IEC 18004 Model 2) as images or
// SVG, with rich styling, using only the Go standard library.
//
// # Quick start
//
//	qr, err := qrkit.New("https://q.mohamedation.com")
//	if err != nil {
//		log.Fatal(err)
//	}
//	if err := qr.Save("code.png"); err != nil { // or "code.svg"
//		log.Fatal(err)
//	}
//
// # Styling
//
// Everything is configured with functional options passed to [New]:
//
//	qr, err := qrkit.New("https://q.mohamedation.com",
//		qrkit.WithSize(600),
//		qrkit.WithForeground(color.NRGBA{0x1b, 0x2a, 0x49, 0xff}),
//		qrkit.WithTransparentBackground(),
//		qrkit.WithModuleShape(qrkit.ShapeRounded),
//		qrkit.WithFinderStyle(qrkit.FinderStyle{Shape: qrkit.FinderRounded}),
//		qrkit.WithLogo(logoImage, qrkit.LogoClip(qrkit.LogoCircle)),
//	)
//
// Module shapes, finder ("eye") shapes and colours, foreground and
// background colours (including transparency), module gaps, quiet zone,
// output size, mask, version range and error-correction level can all be
// chosen independently.
//
// # Logos
//
// [WithLogo] places an image in the centre. The modules beneath it are
// removed and the code relies on error correction to stay readable, so the
// library defaults to the highest recovery level, verifies for every
// error-correction block that the logo stays within a conservative budget,
// and automatically moves to a larger symbol version when that is needed.
// If a logo cannot be placed safely, [New] returns an error wrapping
// [ErrLogoTooLarge]. Always test the final code with real scanners.
//
// # Output
//
// A [QRCode] can be rendered as an [image.Image] ([QRCode.Image]), PNG
// ([QRCode.PNG], [QRCode.WritePNG]), SVG ([QRCode.SVG], [QRCode.WriteSVG]),
// written to a file ([QRCode.Save]) or inspected as a boolean matrix
// ([QRCode.Matrix]) so that you can render it any way you like.
//
// # Concurrency
//
// A [QRCode] is immutable; all its methods are safe for concurrent use.
//
// # Limitations
//
// Supported: versions 1-40, all four error-correction levels, numeric,
// alphanumeric and byte modes (chosen automatically, one mode per code).
// Not supported: Kanji mode, ECI headers, structured append, FNC1, Micro
// QR and rMQR. Text is encoded as UTF-8 bytes, which virtually all modern
// scanners handle.
package qrkit
