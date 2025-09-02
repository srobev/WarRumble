package game

import (
	"image"
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
)

type NineSlice struct{ Left, Right, Top, Bottom int }

// draw a 9-slice image into (x,y,w,h). Falls back to uniform scale if too small.
func drawNineSlice(dst *ebiten.Image, src *ebiten.Image, x, y, w, h int, cap NineSlice) {
	if src == nil || w <= 0 || h <= 0 {
		return
	}
	b := src.Bounds()
	iw, ih := b.Dx(), b.Dy()
	l, r, t, btm := cap.Left, cap.Right, cap.Top, cap.Bottom

	if w < l+r || h < t+btm {
		sx := float64(w) / float64(iw)
		sy := float64(h) / float64(ih)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(sx, sy)
		op.GeoM.Translate(float64(x), float64(y))
		dst.DrawImage(src, op)
		return
	}

	sx := func(x0, y0, x1, y1 int) *ebiten.Image {
		return src.SubImage(image.Rect(b.Min.X+x0, b.Min.Y+y0, b.Min.X+x1, b.Min.Y+y1)).(*ebiten.Image)
	}

	cw := w - l - r
	ch := h - t - btm

	// 4 corners (no scale)
	type piece struct {
		s      *ebiten.Image
		dx, dy int
		sw, sh int
	}
	parts := []piece{
		{sx(0, 0, l, t), x, y, l, t},
		{sx(iw-r, 0, iw, t), x + w - r, y, r, t},
		{sx(0, ih-btm, l, ih), x, y + h - btm, l, btm},
		{sx(iw-r, ih-btm, iw, ih), x + w - r, y + h - btm, r, btm},
	}
	for _, p := range parts {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(p.dx), float64(p.dy))
		dst.DrawImage(p.s, op)
	}

	if t > 0 && cw > 0 {
		sTop := sx(l, 0, iw-r, t)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(cw)/float64(iw-l-r), 1)
		op.GeoM.Translate(float64(x+l), float64(y))
		dst.DrawImage(sTop, op)
	}

	if btm > 0 && cw > 0 {
		sBot := sx(l, ih-btm, iw-r, ih)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(cw)/float64(iw-l-r), 1)
		op.GeoM.Translate(float64(x+l), float64(y+h-btm))
		dst.DrawImage(sBot, op)
	}

	if l > 0 && ch > 0 {
		sLeft := sx(0, t, l, ih-btm)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(1, float64(ch)/float64(ih-t-btm))
		op.GeoM.Translate(float64(x), float64(y+t))
		dst.DrawImage(sLeft, op)
	}

	if r > 0 && ch > 0 {
		sRight := sx(iw-r, t, iw, ih-btm)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(1, float64(ch)/float64(ih-t-btm))
		op.GeoM.Translate(float64(x+w-r), float64(y+t))
		dst.DrawImage(sRight, op)
	}

	if cw > 0 && ch > 0 {
		sMid := sx(l, t, iw-r, ih-btm)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(cw)/float64(iw-l-r), float64(ch)/float64(ih-t-btm))
		op.GeoM.Translate(float64(x+l), float64(y+t))
		dst.DrawImage(sMid, op)
	}
}

func (g *Game) drawNineBtn(screen *ebiten.Image, r rect, hovered bool) {
	g.assets.ensureInit()
	img := g.assets.btn9Base
	if hovered && g.assets.btn9Hover != nil {
		img = g.assets.btn9Hover
	}
	if img != nil {
		drawNineSlice(screen, img, r.x, r.y, r.w, r.h, NineSlice{Left: 6, Right: 6, Top: 6, Bottom: 6})
		return
	}

	col := color.NRGBA{54, 63, 88, 255}
	if hovered {
		col = color.NRGBA{74, 86, 120, 255}
	}
	ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), col)
}
