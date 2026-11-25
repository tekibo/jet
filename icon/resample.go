package icon

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// Resize performs high-quality bilinear interpolation resampling on an image.
func Resize(src image.Image, targetWidth, targetHeight int) *image.RGBA {
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	if srcW == 0 || srcH == 0 || targetWidth == 0 || targetHeight == 0 {
		return dst
	}

	xRatio := float64(srcW) / float64(targetWidth)
	yRatio := float64(srcH) / float64(targetHeight)

	for y := 0; y < targetHeight; y++ {
		srcY := float64(y)*yRatio + float64(srcBounds.Min.Y)
		yFloor := int(math.Floor(srcY))
		yCeil := yFloor + 1
		if yCeil >= srcBounds.Max.Y {
			yCeil = yFloor
		}
		yWeight := srcY - float64(yFloor)

		for x := 0; x < targetWidth; x++ {
			srcX := float64(x)*xRatio + float64(srcBounds.Min.X)
			xFloor := int(math.Floor(srcX))
			xCeil := xFloor + 1
			if xCeil >= srcBounds.Max.X {
				xCeil = xFloor
			}
			xWeight := srcX - float64(xFloor)

			// Sample 4 neighboring pixels
			c00 := toRGBAFloat(src.At(xFloor, yFloor))
			c10 := toRGBAFloat(src.At(xCeil, yFloor))
			c01 := toRGBAFloat(src.At(xFloor, yCeil))
			c11 := toRGBAFloat(src.At(xCeil, yCeil))

			// Bilinear interpolation
			topR := c00.r*(1-xWeight) + c10.r*xWeight
			topG := c00.g*(1-xWeight) + c10.g*xWeight
			topB := c00.b*(1-xWeight) + c10.b*xWeight
			topA := c00.a*(1-xWeight) + c10.a*xWeight

			botR := c01.r*(1-xWeight) + c11.r*xWeight
			botG := c01.g*(1-xWeight) + c11.g*xWeight
			botB := c01.b*(1-xWeight) + c11.b*xWeight
			botA := c01.a*(1-xWeight) + c11.a*xWeight

			finalR := topR*(1-yWeight) + botR*yWeight
			finalG := topG*(1-yWeight) + botG*yWeight
			finalB := topB*(1-yWeight) + botB*yWeight
			finalA := topA*(1-yWeight) + botA*yWeight

			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(math.Round(clamp(finalR, 0, 255))),
				G: uint8(math.Round(clamp(finalG, 0, 255))),
				B: uint8(math.Round(clamp(finalB, 0, 255))),
				A: uint8(math.Round(clamp(finalA, 0, 255))),
			})
		}
	}

	return dst
}

// FitSafeZone centers an image within a total canvas dimension, fitting it inside the safe zone (e.g., 66% for adaptive icon).
func FitSafeZone(src image.Image, canvasW, canvasH int, safeScale float64) *image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, canvasW, canvasH))

	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()
	if srcW == 0 || srcH == 0 {
		return canvas
	}

	maxInnerW := float64(canvasW) * safeScale
	maxInnerH := float64(canvasH) * safeScale

	// Calculate aspect ratio preserving scale
	scaleX := maxInnerW / float64(srcW)
	scaleY := maxInnerH / float64(srcH)
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	targetW := int(math.Round(float64(srcW) * scale))
	targetH := int(math.Round(float64(srcH) * scale))
	if targetW < 1 {
		targetW = 1
	}
	if targetH < 1 {
		targetH = 1
	}

	resized := Resize(src, targetW, targetH)

	// Center in canvas
	offsetX := (canvasW - targetW) / 2
	offsetY := (canvasH - targetH) / 2

	draw.Draw(canvas, image.Rect(offsetX, offsetY, offsetX+targetW, offsetY+targetH), resized, image.Point{}, draw.Over)
	return canvas
}

// HasOpaqueCorners reports whether all four corners of src are nearly opaque,
// indicating a full-bleed square icon rather than a transparent cutout.
func HasOpaqueCorners(src image.Image) bool {
	b := src.Bounds()
	if b.Dx() < 1 || b.Dy() < 1 {
		return false
	}
	pts := [4]image.Point{
		{b.Min.X, b.Min.Y},
		{b.Max.X - 1, b.Min.Y},
		{b.Min.X, b.Max.Y - 1},
		{b.Max.X - 1, b.Max.Y - 1},
	}
	for _, p := range pts {
		_, _, _, a := src.At(p.X, p.Y).RGBA()
		if (a >> 8) < 245 {
			return false
		}
	}
	return true
}

