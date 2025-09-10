package ui

import (
	"fmt"
	"image"
	"image/color"
	"time"

	net "rumble/client/internal/game/net" // your net layer

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

type ShopRenderState struct {
	Fetching   bool
	Err        error
	Offers     []net.Offer
	Toast      string
	ToastUntil time.Time
	// Display-only cooldown (seconds left); compute in caller
	CooldownSec int
}

var (
	shopPad    = 16
	shopTitleH = 24
	shopGap    = 8
	shopCols   = 3
	shopRows   = 3
	shopFooter = 44
)

func DrawShopPanel(screen *ebiten.Image, panel image.Rectangle, s *ShopRenderState) {
	// Panel bg (semi-transparent; no screen.Fill)
	bg := ebiten.NewImage(panel.Dx(), panel.Dy())
	bg.Fill(color.RGBA{20, 20, 30, 180})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(panel.Min.X), float64(panel.Min.Y))
	screen.DrawImage(bg, op)

	// Title
	text.Draw(screen, "Shop", basicfont.Face7x13,
		panel.Min.X+shopPad, panel.Min.Y+shopPad+shopTitleH/2, color.White)

	// Status
	if s.Fetching {
		text.Draw(screen, "Loading...", basicfont.Face7x13,
			panel.Min.X+shopPad, panel.Min.Y+shopPad+shopTitleH+8, color.White)
		// still draw footer
	} else if s.Err != nil {
		text.Draw(screen, "Error loading offers", basicfont.Face7x13,
			panel.Min.X+shopPad, panel.Min.Y+shopPad+shopTitleH+8, color.RGBA{255, 120, 120, 255})
	}

	// Grid rect
	gridTop := panel.Min.Y + shopPad + shopTitleH + 12
	gridLeft := panel.Min.X + shopPad
	gridW := panel.Dx() - 2*shopPad
	gridH := panel.Dy() - shopFooter - (gridTop - panel.Min.Y) - shopPad

	// Card size
	cw := (gridW - (shopCols-1)*shopGap) / shopCols
	ch := (gridH - (shopRows-1)*shopGap) / shopRows

	// Draw up to 9 offers
	max := len(s.Offers)
	if max > shopCols*shopRows {
		max = shopCols * shopRows
	}
	for i := 0; i < max; i++ {
		r := i / shopCols
		c := i % shopCols
		x := gridLeft + c*(cw+shopGap)
		y := gridTop + r*(ch+shopGap)
		card := image.Rect(x, y, x+cw, y+ch)
		drawOfferCard(screen, card, s.Offers[i])
	}

	// Footer with reroll info
	footer := image.Rect(panel.Min.X+shopPad, panel.Max.Y-shopFooter, panel.Max.X-shopPad, panel.Max.Y-shopPad)
	drawShopFooter(screen, footer, s)

	// Toast
	if s.Toast != "" && time.Now().Before(s.ToastUntil) {
		text.Draw(screen, s.Toast, basicfont.Face7x13,
			footer.Min.X, footer.Min.Y-8, color.RGBA{255, 230, 150, 255})
	}
}

func drawOfferCard(screen *ebiten.Image, rect image.Rectangle, o net.Offer) {
	card := ebiten.NewImage(rect.Dx(), rect.Dy())
	card.Fill(color.RGBA{40, 40, 60, 200})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(rect.Min.X), float64(rect.Min.Y))
	screen.DrawImage(card, op)

	// Title & price (simple text; replace with portrait later)
	title := o.Unit
	if o.Type == "perk" && o.PerkID != nil {
		title = *o.PerkID
	}
	text.Draw(screen, title, basicfont.Face7x13, rect.Min.X+8, rect.Min.Y+18, color.White)

	price := "250"
	if o.PriceGold > 0 {
		price = formatGold(o.PriceGold)
	}
	text.Draw(screen, price, basicfont.Face7x13, rect.Max.X-8-len(price)*7, rect.Min.Y+18, color.RGBA{255, 215, 0, 255})
}

func drawShopFooter(screen *ebiten.Image, rect image.Rectangle, s *ShopRenderState) {
	bar := ebiten.NewImage(rect.Dx(), rect.Dy())
	bar.Fill(color.RGBA{30, 30, 45, 220})
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(rect.Min.X), float64(rect.Min.Y))
	screen.DrawImage(bar, op)

	btn := "[ Reroll ]"
	col := color.White
	if s.CooldownSec > 0 {
		btn = "[ Reroll (" + itoa(s.CooldownSec) + "s) ]"
		col = color.RGBA{180, 180, 180, 255}
	}
	text.Draw(screen, btn, basicfont.Face7x13, rect.Min.X+8, rect.Min.Y+rect.Dy()/2+5, col)
}

// Small helpers (no new deps)
func itoa(n int) string       { return fmt.Sprintf("%d", n) } // add: import "fmt"
func formatGold(v int) string { return itoa(v) }
