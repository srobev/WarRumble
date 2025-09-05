package game

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"rumble/shared/protocol"
	"time"

	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

type hpFx struct {
	lastHP      int
	ghostHP     int   // where the yellow chip currently ends (>= current HP)
	holdUntilMs int64 // time to keep the yellow chip still
	lerpStartMs int64 // when the chip starts animating
	lerpStartHP int   // chip start HP when anim begins
	lerpDurMs   int64 // animation duration (ms)

	// Healing flash system
	healGhostHP     int   // where the green healing flash starts (<= current HP)
	healHoldUntilMs int64 // time to keep the green flash still
	healLerpStartMs int64 // when healing flash starts animating
	healLerpStartHP int   // healing flash start HP
	healLerpDurMs   int64 // healing flash animation duration
}

func (g *Game) updateBattle() {
	// Handle camera controls for battle map scrolling and zooming (always active)
	if g.scr == screenBattle {
		// Zoom disabled - battles maintain fixed 20% zoom level

		// Pan with middle mouse button only (avoid conflict with right-click deployment)
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) {
			if !g.cameraDragging {
				g.cameraDragging = true
				g.cameraDragStartX, g.cameraDragStartY = ebiten.CursorPosition()
				g.cameraDragInitialX, g.cameraDragInitialY = g.cameraX, g.cameraY
			} else {
				cx, cy := ebiten.CursorPosition()
				deltaX := cx - g.cameraDragStartX
				deltaY := cy - g.cameraDragStartY
				newCameraX := g.cameraDragInitialX + float64(deltaX)
				newCameraY := g.cameraDragInitialY + float64(deltaY)

				// Apply same 20% boundary limits as edge scrolling
				mapWidth := float64(protocol.ScreenW) * g.cameraZoom
				mapHeight := float64(protocol.ScreenH) * g.cameraZoom
				maxScrollX := mapWidth * 0.2  // 20% outside left/right borders
				maxScrollY := mapHeight * 0.2 // 20% outside top/bottom borders

				// Clamp camera position within boundaries
				if newCameraX > maxScrollX {
					newCameraX = maxScrollX
				} else if newCameraX < -maxScrollX {
					newCameraX = -maxScrollX
				}
				if newCameraY > maxScrollY {
					newCameraY = maxScrollY
				} else if newCameraY < -maxScrollY {
					newCameraY = -maxScrollY
				}

				g.cameraX = newCameraX
				g.cameraY = newCameraY
			}
		} else {
			g.cameraDragging = false
		}

		// Pan with left mouse button when no mini selected (drag to scroll)
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && g.selectedIdx == -1 && !g.dragActive {
			if !g.cameraLeftDragging {
				g.cameraLeftDragging = true
				g.cameraLeftDragStartX, g.cameraLeftDragStartY = ebiten.CursorPosition()
				g.cameraLeftDragInitialX, g.cameraLeftDragInitialY = g.cameraX, g.cameraY
			} else {
				cx, cy := ebiten.CursorPosition()
				deltaX := cx - g.cameraLeftDragStartX
				deltaY := cy - g.cameraLeftDragStartY
				newCameraX := g.cameraLeftDragInitialX + float64(deltaX)
				newCameraY := g.cameraLeftDragInitialY + float64(deltaY)

				// Apply same 20% boundary limits as edge scrolling
				mapWidth := float64(protocol.ScreenW) * g.cameraZoom
				mapHeight := float64(protocol.ScreenH) * g.cameraZoom
				maxScrollX := mapWidth * 0.2  // 20% outside left/right borders
				maxScrollY := mapHeight * 0.2 // 20% outside top/bottom borders

				// Clamp camera position within boundaries
				if newCameraX > maxScrollX {
					newCameraX = maxScrollX
				} else if newCameraX < -maxScrollX {
					newCameraX = -maxScrollX
				}
				if newCameraY > maxScrollY {
					newCameraY = maxScrollY
				} else if newCameraY < -maxScrollY {
					newCameraY = -maxScrollY
				}

				g.cameraX = newCameraX
				g.cameraY = newCameraY
			}
		} else {
			g.cameraLeftDragging = false
		}

		// Edge scrolling when zoomed in and no unit selected
		if g.cameraZoom > 1.0 && g.selectedIdx == -1 && !g.dragActive && !g.cameraDragging && !g.cameraLeftDragging {
			mx, my := ebiten.CursorPosition()
			const edgeThreshold = 50 // pixels from edge to trigger scrolling
			const scrollSpeed = 8.0  // pixels per frame

			// Calculate map boundaries (stop scrolling when we would see more than 20% outside)
			mapWidth := float64(protocol.ScreenW) * g.cameraZoom
			mapHeight := float64(protocol.ScreenH) * g.cameraZoom
			maxScrollX := mapWidth * 0.2  // 20% outside left/right borders
			maxScrollY := mapHeight * 0.2 // 20% outside top/bottom borders

			// Left edge - scroll right to see more left side (stop before exceeding 20%)
			if mx < edgeThreshold && g.cameraX < maxScrollX {
				g.cameraX += scrollSpeed
			}
			// Right edge - scroll left to see more right side (stop before exceeding 20%)
			if mx > protocol.ScreenW-edgeThreshold && g.cameraX > -maxScrollX {
				g.cameraX -= scrollSpeed
			}
			// Top edge - scroll down to see more top side (stop before exceeding 20%)
			if my < edgeThreshold && g.cameraY < maxScrollY {
				g.cameraY += scrollSpeed
			}
			// Bottom edge (accounting for UI) - scroll up to see more bottom side (stop before exceeding 20%)
			if my > protocol.ScreenH-battleHUDH-edgeThreshold && g.cameraY > -maxScrollY {
				g.cameraY -= scrollSpeed
			}
		}
	}

	// Skip deployment and other battle updates if game is paused
	if g.timerPaused {
		return
	}

	if g.endActive {

		w, h := 360, 160
		x := (protocol.ScreenW - w) / 2
		y := (protocol.ScreenH - h) / 2
		g.continueBtn = rect{x: x + (w-120)/2, y: y + h - 50, w: 120, h: 32}

		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			mx, my := ebiten.CursorPosition()
			if g.continueBtn.hit(mx, my) {
				g.onLeaveRoom()
				g.roomID = ""
				g.scr = screenHome

				g.endActive = false
				g.endVictory = false
				g.gameOver = false
				g.victory = false
				g.hand = nil
				g.next = protocol.MiniCardView{}
				g.selectedIdx = -1
				g.dragActive = false
				g.world = &World{
					Units: make(map[int64]*RenderUnit),
					Bases: make(map[int64]protocol.BaseState),
				}

				// Populate obstacles and lanes if available
				if g.currentMapDef != nil {
					g.world.Obstacles = g.currentMapDef.Obstacles
					g.world.Lanes = g.currentMapDef.Lanes
				}
			}
		}
		return
	}

	mx, my := ebiten.CursorPosition()
	handTop := protocol.ScreenH - battleHUDH

	// Use raw mouse coordinates for deployment (server handles logical positioning)
	deployX, deployY := float64(mx), float64(my)

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Check if clicking on a hand card
		for i, r := range g.handRects() {
			if r.hit(mx, my) {
				// If clicking on already selected card, deselect it
				if g.selectedIdx == i && !g.dragActive {
					g.selectedIdx = -1
					return
				}
				g.selectedIdx = i
				g.dragActive = true
				g.dragIdx = i
				g.dragStartX, g.dragStartY = mx, my
				return
			}
		}

		// If clicking outside hand area and a card is selected, deploy it
		if my < handTop && g.selectedIdx >= 0 && g.selectedIdx < len(g.hand) {
			// Check if deployment is within a valid deploy zone
			if g.isInDeployZone(deployX, deployY) {
				g.send("DeployMiniAt", protocol.DeployMiniAt{
					CardIndex: g.selectedIdx,
					X:         deployX,
					Y:         deployY,
					ClientTs:  time.Now().UnixMilli(),
				})
				// Successfully deployed, deselect the unit
				g.selectedIdx = -1
			}
			return
		}

		// If clicking outside hand area with no card selected, deselect
		if my < handTop {
			g.selectedIdx = -1
		}
	}

	if g.dragActive && inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		if my < handTop && g.dragIdx >= 0 && g.dragIdx < len(g.hand) {
			// Check if deployment is within a valid deploy zone
			if g.isInDeployZone(deployX, deployY) {
				g.send("DeployMiniAt", protocol.DeployMiniAt{
					CardIndex: g.dragIdx,
					X:         deployX,
					Y:         deployY,
					ClientTs:  time.Now().UnixMilli(),
				})
				// Successfully deployed, deselect the unit
				g.selectedIdx = -1
			}
			// If deployment failed (invalid zone), still deselect the unit
		}
		g.dragActive = false
		g.dragIdx = -1
		return
	}
}

