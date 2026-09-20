package qrkit

import (
	"bytes"
	"image"
	"image/png"
	"io"
	"math"
)

// Image renders the code as a raster image (*image.NRGBA). Edges are
// anti-aliased and the background is transparent if so configured.
func (q *QRCode) Image() image.Image { return q.render() }

// PNG returns the code encoded as a PNG file.
func (q *QRCode) PNG() ([]byte, error) {
	var buf bytes.Buffer
	if err := q.WritePNG(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WritePNG encodes the code as PNG to w.
func (q *QRCode) WritePNG(w io.Writer) error { return png.Encode(w, q.render()) }

func (q *QRCode) render() *image.NRGBA {
	ms, off, px := q.pixelLayout()
	canvas := image.NewRGBA(image.Rect(0, 0, px, px))
	if bg := q.cfg.bg; bg.A > 0 {
		full := prim{cx: float64(px) / 2, cy: float64(px) / 2, hx: float64(px) / 2, hy: float64(px) / 2, fill: bg}
		full.rasterize(canvas)
	}
	o := float64(off) + float64(q.cfg.quiet*ms)
	s := float64(ms)
	for _, p := range q.prims() {
		t := p.transform(o, o, s)
		t.rasterize(canvas)
	}
	if z := q.zone; z != nil {
		if z.cfg.plate != nil {
			t := z.plateShape(*z.cfg.plate).transform(o, o, s)
			t.rasterize(canvas)
		}
		c := float64(q.size) / 2
		x0 := int(math.Round(o + (c-z.lw/2)*s))
		x1 := int(math.Round(o + (c+z.lw/2)*s))
		y0 := int(math.Round(o + (c-z.lh/2)*s))
		y1 := int(math.Round(o + (c+z.lh/2)*s))
		logo := z.image(x1-x0, y1-y0)
		for y := 0; y < logo.Bounds().Dy(); y++ {
			for x := 0; x < logo.Bounds().Dx(); x++ {
				sp := logo.Pix[logo.PixOffset(x, y):]
				if sp[3] == 0 {
					continue
				}
				i := canvas.PixOffset(x0+x, y0+y)
				d := canvas.Pix[i : i+4 : i+4]
				inv := 1 - float64(sp[3])/255
				for k := 0; k < 4; k++ {
					d[k] = clampByte(float64(sp[k]) + float64(d[k])*inv)
				}
			}
		}
	}
	return unpremultiply(canvas)
}

func unpremultiply(src *image.RGBA) *image.NRGBA {
	dst := image.NewNRGBA(src.Bounds())
	for i := 0; i+3 < len(src.Pix); i += 4 {
		a := uint32(src.Pix[i+3])
		switch a {
		case 0:
		case 255:
			copy(dst.Pix[i:i+4], src.Pix[i:i+4])
		default:
			for k := 0; k < 3; k++ {
				v := (uint32(src.Pix[i+k])*255 + a/2) / a
				if v > 255 {
					v = 255
				}
				dst.Pix[i+k] = uint8(v)
			}
			dst.Pix[i+3] = uint8(a)
		}
	}
	return dst
}
