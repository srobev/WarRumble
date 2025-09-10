package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"rumble/shared/game/types"
)

// PerkData represents the structure in perks.json
type PerkData struct {
	ID      string                 `json:"id"`
	Name    string                 `json:"name"`
	Desc    string                 `json:"desc"`
	Effects map[string]interface{} `json:"effects"`
}

// UnitPerks holds perks for each unit
type UnitPerks map[string][]PerkData

// Global variable for loaded perks
var globalPerks UnitPerks

// LoadPerks loads perks from server/data/perks.json on server boot
func LoadPerks(dataDir string) error {
	path := dataDir + "/perks.json"
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("failed to open perks.json: %w", err)
	}
	defer file.Close()

	if err := json.NewDecoder(file).Decode(&globalPerks); err != nil {
		return fmt.Errorf("failed to decode perks.json: %w", err)
	}

	log.Printf("Loaded perks for %d units", len(globalPerks))
	return nil
}

// GetPerksForUnit returns the perks for a given unit, or nil if not found
func GetPerksForUnit(unitID string) []types.Perk {
	var result []types.Perk
	perks, exists := globalPerks[unitID]
	if !exists {
		return nil
	}

	for _, p := range perks {
		result = append(result, types.Perk{
			ID:          types.PerkID(p.ID),
			Name:        p.Name,
			Description: p.Desc,
			Legendary:   false, // none are legendary
		})
	}
	return result
}
