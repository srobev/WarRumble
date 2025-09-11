package shop

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"rumble/server/account"
	"rumble/shared/game/types"
	"rumble/shared/protocol"
)

const SLOT_COUNT = 9

type Service struct {
	unitCatalog     func() ([]types.UnitMeta, []types.UnitMeta)
	perkCatalog     func() []string           // list of units that have perks
	PerkCatalogFunc func(string) []types.Perk // get perks for unit
}

func NewService() *Service {
	return &Service{
		unitCatalog: func() ([]types.UnitMeta, []types.UnitMeta) {
			return types.ListMinis(), types.ListChampions()
		},
		perkCatalog: func() []string {
			// Units with perks
			return []string{"Sorceress Glacia", "Swordsman"}
		},
		PerkCatalogFunc: func(unitID string) []types.Perk {
			// Since different package, stub for now
			// Will be set later
			return []types.Perk{}
		},
	}
}

// GenerateRoll81 builds 81 slots mixing minis & champions
func GenerateRoll81(seed int64, minis, champs []types.UnitMeta) types.ShopRoll {
	// Set deterministic seed for this roll
	r := rand.New(rand.NewSource(seed))

	var slots []types.ShopSlot
	for i := 0; i < SLOT_COUNT; i++ {
		var unit types.UnitMeta
		var price int64

		// 70/30 split between minis and champions
		if r.Float32() < 0.7 && len(minis) > 0 {
			// Pick mini
			unit = minis[r.Intn(len(minis))]
			price = types.ShopPriceMiniGold
		} else if len(champs) > 0 {
			// Pick champion
			unit = champs[r.Intn(len(champs))]
			price = types.ShopPriceChampionGold
		} else if len(minis) > 0 {
			// Fallback to minis if no champions
			unit = minis[r.Intn(len(minis))]
			price = types.ShopPriceMiniGold
		} else {
			continue
		}

		slot := types.ShopSlot{
			Slot:       i,
			UnitID:     unit.ID,
			IsChampion: unit.IsChampion,
			PriceGold:  price,
			Sold:       false,
		}
		slots = append(slots, slot)
	}

	return types.ShopRoll{
		Slots:   slots,
		Version: 1,
	}
}

// GenerateRollSeed creates a seed for roll generation using time + account state
func GenerateRollSeed(accountID string, timestamp int64) int64 {
	// Use account ID hash + timestamp for deterministic but varied rolls
	hash := int64(0)
	for _, c := range accountID {
		hash = hash*31 + int64(c)
	}
	return hash + timestamp/3600 // Change seed every hour
}

// getOrCreateRoll ensures account has a roll, generating one if needed
func (s *Service) getOrCreateRoll(playerID string, account *account.Account) (types.ShopRoll, error) {
	// If roll exists and is correct size, return it
	if len(account.Shop.Roll) == SLOT_COUNT && account.Shop.Version == 1 {
		roll := types.ShopRoll{
			Slots:   account.Shop.Roll,
			Version: account.Shop.Version,
		}
		return roll, nil
	}

	// Generate new roll
	minis, champs := s.unitCatalog()
	now := time.Now().Unix()
	seed := GenerateRollSeed(playerID, now)

	roll := GenerateRoll81(seed, minis, champs)

	// Update account
	account.Shop.Roll = roll.Slots
	account.Shop.Version = roll.Version
	account.Shop.LastReroll = now

	return roll, nil
}

