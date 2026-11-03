package icon

import (
	"image"
	"image/color"
	"math"
)

// ExtractMonochrome converts any source image (flat, multi-color, or opaque) into a clean,
// stencil-preserved monochrome silhouette (white #FFFFFF with alpha, on transparent #00000000).
// It prevents the "solid blob" problem by preserving internal contrast, lines, and cutouts.
func ExtractMonochrome(src image.Image) *image.RGBA {
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if w == 0 || h == 0 {
		return dst
	}

	// 1. Analyze transparency & sample corner colors
	var transparentPixelCount int
	var totalPixels = w * h
	var luminances = make([][]float64, h)
	var alphas = make([][]float64, h)

	var lumSum float64
	var lumCount float64

	for y := 0; y < h; y++ {
		luminances[y] = make([]float64, w)
		alphas[y] = make([]float64, w)
		for x := 0; x < w; x++ {
			c := src.At(bounds.Min.X+x, bounds.Min.Y+y)
			rf, gf, bf, af := toRGBAFloatComponents(c)
			alphas[y][x] = af
			lum := 0.299*rf + 0.587*gf + 0.114*bf
			luminances[y][x] = lum

			if af < 20 {
				transparentPixelCount++
			} else {
				lumSum += lum
				lumCount++
			}
		}
	}

	hasSignificantTransparency := float64(transparentPixelCount)/float64(totalPixels) > 0.08

	if hasSignificantTransparency {
		// Image already has a transparent canvas.
		// Check if there is internal contrast inside the visible shapes (e.g. dark text/mark inside light circle).
		var avgLum float64
		if lumCount > 0 {
			avgLum = lumSum / lumCount
		}

		var variance float64
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if alphas[y][x] >= 20 {
					diff := luminances[y][x] - avgLum
					variance += diff * diff
				}
			}
		}
		if lumCount > 0 {
			variance /= lumCount
		}
		stdDev := math.Sqrt(variance)

		hasInternalContrast := stdDev > 28.0

		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				a := alphas[y][x]
				if a < 20 {
					dst.SetRGBA(x, y, color.RGBA{0, 0, 0, 0})
					continue
				}

				if hasInternalContrast {
					// Adaptive stencil: carve out dark interior regions or emphasize edges
					lum := luminances[y][x]
					// If pixel is darker than the mean by more than 0.5*stdDev, treat as cutout
					if lum < (avgLum - 0.4*stdDev) {
						dst.SetRGBA(x, y, color.RGBA{0, 0, 0, 0})
					} else {
						// Anti-aliased white pixel scaled by alpha
						alphaFactor := a / 255.0
						dst.SetRGBA(x, y, color.RGBA{255, 255, 255, uint8(clamp(255*alphaFactor, 0, 255))})
					}
				} else {
					// Uniform shape: keep original alpha mask, tint to white
					dst.SetRGBA(x, y, color.RGBA{255, 255, 255, uint8(clamp(a, 0, 255))})
				}
			}
		}
	} else {
		// Fully opaque image (e.g., solid background with logo in center).
		// Detect background color by sampling the 4 corners.
		cTL := src.At(bounds.Min.X, bounds.Min.Y)
		cTR := src.At(bounds.Max.X-1, bounds.Min.Y)
		cBL := src.At(bounds.Min.X, bounds.Max.Y-1)
		cBR := src.At(bounds.Max.X-1, bounds.Max.Y-1)

		bgR, bgG, bgB, _ := averageColors(cTL, cTR, cBL, cBR)

		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				c := src.At(bounds.Min.X+x, bounds.Min.Y+y)
				rf, gf, bf, _ := toRGBAFloatComponents(c)

				// Calculate Euclidean color distance from background
				dDist := math.Sqrt((rf-bgR)*(rf-bgR) + (gf-bgG)*(gf-bgG) + (bf-bgB)*(bf-bgB))

				// If distance is small, it's the background -> transparent
				// If distance is large, it's foreground -> white
				if dDist < 30.0 {
					dst.SetRGBA(x, y, color.RGBA{0, 0, 0, 0})
				} else if dDist < 60.0 {
					// Smooth anti-aliased edge
					factor := (dDist - 30.0) / 30.0
					dst.SetRGBA(x, y, color.RGBA{255, 255, 255, uint8(math.Round(255 * factor))})
				} else {
					dst.SetRGBA(x, y, color.RGBA{255, 255, 255, 255})
				}
			}
		}
	}

	return dst
}

func toRGBAFloatComponents(c color.Color) (float64, float64, float64, float64) {
	r, g, b, a := c.RGBA()
	if a == 0 {
		return 0, 0, 0, 0
	}
	rf := (float64(r) / float64(a)) * 255.0
	gf := (float64(g) / float64(a)) * 255.0
	bf := (float64(b) / float64(a)) * 255.0
	af := float64(a) / 257.0
	return rf, gf, bf, af
}

func averageColors(colors ...color.Color) (float64, float64, float64, float64) {
	var rSum, gSum, bSum, aSum float64
	for _, c := range colors {
		r, g, b, a := toRGBAFloatComponents(c)
		rSum += r
		gSum += g
		bSum += b
		aSum += a
	}
	n := float64(len(colors))
	return rSum / n, gSum / n, bSum / n, aSum / n
}
