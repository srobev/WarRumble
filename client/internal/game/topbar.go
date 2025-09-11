package game

import (
	"fmt"
	"image/color"

	"rumble/client/internal/game/assets/fonts"
	"rumble/shared/protocol"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
)

func (g *Game) drawTopBarHome(screen *ebiten.Image) {
	if g.topBarBg == nil {
		g.topBarBg = loadImage("assets/ui/top_bar_bg.png")
	}

	sw, _ := screen.Size()
	iw := g.topBarBg.Bounds().Dx()

	scaleX := float64(sw) / float64(iw)

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scaleX, 1)
	op.GeoM.Translate(0, 0)
	screen.DrawImage(g.topBarBg, op)

	g.computeTopBarLayout()

	mx, my := ebiten.CursorPosition()
	hoveredUser := g.userBtn.hit(mx, my)
	g.drawNineBtn(screen, g.userBtn, hoveredUser)

	// Remove the resource button - we'll handle resources through the gold area instead

	// avatar placeholder on the left
	const avW, avPad = 28, 10
	avH := g.userBtn.h - 8
	avX := g.userBtn.x + avPad
	avY := g.userBtn.y + (g.userBtn.h-avH)/2
	if img := g.ensureAvatarImage(g.avatar); img != nil {
		iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
		s := mathMin(float64(avW)/float64(iw), float64(avH)/float64(ih))
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(s, s)
		op.GeoM.Translate(float64(avX), float64(avY))
		screen.DrawImage(img, op)
	} else {
		ebitenutil.DrawRect(screen, float64(avX), float64(avY), float64(avW), float64(avH), color.NRGBA{70, 70, 90, 255})
	}

	name := g.name
	if name == "" {
		name = "Player"
	}
	uiFont := fonts.UI(12)
	lb := text.BoundString(uiFont, name)
	baselineY := g.userBtn.y + (g.userBtn.h+lb.Dy())/2 - 2
	text.Draw(screen, name, uiFont, g.userBtn.x+avPad+avW+8, baselineY, color.White)

	if hoveredUser && g.fantasyUI != nil {
		// Use FantasyUI themed tooltip
		title := "Account"
		label := g.name
		if label == "" {
			label = "Player"
		}
		ratingLbl := "PvP rating"
		ratingVal := fmt.Sprintf("%d  (%s)", g.pvpRating, defaultIfEmpty(g.pvpRank, "Unranked"))

		// Calculate tooltip dimensions using UI font
		tooltipFont := fonts.UI(14)
		titleW := text.BoundString(tooltipFont, title).Dx()
		nameW := text.BoundString(tooltipFont, label).Dx()
		rlW := text.BoundString(tooltipFont, ratingLbl).Dx()
		rvW := text.BoundString(tooltipFont, ratingVal).Dx()

		contentW := titleW
		if nameW > contentW {
			contentW = nameW
		}
		if rlW+10+rvW > contentW {
			contentW = rlW + 10 + rvW
		}

		const leftTextX = 68
		const padRight = 12
		tipW := leftTextX + contentW + padRight
		if tipW < 140 {
			tipW = 140
		}
		tipH := 85

		tx := clampInt(mx+14, 0, protocol.ScreenW-tipW)
		ty := clampInt(my+12, 0, protocol.ScreenH-tipH)

		// Draw themed tooltip background
		g.fantasyUI.DrawThemedPanel(screen, tx, ty, tipW, tipH, 0.9)

		// Avatar block with themed styling
		ax := tx + 10
		ay := ty + 10
		aw, ah := 56, 56
		if img := g.ensureAvatarImage(g.avatar); img != nil {
			iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
			s := mathMin(float64(aw)/float64(iw), float64(ah)/float64(ih))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(float64(ax)+(float64(aw)-float64(iw)*s)/2, float64(ay)+(float64(ah)-float64(ih)*s)/2)
			op.Filter = ebiten.FilterLinear // High-quality filtering
			screen.DrawImage(img, op)
		} else {
			// Themed placeholder
			ebitenutil.DrawRect(screen, float64(ax), float64(ay), float64(aw), float64(ah), g.fantasyUI.Theme.Surface)
		}

		// Text lines with theme colors
		fonts.DrawUIWithFallback(screen, title, tx+leftTextX, ty+25, 14, g.fantasyUI.Theme.TextPrimary)
		fonts.DrawUIWithFallback(screen, label, tx+leftTextX, ty+43, 14, g.fantasyUI.Theme.TextSecondary)
		fonts.DrawUIWithFallback(screen, ratingLbl, tx+leftTextX, ty+61, 14, g.fantasyUI.Theme.Secondary)
		fonts.DrawUIWithFallback(screen, ratingVal, tx+leftTextX+rlW+10, ty+61, 14, g.fantasyUI.Theme.TextPrimary)
	}

	title := protocol.GameName
	uiFontTitle := fonts.UI(13) // Slightly larger for title
	tb := text.BoundString(uiFontTitle, title)
	ty := g.titleArea.y + (topBarH+tb.Dy())/2 - 2
	fonts.DrawUIWithFallback(screen, title, g.titleArea.x+8, ty, 16, color.White)

	// Combined resource display - Gold + Dust + Capsules in one area
	resourceStr := fmt.Sprintf("%d", g.accountGold)
	resourceFontSize := 12.0
	resFont := fonts.UI(resourceFontSize)
	gb := text.BoundString(resFont, resourceStr)
	gy := g.goldArea.y + (topBarH+gb.Dy())/2 - 2
	fonts.DrawUIWithFallback(screen, resourceStr, g.goldArea.x, gy, resourceFontSize, color.NRGBA{240, 196, 25, 255})

	hoveredResource := g.goldArea.hit(mx, my)
	if hoveredResource && g.fantasyUI != nil {
		tip := []string{
			"Click see all resources.",
		}
		tipW, tipH := 200, 50
		tx := clampInt(mx-tipW, 0, protocol.ScreenW-tipW)
		ty := clampInt(my, 0, protocol.ScreenH-tipH)

		g.fantasyUI.DrawThemedTooltip(screen, tx, ty, tipW, tipH, "", tip)
	}

	// Draw expanded resource panel if shown
	if g.showResourcePanel {
		g.drawResourcePanel(screen, mx, my)
	}
}

