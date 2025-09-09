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
	Rarity        Rarity
	Rank          int
	ShardsOwned   int
	PerksUnlocked []PerkID
	ActivePerk    *PerkID
}