func (g *Game) drawBattleBar(screen *ebiten.Image) {
	y := protocol.ScreenH - battleHUDH

	ebitenutil.DrawRect(screen, 0, float64(y), float64(protocol.ScreenW), float64(battleHUDH), color.NRGBA{0x1e, 0x1e, 0x29, 0xff})

	g.assets.ensureInit()
	cx := 16
	cy := y + 20
	for i := 0; i < protocol.GoldMax; i++ {
		img := g.assets.coinEmpty
		if i < g.gold {
			img = g.assets.coinFull
		}
		if img != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(cx), float64(cy))
			screen.DrawImage(img, op)
		} else {
			ebitenutil.DrawRect(screen, float64(cx), float64(cy), 20, 20, color.NRGBA{80, 80, 80, 255})
			if i < g.gold {
				ebitenutil.DrawRect(screen, float64(cx+2), float64(cy+2), 16, 16, color.NRGBA{240, 196, 25, 255})
			}
		}
		cx += 24
	}
	text.Draw(screen, fmt.Sprintf("%d/%d", g.gold, protocol.GoldMax), basicfont.Face7x13, cx+8, cy+15, color.NRGBA{239, 229, 182, 255})

	slots := g.handRects()
	for i, r := range slots {
		ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{0x2b, 0x2b, 0x3e, 0xff})
		if i == g.selectedIdx {
			ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), 2, color.NRGBA{240, 196, 25, 255})
		}
		if i < len(g.hand) {
			c := g.hand[i]
			if img := g.ensureMiniImageByName(c.Name); img != nil {
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
				pw := float64(r.w - 8)
				ph := float64(r.h - 24)
				s := mathMin(pw/float64(iw), ph/float64(ih))
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(s, s)
				op.GeoM.Translate(float64(r.x)+4, float64(r.y)+4)
				screen.DrawImage(img, op)
			}
			if c.Cost > g.gold {
				ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{0, 0, 0, 90})
			}
			text.Draw(screen, fmt.Sprintf("%d", c.Cost), basicfont.Face7x13, r.x+6, r.y+14, color.NRGBA{240, 196, 25, 255})
			text.Draw(screen, trim(c.Name, 14), basicfont.Face7x13, r.x+6, r.y+r.h-6, color.NRGBA{239, 229, 182, 255})
		}
	}

	nextW := 70
	nextX := protocol.ScreenW - nextW - 16
	nextY := protocol.ScreenH - battleHUDH + 60
	nextR := rect{x: nextX, y: nextY, w: nextW, h: 116}
	text.Draw(screen, "Next", basicfont.Face7x13, nextX, nextY-6, color.NRGBA{140, 138, 163, 255})
	ebitenutil.DrawRect(screen, float64(nextR.x), float64(nextR.y), float64(nextR.w), float64(nextR.h), color.NRGBA{0x2b, 0x2b, 0x3e, 0xff})
	ebitenutil.DrawRect(screen, float64(nextR.x), float64(nextR.y), float64(nextR.w), float64(nextR.h), color.NRGBA{0, 0, 0, 70})
	if g.next.Name != "" {
		if img := g.ensureMiniImageByName(g.next.Name); img != nil {
			iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
			s := mathMin(float64(nextR.w-8)/float64(iw), float64(116-24)/float64(ih))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(float64(nextR.x)+4, float64(nextR.y)+4)
			screen.DrawImage(img, op)
		} else {
			text.Draw(screen, g.next.Name, basicfont.Face7x13, nextR.x+6, nextR.y+30, color.White)
		}
	}

	if g.dragActive && g.dragIdx >= 0 && g.dragIdx < len(g.hand) {
		mx, my := ebiten.CursorPosition()

		// Drag preview should show at EXACT mouse position (no mirroring)
		previewX, previewY := float64(mx), float64(my)

		if img := g.ensureMiniImageByName(g.hand[g.dragIdx].Name); img != nil {
			iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
			// Use same scale as spawn animation start (1.8x) so drag preview matches falling animation
			s := 1.8 * mathMin(1, 48.0/float64(maxInt(iw, ih)))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(previewX-float64(iw)*s/2, previewY-float64(ih)*s/2)
			op.ColorScale.Scale(1, 1, 1, 0.75)
			screen.DrawImage(img, op)
		} else {
			ebitenutil.DrawRect(screen, previewX-12, previewY-12, 24, 24, color.NRGBA{220, 220, 220, 200})
		}
	}
}

