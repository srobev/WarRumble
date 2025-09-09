package progression

import "rumble/shared/game/types"

var perkUnlocks = map[types.Rarity][]int{
	types.RarityCommon:    {2},
	types.RarityRare:      {2, 4},
	types.RarityEpic:      {2, 4, 5},
	types.RarityLegendary: {2, 4, 6, 10},
}

func AddShards(p *types.UnitProgress, add int) (rankUps int) {
	if add <= 0 {
		return 0
	}
	p.ShardsOwned += add
	cost := p.Rarity.ShardsPerRank()
	for p.ShardsOwned >= cost {
		p.ShardsOwned -= cost
		p.Rank++
		rankUps++
	}
	return
}

func PerkSlotsUnlocked(p *types.UnitProgress) int {
	slots := 0
	for _, gate := range perkUnlocks[p.Rarity] {
		if gate <= p.Rank && gate != 10 {
			slots++
		}
	}
	return slots
}

func LegendaryPerkUnlocked(p *types.UnitProgress) bool {
	if p.Rarity != types.RarityLegendary {
		return false
	}
	for _, gate := range perkUnlocks[p.Rarity] {
		if gate == 10 && p.Rank >= 10 {
			return true
		}
	}
	return false
}
