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

// NewShopView creates a new shop view with all components
func NewShopView(screenWidth int, fantasyUI interface{}, imageLoader func(string) interface{}) *ShopView {
	grid := NewShopGrid(screenWidth)
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
	v.grid.Draw(screen, v.state, v.fantasyUI, v.imageLoader)

	// Draw reroll button
	v.drawRerollButton(screen)
}

// drawTitle renders the shop title header
func (v *ShopView) drawTitle(screen *ebiten.Image) {
	// Draw themed background for title area matching army tab panel
	vector.DrawFilledRect(screen, float32(0), float32(GRID_TOP-50), float32(protocol.ScreenW), 40,
		color.NRGBA{32, 34, 48, 200}, true) // Theme.CardBackground with transparency
	vector.StrokeRect(screen, 0, float32(GRID_TOP-50), float32(protocol.ScreenW), 40, 1,
		color.NRGBA{100, 100, 100, 255}, true) // Theme.Border

}

// drawRerollButton renders the reroll button
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
		// Fallback to basic styling
		var btnColor color.NRGBA
		if btn.Hovered {
			btnColor = color.NRGBA{50, 70, 50, 255}
		} else {
			btnColor = color.NRGBA{30, 50, 30, 255}
		}

		ebitenutil.DrawRect(screen, float64(btn.X), float64(btn.Y), float64(btn.W), float64(btn.H), btnColor)
		vector.StrokeRect(screen, float32(btn.X), float32(btn.Y), float32(btn.W), float32(btn.H), 2,
			color.NRGBA{100, 140, 100, 255}, true)

		textColor := color.NRGBA{200, 255, 200, 255}
		text.Draw(screen, btn.Text, basicfont.Face7x13,
			btn.X+(btn.W-len(btn.Text)*7)/2, btn.Y+btn.H/2+4,
			textColor)
	}
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

	// Could show a success toast/message here
	log.Printf("Shop purchase successful: %d %s (shards: %d/%d)",
		result.Gold, result.UnitID, result.Shards, result.Threshold)
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