// addPerkOffers adds up to 2 perk offers for units with available slots
func (s *Service) addPerkOffers(roll *types.ShopRoll, acc *account.Account) {
	perkUnits := s.perkCatalog()
	var candidates []string

	for _, unitID := range perkUnits {
		progress, exists := acc.Progress[unitID]
		if !exists || progress == nil {
			continue
		}
		if progress.Rarity == 0 {
			continue // Common has 0 slots
		}
		// Check if has purchased perks less than slots
		if len(progress.PerksUnlocked) < int(progress.Rarity) {
			candidates = append(candidates, unitID)
		}
	}

	if len(candidates) == 0 {
		return // no perks available
	}

	// Add up to 2 perk offers, replacing random unit offers if any
	r := rand.New(rand.NewSource(time.Now().Unix()))
	maxPerks := 2
	if maxPerks > len(candidates) {
		maxPerks = len(candidates)
	}

	for i := 0; i < maxPerks; i++ {
		unitID := candidates[r.Intn(len(candidates))]
		candidates = remove(candidates, unitID)

		// Find a unit slot to replace or add at end if possible
		var slotIdx int = -1
		for j := range roll.Slots {
			if roll.Slots[j].OfferType == "" { // unit offer
				slotIdx = j
				break
			}
		}
		if slotIdx == -1 {
			continue // no unit slot to replace
		}

		// Pick random unpurchased perk
		availablePerks := []string{}
		for _, u := range s.PerkCatalogFunc(unitID) {
			found := false
			for _, pur := range acc.Progress[unitID].PerksUnlocked {
				if string(pur) == string(u.ID) {
					found = true
					break
				}
			}
			if !found {
				availablePerks = append(availablePerks, string(u.ID))
			}
		}
		if len(availablePerks) == 0 {
			continue
		}
		perkID := availablePerks[r.Intn(len(availablePerks))]

		// Replace slot
		roll.Slots[slotIdx].OfferType = "perk"
		roll.Slots[slotIdx].UnitID = unitID
		roll.Slots[slotIdx].PerkID = perkID
		roll.Slots[slotIdx].PriceGold = 250
		roll.Slots[slotIdx].IsChampion = false // perks are not champions
	}
}

