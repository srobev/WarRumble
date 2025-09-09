package game

import (
	"image"
	"image/color"
	"rumble/shared/protocol"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

// Which map image to display right now.
func (g *Game) displayMapID() string {
	if g.currentMap != "" {
		return g.currentMap
	}
	if len(g.maps) > 0 {
		return g.maps[0].ID
	}
	return defaultMapID
}

// Compute a *letterboxed* draw rect that preserves aspect ratio (no stretch).
// The image is scaled with s = min(ScreenW/iw, ScreenH/ih).
// Returns the on-screen offset (offX,offY), displayed size (dispW,dispH),
// and the scale factor s.
func (g *Game) mapRenderRect(img *ebiten.Image) (offX, offY, dispW, dispH int, s float64) {
	if img == nil {
		return 0, 0, protocol.ScreenW, protocol.ScreenH, 1
	}
	iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
	if iw == 0 || ih == 0 {
		return 0, 0, protocol.ScreenW, protocol.ScreenH, 1
	}
	sw := float64(protocol.ScreenW) / float64(iw)
	sh := float64(protocol.ScreenH) / float64(ih)
	if sw < sh {
		s = sw
	} else {
		s = sh
	}
	dispW = int(float64(iw) * s)
	dispH = int(float64(ih) * s)
	offX = (protocol.ScreenW - dispW) / 2
	offY = (protocol.ScreenH - dispH) / 2
	return
}

// Letterboxed destination for a bg image (fit to width, keep aspect)
func (g *Game) mapDstRect(imgW, imgH int) rect {
	W, H := protocol.ScreenW, protocol.ScreenH
	scale := float64(W) / float64(imgW)
	dh := int(float64(imgH) * scale)
	y := (H - dh) / 2
	return rect{x: 0, y: y, w: W, h: dh}
}

// Average a few pixels across top & bottom rows to pick a bar color
func (g *Game) mapEdgeColor(mapID string, _ *ebiten.Image) color.NRGBA {
	if c, ok := g.assets.edgeCol[mapID]; ok {
		return c
	}
	if c, ok := g.computeEdgeColorFromFS(mapID); ok {
		g.assets.edgeCol[mapID] = c
		return c
	}
	c := color.NRGBA{20, 20, 24, 255}
	g.assets.edgeCol[mapID] = c
	return c
}

func (g *Game) computeEdgeColorFromFS(mapID string) (color.NRGBA, bool) {

	candidates := []string{
		"assets/maps/" + strings.ToLower(mapID) + ".png",
		"assets/maps/" + strings.ToLower(mapID) + ".jpg",
	}

	tryDecode := func(p string) (color.NRGBA, bool) {
		f, err := assetsFS.Open(p)
		if err != nil {
			return color.NRGBA{}, false
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			return color.NRGBA{}, false
		}

		b := img.Bounds()
		iw, ih := b.Dx(), b.Dy()
		if iw == 0 || ih == 0 {
			return color.NRGBA{}, false
		}

		samples := 16
		sumR, sumG, sumB := 0, 0, 0
		for i := 0; i < samples; i++ {
			x := b.Min.X + i*(iw-1)/maxInt(samples-1, 1)
			r1, g1, b1, _ := img.At(x, b.Min.Y).RGBA()
			r2, g2, b2, _ := img.At(x, b.Max.Y-1).RGBA()
			sumR += int(r1>>8) + int(r2>>8)
			sumG += int(g1>>8) + int(g2>>8)
			sumB += int(b1>>8) + int(b2>>8)
		}
		n := samples * 2
		c := color.NRGBA{
			uint8(sumR / n),
			uint8(sumG / n),
			uint8(sumB / n),
			255,
		}

		c.R = uint8(float64(c.R) * 0.9)
		c.G = uint8(float64(c.G) * 0.9)
		c.B = uint8(float64(c.B) * 0.9)
		return c, true
	}

	for _, p := range candidates {
		if c, ok := tryDecode(p); ok {
			return c, true
		}
	}

	if entries, err := assetsFS.ReadDir("assets/maps"); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := strings.ToLower(e.Name())
			if strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".jpg") {
				if c, ok := tryDecode("assets/maps/" + e.Name()); ok {
					return c, true
				}
			}
		}
	}
	return color.NRGBA{}, false
}

// Letterbox img inside a target rect (x0,y0,w,h)
func (g *Game) mapRenderRectInBounds(x0, y0, w, h int, img *ebiten.Image) (offX, offY, dispW, dispH int, s float64) {
	if img == nil || w <= 0 || h <= 0 {
		return x0, y0, w, h, 1
	}
	iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
	if iw == 0 || ih == 0 {
		return x0, y0, w, h, 1
	}
	sw := float64(w) / float64(iw)
	sh := float64(h) / float64(ih)
	if sw < sh {
		s = sw
	} else {
		s = sh
	}
	dispW = int(float64(iw) * s)
	dispH = int(float64(ih) * s)
	offX = x0 + (w-dispW)/2
	offY = y0 + (h-dispH)/2
	return
}

