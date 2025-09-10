package shop

import (
	"fmt"
	"image/color"
	"rumble/shared/game/types"
	"rumble/shared/protocol"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

const (
	CARDS_PER_ROW = 3   // 3 columns for 3x3 grid
	CARDS_PER_COL = 3   // 3 rows for 3x3 grid
	CARD_WIDTH    = 80  // card width in pixels (larger for 3x3)
	CARD_HEIGHT   = 110 // card height in pixels (larger for 3x3)
	CARD_MARGIN   = 8   // margin between cards
	GRID_TOP      = 100 // top offset from screen
)

// ShopGrid handles the 3x3 shop card grid layout and interaction
type ShopGrid struct {
	x, y          int            // grid top-left position
	width, height int            // total grid dimensions
	rects         []ShopCardRect // rectangles for each shop slot
}

// ShopCardRect represents a single shop card slot position and dimensions
type ShopCardRect struct {
	X, Y, W, H int
	Slot       int
	Data       *types.ShopSlot // reference to the shop slot data
}

// NewShopGrid creates a new shop grid with proper 3x3 layout
func NewShopGrid(screenWidth int) *ShopGrid {
	grid := &ShopGrid{
		x:      8, // left padding
		y:      GRID_TOP,
		width:  screenWidth - 16,                  // leave padding on both sides
		height: protocol.ScreenH - GRID_TOP - 120, // leave space for reroll button
	}

	// Calculate total grid area needed for 3x3
	gridW := CARDS_PER_ROW*CARD_WIDTH + (CARDS_PER_ROW-1)*CARD_MARGIN

	// Center the grid horizontally
	grid.x = (screenWidth - gridW) / 2

	// Create rectangles for each slot
	grid.rects = make([]ShopCardRect, CARDS_PER_ROW*CARDS_PER_COL)
	slot := 0

	for row := 0; row < CARDS_PER_COL; row++ {
		for col := 0; col < CARDS_PER_ROW; col++ {
			x := grid.x + col*(CARD_WIDTH+CARD_MARGIN)
			y := grid.y + row*(CARD_HEIGHT+CARD_MARGIN)

			grid.rects[slot] = ShopCardRect{
				X: x, Y: y,
				W:    CARD_WIDTH,
				H:    CARD_HEIGHT,
				Slot: slot,
			}
			slot++
		}
	}

	return grid
}

// UpdateLayout updates grid layout when shop roll data is available
func (g *ShopGrid) UpdateLayout(roll types.ShopRoll) {
	for i := range g.rects {
		if i < len(roll.Slots) {
			// Create a copy of the slot data
			slot := roll.Slots[i]
			g.rects[i].Data = &slot
		}
	}
}

// GetSlotAtPoint returns the slot index at the given screen coordinates
func (g *ShopGrid) GetSlotAtPoint(x, y int) int {
	for _, card := range g.rects {
		if x >= card.X && x < card.X+card.W &&
			y >= card.Y && y < card.Y+card.H {
			return card.Slot
		}
	}
	return -1
}

// Draw renders the shop grid
func (g *ShopGrid) Draw(screen *ebiten.Image, shopState *ShopState, fantasyUI interface{}, imageLoader func(string) interface{}) {
	for _, card := range g.rects {
		g.drawShopCard(screen, card, shopState, fantasyUI, imageLoader)
	}
}

// drawShopCard renders a single shop card
func (g *ShopGrid) drawShopCard(screen *ebiten.Image, card ShopCardRect, shopState *ShopState, fantasyUI interface{}, imageLoader func(string) interface{}) {
	x, y, w, h := card.X, card.Y, card.W, card.H
	slotData := card.Data

	// Default card styling using Army tab themed colors
	borderColor := color.NRGBA{100, 100, 100, 255} // Theme.Border
	fillColor := color.NRGBA{43, 45, 58, 255}      // Theme.Surface

	// State-based styling
	if slotData == nil {
		// Empty slot
		fillColor = color.NRGBA{32, 34, 48, 200} // Theme.CardBackground
	} else {
		// Rarity-based color matching urban occupation units in army tab
		switch slotData.IsChampion {
		case true:
			borderColor = color.NRGBA{255, 215, 0, 255} // Orange/Gold for champions (Theme.Secondary)
		case false:
			borderColor = color.NRGBA{138, 136, 134, 255} // Grey for minis (Theme.Primary)
		}

		// Sold state
		if slotData.Sold {
			borderColor = color.NRGBA{80, 80, 80, 255}
			fillColor = color.NRGBA{40, 42, 56, 200}
		}

		// Pending state
		if shopState.IsPending(card.Slot) {
			borderColor = color.NRGBA{255, 215, 0, 255} // Gold for pending (Theme.Secondary)
			fillColor = color.NRGBA{48, 45, 35, 245}    // Darker gold tint
		}

		// Hover state
		if card.Slot == shopState.GetHoveredSlot() {
			borderColor = color.NRGBA{160, 160, 160, 255} // Theme.Glow
			// Slightly lighten the fill
			r, g, b, a := fillColor.R, fillColor.G, fillColor.B, fillColor.A
			fillColor = color.NRGBA{
				R: uint8(minInt(255, int(r)+15)),
				G: uint8(minInt(255, int(g)+15)),
				B: uint8(minInt(255, int(b)+15)),
				A: uint8(a),
			}
		}
	}

	// Draw card background
	ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), fillColor)

	// Draw card border
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(h), 2, borderColor, true)

	contentX := x + 4
	contentY := y + 4

	if slotData == nil {
		text.Draw(screen, "Empty", basicfont.Face7x13, contentX, contentY+14, color.NRGBA{120, 120, 130, 255})
	} else {
		// Unit name at bottom (smaller font, centered)
		unitName := strings.Title(slotData.UnitID)
		textW := text.BoundString(basicfont.Face7x13, unitName).Dx()
		text.Draw(screen, unitName, basicfont.Face7x13, x+w/2-textW/2, y+h-16, color.NRGBA{255, 255, 255, 255})

		// Price at bottom-left (smaller)
		priceStr := fmt.Sprintf("%dg", slotData.PriceGold)
		text.Draw(screen, priceStr, basicfont.Face7x13, contentX, y+h-6, color.NRGBA{255, 215, 0, 255}) // Gold color

		// Champion indicator at top-right
		if slotData.IsChampion {
			text.Draw(screen, "CH", basicfont.Face7x13, x+w-20, contentY+10, color.NRGBA{255, 140, 0, 255})
		}

		// Sold overlay
		if slotData.Sold {
			ebitenutil.DrawRect(screen, float64(x+8), float64(y+h/2-10), float64(w-16), 20, color.NRGBA{100, 100, 100, 180})
			text.Draw(screen, "SOLD", basicfont.Face7x13, x+w/2-20, y+h/2+4, color.White)
		}

		// Unit portrait - load actual unit image
		// Position at top-right corner of card
		portraitX := x + w - 44
		portraitY := contentY - 4
		portraitSize := 44

		// Try to load the unit image
		unitImg := imageLoader(slotData.UnitID)
		if unitImg != nil {
			// Cast to ebiten.Image and draw it
			if img, ok := unitImg.(*ebiten.Image); ok && img != nil {
				// Get image bounds
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()

				// Scale to fit portrait size proportionally (keep aspect ratio)
				scale := float64(portraitSize-4) / float64(maxInt(iw, ih))
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(scale, scale)

				// Center the image in the portrait area
				sw, sh := float64(iw)*scale, float64(ih)*scale
				dx := portraitX + 2 + int((float64(portraitSize-4)-sw)/2)
				dy := portraitY + 2 + int((float64(portraitSize-4)-sh)/2)

				op.GeoM.Translate(float64(dx), float64(dy))
				screen.DrawImage(img, op)

				// Draw portrait border/frame
				vector.StrokeRect(screen, float32(portraitX), float32(portraitY),
					float32(portraitSize), float32(portraitSize), 1,
					color.NRGBA{150, 150, 180, 255}, true)
			}
		} else {
			// Fallback to simple placeholder if no image available
			fallbackColor := color.NRGBA{100, 150, 255, 255}
			if slotData.IsChampion {
				fallbackColor = color.NRGBA{255, 180, 60, 255}
			}

			ebitenutil.DrawRect(screen, float64(portraitX), float64(portraitY),
				float64(portraitSize), float64(portraitSize), fallbackColor)

			// Draw unit type characteristic
			label := "M"
			if slotData.IsChampion {
				label = "C"
			}
			text.Draw(screen, label, basicfont.Face7x13,
				portraitX+portraitSize/2-4, portraitY+portraitSize/2+4, color.White)
		}
	}
}

// Helper functions
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