func (g *Game) computeTopBarLayout() {
	// Button paddings and avatar size
	const padX = 10
	const avW = 28

	// Vertical margins inside the bar - reduced for taller button
	const userBtnVMargin = 2

	userBtnH := topBarH - 2*userBtnVMargin

	uname := g.name
	if uname == "" {
		uname = "Player"
	}
	uiFontLayout := fonts.UI(14)
	nameBounds := text.BoundString(uiFontLayout, uname)
	nameW := nameBounds.Dx()

	// Calculate button width with much more generous padding for better proportions
	btnW := padX*2 + avW + 16 + nameW + 40 // Much more padding: 16px between avatar and text, 40px extra
	if btnW < 200 {                        // Much larger minimum width for better proportions
		btnW = 200
	}

	g.userBtn = rect{
		x: pad,
		y: userBtnVMargin,
		w: btnW,
		h: userBtnH,
	}

	// Use combined resource display (Gold + Dust) in the same area - further to the right like account section
	resourceStr := fmt.Sprintf("%d", g.accountGold)
	resFontLayout := fonts.UI(12)
	rb := text.BoundString(resFontLayout, resourceStr)
	g.goldArea = rect{
		x: protocol.ScreenW - pad - rb.Dx(), // Remove extra 12px buffer to match account button style
		y: 0,
		w: rb.Dx() + 12,
		h: topBarH,
	}

	title := protocol.GameName
	titleFontLayout := fonts.UI(14)
	tb := text.BoundString(titleFontLayout, title)
	tx := (protocol.ScreenW - tb.Dx()) / 2
	g.titleArea = rect{x: tx - 8, y: 0, w: tb.Dx() + 16, h: topBarH}

	// Compute resource panel layout when showing (use gold area position as base)
	if g.showResourcePanel {
		g.computeResourcePanelLayout()
	}
}

// Helper for math.Min with float64
func mathMax(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}

// drawNineBtnDimmed draws a slightly transparent button
func (g *Game) drawNineBtnDimmed(screen *ebiten.Image, r rect) {
	g.assets.ensureInit()
	img := g.assets.btn9Base
	if img != nil {
		op := &ebiten.DrawImageOptions{}
		op.ColorM.Scale(1, 1, 1, 0.6)
		drawNineSlice(screen, img, r.x, r.y, r.w, r.h, NineSlice{Left: 6, Right: 6, Top: 6, Bottom: 6})

		// Apply composite operation
		op.GeoM.Reset()
		op.GeoM.Scale(float64(r.w)/float64(img.Bounds().Dx()), float64(r.h)/float64(img.Bounds().Dy()))
		op.GeoM.Translate(float64(r.x), float64(r.y))
		op.ColorM.Reset()
		op.ColorM.Scale(1, 1, 1, 0.6)
		screen.DrawImage(img, op)
		return
	}

	col := color.NRGBA{54, 63, 88, 150} // Dimmed version
	ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), col)
}

