package progression

import "rumble/shared/game/types"

func SetActivePerk(p *types.UnitProgress, perkID types.PerkID, available []types.Perk) bool {
	var target *types.Perk
	for i := range available {
		if available[i].ID == perkID {
			target = &available[i]
			break
		}
	}
	if target == nil {
		return false
	}
	if target.Legendary && !LegendaryPerkUnlocked(p) {
		return false
	}
	if !target.Legendary && PerkSlotsUnlocked(p) < 1 {
		return false
	}
	p.ActivePerk = &perkID
	return true
}
