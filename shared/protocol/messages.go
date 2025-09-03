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
	Name        string  `json:"name"`
	Class       string  `json:"class"`
	SubClass    string  `json:"subclass,omitempty"`
	Role        string  `json:"role"`
	Cost        int     `json:"cost"`
	Portrait    string  `json:"portrait,omitempty"`
	Dmg         int     `json:"dmg,omitempty"`
	Hp          int     `json:"hp,omitempty"`
	Heal        int     `json:"heal,omitempty"`
	Hps         int     `json:"hps,omitempty"`
	Speed       int     `json:"speed,omitempty"`        // 1=slow,2=medium,3=mid-fast,4=fast
	AttackSpeed float64 `json:"attack_speed,omitempty"` // attacks per second
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
	Tick         int64       `json:"tick"`
	UnitsUpsert  []UnitState `json:"unitsUpsert"`
	UnitsRemoved []int64     `json:"unitsRemoved"`
	Bases        []BaseState `json:"bases,omitempty"`
	Events       []string    `json:"events,omitempty"`
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
