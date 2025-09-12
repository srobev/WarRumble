package shop

import (
	"fmt"
	"image/color"
	"log"
	"rumble/shared/game/types"
	"rumble/shared/protocol"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

// ThemedUI interface for calling themed drawing methods
type ThemedUI interface {
	DrawThemedButtonWithStyle(screen *ebiten.Image, x, y, width, height int, label string, state int, isPvpButton bool)
}

// ShopView orchestrates the complete shop UI experience
type ShopView struct {
	state       *ShopState
	grid        *ShopGrid
	rerollBtn   ButtonState
	isVisible   bool
	fantasyUI   interface{}              // FantasyUI interface for consistent styling
	imageLoader func(string) interface{} // Function to load unit images
}

// ButtonState represents a clickable button
type ButtonState struct {
	X, Y, W, H int
	Text       string
	Hovered    bool
	Clicked    bool
}

// UnitImageLoader interface to avoid circular dependencies
type UnitImageLoader interface {
	ensureMiniImageByName(name string) *ebiten.Image
}

// createImageLoader creates an image loader function from a loader interface
func createImageLoader(loader interface{}) func(string) interface{} {
	// First try direct function (current implementation)
	if fn, ok := loader.(func(string) interface{}); ok {
		return fn
	}

	// Try interface with method
	if loaderObj, ok := loader.(UnitImageLoader); ok && loaderObj != nil {
		return func(name string) interface{} {
			return loaderObj.ensureMiniImageByName(name)
		}
	}

	// Fallback
	return func(iconName string) interface{} {
		return nil
	}
}

// NewShopView creates a new shop view with all components
func NewShopView(screenWidth int, fantasyUI interface{}, game interface{}) *ShopView {
	imageLoader := createImageLoader(game)
	grid := NewShopGrid(screenWidth, imageLoader)
	state := NewShopState()

	// Position reroll button at bottom of screen
	rerollBtn := ButtonState{
		X:    screenWidth/2 - 60,     // Center horizontally
		Y:    protocol.ScreenH - 160, // Just above bottom bar
		W:    120,
		H:    35,
		Text: "Reroll (100g)", // TODO: Dynamic price
	}

	return &ShopView{
		state:       state,
		grid:        grid,
		rerollBtn:   rerollBtn,
		isVisible:   false,
		fantasyUI:   fantasyUI,
		imageLoader: imageLoader,
	}
}

// Show makes the shop visible and requests initial data
func (v *ShopView) Show(broadcaster func(string, interface{})) {
	v.isVisible = true
	v.state.SetRoll(types.ShopRoll{}) // Clear current data while loading
	// Request initial shop data
	if broadcaster != nil {
		broadcaster("GetShopRoll", types.GetShopRollReq{})
	}
}

// Hide makes the shop invisible
func (v *ShopView) Hide() {
	v.isVisible = false
}

// IsVisible returns whether the shop is currently visible
func (v *ShopView) IsVisible() bool {
	return v.isVisible
}

// Update handles mouse input and hover detection
func (v *ShopView) Update(mx, my int, broadcaster func(string, interface{})) {
	if !v.isVisible {
		return
	}

	// Update hover states
	slot := v.grid.GetSlotAtPoint(mx, my)
	v.state.SetHoveredSlot(slot)

	// Update button hover
	v.rerollBtn.Hovered = mx >= v.rerollBtn.X && mx < v.rerollBtn.X+v.rerollBtn.W &&
		my >= v.rerollBtn.Y && my < v.rerollBtn.Y+v.rerollBtn.H
}

// HandleClick handles mouse click events at the given coordinates
func (v *ShopView) HandleClick(mx, my int, broadcaster func(string, interface{})) {
	if !v.isVisible || broadcaster == nil {
		return
	}

	// Handle card clicks
	slot := v.grid.GetSlotAtPoint(mx, my)
	if !v.state.IsPending(slot) && slot >= 0 {
		roll := v.state.GetRoll()
		if slot < len(roll.Slots) {
			slotData := &roll.Slots[slot]
			if !slotData.Sold {
				// Send purchase request
				req := types.BuyShopSlotReq{
					Slot:  slot,
					Nonce: fmt.Sprintf("buy-%d-%d", slot, time.Now().UnixMilli()),
				}
				v.state.SetPending(slot, true)
				broadcaster("BuyShopSlot", req)
			}
		}
	}

	// Handle reroll button click
	if v.rerollBtn.Hovered {
		req := types.RerollShopReq{
			Nonce: fmt.Sprintf("reroll-%d", time.Now().UnixMilli()),
		}
		broadcaster("RerollShop", req)
	}
}

// Draw renders the complete shop interface
func (v *ShopView) Draw(screen *ebiten.Image) {
	if !v.isVisible {
		return
	}

	// Draw shop title at top
	v.drawTitle(screen)

	// Draw the shop grid
	v.grid.Draw(screen, v.state)

	// Draw reroll button
	v.drawRerollButton(screen)
}

// drawTitle renders the modern shop title header
func (v *ShopView) drawTitle(screen *ebiten.Image) {
	titleY := float32(GRID_TOP - 60)
	titleHeight := float32(50)

	// Modern dark gradient background
	for y := int(titleY); y < int(titleY+titleHeight); y++ {
		progress := float32(y-int(titleY)) / titleHeight
		// Dark modern gradient from top to bottom
		r := uint8(10 + progress*10) // 10-20 (very dark)
		g := uint8(15 + progress*10) // 15-25
		b := uint8(20 + progress*20) // 20-40 (dark blue)
		alpha := uint8((1 - progress*0.2) * 220)

		vector.DrawFilledRect(screen, 0, float32(y), float32(protocol.ScreenW), 1,
			color.NRGBA{r, g, b, alpha}, true)
	}

	// Clean bottom border line
	vector.StrokeRect(screen, 0, titleY+titleHeight-1, float32(protocol.ScreenW), 1, 1,
		color.NRGBA{51, 65, 85, 160}, true)

	// Modern title with clean typography design
	shopTitle := "SHOPPING MALL"
	titleX := (protocol.ScreenW - 200) / 2
	titleYDraw := int(titleY + 12)

	// Subtle glow effect behind title
	glowSize := 2
	for i := 0; i < glowSize; i++ {
		text.Draw(screen, shopTitle, basicfont.Face7x13, titleX-i, titleYDraw-i,
			color.NRGBA{245, 158, 11, uint8(50 - glowSize*i)}) // Gold glow
	}

	// Main title in modern clean font
	text.Draw(screen, shopTitle, basicfont.Face7x13, titleX, titleYDraw,
		color.NRGBA{248, 250, 252, 255}) // Modern white

	// Modern subtitle
	subtitle := "Premium Wargaming Collection"
	subtitleX := (protocol.ScreenW - 300) / 2
	subtitleY := titleYDraw + 18

	// Subtle background for subtitle
	subtitleW := len(subtitle)*7 + 20
	vector.DrawFilledRect(screen, float32(subtitleX-10), float32(subtitleY-3),
		float32(subtitleW), 16, color.NRGBA{17, 24, 39, 120}, true)

	// Subtitle text
	text.Draw(screen, subtitle, basicfont.Face7x13, subtitleX, subtitleY,
		color.NRGBA{148, 163, 184, 255}) // Modern gray

	// Modern corner accent elements
	accentSize := 8
	vector.DrawFilledCircle(screen, float32(protocol.ScreenW-20), float32(titleY+15),
		float32(accentSize/2), color.NRGBA{245, 158, 11, 180}, true) // Gold accent
	vector.DrawFilledCircle(screen, float32(20), float32(titleY+15),
		float32(accentSize/2), color.NRGBA{245, 158, 11, 180}, true) // Gold accent
}

// drawRerollButton renders the reroll button with gold icon
func (v *ShopView) drawRerollButton(screen *ebiten.Image) {
	btn := v.rerollBtn

	// Determine button state
	var state int
	if btn.Hovered {
		state = 1 // ButtonHover
	} else {
		state = 0 // ButtonNormal
	}

	// Use FantasyUI for consistent styling with bottom bar
	if ui, ok := v.fantasyUI.(ThemedUI); ok {
		ui.DrawThemedButtonWithStyle(screen, btn.X, btn.Y, btn.W, btn.H, btn.Text, state, true)
	} else {
		// Fallback to basic styling with gold icon
		var btnColor color.NRGBA
		if btn.Hovered {
			btnColor = color.NRGBA{50, 70, 50, 255}
		} else {
			btnColor = color.NRGBA{30, 50, 30, 255}
		}

		ebitenutil.DrawRect(screen, float64(btn.X), float64(btn.Y), float64(btn.W), float64(btn.H), btnColor)
		vector.StrokeRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), 2,
			color.NRGBA{100, 140, 100, 255}, true)
	}

	// Draw gold icon and price
	if v.imageLoader != nil {
		goldIcon := v.imageLoader("gold")
		if iconImg, ok := goldIcon.(*ebiten.Image); ok && iconImg != nil {
			iconOp := &ebiten.DrawImageOptions{}
			iconW, iconH := 12, 12
			iconOp.GeoM.Scale(float64(iconW)/float64(iconImg.Bounds().Dx()), float64(iconH)/float64(iconImg.Bounds().Dy()))
			iconOp.GeoM.Translate(float64(btn.X+8), float64(btn.Y+btn.H/2-6))
			screen.DrawImage(iconImg, iconOp)
		}
	}

	// Draw price number
	priceText := "100"
	priceX := btn.X + btn.W/2 + 5
	priceY := btn.Y + btn.H/2 + 4
	textColor := color.NRGBA{200, 255, 200, 255}
	text.Draw(screen, priceText, basicfont.Face7x13, priceX, priceY, textColor)
}

