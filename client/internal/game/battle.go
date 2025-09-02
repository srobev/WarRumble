package game

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"rumble/shared/protocol"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
    "strings"
)

type hpFx struct {
	lastHP      int
	ghostHP     int   // where the yellow chip currently ends (>= current HP)
	holdUntilMs int64 // time to keep the yellow chip still
	lerpStartMs int64 // when the chip starts animating
	lerpStartHP int   // chip start HP when anim begins
	lerpDurMs   int64 // animation duration (ms)
}

func (g *Game) updateBattle() {

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
			}
		}
		return
	}

	mx, my := ebiten.CursorPosition()
	handTop := protocol.ScreenH - battleHUDH

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		for i, r := range g.handRects() {
			if r.hit(mx, my) {
				g.selectedIdx = i
				g.dragActive = true
				g.dragIdx = i
				g.dragStartX, g.dragStartY = mx, my
				return
			}
		}

		if my < handTop && g.selectedIdx >= 0 && g.selectedIdx < len(g.hand) {
			g.send("DeployMiniAt", protocol.DeployMiniAt{
				CardIndex: g.selectedIdx,
				X:         float64(mx),
				Y:         float64(my),
				ClientTs:  time.Now().UnixMilli(),
			})
			return
		}
	}

	if g.dragActive && inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		if my < handTop && g.dragIdx >= 0 && g.dragIdx < len(g.hand) {
			g.send("DeployMiniAt", protocol.DeployMiniAt{
				CardIndex: g.dragIdx,
				X:         float64(mx),
				Y:         float64(my),
				ClientTs:  time.Now().UnixMilli(),
			})
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
		if img := g.ensureMiniImageByName(g.hand[g.dragIdx].Name); img != nil {
			iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
			s := 0.5 * mathMin(1, 48.0/float64(maxInt(iw, ih)))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(float64(mx)-float64(iw)*s/2, float64(my)-float64(ih)*s/2)
			op.ColorScale.Scale(1, 1, 1, 0.75)
			screen.DrawImage(img, op)
		} else {
			ebitenutil.DrawRect(screen, float64(mx-12), float64(my-12), 24, 24, color.NRGBA{220, 220, 220, 200})
		}
	}
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
		fx = &hpFx{lastHP: curHP, ghostHP: curHP}
		m[id] = fx
		return fx
	}

	if curHP < fx.lastHP {
		fx.ghostHP = fx.lastHP
		fx.holdUntilMs = nowMs + 500
		fx.lerpStartMs = 0
		fx.lerpDurMs = 300
		fx.lerpStartHP = fx.ghostHP
	}

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

	fx.lastHP = curHP
	return fx
}
func (g *Game) drawHPBar(screen *ebiten.Image, x, y, w, h float64, cur, max, ghost int) {
	if max <= 0 || w <= 0 || h <= 0 {
		return
	}

    // Background (empty) is dark/black
    ebitenutil.DrawRect(screen, x, y, w, h, color.NRGBA{18, 18, 22, 255})

    pCur := float64(cur) / float64(max)
    pGhost := float64(ghost) / float64(max)
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

    if pGhost > pCur {
        startX := x + w*pCur
        ebitenutil.DrawRect(screen, startX, y, w*(pGhost-pCur), h, color.NRGBA{240, 196, 25, 255})
    }
    // Filled portion color provided by caller via DrawHPBarForOwner
    // (this function no longer decides blue/red here)
}

// drawLevelBadge renders a round ornate badge (if available) or a fallback circle with the unit's level
// to the left of the provided bar rectangle. The badge is sized to be readable even for tiny bars.
func (g *Game) drawLevelBadge(screen *ebiten.Image, barRect image.Rectangle, level int) {
    // target badge size: at least 18px, or 6x bar height (can be tuned via BadgeScale meta)
    bh := barRect.Dy()
    mult := 6.0
    if ornate != nil && ornate.BadgeScale > 0 { mult = 6.0 * ornate.BadgeScale }
    size := int(float64(bh) * mult)
    if size < 18 { size = 18 }
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
        // fallback: simple gold circle with dark border
        vector.DrawFilledCircle(screen, float32(cx), float32(cy), float32(size)/2, color.NRGBA{240,196,25,255}, true)
        vector.DrawFilledCircle(screen, float32(cx), float32(cy), float32(size)/2-1.5, color.NRGBA{200,160,20,255}, true)
    }
    // draw level text centered (dark text with light outline for readability)
    lvlS := fmt.Sprintf("%d", level)
    tw := text.BoundString(basicfont.Face7x13, lvlS).Dx()
    th := text.BoundString(basicfont.Face7x13, lvlS).Dy()
    tx := cx - tw/2
    ty := cy + th/2 - 2
    // outline
    text.Draw(screen, lvlS, basicfont.Face7x13, tx+1, ty, color.NRGBA{0,0,0,200})
    text.Draw(screen, lvlS, basicfont.Face7x13, tx-1, ty, color.NRGBA{0,0,0,200})
    text.Draw(screen, lvlS, basicfont.Face7x13, tx, ty+1, color.NRGBA{0,0,0,200})
    text.Draw(screen, lvlS, basicfont.Face7x13, tx, ty-1, color.NRGBA{0,0,0,200})
    text.Draw(screen, lvlS, basicfont.Face7x13, tx, ty, color.NRGBA{240,240,240,255})
}

// helper to draw HP bar with team color
func (g *Game) DrawHPBarForOwner(screen *ebiten.Image, x, y, w, h float64, cur, max, ghost int, isPlayer bool) {
    // base dark background + yellow ghost handled in drawHPBar; we overlay fill after
    g.drawHPBar(screen, x, y, w, h, cur, max, ghost)
    fill := color.NRGBA{220, 70, 70, 255}
    if isPlayer {
        fill = color.NRGBA{70, 130, 255, 255}
    }
    // draw filled portion on top
    if max > 0 {
        pCur := float64(cur) / float64(max)
        if pCur < 0 { pCur = 0 } else if pCur > 1 { pCur = 1 }
        ebitenutil.DrawRect(screen, x, y, w*pCur, h, fill)
    }
}

// levelForUnitName resolves the visible level for a unit name, matching Army tab values.
// It looks up XP from g.unitXP by exact match, then by case-insensitive match.
func (g *Game) levelForUnitName(name string) int {
    lvl := 1
    if g.unitXP == nil { return lvl }
    if xp, ok := g.unitXP[name]; ok {
        if l, _, _ := computeLevel(xp); l > 0 { return l }
        return lvl
    }
    // case-insensitive fallback
    for k, xp := range g.unitXP {
        if strings.EqualFold(k, name) {
            if l, _, _ := computeLevel(xp); l > 0 { return l }
        }
    }
    return lvl
}

// currentArmyRoundedLevel returns the average level of the current champion+6 minis,
// rounded with .5 up, minimum 1.
func (g *Game) currentArmyRoundedLevel() int {
    names := []string{}
    if g.selectedChampion != "" { names = append(names, g.selectedChampion) }
    for i := 0; i < 6; i++ { if g.selectedOrder[i] != "" { names = append(names, g.selectedOrder[i]) } }
    if len(names) == 0 { return 1 }
    sum := 0.0
    for _, n := range names {
        lvl := 1
        if g.unitXP != nil { if xp, ok := g.unitXP[n]; ok { if l,_,_ := computeLevel(xp); l>0 { lvl = l } } }
        sum += float64(lvl)
    }
    avg := sum / float64(len(names))
    r := int(avg + 0.5)
    if r < 1 { r = 1 }
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