// CreateSplashIcon builds a splash drawable bitmap centered inside Android 12–16's
// circular safe zone, rounding or circularly masking opaque square inputs so corners
// are never roughly clipped by the system splash mask.
func CreateSplashIcon(src image.Image, canvasSize int, usePlate bool) *image.RGBA {
	opaqueCorners := HasOpaqueCorners(src)
	safeScale := 0.50
	if usePlate {
		safeScale = 0.54
	} else if opaqueCorners {
		// 0.52 rounded square (0.22 corner radius) has diagonal ~0.64 < 0.667 (192/288)
		safeScale = 0.52
	}

	if !opaqueCorners {
		return FitSafeZone(src, canvasSize, canvasSize, safeScale)
	}

	innerSize := int(math.Round(float64(canvasSize) * safeScale))
	if innerSize < 16 {
		innerSize = 16
	}
	resized := Resize(src, innerSize, innerSize)
	var masked *image.RGBA
	if usePlate {
		masked = applyCircularMask(resized)
	} else {
		masked = applyRoundedRectMask(resized, 0.22)
	}

	canvas := image.NewRGBA(image.Rect(0, 0, canvasSize, canvasSize))
	offset := (canvasSize - innerSize) / 2
	draw.Draw(canvas, image.Rect(offset, offset, offset+innerSize, offset+innerSize), masked, image.Point{}, draw.Over)
	return canvas
}

func applyRoundedRectMask(src image.Image, radiusFraction float64) *image.RGBA {
	bounds := src.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	if w <= 0 || h <= 0 {
		return dst
	}

	halfW := float64(w) / 2.0
	halfH := float64(h) / 2.0
	r := math.Min(float64(w), float64(h)) * radiusFraction
	bx := halfW - r
	by := halfH - r

	for y := 0; y < h; y++ {
		py := math.Abs(float64(y)+0.5-halfH) - by
		for x := 0; x < w; x++ {
			px := math.Abs(float64(x)+0.5-halfW) - bx
			ox := math.Max(px, 0)
			oy := math.Max(py, 0)
			dist := math.Sqrt(ox*ox+oy*oy) + math.Min(math.Max(px, py), 0) - r

			var cov float64
			switch {
			case dist <= -0.75:
				cov = 1.0
			case dist >= 0.75:
				cov = 0.0
			default:
				cov = (0.75 - dist) / 1.5
			}

			if cov <= 0 {
				dst.SetRGBA(x, y, color.RGBA{0, 0, 0, 0})
				continue
			}

			c := toRGBAFloat(src.At(bounds.Min.X+x, bounds.Min.Y+y))
			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(math.Round(clamp(c.r, 0, 255))),
				G: uint8(math.Round(clamp(c.g, 0, 255))),
				B: uint8(math.Round(clamp(c.b, 0, 255))),
				A: uint8(math.Round(clamp(c.a*cov, 0, 255))),
			})
		}
	}
	return dst
}

// CreateSolidCanvas creates an RGBA image filled with a given color.
func CreateSolidCanvas(width, height int, col color.Color) *image.RGBA {
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: col}, image.Point{}, draw.Src)
	return canvas
}

// CompositeOver composites foreground image centered over a background image of matching dimensions.
func CompositeOver(bg, fg image.Image) *image.RGBA {
	bounds := bg.Bounds()
	result := image.NewRGBA(bounds)
	draw.Draw(result, bounds, bg, bounds.Min, draw.Src)
	draw.Draw(result, bounds, fg, bounds.Min, draw.Over)
	return result
}

type rgbaFloat struct {
	r, g, b, a float64
}

func toRGBAFloat(c color.Color) rgbaFloat {
	r, g, b, a := c.RGBA()
	// c.RGBA() returns values in [0, 0xFFFF] premultiplied by alpha
	if a == 0 {
		return rgbaFloat{0, 0, 0, 0}
	}
	// Un-premultiply
	rf := (float64(r) / float64(a)) * 255.0
	gf := (float64(g) / float64(a)) * 255.0
	bf := (float64(b) / float64(a)) * 255.0
	af := float64(a) / 257.0
	return rgbaFloat{rf, gf, bf, af}
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
