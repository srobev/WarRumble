package progression

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"rumble/shared/game/types"
	"rumble/shared/protocol"
)

// AddShards adds shards to unit progress WITHOUT auto rank-ups (TRADITIONAL SYSTEM)
func AddShards(p *types.UnitProgress, add int) (rankUps int) {
	if add <= 0 {
		return 0
	}
	// Just add shards - NO automatic rank-ups
	p.ShardsOwned += add
	// Manual upgrades only via HandleUpgradeUnit
	return 0 // Never auto rank-up
}

// PerkSlotsUnlocked returns the number of perk slots unlocked for the unit's rarity
func PerkSlotsUnlocked(p *types.UnitProgress) int {
	return int(p.Rarity)
}

// SetActivePerk sets the active perk for a unit (choose one at a time)
func SetActivePerk(p *types.UnitProgress, perkID types.PerkID, available []types.Perk) bool {
	// Check if perk is purchased
	hasPerk := false
	for _, unlocked := range p.PerksUnlocked {
		if unlocked == perkID {
			hasPerk = true
			break
		}
	}
	if !hasPerk {
		return false
	}
	p.ActivePerk = &perkID
	return true
}

type Service struct {
	dataDir string
}

func NewService(dataDir string) *Service {
	s := &Service{dataDir: dataDir}
	s.ensureProgressionDir()
	return s
}

func (s *Service) ensureProgressionDir() {
	dir := filepath.Join(s.dataDir, "progression")
	os.MkdirAll(dir, 0755)
}

func (s *Service) progressionPath(unitID string) string {
	return filepath.Join(s.dataDir, "progression", unitID+".json")
}

// LoadUnitProgress loads unit progress from persistent storage
func (s *Service) LoadUnitProgress(unitID string) (*types.UnitProgress, error) {
	path := s.progressionPath(unitID)

	// Default progress
	progress := &types.UnitProgress{
		UnitID:        unitID,
		Rarity:        types.RarityCommon, // Would need to be loaded from unit data
		Rank:          1,
		ShardsOwned:   0,
		PerksUnlocked: []types.PerkID{},
		ActivePerk:    nil,
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return defaults
			return progress, nil
		}
		return nil, fmt.Errorf("failed to read progress: %w", err)
	}

	if err := json.Unmarshal(b, progress); err != nil {
		log.Printf("Failed to unmarshal progress for %s: %v, using defaults", unitID, err)
		return progress, nil
	}

	return progress, nil
}

// SaveUnitProgress persists unit progress
func (s *Service) SaveUnitProgress(progress *types.UnitProgress) error {
	path := s.progressionPath(progress.UnitID)

	b, err := json.MarshalIndent(progress, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal progress: %w", err)
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0644); err != nil {
		return fmt.Errorf("failed to write progress: %w", err)
	}

	return os.Rename(tmp, path)
}

// BroadcastUnitProgress broadcasts progress sync to all clients
func (s *Service) BroadcastUnitProgress(unitID string, progress *types.UnitProgress, broadcaster func(eventType string, event interface{})) {
	if broadcaster != nil {
		unlocked := PerkSlotsUnlocked(progress)

		event := protocol.UnitProgressSynced{
			UnitID:                unitID,
			Rank:                  progress.Rank,
			ShardsOwned:           progress.ShardsOwned,
			PerkSlotsUnlocked:     unlocked,
			LegendaryPerkUnlocked: false, // No legendary perk for now
		}
		broadcaster("UnitProgressSynced", event)
	}
}

// HandleUnitProgressUpdate processes adding shards to unit progress
func (s *Service) HandleUnitProgressUpdate(unitID string, deltaShards int, broadcaster func(eventType string, event interface{})) error {
	progress, err := s.LoadUnitProgress(unitID)
	if err != nil {
		return fmt.Errorf("failed to load progress: %w", err)
	}

	AddShards(progress, deltaShards)

	if err := s.SaveUnitProgress(progress); err != nil {
		return fmt.Errorf("failed to save progress: %w", err)
	}

	// Broadcast updated progress
	s.BroadcastUnitProgress(unitID, progress, broadcaster)

	return nil
}

// HandleSetActivePerk processes setting active perk for unit
func (s *Service) HandleSetActivePerk(unitID string, perkID types.PerkID, available []types.Perk, broadcaster func(eventType string, event interface{})) error {
	progress, err := s.LoadUnitProgress(unitID)
	if err != nil {
		return fmt.Errorf("failed to load progress: %w", err)
	}

	success := SetActivePerk(progress, perkID, available)
	if !success {
		return fmt.Errorf("failed to set active perk")
	}

	if err := s.SaveUnitProgress(progress); err != nil {
		return fmt.Errorf("failed to save progress: %w", err)
	}

	// Broadcast the change
	if broadcaster != nil {
		var activePerk *protocol.PerkID
		if progress.ActivePerk != nil {
			perkID := protocol.PerkID(*progress.ActivePerk)
			activePerk = &perkID
		}

		event := protocol.ActivePerkChanged{
			UnitID:     unitID,
			ActivePerk: activePerk,
		}
		broadcaster("ActivePerkChanged", event)
	}

	return nil
}

// HandleUpgradeUnit processes unit upgrade (DEPRECATED - use Account.HandleUpgradeUnit)
func (s *Service) HandleUpgradeUnit(username, unitID string, broadcaster func(eventType string, event interface{})) error {
	progress, err := s.LoadUnitProgress(unitID)
	if err != nil {
		return fmt.Errorf("failed to load progress: %w", err)
	}

	// Check if the unit has enough shards for upgrade
	requiredShards := types.GetUpgradeCost(progress.Rank)
	if progress.ShardsOwned < requiredShards {
		return fmt.Errorf("insufficient shards: have %d, need %d", progress.ShardsOwned, requiredShards)
	}

	// Consume shards and upgrade rank
	progress.ShardsOwned -= requiredShards
	progress.Rank++

	if err := s.SaveUnitProgress(progress); err != nil {
		return fmt.Errorf("failed to save progress: %w", err)
	}

	// Broadcast updated progress
	s.BroadcastUnitProgress(unitID, progress, broadcaster)

	log.Printf("[%s] Upgraded unit %s to rank %d (consumed %d shards)", username, unitID, progress.Rank, requiredShards)
	return nil
}
