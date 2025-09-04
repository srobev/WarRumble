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
		// Battle-specific positioning: center in available area between UI elements
		const topUIHeight = 50 // Approximate height of top UI (HP bars, timer)
		bottomUIHeight := battleHUDH
		availableHeight := protocol.ScreenH - topUIHeight - bottomUIHeight
		availableWidth := protocol.ScreenW

		iw, ih := bg.Bounds().Dx(), bg.Bounds().Dy()
		if iw == 0 || ih == 0 {
			offX, offY, dispW, dispH, s = 0, topUIHeight, availableWidth, availableHeight, 1
		} else {
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
			offY = topUIHeight + (availableHeight-dispH)/2
		}
	} else {
		// Normal positioning for non-battle screens
		offX, offY, dispW, dispH, s = g.mapRenderRect(bg)
	}

	col := g.mapEdgeColor(bgName, bg)

	// Draw letterbox bars
	if offY > 0 {
		ebitenutil.DrawRect(screen, 0, 0, float64(protocol.ScreenW), float64(offY), col)
		ebitenutil.DrawRect(screen, 0, float64(offY+dispH),
			float64(protocol.ScreenW), float64(protocol.ScreenH-(offY+dispH)), col)
	}

	if offX > 0 {
		ebitenutil.DrawRect(screen, 0, float64(offY), float64(offX), float64(dispH), col)
		ebitenutil.DrawRect(screen, float64(offX+dispW), float64(offY),
			float64(protocol.ScreenW-(offX+dispW)), float64(dispH), col)
	}

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(float64(offX), float64(offY))
	screen.DrawImage(bg, op)
}
