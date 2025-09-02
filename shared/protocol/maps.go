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

type Lane struct {
    Points []PointF `json:"points"`
    Dir    int      `json:"dir"` // 1 or -1 (flow direction)
}

// MapDef describes a PVE map layout for gameplay
type MapDef struct {
    ID    string `json:"id"`
    Name  string `json:"name"`
    Width  int   `json:"width"`  // background width in pixels (optional)
    Height int   `json:"height"` // background height in pixels (optional)
    Bg     string `json:"bg,omitempty"` // optional background image path

    DeployZones   []RectF  `json:"deployZones"`
    MeetingStones []PointF `json:"meetingStones"`
    GoldMines     []PointF `json:"goldMines"`
    Lanes         []Lane   `json:"lanes"`
}

// C->S
type GetMap struct{ ID string }
type SaveMap struct{ Def MapDef }

// S->C
type MapDefMsg struct{ Def MapDef }
