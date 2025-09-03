package game

import (
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // (optional) register JPEG decoder
	_ "image/png"  // register PNG decoder
	"log"
	"math"
	"path"
	"rumble/client/internal/game/ui"
	"rumble/shared/protocol"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"golang.org/x/image/font/basicfont"

	"embed"
)

//go:embed assets/ui/* assets/ui/avatars/* assets/minis/* assets/maps/*
var assetsFS embed.FS

var ornate *ui.OrnateBar

// drawSmallLevelBadgeSized draws the level inside level_bar.png (if present) at a given pixel height.
func drawSmallLevelBadgeSized(dst *ebiten.Image, x, y, level int, size int) {
	// center for text
	cx, cy := x+size/2, y+size/2
	if ornate == nil {
		// Build ornate bar from embedded assets (no disk dependency)
		ob := &ui.OrnateBar{}
		// Try primary names shipped in repo
		ob.Frame = loadImage("assets/ui/health_bar.png")
		if ob.Frame == nil {
			ob.Frame = loadImage("assets/ui/bar_frame.png")
		}
		ob.Badge = loadImage("assets/ui/level_bar.png")
		if ob.Badge == nil {
			ob.Badge = loadImage("assets/ui/level_badge.png")
		}
		// Defaults reasonable for our images; can be tuned via meta later
		ob.WellOfs = image.Pt(130, 30)
		ob.WellSize = image.Pt(350, 44)
		ob.Mode = "fitpad"
		ob.PadX, ob.PadY = 2, 2
		ob.BadgeScale = 1.0
		ornate = ob
	}
	if ornate != nil && ornate.Badge != nil {
		bb := ornate.Badge.Bounds()
		if bb.Dx() > 0 && bb.Dy() > 0 {
			s := float64(size) / float64(bb.Dy())
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(float64(x), float64(y))
			dst.DrawImage(ornate.Badge, op)
		}
	} else {
		// fallback: small circle
		vector.DrawFilledCircle(dst, float32(cx), float32(cy), float32(size)/2, color.NRGBA{240, 196, 25, 255}, true)
		vector.DrawFilledCircle(dst, float32(cx), float32(cy), float32(size)/2-1.2, color.NRGBA{200, 160, 20, 255}, true)
	}
	// level number with outline
	s := fmt.Sprintf("%d", level)
	tw := text.BoundString(basicfont.Face7x13, s).Dx()
	th := text.BoundString(basicfont.Face7x13, s).Dy()
	tx := cx - tw/2
	ty := cy + th/2 - 2
	text.Draw(dst, s, basicfont.Face7x13, tx+1, ty, color.NRGBA{0, 0, 0, 200})
	text.Draw(dst, s, basicfont.Face7x13, tx-1, ty, color.NRGBA{0, 0, 0, 200})
	text.Draw(dst, s, basicfont.Face7x13, tx, ty+1, color.NRGBA{0, 0, 0, 200})
	text.Draw(dst, s, basicfont.Face7x13, tx, ty-1, color.NRGBA{0, 0, 0, 200})
	text.Draw(dst, s, basicfont.Face7x13, tx, ty, color.NRGBA{250, 250, 250, 255})
}

// default Army/overlay size
func drawSmallLevelBadge(dst *ebiten.Image, x, y, level int) {
	drawSmallLevelBadgeSized(dst, x, y, level, 14)
}

func drawBaseImg(screen *ebiten.Image, img *ebiten.Image, b protocol.BaseState) {
	if img == nil {

		ebitenutil.DrawRect(screen, float64(b.X), float64(b.Y), float64(b.W), float64(b.H), color.NRGBA{90, 90, 120, 255})
		return
	}
	iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
	if iw == 0 || ih == 0 {
		return
	}

	sx := float64(b.W) / float64(iw)
	sy := float64(b.H) / float64(ih)
	s := math.Min(sx, sy)

	op := &ebiten.DrawImageOptions{}
	ox := float64(b.X) + (float64(b.W)-float64(iw)*s)/2
	oy := float64(b.Y) + (float64(b.H)-float64(ih)*s)/2
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(ox, oy)
	// Use linear filtering for smoother scaling on high-resolution displays
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(img, op)
}

// index of real filenames by lowercase name per directory
var embIndex = map[string]map[string]string{}

func buildEmbIndex() {
	if len(embIndex) != 0 {
		return
	}
	for _, dir := range []string{"assets/ui", "assets/minis", "assets/maps"} {
		entries, err := assetsFS.ReadDir(dir)
		if err != nil {
			continue
		}
		m := make(map[string]string, len(entries))
		for _, e := range entries {
			m[strings.ToLower(e.Name())] = e.Name()
		}
		embIndex[dir] = m
	}
}

func loadImage(p string) *ebiten.Image {
	buildEmbIndex()

	f, err := assetsFS.Open(p)
	if err != nil {

		dir, file := path.Split(p)
		key := strings.ToLower(strings.TrimSuffix(file, ""))
		if idx, ok := embIndex[strings.TrimRight(dir, "/")]; ok {
			if real, ok2 := idx[key]; ok2 {
				f, err = assetsFS.Open(dir + real)
			}
		}
	}
	if err != nil {
		log.Println("asset not found in embed:", p)
		return nil
	}
	defer f.Close()

	img, _, err := ebitenutil.NewImageFromReader(f)
	if err != nil {
		log.Println("decode image failed:", p, err)
		return nil
	}
	return img
}

func (a *Assets) ensureInit() {
	if a.btn9Base == nil {
		a.btn9Base = loadImage("assets/ui/btn9_slim.png")
	}
	if a.btn9Hover == nil {
		a.btn9Hover = loadImage("assets/ui/btn9_slim_hover.png")
	}
	if a.minis == nil {
		a.minis = make(map[string]*ebiten.Image)
	}
	if a.bg == nil {
		a.bg = make(map[string]*ebiten.Image)
	}
	if a.baseMe == nil {
		a.baseMe = loadImage("assets/ui/base.png")
	}
	if a.baseEnemy == nil {
		a.baseEnemy = loadImage("assets/ui/base.png")
	}
	if a.baseDead == nil {
		a.baseDead = loadImage("assets/ui/base_destroyed.png")
	}
	if a.coinFull == nil {
		a.coinFull = loadImage("assets/ui/coin.png")
	}
	if a.coinEmpty == nil {
		a.coinEmpty = loadImage("assets/ui/coin_empty.png")
	}
	if a.edgeCol == nil {
		a.edgeCol = make(map[string]color.NRGBA)
	}
}

func (g *Game) portraitKeyFor(name string) string {

	if info, ok := g.nameToMini[name]; ok && info.Portrait != "" {
		return info.Portrait
	}

	return strings.ToLower(strings.ReplaceAll(name, " ", "_")) + ".png"
}

func (g *Game) ensureMiniImageByName(name string) *ebiten.Image {
	g.assets.ensureInit()
	key := g.portraitKeyFor(name)
	if img, ok := g.assets.minis[key]; ok {
		return img
	}
	img := loadImage("assets/minis/" + key)
	g.assets.minis[key] = img
	return img
}

func (g *Game) ensureBgForMap(mapID string) *ebiten.Image {
	g.assets.ensureInit()
	if img, ok := g.assets.bg[mapID]; ok {
		return img
	}

	base := strings.ToLower(mapID)
	for _, p := range []string{
		"assets/maps/" + base + ".png",
		"assets/maps/" + base + ".jpg",
	} {
		if img := loadImage(p); img != nil {
			g.assets.bg[mapID] = img
			return img
		}
	}

	log.Println("map background not found for mapID:", mapID)
	g.assets.bg[mapID] = nil
	return nil
}

