package qrkit

import (
	"image"
	"math"
)

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// resizeRGBA scales src to w x h using premultiplied colour: an exact area
// average when shrinking (avoiding aliasing) and bilinear when enlarging.
func resizeRGBA(src image.Image, w, h int) *image.RGBA {
	s := toRGBA(src)
	sw, sh := s.Bounds().Dx(), s.Bounds().Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if sw == w && sh == h {
		copy(dst.Pix, s.Pix)
		return dst
	}
	fx, fy := float64(sw)/float64(w), float64(sh)/float64(h)
	px := func(x, y int) []uint8 {
		if x < 0 {
			x = 0
		} else if x >= sw {
			x = sw - 1
		}
		if y < 0 {
			y = 0
		} else if y >= sh {
			y = sh - 1
		}
		i := s.PixOffset(x, y)
		return s.Pix[i : i+4]
	}
	for dy := 0; dy < h; dy++ {
		for dx := 0; dx < w; dx++ {
			var acc [4]float64
			if fx <= 1 && fy <= 1 { // bilinear
				sx, sy := (float64(dx)+0.5)*fx-0.5, (float64(dy)+0.5)*fy-0.5
				x0, y0 := int(math.Floor(sx)), int(math.Floor(sy))
				tx, ty := sx-float64(x0), sy-float64(y0)
				for j := 0; j < 2; j++ {
					for i := 0; i < 2; i++ {
						wt := (1 - tx + float64(i)*(2*tx-1)) * (1 - ty + float64(j)*(2*ty-1))
						p := px(x0+i, y0+j)
						for k := 0; k < 4; k++ {
							acc[k] += wt * float64(p[k])
						}
					}
				}
			} else { // area average
				x0, x1 := float64(dx)*fx, float64(dx+1)*fx
				y0, y1 := float64(dy)*fy, float64(dy+1)*fy
				var total float64
				for y := int(y0); y < sh && float64(y) < y1; y++ {
					wy := math.Min(float64(y+1), y1) - math.Max(float64(y), y0)
					for x := int(x0); x < sw && float64(x) < x1; x++ {
						wt := wy * (math.Min(float64(x+1), x1) - math.Max(float64(x), x0))
						p := px(x, y)
						for k := 0; k < 4; k++ {
							acc[k] += wt * float64(p[k])
						}
						total += wt
					}
				}
				if total > 0 {
					for k := range acc {
						acc[k] /= total
					}
				}
			}
			i := dst.PixOffset(dx, dy)
			for k := 0; k < 4; k++ {
				dst.Pix[i+k] = uint8(math.Min(255, acc[k]+0.5))
			}
		}
	}
	return dst
}
