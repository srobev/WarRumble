package game

import (
	"fmt"
	"log"
	"rumble/shared/protocol"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

// ---------- Home Screen Input Handling ----------

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
		g.updateArmyTab(mx, my)
	case tabMap:
		g.updateMapTab(mx, my)
	case tabPvp:
		g.updatePvpTab(mx, my)
	case tabSocial:
		g.updateSocial()
	case tabSettings:
		g.updateSettingsTab(mx, my)
	}
}

// Army tab input handling
func (g *Game) updateArmyTab(mx, my int) {
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
	g.hoveredSelectedChampionXP = false
	g.hoveredMiniSlotLevel = -1
	g.hoveredMiniSlotCost = -1
	g.hoveredMiniSlotCard = -1
	g.hoveredMiniSlotXP = -1
	g.hoveredCollectionLevel = -1
	g.hoveredCollectionCost = -1
	g.hoveredCollectionCard = -1
	g.hoveredCollectionXP = -1
	g.hoveredOverlayLevel = false
	g.hoveredOverlayCost = false
	g.xpBarHovered = false

	// Check hover for champion strip (top selection area)
	for i, r := range g.champStripRects {
		if r.hit(mx, my) {
			idx := start + i
			if idx >= 0 && idx < len(g.champions) {
				it := g.champions[idx]

				// Check level badge area (bottom-left corner)
				levelBadgeRect := rect{x: r.x + 8, y: r.y + r.h - 28, w: 20, h: 20}
				if levelBadgeRect.hit(mx, my) {
					g.hoveredChampionLevel = idx
				}

				// Check cost text area (top-right corner)
				costS := fmt.Sprintf("%d", it.Cost)
				cw := text.BoundString(basicfont.Face7x13, costS).Dx()
				costRect := rect{x: r.x + r.w - cw - 8, y: r.y + 8, w: cw, h: 14}
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
			levelBadgeRect := rect{x: chRect.x + 8, y: chRect.y + chRect.h - 28, w: 20, h: 20}
			if levelBadgeRect.hit(mx, my) {
				g.hoveredSelectedChampionLevel = true
			}

			// Check cost text area (top-right)
			// Get champion cost for hover detection
			hoverChampionCost := 0
			for _, champ := range g.champions {
				if champ.Name == g.selectedChampion {
					hoverChampionCost = champ.Cost
					break
				}
			}
			costStr := fmt.Sprintf("%d", hoverChampionCost)
			costW := len(costStr) * 7
			costRect := rect{x: chRect.x + chRect.w - costW - 8, y: chRect.y + 8, w: costW, h: 14}
			if costRect.hit(mx, my) {
				g.hoveredSelectedChampionCost = true
			}

			// Check XP bar hover for selected champion
			championXPBarRect := rect{x: chRect.x + 36, y: chRect.y + chRect.h - 18, w: chRect.w - 48, h: 8}
			if championXPBarRect.hit(mx, my) {
				g.hoveredSelectedChampionXP = true
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

			// Check level badge area (bottom-left)
			levelBadgeRect := rect{x: slotRect.x + 8, y: slotRect.y + slotRect.h - 28, w: 20, h: 20}
			if levelBadgeRect.hit(mx, my) {
				g.hoveredMiniSlotLevel = i - 1
			}

			// Check cost text area (top-right)
			if info, ok := g.nameToMini[name]; ok {
				costS := fmt.Sprintf("%d", info.Cost)
				cw := text.BoundString(basicfont.Face7x13, costS).Dx()
				costRect := rect{x: slotRect.x + slotRect.w - cw - 8, y: slotRect.y + 8, w: cw, h: 14}
				if costRect.hit(mx, my) {
					g.hoveredMiniSlotCost = i - 1
				}
			}

			// Check XP bar hover
			xpBarRect := rect{x: slotRect.x + 36, y: slotRect.y + slotRect.h - 18, w: slotRect.w - 48, h: 8}
			if xpBarRect.hit(mx, my) {
				g.hoveredMiniSlotXP = i - 1
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

	// Build current visible items list to match drawing order
	avail := make([]protocol.MiniInfo, 0, len(g.minisOnly))
	for _, mi := range g.minisOnly {
		if !g.selectedMinis[mi.Name] {
			avail = append(avail, mi)
		}
	}

	// Check hover for collection XP bars
	for i, r := range g.collRects {
		if r.hit(mx, my) {
			idx := start2 + i
			if idx >= 0 && idx < len(avail) {
				// Check XP bar hover for collection items
				xpBarRect := rect{x: r.x + 36, y: r.y + r.h - 18, w: r.w - 48, h: 8}
				if xpBarRect.hit(mx, my) {
					g.hoveredCollectionXP = idx
				}
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
}

// Map tab input handling
func (g *Game) updateMapTab(mx, my int) {
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
}

// PvP tab input handling
func (g *Game) updatePvpTab(mx, my int) {
	queueBtn, leaveBtn, createBtn, cancelBtn, joinInput, joinBtn := g.pvpLayout()

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
}

// Settings tab input handling
func (g *Game) updateSettingsTab(mx, my int) {
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
