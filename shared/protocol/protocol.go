package protocol

// PerkID alias to avoid circular import (same as types.PerkID)
type PerkID = string

type CapsuleStock struct {
	Rare      int `json:"rare"`
	Epic      int `json:"epic"`
	Legendary int `json:"legendary"`
}

type Profile struct {
	PlayerID  int64               `json:"playerId"`
	Name      string              `json:"name"`
	Army      []string            `json:"army"`             // active: [champion, 6 minis]
	Armies    map[string][]string `json:"armies,omitempty"` // all saved armies: champ -> 6 minis
	Gold      int                 `json:"gold"`
	AccountXP int                 `json:"accountXp"`
	UnitXP    map[string]int      `json:"unitXp,omitempty"` // per-mini XP, e.g. "Archer": 120
	Resources map[string]int      `json:"resources,omitempty"`
	Dust      int                 `json:"dust"`       // upgrade dust currency
	Capsules  CapsuleStock        `json:"capsules"`   // upgrade capsules by rarity
	PvPRating int                 `json:"pvp_rating"` // e.g. 1200 base
	PvPRank   string              `json:"pvp_rank"`   // derived server-side
	Avatar    string              `json:"avatar"`     // in game avatar
	GuildID   string              `json:"guildId,omitempty"`
}

// Existing messages stay the same:
type GetProfile struct{}
type SaveArmy struct {
	Cards []string `json:"cards"` // [champion, 6 minis]
}

type Logout struct{}

// Request another user's profile by name (server reads persisted file)
type GetUserProfile struct{ Name string }
type UserProfile struct{ Profile Profile }

// Spell casting messages
type CastSpell struct {
	SpellName string  `json:"spellName"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	ClientTs  int64   `json:"clientTs"`
}

// Unit Progression messages
type UnitProgressUpdate struct {
	UnitID      string `json:"unitId"`
	DeltaShards int    `json:"deltaShards"`
}

type UnitProgressSynced struct {
	UnitID                string `json:"unitId"`
	Rank                  int    `json:"rank"`
	ShardsOwned           int    `json:"shardsOwned"`
	PerkSlotsUnlocked     int    `json:"perkSlotsUnlocked"`
	LegendaryPerkUnlocked bool   `json:"legendaryPerkUnlocked"`
}

type SetActivePerk struct {
	UnitID string `json:"unitId"`
	PerkID PerkID `json:"perkId"`
}

type ActivePerkChanged struct {
	UnitID     string  `json:"unitId"`
	ActivePerk *PerkID `json:"activePerk"`
}

type UpgradeUnit struct {
	UnitID string `json:"unitId"`
}