// UpdateRoll handles incoming shop roll data
func (v *ShopView) UpdateRoll(roll types.ShopRoll) {
	v.state.SetRoll(roll)
	v.grid.UpdateLayout(roll)
}

// UpdateGold handles gold synchronizations
func (v *ShopView) UpdateGold(gold int64) {
	// Update the gold in the shop state for display
	v.state.SetCurrentGold(gold)
}

// ClearPending clears all pending purchase states
func (v *ShopView) ClearPending() {
	v.state.ClearAllPending()
}

// SetPending marks a specific slot as pending
func (v *ShopView) SetPending(slot int, pending bool) {
	v.state.SetPending(slot, pending)
}

// UpdateShopRoll handles incoming shop roll data
func (v *ShopView) UpdateShopRoll(roll types.ShopRoll) {
	v.state.SetRoll(roll)
	v.grid.UpdateLayout(roll)
}

// OnBuyResult handles the result of a shop purchase
func (v *ShopView) OnBuyResult(result protocol.BuyShopResult) {
	// Clear pending state for this slot
	v.state.SetPending(result.Slot, false)

	// Mark the slot as sold in the roll
	roll := v.state.GetRoll()
	if result.Slot >= 0 && result.Slot < len(roll.Slots) {
		roll.Slots[result.Slot].Sold = true
		v.state.SetRoll(roll)
		v.grid.UpdateLayout(roll)
	}

	// Simple log showing total shards - just for verification
	log.Printf("Shop purchase successful: %d %s (total shards: %d)",
		result.Gold, result.UnitID, result.Shards)
}

// getCurrentGold returns current player gold for display
func (v *ShopView) getCurrentGold() int64 {
	// Try to return local cached gold first
	if gold := v.state.GetCurrentGold(); gold > 0 {
		return gold
	}
	// If no cached gold, return a default value or indication that it needs sync
	return 0
}