// remove helper
func remove(slice []string, item string) []string {
	for i, v := range slice {
		if v == item {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

// HandleGetShopRoll processes shop roll requests
func (s *Service) HandleGetShopRoll(playerID string, broadcaster func(eventType string, event interface{})) error {
	acc, err := account.LoadAccount(playerID)
	if err != nil {
		return fmt.Errorf("failed to load account: %w", err)
	}

	roll, err := s.getOrCreateRoll(playerID, acc)
	if err != nil {
		return err
	}

	// Add perk offers if eligible
	s.addPerkOffers(&roll, acc)

	// Update account with the new roll including perks
	acc.Shop.Roll = roll.Slots

	// Save account with new roll if needed
	if err := account.SaveAccount(acc); err != nil {
		return fmt.Errorf("failed to save account: %w", err)
	}

	// Broadcast roll sync
	if broadcaster != nil {
		broadcaster("ShopRollSynced", protocol.ShopRollSynced{Roll: roll})
	}

	return nil
}

// HandleRerollShop processes reroll requests
func (s *Service) HandleRerollShop(playerID string, req types.RerollShopReq, broadcaster func(eventType string, event interface{})) error {
	// Check for replay (nonce seen)
	acc, err := account.LoadAccount(playerID)
	if err != nil {
		return fmt.Errorf("failed to load account: %w", err)
	}

	if acc.Shop.NonceSeen == nil {
		acc.Shop.NonceSeen = make(map[string]bool)
	}

	if acc.Shop.NonceSeen[req.Nonce] {
		return fmt.Errorf("duplicate reroll request")
	}

	// Generate new roll
	minis, champs := s.unitCatalog()
	now := time.Now().Unix()
	seed := GenerateRollSeed(playerID, now+1) // Different seed

	roll := GenerateRoll81(seed, minis, champs)

	// Add perk offers if eligible
	s.addPerkOffers(&roll, acc)

	// Update account
	acc.Shop.Roll = roll.Slots
	acc.Shop.LastReroll = now
	acc.Shop.Sold = make(map[int]bool) // Clear sold items
	acc.Shop.NonceSeen[req.Nonce] = true

	// Save account (non-fatal if it fails)
	if saveErr := account.SaveAccount(acc); saveErr != nil {
		log.Printf("REROLL: Failed to save account but continuing: %v", saveErr)
	}

	// Broadcast updated roll
	if broadcaster != nil {
		broadcaster("ShopRollSynced", protocol.ShopRollSynced{Roll: roll})
	}

	return nil
}

// HandleBuyShopSlot processes slot purchase requests
func (s *Service) HandleBuyShopSlot(playerID string, req types.BuyShopSlotReq, broadcaster func(eventType string, event interface{})) error {
	acc, err := account.LoadAccount(playerID)
	if err != nil {
		return fmt.Errorf("failed to load account: %w", err)
	}

	// Validate slot
	if req.Slot < 0 || req.Slot >= SLOT_COUNT {
		return fmt.Errorf("invalid slot: %d", req.Slot)
	}

	// Check for replay
	if acc.Shop.NonceSeen == nil {
		acc.Shop.NonceSeen = make(map[string]bool)
	}

	if acc.Shop.NonceSeen[req.Nonce] {
		return fmt.Errorf("duplicate buy request")
	}

	// Find slot
	var slot *types.ShopSlot
	slotFound := false
	for i := range acc.Shop.Roll {
		if acc.Shop.Roll[i].Slot == req.Slot {
			slot = &acc.Shop.Roll[i]
			slotFound = true
			break
		}
	}

	if !slotFound {
		return fmt.Errorf("slot not found: %d", req.Slot)
	}

	// Check if already sold
	if acc.Shop.Sold == nil {
		acc.Shop.Sold = make(map[int]bool)
	}

	if acc.Shop.Sold[req.Slot] || slot.Sold {
		return fmt.Errorf("slot already sold: %d", req.Slot)
	}

	// Check gold (log for debugging)
	log.Printf("Account %s checking gold: %d < %d?", playerID, acc.Gold, slot.PriceGold)
	if acc.Gold < slot.PriceGold {
		if broadcaster != nil {
			broadcaster("Error", protocol.Error{Code: "INSUFFICIENT_FUNDS"})
		}
		return fmt.Errorf("insufficient funds: has %d, needs %d", acc.Gold, slot.PriceGold)
	}

	var progress *types.UnitProgress
	var costPerRank int

	// Handle perk purchase
	if slot.OfferType == "perk" {
		// Load unit progress from account
		var err error
		progress, err = acc.LoadUnitProgress(slot.UnitID)
		if err != nil {
			return fmt.Errorf("failed to load progress for perk unit: %w", err)
		}

		// Check if perk is eligible (slots available)
		if len(progress.PerksUnlocked) >= int(progress.Rarity) {
			return fmt.Errorf("no available perk slots for unit %s", slot.UnitID)
		}

		// Check if perk already purchased
		for _, perk := range progress.PerksUnlocked {
			if string(perk) == slot.PerkID {
				return fmt.Errorf("perk %s already purchased", slot.PerkID)
			}
		}

		// Add perk to account
		progress.PerksUnlocked = append(progress.PerksUnlocked, types.PerkID(slot.PerkID))

		// Deduct gold
		acc.Gold -= slot.PriceGold

		// Mark as sold
		slot.Sold = true
		acc.Shop.Sold[req.Slot] = true
		acc.Shop.NonceSeen[req.Nonce] = true

		// Save progress using account service
		if err := acc.SaveUnitProgress(progress, playerID); err != nil {
			return fmt.Errorf("failed to save progress: %w", err)
		}

		costPerRank = 0
	} else {
		// Unit purchase logic (unchanged)
		// Get unit progress
		meta := types.GetUnitMeta(slot.UnitID)
		if meta == nil {
			return fmt.Errorf("unit not found: %s", slot.UnitID)
		}

		// Load/create progress using account
		progress, err = acc.LoadUnitProgress(slot.UnitID)
		if err != nil {
			return fmt.Errorf("failed to load progress: %w", err)
		}

		// Set rarity from meta
		progress.Rarity = meta.Rarity

		// Calculate shard cost for next rank using shared progression logic
		costPerRank = types.GetUpgradeCost(progress.Rank)

		// Add one shard using account service
		_, shardErr := acc.AddShards(slot.UnitID, 1)
		if shardErr != nil {
			return fmt.Errorf("failed to add shards to unit: %w", shardErr)
		}

		// Reload progress after AddShards to get the updated data
		progress, err = acc.LoadUnitProgress(slot.UnitID)
		if err != nil {
			return fmt.Errorf("failed to reload progress: %w", err)
		}

		// Deduct gold
		acc.Gold -= slot.PriceGold

		// Mark as sold
		slot.Sold = true
		acc.Shop.Sold[req.Slot] = true
		acc.Shop.NonceSeen[req.Nonce] = true

		// Save account (reinforce the save)
		if err := account.SaveAccount(acc); err != nil {
			return fmt.Errorf("failed to save account: %w", err)
		}
	}

	// Broadcast events
	if broadcaster != nil {
		// Gold sync
		broadcaster("GoldSynced", protocol.GoldSynced{Gold: acc.Gold})

		// Shop roll update
		roll := types.ShopRoll{
			Slots:   acc.Shop.Roll,
			Version: acc.Shop.Version,
		}
		broadcaster("ShopRollSynced", protocol.ShopRollSynced{Roll: roll})

		// Buy result
		result := protocol.BuyShopResult{
			Slot:      req.Slot,
			UnitID:    slot.UnitID,
			Gold:      acc.Gold,
			Shards:    progress.ShardsOwned,
			Rank:      progress.Rank,
			Threshold: costPerRank,
		}
		broadcaster("BuyShopResult", result)
	}

	return nil
}
