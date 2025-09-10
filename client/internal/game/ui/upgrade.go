package ui

import (
	"image"
	"image/color"

	net "rumble/client/internal/game/net"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

type UnitRenderState struct {
	Unit  string
	Perks []net.PerkView
	// TODO: add rarity/shards/level when you have those available in client
	Fetching bool
	Err      error
}

func DrawUnitPanel(screen *ebiten.Image, panel image.Rectangle, s *UnitRenderState) {
	bg := ebiten.NewImage(panel.Dx(), panel.Dy())
	bg.Fill(color.RGBA{20, 25, 35, 200})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(panel.Min.X), float64(panel.Min.Y))
	screen.DrawImage(bg, op)

	// Title
	title := "Unit"
	if s.Unit != "" {
		title = s.Unit
	}
	text.Draw(screen, title, basicfont.Face7x13, panel.Min.X+16, panel.Min.Y+24, color.White)

	if s.Fetching {
		text.Draw(screen, "Loading...", basicfont.Face7x13, panel.Min.X+16, panel.Min.Y+48, color.White)
		return
	}
	if s.Err != nil {
		text.Draw(screen, "Error loading perks", basicfont.Face7x13, panel.Min.X+16, panel.Min.Y+48, color.RGBA{255, 120, 120, 255})
	}

	// Right column for 3 perk cards
	top := panel.Min.Y + 56
	left := panel.Min.X + 16
	w := panel.Dx() - 32
	cardH := 80
	gap := 10

	for i := 0; i < len(s.Perks) && i < 3; i++ {
		y := top + i*(cardH+gap)
		r := image.Rect(left, y, left+w, y+cardH)
		drawPerkCard(screen, r, s.Perks[i])
	}
}

func drawPerkCard(screen *ebiten.Image, rect image.Rectangle, p net.PerkView) {
	ctx := ebiten.NewImage(rect.Dx(), rect.Dy())
	col := color.RGBA{45, 45, 65, 255}
	if p.Active {
		col = color.RGBA{35, 65, 35, 255}
	}
	ctx.Fill(col)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(rect.Min.X), float64(rect.Min.Y))
	screen.DrawImage(ctx, op)

	text.Draw(screen, p.Name, basicfont.Face7x13, rect.Min.X+8, rect.Min.Y+18, color.White)

	desc := p.Desc
	if len(desc) > 80 {
		desc = desc[:77] + "..."
	}
	text.Draw(screen, desc, basicfont.Face7x13, rect.Min.X+8, rect.Min.Y+36, color.RGBA{200, 200, 220, 255})

	state := "Locked"
	if !p.Locked && !p.Purchased {
		state = "Unpurchased (250)"
	}
	if p.Purchased && !p.Active {
		state = "Activate"
	}
	if p.Active {
		state = "Active"
	}
	text.Draw(screen, state, basicfont.Face7x13, rect.Min.X+8, rect.Min.Y+58, color.RGBA{230, 230, 140, 255})
}
