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

type Obstacle struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Type   string  `json:"type"`   // obstacle type (e.g., "tree", "rock", "building")
	Image  string  `json:"image"`  // image path
	Width  float64 `json:"width"`  // normalized width (0-1)
	Height float64 `json:"height"` // normalized height (0-1)
}

type DecorativeElement struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Image  string  `json:"image"`  // image path
	Width  float64 `json:"width"`  // normalized width (0-1)
	Height float64 `json:"height"` // normalized height (0-1)
	Layer  int     `json:"layer"`  // rendering layer (0=background, 1=middle, 2=foreground)
}

// MapDef describes a PVE map layout for gameplay
type MapDef struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Width  int    `json:"width"`        // background width in pixels (optional)
	Height int    `json:"height"`       // background height in pixels (optional)
	Bg     string `json:"bg,omitempty"` // optional background image path

	// Background positioning and scaling
	BgScale   float64 `json:"bgScale,omitempty"`   // background scale factor
	BgOffsetX float64 `json:"bgOffsetX,omitempty"` // background X offset
	BgOffsetY float64 `json:"bgOffsetY,omitempty"` // background Y offset

	DeployZones        []DeployZone        `json:"deployZones"`
	MeetingStones      []PointF            `json:"meetingStones"`
	GoldMines          []PointF            `json:"goldMines"`
	Lanes              []Lane              `json:"lanes"`
	Obstacles          []Obstacle          `json:"obstacles"`
	DecorativeElements []DecorativeElement `json:"decorativeElements,omitempty"`

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
