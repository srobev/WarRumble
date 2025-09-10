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
	ActivePerk    *PerkID  `json:"activePerk"`
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
