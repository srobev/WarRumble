package shop

import (
	"log"
	"rumble/shared/game/types"
)

// ShopActions handles all shop-related network communication
type ShopActions struct {
	view *ShopView
	gold int64 // Cached gold value for UI
}

// NewShopActions creates a new shop actions instance
func NewShopActions(view *ShopView) *ShopActions {
	return &ShopActions{
		view: view,
		gold: 0,
	}
}

// SendGetShopRoll requests the current shop roll from server
func (a *ShopActions) SendGetShopRoll(broadcaster func(string, interface{})) {
	if broadcaster == nil {
		return
	}

	req := types.GetShopRollReq{}
	broadcaster("GetShopRoll", req)
	log.Println("ShopActions: Requested shop roll")
}

// SendBuyShopSlot sends a purchase request for the given slot
func (a *ShopActions) SendBuyShopSlot(slot int, broadcaster func(string, interface{})) {
	if broadcaster == nil || slot < 0 {
		return
	}

	req := types.BuyShopSlotReq{
		Slot:  slot,
		Nonce: a.generateNonce(), // Generate secure nonce
	}
	broadcaster("BuyShopSlot", req)
	log.Printf("ShopActions: Sent purchase request for slot %d", slot)
}

// SendRerollShop requests a shop reroll
func (a *ShopActions) SendRerollShop(broadcaster func(string, interface{})) {
	if broadcaster == nil {
		return
	}

	// Check if player has enough gold (simulate client-side validation)
	if a.gold < 100 { // TODO: Get actual reroll cost from server
		log.Println("ShopActions: Insufficient gold for reroll")
		return
	}

	req := types.RerollShopReq{
		Nonce: a.generateNonce(),
	}
	broadcaster("RerollShop", req)
	log.Println("ShopActions: Sent reroll request")
}

// HandleShopRollSynced processes incoming shop roll updates
func (a *ShopActions) HandleShopRollSynced(resp types.ShopRollSynced) {
	log.Printf("ShopActions: Received shop roll with %d slots", len(resp.Roll.Slots))
	a.view.UpdateRoll(resp.Roll)
}

// HandleBuyShopResult processes purchase responses
func (a *ShopActions) HandleBuyShopResult(result types.BuyShopResult) {
	log.Printf("ShopActions: Purchase result - slot %d, unit %s, shards %d, gold %d",
		result.Slot, result.UnitID, result.Shards, result.Gold)

	// Update player gold
	a.gold = result.Gold
	a.view.UpdateGold(a.gold)

	// Mark purchase as completed
	a.view.SetPending(result.Slot, false)

	// The UI will automatically update when receiving the ShopRollSynced
	// TODO: Show purchase feedback message
}

// HandleGoldSynced processes gold updates
func (a *ShopActions) HandleGoldSynced(gold int64) {
	if gold != a.gold {
		log.Printf("ShopActions: Gold updated to %d", gold)
		a.gold = gold
		a.view.UpdateGold(gold)
	}
}

// getCurrentGold returns the current gold amount
func (a *ShopActions) GetGold() int64 {
	return a.gold
}

// generateNonce generates a simple nonce for requests (TODO: Make more secure)
func (a *ShopActions) generateNonce() string {
	// TODO: Use proper random UUID
	return "SIMPLE_NONCE_123" // Simple placeholder for now
}
