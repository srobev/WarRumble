package shop

import (
	"fmt"
	"image/color"

	"rumble/shared/game/types"
	"rumble/shared/protocol"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

// LoadLobbyMinis loads unit information from the server's data
func LoadLobbyMinis() []protocol.MiniInfo {
	// This should be connected to the server data loading
	// For now, return some basic unit info for shop display
	return []protocol.MiniInfo{
		{Name: "Test Unit 1", Cost: 10, Features: []string{"Melee", "Fast"}},
		{Name: "Test Unit 2", Cost: 15, Features: []string{"Ranged", "Armored"}},
		{Name: "Test Unit 3", Cost: 20, Features: []string{"Magic", "AoE"}},
	}
}

const (
	CARDS_PER_ROW = 3
	CARDS_PER_COL = 3
	CARD_WIDTH    = 140
	CARD_HEIGHT   = 180
	CARD_SPACING  = 16
	GRID_TOP      = 140

	CARD_BG      = 0x1e293b
	CARD_HOVER   = 0x334155
	BORDER_COLOR = 0x475569
	GOLD_ACCENT  = 0xf59e0b

	TEXT_WHITE = 0xf8fafc
	TEXT_GRAY  = 0x94a3b8
)

// PurchaseDialog represents the info dialog
type PurchaseDialog struct {
	Visible    bool
	Slot       int
	SlotData   *types.ShopSlot
	UnitInfo   *protocol.MiniInfo
	X, Y, W, H int
}

// ShopGrid handles the clean shop grid
type ShopGrid struct {
	x, y          int
	width, height int
	rects         []ShopCardRect
	Dialog        *PurchaseDialog
	imageLoader   func(string) interface{}
}

// ShopCardRect represents a card slot
type ShopCardRect struct {
	X, Y, W, H int
	Slot       int
	Data       *types.ShopSlot
}

func getUnitFeatures(miniinfo *protocol.MiniInfo) string {
	if miniinfo == nil {
		return "Unknown unit"
	}

	features := ""
	for i, feature := range miniinfo.Features {
		if i > 0 {
			features += ", "
		}
		features += feature
	}
	return features
}

func (g *ShopGrid) getUnitInfo(unitID string) *protocol.MiniInfo {
	for _, mini := range LoadLobbyMinis() {
		if mini.Name == unitID {
			return &mini
		}
	}
	return nil
}

// NewShopGrid creates a clean shop grid

func NewShopGrid(screenWidth int, imageLoader func(string) interface{}) *ShopGrid {
	grid := &ShopGrid{
		x:           (screenWidth - (CARD_WIDTH*CARDS_PER_ROW + CARD_SPACING*2)) / 2, // Center the grid
		y:           GRID_TOP,
		width:       screenWidth - 16,
		imageLoader: imageLoader,
		Dialog:      &PurchaseDialog{},
	}

	grid.rects = make([]ShopCardRect, CARDS_PER_ROW*CARDS_PER_COL)
	slot := 0

	for row := 0; row < CARDS_PER_COL; row++ {
		for col := 0; col < CARDS_PER_ROW; col++ {
			x := grid.x + col*(CARD_WIDTH+CARD_SPACING)
			y := grid.y + row*(CARD_HEIGHT+CARD_SPACING)

			grid.rects[slot] = ShopCardRect{
				X: x, Y: y, W: CARD_WIDTH, H: CARD_HEIGHT,
				Slot: slot,
			}
			slot++
		}
	}

	return grid
}

// UpdateLayout updates when shop data changes
func (g *ShopGrid) UpdateLayout(roll types.ShopRoll) {
	for i := range g.rects {
		if i < len(roll.Slots) {
			slotCopy := roll.Slots[i]
			g.rects[i].Data = &slotCopy
		}
	}
}

// GetSlotAtPoint returns slot at coordinates
func (g *ShopGrid) GetSlotAtPoint(x, y int) int {
	for _, card := range g.rects {
		if x >= card.X && x < card.X+card.W &&
			y >= card.Y && y < card.Y+card.H {
			return card.Slot
		}
	}
	return -1
}

// HandleClick handles clicks for info dialog
func (g *ShopGrid) HandleClick(x, y int, broadcaster func(string, interface{})) {
	if g.Dialog.Visible {
		if g.handleDialogClick(x, y, broadcaster) {
			return
		}
	}

	slot := g.GetSlotAtPoint(x, y)
	if slot >= 0 && slot < len(g.rects) {
		slotData := g.rects[slot].Data
		if slotData != nil && !slotData.Sold {
			// Show purchase dialog instead of buying
			g.showPurchaseDialog(slot, slotData)
		}
	}
}

// handleDialogClick handles clicks on dialog
func (g *ShopGrid) handleDialogClick(x, y int, broadcaster func(string, interface{})) bool {
	if x < g.Dialog.X || x > g.Dialog.X+g.Dialog.W ||
		y < g.Dialog.Y || y > g.Dialog.Y+g.Dialog.H {
		// Click outside dialog - hide it
		g.Dialog.Visible = false
		return false
	}

	// Check if buy button clicked
	buyBtnX := g.Dialog.X + g.Dialog.W/2 - 40
	buyBtnY := g.Dialog.Y + g.Dialog.H - 50
	buyBtnW := 80
	buyBtnH := 30

	if x >= buyBtnX && x < buyBtnX+buyBtnW &&
		y >= buyBtnY && y < buyBtnY+buyBtnH {
		// Buy the unit
		req := types.BuyShopSlotReq{
			Slot:  g.Dialog.Slot,
			Nonce: fmt.Sprintf("buy-%d-%d", g.Dialog.Slot, 0), // TODO: generate proper nonce
		}
		broadcaster("BuyShopSlot", req)
		g.Dialog.Visible = false
		return true
	}

	return true
}

// showPurchaseDialog shows the info menu
func (g *ShopGrid) showPurchaseDialog(slot int, slotData *types.ShopSlot) {
	slotRect := g.rects[slot]
	unitInfo := g.getUnitInfo(slotData.UnitID)

	g.Dialog = &PurchaseDialog{
		Visible:  true,
		Slot:     slot,
		SlotData: slotData,
		UnitInfo: unitInfo,
		X:        slotRect.X + slotRect.W/2 - 150, // Center dialog on card
		Y:        slotRect.Y + 20,
		W:        300,
		H:        250,
	}
}

// Draw renders the shop with clean design
func (g *ShopGrid) Draw(screen *ebiten.Image, shopState *ShopState) {
	// Draw dialog first (behind cards)
	if g.Dialog.Visible {
		g.drawPurchaseDialog(screen)
	}

	// Draw cards
	for _, card := range g.rects {
		g.drawCleanCard(screen, card, shopState)
	}
}

// drawPurchaseDialog renders the info menu
func (g *ShopGrid) drawPurchaseDialog(screen *ebiten.Image) {
	dlg := g.Dialog

	if dlgUnitInfo := dlg.UnitInfo; dlgUnitInfo != nil {
		// Dimmed background
		vector.DrawFilledRect(screen, 0, 0, float32(dlg.W), float32(dlg.H),
			color.NRGBA{0, 0, 0, 120}, true)

		// Dialog background
		vector.DrawFilledRect(screen, float32(dlg.X), float32(dlg.Y),
			float32(dlg.W), float32(dlg.H), color.NRGBA{30, 37, 59, 240}, true)
		vector.StrokeRect(screen, float32(dlg.X), float32(dlg.Y),
			float32(dlg.W), float32(dlg.H), 2, color.NRGBA{71, 85, 105, 255}, true)

		// Large portrait in center-top
		portraitX := dlg.X + dlg.W/2
		portraitY := dlg.Y + 50
		portraitSize := 80

		vector.DrawFilledCircle(screen, float32(portraitX), float32(portraitY),
			float32(portraitSize/2+3), color.NRGBA{17, 24, 39, 200}, true)

		// Load actual unit image
		if g.imageLoader != nil {
			unitImg := g.imageLoader(dlgUnitInfo.Name)
			if img, ok := unitImg.(*ebiten.Image); ok && img != nil {
				op := &ebiten.DrawImageOptions{}
				bounds := img.Bounds()
				scale := float64(portraitSize-8) / float64(maxInt(bounds.Dx(), bounds.Dy()))
				op.GeoM.Scale(scale, scale)
				dx := portraitX - int(float64(bounds.Dx())*scale/2)
				dy := portraitY - int(float64(bounds.Dy())*scale/2)
				op.GeoM.Translate(float64(dx), float64(dy))
				screen.DrawImage(img, op)
			}
		}

		// Portrait border
		vector.StrokeCircle(screen, float32(portraitX), float32(portraitY),
			float32(portraitSize/2), 2, color.NRGBA{71, 85, 105, 255}, true)

		// Unit info text
		textY := portraitY + portraitSize + 25
		centerX := dlg.X + dlg.W/2

		// Features
		features := getUnitFeatures(dlgUnitInfo)
		featW := len(features) * 5
		text.Draw(screen, "Features:", basicfont.Face7x13, centerX-featW/2-20, textY,
			color.NRGBA{148, 163, 184, 255})
		text.Draw(screen, features, basicfont.Face7x13, centerX-featW/2-20, textY+20,
			color.NRGBA{248, 250, 252, 255})

		// Cost with gold icon
		costText := fmt.Sprintf("Cost: %d", dlgUnitInfo.Cost)
		costW := len(costText) * 6

		// Draw gold icon
		if g.imageLoader != nil {
			goldIcon := g.imageLoader("gold")
			if iconImg, ok := goldIcon.(*ebiten.Image); ok && iconImg != nil {
				iconOp := &ebiten.DrawImageOptions{}
				iconW, iconH := 14, 14
				iconOp.GeoM.Scale(float64(iconW)/float64(iconImg.Bounds().Dx()), float64(iconH)/float64(iconImg.Bounds().Dy()))
				iconOp.GeoM.Translate(float64(centerX-costW/2-20), float64(textY+47))
				screen.DrawImage(iconImg, iconOp)
			}
		}

		// Draw cost text
		text.Draw(screen, "Cost:", basicfont.Face7x13, centerX-costW/2+8, textY+50,
			color.NRGBA{245, 158, 11, 255})
		text.Draw(screen, costText[6:], basicfont.Face7x13, centerX-costW/2+40, textY+50,
			color.NRGBA{245, 158, 11, 255})

		// Buy button
		buyBtnX := dlg.X + dlg.W/2 - 40
		buyBtnY := dlg.Y + dlg.H - 50
		vector.DrawFilledRect(screen, float32(buyBtnX), float32(buyBtnY), 80, 30,
			color.NRGBA{34, 197, 94, 220}, true)
		vector.StrokeRect(screen, float32(buyBtnX), float32(buyBtnY), 80, 30, 2,
			color.NRGBA{22, 163, 74, 255}, true)

		buyText := "BUY"
		text.Draw(screen, buyText, basicfont.Face7x13, buyBtnX+35-len(buyText)*3, buyBtnY+20,
			color.NRGBA{255, 255, 255, 255})
	}
}

// drawCleanCard renders a clean, modern card
func (g *ShopGrid) drawCleanCard(screen *ebiten.Image, card ShopCardRect, shopState *ShopState) {
	x, y := card.X, card.Y
	slotData := card.Data

	// Determine card colors
	var bgColor, borderColor color.NRGBA
	isHovered := (card.Slot == shopState.GetHoveredSlot())

	if slotData == nil {
		// Empty card
		bgColor = color.NRGBA{30, 37, 59, 180}
		borderColor = color.NRGBA{71, 85, 105, 200}
	} else if slotData.Sold {
		// Sold state
		bgColor = color.NRGBA{31, 41, 55, 180}
		borderColor = color.NRGBA{107, 114, 128, 150}
	} else if shopState.IsPending(card.Slot) {
		// Buying state
		if isHovered {
			borderColor = color.NRGBA{0, 185, 241, 220}
		} else {
			borderColor = color.NRGBA{34, 197, 94, 220}
		}
	} else if isHovered {
		// Hover state
		borderColor = color.NRGBA{59, 130, 246, 220}
	} else {
		// Normal state
		borderColor = color.NRGBA{71, 85, 105, 160}
	}

	// Shadow
	vector.StrokeRect(screen, float32(x-3), float32(y+3),
		float32(card.W), float32(card.H), 1, color.NRGBA{0, 0, 0, 50}, true)

	// Main background
	vector.DrawFilledRect(screen, float32(x), float32(y),
		float32(card.W), float32(card.H), bgColor, true)

	// Borders
	borderThickness := float32(1)
	if isHovered {
		borderThickness = 2
		vector.StrokeRect(screen, float32(x-1), float32(y-1),
			float32(card.W+2), float32(card.H+2), borderThickness, borderColor, true)
	}

	// Champion accent line
	if slotData != nil && slotData.IsChampion {
		vector.DrawFilledRect(screen, float32(x), float32(y), float32(card.W), 3,
			color.NRGBA{245, 158, 11, 255}, true)
	}

	// Content
	if slotData != nil {
		g.drawCardContent(screen, card, shopState)
	} else {
		g.drawEmptyCard(screen, card)
	}
}

// drawCardContent renders the unit portrait - clean and centered
func (g *ShopGrid) drawCardContent(screen *ebiten.Image, card ShopCardRect, shopState *ShopState) {
	slotData := card.Data
	if slotData == nil {
		return
	}

	centerX := card.X + card.W/2
	centerY := card.Y + card.H/2
	portraitSize := 80 // Much bigger as requested

	// Portrait background circle
	vector.DrawFilledCircle(screen, float32(centerX), float32(centerY),
		float32(portraitSize/2+2), color.NRGBA{17, 24, 39, 220}, true)

	// Load and draw unit image
	if g.imageLoader != nil {
		unitImg := g.imageLoader(slotData.UnitID)
		if img, ok := unitImg.(*ebiten.Image); ok && img != nil {
			op := &ebiten.DrawImageOptions{}
			bounds := img.Bounds()
			scale := float64(portraitSize-8) / float64(maxInt(bounds.Dx(), bounds.Dy()))
			op.GeoM.Scale(scale, scale)
			dx := centerX - int(float64(bounds.Dx())*scale/2)
			dy := centerY - int(float64(bounds.Dy())*scale/2)
			op.GeoM.Translate(float64(dx), float64(dy))
			screen.DrawImage(img, op)
		}
	}

	// Portrait border
	borderW := float32(1)
	if card.Slot == shopState.GetHoveredSlot() {
		borderW = 2
	}
	vector.StrokeCircle(screen, float32(centerX), float32(centerY),
		float32(portraitSize/2), borderW, hexToColor(GOLD_ACCENT), true)

	if shopState.IsPending(card.Slot) {
		vector.StrokeCircle(screen, float32(centerX), float32(centerY),
			float32(portraitSize/2+3), borderW, hexToColor(GOLD_ACCENT), true)
	}

	// Show price at bottom of card
	if !slotData.Sold {
		g.drawPriceDisplay(screen, card, slotData)
	}
}

// drawPriceDisplay shows gold icon and price on the card
func (g *ShopGrid) drawPriceDisplay(screen *ebiten.Image, card ShopCardRect, slotData *types.ShopSlot) {
	priceY := card.Y + card.H - 24
	priceCenterX := card.X + card.W/2

	// Price number (centered)
	priceText := fmt.Sprintf("%d", slotData.PriceGold)
	priceWidth := len(priceText) * 7            // Approximate character width
	goldColor := color.NRGBA{245, 158, 11, 255} // Gold color

	// Combined icon and text width for centering
	iconSize := 12
	iconSpacing := 6 // Spacing between icon and text
	totalWidth := iconSize + iconSpacing + priceWidth
	leftEdge := priceCenterX - totalWidth/2

	// Gold icon positioned at the left edge
	goldIconX := leftEdge
	if g.imageLoader != nil {
		goldIcon := g.imageLoader("gold")
		if iconImg, ok := goldIcon.(*ebiten.Image); ok && iconImg != nil {
			op := &ebiten.DrawImageOptions{}
			iconW, iconH := iconSize, iconSize
			op.GeoM.Scale(float64(iconW)/float64(iconImg.Bounds().Dx()), float64(iconH)/float64(iconImg.Bounds().Dy()))
			op.GeoM.Translate(float64(goldIconX), float64(priceY-6)) // Center Y position
			screen.DrawImage(iconImg, op)
		}
	}

	// Price text positioned after the icon with proper spacing
	priceX := goldIconX + iconSize + iconSpacing
	text.Draw(screen, priceText, basicfont.Face7x13, priceX, priceY+4, goldColor)
}

// drawEmptyCard renders empty slot
func (g *ShopGrid) drawEmptyCard(screen *ebiten.Image, card ShopCardRect) {
	centerX := card.X + card.W/2
	centerY := card.Y + card.H/2

	text.Draw(screen, "🔍", basicfont.Face7x13, centerX-8, centerY-5,
		color.NRGBA{100, 116, 139, 160})
}

// Utility functions
func hexToColor(hex uint32) color.NRGBA {
	return color.NRGBA{
		R: uint8(hex >> 16),
		G: uint8(hex >> 8),
		B: uint8(hex),
		A: 255,
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
