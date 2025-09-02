package game

import (
	"image/color"
	"rumble/shared/protocol"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

func (g *Game) drawBottomBar(screen *ebiten.Image) {

	if g.bottomBarBg == nil {
		img, _, err := ebitenutil.NewImageFromFile("internal/game/assets/ui/bottom_bar_bg.png")
		if err != nil {
			panic("failed to load bottom_bar_bg.png: " + err.Error())
		}
		g.bottomBarBg = img
	}

	sw, sh := screen.Size()
	iw := g.bottomBarBg.Bounds().Dx()
	ih := g.bottomBarBg.Bounds().Dy()

	scaleX := float64(sw) / float64(iw)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scaleX, 1)
	op.GeoM.Translate(0, float64(sh-ih))
	screen.DrawImage(g.bottomBarBg, op)

	g.computeBottomBarLayout()

	{
		btns := []*rect{&g.armyBtn, &g.mapBtn, &g.pvpBtn, &g.socialBtn, &g.settingsBtn}
		gap := 16

		totalW := 0
		for i, b := range btns {
			if b.w == 0 {
				b.w = 120
			}
			totalW += b.w
			if i < len(btns)-1 {
				totalW += gap
			}
		}

		startX := (sw - totalW) / 2
		x := startX
		for _, b := range btns {
			b.x = x
			x += b.w + gap
		}
	}

	drawBtn := func(r rect, label string, active, enabled bool) {
		col := color.NRGBA{60, 60, 80, 255}
		if active {
			col = color.NRGBA{80, 80, 110, 255}
		}
		if !enabled {
			col = color.NRGBA{50, 50, 50, 255}
		}
		ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), col)

		lb := text.BoundString(basicfont.Face7x13, label)
		tx := r.x + (r.w-lb.Dx())/2
		ty := r.y + (r.h+13)/2 - 2
		txt := color.Color(color.White)
		if !enabled {
			txt = color.NRGBA{220, 220, 220, 255}
		}
		text.Draw(screen, label, basicfont.Face7x13, tx, ty, txt)
	}

	drawBtn(g.armyBtn, "Army", g.activeTab == tabArmy, true)
	drawBtn(g.mapBtn, "Map", g.activeTab == tabMap, true)
	drawBtn(g.pvpBtn, "PvP", g.activeTab == tabPvp, true)
    drawBtn(g.socialBtn, "Social", g.activeTab == tabSocial, true)
	drawBtn(g.settingsBtn, "Settings", g.activeTab == tabSettings, true)
}

func (g *Game) computeBottomBarLayout() {
	type item struct {
		label string
		out   *rect
	}
	items := []item{
		{"Army", &g.armyBtn},
		{"Map", &g.mapBtn},
		{"PvP", &g.pvpBtn},
		{"Social", &g.socialBtn},
		{"Settings", &g.settingsBtn},
	}

	availW := protocol.ScreenW - 2*pad
	spacing := pad
	btnH := btnH
	y0 := protocol.ScreenH - menuBarH + (menuBarH-btnH)/2

	basePadX := 16
	minPadX := 10
	minW := 64

	widths := make([]int, len(items))
	for i, it := range items {
		tw := text.BoundString(basicfont.Face7x13, it.label).Dx()
		w := tw + basePadX*2
		if w < minW {
			w = minW
		}
		widths[i] = w
	}

	maxPer := (availW - spacing*(len(items)-1)) / len(items)
	if maxPer < minW {
		maxPer = minW
	}
	for i := range widths {
		if widths[i] > maxPer {
			widths[i] = maxPer
		}
	}

	total := 0
	for _, w := range widths {
		total += w
	}
	need := total + spacing*(len(items)-1)
	if need > availW {

		spacing2 := spacing - (need-availW)/(len(items)-1) - 1
		if spacing2 < 4 {
			spacing2 = 4
		}
		spacing = spacing2

		need = total + spacing*(len(items)-1)
		if need > availW {
			for i := range widths {
				tw := text.BoundString(basicfont.Face7x13, items[i].label).Dx()
				minWi := tw + minPadX*2
				if minWi > widths[i] {
					minWi = widths[i]
				}
				widths[i] = minWi
			}
		}
	}

	finalTotal := 0
	for _, w := range widths {
		finalTotal += w
	}
	finalNeed := finalTotal + spacing*(len(items)-1)
	startX := pad + (availW-finalNeed)/2
	if startX < pad {
		startX = pad
	}

	x := startX
	for i, it := range items {
		*it.out = rect{x: x, y: y0, w: widths[i], h: btnH}
		x += widths[i] + spacing
	}
}
