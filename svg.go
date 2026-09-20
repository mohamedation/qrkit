package qrkit

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image/color"
	"image/png"
	"io"
	"strconv"
)

// SVG returns the code as a standalone SVG document. SVG output is
// resolution independent; a logo is embedded as a PNG data URI.
func (q *QRCode) SVG() (string, error) {
	var buf bytes.Buffer
	if err := q.WriteSVG(&buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// WriteSVG writes the code as an SVG document to w.
func (q *QRCode) WriteSVG(w io.Writer) error {
	_, _, px := q.pixelLayout()
	total := q.totalModules()
	quiet := float64(q.cfg.quiet)

	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 %d %d" width="%d" height="%d" role="img" aria-label="QR code">`+"\n",
		total, total, px, px)
	if bg := q.cfg.bg; bg.A > 0 {
		fmt.Fprintf(&b, `<rect width="%d" height="%d"%s/>`+"\n", total, total, fillAttrs(bg))
	}

	// One path per colour: adjacent modules are unioned by the renderer,
	// which avoids hairline seams between squares.
	type group struct {
		col color.NRGBA
		d   []byte
	}
	var groups []*group
	for _, p := range q.prims() {
		var g *group
		for _, c := range groups {
			if c.col == p.fill {
				g = c
				break
			}
		}
		if g == nil {
			g = &group{col: p.fill}
			groups = append(groups, g)
		}
		g.d = appendPath(g.d, p, quiet)
	}
	for _, g := range groups {
		fmt.Fprintf(&b, `<path%s fill-rule="evenodd" d="%s"/>`+"\n", fillAttrs(g.col), g.d)
	}

	if z := q.zone; z != nil {
		if z.cfg.plate != nil {
			d := appendPath(nil, z.plateShape(*z.cfg.plate), quiet)
			fmt.Fprintf(&b, `<path%s d="%s"/>`+"\n", fillAttrs(*z.cfg.plate), d)
		}
		// Embed at the logo's native resolution (at most 1024px on the long
		// side), with the clip shape already applied.
		bounds := z.cfg.img.Bounds()
		long := minInt(1024, maxInt(bounds.Dx(), bounds.Dy()))
		pw, ph := long, long
		if z.lw >= z.lh {
			ph = maxInt(1, int(float64(pw)*z.lh/z.lw))
		} else {
			pw = maxInt(1, int(float64(ph)*z.lw/z.lh))
		}
		var lb bytes.Buffer
		if err := png.Encode(&lb, z.image(pw, ph)); err != nil {
			return err
		}
		c := float64(q.size) / 2
		fmt.Fprintf(&b, `<image x="%s" y="%s" width="%s" height="%s" preserveAspectRatio="none" xlink:href="data:image/png;base64,%s"/>`+"\n",
			num(quiet+c-z.lw/2), num(quiet+c-z.lh/2), num(z.lw), num(z.lh),
			base64.StdEncoding.EncodeToString(lb.Bytes()))
	}
	b.WriteString("</svg>\n")
	_, err := w.Write(b.Bytes())
	return err
}

func fillAttrs(c color.NRGBA) string {
	s := fmt.Sprintf(` fill="#%02x%02x%02x"`, c.R, c.G, c.B)
	if c.A < 255 {
		s += ` fill-opacity="` + strconv.FormatFloat(float64(c.A)/255, 'f', 3, 64) + `"`
	}
	return s
}

func num(v float64) string { return string(appendNum(nil, v)) }

func appendNum(b []byte, v float64) []byte {
	start := len(b)
	b = strconv.AppendFloat(b, v, 'f', 3, 64)
	end := len(b)
	for end > start && b[end-1] == '0' {
		end--
	}
	if end > start && b[end-1] == '.' {
		end--
	}
	b = b[:end]
	if string(b[start:]) == "-0" {
		b = append(b[:start], '0')
	}
	return b
}

func appendPath(d []byte, p prim, off float64) []byte {
	d = appendSubpath(d, p, off)
	if p.hole != nil {
		d = appendSubpath(d, *p.hole, off)
	}
	return d
}

func appendSubpath(d []byte, p prim, off float64) []byte {
	x0, x1 := off+p.cx-p.hx, off+p.cx+p.hx
	y0, y1 := off+p.cy-p.hy, off+p.cy+p.hy
	pt := func(cmd byte, x, y float64) {
		d = append(d, cmd)
		d = appendNum(d, x)
		d = append(d, ',')
		d = appendNum(d, y)
	}
	if p.diamond {
		cx, cy := off+p.cx, off+p.cy
		pt('M', cx, y0)
		pt('L', x1, cy)
		pt('L', cx, y1)
		pt('L', x0, cy)
		return append(d, 'Z')
	}
	if p.r == [4]float64{} {
		pt('M', x0, y0)
		d = append(d, 'H')
		d = appendNum(d, x1)
		d = append(d, 'V')
		d = appendNum(d, y1)
		d = append(d, 'H')
		d = appendNum(d, x0)
		return append(d, 'Z')
	}
	arc := func(r, x, y float64) {
		if r <= 0 {
			return
		}
		d = append(d, 'A')
		d = appendNum(d, r)
		d = append(d, ',')
		d = appendNum(d, r)
		d = append(d, " 0 0 1 "...)
		d = appendNum(d, x)
		d = append(d, ',')
		d = appendNum(d, y)
	}
	tl, tr, br, bl := p.r[0], p.r[1], p.r[2], p.r[3]
	pt('M', x0+tl, y0)
	d = append(d, 'H')
	d = appendNum(d, x1-tr)
	arc(tr, x1, y0+tr)
	d = append(d, 'V')
	d = appendNum(d, y1-br)
	arc(br, x1-br, y1)
	d = append(d, 'H')
	d = appendNum(d, x0+bl)
	arc(bl, x0, y1-bl)
	d = append(d, 'V')
	d = appendNum(d, y0+tl)
	arc(tl, x0+tl, y0)
	return append(d, 'Z')
}