// isInDeployZone checks if a given point (x, y) is within any deploy zone
func (g *Game) isInDeployZone(x, y float64) bool {
	if g.currentMapDef == nil {
		return true // Allow deployment anywhere if no map definition
	}

	// Check if PvP mirroring is active
	shouldMirror := g.shouldMirrorForPvp()

	// Convert screen coordinates to world coordinates (inverse camera transformation)
	// World coordinates = (screen coordinates - camera offset) / camera zoom
	worldX := (x - g.cameraX) / g.cameraZoom
	worldY := (y - g.cameraY) / g.cameraZoom

	// If mirroring is active, apply inverse mirroring to the world coordinates
	if shouldMirror {
		// Apply inverse mirroring to the world coordinates
		worldY = float64(protocol.ScreenH) - worldY
	}

	// Convert world coordinates to normalized coordinates (0-1)
	normX := worldX / float64(protocol.ScreenW)
	normY := worldY / float64(protocol.ScreenH)

	// Check if point is within any deploy zone
	for _, zone := range g.currentMapDef.DeployZones {
		if normX >= zone.X && normX <= zone.X+zone.W &&
			normY >= zone.Y && normY <= zone.Y+zone.H {
			return true
		}
	}

	return false
}

func (g *Game) handRects() []rect {

	cardW, cardH := 100, 116
	gap := 10
	rowW := 4*cardW + 3*gap
	rowX := (protocol.ScreenW - rowW) / 2
	rowY := protocol.ScreenH - battleHUDH + 60

	n := len(g.hand)
	if n > 4 {
		n = 4
	}

	out := make([]rect, n)
	for i := 0; i < n; i++ {
		cx := rowX + i*(cardW+gap)
		cy := rowY
		out[i] = rect{x: cx, y: cy, w: cardW, h: cardH}
	}
	return out
}