// ---------- Home (Army / Map tabs) ----------
func (g *Game) updateHome() {
	mx, my := ebiten.CursorPosition()
	g.computeTopBarLayout()
	g.computeBottomBarLayout()

	if g.activeTab == tabArmy && len(g.minisAll) == 0 {
		g.requestLobbyDataOnce()
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if g.armyBtn.hit(mx, my) {
			g.activeTab = tabArmy
		}
		if g.mapBtn.hit(mx, my) {
			g.activeTab = tabMap
		}
		if g.settingsBtn.hit(mx, my) {
			g.activeTab = tabSettings
		}
		if g.logoutBtn.hit(mx, my) && g.activeTab == tabSettings {
			if g.net != nil && !g.net.IsClosed() {
				g.send("Logout", protocol.Logout{})
			}
			ClearToken()
			ClearUsername()
			g.resetToLoginNoAutoConnect()
		}
		if g.pvpBtn.hit(mx, my) {
			g.activeTab = tabPvp
		}
		if g.socialBtn.hit(mx, my) {
			g.activeTab = tabSocial
		}
	}

	if len(g.minisAll) == 0 {
		g.send("ListMinis", protocol.ListMinis{})
	}

	if inpututil.IsKeyJustPressed(ebiten.KeyF) && ebiten.IsKeyPressed(ebiten.KeyAlt) {
		g.fullscreen = !g.fullscreen
		ebiten.SetFullscreen(g.fullscreen)
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && my < topBarH {
		if g.userBtn.hit(mx, my) {
			log.Println("Account clicked")
			g.showProfile = true
			return

		} else if g.goldArea.hit(mx, my) {
			log.Println("Gold clicked")
		}
		return
	}

	if g.showProfile {
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			mx, my := ebiten.CursorPosition()

			if g.profCloseBtn.hit(mx, my) {
				g.showProfile = false
				return
			}

			if g.profLogoutBtn.hit(mx, my) {

				if g.net != nil && !g.net.IsClosed() {
					g.send("Logout", protocol.Logout{})
				}

				ClearToken()
				ClearUsername()
				g.resetToLoginNoAutoConnect()
				return
			}

			for i, r := range g.avatarRects {
				if r.hit(mx, my) && i >= 0 && i < len(g.avatars) {
					choice := g.avatars[i]
					_ = SaveAvatar(choice)
					g.avatar = choice
					g.send("SetAvatar", protocol.SetAvatar{Avatar: choice})
					break
				}
			}
		}

		return
	}

	switch g.activeTab {
	case tabArmy:
		mx, my := ebiten.CursorPosition()

		// ---- Layout numbers shared with Draw ----
		const champCardW, champCardH = 100, 116 // small cards in strip
		const miniCardW, miniCardH = 100, 116
		const gap = 10
		const stripH = champCardH + 8
		const dragThresh = 6 // pixels: below this is a click, above is a drag

		stripX := pad
		stripY := topBarH + pad
		stripW := protocol.ScreenW - 2*pad
		stepPx := champCardW + gap
		g.champStripArea = rect{x: stripX, y: stripY, w: stripW, h: stripH}

		bigW := 200
		bigH := miniCardH*2 + gap
		topY := stripY + stripH + 12
		leftX := pad
		rightX := leftX + bigW + 16

		// Overlay input handling
		if g.miniOverlayOpen {
			// Compute overlay rects (match draw)
			wDlg, hDlg := 580, 300
			slotsBottom := topY + 2*(miniCardH+gap) - gap
			dlgX := (protocol.ScreenW - wDlg) / 2
			dlgY := slotsBottom + 28
			if dlgY+hDlg > protocol.ScreenH-12 {
				dlgY = (protocol.ScreenH - hDlg) / 2
			}
			closeR := rect{x: dlgX + wDlg - 28, y: dlgY + 8, w: 20, h: 20}
			primaryR := rect{x: dlgX + 16 + (120-110)/2, y: dlgY + 36 + 140 + 10, w: 110, h: 26}

			// XP bar hover detection
			barX, barY := dlgX+160, dlgY+50+120
			barW, barH := wDlg-170-24, 30
			xpBarRect := rect{x: barX, y: barY, w: barW, h: barH}
			g.xpBarHovered = xpBarRect.hit(mx, my)
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
				// Close
				if closeR.hit(mx, my) {
					g.miniOverlayOpen = false
					g.miniOverlayMode = ""
					g.slotDragFrom, g.slotDragActive = -1, false
					g.overlayJustClosed = true
					return
				}
				// Equip from collection
				if g.miniOverlayFrom == "collection" && primaryR.hit(mx, my) {
					name := g.miniOverlayName
					empty := -1
					for i := 0; i < 6; i++ {
						if g.selectedOrder[i] == "" {
							empty = i
							break
						}
					}
					if empty >= 0 && !g.selectedMinis[name] {
						g.selectedOrder[empty] = name
						g.selectedMinis[name] = true
						g.setChampArmyFromSelected()
						g.autoSaveCurrentChampionArmy()
						g.miniOverlayOpen = false
						g.miniOverlayMode = ""
						g.overlayJustClosed = true
						return
					}
					g.miniOverlayMode = "switch_target_slot"
					return
				}
				// Target slot
				if g.miniOverlayMode == "switch_target_slot" {
					for i := 1; i <= 6; i++ {
						if g.armySlotRects[i].hit(mx, my) {
							to := i - 1
							prev := g.selectedOrder[to]
							if prev != "" {
								delete(g.selectedMinis, prev)
							}
							g.selectedOrder[to] = g.miniOverlayName
							g.selectedMinis[g.miniOverlayName] = true
							g.setChampArmyFromSelected()
							g.autoSaveCurrentChampionArmy()
							g.miniOverlayOpen = false
							g.miniOverlayMode = ""
							g.overlayJustClosed = true
							return
						}
					}
					return
				}
				return
			}
			// Block other interactions while overlay open
			return
		}
		if g.overlayJustClosed {
			// Clear as soon as the button is no longer pressed
			if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
				g.overlayJustClosed = false
			}
			return
		}

		cols := maxInt(1, (stripW+gap)/(champCardW+gap))
		start := clampInt(g.champStripScroll, 0, maxInt(0, len(g.champions)-cols))
		// center the visible champion strip within stripW
		visW := cols*champCardW + maxInt(0, cols-1)*gap
		baseX := stripX + (stripW-visW)/2
		g.champStripRects = g.champStripRects[:0]
		for i := 0; i < cols && start+i < len(g.champions); i++ {
			x := baseX + i*(champCardW+gap)
			g.champStripRects = append(g.champStripRects, rect{x: x, y: stripY, w: champCardW, h: champCardH})
		}

		if g.champStripArea.hit(mx, my) && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {

			g.champDragStartX = mx
			g.champDragLastX = mx
			g.champDragAccum = 0

		}

		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && g.champStripArea.hit(mx, my) {
			dx := mx - g.champDragStartX
			if !g.champDragActive && (dx >= dragThresh || dx <= -dragThresh) {
				g.champDragActive = true
			}
			if g.champDragActive {
				maxStart := maxInt(0, len(g.champions)-cols)

				step := mx - g.champDragLastX
				g.champDragLastX = mx
				g.champDragAccum += step
				for g.champDragAccum <= -stepPx && g.champStripScroll < maxStart {
					g.champStripScroll++
					g.champDragAccum += stepPx
				}
				for g.champDragAccum >= stepPx && g.champStripScroll > 0 {
					g.champStripScroll--
					g.champDragAccum -= stepPx
				}
			}
		}

		if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
			if g.champStripArea.hit(mx, my) && !g.champDragActive && (mx-g.champDragStartX <= dragThresh && g.champDragStartX-mx <= dragThresh) {
				for i, r := range g.champStripRects {
					if r.hit(mx, my) {
						ch := g.champions[start+i].Name
						g.setChampArmyFromSelected()
						g.loadSelectedForChampion(ch)
						g.autoSaveCurrentChampionArmy()
						break
					}
				}
			}

			g.champDragActive = false
			g.champDragAccum = 0
		}

		touchIDs := ebiten.AppendTouchIDs(nil)
		if len(touchIDs) > 0 {
			tid := touchIDs[0]
			tx, ty := ebiten.TouchPosition(tid)
			_ = ty
			if g.activeTouchID == -1 {
				if g.champStripArea.hit(tx, stripY+1) {
					g.activeTouchID = tid
					g.champDragStartX = tx
					g.champDragLastX = tx
					g.champDragAccum = 0
					g.champDragActive = false
				}
			} else if g.activeTouchID == tid {
				dx := tx - g.champDragStartX
				if !g.champDragActive && (dx >= dragThresh || dx <= -dragThresh) {
					g.champDragActive = true
				}
				if g.champDragActive {
					maxStart := maxInt(0, len(g.champions)-cols)
					step := tx - g.champDragLastX
					g.champDragLastX = tx
					g.champDragAccum += step
					for g.champDragAccum <= -stepPx && g.champStripScroll < maxStart {
						g.champStripScroll++
						g.champDragAccum += stepPx
					}
					for g.champDragAccum >= stepPx && g.champStripScroll > 0 {
						g.champStripScroll--
						g.champDragAccum -= stepPx
					}
				}
			}
		} else if g.activeTouchID != -1 {

			g.activeTouchID = -1
			g.champDragActive = false
			g.champDragAccum = 0
		}

		// Recompute equipped slot rects every frame to keep geometry in sync with drawing
		{
			// Layout numbers shared with draw
			const champCardW, champCardH = 100, 116
			const miniCardW, miniCardH = 100, 116
			const gap = 10
			const stripH = champCardH + 8

			stripX := pad
			stripY := topBarH + pad
			bigW := 200
			topY := stripY + stripH + 12
			// center champion + 2x3 slots as a block
			totalW := bigW + 16 + 3*miniCardW + 2*gap
			startX := (protocol.ScreenW - totalW) / 2
			leftX := startX
			rightX := leftX + bigW + 16

			// champion slot
			g.armySlotRects[0] = rect{x: leftX, y: topY, w: bigW, h: miniCardH*2 + gap}
			// 6 mini slots (2x3)
			k := 1
			for row := 0; row < 2; row++ {
				for col := 0; col < 3; col++ {
					g.armySlotRects[k] = rect{
						x: rightX + col*(miniCardW+gap),
						y: topY + row*(miniCardH+gap),
						w: miniCardW, h: miniCardH,
					}
					k++
				}
			}
			_ = stripX // silence unused vars in case
		}

		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !g.champDragActive && !g.miniOverlayOpen {

			g.armySlotRects[0] = rect{x: leftX, y: topY, w: bigW, h: bigH}

			k := 1
			for row := 0; row < 2; row++ {
				for col := 0; col < 3; col++ {
					g.armySlotRects[k] = rect{
						x: rightX + col*(miniCardW+gap),
						y: topY + row*(miniCardH+gap),
						w: miniCardW, h: miniCardH,
					}
					k++
				}
			}
			// champion overlay (display only)
			if g.armySlotRects[0].hit(mx, my) && g.selectedChampion != "" {
				g.miniOverlayName = g.selectedChampion
				g.miniOverlayFrom = "champion"
				g.miniOverlaySlot = -1
				g.miniOverlayMode = ""
				g.miniOverlayOpen = true
				return
			}
			for i := 1; i <= 6; i++ {
				if g.armySlotRects[i].hit(mx, my) {
					// start potential drag from this slot; overlay will open on release if not dragged
					g.slotDragFrom = i - 1
					g.slotDragStartX, g.slotDragStartY = mx, my
					g.slotDragActive = false
					return
				}
			}
		}

		// Drag between equipped slots
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && g.slotDragFrom >= 0 {
			dx := mx - g.slotDragStartX
			dy := my - g.slotDragStartY
			const dragThresh = 6
			if !g.slotDragActive && (dx >= dragThresh || dx <= -dragThresh || dy >= dragThresh || dy <= -dragThresh) {
				g.slotDragActive = true
			}
		}
		if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) && g.slotDragFrom >= 0 {
			from := g.slotDragFrom
			g.slotDragFrom = -1
			if g.slotDragActive {
				// perform swap if released over another slot
				// Use precomputed rects for accuracy
				for i := 1; i <= 6; i++ {
					if g.armySlotRects[i].hit(mx, my) {
						to := i - 1
						if to >= 0 && to < 6 && to != from {
							g.selectedOrder[from], g.selectedOrder[to] = g.selectedOrder[to], g.selectedOrder[from]
							g.setChampArmyFromSelected()
							g.autoSaveCurrentChampionArmy()
						}
						break
					}
				}
				g.slotDragActive = false
			} else {
				// treat as click: open overlay for that slot
				if from >= 0 && from < 6 {
					name := g.selectedOrder[from]
					g.miniOverlayName = name
					g.miniOverlayFrom = "slot"
					g.miniOverlaySlot = from
					g.miniOverlayMode = ""
					g.miniOverlayOpen = true
				}
			}
		}

		// Right-click on selected mini slots to open XP overlay
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
			// Recompute slot rects like above
			g.armySlotRects[0] = rect{x: leftX, y: topY, w: bigW, h: bigH}
			k := 1
			for row := 0; row < 2; row++ {
				for col := 0; col < 3; col++ {
					g.armySlotRects[k] = rect{
						x: rightX + col*(miniCardW+gap),
						y: topY + row*(miniCardH+gap),
						w: miniCardW, h: miniCardH,
					}
					k++
				}
			}
			order := g.selectedMinisList()
			for i := 1; i <= 6; i++ {
				if g.armySlotRects[i].hit(mx, my) {
					if i-1 < len(order) {
						g.miniOverlayName = order[i-1]
						g.miniOverlayOpen = true
					}
					break
				}
			}
		}

		// Reset hover states (already declared above)
		g.hoveredChampionLevel = -1
		g.hoveredChampionCost = -1
		g.hoveredChampionCard = -1
		g.hoveredSelectedChampionLevel = false
		g.hoveredSelectedChampionCost = false
		g.hoveredSelectedChampionCard = false
		g.hoveredMiniSlotLevel = -1
		g.hoveredMiniSlotCost = -1
		g.hoveredMiniSlotCard = -1
		g.hoveredCollectionLevel = -1
		g.hoveredCollectionCost = -1
		g.hoveredCollectionCard = -1
		g.hoveredOverlayLevel = false
		g.hoveredOverlayCost = false

		// Check hover for champion strip (top selection area)
		for i, r := range g.champStripRects {
			if r.hit(mx, my) {
				idx := start + i
				if idx >= 0 && idx < len(g.champions) {
					it := g.champions[idx]

					// Check level badge area (top-left corner)
					levelBadgeRect := rect{x: r.x + 4, y: r.y + 4, w: 24, h: 24}
					if levelBadgeRect.hit(mx, my) {
						g.hoveredChampionLevel = idx
					}

					// Check cost text area (bottom-right corner - updated position)
					costS := fmt.Sprintf("%d", it.Cost)
					cw := text.BoundString(basicfont.Face7x13, costS).Dx()
					costRect := rect{x: r.x + r.w - 6 - cw, y: r.y + r.h - 16, w: cw, h: 16}
					if costRect.hit(mx, my) {
						g.hoveredChampionCost = idx
					}

					// Set hover for the entire card (for frame effect)
					g.hoveredChampionCard = idx
				}
				break
			}
		}

		// Check hover for selected champion card (big one in middle)
		if g.selectedChampion != "" {
			chRect := g.armySlotRects[0] // champion slot rect
			if chRect.hit(mx, my) {
				// Check level badge area
				levelBadgeRect := rect{x: chRect.x + 4, y: chRect.y + 4, w: 24, h: 24}
				if levelBadgeRect.hit(mx, my) {
					g.hoveredSelectedChampionLevel = true
				}

				// Check cost text area (bottom area of big card)
				costRect := rect{x: chRect.x + 8, y: chRect.y + chRect.h - 20, w: chRect.w - 16, h: 16}
				if costRect.hit(mx, my) {
					g.hoveredSelectedChampionCost = true
				}

				// Set hover for the entire champion card (for frame effect)
				g.hoveredSelectedChampionCard = true
			}
		}

		// Check hover for equipped mini slots (2x3 grid)
		for i := 1; i <= 6; i++ {
			slotRect := g.armySlotRects[i]
			if slotRect.hit(mx, my) && i-1 < len(g.selectedOrder) && g.selectedOrder[i-1] != "" {
				name := g.selectedOrder[i-1]

				// Check level badge area (top-left)
				levelBadgeRect := rect{x: slotRect.x + 4, y: slotRect.y + 4, w: 24, h: 24}
				if levelBadgeRect.hit(mx, my) {
					g.hoveredMiniSlotLevel = i - 1
				}

				// Check cost text area (bottom-right)
				if info, ok := g.nameToMini[name]; ok {
					costS := fmt.Sprintf("%d", info.Cost)
					cw := text.BoundString(basicfont.Face7x13, costS).Dx()
					costRect := rect{x: slotRect.x + slotRect.w - cw - 8, y: slotRect.y + slotRect.h - 16, w: cw, h: 16}
					if costRect.hit(mx, my) {
						g.hoveredMiniSlotCost = i - 1
					}
				}

				// Set hover for the entire mini slot card (for frame effect)
				g.hoveredMiniSlotCard = i - 1
			}
		}

		gridTop := topY + bigH + 16
		gridLeft := pad
		gridRight := protocol.ScreenW - pad
		gridW := gridRight - gridLeft
		gridH := protocol.ScreenH - menuBarH - pad - gridTop
		visRows := maxInt(1, (gridH+gap)/(miniCardH+gap))
		cols2 := maxInt(1, (gridW+gap)/(miniCardW+gap))
		g.collArea = rect{x: gridLeft, y: gridTop, w: gridW, h: gridH}
		totalRows := (len(g.minisOnly) + cols2 - 1) / cols2
		maxRowsStart := maxInt(0, totalRows-visRows)
		stepPy := miniCardH + gap

		if g.collArea.hit(mx, my) && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			g.collDragStartY = my
			g.collDragLastY = my
			g.collDragAccum = 0
			g.collDragActive = false
		}

		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && g.collArea.hit(mx, my) {
			dy0 := my - g.collDragStartY
			if !g.collDragActive && (dy0 >= dragThresh || dy0 <= -dragThresh) {
				g.collDragActive = true
			}
			if g.collDragActive {
				dy := my - g.collDragLastY
				g.collDragLastY = my
				g.collDragAccum += dy
				for g.collDragAccum <= -stepPy && g.collScroll < maxRowsStart {
					g.collScroll++
					g.collDragAccum += stepPy
				}
				for g.collDragAccum >= stepPy && g.collScroll > 0 {
					g.collScroll--
					g.collDragAccum -= stepPy
				}
			}
		}

		items := g.minisOnly
		start2 := g.collScroll * cols2
		g.collRects = g.collRects[:0]
		maxItems := visRows * cols2
		for i := 0; i < maxItems && start2+i < len(items); i++ {
			c := i % cols2
			rw := i / cols2
			x := gridLeft + c*(miniCardW+gap)
			y := gridTop + rw*(miniCardH+gap)
			g.collRects = append(g.collRects, rect{x: x, y: y, w: miniCardW, h: miniCardH})
		}

		// Check hover for collection grid items - reset hover states first
		g.hoveredCollectionLevel = -1
		g.hoveredCollectionCost = -1

		// Build current visible items list to match drawing order
		avail := make([]protocol.MiniInfo, 0, len(g.minisOnly))
		for _, mi := range g.minisOnly {
			if !g.selectedMinis[mi.Name] {
				avail = append(avail, mi)
			}
		}

		// Check each visible grid rectangle
		for i, r := range g.collRects {
			if r.hit(mx, my) {
				idx := start2 + i
				if idx >= 0 && idx < len(avail) {
					it := avail[idx]

					// Check level badge area (top-left) - matches draw position exactly: r.x+4, r.y+4
					levelBadgeRect := rect{x: r.x + 4, y: r.y + 4, w: 24, h: 24}
					if levelBadgeRect.hit(mx, my) {
						g.hoveredCollectionLevel = idx
					}

					// Check cost text area (bottom-right) - matches draw position exactly: r.x+r.w-8-cw, r.y+r.h-16
					costS := fmt.Sprintf("%d", it.Cost)
					cw := text.BoundString(basicfont.Face7x13, costS).Dx()
					costRect := rect{x: r.x + r.w - cw - 8, y: r.y + r.h - 16, w: cw, h: 16}
					if costRect.hit(mx, my) {
						g.hoveredCollectionCost = idx
					}

					// Set hover for the entire collection card (for frame effect)
					g.hoveredCollectionCard = idx
				}
				break
			}
		}

		// Check hover for mini overlay (when open)
		if g.miniOverlayOpen {
			// Position calculations matching the draw function
			w, h := 580, 300
			x := (protocol.ScreenW - w) / 2
			stripY := topBarH + pad
			stripH := 116 + 8
			topY := stripY + stripH + 12
			slotsBottom := topY + 2*(miniCardH+gap) - gap
			y := slotsBottom + 28
			if y+h > protocol.ScreenH-12 {
				y = (protocol.ScreenH - h) / 2
			}

			// Check level badge area in overlay - matches draw position exactly (px+0, py+0)
			// where px = x + 16, py = y + 36
			levelBadgeRect := rect{x: x + 16, y: y + 36, w: 24, h: 24}
			if levelBadgeRect.hit(mx, my) {
				g.hoveredOverlayLevel = true
			}

			// Check cost text area in overlay (right side stats) - matches draw position
			costRect := rect{x: x + 170, y: y + 50, w: 100, h: 16}
			if costRect.hit(mx, my) {
				g.hoveredOverlayCost = true
			}
		}

		if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) && !g.miniOverlayOpen && g.collArea.hit(mx, my) && !g.collDragActive &&
			(my-g.collDragStartY <= dragThresh && g.collDragStartY-my <= dragThresh) {
			for i, r := range g.collRects {
				if r.hit(mx, my) {
					idx := start2 + i
					if idx >= 0 && idx < len(avail) {
						g.miniOverlayName = avail[idx].Name
						g.miniOverlayFrom = "collection"
						g.miniOverlaySlot = -1
						g.miniOverlayMode = ""
						g.miniOverlayOpen = true
					}
					break
				}
			}
		}

		if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
			g.collDragActive = false
			g.collDragAccum = 0
		}

		tids := ebiten.AppendTouchIDs(nil)
		if len(tids) > 0 {
			tid := tids[0]
			tx, ty := ebiten.TouchPosition(tid)
			if g.collTouchID == -1 {
				if g.collArea.hit(tx, ty) {
					g.collTouchID = tid
					g.collDragStartY = ty
					g.collDragLastY = ty
					g.collDragAccum = 0
					g.collDragActive = false
				}
			} else if g.collTouchID == tid {
				dy0 := ty - g.collDragStartY
				if !g.collDragActive && (dy0 >= dragThresh || dy0 <= -dragThresh) {
					g.collDragActive = true
				}
				if g.collDragActive {
					dy := ty - g.collDragLastY
					g.collDragLastY = ty
					g.collDragAccum += dy
					for g.collDragAccum <= -stepPy && g.collScroll < maxRowsStart {
						g.collScroll++
						g.collDragAccum += stepPy
					}
					for g.collDragAccum >= stepPy && g.collScroll > 0 {
						g.collScroll--
						g.collDragAccum -= stepPy
					}
				}
			}
		} else if g.collTouchID != -1 {

			g.collTouchID = -1
			g.collDragActive = false
			g.collDragAccum = 0
		}

		if _, wY := ebiten.Wheel(); wY != 0 {
			if g.champStripArea.hit(mx, my) {
				g.champStripScroll -= int(wY)
				maxStart := maxInt(0, len(g.champions)-cols)
				g.champStripScroll = clampInt(g.champStripScroll, 0, maxStart)
			} else if g.collArea.hit(mx, my) {
				g.collScroll -= int(wY)
				if g.collScroll < 0 {
					g.collScroll = 0
				}
				totalRows := (len(g.minisOnly) + cols2 - 1) / cols2
				maxRowsStart := maxInt(0, totalRows-visRows)
				if g.collScroll > maxRowsStart {
					g.collScroll = maxRowsStart
				}
			}
		}
	case tabMap:
		g.ensureMapHotspots()

		mx, my = ebiten.CursorPosition()
		disp := g.displayMapID()
		bg := g.ensureBgForMap(disp)
		offX, offY, dispW, dispH, _ := g.mapRenderRect(bg)

		g.hoveredHS = -1
		hsList := g.mapHotspots[disp]
		if hsList == nil {
			hsList = g.mapHotspots[defaultMapID]
		}
		for i, hs := range hsList {
			cx := offX + int(hs.X*float64(dispW))
			cy := offY + int(hs.Y*float64(dispH))
			dx, dy := mx-cx, my-cy
			if dx*dx+dy*dy <= hs.Rpx*hs.Rpx {
				g.hoveredHS = i
				break
			}
		}

		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if g.hoveredHS >= 0 {
				g.selectedHS = g.hoveredHS

				hs := hsList[g.selectedHS]
				arenaID := hs.TargetMapID
				if arenaID == "" {
					arenaID = g.arenaForHotspot(disp, hs.ID)
				}

				g.onMapClicked(arenaID)
			}

			if g.selectedHS >= 0 && g.selectedHS < len(hsList) {
				hs := hsList[g.selectedHS]
				cx := offX + int(hs.X*float64(dispW))
				cy := offY + int(hs.Y*float64(dispH))
				g.startBtn = rect{x: cx + 22, y: cy - 16, w: 90, h: 28}
				if g.startBtn.hit(mx, my) {
					g.onStartBattle()
				}
			}
		}

		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) &&
			mx >= offX && mx < offX+dispW && my >= offY && my < offY+dispH {
			nx := (float64(mx - offX)) / float64(dispW)
			ny := (float64(my - offY)) / float64(dispH)
			log.Printf("map '%s' pick: X: %.4f, Y: %.4f", disp, nx, ny)
		}
	case tabPvp:

		queueBtn, leaveBtn, createBtn, cancelBtn, joinInput, joinBtn := g.pvpLayout()
		mx, my := ebiten.CursorPosition()

		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Handle button clicks based on current state (matching the drawing logic)
			if !g.pvpQueued && queueBtn.hit(mx, my) {
				// Queue PvP button clicked
				g.pvpQueued = true
				g.pvpStatus = "Queueing for PvP…"
				g.send("JoinPvpQueue", struct{}{})
			} else if g.pvpQueued && leaveBtn.hit(mx, my) {
				// Leave Queue button clicked
				g.pvpQueued = false
				g.pvpStatus = "Left queue."
				g.send("LeavePvpQueue", struct{}{})
			} else if !g.pvpHosting && createBtn.hit(mx, my) {
				// Create Friendly Code button clicked
				g.pvpHosting = true
				g.pvpCode = ""
				g.pvpStatus = "Requesting friendly code…"
				g.send("FriendlyCreate", protocol.FriendlyCreate{})
			} else if g.pvpHosting && cancelBtn.hit(mx, my) {
				// Cancel Friendly button clicked
				g.pvpHosting = false
				g.pvpStatus = "Cancelled friendly host."
				g.pvpCode = ""
				g.send("FriendlyCancel", protocol.FriendlyCancel{})
			} else if joinInput.hit(mx, my) {
				// Input field clicked
				g.pvpInputActive = true
			} else if g.pvpCodeArea.hit(mx, my) && g.pvpHosting && g.pvpCode != "" {
				// Code area clicked (copy to clipboard)
				if err := clipboard.WriteAll(g.pvpCode); err != nil {
					g.pvpStatus = "Couldn't copy (install xclip/xsel on Linux)."
					log.Println("clipboard copy failed:", err)
				} else {
					g.pvpStatus = "Code copied to clipboard."
				}
			} else if joinBtn.hit(mx, my) {
				// Join button clicked
				code := strings.ToUpper(strings.TrimSpace(g.pvpCodeInput))
				if code == "" {
					g.pvpStatus = "Enter a code first."
				} else {
					g.pvpStatus = "Joining room " + code + "…"
					g.send("FriendlyJoin", protocol.FriendlyJoin{Code: code})
				}
				g.pvpInputActive = false
			} else {
				// Clicked elsewhere - deactivate input
				g.pvpInputActive = false
			}
		}

		if g.pvpInputActive {
			// Handle paste functionality (Ctrl+V / Cmd+V)
			if (ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)) && inpututil.IsKeyJustPressed(ebiten.KeyV) {
				if pastedText, err := clipboard.ReadAll(); err == nil {
					// Filter pasted text to only allow uppercase letters and numbers
					filtered := ""
					for _, r := range strings.ToUpper(pastedText) {
						if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
							filtered += string(r)
						}
					}
					// Add filtered text up to the 8 character limit
					for _, r := range filtered {
						if len(g.pvpCodeInput) >= 8 {
							break
						}
						g.pvpCodeInput += string(r)
					}
				}
			} else {
				// Handle individual key presses
				for _, k := range inpututil.AppendJustPressedKeys(nil) {
					switch k {
					case ebiten.KeyBackspace:
						if len(g.pvpCodeInput) > 0 {
							g.pvpCodeInput = g.pvpCodeInput[:len(g.pvpCodeInput)-1]
						}
					case ebiten.KeyEnter:
						code := strings.ToUpper(strings.TrimSpace(g.pvpCodeInput))
						if code == "" {
							g.pvpStatus = "Enter a code first."
						} else {
							g.pvpStatus = "Joining room " + code + "…"
							g.send("FriendlyJoin", protocol.FriendlyJoin{Code: code})
						}
						g.pvpInputActive = false

					default:

						if k >= ebiten.KeyA && k <= ebiten.KeyZ {
							if len(g.pvpCodeInput) < 8 {
								g.pvpCodeInput += string('A' + (k - ebiten.KeyA))
							}
							continue
						}

						if k >= ebiten.Key0 && k <= ebiten.Key9 {
							if len(g.pvpCodeInput) < 8 {
								g.pvpCodeInput += string('0' + (k - ebiten.Key0))
							}
							continue
						}

						if k >= ebiten.KeyKP0 && k <= ebiten.KeyKP9 {
							if len(g.pvpCodeInput) < 8 {
								g.pvpCodeInput += string('0' + (k - ebiten.KeyKP0))
							}
							continue
						}
					}
				}
			}
		}

		if time.Since(g.lbLastReq) > 10*time.Second {
			g.send("GetLeaderboard", protocol.GetLeaderboard{})
			g.lbLastReq = time.Now()
		}

	case tabSocial:
		g.updateSocial()
	case tabSettings:
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if g.fsOnBtn.hit(mx, my) {
				ebiten.SetFullscreen(true)
				g.fullscreen = true
			}
			if g.fsOffBtn.hit(mx, my) {
				ebiten.SetFullscreen(false)
				g.fullscreen = false
			}

		}
	}
}

