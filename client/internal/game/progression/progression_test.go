package progression

import (
	"rumble/shared/game/types"
	"testing"
)

func TestCommonRankUp(t *testing.T) {
	p := types.UnitProgress{UnitID: "u", Rarity: types.RarityCommon, Rank: 1}
	if ups := AddShards(&p, 3); ups != 1 || p.Rank != 2 || p.ShardsOwned != 0 {
		t.Fatalf("want rank2, got rank=%d shards=%d ups=%d", p.Rank, p.ShardsOwned, ups)
	}
	if PerkSlotsUnlocked(&p) != 1 {
		t.Fatalf("common should unlock 1 slot at rank 2")
	}
}

func TestRareToRank4(t *testing.T) {
	p := types.UnitProgress{UnitID: "u", Rarity: types.RarityRare, Rank: 1}
	AddShards(&p, 10) // rank 2
	AddShards(&p, 20) // rank 4
	if p.Rank != 4 {
		t.Fatalf("want rank 4, got %d", p.Rank)
	}
	if PerkSlotsUnlocked(&p) != 2 {
		t.Fatalf("rare slots at rank4")
	}
}

func TestLegendaryRank10Gate(t *testing.T) {
	p := types.UnitProgress{UnitID: "u", Rarity: types.RarityLegendary, Rank: 1}
	AddShards(&p, 225) // 9 rank-ups * 25 = 225 → rank 10
	if p.Rank != 10 {
		t.Fatalf("want rank 10, got %d", p.Rank)
	}
	if !LegendaryPerkUnlocked(&p) {
		t.Fatalf("legendary perk must unlock at rank 10")
	}
}