func safeName(hand []protocol.MiniCardView, i int) string {
	if i < 0 || i >= len(hand) {
		return ""
	}
	return hand[i].Name
}
func safeCost(hand []protocol.MiniCardView, i int) int {
	if i < 0 || i >= len(hand) {
		return 0
	}
	return hand[i].Cost
}

func (g *Game) hpfxStep(m map[int64]*hpFx, id int64, curHP int, nowMs int64) *hpFx {
	fx := m[id]
	if fx == nil {
		fx = &hpFx{lastHP: curHP, ghostHP: curHP, healGhostHP: curHP}
		m[id] = fx
		return fx
	}

	// Handle damage (yellow flash)
	if curHP < fx.lastHP {
		fx.ghostHP = fx.lastHP
		fx.holdUntilMs = nowMs + 500
		fx.lerpStartMs = 0
		fx.lerpDurMs = 300
		fx.lerpStartHP = fx.ghostHP
	}

	// Handle healing (green flash)
	if curHP > fx.lastHP {
		fx.healGhostHP = fx.lastHP
		fx.healHoldUntilMs = nowMs + 500
		fx.healLerpStartMs = 0
		fx.healLerpDurMs = 300
		fx.healLerpStartHP = fx.healGhostHP
	}

	// Animate damage flash
	if fx.ghostHP > curHP && nowMs > fx.holdUntilMs {
		if fx.lerpStartMs == 0 {
			fx.lerpStartMs = nowMs
			fx.lerpStartHP = fx.ghostHP
		}
		t := float64(nowMs-fx.lerpStartMs) / float64(fx.lerpDurMs)
		if t >= 1 {
			fx.ghostHP = curHP
			fx.lerpStartMs = 0
		} else {
			gh := float64(fx.lerpStartHP) + (float64(curHP)-float64(fx.lerpStartHP))*t
			fx.ghostHP = int(math.Round(gh))
		}
	}

	// Animate healing flash
	if fx.healGhostHP < curHP && nowMs > fx.healHoldUntilMs {
		if fx.healLerpStartMs == 0 {
			fx.healLerpStartMs = nowMs
			fx.healLerpStartHP = fx.healGhostHP
		}
		t := float64(nowMs-fx.healLerpStartMs) / float64(fx.healLerpDurMs)
		if t >= 1 {
			fx.healGhostHP = curHP
			fx.healLerpStartMs = 0
		} else {
			gh := float64(fx.healLerpStartHP) + (float64(curHP)-float64(fx.healLerpStartHP))*t
			fx.healGhostHP = int(math.Round(gh))
		}
	}

	fx.lastHP = curHP
	return fx
}
func (g *Game) drawHPBar(screen *ebiten.Image, x, y, w, h float64, cur, max, ghost, healGhost int) {
	if max <= 0 || w <= 0 || h <= 0 {
		return
	}

	// Background (empty) is dark/black
	ebitenutil.DrawRect(screen, x, y, w, h, color.NRGBA{18, 18, 22, 255})

	pCur := float64(cur) / float64(max)
	pGhost := float64(ghost) / float64(max)
	pHealGhost := float64(healGhost) / float64(max)

	if pCur < 0 {
		pCur = 0
	} else if pCur > 1 {
		pCur = 1
	}
	if pGhost < 0 {
		pGhost = 0
	} else if pGhost > 1 {
		pGhost = 1
	}
	if pHealGhost < 0 {
		pHealGhost = 0
	} else if pHealGhost > 1 {
		pHealGhost = 1
	}

	// Draw damage flash (yellow) - appears when HP decreases
	if pGhost > pCur {
		startX := x + w*pCur
		ebitenutil.DrawRect(screen, startX, y, w*(pGhost-pCur), h, color.NRGBA{240, 196, 25, 255})
	}

	// Draw healing flash (green) - appears when HP increases
	if pHealGhost < pCur {
		startX := x + w*pHealGhost
		ebitenutil.DrawRect(screen, startX, y, w*(pCur-pHealGhost), h, color.NRGBA{100, 255, 100, 255})
	}

	// Filled portion color provided by caller via DrawHPBarForOwner
	// (this function no longer decides blue/red here)
}