func (g *Game) drawArenaBG(screen *ebiten.Image) {
	if g.currentArena == "" {
		return
	}

	// Use bg from map definition if available, otherwise fall back to arena ID
	bgName := g.currentArena
	if g.currentMapDef != nil && g.currentMapDef.Bg != "" {
		// Remove .png extension if present
		if strings.HasSuffix(g.currentMapDef.Bg, ".png") {
			bgName = strings.TrimSuffix(g.currentMapDef.Bg, ".png")
		} else {
			bgName = g.currentMapDef.Bg
		}
	}

	bg := g.ensureBgForMap(bgName)
	if bg == nil {
		return
	}

	var offX, offY, dispW, dispH int
	var s float64

	if g.scr == screenBattle {
		// Battle-specific positioning: use full available area (HP bars overlay on map)
		bottomUIHeight := battleHUDH
		availableHeight := protocol.ScreenH - bottomUIHeight // Full height minus bottom UI only
		availableWidth := protocol.ScreenW

		iw, ih := bg.Bounds().Dx(), bg.Bounds().Dy()
		if iw == 0 || ih == 0 {
			offX, offY, dispW, dispH, s = 0, 0, availableWidth, availableHeight, 1
		} else {
			// Check if we should use natural map size (no scaling) for large maps
			useNaturalSize := iw > availableWidth || ih > availableHeight

			if useNaturalSize {
				// Use natural size for maps larger than viewport - enable free scrolling
				s = 1.0 // No scaling
				dispW = iw
				dispH = ih
				// Center the map initially, but allow full scrolling to edges
				offX = (availableWidth - dispW) / 2
				offY = (availableHeight - dispH) / 2
			} else if g.currentMapDef != nil && g.currentMapDef.BgScale > 0 {
				// Use saved background scaling for smaller maps
				s = g.currentMapDef.BgScale
				dispW = int(float64(iw) * s)
				dispH = int(float64(ih) * s)
				// Perfect centering: remove BgOffsetX/BgOffsetY to center all maps
				offX = (availableWidth - dispW) / 2  // Comment out: + int(g.currentMapDef.BgOffsetX)
				offY = (availableHeight - dispH) / 2 // Comment out: + int(g.currentMapDef.BgOffsetY)
			} else {
				// Fallback to fit-to-viewport scaling for small maps
				sw := float64(availableWidth) / float64(iw)
				sh := float64(availableHeight) / float64(ih)
				if sw < sh {
					s = sw
				} else {
					s = sh
				}
				dispW = int(float64(iw) * s)
				dispH = int(float64(ih) * s)
				offX = (availableWidth - dispW) / 2
				offY = (availableHeight - dispH) / 2
			}
		}
	} else {
		// Normal positioning for non-battle screens
		offX, offY, dispW, dispH, s = g.mapRenderRect(bg)
	}

	// Use black for areas outside the map
	col := color.NRGBA{0, 0, 0, 255}

	// Draw dark gray letterbox bars to cover entire screen outside map
	// These bars must account for camera position to prevent them from covering visible map content
	transformedImageTop := float64(offY) + g.cameraY
	transformedImageBottom := float64(offY+dispH) + g.cameraY
	transformedImageLeft := float64(offX) + g.cameraX
	transformedImageRight := float64(offX+dispW) + g.cameraX

	// Top bar - covers area above transformed image
	if transformedImageTop > 0 {
		ebitenutil.DrawRect(screen, 0, 0, float64(protocol.ScreenW), transformedImageTop, col)
	}

	// Bottom bar - covers area below transformed image
	if transformedImageBottom < float64(protocol.ScreenH) {
		barHeight := float64(protocol.ScreenH) - transformedImageBottom
		ebitenutil.DrawRect(screen, 0, transformedImageBottom,
			float64(protocol.ScreenW), barHeight, col)
	}

	// Left bar - covers area to the left of transformed image
	if transformedImageLeft > 0 {
		barWidth := transformedImageLeft
		barHeight := float64(dispH) * s * g.cameraZoom // Match scaled image height
		// Start Y should align with transformed image Y position
		startY := transformedImageTop
		if startY < 0 {
			barHeight += startY // Adjust height if image starts above screen
			startY = 0
		}
		if startY+barHeight > float64(protocol.ScreenH) {
			barHeight = float64(protocol.ScreenH) - startY // Cap at screen bottom
		}
		ebitenutil.DrawRect(screen, 0, startY, barWidth, barHeight, col)
	}

	// Right bar - covers area to the right of transformed image
	if transformedImageRight < float64(protocol.ScreenW) {
		barWidth := float64(protocol.ScreenW) - transformedImageRight
		barHeight := float64(dispH) * s * g.cameraZoom // Match scaled image height
		barX := transformedImageRight
		// Start Y should align with transformed image Y position
		startY := transformedImageTop
		if startY < 0 {
			barHeight += startY // Adjust height if image starts above screen
			startY = 0
		}
		if startY+barHeight > float64(protocol.ScreenH) {
			barHeight = float64(protocol.ScreenH) - startY // Cap at screen bottom
		}
		if barWidth > 0 && barHeight > 0 {
			ebitenutil.DrawRect(screen, barX, startY, barWidth, barHeight, col)
		}
	}

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s*g.cameraZoom, s*g.cameraZoom)
	op.GeoM.Translate(float64(offX)+g.cameraX, float64(offY)+g.cameraY)
	screen.DrawImage(bg, op)
}
