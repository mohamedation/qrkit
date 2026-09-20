// Command qrkit generates QR codes from the command line.
//
//	qrkit -o code.png "https://q.mohamedation.com"
//	echo -n "some text" | qrkit -o code.svg -shape rounded -fg "#1b2a49" -
//	qrkit -terminal "hello"
//
// Run "qrkit -h" for all options.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/mohamedation/qrkit"
)

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout io.Writer) error {
	fs := flag.NewFlagSet("qrkit", flag.ContinueOnError)
	var (
		out      = fs.String("o", "", "output file (.png or .svg); omit with -terminal")
		size     = fs.Int("size", 512, "image size in pixels")
		level    = fs.String("level", "", "error correction: L, M, Q or H (default M, or H with a logo)")
		fg       = fs.String("fg", "#000000", "foreground colour (#RGB, #RRGGBB, #RRGGBBAA)")
		bg       = fs.String("bg", "#ffffff", "background colour, or \"transparent\"")
		shape    = fs.String("shape", "square", "module shape: square, rounded, circle, diamond, vbars, hbars")
		finder   = fs.String("finder", "square", "finder shape: square, rounded, circle, modules")
		finderC  = fs.String("finder-color", "", "finder colour (default: foreground)")
		scale    = fs.Float64("scale", 1, "module scale in (0,1]; <1 leaves gaps")
		quiet    = fs.Int("quiet", 4, "quiet zone in modules")
		logoPath = fs.String("logo", "", "path to a PNG/JPEG/GIF logo to place in the centre")
		logoSize = fs.Float64("logo-scale", 0.2, "logo size as a fraction of the code width")
		logoClip = fs.String("logo-shape", "square", "logo clip: square, rounded, circle")
		logoPlt  = fs.String("logo-plate", "", "colour of a plate behind the logo")
		terminal = fs.Bool("terminal", false, "print the code to the terminal using ANSI colours")
	)
	fs.SetOutput(os.Stderr)
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: qrkit [options] <text | ->   (use - to read stdin)")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("expected exactly one content argument")
	}
	content := fs.Arg(0)
	if content == "-" {
		b, err := io.ReadAll(stdin)
		if err != nil {
			return err
		}
		content = strings.TrimRight(string(b), "\r\n")
	}
	if *out == "" && !*terminal {
		return fmt.Errorf("specify an output file with -o or use -terminal")
	}

	opts := []qrkit.Option{qrkit.WithSize(*size), qrkit.WithModuleScale(*scale), qrkit.WithQuietZone(*quiet)}
	if *level != "" {
		l, ok := map[string]qrkit.RecoveryLevel{"L": qrkit.LevelLow, "M": qrkit.LevelMedium, "Q": qrkit.LevelQuartile, "H": qrkit.LevelHigh}[strings.ToUpper(*level)]
		if !ok {
			return fmt.Errorf("invalid -level %q", *level)
		}
		opts = append(opts, qrkit.WithRecoveryLevel(l))
	}
	fgc, err := parseColor(*fg)
	if err != nil {
		return fmt.Errorf("-fg: %w", err)
	}
	bgc, err := parseColor(*bg)
	if err != nil {
		return fmt.Errorf("-bg: %w", err)
	}
	opts = append(opts, qrkit.WithForeground(fgc), qrkit.WithBackground(bgc))

	shapes := map[string]qrkit.ModuleShape{"square": qrkit.ShapeSquare, "rounded": qrkit.ShapeRounded, "circle": qrkit.ShapeCircle,
		"diamond": qrkit.ShapeDiamond, "vbars": qrkit.ShapeVerticalBars, "hbars": qrkit.ShapeHorizontalBars}
	s, ok := shapes[strings.ToLower(*shape)]
	if !ok {
		return fmt.Errorf("invalid -shape %q", *shape)
	}
	opts = append(opts, qrkit.WithModuleShape(s))

	finders := map[string]qrkit.FinderShape{"square": qrkit.FinderSquare, "rounded": qrkit.FinderRounded, "circle": qrkit.FinderCircle, "modules": qrkit.FinderModules}
	f, ok := finders[strings.ToLower(*finder)]
	if !ok {
		return fmt.Errorf("invalid -finder %q", *finder)
	}
	fstyle := qrkit.FinderStyle{Shape: f}
	if *finderC != "" {
		if fstyle.OuterColor, err = parseColor(*finderC); err != nil {
			return fmt.Errorf("-finder-color: %w", err)
		}
	}
	opts = append(opts, qrkit.WithFinderStyle(fstyle))

	if *logoPath != "" {
		file, err := os.Open(*logoPath)
		if err != nil {
			return err
		}
		img, _, err := image.Decode(file)
		file.Close()
		if err != nil {
			return fmt.Errorf("decoding logo: %w", err)
		}
		clips := map[string]qrkit.LogoShape{"square": qrkit.LogoSquare, "rounded": qrkit.LogoRounded, "circle": qrkit.LogoCircle}
		c, ok := clips[strings.ToLower(*logoClip)]
		if !ok {
			return fmt.Errorf("invalid -logo-shape %q", *logoClip)
		}
		lo := []qrkit.LogoOption{qrkit.LogoScale(*logoSize), qrkit.LogoClip(c)}
		if *logoPlt != "" {
			pc, err := parseColor(*logoPlt)
			if err != nil {
				return fmt.Errorf("-logo-plate: %w", err)
			}
			lo = append(lo, qrkit.LogoPlate(pc))
		}
		opts = append(opts, qrkit.WithLogo(img, lo...))
	}

	qr, err := qrkit.New(content, opts...)
	if err != nil {
		return err
	}
	if *terminal {
		printTerminal(stdout, qr, *quiet)
	}
	if *out != "" {
		return qr.Save(*out)
	}
	return nil
}

// printTerminal draws the symbol with explicit black/white ANSI backgrounds
// so that it scans regardless of the terminal theme.
func printTerminal(w io.Writer, qr *qrkit.QRCode, quiet int) {
	n := qr.Size()
	for y := -quiet; y < n+quiet; y++ {
		var sb strings.Builder
		for x := -quiet; x < n+quiet; x++ {
			if qr.IsDark(x, y) {
				sb.WriteString("\x1b[40m  \x1b[0m")
			} else {
				sb.WriteString("\x1b[47m  \x1b[0m")
			}
		}
		fmt.Fprintln(w, sb.String())
	}
}

func parseColor(s string) (color.Color, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "transparent", "none":
		return color.NRGBA{}, nil
	case "black":
		return color.NRGBA{0, 0, 0, 255}, nil
	case "white":
		return color.NRGBA{255, 255, 255, 255}, nil
	}
	h := strings.TrimPrefix(s, "#")
	if len(h) == 3 {
		h = string([]byte{h[0], h[0], h[1], h[1], h[2], h[2]})
	}
	if len(h) != 6 && len(h) != 8 {
		return nil, fmt.Errorf("invalid colour %q (want #RGB, #RRGGBB or #RRGGBBAA)", s)
	}
	v, err := strconv.ParseUint(h, 16, 32)
	if err != nil {
		return nil, fmt.Errorf("invalid colour %q", s)
	}
	if len(h) == 6 {
		return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 255}, nil
	}
	return color.NRGBA{uint8(v >> 24), uint8(v >> 16), uint8(v >> 8), uint8(v)}, nil
}