// drawLevelBadge renders a round ornate badge (if available) or a fallback circle with the unit's level
// to the left of the provided bar rectangle. The badge is sized to be readable even for tiny bars.
func (g *Game) drawLevelBadge(screen *ebiten.Image, barRect image.Rectangle, level int, isPlayer bool) {
	// target badge size: at least 18px, or 6x bar height (can be tuned via BadgeScale meta)
	bh := barRect.Dy()
	mult := 6.0
	if ornate != nil && ornate.BadgeScale > 0 {
		mult = 6.0 * ornate.BadgeScale
	}
	size := int(float64(bh) * mult)
	if size < 18 {
		size = 18
	}
	// place badge center to the left of the bar, with a small gap
	gap := 6
	cx := barRect.Min.X - gap - size/2
	cy := barRect.Min.Y + bh/2
	if ornate != nil && ornate.Badge != nil {
		// scale ornate badge to size
		bb := ornate.Badge.Bounds()
		if bb.Dx() > 0 && bb.Dy() > 0 {
			s := float64(size) / float64(bb.Dy())
			x := float64(cx) - float64(bb.Dx())*s/2
			y := float64(cy) - float64(bb.Dy())*s/2
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(x, y)
			screen.DrawImage(ornate.Badge, op)
		}
	} else {
		// fallback: colored circle based on ownership with dark border
		var badgeColor, borderColor color.NRGBA
		if isPlayer {
			// Player units: blue badge
			badgeColor = color.NRGBA{70, 130, 255, 255}
			borderColor = color.NRGBA{30, 80, 200, 255}
		} else {
			// Enemy units: red badge
			badgeColor = color.NRGBA{220, 70, 70, 255}
			borderColor = color.NRGBA{180, 30, 30, 255}
		}
		vector.DrawFilledCircle(screen, float32(cx), float32(cy), float32(size)/2, badgeColor, true)
		vector.DrawFilledCircle(screen, float32(cx), float32(cy), float32(size)/2-1.5, borderColor, true)
	}
	// draw level text centered (dark text with light outline for readability)
	lvlS := fmt.Sprintf("%d", level)
	tw := text.BoundString(basicfont.Face7x13, lvlS).Dx()
	th := text.BoundString(basicfont.Face7x13, lvlS).Dy()
	tx := cx - tw/2
	ty := cy + th/2 - 2
	// outline
	text.Draw(screen, lvlS, basicfont.Face7x13, tx+1, ty, color.NRGBA{0, 0, 0, 200})
	text.Draw(screen, lvlS, basicfont.Face7x13, tx-1, ty, color.NRGBA{0, 0, 0, 200})
	text.Draw(screen, lvlS, basicfont.Face7x13, tx, ty+1, color.NRGBA{0, 0, 0, 200})
	text.Draw(screen, lvlS, basicfont.Face7x13, tx, ty-1, color.NRGBA{0, 0, 0, 200})
	text.Draw(screen, lvlS, basicfont.Face7x13, tx, ty, color.NRGBA{240, 240, 240, 255})
}

