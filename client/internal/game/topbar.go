package game

import (
	"fmt"
	"image/color"

	"rumble/shared/protocol"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
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
	lb := text.BoundString(basicfont.Face7x13, name)
	baselineY := g.userBtn.y + (g.userBtn.h+lb.Dy())/2 - 2
	text.Draw(screen, name, basicfont.Face7x13, g.userBtn.x+avPad+avW+8, baselineY, color.White)

	if hoveredUser {
		// Dynamically size hover card to fit username and rating text
		title := "Account"
		label := g.name
		if label == "" {
			label = "Player"
		}
		ratingLbl := "PvP rating"
		ratingVal := fmt.Sprintf("%d  (%s)", g.pvpRating, defaultIfEmpty(g.pvpRank, "Unranked"))

		titleW := text.BoundString(basicfont.Face7x13, title).Dx()
		nameW := text.BoundString(basicfont.Face7x13, label).Dx()
		rlW := text.BoundString(basicfont.Face7x13, ratingLbl).Dx()
		rvW := text.BoundString(basicfont.Face7x13, ratingVal).Dx()

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
		ebitenutil.DrawRect(screen, float64(tx), float64(ty), float64(tipW), float64(tipH), color.NRGBA{30, 30, 45, 240})

		// Avatar block
		ax := tx + 10
		ay := ty + 10
		aw, ah := 56, 56
		if img := g.ensureAvatarImage(g.avatar); img != nil {
			iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
			s := mathMin(float64(aw)/float64(iw), float64(ah)/float64(ih))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(float64(ax)+(float64(aw)-float64(iw)*s)/2, float64(ay)+(float64(ah)-float64(ih)*s)/2)
			screen.DrawImage(img, op)
		} else {
			ebitenutil.DrawRect(screen, float64(ax), float64(ay), float64(aw), float64(ah), color.NRGBA{70, 70, 90, 255})
		}

		// Text lines
		text.Draw(screen, title, basicfont.Face7x13, tx+leftTextX, ty+25, color.NRGBA{240, 196, 25, 255})
		text.Draw(screen, label, basicfont.Face7x13, tx+leftTextX, ty+43, color.White)
		text.Draw(screen, ratingLbl, basicfont.Face7x13, tx+leftTextX, ty+61, color.NRGBA{240, 196, 25, 255})
		text.Draw(screen, ratingVal, basicfont.Face7x13, tx+leftTextX+rlW+10, ty+61, color.White)
	}

	title := protocol.GameName
	tb := text.BoundString(basicfont.Face7x13, title)
	ty := g.titleArea.y + (topBarH+tb.Dy())/2 - 2
	text.Draw(screen, title, basicfont.Face7x13, g.titleArea.x+8, ty, color.White)

	goldStr := fmt.Sprintf("Gold: %d", g.accountGold)
	gb := text.BoundString(basicfont.Face7x13, goldStr)
	gy := g.goldArea.y + (topBarH+gb.Dy())/2 - 2
	text.Draw(screen, goldStr, basicfont.Face7x13, g.goldArea.x+6, gy, color.NRGBA{240, 196, 25, 255})

	hoveredGold := g.goldArea.hit(mx, my)
	if hoveredGold {
		tip := []string{
			"Your account gold.",
			"Earn gold from battles, events,",
			"or rewards. Spend it in the shop.",
		}
		tipW, tipH := 260, 66
		tx := clampInt(mx-tipW-10, 0, protocol.ScreenW-tipW)
		ty := clampInt(my+12, 0, protocol.ScreenH-tipH)
		ebitenutil.DrawRect(screen, float64(tx), float64(ty), float64(tipW), float64(tipH), color.NRGBA{30, 30, 45, 240})
		text.Draw(screen, "Gold", basicfont.Face7x13, tx+10, ty+18, color.NRGBA{240, 196, 25, 255})
		text.Draw(screen, tip[0], basicfont.Face7x13, tx+10, ty+34, color.White)
		text.Draw(screen, tip[1], basicfont.Face7x13, tx+10, ty+48, color.NRGBA{210, 210, 220, 255})
		text.Draw(screen, tip[2], basicfont.Face7x13, tx+10, ty+62, color.NRGBA{210, 210, 220, 255})
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
	nameBounds := text.BoundString(basicfont.Face7x13, uname)
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

	goldStr := fmt.Sprintf("Gold: %d", g.accountGold)
	gb := text.BoundString(basicfont.Face7x13, goldStr)
	g.goldArea = rect{
		x: protocol.ScreenW - pad - gb.Dx() - 12,
		y: 0,
		w: gb.Dx() + 12,
		h: topBarH,
	}

	title := protocol.GameName
	tb := text.BoundString(basicfont.Face7x13, title)
	tx := (protocol.ScreenW - tb.Dx()) / 2
	g.titleArea = rect{x: tx - 8, y: 0, w: tb.Dx() + 16, h: topBarH}
}
