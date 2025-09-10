package game

// ---- Core enums / layout constants ----

type connState int

const (
	stateIdle connState = iota
	stateConnecting
	stateConnected
	stateFailed

	// UI layout
	menuBarH   = 56
	topBarH    = 44
	battleHUDH = 160

	pad  = 8
	btnW = 120
	btnH = 32
	rowH = 20
)

type screen int

const (
	screenLogin screen = iota
	screenHome
	screenBattle
)

type tab int

const (
	tabShop tab = iota
	tabArmy
	tabMap
	tabPvp
	tabSocial
	tabSettings
)

// ---- Small utility types ----

type rect struct{ x, y, w, h int }

func (r rect) hit(mx, my int) bool {
	return mx >= r.x && mx <= r.x+r.w && my >= r.y && my <= r.y+r.h
}

// Used by async connection
type connResult struct {
	n   *Net
	err error
}

// ---- Map hotspot types used across files ----

type HitRect struct {
	ID, Name, Info string
	L, T, R, B     float64 // normalized 0..1
}

type HSRect struct {
	Left, Top, Right, Bottom float64 // normalized 0..1
}

type Hotspot struct {
	ID, Name, Info string
	X, Y           float64 // normalized center
	Rpx            int     // draw radius / hit radius (px)
	HitRect        *HSRect // optional precise rect for hit-testing
	TargetMapID    string  // when set, which arena/map to launch
}

// World map image fallback id used across files
const defaultMapID = "rumble_world"