// helper to draw HP bar with team color
func (g *Game) DrawHPBarForOwner(screen *ebiten.Image, x, y, w, h float64, cur, max, ghost, healGhost int, isPlayer bool) {
	// base dark background + yellow ghost + green healing flash handled in drawHPBar; we overlay fill after
	g.drawHPBar(screen, x, y, w, h, cur, max, ghost, healGhost)
	fill := color.NRGBA{220, 70, 70, 255}
	if isPlayer {
		fill = color.NRGBA{70, 130, 255, 255}
	}
	// draw filled portion on top
	if max > 0 {
		pCur := float64(cur) / float64(max)
		if pCur < 0 {
			pCur = 0
		} else if pCur > 1 {
			pCur = 1
		}
		ebitenutil.DrawRect(screen, x, y, w*pCur, h, fill)
	}
}

// levelForUnitName resolves the visible level for a unit name, matching Army tab values.
// It looks up XP from g.unitXP by exact match, then by case-insensitive match.
func (g *Game) levelForUnitName(name string) int {
	lvl := 1
	if g.unitXP == nil {
		return lvl
	}
	if xp, ok := g.unitXP[name]; ok {
		if l, _, _ := computeLevel(xp); l > 0 {
			return l
		}
		return lvl
	}
	// case-insensitive fallback
	for k, xp := range g.unitXP {
		if strings.EqualFold(k, name) {
			if l, _, _ := computeLevel(xp); l > 0 {
				return l
			}
		}
	}
	return lvl
}

// currentArmyRoundedLevel returns the average level of the current champion+6 minis,
// rounded with .5 up, minimum 1.
func (g *Game) currentArmyRoundedLevel() int {
	names := []string{}
	if g.selectedChampion != "" {
		names = append(names, g.selectedChampion)
	}
	for i := 0; i < 6; i++ {
		if g.selectedOrder[i] != "" {
			names = append(names, g.selectedOrder[i])
		}
	}
	if len(names) == 0 {
		return 1
	}
	sum := 0.0
	for _, n := range names {
		lvl := 1
		if g.unitXP != nil {
			if xp, ok := g.unitXP[n]; ok {
				if l, _, _ := computeLevel(xp); l > 0 {
					lvl = l
				}
			}
		}
		sum += float64(lvl)
	}
	avg := sum / float64(len(names))
	r := int(avg + 0.5)
	if r < 1 {
		r = 1
	}
	return r
}

type battleHPBar struct {
	x, y, w, h int
	// animation
	maxHP      int
	targetHP   int // real hp you set each frame
	displayHP  int // lags behind to show yellow damage
	flashTicks int // frames of yellow blinking left
	// colors
	colBase    color.NRGBA
	colFlash   color.NRGBA
	colMissing color.NRGBA
}

// shouldMirrorForPvp determines if the screen should be mirrored for PvP
// to ensure each player sees their base at the bottom
func (g *Game) shouldMirrorForPvp() bool {
	// Check if this is PvP (has exactly 2 bases with different owners)
	isPvP := false
	playerBaseY := 0.0
	playerBaseCount := 0
	enemyBaseCount := 0
	for _, b := range g.world.Bases {
		if b.OwnerID == g.playerID {
			playerBaseY = float64(b.Y)
			playerBaseCount++
		} else {
			enemyBaseCount++
		}
	}
	// Only consider it PvP if there are exactly 2 bases AND it's actually a PvP room
	// (PvE also has 2 bases but should not be mirrored)
	if playerBaseCount == 1 && enemyBaseCount == 1 && strings.Contains(g.roomID, "pvp-") {
		isPvP = true
	}

	// Only apply mirroring in actual PvP scenarios (2+ players)
	// PVE should never be mirrored
	if !isPvP {
		return false
	}

	// PvP Mirroring: Ensure both players see their base at bottom
	// Mirror if player's base is at the TOP (needs to be moved to bottom)
	return playerBaseY < float64(protocol.ScreenH)/2
}

// battleHPs returns player and enemy HP values for battle UI
func (g *Game) battleHPs() (myCur, myMax, enCur, enMax int) {
	for _, b := range g.world.Bases {
		if b.OwnerID == g.playerID {
			myCur = b.HP
			myMax = b.MaxHP
		} else {
			enCur = b.HP
			enMax = b.MaxHP
		}
	}
	return
}

