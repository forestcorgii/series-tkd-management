package main

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

// Generates crisp, branded PNG icons for Series TKD PWA.
// Colors:
// Brand Crimson: #990303 (153, 3, 3)
// Bright Crimson: #DC2626 (220, 38, 38)
// Dark Crimson: #6B0101 (107, 1, 1)
// Gold: #FBBF24 (251, 191, 36)
// White: #FFFFFF (255, 255, 255)
// Belt: #111827 (17, 24, 39)

func main() {
	outDir := filepath.Join("web", "static", "icons")
	_ = os.MkdirAll(outDir, 0755)

	generateIcon(filepath.Join(outDir, "icon-512.png"), 512, false)
	generateIcon(filepath.Join(outDir, "icon-192.png"), 192, false)
	generateIcon(filepath.Join(outDir, "icon-maskable-512.png"), 512, true)
	generateIcon(filepath.Join(outDir, "icon-maskable-192.png"), 192, true)
	generateIcon(filepath.Join(outDir, "apple-touch-icon.png"), 180, false)
}

func generateIcon(filename string, size int, maskable bool) {
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Scale factor relative to 512 base size
	s := float64(size) / 512.0

	// Safe zone scale for maskable (central 80% to ensure no clipping in circle/squircle masks)
	contentScale := s
	offset := 0.0
	if maskable {
		contentScale = s * 0.78
		offset = (float64(size) - 512.0*contentScale) / 2.0
	}

	cornerRad := 108.0 * s
	if maskable {
		cornerRad = 0 // Full bleed background for maskable icons
	}

	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			fx := float64(x)
			fy := float64(y)

			// Rounded rectangle test for non-maskable icons
			if !maskable {
				if !insideRoundedRect(fx, fy, 16*s, 16*s, float64(size)-32*s, float64(size)-32*s, cornerRad) {
					img.Set(x, y, color.Transparent)
					continue
				}
			}

			// Background Gradient: #B91C1C at top-left to #550000 at bottom-right
			t := (fx + fy) / (2.0 * float64(size))
			if t < 0 {
				t = 0
			}
			if t > 1 {
				t = 1
			}

			r := uint8(185.0*(1.0-t) + 85.0*t)
			g := uint8(28.0*(1.0-t) + 0.0*t)
			b := uint8(28.0*(1.0-t) + 0.0*t)
			c := color.RGBA{R: r, G: g, B: b, A: 255}

			// Subtle border highlight on non-maskable
			if !maskable && isBorder(fx, fy, 16*s, 16*s, float64(size)-32*s, float64(size)-32*s, cornerRad, 3*s) {
				c = color.RGBA{R: 220, G: 38, B: 38, A: 255}
			}

			img.Set(x, y, c)
		}
	}

	// Helper to draw filled rectangles in scaled coordinates
	drawRect := func(rx, ry, rw, rh float64, col color.RGBA) {
		sx := rx*contentScale + offset
		sy := ry*contentScale + offset
		sw := rw * contentScale
		sh := rh * contentScale

		minX := int(math.Floor(sx))
		maxX := int(math.Ceil(sx + sw))
		minY := int(math.Floor(sy))
		maxY := int(math.Ceil(sy + sh))

		for py := minY; py <= maxY; py++ {
			if py < 0 || py >= size {
				continue
			}
			for px := minX; px <= maxX; px++ {
				if px < 0 || px >= size {
					continue
				}
				img.Set(px, py, col)
			}
		}
	}

	white := color.RGBA{R: 255, G: 253, B: 244, A: 255}
	gold := color.RGBA{R: 251, G: 191, B: 36, A: 255}
	beltBlack := color.RGBA{R: 17, G: 24, B: 39, A: 255}

	// Stylized "S" Glyphs:
	// Top bar: [150, 130] to [245, 172]
	drawRect(150, 130, 95, 42, white)
	// Top left stem: [150, 172] to [192, 224]
	drawRect(150, 172, 42, 52, white)
	// Middle bar: [150, 214] to [245, 256]
	drawRect(150, 214, 95, 42, white)
	// Bottom right stem: [203, 256] to [245, 308]
	drawRect(203, 256, 42, 52, white)
	// Bottom bar: [150, 298] to [245, 340]
	drawRect(150, 298, 95, 42, white)

	// Stylized "T" Glyphs:
	// Top horizontal bar: [265, 130] to [362, 172]
	drawRect(265, 130, 97, 42, white)
	// Center vertical stem: [292, 172] to [334, 340]
	drawRect(292, 172, 44, 168, white)

	// Gold dynamic martial arts accent slash (top-right of T)
	drawRect(346, 130, 16, 16, gold)

	// Base Taekwondo Belt bar
	drawRect(138, 362, 236, 26, beltBlack)
	// Dan rank gold stripes on the right tip of the belt
	drawRect(340, 362, 7, 26, gold)
	drawRect(353, 362, 7, 26, gold)

	f, err := os.Create(filename)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		panic(err)
	}
}

func insideRoundedRect(x, y, rx, ry, w, h, radius float64) bool {
	if x < rx || x > rx+w || y < ry || y > ry+h {
		return false
	}
	// Check corners
	cx, cy := 0.0, 0.0
	inCorner := false
	if x < rx+radius && y < ry+radius {
		cx, cy = rx+radius, ry+radius
		inCorner = true
	} else if x > rx+w-radius && y < ry+radius {
		cx, cy = rx+w-radius, ry+radius
		inCorner = true
	} else if x < rx+radius && y > ry+h-radius {
		cx, cy = rx+radius, ry+h-radius
		inCorner = true
	} else if x > rx+w-radius && y > ry+h-radius {
		cx, cy = rx+w-radius, ry+h-radius
		inCorner = true
	}

	if inCorner {
		dx := x - cx
		dy := y - cy
		return dx*dx+dy*dy <= radius*radius
	}
	return true
}

func isBorder(x, y, rx, ry, w, h, radius, stroke float64) bool {
	if !insideRoundedRect(x, y, rx, ry, w, h, radius) {
		return false
	}
	return !insideRoundedRect(x, y, rx+stroke, ry+stroke, w-2*stroke, h-2*stroke, radius-stroke)
}
