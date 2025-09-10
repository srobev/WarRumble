package game

import (
	"rumble/shared/protocol"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

// Bottom bar button states
var bottomBarButtons map[string]*FantasyButton

func (g *Game) drawBottomBar(screen *ebiten.Image) {
	// Initialize bottom bar buttons if not already done
	if bottomBarButtons == nil {
		bottomBarButtons = make(map[string]*FantasyButton)
	}

	if g.bottomBarBg == nil {
		g.bottomBarBg = loadImage("assets/ui/bottom_bar_bg.png")
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

	// Get mouse position for hover detection
	mx, my := ebiten.CursorPosition()

	// Define button configurations
	buttonConfigs := []struct {
		key    string
		label  string
		rect   rect
		active bool
	}{
		{"shop", "Shop", g.shopBtn, g.activeTab == tabShop},
		{"army", "Army", g.armyBtn, g.activeTab == tabArmy},
		{"map", "Map", g.mapBtn, g.activeTab == tabMap},
		{"pvp", "PvP", g.pvpBtn, g.activeTab == tabPvp},
		{"social", "Social", g.socialBtn, g.activeTab == tabSocial},
		{"settings", "Settings", g.settingsBtn, g.activeTab == tabSettings},
	}

	// Create or update fantasy buttons
	for _, config := range buttonConfigs {
		btn, exists := bottomBarButtons[config.key]
		if !exists {
			btn = NewFantasyButton(config.rect.x, config.rect.y, config.rect.w, config.rect.h, config.label, g.fantasyUI.Theme, nil)
			btn.IsBottomBarButton = true // Enable bottom bar specific styling
			bottomBarButtons[config.key] = btn
		}

		// Update button position and size
		btn.X, btn.Y = config.rect.x, config.rect.y
		btn.Width, btn.Height = config.rect.w, config.rect.h

		// Determine button state based on hover and active status
		if btn.Contains(mx, my) {
			if config.active {
				btn.SetState(ButtonPressed) // Active + hover = pressed state
			} else {
				btn.SetState(ButtonHover) // Just hover
			}
		} else {
			if config.active {
				btn.SetState(ButtonPressed) // Active but not hovered = pressed state
			} else {
				btn.SetState(ButtonNormal) // Normal state
			}
		}

		// Draw the enhanced button
		btn.Draw(screen)
	}
}

func (g *Game) computeBottomBarLayout() {
	type item struct {
		label string
		out   *rect
	}
	items := []item{
		{"Shop", &g.shopBtn},
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