func (g *Game) drawHomeContent(screen *ebiten.Image) {
	switch g.activeTab {
	case tabArmy:
		// Layout numbers (mirror Update)
		const champCardW, champCardH = 100, 116
		const miniCardW, miniCardH = 100, 116
		const gap = 10
		const stripH = champCardH + 8

		stripX := pad
		stripY := topBarH + pad
		stripW := protocol.ScreenW - 2*pad
		g.champStripArea = rect{x: stripX, y: stripY, w: stripW, h: stripH}

		cols := maxInt(1, (stripW+gap)/(champCardW+gap))
		start := clampInt(g.champStripScroll, 0, maxInt(0, len(g.champions)-cols))
		// Center the visible strip block
		visW := cols*champCardW + maxInt(0, cols-1)*gap
		baseX := stripX + (stripW-visW)/2
		g.champStripRects = g.champStripRects[:0]
		for i := 0; i < cols && start+i < len(g.champions); i++ {
			x := baseX + i*(champCardW+gap)
			r := rect{x: x, y: stripY, w: champCardW, h: champCardH}
			g.champStripRects = append(g.champStripRects, r)

			it := g.champions[start+i]

			// Use FantasyUI themed card
			if g.fantasyUI != nil {
				isSelected := g.selectedChampion == it.Name
				isHovered := g.hoveredChampionCard >= 0 && g.hoveredChampionCard == start+i

				g.fantasyUI.DrawEnhancedUnitCard(screen, r.x, r.y, r.w, r.h,
					it.Name, strings.Title(it.Class), 1, it.Cost, nil, isSelected, isHovered)
			} else {
				// Fallback to basic styling
				ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{0x2b, 0x2b, 0x3e, 0xff})
			}

			if img := g.ensureMiniImageByName(it.Name); img != nil {
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
				pw, ph := float64(r.w-8), float64(r.h-24)
				s := mathMin(pw/float64(iw), ph/float64(ih))
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(s, s)
				op.GeoM.Translate(float64(r.x)+4, float64(r.y)+4)
				op.Filter = ebiten.FilterLinear // High-quality filtering
				screen.DrawImage(img, op)
			}

			// Level top-left (keep existing level badge)
			lvl := 1
			if g.unitXP != nil {
				if xp, ok := g.unitXP[it.Name]; ok {
					l, _, _ := computeLevel(xp)
					if l > 0 {
						lvl = l
					}
				}
			}
			drawSmallLevelBadgeSized(screen, r.x+4, r.y+4, lvl, 24)

			// Selection indicator with theme color
			if g.selectedChampion == it.Name {
				selectionColor := color.NRGBA{240, 196, 25, 255}
				if g.fantasyUI != nil {
					selectionColor = g.fantasyUI.Theme.Secondary
				}
				ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), 2, selectionColor)
			}
		}

		topY := stripY + stripH + 12
		bigW := 200
		bigH := miniCardH*2 + gap
		// Center champion card + 2x3 grid as a block
		totalW := bigW + 16 + 3*miniCardW + 2*gap
		startX := (protocol.ScreenW - totalW) / 2
		leftX := startX

		chRect := rect{x: leftX, y: topY, w: bigW, h: bigH}
		if g.selectedChampion != "" {
			// Get champion cost from champion data
			championCost := 0
			for _, champ := range g.champions {
				if champ.Name == g.selectedChampion {
					championCost = champ.Cost
					break
				}
			}

			// Use FantasyUI themed card for selected champion
			if g.fantasyUI != nil {
				g.fantasyUI.DrawEnhancedUnitCard(screen, chRect.x, chRect.y, chRect.w, chRect.h,
					g.selectedChampion, "Champion", 1, championCost, nil, true, g.hoveredSelectedChampionCard)
			} else {
				ebitenutil.DrawRect(screen, float64(chRect.x), float64(chRect.y), float64(chRect.w), float64(chRect.h), color.NRGBA{0x2b, 0x2b, 0x3e, 0xff})
			}

			if img := g.ensureMiniImageByName(g.selectedChampion); img != nil {
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
				pw, ph := float64(chRect.w-8), float64(chRect.h-24)
				s := mathMin(pw/float64(iw), ph/float64(ih))
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(s, s)
				op.GeoM.Translate(float64(chRect.x)+4, float64(chRect.y)+4)
				op.Filter = ebiten.FilterLinear // High-quality filtering
				screen.DrawImage(img, op)
			}

			// Level top-left of champion card
			lvl := 1
			if g.unitXP != nil {
				if xp, ok := g.unitXP[g.selectedChampion]; ok {
					l, _, _ := computeLevel(xp)
					if l > 0 {
						lvl = l
					}
				}
			}
			drawSmallLevelBadgeSized(screen, chRect.x+4, chRect.y+4, lvl, 24)
		} else {
			// Empty champion slot with themed styling
			if g.fantasyUI != nil {
				g.fantasyUI.DrawThemedCard(screen, chRect.x, chRect.y, chRect.w, chRect.h,
					"", []string{"Select a champion from above"})
			} else {
				ebitenutil.DrawRect(screen, float64(chRect.x), float64(chRect.y), float64(chRect.w), float64(chRect.h), color.NRGBA{0x2b, 0x2b, 0x3e, 0xff})
				text.Draw(screen, "Champion (select above)", basicfont.Face7x13, chRect.x+6, chRect.y+18, color.NRGBA{200, 200, 200, 255})
			}
		}

		gridX := leftX + bigW + 16
		gridY := topY
		k := 0
		for row := 0; row < 2; row++ {
			for col := 0; col < 3; col++ {
				r := rect{
					x: gridX + col*(miniCardW+gap),
					y: gridY + row*(miniCardH+gap),
					w: miniCardW, h: miniCardH,
				}

				if k < 6 && g.selectedOrder[k] != "" {
					name := g.selectedOrder[k]

					// Get mini cost from mini data
					miniCost := 0
					if info, ok := g.nameToMini[name]; ok {
						miniCost = info.Cost
					}

					// Use FantasyUI themed card for equipped minis
					if g.fantasyUI != nil {
						isHovered := g.hoveredMiniSlotCard >= 0 && g.hoveredMiniSlotCard == k

						g.fantasyUI.DrawEnhancedUnitCard(screen, r.x, r.y, r.w, r.h,
							name, "Mini", 1, miniCost, nil, false, isHovered)
					} else {
						ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{0x26, 0x26, 0x35, 0xff})
					}

					if img := g.ensureMiniImageByName(name); img != nil {
						iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
						pw, ph := float64(r.w-8), float64(r.h-24)
						s := mathMin(pw/float64(iw), ph/float64(ih))
						op := &ebiten.DrawImageOptions{}
						op.GeoM.Scale(s, s)
						op.GeoM.Translate(float64(r.x)+4, float64(r.y)+4)
						op.Filter = ebiten.FilterLinear // High-quality filtering
						screen.DrawImage(img, op)
					}

					// Level top-left for equipped minis
					lvl := 1
					if g.unitXP != nil {
						if xp, ok := g.unitXP[name]; ok {
							l, _, _ := computeLevel(xp)
							if l > 0 {
								lvl = l
							}
						}
					}
					drawSmallLevelBadgeSized(screen, r.x+4, r.y+4, lvl, 24)
				} else {
					// Empty mini slot with themed styling
					if g.fantasyUI != nil {
						g.fantasyUI.DrawThemedCard(screen, r.x, r.y, r.w, r.h,
							"", []string{"Empty slot"})
					} else {
						ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{0x26, 0x26, 0x35, 0xff})
						text.Draw(screen, "Mini", basicfont.Face7x13, r.x+6, r.y+14, color.NRGBA{200, 200, 200, 255})
						text.Draw(screen, "(empty)", basicfont.Face7x13, r.x+6, r.y+r.h-6, color.NRGBA{160, 160, 160, 255})
					}
				}
				k++
			}
		}
		// count equipped
		cnt := 0
		for i := 0; i < 6; i++ {
			if g.selectedOrder[i] != "" {
				cnt++
			}
		}
		text.Draw(screen, fmt.Sprintf("Minis: %d/6", cnt), basicfont.Face7x13, gridX, gridY-6, color.White)

		gridTop := topY + bigH + 16
		// Center the collection grid rows
		maxW := protocol.ScreenW - 2*pad
		cols2 := maxInt(1, (maxW+gap)/(miniCardW+gap))
		contentW := cols2*miniCardW + maxInt(0, cols2-1)*gap
		gridLeft := (protocol.ScreenW - contentW) / 2
		gridW := contentW
		gridH := protocol.ScreenH - menuBarH - pad - gridTop
		visRows := maxInt(1, (gridH+gap)/(miniCardH+gap))
		g.collArea = rect{x: gridLeft, y: gridTop, w: gridW, h: gridH}

		start2 := g.collScroll * cols2
		// Build available (non-equipped) minis list
		avail := make([]protocol.MiniInfo, 0, len(g.minisOnly))
		for _, mi := range g.minisOnly {
			if !g.selectedMinis[mi.Name] {
				avail = append(avail, mi)
			}
		}
		g.collRects = g.collRects[:0]
		maxItems := visRows * cols2
		for i := 0; i < maxItems && start2+i < len(avail); i++ {
			c := i % cols2
			rw := i / cols2
			x := gridLeft + c*(miniCardW+gap)
			y := gridTop + rw*(miniCardH+gap)
			r := rect{x: x, y: y, w: miniCardW, h: miniCardH}
			g.collRects = append(g.collRects, r)

			it := avail[start2+i]

			// Use FantasyUI themed card for collection items
			if g.fantasyUI != nil {
				isSelected := g.selectedMinis[it.Name]
				isHovered := g.hoveredCollectionCard >= 0 && g.hoveredCollectionCard == start2+i

				g.fantasyUI.DrawEnhancedUnitCard(screen, r.x, r.y, r.w, r.h,
					it.Name, strings.Title(it.Class), 1, it.Cost, nil, isSelected, isHovered)
			} else {
				// Fallback to basic styling
				ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{0x2b, 0x2b, 0x3e, 0xff})
			}

			if img := g.ensureMiniImageByName(it.Name); img != nil {
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
				pw, ph := float64(r.w-8), float64(r.h-24)
				s := mathMin(pw/float64(iw), ph/float64(ih))
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(s, s)
				op.GeoM.Translate(float64(r.x)+4, float64(r.y)+4)
				op.Filter = ebiten.FilterLinear // High-quality filtering
				screen.DrawImage(img, op)
			}

			// Level top-left for collection items
			lvl := 1
			if g.unitXP != nil {
				if xp, ok := g.unitXP[it.Name]; ok {
					l, _, _ := computeLevel(xp)
					if l > 0 {
						lvl = l
					}
				}
			}
			drawSmallLevelBadgeSized(screen, r.x+4, r.y+4, lvl, 24)

			// Selection indicator with theme color
			if g.selectedMinis[it.Name] {
				selectionColor := color.NRGBA{240, 196, 25, 255}
				if g.fantasyUI != nil {
					selectionColor = g.fantasyUI.Theme.Secondary
				}
				ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), 2, selectionColor)
			}
		}

		// Draw hover tooltips for champion cards
		mx, my := ebiten.CursorPosition()
		if g.hoveredChampionLevel >= 0 && g.hoveredChampionLevel < len(g.champions) {
			it := g.champions[g.hoveredChampionLevel]
			lvl := 1
			if g.unitXP != nil {
				if xp, ok := g.unitXP[it.Name]; ok {
					l, _, _ := computeLevel(xp)
					if l > 0 {
						lvl = l
					}
				}
			}

			tooltipText := fmt.Sprintf("Level %d", lvl)
			tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
			th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

			// Position tooltip above the level badge
			tooltipX := mx - tw/2
			tooltipY := my - th - 8

			// Keep tooltip on screen
			if tooltipX < 4 {
				tooltipX = 4
			}
			if tooltipX+tw+8 > protocol.ScreenW {
				tooltipX = protocol.ScreenW - tw - 8
			}
			if tooltipY < 4 {
				tooltipY = my + 16
			}

			// Draw tooltip background
			ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

			// Draw tooltip text
			text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.White)
		}

		if g.hoveredChampionCost >= 0 && g.hoveredChampionCost < len(g.champions) {
			it := g.champions[g.hoveredChampionCost]

			tooltipText := fmt.Sprintf("Deploy cost %d", it.Cost)
			tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
			th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

			// Position tooltip above the cost text
			tooltipX := mx - tw/2
			tooltipY := my - th - 8

			// Keep tooltip on screen
			if tooltipX < 4 {
				tooltipX = 4
			}
			if tooltipX+tw+8 > protocol.ScreenW {
				tooltipX = protocol.ScreenW - tw - 8
			}
			if tooltipY < 4 {
				tooltipY = my + 16
			}

			// Draw tooltip background
			ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

			// Draw tooltip text
			text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.NRGBA{255, 215, 0, 255}) // Gold color for cost
		}

		// Draw tooltips for selected champion
		if g.hoveredSelectedChampionLevel && g.selectedChampion != "" {
			lvl := 1
			if g.unitXP != nil {
				if xp, ok := g.unitXP[g.selectedChampion]; ok {
					l, _, _ := computeLevel(xp)
					if l > 0 {
						lvl = l
					}
				}
			}

			tooltipText := fmt.Sprintf("Level %d", lvl)
			tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
			th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

			// Position tooltip above the level badge
			tooltipX := mx - tw/2
			tooltipY := my - th - 8

			// Keep tooltip on screen
			if tooltipX < 4 {
				tooltipX = 4
			}
			if tooltipX+tw+8 > protocol.ScreenW {
				tooltipX = protocol.ScreenW - tw - 8
			}
			if tooltipY < 4 {
				tooltipY = my + 16
			}

			// Draw tooltip background
			ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

			// Draw tooltip text
			text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.White)
		}

		if g.hoveredSelectedChampionCost && g.selectedChampion != "" {
			if info, ok := g.nameToMini[g.selectedChampion]; ok {
				tooltipText := fmt.Sprintf("Deploy cost %d", info.Cost)
				tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
				th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

				// Position tooltip above the cost text
				tooltipX := mx - tw/2
				tooltipY := my - th - 8

				// Keep tooltip on screen
				if tooltipX < 4 {
					tooltipX = 4
				}
				if tooltipX+tw+8 > protocol.ScreenW {
					tooltipX = protocol.ScreenW - tw - 8
				}
				if tooltipY < 4 {
					tooltipY = my + 16
				}

				// Draw tooltip background
				ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

				// Draw tooltip text
				text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.NRGBA{255, 215, 0, 255}) // Gold color for cost
			}
		}

		// Draw tooltips for equipped mini slots
		if g.hoveredMiniSlotLevel >= 0 && g.hoveredMiniSlotLevel < len(g.selectedOrder) && g.selectedOrder[g.hoveredMiniSlotLevel] != "" {
			name := g.selectedOrder[g.hoveredMiniSlotLevel]
			lvl := 1
			if g.unitXP != nil {
				if xp, ok := g.unitXP[name]; ok {
					l, _, _ := computeLevel(xp)
					if l > 0 {
						lvl = l
					}
				}
			}

			tooltipText := fmt.Sprintf("Level %d", lvl)
			tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
			th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

			// Position tooltip above the level badge
			tooltipX := mx - tw/2
			tooltipY := my - th - 8

			// Keep tooltip on screen
			if tooltipX < 4 {
				tooltipX = 4
			}
			if tooltipX+tw+8 > protocol.ScreenW {
				tooltipX = protocol.ScreenW - tw - 8
			}
			if tooltipY < 4 {
				tooltipY = my + 16
			}

			// Draw tooltip background
			ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

			// Draw tooltip text
			text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.White)
		}

		if g.hoveredMiniSlotCost >= 0 && g.hoveredMiniSlotCost < len(g.selectedOrder) && g.selectedOrder[g.hoveredMiniSlotCost] != "" {
			name := g.selectedOrder[g.hoveredMiniSlotCost]
			if info, ok := g.nameToMini[name]; ok {
				tooltipText := fmt.Sprintf("Deploy cost %d", info.Cost)
				tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
				th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

				// Position tooltip above the cost text
				tooltipX := mx - tw/2
				tooltipY := my - th - 8

				// Keep tooltip on screen
				if tooltipX < 4 {
					tooltipX = 4
				}
				if tooltipX+tw+8 > protocol.ScreenW {
					tooltipX = protocol.ScreenW - tw - 8
				}
				if tooltipY < 4 {
					tooltipY = my + 16
				}

				// Draw tooltip background
				ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

				// Draw tooltip text
				text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.NRGBA{255, 215, 0, 255}) // Gold color for cost
			}
		}

		// Draw tooltips for collection grid items
		if g.hoveredCollectionLevel >= 0 && g.hoveredCollectionLevel < len(avail) {
			it := avail[g.hoveredCollectionLevel]
			lvl := 1
			if g.unitXP != nil {
				if xp, ok := g.unitXP[it.Name]; ok {
					l, _, _ := computeLevel(xp)
					if l > 0 {
						lvl = l
					}
				}
			}

			tooltipText := fmt.Sprintf("Level %d", lvl)
			tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
			th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

			// Position tooltip above the level badge
			tooltipX := mx - tw/2
			tooltipY := my - th - 8

			// Keep tooltip on screen
			if tooltipX < 4 {
				tooltipX = 4
			}
			if tooltipX+tw+8 > protocol.ScreenW {
				tooltipX = protocol.ScreenW - tw - 8
			}
			if tooltipY < 4 {
				tooltipY = my + 16
			}

			// Draw tooltip background
			ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

			// Draw tooltip text
			text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.White)
		}

		if g.hoveredCollectionCost >= 0 && g.hoveredCollectionCost < len(avail) {
			it := avail[g.hoveredCollectionCost]

			tooltipText := fmt.Sprintf("Deploy cost %d", it.Cost)
			tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
			th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

			// Position tooltip above the cost text
			tooltipX := mx - tw/2
			tooltipY := my - th - 8

			// Keep tooltip on screen
			if tooltipX < 4 {
				tooltipX = 4
			}
			if tooltipX+tw+8 > protocol.ScreenW {
				tooltipX = protocol.ScreenW - tw - 8
			}
			if tooltipY < 4 {
				tooltipY = my + 16
			}

			// Draw tooltip background
			ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

			// Draw tooltip text
			text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.NRGBA{255, 215, 0, 255}) // Gold color for cost
		}

		// Draw tooltips for mini overlay
		if g.hoveredOverlayLevel && g.miniOverlayName != "" {
			lvl := 1
			if g.unitXP != nil {
				if xp, ok := g.unitXP[g.miniOverlayName]; ok {
					l, _, _ := computeLevel(xp)
					if l > 0 {
						lvl = l
					}
				}
			}

			tooltipText := fmt.Sprintf("Level %d", lvl)
			tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
			th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

			// Position tooltip above the level badge
			tooltipX := mx - tw/2
			tooltipY := my - th - 8

			// Keep tooltip on screen
			if tooltipX < 4 {
				tooltipX = 4
			}
			if tooltipX+tw+8 > protocol.ScreenW {
				tooltipX = protocol.ScreenW - tw - 8
			}
			if tooltipY < 4 {
				tooltipY = my + 16
			}

			// Draw tooltip background
			ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

			// Draw tooltip text
			text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.White)
		}

		if g.hoveredOverlayCost && g.miniOverlayName != "" {
			if info, ok := g.nameToMini[g.miniOverlayName]; ok {
				tooltipText := fmt.Sprintf("Deploy cost %d", info.Cost)
				tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
				th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

				// Position tooltip above the cost text
				tooltipX := mx - tw/2
				tooltipY := my - th - 8

				// Keep tooltip on screen
				if tooltipX < 4 {
					tooltipX = 4
				}
				if tooltipX+tw+8 > protocol.ScreenW {
					tooltipX = protocol.ScreenW - tw - 8
				}
				if tooltipY < 4 {
					tooltipY = my + 16
				}

				// Draw tooltip background
				ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

				// Draw tooltip text
				text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.NRGBA{255, 215, 0, 255}) // Gold color for cost
			}
		}

		if g.armyMsg != "" {
			text.Draw(screen, g.armyMsg, basicfont.Face7x13, pad, protocol.ScreenH-menuBarH-24, color.White)
		}

		// Right-click selected mini slots to open XP overlay handled in Update
		// Mini XP overlay drawing + actions
		if g.miniOverlayOpen && g.miniOverlayName != "" {
			ebitenutil.DrawRect(screen, 0, 0, float64(protocol.ScreenW), float64(protocol.ScreenH), color.NRGBA{0, 0, 0, 120})
			w, h := 580, 300
			x := (protocol.ScreenW - w) / 2
			// Position a bit lower so 2x3 slots remain visible when selecting
			const miniCardH = 116
			const gap = 10
			stripY := topBarH + pad
			stripH := 116 + 8
			topY := stripY + stripH + 12
			slotsBottom := topY + 2*(miniCardH+gap) - gap
			y := slotsBottom + 28
			if y+h > protocol.ScreenH-12 {
				y = (protocol.ScreenH - h) / 2
			}
			ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), color.NRGBA{32, 34, 48, 245})
			// close box (draw only; click handled in Update)
			closeR := rect{x: x + w - 28, y: y + 8, w: 20, h: 20}
			ebitenutil.DrawRect(screen, float64(closeR.x), float64(closeR.y), float64(closeR.w), float64(closeR.h), color.NRGBA{60, 60, 80, 255})
			text.Draw(screen, "X", basicfont.Face7x13, closeR.x+6, closeR.y+14, color.White)
			// portrait
			if img := g.ensureMiniImageByName(g.miniOverlayName); img != nil {
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
				pw, ph := 120, 140
				px := x + 16
				py := y + 36
				s := mathMin(float64(pw)/float64(iw), float64(ph)/float64(ih))
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(s, s)
				ox := float64(px) + (float64(pw)-float64(iw)*s)/2
				oy := float64(py) + (float64(ph)-float64(ih)*s)/2
				op.GeoM.Translate(ox, oy)
				screen.DrawImage(img, op)
				// Level badge over portrait top-left (bigger)
				lvl := 1
				if g.unitXP != nil {
					if v, ok := g.unitXP[g.miniOverlayName]; ok {
						if l, _, _ := computeLevel(v); l > 0 {
							lvl = l
						}
					}
				}
				drawSmallLevelBadgeSized(screen, px+0, py+0, lvl, 24)
			}
			// Unit name as title
			text.Draw(screen, g.miniOverlayName, basicfont.Face7x13, x+170, y+24, color.NRGBA{255, 215, 0, 255}) // Gold color for title
			// Stats block (right side)
			if info, ok := g.nameToMini[g.miniOverlayName]; ok {
				sy := y + 50
				startY := sy // Remember the starting Y position
				statCount := 0
				switchedToRight := false
				stat := func(label, val string, isRightColumn bool) {
					colX := x + 170
					if isRightColumn {
						colX = x + 320 // About 30 characters distance from left column
						if !switchedToRight {
							switchedToRight = true
							sy = startY // Reset to first stat's Y position for right column
						}
					}
					text.Draw(screen, label+": "+val, basicfont.Face7x13, colX, sy, color.NRGBA{220, 220, 230, 255})
					if !isRightColumn {
						statCount++
					}
					sy += 16
				}
				// Class / Cost first
				stat("Class", strings.Title(info.Class), false)
				stat("Cost", fmt.Sprintf("%d", info.Cost), false)
				// Damage / Health
				// Scale by level: 10% per level above 1
				lvlStat := 1
				if g.unitXP != nil {
					if v, ok2 := g.unitXP[g.miniOverlayName]; ok2 {
						if l, _, _ := computeLevel(v); l > 0 {
							lvlStat = l
						}
					}
				}
				scale := 1.0 + 0.10*float64(lvlStat-1)
				if info.Dmg > 0 {
					stat("Damage", fmt.Sprintf("%d", int(float64(info.Dmg)*scale)), false)
					// Calculate DPS (damage per second based on attack speed)
					attackSpeed := 1.0 // default
					if info.AttackSpeed > 0 {
						attackSpeed = info.AttackSpeed
					}
					dps := float64(info.Dmg) * scale / attackSpeed
					stat("DPS", fmt.Sprintf("%.1f", dps), false)
				}
				if info.Hp > 0 {
					stat("Health", fmt.Sprintf("%d", int(float64(info.Hp)*scale)), false)
				}
				// Healer stats
				if info.SubClass == "healer" {
					if info.Hps > 0 {
						stat("HPS", fmt.Sprintf("%d", int(float64(info.Hps)*scale)), statCount >= 7)
					}
					if info.Heal > 0 {
						stat("Heal", fmt.Sprintf("%d", int(float64(info.Heal)*scale)), statCount >= 7)
					}
				}
				// Speed scale
				if info.Speed > 0 {
					sp := map[int]string{1: "Slow", 2: "Medium", 3: "Mid-fast", 4: "Fast"}
					sv := sp[info.Speed]
					if sv == "" {
						sv = fmt.Sprintf("%d", info.Speed)
					}
					stat("Speed", sv, statCount >= 7)
				}
				// After stats, place XP bar
				// Position XP bar below the stats area, not after the last stat
				barX, barY := x+160, startY+120
				barW, barH := w-170-24, 30
				// Draw very dark yellow frame around the whole progress bar dimension
				ebitenutil.DrawRect(screen, float64(barX-2), float64(barY-2), float64(barW+4), float64(barH+4), color.NRGBA{100, 90, 0, 255})
				// Fill the entire bar with overlay background color (transparent/missing XP)
				ebitenutil.DrawRect(screen, float64(barX), float64(barY), float64(barW), float64(barH), color.NRGBA{32, 34, 48, 245})
				// Level/XP calc
				xp := 0
				if g.unitXP != nil {
					if v, ok := g.unitXP[g.miniOverlayName]; ok {
						xp = v
					}
				}
				lvl, cur, next := computeLevel(xp)
				if lvl > 20 {
					lvl = 20
				}
				frac := 1.0
				if next > 0 {
					frac = float64(cur) / float64(next)
				}
				fillW := int(float64(barW) * frac)
				// Purpleish blue fill only for the filled portion
				if fillW > 0 {
					ebitenutil.DrawRect(screen, float64(barX), float64(barY), float64(fillW), float64(barH), color.NRGBA{138, 43, 226, 200})
				}
				// XP text centered over the bar (no 'Level X' text)
				var s string
				if next > 0 {
					s = fmt.Sprintf("%d/%d", cur, next)
				} else {
					s = "max"
				}
				tw := text.BoundString(basicfont.Face7x13, s).Dx()
				tx := barX + (barW-tw)/2
				ty := barY + barH/2 + 6
				text.Draw(screen, s, basicfont.Face7x13, tx+1, ty, color.NRGBA{0, 0, 0, 200})
				text.Draw(screen, s, basicfont.Face7x13, tx-1, ty, color.NRGBA{0, 0, 0, 200})
				text.Draw(screen, s, basicfont.Face7x13, tx, ty+1, color.NRGBA{0, 0, 0, 200})
				text.Draw(screen, s, basicfont.Face7x13, tx, ty-1, color.NRGBA{0, 0, 0, 200})
				text.Draw(screen, s, basicfont.Face7x13, tx, ty, color.White)

				// XP bar hover tooltip
				if g.xpBarHovered {
					mx, my := ebiten.CursorPosition()
					// Calculate next level and XP required
					nextLevel := lvl + 1
					if nextLevel > 20 {
						nextLevel = 20
					}
					xpRequired := 0
					if next > 0 {
						xpRequired = next - cur
					}

					// Tooltip text
					tooltipText := fmt.Sprintf("XP to level %d: %d", nextLevel, xpRequired)

					// Measure text for tooltip box
					tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
					th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

					// Position tooltip above cursor with some offset
					tooltipX := mx - tw/2
					tooltipY := my - th - 8

					// Keep tooltip on screen
					if tooltipX < 4 {
						tooltipX = 4
					}
					if tooltipX+tw+8 > protocol.ScreenW {
						tooltipX = protocol.ScreenW - tw - 8
					}
					if tooltipY < 4 {
						tooltipY = my + 16
					}

					// Draw tooltip background
					ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

					// Draw tooltip text
					text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.White)
				}
			}

			// Action button (draw only; handled in Update)
			btn := func(rx, ry int, label string) rect {
				r := rect{x: rx, y: ry, w: 110, h: 26}
				ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{70, 110, 70, 255})
				text.Draw(screen, label, basicfont.Face7x13, r.x+10, r.y+18, color.White)
				return r
			}
			if g.miniOverlayFrom == "collection" {
				// Place Equip button below the portrait, centered under it
				pbx := x + 16 + (120-110)/2 // portrait width=120, button width=110
				pby := y + 36 + 140 + 10    // portrait top + height + gap
				_ = btn(pbx, pby, "Equip")
			}
			// Target selection modes
			if g.miniOverlayMode == "switch_target_slot" {
				// Highlight slots and accept click
				// Recompute slot rects
				stripY := topBarH + pad
				stripH := 116 + 8
				topY := stripY + stripH + 12
				bigW := 200
				// Center champion + slots block
				const miniCardW, miniCardH = 100, 116
				const gap = 10
				totalW := bigW + 16 + 3*miniCardW + 2*gap
				startX := (protocol.ScreenW - totalW) / 2
				leftX := startX
				rightX := leftX + bigW + 16
				k := 1
				slots := make([]rect, 6)
				for row := 0; row < 2; row++ {
					for col := 0; col < 3; col++ {
						slots[k-1] = rect{x: rightX + col*(miniCardW+gap), y: topY + row*(miniCardH+gap), w: miniCardW, h: miniCardH}
						k++
					}
				}
				// draw faint highlight
				for _, r := range slots {
					ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), 2, color.NRGBA{240, 196, 25, 200})
				}
				if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
					mx, my := ebiten.CursorPosition()
					for si, r := range slots {
						if r.hit(mx, my) {
							// place into exact slot si
							prev := g.selectedOrder[si]
							if prev != "" {
								delete(g.selectedMinis, prev)
							}
							g.selectedOrder[si] = g.miniOverlayName
							g.selectedMinis[g.miniOverlayName] = true
							g.setChampArmyFromSelected()
							g.autoSaveCurrentChampionArmy()
							g.miniOverlayOpen = false
							g.miniOverlayMode = ""
							break
						}
					}
				}
			}
		}
	case tabMap:
		disp := g.displayMapID()
		bg := g.ensureBgForMap(disp)
		offX, offY, dispW, dispH, s := g.mapRenderRect(bg)

		if bg != nil {
			c := g.mapEdgeColor(disp, bg)
			if offY > 0 {
				ebitenutil.DrawRect(screen, 0, 0, float64(protocol.ScreenW), float64(offY), c)
				ebitenutil.DrawRect(screen, 0, float64(offY+dispH),
					float64(protocol.ScreenW), float64(protocol.ScreenH-(offY+dispH)), c)
			}
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(float64(offX), float64(offY))
			// Use linear filtering for smoother scaling on high-resolution displays
			op.Filter = ebiten.FilterLinear
			screen.DrawImage(bg, op)
		}

		text.Draw(screen, "Map — click a location, then press Start",
			basicfont.Face7x13, pad, topBarH-6, color.White)

		g.ensureMapHotspots()
		hsList := g.mapHotspots[disp]
		if hsList == nil {
			hsList = g.mapHotspots[defaultMapID]
		}
		for i, hs := range hsList {
			cx := offX + int(hs.X*float64(dispW))
			cy := offY + int(hs.Y*float64(dispH))

			col := color.NRGBA{0x66, 0x99, 0xcc, 0xff}
			if i == g.hoveredHS {
				col = color.NRGBA{0xa0, 0xd0, 0xff, 0xff}
			}
			if i == g.selectedHS {
				col = color.NRGBA{240, 196, 25, 255}
			}

			ebitenutil.DrawRect(screen, float64(cx-2), float64(cy-2), 4, 4, col)
		}

		if g.hoveredHS >= 0 && g.hoveredHS < len(hsList) {
			hs := hsList[g.hoveredHS]
			mx, my := ebiten.CursorPosition()
			w, h := 260, 46
			x := clampInt(mx+14, 0, protocol.ScreenW-w)
			y := clampInt(my-8-h, 0, protocol.ScreenH-h)
			ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h),
				color.NRGBA{30, 30, 45, 240})
			text.Draw(screen, hs.Name, basicfont.Face7x13, x+8, y+18, color.White)
			if hs.Info != "" {
				text.Draw(screen, hs.Info, basicfont.Face7x13, x+8, y+34, color.NRGBA{200, 200, 200, 255})
			}
		}

		if g.selectedHS >= 0 && g.selectedHS < len(hsList) {
			hs := hsList[g.selectedHS]
			cx := offX + int(hs.X*float64(dispW))
			cy := offY + int(hs.Y*float64(dispH))
			g.startBtn = rect{x: cx + 22, y: cy - 16, w: 90, h: 28}

			btnCol := color.NRGBA{70, 110, 70, 255}
			label := "Start"
			if g.roomID == "" {
				btnCol = color.NRGBA{110, 110, 70, 255}
				label = "Start…"
			}
			ebitenutil.DrawRect(screen, float64(g.startBtn.x), float64(g.startBtn.y),
				float64(g.startBtn.w), float64(g.startBtn.h), btnCol)
			text.Draw(screen, label, basicfont.Face7x13, g.startBtn.x+18, g.startBtn.y+18, color.White)
		}
	case tabPvp:

		contentY := topBarH
		contentH := protocol.ScreenH - menuBarH - contentY

		// Draw themed background panel with enhanced styling
		if g.fantasyUI != nil {
			g.fantasyUI.DrawThemedPanel(screen, 0, contentY, protocol.ScreenW, contentH, 0.9)

			// Add subtle pattern overlay for PvP section
			g.fantasyUI.CreateBackgroundPattern(protocol.ScreenW, contentH)
			pattern := g.fantasyUI.CreateBackgroundPattern(protocol.ScreenW, contentH)
			if pattern != nil {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(0, float64(contentY))
				op.ColorM.Scale(1, 1, 1, 0.3) // Very subtle opacity
				screen.DrawImage(pattern, op)
			}
		} else {
			ebitenutil.DrawRect(screen, 0, float64(contentY), float64(protocol.ScreenW), float64(contentH), color.NRGBA{0x20, 0x20, 0x28, 0xFF})
		}

		// Add PvP section title with enhanced styling
		titleY := contentY + 20
		if g.fantasyUI != nil {
			// Draw ornate title background
			vector.DrawFilledRect(screen, float32(pad), float32(titleY-8), float32(protocol.ScreenW-2*pad), 40, g.fantasyUI.Theme.CardBackground, true)
			vector.StrokeRect(screen, float32(pad), float32(titleY-8), float32(protocol.ScreenW-2*pad), 40, 2, g.fantasyUI.Theme.Border, true)

			// Add title glow effect
			vector.StrokeRect(screen, float32(pad+2), float32(titleY-6), float32(protocol.ScreenW-2*pad-4), 36, 1, g.fantasyUI.Theme.Glow, true)

			text.Draw(screen, "⚔️ Player vs Player Arena", basicfont.Face7x13, pad+12, titleY+6, g.fantasyUI.Theme.TextPrimary)
			text.Draw(screen, "Battle other players in ranked matches", basicfont.Face7x13, pad+12, titleY+22, g.fantasyUI.Theme.TextSecondary)
		} else {
			text.Draw(screen, "⚔️ Player vs Player Arena", basicfont.Face7x13, pad+8, titleY, color.NRGBA{240, 196, 25, 255})
			text.Draw(screen, "Battle other players in ranked matches", basicfont.Face7x13, pad+8, titleY+16, color.NRGBA{200, 200, 200, 255})
		}

		queueBtn, leaveBtn, createBtn, cancelBtn, joinInput, joinBtn := g.pvpLayout()

		// Enhanced button states with hover detection
		mx, my := ebiten.CursorPosition()
		queueHovered := queueBtn.hit(mx, my)
		leaveHovered := leaveBtn.hit(mx, my)
		createHovered := createBtn.hit(mx, my)
		cancelHovered := cancelBtn.hit(mx, my)
		joinHovered := joinBtn.hit(mx, my)

		// Draw themed buttons with enhanced states and conditional visibility
		if g.fantasyUI != nil {
			// Queue PvP button - only show when not queued
			if !g.pvpQueued {
				queueState := ButtonNormal
				if queueHovered {
					queueState = ButtonHover
				}
				g.fantasyUI.DrawThemedButtonWithStyle(screen, queueBtn.x, queueBtn.y, queueBtn.w, queueBtn.h, "Queue PvP", queueState, true)
			}

			// Leave Queue button - only show when queued
			if g.pvpQueued {
				leaveState := ButtonNormal
				if leaveHovered {
					leaveState = ButtonHover
				}
				g.fantasyUI.DrawThemedButtonWithStyle(screen, leaveBtn.x, leaveBtn.y, leaveBtn.w, leaveBtn.h, "Leave Queue", leaveState, true)
			}

			// Create Friendly Code button - only show when not hosting
			if !g.pvpHosting {
				createState := ButtonNormal
				if createHovered {
					createState = ButtonHover
				}
				g.fantasyUI.DrawThemedButtonWithStyle(screen, createBtn.x, createBtn.y, createBtn.w, createBtn.h, "Create Friendly Code", createState, true)
			}

			// Cancel button - only show when hosting
			if g.pvpHosting {
				cancelState := ButtonNormal
				if cancelHovered {
					cancelState = ButtonHover
				}
				g.fantasyUI.DrawThemedButtonWithStyle(screen, cancelBtn.x, cancelBtn.y, cancelBtn.w, cancelBtn.h, "Cancel Friendly", cancelState, true)
			}

			// Join button - always visible
			joinState := ButtonNormal
			if joinHovered {
				joinState = ButtonHover
			}
			g.fantasyUI.DrawThemedButtonWithStyle(screen, joinBtn.x, joinBtn.y, joinBtn.w, joinBtn.h, "Join", joinState, true)
		} else {
			// Fallback to basic buttons
			ebitenutil.DrawRect(screen, float64(queueBtn.x), float64(queueBtn.y), float64(queueBtn.w), float64(queueBtn.h),
				map[bool]color.NRGBA{true: {80, 110, 80, 255}, false: {60, 60, 80, 255}}[g.pvpQueued])
			text.Draw(screen, "Queue PvP", basicfont.Face7x13, queueBtn.x+16, queueBtn.y+18, color.White)

			ebitenutil.DrawRect(screen, float64(leaveBtn.x), float64(leaveBtn.y), float64(leaveBtn.w), float64(leaveBtn.h),
				color.NRGBA{90, 70, 70, 255})
			text.Draw(screen, "Leave Queue", basicfont.Face7x13, leaveBtn.x+16, leaveBtn.y+18, color.White)

			ebitenutil.DrawRect(screen, float64(createBtn.x), float64(createBtn.y), float64(createBtn.w), float64(createBtn.h),
				map[bool]color.NRGBA{true: {80, 110, 80, 255}, false: {60, 60, 80, 255}}[g.pvpHosting])
			text.Draw(screen, "Create Friendly Code", basicfont.Face7x13, createBtn.x+16, createBtn.y+18, color.White)

			ebitenutil.DrawRect(screen, float64(cancelBtn.x), float64(cancelBtn.y), float64(cancelBtn.w), float64(cancelBtn.h),
				color.NRGBA{90, 70, 70, 255})
			text.Draw(screen, "Cancel", basicfont.Face7x13, cancelBtn.x+16, cancelBtn.y+18, color.White)
		}

		g.pvpCodeArea = rect{}
		if g.pvpHosting && g.pvpCode != "" {
			msg := "Your code: " + g.pvpCode

			lb := text.BoundString(basicfont.Face7x13, msg)
			bx := createBtn.x
			by := createBtn.y + createBtn.h + 12
			bw := lb.Dx() + 18
			bh := 26

			g.pvpCodeArea = rect{x: bx, y: by, w: bw, h: bh}

			// Draw themed code display area
			if g.fantasyUI != nil {
				// Draw the card background and border
				vector.DrawFilledRect(screen, float32(bx), float32(by), float32(bw), float32(bh), g.fantasyUI.Theme.CardBackground, true)
				vector.StrokeRect(screen, float32(bx), float32(by), float32(bw), float32(bh), 3, g.fantasyUI.Theme.Border, true)
				vector.StrokeRect(screen, float32(bx+3), float32(by+3), float32(bw-6), float32(bh-6), 1, g.fantasyUI.Theme.Glow, true)

				// Draw the code text centered in the card
				text.Draw(screen, msg, basicfont.Face7x13, bx+9, by+18, g.fantasyUI.Theme.TextPrimary)
			} else {
				ebitenutil.DrawRect(screen, float64(bx), float64(by), float64(bw), float64(bh), color.NRGBA{54, 63, 88, 255})
				text.Draw(screen, msg, basicfont.Face7x13, bx+9, by+18, color.White)
			}

			hintX := bx + bw + 12
			hintY := by + (bh+13)/2 - 2
			text.Draw(screen, "Click to copy", basicfont.Face7x13, hintX, hintY, color.NRGBA{160, 160, 170, 255})
		}

		// Enhanced input field with themed styling
		if g.fantasyUI != nil {
			// Draw themed input field background - use darker colors for better text visibility
			inputColor := color.NRGBA{45, 45, 60, 255} // Darker background
			if g.pvpInputActive {
				inputColor = color.NRGBA{55, 55, 75, 255} // Slightly lighter when active
			}
			vector.DrawFilledRect(screen, float32(joinInput.x), float32(joinInput.y),
				float32(joinInput.w), float32(joinInput.h), inputColor, true)
			vector.StrokeRect(screen, float32(joinInput.x), float32(joinInput.y),
				float32(joinInput.w), float32(joinInput.h), 2, g.fantasyUI.Theme.Border, true)
		} else {
			ebitenutil.DrawRect(screen, float64(joinInput.x), float64(joinInput.y), float64(joinInput.w), float64(joinInput.h),
				color.NRGBA{38, 38, 53, 255})
		}

		label := g.pvpCodeInput
		if label == "" && !g.pvpInputActive {
			label = "Enter code..."
			text.Draw(screen, label, basicfont.Face7x13, joinInput.x+8, joinInput.y+17, color.NRGBA{150, 150, 160, 255})
		} else {
			text.Draw(screen, label, basicfont.Face7x13, joinInput.x+8, joinInput.y+17, color.White)
		}

		bottomY := joinBtn.y + joinBtn.h
		sepY := bottomY + 20

		// Draw themed separator
		if g.fantasyUI != nil {
			vector.DrawFilledRect(screen, float32(pad), float32(sepY), float32(protocol.ScreenW-2*pad), 2, g.fantasyUI.Theme.Border, true)
		} else {
			ebitenutil.DrawRect(screen, float64(pad), float64(sepY), float64(protocol.ScreenW-2*pad), 1, color.NRGBA{90, 90, 120, 255})
		}

		// Status panel with themed styling
		panelY := sepY + 14
		panelH := 54

		if g.fantasyUI != nil {
			// Draw status panel manually to avoid the underline that passes through the text
			vector.DrawFilledRect(screen, float32(pad), float32(panelY), float32(protocol.ScreenW-2*pad), float32(panelH), g.fantasyUI.Theme.CardBackground, true)
			vector.StrokeRect(screen, float32(pad), float32(panelY), float32(protocol.ScreenW-2*pad), float32(panelH), 3, g.fantasyUI.Theme.Border, true)
			vector.StrokeRect(screen, float32(pad+3), float32(panelY+3), float32(protocol.ScreenW-2*pad-6), float32(panelH-6), 1, g.fantasyUI.Theme.Glow, true)

			// Draw title and content manually without the underline
			text.Draw(screen, "Status", basicfont.Face7x13, pad+12, panelY+18, g.fantasyUI.Theme.TextPrimary)

			msg := g.pvpStatus
			if msg == "" {
				msg = "—"
			}
			text.Draw(screen, msg, basicfont.Face7x13, pad+12, panelY+36, g.fantasyUI.Theme.TextSecondary)
		} else {
			ebitenutil.DrawRect(screen, float64(pad), float64(panelY), float64(protocol.ScreenW-2*pad), float64(panelH), color.NRGBA{0x24, 0x24, 0x30, 0xFF})
			text.Draw(screen, "Status", basicfont.Face7x13, pad+8, panelY+18, color.NRGBA{240, 196, 25, 255})

			msg := g.pvpStatus
			if msg == "" {
				msg = "—"
			}
			text.Draw(screen, msg, basicfont.Face7x13, pad+8, panelY+36, color.White)
		}

		// Leaderboard panel with enhanced styling
		panelPad := pad
		rows := minInt(50, len(g.pvpLeaders))
		const rowH = 16
		leaderboardPanelH := 16 + 16 + rows*rowH + 8
		if leaderboardPanelH < 120 {
			leaderboardPanelH = 120
		}

		leaderboardPanelTop := protocol.ScreenH - menuBarH - leaderboardPanelH - 8
		if leaderboardPanelTop < topBarH+180 {
			leaderboardPanelTop = topBarH + 180
		}

		// Draw themed leaderboard panel
		if g.fantasyUI != nil {
			g.fantasyUI.DrawThemedCard(screen, panelPad, leaderboardPanelTop,
				protocol.ScreenW-2*panelPad, leaderboardPanelH, "Top 50 - PvP Leaderboard", []string{})

			// Add timestamp if available
			if g.lbLastStamp != 0 {
				ts := time.UnixMilli(g.lbLastStamp).Format("15:04:05")
				text.Draw(screen, "as of "+ts, basicfont.Face7x13, panelPad+240, leaderboardPanelTop+18, color.NRGBA{170, 170, 180, 255})
			}
		} else {
			ebitenutil.DrawRect(screen, float64(panelPad), float64(leaderboardPanelTop),
				float64(protocol.ScreenW-2*panelPad), float64(leaderboardPanelH), color.NRGBA{0x24, 0x24, 0x30, 0xFF})

			text.Draw(screen, "Top 50 - PvP Leaderboard", basicfont.Face7x13, panelPad+8, leaderboardPanelTop+18, color.White)
			if g.lbLastStamp != 0 {
				ts := time.UnixMilli(g.lbLastStamp).Format("15:04:05")
				text.Draw(screen, "as of "+ts, basicfont.Face7x13, panelPad+240, leaderboardPanelTop+18, color.NRGBA{170, 170, 180, 255})
			}
		}

		colRankX := panelPad + 8
		colNameX := panelPad + 58
		colRatX := panelPad + 360
		colTierX := panelPad + 440

		hdrY := leaderboardPanelTop + 36
		text.Draw(screen, "#", basicfont.Face7x13, colRankX, hdrY, color.NRGBA{200, 200, 210, 255})
		text.Draw(screen, "Player", basicfont.Face7x13, colNameX, hdrY, color.NRGBA{200, 200, 210, 255})
		text.Draw(screen, "Rating", basicfont.Face7x13, colRatX, hdrY, color.NRGBA{200, 200, 210, 255})
		text.Draw(screen, "Rank", basicfont.Face7x13, colTierX, hdrY, color.NRGBA{200, 200, 210, 255})

		rowY := hdrY + 16
		maxRows := minInt(50, len(g.pvpLeaders))
		for i := 0; i < maxRows; i++ {
			e := g.pvpLeaders[i]
			y := rowY + i*rowH

			// Alternate row colors with theme integration
			if i%2 == 0 {
				if g.fantasyUI != nil {
					vector.DrawFilledRect(screen, float32(panelPad+4), float32(y-12),
						float32(protocol.ScreenW-2*panelPad-8), float32(rowH), g.fantasyUI.Theme.Surface, true)
				} else {
					ebitenutil.DrawRect(screen, float64(panelPad+4), float64(y-12),
						float64(protocol.ScreenW-2*panelPad-8), rowH, color.NRGBA{0x28, 0x28, 0x36, 0xFF})
				}
			}

			text.Draw(screen, fmt.Sprintf("%2d.", i+1), basicfont.Face7x13, colRankX, y, color.White)
			text.Draw(screen, trim(e.Name, 22), basicfont.Face7x13, colNameX, y, color.White)
			text.Draw(screen, fmt.Sprintf("%d", e.Rating), basicfont.Face7x13, colRatX, y, color.White)
			text.Draw(screen, e.Rank, basicfont.Face7x13, colTierX, y, color.NRGBA{240, 196, 25, 255})
		}

	case tabSocial:
		g.drawSocial(screen)
	case tabSettings:
		// Settings panel background
		contentY := topBarH + pad
		contentH := protocol.ScreenH - menuBarH - contentY
		ebitenutil.DrawRect(screen, float64(pad), float64(contentY), float64(protocol.ScreenW-2*pad), float64(contentH), color.NRGBA{0x20, 0x20, 0x28, 0xFF})

		// Title
		y := contentY + 20
		text.Draw(screen, "⚙️ Settings", basicfont.Face7x13, pad+8, y, color.NRGBA{240, 196, 25, 255})

		// Display Settings Section
		y += 40
		text.Draw(screen, "Display Settings", basicfont.Face7x13, pad+8, y, color.NRGBA{200, 200, 210, 255})

		y += 25
		text.Draw(screen, "Fullscreen:", basicfont.Face7x13, pad+16, y, color.White)

		g.fsOnBtn = rect{x: pad + 140, y: y - 14, w: 80, h: 20}
		g.fsOffBtn = rect{x: g.fsOnBtn.x + 90, y: y - 14, w: 80, h: 20}

		onCol := color.NRGBA{70, 110, 70, 255}
		offCol := color.NRGBA{110, 70, 70, 255}
		neutral := color.NRGBA{60, 60, 80, 255}

		ebitenutil.DrawRect(screen, float64(g.fsOnBtn.x), float64(g.fsOnBtn.y), float64(g.fsOnBtn.w), float64(g.fsOnBtn.h),
			map[bool]color.NRGBA{true: onCol, false: neutral}[g.fullscreen])
		text.Draw(screen, "ON", basicfont.Face7x13, g.fsOnBtn.x+26, g.fsOnBtn.y+14, color.White)

		ebitenutil.DrawRect(screen, float64(g.fsOffBtn.x), float64(g.fsOffBtn.y), float64(g.fsOffBtn.w), float64(g.fsOffBtn.h),
			map[bool]color.NRGBA{true: neutral, false: offCol}[g.fullscreen])
		text.Draw(screen, "OFF", basicfont.Face7x13, g.fsOffBtn.x+24, g.fsOffBtn.y+14, color.White)

		// Game Settings Section
		y += 50
		text.Draw(screen, "Game Settings", basicfont.Face7x13, pad+8, y, color.NRGBA{200, 200, 210, 255})

		y += 25
		text.Draw(screen, "Auto-save Army:", basicfont.Face7x13, pad+16, y, color.White)

		// Auto-save toggle (for now, just show current state)
		autoSaveStatus := "Enabled"
		if !g.autoSaveEnabled {
			autoSaveStatus = "Disabled"
		}
		text.Draw(screen, autoSaveStatus, basicfont.Face7x13, pad+140, y, color.NRGBA{180, 180, 190, 255})

		// Account Settings Section
		y += 50
		text.Draw(screen, "Account Settings", basicfont.Face7x13, pad+8, y, color.NRGBA{200, 200, 210, 255})

		y += 25
		text.Draw(screen, "Player:", basicfont.Face7x13, pad+16, y, color.White)
		text.Draw(screen, g.name, basicfont.Face7x13, pad+140, y, color.NRGBA{240, 196, 25, 255})

		y += 20
		text.Draw(screen, "Gold:", basicfont.Face7x13, pad+16, y, color.White)
		goldStr := fmt.Sprintf("%d", g.accountGold)
		text.Draw(screen, goldStr, basicfont.Face7x13, pad+140, y, color.NRGBA{255, 215, 0, 255})

		// Logout button
		y += 30
		g.logoutBtn = rect{x: pad + 16, y: y - 6, w: 100, h: 24}
		ebitenutil.DrawRect(screen, float64(g.logoutBtn.x), float64(g.logoutBtn.y), float64(g.logoutBtn.w), float64(g.logoutBtn.h), color.NRGBA{110, 70, 70, 255})
		text.Draw(screen, "Logout", basicfont.Face7x13, g.logoutBtn.x+20, g.logoutBtn.y+16, color.White)

		// Controls Section
		y += 50
		text.Draw(screen, "Controls", basicfont.Face7x13, pad+8, y, color.NRGBA{200, 200, 210, 255})

		y += 18
		text.Draw(screen, "Alt+F - Toggle Fullscreen", basicfont.Face7x13, pad+16, y, color.NRGBA{180, 180, 190, 255})
		y += 18
		text.Draw(screen, "Mouse - Navigate & Select", basicfont.Face7x13, pad+16, y, color.NRGBA{180, 180, 190, 255})
		y += 18
		text.Draw(screen, "Touch - Mobile Controls", basicfont.Face7x13, pad+16, y, color.NRGBA{180, 180, 190, 255})

		// Version/Info
		y = protocol.ScreenH - menuBarH - 40
		text.Draw(screen, protocol.GameName+" v"+protocol.GameVersion, basicfont.Face7x13, pad+8, y, color.NRGBA{150, 150, 160, 255})
		text.Draw(screen, "by S. Robev", basicfont.Face7x13, pad+8, y+16, color.NRGBA{120, 120, 130, 255})
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (g *Game) createRoomFor(mapID string) {
	g.pendingArena = mapID
	g.send("CreatePve", protocol.CreatePve{MapID: mapID})
}

