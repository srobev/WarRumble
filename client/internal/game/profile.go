package game

import (
	"fmt"
	"image/color"
	"rumble/shared/protocol"
	"sort"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

func (g *Game) listAvatars() []string {
	buildEmbIndex()
	entries, err := assetsFS.ReadDir("assets/ui/avatars")
	if err != nil {
		return []string{"default.png"}
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		low := strings.ToLower(name)
		if strings.HasSuffix(low, ".png") || strings.HasSuffix(low, ".jpg") || strings.HasSuffix(low, ".jpeg") {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		out = []string{"default.png"}
	}
	return out
}

func (g *Game) ensureAvatarImage(file string) *ebiten.Image {
	g.assets.ensureInit()
	if file == "" {
		file = "default.png"
	}
	key := "avatars/" + strings.ToLower(file)
	if img, ok := g.assets.minis[key]; ok {
		return img
	}
	img := loadImage("assets/ui/" + key)
	g.assets.minis[key] = img
	return img
}

func (g *Game) drawProfileOverlay(screen *ebiten.Image) {

	ebitenutil.DrawRect(
		screen, 0, 0,
		float64(protocol.ScreenW), float64(protocol.ScreenH),
		color.NRGBA{10, 10, 18, 140},
	)

	if len(g.avatars) == 0 {
		g.avatars = g.listAvatars()
	}
	const cols = 6
	const cell = 60
	const gap = 10

	rows := (len(g.avatars) + cols - 1) / cols
	gridH := 0
	if rows > 0 {
		gridH = rows*cell + (rows-1)*gap
	}

	const headerH = 64 // title + subtitle + close
	const statsH = 72  // pvp stats block height
	const padOut = 16

	w := 520

	h := headerH + gridH + statsH + padOut*2 + 8

	if h > protocol.ScreenH-80 {
		h = protocol.ScreenH - 80
	}
	x := (protocol.ScreenW - w) / 2
	y := (protocol.ScreenH - h) / 2

	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), color.NRGBA{32, 32, 44, 230})

	title := g.name

	if title == "" {
		title = "Player"
	}
	ty := y + 22
	text.Draw(screen, title, basicfont.Face7x13, x+padOut, ty, color.White)

	text.Draw(screen, "Choose your avatar", basicfont.Face7x13, x+padOut, ty+20, color.NRGBA{210, 210, 220, 255})

	cW, cH := 28, 28
	g.profCloseBtn = rect{
		x: x + w - padOut - cW,
		y: y + (headerH-cH)/2,
		w: cW,
		h: cH,
	}

	ebitenutil.DrawRect(
		screen,
		float64(g.profCloseBtn.x), float64(g.profCloseBtn.y),
		float64(g.profCloseBtn.w), float64(g.profCloseBtn.h),
		color.NRGBA{70, 70, 90, 255},
	)

	label := "x"
	lb := text.BoundString(basicfont.Face7x13, label)
	tx := g.profCloseBtn.x + (g.profCloseBtn.w-lb.Dx())/2
	gy := g.profCloseBtn.y + (g.profCloseBtn.h+13)/2 - 2
	text.Draw(screen, label, basicfont.Face7x13, tx, gy, color.White)

	gridX := x + padOut
	gridY := y + headerH
	g.avatarRects = g.avatarRects[:0]

	for i, name := range g.avatars {
		c := i % cols
		r := i / cols
		cx := gridX + c*(cell+gap)
		cy := gridY + r*(cell+gap)
		rct := rect{x: cx, y: cy, w: cell, h: cell}
		g.avatarRects = append(g.avatarRects, rct)

		ebitenutil.DrawRect(screen, float64(cx), float64(cy), float64(cell), float64(cell),
			color.NRGBA{43, 43, 62, 255})

		if img := g.ensureAvatarImage(name); img != nil {
			iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
			s := mathMin(float64(cell-8)/float64(iw), float64(cell-8)/float64(ih))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(
				float64(cx)+(float64(cell)-float64(iw)*s)/2,
				float64(cy)+(float64(cell)-float64(ih)*s)/2,
			)
			screen.DrawImage(img, op)
		} else {
			text.Draw(screen, "?", basicfont.Face7x13, cx+cell/2-4, cy+cell/2+4, color.White)
		}

		if strings.EqualFold(name, g.avatar) {
			ebitenutil.DrawRect(screen, float64(cx), float64(cy), float64(cell), 2,
				color.NRGBA{240, 196, 25, 255})
		}
	}

	statsTop := gridY + gridH + 16
	if rows == 0 {
		statsTop = gridY + 8
	}

	ebitenutil.DrawRect(screen,
		float64(x+padOut-2), float64(statsTop-10),
		float64(w-2*padOut+4), float64(statsH), color.NRGBA{36, 36, 52, 255})

	y0 := statsTop + 10
	text.Draw(screen, "PvP Stats", basicfont.Face7x13, x+padOut, y0, color.White)

	lineY := y0 + 18
	text.Draw(screen, fmt.Sprintf("Rating: %d", g.pvpRating),
		basicfont.Face7x13, x+padOut, lineY, color.NRGBA{220, 220, 230, 255})
	text.Draw(screen, fmt.Sprintf("Rank:   %s", g.pvpRank),
		basicfont.Face7x13, x+padOut+160, lineY, color.NRGBA{240, 196, 25, 255})

	btnW, btnH := 96, 28
	bx := x + w - btnW - padOut
	by := y + h - btnH - padOut

	g.profLogoutBtn = rect{x: bx, y: by, w: btnW, h: btnH}

	ebitenutil.DrawRect(screen,
		float64(bx), float64(by),
		float64(btnW), float64(btnH),
		color.NRGBA{120, 40, 40, 255},
	)

    text.Draw(screen, "Logout",
        basicfont.Face7x13, bx+20, by+18, color.White)
}
