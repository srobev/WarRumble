package shop

import (
	"rumble/shared/game/types"
)

// ShopState manages local shop state and synchronization
type ShopState struct {
	currentRoll    types.ShopRoll
	pendingCards   map[int]bool // slot -> is pending purchase
	hoveredCardIdx int          // -1 if none, slot index otherwise
	scrollOffset   int          // for potential vertical scrolling
	needsRedraw    bool
	currentGold    int64 // player's current shop gold
}

// NewShopState creates a new shop state instance
func NewShopState() *ShopState {
	return &ShopState{
		pendingCards:   make(map[int]bool),
		hoveredCardIdx: -1,
		scrollOffset:   0,
		needsRedraw:    true,
	}
}

// SetRoll updates the shop roll and marks for redraw
func (s *ShopState) SetRoll(roll types.ShopRoll) {
	s.currentRoll = roll
	s.needsRedraw = true
}

// IsPending checks if a slot is pending purchase
func (s *ShopState) IsPending(slot int) bool {
	return s.pendingCards[slot]
}

// SetPending marks a slot as pending purchase
func (s *ShopState) SetPending(slot int, pending bool) {
	if pending {
		s.pendingCards[slot] = true
	} else {
		delete(s.pendingCards, slot)
	}
	s.needsRedraw = true
}

// ClearAllPending clears all pending operations
func (s *ShopState) ClearAllPending() {
	s.pendingCards = make(map[int]bool)
	s.needsRedraw = true
}

// SetHoveredSlot sets the currently hovered slot
func (s *ShopState) SetHoveredSlot(slot int) {
	if s.hoveredCardIdx != slot {
		s.hoveredCardIdx = slot
		s.needsRedraw = true
	}
}

// GetHoveredSlot returns the currently hovered slot
func (s *ShopState) GetHoveredSlot() int {
	return s.hoveredCardIdx
}

// GetRoll returns the current shop roll
func (s *ShopState) GetRoll() types.ShopRoll {
	return s.currentRoll
}

// NeedsRedraw returns whether the UI needs to be redrawn
func (s *ShopState) NeedsRedraw() bool {
	return s.needsRedraw
}

// MarkRedrawHandled clears the redraw flag
func (s *ShopState) MarkRedrawHandled() {
	s.needsRedraw = false
}

// GetCurrentGold returns the player's current shop gold
func (s *ShopState) GetCurrentGold() int64 {
	return s.currentGold
}

// SetCurrentGold sets the player's current shop gold
func (s *ShopState) SetCurrentGold(gold int64) {
	s.currentGold = gold
	s.needsRedraw = true
}
