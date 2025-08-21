package protocol

type Profile struct {
	PlayerID  int64               `json:"playerId"`
	Name      string              `json:"name"`
	Army      []string            `json:"army"`             // active: [champion, 6 minis]
	Armies    map[string][]string `json:"armies,omitempty"` // all saved armies: champ -> 6 minis
	Gold      int                 `json:"gold"`
	AccountXP int                 `json:"accountXp"`
	UnitXP    map[string]int      `json:"unitXp,omitempty"` // per-mini XP, e.g. "Archer": 120
	Resources map[string]int      `json:"resources,omitempty"`
	PvPRating int                 `json:"pvp_rating"` // e.g. 1200 base
	PvPRank   string              `json:"pvp_rank"`   // derived server-side
	Avatar    string              `json:"avatar"`     // in game avatar
}

// Existing messages stay the same:
type GetProfile struct{}
type SaveArmy struct {
	Cards []string `json:"cards"` // [champion, 6 minis]
}

type Logout struct{}
