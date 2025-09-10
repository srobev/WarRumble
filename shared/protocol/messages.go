package protocol

import "encoding/json"

// Envelope
type MsgEnvelope struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// ================= C -> S =================

// Legacy gameflow (kept for PvP/future)
type Join struct {
	Name  string `json:"name"`
	Token string `json:"token,omitempty"`
	Room  string `json:"room,omitempty"`
}
type Ready struct{}

type DeployMiniAt struct {
	CardIndex int     `json:"cardIndex"`
	X         float64 `json:"x"`
	Y         float64 `json:"y"`
	ClientTs  int64   `json:"clientTs"`
}

// Menu / Profile / Lobby
type SetName struct {
	Name string `json:"name"`
}

// Minis list for Army UI
type ListMinis struct{}

type MiniInfo struct {
	Name        string   `json:"name"`
	Class       string   `json:"class"`
	SubClass    string   `json:"subclass,omitempty"`
	Role        string   `json:"role"`
	Cost        int      `json:"cost"`
	Portrait    string   `json:"portrait,omitempty"`
	Dmg         int      `json:"dmg,omitempty"`
	Hp          int      `json:"hp,omitempty"`
	Heal        int      `json:"heal,omitempty"`
	Hps         int      `json:"hps,omitempty"`
	Speed       int      `json:"speed,omitempty"`        // 1=slow,2=medium,3=mid-fast,4=fast
	AttackSpeed float64  `json:"attack_speed,omitempty"` // attacks per second
	Features    []string `json:"features,omitempty"`     // Special features like "siege"
}
type Minis struct {
	Items []MiniInfo `json:"items"`
}

// Maps / Rooms
type ListMaps struct{}
type MapInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Desc string `json:"desc,omitempty"`
}
type Maps struct {
	Items []MapInfo `json:"items"`
}

type CreatePve struct {
	MapID string `json:"mapId"`
}

type CreatePvp struct {
	MapID string
}

type LeaveRoom struct{}
type StartBattle struct{}

// Optional (for PvP later)
type SetReady struct {
	Ready bool `json:"ready"`
}

// Currency operations (Gold)
type GrantGold struct {
	Amount int64  `json:"amount"`
	Reason string `json:"reason"`
}

type SpendGold struct {
	Amount int64  `json:"amount"`
	Reason string `json:"reason"`
	Nonce  string `json:"nonce"` // For deduplication
}

// ================= S -> C =================

type MiniCardView struct {
	Name     string `json:"name"`
	Portrait string `json:"portrait"`
	Cost     int    `json:"cost"`
	Class    string `json:"class"`
}

type Init struct {
	PlayerID  int64          `json:"playerId"`
	MapWidth  int            `json:"mapWidth"`
	MapHeight int            `json:"mapHeight"`
	Hand      []MiniCardView `json:"hand"`
	Next      MiniCardView   `json:"next"`
	Tick      int64          `json:"tick"`
}

type GoldUpdate struct {
	PlayerID int64 `json:"playerId"`
	Gold     int   `json:"gold"`
}

type HandUpdate struct {
	Hand []MiniCardView `json:"hand"`
	Next MiniCardView   `json:"next"`
}

