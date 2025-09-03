package ui

import (
	"encoding/json"
	"image"
	"image/color"
	"os"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type OrnateBar struct {
	Badge    *ebiten.Image // may be nil when assets unavailable
	Frame    *ebiten.Image // may be nil when assets unavailable
	WellOfs  image.Point   // top-left of inner well (in source PNG pixels)
	WellSize image.Point   // size of inner well (in source PNG pixels)
	// optional meta
	Mode       string // "fit", "fitpad", or "well" (default "fitpad")
	PadX       int
	PadY       int
	BadgeScale float64 // multiplier when drawing ornate badge (default 1.0)
}

func LoadOrnateBar() *OrnateBar {
	// For mobile builds, assets are not available via file system
	// Return nil to use fallback UI
	frame := (*ebiten.Image)(nil)
	badge := (*ebiten.Image)(nil)
	ob := &OrnateBar{
		Badge:      badge,
		Frame:      frame,
		WellOfs:    image.Pt(130, 30),
		WellSize:   image.Pt(350, 44),
		Mode:       "fitpad",
		PadX:       2,
		PadY:       2,
		BadgeScale: 1.0,
	}
	// Optional meta to calibrate
	type meta struct {
		Mode       string  `json:"mode"`
		PadX       int     `json:"padX"`
		PadY       int     `json:"padY"`
		WellOfs    [2]int  `json:"wellOfs"`
		WellSize   [2]int  `json:"wellSize"`
		BadgeScale float64 `json:"badgeScale"`
	}
	var m meta
	if b, err := os.ReadFile("assets/ui/health_bar.meta.json"); err == nil {
		if json.Unmarshal(b, &m) == nil {
			if m.Mode != "" {
				ob.Mode = m.Mode
			}
			if m.PadX != 0 || m.PadY != 0 {
				ob.PadX, ob.PadY = m.PadX, m.PadY
			}
			if m.WellOfs[0] != 0 || m.WellOfs[1] != 0 {
				ob.WellOfs = image.Pt(m.WellOfs[0], m.WellOfs[1])
			}
			if m.WellSize[0] != 0 || m.WellSize[1] != 0 {
				ob.WellSize = image.Pt(m.WellSize[0], m.WellSize[1])
			}
			if m.BadgeScale > 0 {
				ob.BadgeScale = m.BadgeScale
			}
		}
	}
	return ob
}

// DrawOverlay: if frame missing, draw a simple rounded rectangle fallback.
func (o *OrnateBar) DrawOverlay(dst *ebiten.Image, barRect image.Rectangle) {
	if o.Frame == nil {
		// simple fallback frame
		ebitenutil.DrawRect(dst, float64(barRect.Min.X-2), float64(barRect.Min.Y-2), float64(barRect.Dx()+4), float64(barRect.Dy()+4), color.NRGBA{50, 50, 70, 255})
		ebitenutil.DrawRect(dst, float64(barRect.Min.X), float64(barRect.Min.Y), float64(barRect.Dx()), float64(barRect.Dy()), color.NRGBA{30, 34, 48, 255})
		return
	}
	sx := float64(barRect.Dx()) / float64(o.WellSize.X)
	sy := float64(barRect.Dy()) / float64(o.WellSize.Y)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(sx, sy)
	op.GeoM.Translate(
		float64(barRect.Min.X)-float64(o.WellOfs.X)*sx,
		float64(barRect.Min.Y)-float64(o.WellOfs.Y)*sy,
	)
	dst.DrawImage(o.Frame, op)
}

// DrawOverlayFit scales the whole frame image to exactly cover barRect.
// Use this when the frame is already a tight border image around the bar.
func (o *OrnateBar) DrawOverlayFit(dst *ebiten.Image, barRect image.Rectangle) {
	if o.Frame == nil {
		// fallback simple border
		ebitenutil.DrawRect(dst, float64(barRect.Min.X-1), float64(barRect.Min.Y-1), float64(barRect.Dx()+2), 1, color.NRGBA{0, 0, 0, 160})
		ebitenutil.DrawRect(dst, float64(barRect.Min.X-1), float64(barRect.Max.Y), float64(barRect.Dx()+2), 1, color.NRGBA{0, 0, 0, 160})
		ebitenutil.DrawRect(dst, float64(barRect.Min.X-1), float64(barRect.Min.Y), 1, float64(barRect.Dy()), color.NRGBA{0, 0, 0, 160})
		ebitenutil.DrawRect(dst, float64(barRect.Max.X), float64(barRect.Min.Y), 1, float64(barRect.Dy()), color.NRGBA{0, 0, 0, 160})
		return
	}
	fb := o.Frame.Bounds()
	if fb.Dx() == 0 || fb.Dy() == 0 {
		return
	}
	sx := float64(barRect.Dx()) / float64(fb.Dx())
	sy := float64(barRect.Dy()) / float64(fb.Dy())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(sx, sy)
	op.GeoM.Translate(float64(barRect.Min.X), float64(barRect.Min.Y))
	dst.DrawImage(o.Frame, op)
}

// DrawOverlayFitPad scales the frame to a rectangle expanded by (padX,padY) pixels
// on all sides. Useful to make the border read a bit thicker than the bar.
func (o *OrnateBar) DrawOverlayFitPad(dst *ebiten.Image, barRect image.Rectangle, padX, padY int) {
	if padX != 0 || padY != 0 {
		barRect = image.Rect(barRect.Min.X-padX, barRect.Min.Y-padY, barRect.Max.X+padX, barRect.Max.Y+padY)
	}
	o.DrawOverlayFit(dst, barRect)
}

// DrawForRect chooses the best overlay draw mode based on meta
func (o *OrnateBar) DrawForRect(dst *ebiten.Image, barRect image.Rectangle) {
	switch o.Mode {
	case "well":
		o.DrawOverlay(dst, barRect)
	case "fit":
		o.DrawOverlayFit(dst, barRect)
	default: // fitpad
		o.DrawOverlayFitPad(dst, barRect, o.PadX, o.PadY)
	}
}

// DrawBadgeLeft: draw the badge if available; otherwise skip.
func (o *OrnateBar) DrawBadgeLeft(dst *ebiten.Image, barRect image.Rectangle, gap int) {
	if o.Badge == nil {
		return
	}
	bb := o.Badge.Bounds()
	scale := float64(barRect.Dy()) / float64(bb.Dy())
	x := float64(barRect.Min.X) - float64(bb.Dx())*scale - float64(gap)
	y := float64(barRect.Min.Y) + (float64(barRect.Dy())-float64(bb.Dy())*scale)/2
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(x, y)
	dst.DrawImage(o.Badge, op)
}