// drawBattleTopBars draws the HP bars and timer at the top of the battle screen
func (g *Game) drawBattleTopBars(screen *ebiten.Image, myCur, myMax, enCur, enMax int) {
	const pad = 12
	const barH = 18
	const avatar = 40
	const gap = 8

	y := 8

	base := protocol.ScreenW/2 - (pad + avatar + gap + pad)
	lw := int(float64(base) * 0.67)
	if lw < 120 {
		lw = 120
	}
	rw := lw

	lx := pad + avatar + gap
	if g.playerHB == nil {
		g.playerHB = &battleHPBar{
			x: lx, y: y, w: lw, h: barH,
			colBase:    color.NRGBA{70, 130, 255, 255},
			colFlash:   color.NRGBA{240, 196, 25, 255},
			colMissing: color.NRGBA{10, 10, 14, 255},
		}
	}
	g.playerHB.x, g.playerHB.y, g.playerHB.w, g.playerHB.h = lx, y, lw, barH
	g.playerHB.Set(myCur, myMax)
	g.playerHB.Update()
	g.playerHB.Draw(screen)
	if img := g.ensureAvatarImage(g.avatar); img != nil {
		iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
		s := math.Min(float64(avatar)/float64(iw), float64(avatar)/float64(ih))
		var op ebiten.DrawImageOptions
		op.GeoM.Scale(s, s)
		ax := float64(pad) + (float64(avatar)-float64(iw)*s)/2
		ay := float64(y) + (float64(barH)-float64(avatar))/2
		if ay < 2 {
			ay = 2
		}
		op.GeoM.Translate(ax, ay)
		screen.DrawImage(img, &op)
	}

	rx := protocol.ScreenW - pad - avatar - gap - rw
	if g.enemyHB == nil {
		g.enemyHB = &battleHPBar{
			x: rx, y: y, w: rw, h: barH,
			colBase:    color.NRGBA{220, 70, 70, 255},
			colFlash:   color.NRGBA{240, 196, 25, 255},
			colMissing: color.NRGBA{10, 10, 14, 255},
		}
	}
	g.enemyHB.x, g.enemyHB.y, g.enemyHB.w, g.enemyHB.h = rx, y, rw, barH
	g.enemyHB.Set(enCur, enMax)
	g.enemyHB.Update()
	g.enemyHB.Draw(screen)
	// Enemy avatar: PvP -> opponent avatar; PvE -> target (base/boss)
	var eimg *ebiten.Image
	if g.enemyAvatar != "" {
		eimg = g.ensureAvatarImage(g.enemyAvatar)
	} else {
		eimg = g.enemyTargetAvatarImage()
	}
	if eimg != nil {
		iw, ih := eimg.Bounds().Dx(), eimg.Bounds().Dy()
		s := math.Min(float64(avatar)/float64(iw), float64(avatar)/float64(ih))
		var op ebiten.DrawImageOptions
		op.GeoM.Scale(s, s)
		ax := float64(protocol.ScreenW-pad-avatar) + (float64(avatar)-float64(iw)*s)/2
		ay := float64(y) + (float64(barH)-float64(avatar))/2
		if ay < 2 {
			ay = 2
		}
		op.GeoM.Translate(ax, ay)
		screen.DrawImage(eimg, &op)
	}

	// Draw timer in the middle between HP bars (smaller and more compact)
	timerX := (lx + lw + rx) / 2
	timerY := y

	// Update timer display
	minutes := g.timerRemainingSeconds / 60
	seconds := g.timerRemainingSeconds % 60
	if g.timerPaused {
		g.timerDisplay = "PAUSED"
	} else {
		g.timerDisplay = fmt.Sprintf("%02d:%02d", minutes, seconds)
	}

	// Smaller timer background
	timerBgW := 80
	timerBgH := 24
	timerBgX := timerX - timerBgW/2
	ebitenutil.DrawRect(screen, float64(timerBgX-1), float64(timerY-1), float64(timerBgW+2), float64(timerBgH+2), color.NRGBA{0, 0, 0, 255})
	ebitenutil.DrawRect(screen, float64(timerBgX), float64(timerY), float64(timerBgW), float64(timerBgH), color.NRGBA{32, 32, 44, 255})

	// Draw timer text (centered)
	textW := text.BoundString(basicfont.Face7x13, g.timerDisplay).Dx()
	textX := timerBgX + (timerBgW-textW)/2
	text.Draw(screen, g.timerDisplay, basicfont.Face7x13, textX, timerY+18, color.NRGBA{239, 229, 182, 255})

	// Draw pause button (PvE only) - smaller and attached to timer
	if g.roomID != "" && !strings.Contains(g.roomID, "pvp-") {
		pauseBtnW := 24
		pauseBtnH := 24
		pauseBtnX := timerBgX + timerBgW + 2         // Attached to timer with small gap
		pauseBtnY := timerY + (timerBgH-pauseBtnH)/2 // Vertically centered

		g.timerBtn = rect{x: pauseBtnX, y: pauseBtnY, w: pauseBtnW, h: pauseBtnH}

		// Button background
		ebitenutil.DrawRect(screen, float64(pauseBtnX-1), float64(pauseBtnY-1), float64(pauseBtnW+2), float64(pauseBtnH+2), color.NRGBA{0, 0, 0, 255})
		ebitenutil.DrawRect(screen, float64(pauseBtnX), float64(pauseBtnY), float64(pauseBtnW), float64(pauseBtnH), color.NRGBA{70, 110, 70, 255})

		if g.timerPaused {
			// Play symbol (triangle pointing right) when paused
			triangleX := pauseBtnX + 4
			triangleY := pauseBtnY + 4
			// Draw triangle pointing right using rectangles
			for i := 0; i < 8; i++ {
				height := 16 - i*2
				if height > 0 {
					ebitenutil.DrawRect(screen, float64(triangleX+i), float64(triangleY+i), 1, float64(height), color.NRGBA{239, 229, 182, 255})
				}
			}
		} else {
			// Pause symbol (two vertical bars) when playing
			barW := 3
			barH := 12
			bar1X := pauseBtnX + 6
			bar2X := pauseBtnX + 13
			barY := pauseBtnY + 6
			ebitenutil.DrawRect(screen, float64(bar1X), float64(barY), float64(barW), float64(barH), color.NRGBA{239, 229, 182, 255})
			ebitenutil.DrawRect(screen, float64(bar2X), float64(barY), float64(barW), float64(barH), color.NRGBA{239, 229, 182, 255})
		}
	}

	// Draw pause overlay if active
	if g.pauseOverlay {
		// Semi-transparent overlay
		overlay := ebiten.NewImage(protocol.ScreenW, protocol.ScreenH)
		overlay.Fill(color.NRGBA{0, 0, 0, 140})
		screen.DrawImage(overlay, nil)

		// Pause menu
		menuW := 300
		menuH := 250
		menuX := (protocol.ScreenW - menuW) / 2
		menuY := (protocol.ScreenH - menuH) / 2

		ebitenutil.DrawRect(screen, float64(menuX), float64(menuY), float64(menuW), float64(menuH), color.NRGBA{32, 32, 44, 255})
		ebitenutil.DrawRect(screen, float64(menuX), float64(menuY), float64(menuW), 2, color.NRGBA{239, 229, 182, 255})
		ebitenutil.DrawRect(screen, float64(menuX), float64(menuY+menuH-2), float64(menuW), 2, color.NRGBA{239, 229, 182, 255})

		// Title
		text.Draw(screen, "GAME PAUSED", basicfont.Face7x13, menuX+100, menuY+30, color.NRGBA{239, 229, 182, 255})

		// Resume button
		resumeBtnX := menuX + 50
		resumeBtnY := menuY + 50
		ebitenutil.DrawRect(screen, float64(resumeBtnX), float64(resumeBtnY), 200, 40, color.NRGBA{70, 130, 70, 255})
		text.Draw(screen, "RESUME", basicfont.Face7x13, resumeBtnX+70, resumeBtnY+25, color.NRGBA{239, 229, 182, 255})

		// Restart button
		restartBtnX := menuX + 50
		restartBtnY := menuY + 100
		ebitenutil.DrawRect(screen, float64(restartBtnX), float64(restartBtnY), 200, 40, color.NRGBA{100, 100, 70, 255})
		text.Draw(screen, "RESTART MATCH", basicfont.Face7x13, restartBtnX+40, restartBtnY+25, color.NRGBA{239, 229, 182, 255})

		// Surrender button
		surrenderBtnX := menuX + 50
		surrenderBtnY := menuY + 150
		ebitenutil.DrawRect(screen, float64(surrenderBtnX), float64(surrenderBtnY), 200, 40, color.NRGBA{110, 70, 70, 255})
		text.Draw(screen, "SURRENDER", basicfont.Face7x13, surrenderBtnX+60, surrenderBtnY+25, color.NRGBA{239, 229, 182, 255})
	}
}
