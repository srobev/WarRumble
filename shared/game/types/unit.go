package types

type PerkID string

type Perk struct {
	ID          PerkID
	Name        string
	Description string
	Legendary   bool
}

type UnitProgress struct {
	UnitID        string
	Rarity        Rarity   `json:"rarity"`
	Rank          int      `json:"rank"`
	ShardsOwned   int      `json:"shardsOwned"`
	PerksUnlocked []PerkID `json:"perksUnlocked"`
	ActivePerk    *PerkID  `json:"activePerk,omitempty"`
}

// GetUpgradeCost returns the shards required to upgrade from given rank (TRADITIONAL SYSTEM)
func GetUpgradeCost(rank int) int {
	switch rank {
	case 1:
		return 3 // Rank 1 → Rank 2: 3 shards (clones needed)
	case 2:
		return 10 // Rank 2 → Rank 3: 10 shards (clones needed)
	case 3:
		return 25 // Rank 3 → Rank 4: 25 shards (clones needed)
	case 4:
		return 25 // Rank 4 → Rank 5: 25 shards (clones needed)
	default:
		return 999 // Max rank reached
	}
}

type PerkEffect struct {
	Type              string  `json:"type"`
	Radius            float64 `json:"radius,omitempty"`
	SlowPct           float64 `json:"slow_pct,omitempty"`
	Nth               int     `json:"nth,omitempty"`
	BonusDmgPct       float64 `json:"bonus_dmg_pct,omitempty"`
	OnDeathSlowPct    float64 `json:"slow_pct,omitempty"`
	OnDeathDurationMs int     `json:"duration_ms,omitempty"`
	DrPct             float64 `json:"dr_pct,omitempty"`
	AllyRadius        float64 `json:"ally_radius,omitempty"`
	ArmorBonusPct     float64 `json:"armor_bonus_pct,omitempty"`
	HpThresholdPct    float64 `json:"hp_threshold_pct,omitempty"`
	AllyDmgPct        float64 `json:"ally_dmg_pct,omitempty"`
}
