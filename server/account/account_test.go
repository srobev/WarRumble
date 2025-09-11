package account

import (
	"encoding/json"
	"fmt"
	"testing"

	"rumble/shared/game/types"
)

func TestProgressSaveLoad(t *testing.T) {
	// Create test account
	acc := &Account{
		ID:       "test_user",
		Name:     "test_user",
		Gold:     1000,
		Progress: make(map[string]*types.UnitProgress),
	}

	// Create test progress
	progress := &types.UnitProgress{
		UnitID:        "test_unit",
		Rarity:        types.RarityRare,
		Rank:          2,
		ShardsOwned:   5,
		PerksUnlocked: []types.PerkID{"test_perk"},
	}

	// Save progress
	err := acc.SaveUnitProgress(progress, "test_user")
	if err != nil {
		t.Fatalf("SaveUnitProgress failed: %v", err)
	}

	// Verify progress is in the map
	if len(acc.Progress) == 0 {
		t.Fatal("Progress map is empty after save")
	}

	if savedProgress, exists := acc.Progress["test_unit"]; !exists {
		t.Fatal("Unit progress not found in map")
	} else {
		if savedProgress.ShardsOwned != 5 {
			t.Errorf("Shards not saved correctly: got %d, want 5", savedProgress.ShardsOwned)
		}
	}

	// Test JSON marshaling
	jsonData, err := json.MarshalIndent(acc, "", "  ")
	if err != nil {
		t.Fatalf("JSON marshal failed: %v", err)
	}

	fmt.Printf("JSON Output:\n%s\n", string(jsonData))

	// Test unmarshaling
	var newAcc Account
	err = json.Unmarshal(jsonData, &newAcc)
	if err != nil {
		t.Fatalf("JSON unmarshal failed: %v", err)
	}

	// Verify data survived round trip
	if len(newAcc.Progress) == 0 {
		t.Fatal("Progress map empty after unmarshal")
	}

	if loadedProgress, exists := newAcc.Progress["test_unit"]; !exists {
		t.Fatal("Unit progress lost during unmarshal")
	} else {
		if loadedProgress.ShardsOwned != 5 {
			t.Errorf("Shards not preserved: got %d, want 5", loadedProgress.ShardsOwned)
		}
		if loadedProgress.Rank != 2 {
			t.Errorf("Rank not preserved: got %d, want 2", loadedProgress.Rank)
		}
	}
}
