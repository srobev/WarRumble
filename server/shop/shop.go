package shop

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"rumble/server/account"
	"rumble/server/progression"
	"rumble/shared/game/types"
	"rumble/shared/protocol"
)

const SLOT_COUNT = 9

type Service struct {
	progressionService *progression.Service
	unitCatalog        func() ([]types.UnitMeta, []types.UnitMeta)
}

func NewService(progressionService *progression.Service) *Service {
	return &Service{
		progressionService: progressionService,
		unitCatalog: func() ([]types.UnitMeta, []types.UnitMeta) {
			return types.ListMinis(), types.ListChampions()
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

	// TEMPORARY: Provide default gold if Account has 0
	// This gives players some gold to start with the shop
	if acc.Gold == 0 {
		log.Printf("ACCOUNT %s has 0 gold, providing 1000 starting gold", playerID)
		acc.Gold = 1000 // Give starting gold

		// Try to save updated account (may fail if account ID is invalid)
		if saveErr := account.SaveAccount(acc); saveErr != nil {
			log.Printf("FAILED to save account with starting gold: %v", saveErr)
			// Continue anyway - account may not save but transaction will work locally
		}
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

	// Get unit progress
	meta := types.GetUnitMeta(slot.UnitID)
	if meta == nil {
		return fmt.Errorf("unit not found: %s", slot.UnitID)
	}

	// Load/create progress
	progress, err := s.progressionService.LoadUnitProgress(slot.UnitID)
	if err != nil {
		return fmt.Errorf("failed to load progress: %w", err)
	}

	// Set rarity from meta
	progress.Rarity = meta.Rarity

	// Calculate shard cost for next rank
	costPerRank := meta.Rarity.ShardsPerRank()

	// Add one shard
	rankUps := progression.AddShards(progress, 1)

	// Deduct gold
	acc.Gold -= slot.PriceGold

	// Mark as sold
	slot.Sold = true
	acc.Shop.Sold[req.Slot] = true
	acc.Shop.NonceSeen[req.Nonce] = true

	// Initialize progress map if needed
	if acc.Progress == nil {
		acc.Progress = make(map[string]*types.UnitProgress)
	}
	acc.Progress[slot.UnitID] = progress

	// Save progress
	if err := s.progressionService.SaveUnitProgress(progress); err != nil {
		return fmt.Errorf("failed to save progress: %w", err)
	}

	// Save account
	if err := account.SaveAccount(acc); err != nil {
		return fmt.Errorf("failed to save account: %w", err)
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

		// Unit progress update if rank changed
		if rankUps > 0 {
			s.progressionService.BroadcastUnitProgress(slot.UnitID, progress, broadcaster)
		}
	}

	return nil
}
