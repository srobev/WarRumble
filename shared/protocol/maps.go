package protocol

// Basic geometry types for maps
type PointF struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type RectF struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

type DeployZone struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	W     float64 `json:"w"`
	H     float64 `json:"h"`
	Owner string  `json:"owner"` // "player" or "enemy"
}

type Lane struct {
	Points []PointF `json:"points"`
	Dir    int      `json:"dir"` // 1 or -1 (flow direction)
}

// MapDef describes a PVE map layout for gameplay
type MapDef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Width  int    `json:"width"`        // background width in pixels (optional)
	Height int    `json:"height"`       // background height in pixels (optional)
	Bg     string `json:"bg,omitempty"` // optional background image path

	DeployZones   []DeployZone `json:"deployZones"`
	MeetingStones []PointF     `json:"meetingStones"`
	GoldMines     []PointF     `json:"goldMines"`
	Lanes         []Lane       `json:"lanes"`

	// Base positions for PvP (configurable per map)
	PlayerBase PointF `json:"playerBase,omitempty"` // Player base position (normalized 0-1)
	EnemyBase  PointF `json:"enemyBase,omitempty"`  // Enemy base position (normalized 0-1)

	// Match timer configuration
	TimeLimit int `json:"timeLimit,omitempty"` // Time limit in seconds (default 180 = 3:00)

	// Arena mode (for PvP) - if true, bottom 50% is mirrored to top 50%
	IsArena bool `json:"isArena,omitempty"` // Whether this is an arena map (auto-mirrors bottom to top)
}

// C->S
type GetMap struct{ ID string }
type SaveMap struct{ Def MapDef }

// Timer and pause related messages
type PauseGame struct{}
type ResumeGame struct{}
type RestartMatch struct{}
type SurrenderMatch struct{}

// S->C
type MapDefMsg struct{ Def MapDef }
type TimerUpdate struct {
	RemainingSeconds int  `json:"remainingSeconds"`
	IsPaused         bool `json:"isPaused"`
}