// computeResourcePanelLayout positions the resource panel elements
func (g *Game) computeResourcePanelLayout() {
	// Position resource panel below the gold area
	const panelPadding = 8
	const itemHeight = 20
	const itemSpacing = 4

	// Start positioning below the top bar, aligned with gold area
	baseY := topBarH + panelPadding
	baseX := g.goldArea.x + (g.goldArea.w / 2) - 100 // Center the panel around gold area

	// Resource panel background
	g.resourcePanelArea = rect{
		x: baseX - 5,
		y: topBarH + 4,
		w: 210,
		h: 100,
	}

	// Dust area
	g.dustArea = rect{
		x: baseX,
		y: baseY,
		w: 120,
		h: itemHeight,
	}

	// Rare capsules
	g.rareCapsArea = rect{
		x: baseX,
		y: baseY + itemHeight + itemSpacing,
		w: 70,
		h: itemHeight,
	}

	// Epic capsules
	g.epicCapsArea = rect{
		x: baseX + 80,
		y: baseY + itemHeight + itemSpacing,
		w: 70,
		h: itemHeight,
	}

	// Legendary capsules
	g.legendaryCapsArea = rect{
		x: baseX,
		y: baseY + 2*itemHeight + 2*itemSpacing,
		w: 140,
		h: itemHeight,
	}
}

// drawResourcePanel draws the expanded resource panel showing all resources
func (g *Game) drawResourcePanel(screen *ebiten.Image, mx, my int) {
	if g.fantasyUI == nil {
		// Fallback to basic drawing if no fantasy UI
		ebitenutil.DrawRect(screen, float64(g.resourcePanelArea.x), float64(g.resourcePanelArea.y),
			float64(g.resourcePanelArea.w), float64(g.resourcePanelArea.h), color.NRGBA{0, 0, 20, 200})
	} else {
		// Draw themed panel background
		g.fantasyUI.DrawThemedPanel(screen, g.resourcePanelArea.x, g.resourcePanelArea.y,
			g.resourcePanelArea.w, g.resourcePanelArea.h, 0.9)
	}

	textColorPrimary := color.NRGBA{240, 240, 240, 255}
	textColorSecondary := color.NRGBA{200, 200, 200, 255}

	// Draw resource items using UI fonts with fallback
	dustStr := fmt.Sprintf("⭐ Dust: %d", g.dust)
	fonts.DrawUIWithFallback(screen, dustStr, g.dustArea.x+2, g.dustArea.y+14, 14, textColorPrimary)

	rareStr := fmt.Sprintf("Rare: %d", g.capsules.Rare)
	fonts.DrawUIWithFallback(screen, rareStr, g.rareCapsArea.x+2, g.rareCapsArea.y+14, 14, textColorSecondary)

	epicStr := fmt.Sprintf("Epic: %d", g.capsules.Epic)
	fonts.DrawUIWithFallback(screen, epicStr, g.epicCapsArea.x+2, g.epicCapsArea.y+14, 14, textColorSecondary)

	legendaryStr := fmt.Sprintf("Legendary: %d", g.capsules.Legendary)
	fonts.DrawUIWithFallback(screen, legendaryStr, g.legendaryCapsArea.x+2, g.legendaryCapsArea.y+14, 14, textColorSecondary)

	// Draw tooltips for hovered resource types
	hoveredDust := g.dustArea.hit(mx, my)
	hoveredRare := g.rareCapsArea.hit(mx, my)
	hoveredEpic := g.epicCapsArea.hit(mx, my)
	hoveredLegendary := g.legendaryCapsArea.hit(mx, my)

	if (hoveredDust || hoveredRare || hoveredEpic || hoveredLegendary) && g.fantasyUI != nil {
		var tipTitle, tipDesc string
		tipW, tipH := 300, 70

		if hoveredDust {
			tipTitle = "Dust"
			tipDesc = "Upgrade currency for all unit advancements."
		} else if hoveredRare {
			tipTitle = "Rare Capsules"
			tipDesc = "Required for rare unit upgrades."
		} else if hoveredEpic {
			tipTitle = "Epic Capsules"
			tipDesc = "Required for epic unit upgrades."
		} else if hoveredLegendary {
			tipTitle = "Legendary Capsules"
			tipDesc = "Required for legendary unit upgrades."
		}

		tx := clampInt(mx-tipW-10, 0, protocol.ScreenW-tipW)
		ty := clampInt(my+12, 0, protocol.ScreenH-tipH)

		g.fantasyUI.DrawThemedTooltip(screen, tx, ty, tipW, tipH, tipTitle, []string{tipDesc})
	}
}