type UnitState struct {
	ID       int64   `json:"id"`
	Name     string  `json:"name"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	HP       int     `json:"hp"`
	MaxHP    int     `json:"maxHp"`
	OwnerID  int64   `json:"ownerId"`
	Facing   float64 `json:"facing"`
	Class    string  `json:"class"`
	Range    int     `json:"range"`
	Particle string  `json:"particle,omitempty"`
}

type ProjectileState struct {
	ID             int64   `json:"id"`
	X              float64 `json:"x"`
	Y              float64 `json:"y"`
	TX             float64 `json:"tx"`
	TY             float64 `json:"ty"`
	Damage         int     `json:"damage"`
	OwnerID        int64   `json:"ownerId"`
	TargetID       int64   `json:"targetId"`
	ProjectileType string  `json:"projectileType"`
	Active         bool    `json:"active"`
}

type BaseState struct {
	OwnerID int64 `json:"ownerId"`
	HP      int   `json:"hp"`
	MaxHP   int   `json:"maxHp"`
	X       int   `json:"x"`
	Y       int   `json:"y"`
	W       int   `json:"w"`
	H       int   `json:"h"`
}

type StateDelta struct {
	Tick         int64             `json:"tick"`
	UnitsUpsert  []UnitState       `json:"unitsUpsert"`
	UnitsRemoved []int64           `json:"unitsRemoved"`
	Projectiles  []ProjectileState `json:"projectiles,omitempty"`
	Bases        []BaseState       `json:"bases,omitempty"`
	Events       []string          `json:"events,omitempty"`
}

type HealingEvent struct {
	HealerID   int64   `json:"healerId"`   // ID of the healing unit
	HealerX    float64 `json:"healerX"`    // Position of healer
	HealerY    float64 `json:"healerY"`    // Position of healer
	TargetID   int64   `json:"targetId"`   // ID of the healed unit
	TargetX    float64 `json:"targetX"`    // Position of target
	TargetY    float64 `json:"targetY"`    // Position of target
	HealAmount int     `json:"healAmount"` // Amount healed
	HealerName string  `json:"healerName"` // Name of healer unit
	TargetName string  `json:"targetName"` // Name of target unit
}

type UnitDeathEvent struct {
	UnitID       int64   `json:"unitId"`       // ID of the dying unit
	UnitX        float64 `json:"unitX"`        // Position of dying unit
	UnitY        float64 `json:"unitY"`        // Position of dying unit
	UnitName     string  `json:"unitName"`     // Name of dying unit
	UnitClass    string  `json:"unitClass"`    // Class of dying unit (melee, range, etc.)
	UnitSubclass string  `json:"unitSubclass"` // Subclass of dying unit (healer, etc.)
	KillerID     int64   `json:"killerId"`     // ID of the unit that killed this one (0 if base damage)
}

type UnitSpawnEvent struct {
	UnitID       int64   `json:"unitId"`       // ID of the spawning unit
	UnitX        float64 `json:"unitX"`        // Position where unit will spawn
	UnitY        float64 `json:"unitY"`        // Position where unit will spawn
	UnitName     string  `json:"unitName"`     // Name of spawning unit
	UnitClass    string  `json:"unitClass"`    // Class of spawning unit (melee, range, etc.)
	UnitSubclass string  `json:"unitSubclass"` // Subclass of spawning unit (healer, etc.)
	OwnerID      int64   `json:"ownerId"`      // ID of the player who owns the unit
}

type VictoryEvent struct {
	WinnerID   int64  `json:"winnerId"`   // ID of the winning player
	WinnerName string `json:"winnerName"` // Name of the winning player
	MatchType  string `json:"matchType"`  // Type of match (pve, pvp, etc.)
	Duration   int    `json:"duration"`   // Match duration in seconds
	GoldEarned int    `json:"goldEarned"` // Gold earned from the match
	XPGained   int    `json:"xpGained"`   // Total XP gained from the match
}

type DefeatEvent struct {
	LoserID    int64  `json:"loserId"`    // ID of the losing player
	LoserName  string `json:"loserName"`  // Name of the losing player
	WinnerID   int64  `json:"winnerId"`   // ID of the winning player
	WinnerName string `json:"winnerName"` // Name of the winning player
	MatchType  string `json:"matchType"`  // Type of match (pve, pvp, etc.)
	Duration   int    `json:"duration"`   // Match duration in seconds
}

type BaseDamageEvent struct {
	BaseID       int64   `json:"baseId"`       // ID of the damaged base
	BaseX        float64 `json:"baseX"`        // Position of the base
	BaseY        float64 `json:"baseY"`        // Position of the base
	Damage       int     `json:"damage"`       // Amount of damage dealt
	AttackerID   int64   `json:"attackerId"`   // ID of the unit that dealt damage (0 if environmental)
	AttackerName string  `json:"attackerName"` // Name of the attacking unit
	BaseHP       int     `json:"baseHp"`       // Current HP after damage
	BaseMaxHP    int     `json:"baseMaxHp"`    // Maximum HP of the base
}

type AoEDamageEvent struct {
	TargetID     int64   `json:"targetId"`     // ID of the unit that took AoE damage
	TargetX      float64 `json:"targetX"`      // Position of the target unit
	TargetY      float64 `json:"targetY"`      // Position of the target unit
	Damage       int     `json:"damage"`       // Amount of AoE damage dealt
	AttackerID   int64   `json:"attackerId"`   // ID of the unit that fired the projectile (0 if environmental)
	AttackerName string  `json:"attackerName"` // Name of the attacking unit/projectile
	TargetName   string  `json:"targetName"`   // Name of the target unit
	ImpactX      float64 `json:"impactX"`      // X position where the projectile impacted
	ImpactY      float64 `json:"impactY"`      // Y position where the projectile impacted
}

type SpellCastEvent struct {
	SpellName string  `json:"spellName"` // Name of the spell being cast
	CasterID  int64   `json:"casterId"`  // ID of the player casting the spell
	TargetX   float64 `json:"targetX"`   // X position where spell was cast
	TargetY   float64 `json:"targetY"`   // Y position where spell was cast
}

type FullSnapshot struct {
	Tick  int64       `json:"tick"`
	Units []UnitState `json:"units"`
	Bases []BaseState `json:"bases"`
}

type RoomCreated struct {
	RoomID string `json:"roomId"`
}
type RoomStatus struct {
	RoomID  string         `json:"roomId"`
	Players []int64        `json:"players"`
	Ready   map[int64]bool `json:"ready"`
	HostID  int64
}

type JoinRoom struct {
	RoomID string
}

type RoomJoined struct {
	RoomID string
}

type ErrorMsg struct {
	Message string `json:"message"`
}

// Currency events
type GoldSynced struct {
	Gold int64 `json:"gold"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type GameOver struct {
	WinnerID int64  `json:"winner_id"`
	Reason   string `json:"reason,omitempty"`
}

// Queue-based PvP
type JoinPvpQueue struct{}
type LeavePvpQueue struct{}
type QueueStatus struct {
	InQueue bool
	Players int
}
type PvpMatched struct {
	RoomID   string
	Opponent string
}

// Friendly duels
type FriendlyCreate struct{} // client -> server
type FriendlyCancel struct{} // client -> server
type FriendlyJoin struct {
	Code string `json:"code"`
} // client -> server
type FriendlyCode struct {
	Code string `json:"code"`
} // server -> client
type FriendlyReady struct{ RoomID string }

// Shop messages - Roll types are handled dynamically since they're complex
type ShopRollSynced struct {
	Roll interface{} `json:"roll"`
}

type GetShopRollReq struct{}

type RerollShopReq struct {
	Nonce string `json:"nonce"`
}

type BuyShopSlotReq struct {
	Slot  int    `json:"slot"`
	Nonce string `json:"nonce"`
}

type BuyShopResult struct {
	Slot      int    `json:"slot"`
	UnitID    string `json:"unitId"`
	Gold      int64  `json:"gold"`
	Shards    int    `json:"shards"`    // total shards for this unit after buy
	Rank      int    `json:"rank"`      // unit rank after buy
	Threshold int    `json:"threshold"` // rarity threshold (3/10/25/25)
}