func defaultIfEmpty(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// call every frame with latest hp + max
func (b *battleHPBar) Set(cur, max int) {
	if max <= 0 {
		max = 1
	}
	if b.maxHP != max {
		b.maxHP = max
		if b.displayHP > max {
			b.displayHP = max
		}
	}
	if cur < 0 {
		cur = 0
	}
	if cur > max {
		cur = max
	}

	if cur < b.targetHP {
		b.flashTicks = 22

	}

	if cur > b.displayHP {
		b.displayHP = cur
	}

	b.targetHP = cur
}

// Returns the image to show as the enemy "avatar" — base for now, boss later.
func (g *Game) enemyTargetAvatarImage() *ebiten.Image {
	if g.enemyTargetThumb != nil {
		return g.enemyTargetThumb
	}

	if g.enemyBossPortrait != "" {
		if img := g.ensureAvatarImage(g.enemyBossPortrait); img != nil {
			g.enemyTargetThumb = img
			return img
		}
	}

	tryPaths := []string{
		"assets/ui/base.png",
		"assets/ui/base_avatar.png",
	}
	for _, p := range tryPaths {
		if img := loadImage(p); img != nil {
			g.enemyTargetThumb = img
			return img
		}
	}

	if img := g.ensureAvatarImage("default.png"); img != nil {
		g.enemyTargetThumb = img
		return img
	}
	return nil
}
