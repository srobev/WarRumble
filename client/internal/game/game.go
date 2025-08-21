package game

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // (optional) register JPEG decoder
	_ "image/png"  // register PNG decoder
	"log"
	"math"
	"path"
	"rumble/client/internal/netcfg"
	"rumble/shared/protocol"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/atotto/clipboard"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"

	"embed"
)

//go:embed assets/ui/* assets/minis/* assets/maps/*
var assetsFS embed.FS

type connState int

const (
	stateIdle connState = iota
	stateConnecting
	stateConnected
	stateFailed

	// Small bar for Home (Army/Map/Settings)
	menuBarH = 56

	// only on Home screen (Army/Map/Settings)
	topBarH = 44

	// Tall battle HUD like your old hud.go
	battleHUDH = 160

	pad  = 8
	btnW = 120
	btnH = 32
	rowH = 20
)

type screen int

type connResult struct {
	n   *Net
	err error
}

const (
	screenLogin screen = iota
	screenHome
	screenBattle
)

type tab int

const (
	tabArmy tab = iota
	tabMap
	tabPvp
	tabSocial
	tabSettings
)

type rect struct{ x, y, w, h int }

func (r rect) hit(mx, my int) bool { return mx >= r.x && mx <= r.x+r.w && my >= r.y && my <= r.y+r.h }

type Game struct {
	// connection/boot UI
	connOnce   sync.Once
	connCh     chan connResult
	connSt     connState
	connErrMsg string

	net   *Net
	world *World

	// app/profile
	scr       screen
	activeTab tab
	nameInput string
	playerID  int64
	name      string

	// battle
	gold        int
	hand        []protocol.MiniCardView
	next        protocol.MiniCardView
	selectedIdx int

	// Army tab: split pickers
	minisAll         []protocol.MiniInfo // everything from server
	champions        []protocol.MiniInfo // role/class champion
	minisOnly        []protocol.MiniInfo // role mini && class != spell
	selectedChampion string
	selectedMinis    map[string]bool
	armyMsg          string

	// scroll state + clickable rects
	champScroll int
	miniScroll  int
	// minis collection drag/touch (vertical)
	collDragActive bool
	collDragStartY int
	collDragLastY  int
	collDragAccum  int
	collTouchID    ebiten.TouchID

	// Map tab
	maps        []protocol.MapInfo
	mapRects    []rect
	selectedMap int
	roomID      string
	startBtn    rect
	// --- Top bar (Home only) ---
	accountGold int  // separate "meta" currency (not battle gold)
	userBtn     rect // clickable later for profile/statistics
	titleArea   rect
	goldArea    rect
	// bottom bar buttons
	armyBtn, mapBtn, pvpBtn, socialBtn, settingsBtn rect

	// settings
	fullscreen        bool
	fsOnBtn, fsOffBtn rect

	assets     Assets
	nameToMini map[string]protocol.MiniInfo // by mini Name
	currentMap string

	// drag & drop state
	dragActive bool
	dragIdx    int
	dragStartX int
	dragStartY int

	gameOver bool
	victory  bool

	endActive   bool
	endVictory  bool
	continueBtn rect

	// Army grid UI
	showChamp bool // true: show champions, false: show minis

	// --- Army: new UI state ---
	// champion strip (top)
	champStripArea   rect
	champStripRects  []rect
	champStripScroll int // index of first visible champion
	// --- Champion strip drag/touch scroll ---
	champDragActive bool
	champDragStartX int
	champDragLastX  int
	champDragAccum  int // pixels accumulated to convert into column steps

	// touch tracking (single-pointer drag)
	activeTouchID ebiten.TouchID

	// selected champion + minis (2x3 grid)
	armySlotRects [7]rect // [0]=champion big card, [1..6] mini slots

	// collection grid (minis only)
	collArea   rect
	collRects  []rect
	collScroll int // row-based scroll for minis collection

	// data for per-champion armies (client-side cache)
	champToMinis map[string]map[string]bool // champName -> set(miniName)

	// Map tab (new hotspot UI)
	mapHotspots  map[string][]Hotspot // key: mapID -> hotspots
	hoveredHS    int                  // -1 if none
	selectedHS   int                  // -1 if none
	mapDebug     bool                 // add in Game struct
	rectHotspots map[string][]HitRect
	showRects    bool // optional debug outline toggle

	currentArena string
	pendingArena string

	// HP bar FX (recent-damage yellow chip)
	hpFxUnits map[int64]*hpFx
	hpFxBases map[int64]*hpFx

	// --- PvP UI state ---
	pvpStatus      string // status line at the top of the PvP tab
	pvpQueued      bool   // currently in matchmaking queue
	pvpHosting     bool   // currently hosting a friendly code
	pvpCode        string // last code we got back from the server (when hosting)
	pvpCodeInput   string // what the user typed into the "Join with code" field
	pvpInputActive bool   // text input focus for the code field
	pvpCodeArea    rect   // This will be the pvpcode copy area
	// profile PvP
	pvpRating int
	pvpRank   string
	// PvP leaderboard
	pvpLeaders  []protocol.LeaderboardEntry
	lbLastReq   time.Time
	lbLastStamp int64 // server GeneratedAt (optional)

	//Avatars and profile
	avatar      string
	showProfile bool

	avatars      []string // discovered from assets/ui/avatars
	avatarRects  []rect
	profileOpen bool
	profCloseBtn rect
	profLogoutBtn rect
}

type hpFx struct {
	lastHP      int
	ghostHP     int   // where the yellow chip currently ends (>= current HP)
	holdUntilMs int64 // time to keep the yellow chip still
	lerpStartMs int64 // when the chip starts animating
	lerpStartHP int   // chip start HP when anim begins
	lerpDurMs   int64 // animation duration (ms)
}

// --- image assets (safe-loading) ---
type Assets struct {
	btn9Base  *ebiten.Image
	btn9Hover *ebiten.Image

	minis               map[string]*ebiten.Image // key: portrait filename (or derived)
	baseMe              *ebiten.Image
	baseEnemy           *ebiten.Image
	baseDead            *ebiten.Image            // optional destroyed variant
	bg                  map[string]*ebiten.Image // key: mapID -> background
	coinFull, coinEmpty *ebiten.Image

	edgeCol map[string]color.NRGBA // <- new: per-map letterbox color
}

type HitRect struct {
	ID, Name, Info string
	L, T, R, B     float64 // normalized 0..1 in image space
}

type Hotspot struct {
	ID, Name, Info string
	X, Y           float64 // normalized center (for the visible dot)
	Rpx            int     // pixel radius for fallback circular hit & for visuals
	HitRect        *HSRect // optional: if set, use this invisible rectangle for hit-testing
	TargetMapID    string  // <- NEW: which arena/map to launch
}

type HSRect struct {
	Left, Top, Right, Bottom float64 // normalized 0..1 in image space
}

func drawBaseImg(screen *ebiten.Image, img *ebiten.Image, b protocol.BaseState) {
	if img == nil {
		// fallback if no art
		ebitenutil.DrawRect(screen, float64(b.X), float64(b.Y), float64(b.W), float64(b.H), color.NRGBA{90, 90, 120, 255})
		return
	}
	iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
	if iw == 0 || ih == 0 {
		return
	}
	// preserve aspect ratio: "contain" inside W×H
	sx := float64(b.W) / float64(iw)
	sy := float64(b.H) / float64(ih)
	s := math.Min(sx, sy)

	op := &ebiten.DrawImageOptions{}
	ox := float64(b.X) + (float64(b.W)-float64(iw)*s)/2
	oy := float64(b.Y) + (float64(b.H)-float64(ih)*s)/2
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(ox, oy)
	screen.DrawImage(img, op)
}

// index of real filenames by lowercase name per directory
var embIndex = map[string]map[string]string{}

func buildEmbIndex() {
	if len(embIndex) != 0 {
		return
	}
	for _, dir := range []string{"assets/ui", "assets/minis", "assets/maps"} {
		entries, err := assetsFS.ReadDir(dir)
		if err != nil {
			continue
		}
		m := make(map[string]string, len(entries))
		for _, e := range entries {
			m[strings.ToLower(e.Name())] = e.Name()
		}
		embIndex[dir] = m
	}
}

func loadImage(p string) *ebiten.Image {
	buildEmbIndex()
	// first try exact path
	f, err := assetsFS.Open(p)
	if err != nil {
		// try case-insensitive filename within the same dir
		dir, file := path.Split(p)                           // dir always with '/' for embed
		key := strings.ToLower(strings.TrimSuffix(file, "")) // normalize
		if idx, ok := embIndex[strings.TrimRight(dir, "/")]; ok {
			if real, ok2 := idx[key]; ok2 {
				f, err = assetsFS.Open(dir + real)
			}
		}
	}
	if err != nil {
		log.Println("asset not found in embed:", p)
		return nil
	}
	defer f.Close()

	img, _, err := ebitenutil.NewImageFromReader(f)
	if err != nil {
		log.Println("decode image failed:", p, err)
		return nil
	}
	return img
}

func (a *Assets) ensureInit() {
	if a.btn9Base == nil {
		a.btn9Base = loadImage("assets/ui/btn9_slim.png")
	}
	if a.btn9Hover == nil {
		a.btn9Hover = loadImage("assets/ui/btn9_slim_hover.png")
	} // may be nil
	if a.minis == nil {
		a.minis = make(map[string]*ebiten.Image)
	}
	if a.bg == nil {
		a.bg = make(map[string]*ebiten.Image)
	}
	if a.baseMe == nil {
		a.baseMe = loadImage("assets/ui/base.png")
	}
	if a.baseEnemy == nil {
		a.baseEnemy = loadImage("assets/ui/base.png")
	}
	if a.baseDead == nil {
		a.baseDead = loadImage("assets/ui/base_destroyed.png")
	}
	if a.coinFull == nil {
		a.coinFull = loadImage("assets/ui/coin.png")
	}
	if a.coinEmpty == nil {
		a.coinEmpty = loadImage("assets/ui/coin_empty.png")
	}
	if a.edgeCol == nil {
		a.edgeCol = make(map[string]color.NRGBA)
	}
}

func (g *Game) portraitKeyFor(name string) string {
	// Try to use portrait from minis list; else derive "name.png"
	if info, ok := g.nameToMini[name]; ok && info.Portrait != "" {
		return info.Portrait
	}
	// derive: lowercase, spaces -> underscore
	return strings.ToLower(strings.ReplaceAll(name, " ", "_")) + ".png"
}

func (g *Game) ensureMiniImageByName(name string) *ebiten.Image {
	g.assets.ensureInit()
	key := g.portraitKeyFor(name)
	if img, ok := g.assets.minis[key]; ok {
		return img
	}
	img := loadImage("assets/minis/" + key)
	g.assets.minis[key] = img // may be nil; that’s okay (fallback to rect)
	return img
}

func (g *Game) ensureBgForMap(mapID string) *ebiten.Image {
	g.assets.ensureInit()
	if img, ok := g.assets.bg[mapID]; ok {
		return img
	}

	base := strings.ToLower(mapID)
	for _, p := range []string{
		"assets/maps/" + base + ".png",
		"assets/maps/" + base + ".jpg",
	} {
		if img := loadImage(p); img != nil {
			g.assets.bg[mapID] = img
			return img
		}
	}

	log.Println("map background not found for mapID:", mapID)
	g.assets.bg[mapID] = nil // do NOT alias to an arbitrary file
	return nil
}

func (g *Game) arenaForHotspot(worldID, hsID string) string {
	// one world map:
	if worldID == "rumble_world" {
		switch hsID {
		case "north_tower":
			return "north_tower"
		case "spawn_west":
			return "west_keep"
		case "spawn_east":
			return "east_gate"
		case "mid_bridge":
			return "mid_bridge"
		case "south_gate":
			return "south_gate"
		}
	}
	// fallback
	return "north_tower"
}

func (g *Game) ensureMapHotspots() {
	if g.mapHotspots != nil {
		return
	}

	g.mapHotspots = map[string][]Hotspot{
		"rumble_world": {
			{ID: "spawn_west", Name: "Western Keep", Info: "Good for melee rush.",
				X: 0.1250, Y: 0.5767, Rpx: 18,
				HitRect: &HSRect{Left: 0.18, Top: 0.54, Right: 0.26, Bottom: 0.62}},
			{ID: "spawn_east", Name: "Eastern Gate", Info: "Open field, risky.",
				X: 0.6317, Y: 0.4367, Rpx: 18,
				HitRect: &HSRect{Left: 0.74, Top: 0.43, Right: 0.82, Bottom: 0.51}},
			{ID: "mid_bridge", Name: "Central Bridge", Info: "Choke point.",
				X: 0.2817, Y: 0.1617, Rpx: 18,
				HitRect: &HSRect{Left: 0.47, Top: 0.47, Right: 0.53, Bottom: 0.53}},
			{ID: "north_tower", Name: "North Tower", Info: "High ground.",
				X: 0.7000, Y: 0.1400, Rpx: 18,
				HitRect: &HSRect{Left: 0.53, Top: 0.22, Right: 0.59, Bottom: 0.30}},
			{ID: "south_gate", Name: "South Gate", Info: "Wide approach.",
				X: 0.3567, Y: 0.7350, Rpx: 18,
				HitRect: &HSRect{Left: 0.40, Top: 0.74, Right: 0.48, Bottom: 0.82}},
		},
	}
}

func (g *Game) ensureMapRects() {
	if g.rectHotspots != nil {
		return
	}
	g.rectHotspots = map[string][]HitRect{
		"rumble_world": {
			// TODO: put your exact circle rectangles here (normalized)
			{ID: "spawn_west", Name: "Western Keep", Info: "Good for melee rush.", L: 0.185, T: 0.540, R: 0.255, B: 0.615},
			{ID: "spawn_east", Name: "Eastern Gate", Info: "Open field, risky.", L: 0.745, T: 0.440, R: 0.815, B: 0.515},
			{ID: "mid_bridge", Name: "Central Bridge", Info: "Choke point.", L: 0.470, T: 0.475, R: 0.530, B: 0.545},
			{ID: "north_tower", Name: "North Tower", Info: "High ground.", L: 0.530, T: 0.210, R: 0.590, B: 0.280},
			{ID: "south_gate", Name: "South Gate", Info: "Wide approach.", L: 0.410, T: 0.750, R: 0.470, B: 0.820},
		},
	}

	// Build/overwrite the point list from rect centers so indices match.
	g.ensureMapHotspots()
	for mapID, rects := range g.rectHotspots {
		hs := make([]Hotspot, len(rects))
		for i, r := range rects {
			hs[i] = Hotspot{
				ID: r.ID, Name: r.Name, Info: r.Info,
				X: (r.L + r.R) * 0.5, Y: (r.T + r.B) * 0.5, Rpx: 18,
			}
		}
		g.mapHotspots[mapID] = hs
	}
}

// New creates the game. Optional arg is kept for back-compat.
// If provided: "android"/"ios"/"desktop" sets platform; anything else is treated as player name.
func New(args ...string) ebiten.Game {
	if len(args) > 0 {
		switch args[0] {
		case "android", "ios", "desktop":
			platform = args[0]
		default:
			playerName = args[0]
		}
	}
	g := &Game{
		world: &World{
			Units: make(map[int64]*RenderUnit),
			Bases: make(map[int64]protocol.BaseState),
		},
		scr:              screenLogin,
		activeTab:        tabArmy,
		name:             playerName,
		selectedIdx:      -1,
		selectedMinis:    map[string]bool{},
		showChamp:        true,
		champStripScroll: 0,
		champToMinis:     map[string]map[string]bool{},
		accountGold:      100,
		activeTouchID:    -1,
		collTouchID:      -1,
		currentMap:       defaultMapID,
		hpFxUnits:        make(map[int64]*hpFx),
		hpFxBases:        make(map[int64]*hpFx),

		// NEW: initialize connection UI
		connCh: make(chan connResult, 1),
		connSt: stateConnecting,

	}

	// NEW: start background connect immediately so Android shows the overlay on frame 1
	g.connOnce.Do(func() { go g.connectAsync() })

	return g
}

func (g *Game) Update() error {
    // Recreate Net if needed
    if g.net == nil || g.net.IsClosed() {
        n, err := NewNet(netcfg.ServerURL)
        if err != nil {
            // don’t crash—just keep trying next frame
            return nil
        }
        g.net = n
    }
	// Kick off the connection exactly once
	// Poll background connect result (non-blocking)
	select {
	case res := <-g.connCh:
		if res.err != nil {
			g.connSt = stateFailed
			g.connErrMsg = res.err.Error()
		} else {
			g.net = res.n
			g.connSt = stateConnected
		}
	default:
	}


	if g.connSt != stateConnected {
		// Retry input (R on desktop, tap/click on mobile)
		if inpututil.IsKeyJustPressed(ebiten.KeyR) && g.connSt == stateFailed {
			g.retryConnect()
		}
		if g.connSt == stateFailed {
			if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) ||
				len(ebiten.AppendTouchIDs(nil)) > 0 {
				g.retryConnect()
			}
		}
		return nil
	}

		// drain ws (only if net is alive)
    // Drain WS only if we have a live connection
    if g.net != nil && !g.net.IsClosed() {
        for {
            select {
            case env := <-g.net.inCh:
                g.handle(env)
            default:
                goto afterMessages
            }
        }
    }

if g.profileOpen && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
    mx, my := ebiten.CursorPosition()
    if g.profCloseBtn.hit(mx, my) {
        g.profileOpen = false
        g.profCloseBtn = rect{}
        g.profLogoutBtn    = rect{}
    } else if g.profLogoutBtn.hit(mx, my) {
        g.send("Logout", protocol.Logout{})
    }
}
afterMessages:

	// keep end state up-to-date outside of Draw, and ONLY when bases are known
	if g.scr == screenBattle {
		myHP, enemyHP := -1, -1
		for _, b := range g.world.Bases {
			if b.OwnerID == g.playerID {
				myHP = b.HP
			} else {
				enemyHP = b.HP
			}
		}
		if myHP >= 0 && enemyHP >= 0 {
			g.endActive = (myHP <= 0 || enemyHP <= 0)
			g.endVictory = (enemyHP <= 0 && myHP > 0)
		} else {
			// while bases are not yet synced, don't block input
			g.endActive = false
			g.endVictory = false
		}
	}

	switch g.scr {
	case screenLogin:
	for _, k := range inpututil.AppendJustPressedKeys(nil) {
	switch k {
		case ebiten.KeyEnter:
			g.name = strings.TrimSpace(g.nameInput)
			if g.name == "" {
				g.name = "Player"
			}
			// Always ensure a fresh socket on login attempt
			if !g.ensureNet() {
				break // try again next frame
			}
			g.send("SetName", protocol.SetName{Name: g.name})
			g.send("GetProfile", protocol.GetProfile{})
			g.send("ListMinis", protocol.ListMinis{})
			g.send("ListMaps", protocol.ListMaps{})
			g.scr = screenHome
		}
	}
		g.updateLogin()
	case screenHome:
		g.updateHome()
	case screenBattle:
		g.updateBattle()
	}

	g.world.LerpPositions()
	return nil
}

// ---------- Login ----------
func (g *Game) updateLogin() {
	// simple text input
	for _, k := range inpututil.AppendJustPressedKeys(nil) {
		switch k {
		case ebiten.KeyEnter:
// inside updateLogin() on Enter:
g.name = strings.TrimSpace(g.nameInput)
if g.name == "" { g.name = "Player" }
if g.net == nil || g.net.IsClosed() {
    if n, err := NewNet(netcfg.ServerURL); err == nil {
        g.net = n
    } else {
        log.Printf("NET: re-dial failed: %v", err)
        return
    }
}
g.send("SetName", protocol.SetName{Name: g.name})
g.send("GetProfile", protocol.GetProfile{})
g.send("ListMinis", protocol.ListMinis{})
g.send("ListMaps", protocol.ListMaps{})
g.scr = screenHome
		case ebiten.KeyBackspace:
			if len(g.nameInput) > 0 {
				g.nameInput = g.nameInput[:len(g.nameInput)-1]
			}
		default:
			s := k.String()
			if len(s) == 1 {
				g.nameInput += s
			}
		}
	}
}

// ---------- Home (Army / Map tabs) ----------
func (g *Game) updateHome() {
	mx, my := ebiten.CursorPosition() // home uses window coords fine
	g.computeTopBarLayout()
	g.computeBottomBarLayout()
	// bottom bar button clicks
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if g.armyBtn.hit(mx, my) {
			g.activeTab = tabArmy
		}
		if g.mapBtn.hit(mx, my) {
			g.activeTab = tabMap
		}
		if g.settingsBtn.hit(mx, my) {
			g.activeTab = tabSettings
		}
		if g.pvpBtn.hit(mx, my) {
			g.activeTab = tabPvp
		}
	}

	// If somehow we reached Home without minis loaded, fetch them.
	if len(g.minisAll) == 0 {
		g.send("ListMinis", protocol.ListMinis{})
	}

	// Global fullscreen toggle via 'F'
	if inpututil.IsKeyJustPressed(ebiten.KeyF) && ebiten.IsKeyPressed(ebiten.KeyAlt) {
		g.fullscreen = !g.fullscreen
		ebiten.SetFullscreen(g.fullscreen)
	}
	// Block click-through on the top bar
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && my < topBarH {
		if g.userBtn.hit(mx, my) {
			log.Println("Account clicked") // hook your profile overlay here
			g.showProfile = true
			return

		} else if g.goldArea.hit(mx, my) {
			log.Println("Gold clicked") // hook a shop or info panel here
		}
		return // still block clicks from leaking
	}

if g.showProfile {
    if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
        mx, my := ebiten.CursorPosition()

        // 1) Close
        if g.profCloseBtn.hit(mx, my) {
            g.showProfile = false
            return
        }

        // 2) Logout (must be BEFORE any early returns)
		if g.profLogoutBtn.hit(mx, my) {
			g.send("Logout", protocol.Logout{})
			//g.resetToLogin()
			return
		}

        // 3) Avatar tiles
        for i, r := range g.avatarRects {
            if r.hit(mx, my) && i >= 0 && i < len(g.avatars) {
                choice := g.avatars[i]
                g.avatar = choice
                g.send("SetAvatar", protocol.SetAvatar{Avatar: choice})
                break
            }
        }
    }
    // IMPORTANT: block other Home input while overlay is up
    return
}


if g.profLogoutBtn.hit(mx, my) {
		// 1) tell server while socket is alive
		g.send("Logout", protocol.Logout{})

		// 2) tear down the client connection so Update() will reconnect cleanly later
		if g.net != nil {
			g.net.Close()  // implement to close websocket; safe to call multiple times
			g.net = nil
		}

		// 3) local UI reset
		g.showProfile = false
		g.scr = screenLogin
		g.name = ""
		g.nameInput = ""
		g.avatar = ""
		g.pvpRating = 0
		g.pvpRank = ""
		g.selectedChampion = ""
		g.selectedMinis = map[string]bool{}
	return

}
	switch g.activeTab {
	case tabArmy:
		mx, my := ebiten.CursorPosition()

		// ---- Layout numbers shared with Draw ----
		const champCardW, champCardH = 100, 116 // small cards in strip
		const miniCardW, miniCardH = 100, 116
		const gap = 10
		const stripH = champCardH + 8
		const dragThresh = 6 // pixels: below this is a click, above is a drag

		stripX := pad
		stripY := topBarH + pad
		stripW := protocol.ScreenW - 2*pad
		stepPx := champCardW + gap
		g.champStripArea = rect{x: stripX, y: stripY, w: stripW, h: stripH}

		// big champion card + 2x3 selected minis, placed below strip
		bigW := 200
		bigH := miniCardH*2 + gap
		topY := stripY + stripH + 12
		leftX := pad
		rightX := leftX + bigW + 16 // minis grid starts here

		// build champion strip rects for visible champions
		cols := maxInt(1, (stripW+gap)/(champCardW+gap))
		start := clampInt(g.champStripScroll, 0, maxInt(0, len(g.champions)-cols))
		g.champStripRects = g.champStripRects[:0]
		for i := 0; i < cols && start+i < len(g.champions); i++ {
			x := stripX + i*(champCardW+gap)
			g.champStripRects = append(g.champStripRects, rect{x: x, y: stripY, w: champCardW, h: champCardH})
		}

		// ---------- CHAMP STRIP: mouse drag vs click (with threshold) ----------
		// Arm drag on press (record start), but don't "activate" until we move far enough
		if g.champStripArea.hit(mx, my) && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Use the existing fields to store press info
			g.champDragStartX = mx
			g.champDragLastX = mx
			g.champDragAccum = 0
			// DO NOT set champDragActive yet (wait for threshold)
		}
		// While holding, if we moved enough, consider it a drag and scroll
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && g.champStripArea.hit(mx, my) {
			dx := mx - g.champDragStartX
			if !g.champDragActive && (dx >= dragThresh || dx <= -dragThresh) {
				g.champDragActive = true
			}
			if g.champDragActive {
				maxStart := maxInt(0, len(g.champions)-cols)
				// emulate moveChampDrag inline so we don't miss frames
				step := mx - g.champDragLastX
				g.champDragLastX = mx
				g.champDragAccum += step
				for g.champDragAccum <= -stepPx && g.champStripScroll < maxStart {
					g.champStripScroll++
					g.champDragAccum += stepPx
				}
				for g.champDragAccum >= stepPx && g.champStripScroll > 0 {
					g.champStripScroll--
					g.champDragAccum -= stepPx
				}
			}
		}
		// On release: if NOT a drag (moved less than threshold), treat as click select
		if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
			if g.champStripArea.hit(mx, my) && !g.champDragActive && (mx-g.champDragStartX <= dragThresh && g.champDragStartX-mx <= dragThresh) {
				for i, r := range g.champStripRects {
					if r.hit(mx, my) {
						ch := g.champions[start+i].Name
						g.setChampArmyFromSelected()
						g.loadSelectedForChampion(ch)
						g.autoSaveCurrentChampionArmy()
						break
					}
				}
			}
			// reset drag state
			g.champDragActive = false
			g.champDragAccum = 0
		}

		// ---------- CHAMP STRIP: touch drag (kept simple) ----------
		touchIDs := ebiten.AppendTouchIDs(nil)
		if len(touchIDs) > 0 {
			tid := touchIDs[0]
			tx, ty := ebiten.TouchPosition(tid)
			_ = ty
			if g.activeTouchID == -1 {
				if g.champStripArea.hit(tx, stripY+1) {
					g.activeTouchID = tid
					g.champDragStartX = tx
					g.champDragLastX = tx
					g.champDragAccum = 0
					g.champDragActive = false
				}
			} else if g.activeTouchID == tid {
				dx := tx - g.champDragStartX
				if !g.champDragActive && (dx >= dragThresh || dx <= -dragThresh) {
					g.champDragActive = true
				}
				if g.champDragActive {
					maxStart := maxInt(0, len(g.champions)-cols)
					step := tx - g.champDragLastX
					g.champDragLastX = tx
					g.champDragAccum += step
					for g.champDragAccum <= -stepPx && g.champStripScroll < maxStart {
						g.champStripScroll++
						g.champDragAccum += stepPx
					}
					for g.champDragAccum >= stepPx && g.champStripScroll > 0 {
						g.champStripScroll--
						g.champDragAccum -= stepPx
					}
				}
			}
		} else if g.activeTouchID != -1 {
			// touch ended
			g.activeTouchID = -1
			g.champDragActive = false
			g.champDragAccum = 0
		}

		// ---------- SELECTED SLOTS: CLICK TO REMOVE ----------
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && !g.champDragActive {
			// [0] champion big card
			g.armySlotRects[0] = rect{x: leftX, y: topY, w: bigW, h: bigH}
			// [1..6] 2x3 grid to the right
			k := 1
			for row := 0; row < 2; row++ {
				for col := 0; col < 3; col++ {
					g.armySlotRects[k] = rect{
						x: rightX + col*(miniCardW+gap),
						y: topY + row*(miniCardH+gap),
						w: miniCardW, h: miniCardH,
					}
					k++
				}
			}
			for i := 1; i <= 6; i++ {
				if g.armySlotRects[i].hit(mx, my) {
					order := g.selectedMinisList()
					if i-1 < len(order) {
						delete(g.selectedMinis, order[i-1])
						g.setChampArmyFromSelected()
						g.autoSaveCurrentChampionArmy()
					}
					return
				}
			}
		}

		// ---------- MINIS COLLECTION: dims / page ----------
		gridTop := topY + bigH + 16
		gridLeft := pad
		gridRight := protocol.ScreenW - pad
		gridW := gridRight - gridLeft
		gridH := protocol.ScreenH - menuBarH - pad - gridTop
		visRows := maxInt(1, (gridH+gap)/(miniCardH+gap))
		cols2 := maxInt(1, (gridW+gap)/(miniCardW+gap))
		g.collArea = rect{x: gridLeft, y: gridTop, w: gridW, h: gridH}
		totalRows := (len(g.minisOnly) + cols2 - 1) / cols2
		maxRowsStart := maxInt(0, totalRows-visRows)
		stepPy := miniCardH + gap

		// ---------- MINIS: mouse drag vs click (with threshold) ----------
		// Arm drag on press
		if g.collArea.hit(mx, my) && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			g.collDragStartY = my
			g.collDragLastY = my
			g.collDragAccum = 0
			g.collDragActive = false // arm but not active
		}
		// While holding, activate drag after threshold and scroll rows
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && g.collArea.hit(mx, my) {
			dy0 := my - g.collDragStartY
			if !g.collDragActive && (dy0 >= dragThresh || dy0 <= -dragThresh) {
				g.collDragActive = true
			}
			if g.collDragActive {
				dy := my - g.collDragLastY
				g.collDragLastY = my
				g.collDragAccum += dy
				for g.collDragAccum <= -stepPy && g.collScroll < maxRowsStart {
					g.collScroll++
					g.collDragAccum += stepPy
				}
				for g.collDragAccum >= stepPy && g.collScroll > 0 {
					g.collScroll--
					g.collDragAccum -= stepPy
				}
			}
		}
		// Build visible rects BEFORE we decide tap vs drag on release
		items := g.minisOnly
		start2 := g.collScroll * cols2
		g.collRects = g.collRects[:0]
		maxItems := visRows * cols2
		for i := 0; i < maxItems && start2+i < len(items); i++ {
			c := i % cols2
			rw := i / cols2
			x := gridLeft + c*(miniCardW+gap)
			y := gridTop + rw*(miniCardH+gap)
			g.collRects = append(g.collRects, rect{x: x, y: y, w: miniCardW, h: miniCardH})
		}
		// On release: if NOT a drag (moved less than threshold), treat as click-to-toggle
		if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) && g.collArea.hit(mx, my) && !g.collDragActive &&
			(my-g.collDragStartY <= dragThresh && g.collDragStartY-my <= dragThresh) {
			for i, r := range g.collRects {
				if r.hit(mx, my) {
					idx := start2 + i
					if idx >= 0 && idx < len(items) {
						name := items[idx].Name
						if g.selectedMinis[name] {
							delete(g.selectedMinis, name)
						} else if len(g.selectedMinis) < 6 {
							g.selectedMinis[name] = true
						}
						g.setChampArmyFromSelected()
						g.autoSaveCurrentChampionArmy()
					}
					break
				}
			}
		}
		// Finally reset minis drag state on mouse release (regardless of click or drag)
		if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
			g.collDragActive = false
			g.collDragAccum = 0
		}

		// ---------- MINIS: touch drag (vertical) ----------
		tids := ebiten.AppendTouchIDs(nil)
		if len(tids) > 0 {
			tid := tids[0]
			tx, ty := ebiten.TouchPosition(tid)
			if g.collTouchID == -1 {
				if g.collArea.hit(tx, ty) {
					g.collTouchID = tid
					g.collDragStartY = ty
					g.collDragLastY = ty
					g.collDragAccum = 0
					g.collDragActive = false
				}
			} else if g.collTouchID == tid {
				dy0 := ty - g.collDragStartY
				if !g.collDragActive && (dy0 >= dragThresh || dy0 <= -dragThresh) {
					g.collDragActive = true
				}
				if g.collDragActive {
					dy := ty - g.collDragLastY
					g.collDragLastY = ty
					g.collDragAccum += dy
					for g.collDragAccum <= -stepPy && g.collScroll < maxRowsStart {
						g.collScroll++
						g.collDragAccum += stepPy
					}
					for g.collDragAccum >= stepPy && g.collScroll > 0 {
						g.collScroll--
						g.collDragAccum -= stepPy
					}
				}
			}
		} else if g.collTouchID != -1 {
			// touch ended
			g.collTouchID = -1
			g.collDragActive = false
			g.collDragAccum = 0
		}

		// ---------- WHEEL SCROLL (kept) ----------
		if _, wY := ebiten.Wheel(); wY != 0 {
			if g.champStripArea.hit(mx, my) {
				g.champStripScroll -= int(wY)
				maxStart := maxInt(0, len(g.champions)-cols)
				g.champStripScroll = clampInt(g.champStripScroll, 0, maxStart)
			} else if g.collArea.hit(mx, my) {
				g.collScroll -= int(wY)
				if g.collScroll < 0 {
					g.collScroll = 0
				}
				totalRows := (len(g.minisOnly) + cols2 - 1) / cols2
				maxRowsStart := maxInt(0, totalRows-visRows)
				if g.collScroll > maxRowsStart {
					g.collScroll = maxRowsStart
				}
			}
		}
	case tabMap:
		g.ensureMapHotspots()

		mx, my = ebiten.CursorPosition()
		disp := g.displayMapID()
		bg := g.ensureBgForMap(disp)
		offX, offY, dispW, dispH, _ := g.mapRenderRect(bg)

		// Hover (reset first)
		g.hoveredHS = -1
		hsList := g.mapHotspots[disp]
		if hsList == nil {
			hsList = g.mapHotspots[defaultMapID]
		}
		for i, hs := range hsList {
			cx := offX + int(hs.X*float64(dispW))
			cy := offY + int(hs.Y*float64(dispH))
			dx, dy := mx-cx, my-cy
			if dx*dx+dy*dy <= hs.Rpx*hs.Rpx {
				g.hoveredHS = i
				break
			}
		}

		// Select HS / create room / Start button
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if g.hoveredHS >= 0 {
				g.selectedHS = g.hoveredHS

				// decide arena id for this hotspot
				hs := hsList[g.selectedHS]
				arenaID := hs.TargetMapID
				if arenaID == "" {
					arenaID = g.arenaForHotspot(disp, hs.ID)
				}

				// Create the room ONCE and remember the arena
				g.onMapClicked(arenaID) // sets currentArena + sends CreatePve
			}

			// Start button click
			if g.selectedHS >= 0 && g.selectedHS < len(hsList) {
				hs := hsList[g.selectedHS]
				cx := offX + int(hs.X*float64(dispW))
				cy := offY + int(hs.Y*float64(dispH))
				g.startBtn = rect{x: cx + 22, y: cy - 16, w: 90, h: 28}
				if g.startBtn.hit(mx, my) {
					g.onStartBattle() // only StartBattle here
				}
			}
		}
		// Right-click anywhere on the map to print normalized coords for tuning
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) &&
			mx >= offX && mx < offX+dispW && my >= offY && my < offY+dispH {
			nx := (float64(mx - offX)) / float64(dispW)
			ny := (float64(my - offY)) / float64(dispH)
			log.Printf("map '%s' pick: X: %.4f, Y: %.4f", disp, nx, ny)
		}
	case tabPvp:
		// layout
		queueBtn, leaveBtn, createBtn, cancelBtn, joinInput, joinBtn := g.pvpLayout()
		mx, my := ebiten.CursorPosition()

		// mouse clicks
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			switch {
			case queueBtn.hit(mx, my):
				if !g.pvpQueued {
					g.pvpQueued = true
					g.pvpStatus = "Queueing for PvP…"
					g.send("JoinPvpQueue", struct{}{})
				}
			case leaveBtn.hit(mx, my):
				if g.pvpQueued {
					g.pvpQueued = false
					g.pvpStatus = "Left queue."
					g.send("LeavePvpQueue", struct{}{})
				}
			case createBtn.hit(mx, my):
				if !g.pvpHosting {
					g.pvpHosting = true
					g.pvpCode = ""
					g.pvpStatus = "Requesting friendly code…"
					g.send("FriendlyCreate", protocol.FriendlyCreate{})
				}
			case cancelBtn.hit(mx, my):
				if g.pvpHosting {
					g.pvpHosting = false
					g.pvpStatus = "Cancelled friendly host."
					g.pvpCode = ""
					g.send("FriendlyCancel", protocol.FriendlyCancel{})
				}
			case joinInput.hit(mx, my):
				g.pvpInputActive = true
			case g.pvpCodeArea.hit(mx, my) && g.pvpHosting && g.pvpCode != "":
				if err := clipboard.WriteAll(g.pvpCode); err != nil {
					// Atotto needs a clipboard backend on Linux (xclip/xsel).
					// Surface the problem instead of silently failing.
					g.pvpStatus = "Couldn’t copy (install xclip/xsel on Linux)."
					log.Println("clipboard copy failed:", err)
				} else {
					g.pvpStatus = "Code copied to clipboard."
				}
			case joinBtn.hit(mx, my):
				code := strings.ToUpper(strings.TrimSpace(g.pvpCodeInput))
				if code == "" {
					g.pvpStatus = "Enter a code first."
				} else {
					g.pvpStatus = "Joining room " + code + "…"
					g.send("FriendlyJoin", protocol.FriendlyJoin{Code: code})
				}
				g.pvpInputActive = false
			default:
				// click elsewhere removes focus from input
				g.pvpInputActive = false
			}
		}

		// keyboard input for the code field
		if g.pvpInputActive {
			for _, k := range inpututil.AppendJustPressedKeys(nil) {
				switch k {
				case ebiten.KeyBackspace:
					if len(g.pvpCodeInput) > 0 {
						g.pvpCodeInput = g.pvpCodeInput[:len(g.pvpCodeInput)-1]
					}
				case ebiten.KeyEnter:
					code := strings.ToUpper(strings.TrimSpace(g.pvpCodeInput))
					if code == "" {
						g.pvpStatus = "Enter a code first."
					} else {
						g.pvpStatus = "Joining room " + code + "…"
						g.send("FriendlyJoin", protocol.FriendlyJoin{Code: code})
					}
					g.pvpInputActive = false

				default:
					// Letters A–Z
					if k >= ebiten.KeyA && k <= ebiten.KeyZ {
						if len(g.pvpCodeInput) < 8 {
							g.pvpCodeInput += string('A' + (k - ebiten.KeyA))
						}
						continue
					}
					// Digits 0–9 (top row)
					if k >= ebiten.Key0 && k <= ebiten.Key9 {
						if len(g.pvpCodeInput) < 8 {
							g.pvpCodeInput += string('0' + (k - ebiten.Key0))
						}
						continue
					}
					// Digits 0–9 (numpad)
					if k >= ebiten.KeyKP0 && k <= ebiten.KeyKP9 {
						if len(g.pvpCodeInput) < 8 {
							g.pvpCodeInput += string('0' + (k - ebiten.KeyKP0))
						}
						continue
					}
				}
			}
		}
		// Periodically refresh leaderboard while on PvP tab
		if time.Since(g.lbLastReq) > 10*time.Second {
			g.send("GetLeaderboard", protocol.GetLeaderboard{})
			g.lbLastReq = time.Now()
		}

	case tabSettings:
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if g.fsOnBtn.hit(mx, my) {
				ebiten.SetFullscreen(true)
				g.fullscreen = true
			}
			if g.fsOffBtn.hit(mx, my) {
				ebiten.SetFullscreen(false)
				g.fullscreen = false
			}
		}
	}
}

func (g *Game) rowsPerCol() int {
	contentTop := pad + 40
	contentH := protocol.ScreenH - menuBarH - pad - contentTop
	return maxInt(1, contentH/rowH)
}

func (g *Game) trySaveArmy() {
	names := g.buildArmyNames()
	if msg := g.validateArmy(names); msg != "" {
		g.armyMsg = msg
		return
	}
	g.armyMsg = "Saved!"
	g.onArmySave(names)
}

func (g *Game) buildArmyNames() []string {
	out := make([]string, 0, 7)
	if g.selectedChampion != "" {
		out = append(out, g.selectedChampion)
	}
	// deterministic order for minis
	minis := make([]string, 0, len(g.selectedMinis))
	for _, m := range g.minisOnly {
		if g.selectedMinis[m.Name] {
			minis = append(minis, m.Name)
		}
		if len(minis) == 6 {
			break
		}
	}
	out = append(out, minis...)
	return out
}

func (g *Game) validateArmy(names []string) string {
	if len(names) != 7 {
		return "Select exactly 1 Champion and 6 Minis."
	}
	info := map[string]protocol.MiniInfo{}
	for _, m := range g.minisAll {
		info[m.Name] = m
	}
	// first must be champion
	first := info[names[0]]
	if !(strings.EqualFold(first.Role, "champion") || strings.EqualFold(first.Class, "champion")) {
		return "First card must be a Champion."
	}
	// next 6 must be minis non-spell
	for i := 1; i < 7; i++ {
		m := info[names[i]]
		if !strings.EqualFold(m.Role, "mini") || strings.EqualFold(m.Class, "spell") {
			return "Slots 2..7 must be Minis (non-spell)."
		}
	}
	return ""
}

// ---------- Battle ----------

func (g *Game) updateBattle() {
	// If end overlay is up, only the Continue button is active
	if g.endActive {
		// compute the same rect as Draw
		w, h := 360, 160
		x := (protocol.ScreenW - w) / 2
		y := (protocol.ScreenH - h) / 2
		g.continueBtn = rect{x: x + (w-120)/2, y: y + h - 50, w: 120, h: 32}

		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			mx, my := ebiten.CursorPosition()
			if g.continueBtn.hit(mx, my) {
				g.onLeaveRoom()
				g.roomID = ""
				g.scr = screenHome

				// clear all battle state
				g.endActive = false
				g.endVictory = false
				g.gameOver = false
				g.victory = false
				g.hand = nil
				g.next = protocol.MiniCardView{}
				g.selectedIdx = -1
				g.dragActive = false
				g.world = &World{
					Units: make(map[int64]*RenderUnit),
					Bases: make(map[int64]protocol.BaseState),
				}
			}
		}
		return
	}

	// use Ebiten's cursor directly (home already works with this)
	mx, my := ebiten.CursorPosition()
	handTop := protocol.ScreenH - battleHUDH

	// select/drag from a card
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		for i, r := range g.handRects() {
			if r.hit(mx, my) {
				g.selectedIdx = i
				g.dragActive = true
				g.dragIdx = i
				g.dragStartX, g.dragStartY = mx, my
				return
			}
		}
		// click-to-deploy on the field
		if my < handTop && g.selectedIdx >= 0 && g.selectedIdx < len(g.hand) {
			g.send("DeployMiniAt", protocol.DeployMiniAt{
				CardIndex: g.selectedIdx,
				X:         float64(mx),
				Y:         float64(my),
				ClientTs:  time.Now().UnixMilli(),
			})
			return
		}
	}

	// drag release → deploy
	if g.dragActive && inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		if my < handTop && g.dragIdx >= 0 && g.dragIdx < len(g.hand) {
			g.send("DeployMiniAt", protocol.DeployMiniAt{
				CardIndex: g.dragIdx,
				X:         float64(mx),
				Y:         float64(my),
				ClientTs:  time.Now().UnixMilli(),
			})
		}
		g.dragActive = false
		g.dragIdx = -1
		return
	}
}

// ---------- Draw ----------

func (g *Game) Draw(screen *ebiten.Image) {
	screen.Fill(color.NRGBA{20, 20, 24, 255})
	// Loading/connection overlay
	//if g.connSt == stateConnecting || g.connSt == stateFailed {
	if g.connSt != stateConnected {

		// Dim background
		//overlay := ebiten.NewImage(protocol.ScreenW, protocol.ScreenH)
		//overlay.Fill(color.NRGBA{0, 0, 0, 140})
		//screen.DrawImage(overlay, nil)
		ebitenutil.DrawRect(
			screen,
			0, 0,
			float64(protocol.ScreenW), float64(protocol.ScreenH),
			color.NRGBA{0, 0, 0, 140},
		)
		// Panel
		w, h := 420, 140
		x := (protocol.ScreenW - w) / 2
		y := (protocol.ScreenH - h) / 2
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), color.NRGBA{32, 32, 44, 255})

		if g.connSt == stateConnecting {
			text.Draw(screen, "Connecting to server…", basicfont.Face7x13, x+20, y+46, color.White)
			text.Draw(screen, netcfg.ServerURL, basicfont.Face7x13, x+20, y+66, color.NRGBA{200, 200, 210, 255})
		} else {
			text.Draw(screen, "Unable to connect to server", basicfont.Face7x13, x+20, y+46, color.NRGBA{255, 120, 120, 255})
			if g.connErrMsg != "" {
				text.Draw(screen, g.connErrMsg, basicfont.Face7x13, x+20, y+66, color.NRGBA{220, 200, 200, 255})
			}
			text.Draw(screen, "Press R (desktop) or Tap (mobile) to retry", basicfont.Face7x13, x+20, y+96, color.NRGBA{220, 220, 220, 255})
		}

		// While overlay is up, don't draw the rest of the game
		return
	}
if !g.profileOpen {
    g.profCloseBtn = rect{}
    g.profLogoutBtn = rect{}
}
	if g.scr == screenBattle {
		g.drawArenaBG(screen)
	}
	nowMs := time.Now().UnixMilli()

	// Bases
	for id, b := range g.world.Bases {
		g.assets.ensureInit()
		img := g.assets.baseEnemy
		if b.OwnerID == g.playerID {
			img = g.assets.baseMe
		}
		drawBaseImg(screen, img, b)

		if b.MaxHP > 0 {
			fx := g.hpfxStep(g.hpFxBases, id, b.HP, nowMs)
			g.drawHPBar(screen,
				float64(b.X), float64(b.Y-6),
				float64(b.W), 4,
				b.HP, b.MaxHP, fx.ghostHP)
		}
	}

	// Units
	const unitTargetPX = 42.0
	for id, u := range g.world.Units {
		if img := g.ensureMiniImageByName(u.Name); img != nil {
			op := &ebiten.DrawImageOptions{}
			iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
			s := unitTargetPX / float64(maxInt(1, maxInt(iw, ih)))
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(u.X-float64(iw)*s/2, u.Y-float64(ih)*s/2)
			screen.DrawImage(img, op)
		} else {
			ebitenutil.DrawRect(screen, u.X-6, u.Y-6, 12, 12, color.White)
		}

		if u.MaxHP > 0 {
			barW := 26.0
			bx := u.X - barW/2
			by := u.Y - unitTargetPX/2 - 6

			fx := g.hpfxStep(g.hpFxUnits, id, u.HP, nowMs)
			g.drawHPBar(screen, bx, by, barW, 3, u.HP, u.MaxHP, fx.ghostHP)
		}
	}

	//	text.Draw(screen, "War Rumble — by s.robev", basicfont.Face7x13, pad, 16, color.White)

	switch g.scr {
	case screenLogin:
		text.Draw(screen, "Enter username, then press Enter:", basicfont.Face7x13, pad, 48, color.White)
		ebitenutil.DrawRect(screen, float64(pad), 54, 260, 18, color.NRGBA{40, 40, 40, 255})
		text.Draw(screen, g.nameInput, basicfont.Face7x13, pad+4, 68, color.White)

	case screenHome:
		g.drawHomeContent(screen)
		g.drawTopBarHome(screen)
		g.drawBottomBar(screen)
		// If profile overlay is open, only handle its interactions and block others
		if g.showProfile {
			g.drawProfileOverlay(screen)
		}

	case screenBattle:
		g.drawBattleBar(screen)
	}

	// End overlay only during battle
	if g.scr == screenBattle && g.endActive {
		overlay := ebiten.NewImage(protocol.ScreenW, protocol.ScreenH)
		overlay.Fill(color.NRGBA{0, 0, 0, 140})
		screen.DrawImage(overlay, nil)

		w, h := 360, 160
		x := (protocol.ScreenW - w) / 2
		y := (protocol.ScreenH - h) / 2
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), color.NRGBA{30, 30, 45, 240})

		title := "Defeat"
		if g.endVictory {
			title = "Victory!"
		}
		text.Draw(screen, title, basicfont.Face7x13, x+20, y+40, color.White)

		g.continueBtn = rect{x: x + (w-120)/2, y: y + h - 50, w: 120, h: 32}
		ebitenutil.DrawRect(screen, float64(g.continueBtn.x), float64(g.continueBtn.y), float64(g.continueBtn.w), float64(g.continueBtn.h), color.NRGBA{70, 110, 70, 255})
		text.Draw(screen, "Continue", basicfont.Face7x13, g.continueBtn.x+18, g.continueBtn.y+20, color.White)
	}
}

func (g *Game) drawHomeContent(screen *ebiten.Image) {
	switch g.activeTab {
	case tabArmy:
		// Layout numbers (mirror Update)
		const champCardW, champCardH = 100, 116
		const miniCardW, miniCardH = 100, 116
		const gap = 10
		const stripH = champCardH + 8

		stripX := pad
		stripY := topBarH + pad
		stripW := protocol.ScreenW - 2*pad
		g.champStripArea = rect{x: stripX, y: stripY, w: stripW, h: stripH}

		cols := maxInt(1, (stripW+gap)/(champCardW+gap))
		start := clampInt(g.champStripScroll, 0, maxInt(0, len(g.champions)-cols))
		g.champStripRects = g.champStripRects[:0]
		for i := 0; i < cols && start+i < len(g.champions); i++ {
			x := stripX + i*(champCardW+gap)
			r := rect{x: x, y: stripY, w: champCardW, h: champCardH}
			g.champStripRects = append(g.champStripRects, r)

			it := g.champions[start+i]
			// panel
			ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{0x2b, 0x2b, 0x3e, 0xff})
			// portrait
			if img := g.ensureMiniImageByName(it.Name); img != nil {
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
				pw, ph := float64(r.w-8), float64(r.h-24)
				s := mathMin(pw/float64(iw), ph/float64(ih))
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(s, s)
				op.GeoM.Translate(float64(r.x)+4, float64(r.y)+4)
				screen.DrawImage(img, op)
			}
			// cost + name
			text.Draw(screen, fmt.Sprintf("%d", it.Cost), basicfont.Face7x13, r.x+6, r.y+14, color.NRGBA{240, 196, 25, 255})
			text.Draw(screen, trim(it.Name, 14), basicfont.Face7x13, r.x+6, r.y+r.h-6, color.NRGBA{239, 229, 182, 255})

			// active highlight
			if g.selectedChampion == it.Name {
				ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), 2, color.NRGBA{240, 196, 25, 255})
			}
		}

		// Big champion card + Selected Minis (2x3)
		topY := stripY + stripH + 12
		leftX := pad
		bigW := 200
		bigH := miniCardH*2 + gap

		// big champ card
		chRect := rect{x: leftX, y: topY, w: bigW, h: bigH}
		ebitenutil.DrawRect(screen, float64(chRect.x), float64(chRect.y), float64(chRect.w), float64(chRect.h), color.NRGBA{0x2b, 0x2b, 0x3e, 0xff})
		if g.selectedChampion != "" {
			if img := g.ensureMiniImageByName(g.selectedChampion); img != nil {
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
				pw, ph := float64(chRect.w-8), float64(chRect.h-24)
				s := mathMin(pw/float64(iw), ph/float64(ih))
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(s, s)
				op.GeoM.Translate(float64(chRect.x)+4, float64(chRect.y)+4)
				screen.DrawImage(img, op)
			}
			text.Draw(screen, "Champion", basicfont.Face7x13, chRect.x+6, chRect.y+14, color.NRGBA{240, 196, 25, 255})
			text.Draw(screen, trim(g.selectedChampion, 18), basicfont.Face7x13, chRect.x+6, chRect.y+chRect.h-6, color.NRGBA{239, 229, 182, 255})
		} else {
			text.Draw(screen, "Champion (select above)", basicfont.Face7x13, chRect.x+6, chRect.y+18, color.NRGBA{200, 200, 200, 255})
		}

		// selected minis 2x3 grid to the right
		gridX := leftX + bigW + 16
		gridY := topY
		order := g.selectedMinisList()
		k := 0
		for row := 0; row < 2; row++ {
			for col := 0; col < 3; col++ {
				r := rect{
					x: gridX + col*(miniCardW+gap),
					y: gridY + row*(miniCardH+gap),
					w: miniCardW, h: miniCardH,
				}
				ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{0x26, 0x26, 0x35, 0xff})
				if k < len(order) {
					name := order[k]
					// filled slot
					if img := g.ensureMiniImageByName(name); img != nil {
						iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
						pw, ph := float64(r.w-8), float64(r.h-24)
						s := mathMin(pw/float64(iw), ph/float64(ih))
						op := &ebiten.DrawImageOptions{}
						op.GeoM.Scale(s, s)
						op.GeoM.Translate(float64(r.x)+4, float64(r.y)+4)
						screen.DrawImage(img, op)
					}
					info := g.nameToMini[name]
					text.Draw(screen, fmt.Sprintf("%d", info.Cost), basicfont.Face7x13, r.x+6, r.y+14, color.NRGBA{240, 196, 25, 255})
					text.Draw(screen, trim(name, 14), basicfont.Face7x13, r.x+6, r.y+r.h-6, color.NRGBA{239, 229, 182, 255})
				} else {
					// empty
					text.Draw(screen, "Mini", basicfont.Face7x13, r.x+6, r.y+14, color.NRGBA{200, 200, 200, 255})
					text.Draw(screen, "(empty)", basicfont.Face7x13, r.x+6, r.y+r.h-6, color.NRGBA{160, 160, 160, 255})
				}
				k++
			}
		}
		text.Draw(screen, fmt.Sprintf("Minis: %d/6", len(order)), basicfont.Face7x13, gridX, gridY-6, color.White)

		// Minis collection grid below
		gridTop := topY + bigH + 16
		gridLeft := pad
		gridRight := protocol.ScreenW - pad
		gridW := gridRight - gridLeft
		cols2 := maxInt(1, (gridW+gap)/(miniCardW+gap))
		gridH := protocol.ScreenH - menuBarH - pad - gridTop
		visRows := maxInt(1, (gridH+gap)/(miniCardH+gap))
		g.collArea = rect{x: gridLeft, y: gridTop, w: gridW, h: gridH}

		start2 := g.collScroll * cols2
		g.collRects = g.collRects[:0]
		maxItems := visRows * cols2
		for i := 0; i < maxItems && start2+i < len(g.minisOnly); i++ {
			c := i % cols2
			rw := i / cols2
			x := gridLeft + c*(miniCardW+gap)
			y := gridTop + rw*(miniCardH+gap)
			r := rect{x: x, y: y, w: miniCardW, h: miniCardH}
			g.collRects = append(g.collRects, r)

			it := g.minisOnly[start2+i]
			// panel
			ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{0x2b, 0x2b, 0x3e, 0xff})
			// portrait
			if img := g.ensureMiniImageByName(it.Name); img != nil {
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
				pw, ph := float64(r.w-8), float64(r.h-24)
				s := mathMin(pw/float64(iw), ph/float64(ih))
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(s, s)
				op.GeoM.Translate(float64(r.x)+4, float64(r.y)+4)
				screen.DrawImage(img, op)
			}
			// cost + name
			text.Draw(screen, fmt.Sprintf("%d", it.Cost), basicfont.Face7x13, r.x+6, r.y+14, color.NRGBA{240, 196, 25, 255})
			text.Draw(screen, trim(it.Name, 14), basicfont.Face7x13, r.x+6, r.y+r.h-6, color.NRGBA{239, 229, 182, 255})

			// highlight if selected
			if g.selectedMinis[it.Name] {
				ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), 2, color.NRGBA{240, 196, 25, 255})
			}
		}

		// any status message
		if g.armyMsg != "" {
			text.Draw(screen, g.armyMsg, basicfont.Face7x13, pad, protocol.ScreenH-menuBarH-24, color.White)
		}
	case tabMap:
		disp := g.displayMapID()
		bg := g.ensureBgForMap(disp)
		offX, offY, dispW, dispH, s := g.mapRenderRect(bg)

		// Letterbox bars that match the image edge color
		if bg != nil {
			c := g.mapEdgeColor(disp, bg)
			if offY > 0 {
				ebitenutil.DrawRect(screen, 0, 0, float64(protocol.ScreenW), float64(offY), c)
				ebitenutil.DrawRect(screen, 0, float64(offY+dispH),
					float64(protocol.ScreenW), float64(protocol.ScreenH-(offY+dispH)), c)
			}
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(float64(offX), float64(offY))
			screen.DrawImage(bg, op)
		}

		// Title
		text.Draw(screen, "Map — click a location, then press Start",
			basicfont.Face7x13, pad, topBarH-6, color.White)

		// Hotspots
		g.ensureMapHotspots()
		hsList := g.mapHotspots[disp]
		if hsList == nil {
			hsList = g.mapHotspots[defaultMapID]
		}
		for i, hs := range hsList {
			cx := offX + int(hs.X*float64(dispW))
			cy := offY + int(hs.Y*float64(dispH))

			col := color.NRGBA{0x66, 0x99, 0xcc, 0xff} // base
			if i == g.hoveredHS {
				col = color.NRGBA{0xa0, 0xd0, 0xff, 0xff}
			}
			if i == g.selectedHS {
				col = color.NRGBA{240, 196, 25, 255}
			}

			// tiny center dot only (no big squares)
			ebitenutil.DrawRect(screen, float64(cx-2), float64(cy-2), 4, 4, col)
		}

		// Tooltip (guard against empty list)
		if g.hoveredHS >= 0 && g.hoveredHS < len(hsList) {
			hs := hsList[g.hoveredHS]
			mx, my := ebiten.CursorPosition()
			w, h := 260, 46
			x := clampInt(mx+14, 0, protocol.ScreenW-w)
			y := clampInt(my-8-h, 0, protocol.ScreenH-h)
			ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h),
				color.NRGBA{30, 30, 45, 240})
			text.Draw(screen, hs.Name, basicfont.Face7x13, x+8, y+18, color.White)
			if hs.Info != "" {
				text.Draw(screen, hs.Info, basicfont.Face7x13, x+8, y+34, color.NRGBA{200, 200, 200, 255})
			}
		}

		// Start button (guard against empty list)
		if g.selectedHS >= 0 && g.selectedHS < len(hsList) {
			hs := hsList[g.selectedHS]
			cx := offX + int(hs.X*float64(dispW))
			cy := offY + int(hs.Y*float64(dispH))
			g.startBtn = rect{x: cx + 22, y: cy - 16, w: 90, h: 28}

			btnCol := color.NRGBA{70, 110, 70, 255}
			label := "Start"
			if g.roomID == "" {
				btnCol = color.NRGBA{110, 110, 70, 255}
				label = "Start…"
			}
			ebitenutil.DrawRect(screen, float64(g.startBtn.x), float64(g.startBtn.y),
				float64(g.startBtn.w), float64(g.startBtn.h), btnCol)
			text.Draw(screen, label, basicfont.Face7x13, g.startBtn.x+18, g.startBtn.y+18, color.White)
		}
	case tabPvp:
		// Wipe the content area so logs/other UI never show through
		contentY := topBarH
		contentH := protocol.ScreenH - menuBarH - contentY
		ebitenutil.DrawRect(screen, 0, float64(contentY), float64(protocol.ScreenW), float64(contentH), color.NRGBA{0x20, 0x20, 0x28, 0xFF})

		queueBtn, leaveBtn, createBtn, cancelBtn, joinInput, joinBtn := g.pvpLayout()

		// Matchmaking row
		ebitenutil.DrawRect(screen, float64(queueBtn.x), float64(queueBtn.y), float64(queueBtn.w), float64(queueBtn.h),
			map[bool]color.NRGBA{true: {80, 110, 80, 255}, false: {60, 60, 80, 255}}[g.pvpQueued])
		text.Draw(screen, "Queue PvP", basicfont.Face7x13, queueBtn.x+16, queueBtn.y+18, color.White)

		ebitenutil.DrawRect(screen, float64(leaveBtn.x), float64(leaveBtn.y), float64(leaveBtn.w), float64(leaveBtn.h),
			color.NRGBA{90, 70, 70, 255})
		text.Draw(screen, "Leave Queue", basicfont.Face7x13, leaveBtn.x+16, leaveBtn.y+18, color.White)

		// Friendly host row
		ebitenutil.DrawRect(screen, float64(createBtn.x), float64(createBtn.y), float64(createBtn.w), float64(createBtn.h),
			map[bool]color.NRGBA{true: {80, 110, 80, 255}, false: {60, 60, 80, 255}}[g.pvpHosting])
		text.Draw(screen, "Create Friendly Code", basicfont.Face7x13, createBtn.x+16, createBtn.y+18, color.White)

		ebitenutil.DrawRect(screen, float64(cancelBtn.x), float64(cancelBtn.y), float64(cancelBtn.w), float64(cancelBtn.h),
			color.NRGBA{90, 70, 70, 255})
		text.Draw(screen, "Cancel", basicfont.Face7x13, cancelBtn.x+16, cancelBtn.y+18, color.White)

		// Show the code we’re hosting (if any) as a clickable badge
		g.pvpCodeArea = rect{} // reset each frame
		if g.pvpHosting && g.pvpCode != "" {
			msg := "Your code: " + g.pvpCode

			// Size a neat pill around the text
			lb := text.BoundString(basicfont.Face7x13, msg)
			bx := createBtn.x
			by := createBtn.y + createBtn.h + 12
			bw := lb.Dx() + 18
			bh := 26

			// store hitbox
			g.pvpCodeArea = rect{x: bx, y: by, w: bw, h: bh}

			// draw pill
			ebitenutil.DrawRect(screen, float64(bx), float64(by), float64(bw), float64(bh), color.NRGBA{54, 63, 88, 255})
			text.Draw(screen, msg, basicfont.Face7x13, bx+9, by+18, color.White)

			// small hint
			// existing pill draw above…
			hintX := bx + bw + 12
			hintY := by + (bh+13)/2 - 2
			text.Draw(screen, "Click to copy", basicfont.Face7x13, hintX, hintY, color.NRGBA{160, 160, 170, 255})
		}

		// Join-with-code row
		ebitenutil.DrawRect(screen, float64(joinInput.x), float64(joinInput.y), float64(joinInput.w), float64(joinInput.h),
			color.NRGBA{38, 38, 53, 255})
		label := g.pvpCodeInput
		if label == "" && !g.pvpInputActive {
			label = "Enter code..."
			text.Draw(screen, label, basicfont.Face7x13, joinInput.x+8, joinInput.y+17, color.NRGBA{150, 150, 160, 255})
		} else {
			text.Draw(screen, label, basicfont.Face7x13, joinInput.x+8, joinInput.y+17, color.White)
		}

		ebitenutil.DrawRect(screen, float64(joinBtn.x), float64(joinBtn.y), float64(joinBtn.w), float64(joinBtn.h),
			color.NRGBA{70, 110, 70, 255})
		text.Draw(screen, "Join", basicfont.Face7x13, joinBtn.x+38, joinBtn.y+18, color.White)

		// after you draw all buttons + (optional) pill + join row, add:
		bottomY := joinBtn.y + joinBtn.h
		sepY := bottomY + 20

		// thin separator line
		ebitenutil.DrawRect(screen, float64(pad), float64(sepY), float64(protocol.ScreenW-2*pad), 1, color.NRGBA{90, 90, 120, 255})

		// status panel
		panelY := sepY + 14
		panelH := 54
		ebitenutil.DrawRect(screen, float64(pad), float64(panelY), float64(protocol.ScreenW-2*pad), float64(panelH), color.NRGBA{0x24, 0x24, 0x30, 0xFF})
		text.Draw(screen, "Status", basicfont.Face7x13, pad+8, panelY+18, color.NRGBA{240, 196, 25, 255})

		msg := g.pvpStatus
		if msg == "" {
			msg = "—"
		}
		text.Draw(screen, msg, basicfont.Face7x13, pad+8, panelY+36, color.White)
		// ---- Leaderboard panel (bottom area above bottom bar) ----
		panelPad := pad
		// fixed height (fits 50 rows + header nicely)
		rows := minInt(50, len(g.pvpLeaders))
		const rowH = 16
		panelH = 16 + 16 + rows*rowH + 8 // title + header + rows + bottom pad
		if panelH < 120 {
			panelH = 120
		}

		panelTop := protocol.ScreenH - menuBarH - panelH - 8
		if panelTop < topBarH+180 {
			panelTop = topBarH + 180 // keep PvP controls visible
		}

		// background
		ebitenutil.DrawRect(screen, float64(panelPad), float64(panelTop),
			float64(protocol.ScreenW-2*panelPad), float64(panelH), color.NRGBA{0x24, 0x24, 0x30, 0xFF})

		// title row
		text.Draw(screen, "Top 50 - PvP Leaderboard", basicfont.Face7x13, panelPad+8, panelTop+18, color.White)
		if g.lbLastStamp != 0 {
			ts := time.UnixMilli(g.lbLastStamp).Format("15:04:05")
			text.Draw(screen, "as of "+ts, basicfont.Face7x13, panelPad+240, panelTop+18, color.NRGBA{170, 170, 180, 255})
		}

		// columns
		colRankX := panelPad + 8
		colNameX := panelPad + 58
		colRatX := panelPad + 360
		colTierX := panelPad + 440

		hdrY := panelTop + 36
		text.Draw(screen, "#", basicfont.Face7x13, colRankX, hdrY, color.NRGBA{200, 200, 210, 255})
		text.Draw(screen, "Player", basicfont.Face7x13, colNameX, hdrY, color.NRGBA{200, 200, 210, 255})
		text.Draw(screen, "Rating", basicfont.Face7x13, colRatX, hdrY, color.NRGBA{200, 200, 210, 255})
		text.Draw(screen, "Rank", basicfont.Face7x13, colTierX, hdrY, color.NRGBA{200, 200, 210, 255})

		// rows
		rowY := hdrY + 16
		maxRows := minInt(50, len(g.pvpLeaders))
		for i := 0; i < maxRows; i++ {
			e := g.pvpLeaders[i]
			y := rowY + i*rowH

			// zebra
			if i%2 == 0 {
				ebitenutil.DrawRect(screen, float64(panelPad+4), float64(y-12),
					float64(protocol.ScreenW-2*panelPad-8), rowH, color.NRGBA{0x28, 0x28, 0x36, 0xFF})
			}

			text.Draw(screen, fmt.Sprintf("%2d.", i+1), basicfont.Face7x13, colRankX, y, color.White)
			text.Draw(screen, trim(e.Name, 22), basicfont.Face7x13, colNameX, y, color.White)
			text.Draw(screen, fmt.Sprintf("%d", e.Rating), basicfont.Face7x13, colRatX, y, color.White)
			text.Draw(screen, e.Rank, basicfont.Face7x13, colTierX, y, color.NRGBA{240, 196, 25, 255})
		}

	case tabSettings:
		// ===== Settings content: Fullscreen toggle =====
		y := topBarH + pad + 40
		text.Draw(screen, "Settings", basicfont.Face7x13, pad, y-20, color.White)
		text.Draw(screen, "Fullscreen:", basicfont.Face7x13, pad, y, color.White)

		g.fsOnBtn = rect{x: pad + 120, y: y - 14, w: 80, h: 20}
		g.fsOffBtn = rect{x: g.fsOnBtn.x + 90, y: y - 14, w: 80, h: 20}

		onCol := color.NRGBA{70, 110, 70, 255}
		offCol := color.NRGBA{110, 70, 70, 255}
		neutral := color.NRGBA{60, 60, 80, 255}

		// ON button
		ebitenutil.DrawRect(screen, float64(g.fsOnBtn.x), float64(g.fsOnBtn.y), float64(g.fsOnBtn.w), float64(g.fsOnBtn.h),
			map[bool]color.NRGBA{true: onCol, false: neutral}[g.fullscreen])
		text.Draw(screen, "ON", basicfont.Face7x13, g.fsOnBtn.x+26, g.fsOnBtn.y+14, color.White)

		// OFF button
		ebitenutil.DrawRect(screen, float64(g.fsOffBtn.x), float64(g.fsOffBtn.y), float64(g.fsOffBtn.w), float64(g.fsOffBtn.h),
			map[bool]color.NRGBA{true: neutral, false: offCol}[g.fullscreen])
		text.Draw(screen, "OFF", basicfont.Face7x13, g.fsOffBtn.x+24, g.fsOffBtn.y+14, color.White)

		text.Draw(screen, "Tip: press F to toggle fullscreen", basicfont.Face7x13, pad, y+40, color.White)
	}
}

func (g *Game) drawBottomBar(screen *ebiten.Image) {
	y := protocol.ScreenH - menuBarH
	ebitenutil.DrawRect(screen, 0, float64(y), float64(protocol.ScreenW), float64(menuBarH), color.NRGBA{32, 32, 40, 255})
	ebitenutil.DrawRect(screen, 0, float64(y-1), float64(protocol.ScreenW), 1, color.NRGBA{90, 90, 120, 255})

	// compute once here so Draw and Update see the same rects
	g.computeBottomBarLayout()

	drawBtn := func(r rect, label string, active, enabled bool) {
		col := color.NRGBA{60, 60, 80, 255}
		if active {
			col = color.NRGBA{80, 80, 110, 255}
		}
		if !enabled {
			col = color.NRGBA{50, 50, 50, 255}
		}
		ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), col)

		// center text inside the rect
		lb := text.BoundString(basicfont.Face7x13, label)
		tx := r.x + (r.w-lb.Dx())/2
		ty := r.y + (r.h+13)/2 - 2
		txt := color.Color(color.White)
		if !enabled {
			txt = color.NRGBA{220, 220, 220, 255}
		}
		text.Draw(screen, label, basicfont.Face7x13, tx, ty, txt)
	}

	drawBtn(g.armyBtn, "Army", g.activeTab == tabArmy, true)
	drawBtn(g.mapBtn, "Map", g.activeTab == tabMap, true)
	drawBtn(g.pvpBtn, "PvP", g.activeTab == tabPvp, true)
	drawBtn(g.socialBtn, "Social", g.activeTab == tabSocial, false)
	drawBtn(g.settingsBtn, "Settings", g.activeTab == tabSettings, true)
}

func (g *Game) drawBattleBar(screen *ebiten.Image) {
	y := protocol.ScreenH - battleHUDH

	// Bottom HUD panel (tall)
	ebitenutil.DrawRect(screen, 0, float64(y), float64(protocol.ScreenW), float64(battleHUDH), color.NRGBA{0x1e, 0x1e, 0x29, 0xff})

	// --- Gold coins on the left ---
	g.assets.ensureInit()
	cx := 16
	cy := y + 20
	for i := 0; i < protocol.GoldMax; i++ {
		img := g.assets.coinEmpty
		if i < g.gold {
			img = g.assets.coinFull
		}
		if img != nil {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(cx), float64(cy))
			screen.DrawImage(img, op)
		} else {
			ebitenutil.DrawRect(screen, float64(cx), float64(cy), 20, 20, color.NRGBA{80, 80, 80, 255})
			if i < g.gold {
				ebitenutil.DrawRect(screen, float64(cx+2), float64(cy+2), 16, 16, color.NRGBA{240, 196, 25, 255})
			}
		}
		cx += 24
	}
	text.Draw(screen, fmt.Sprintf("%d/%d", g.gold, protocol.GoldMax), basicfont.Face7x13, cx+8, cy+15, color.NRGBA{239, 229, 182, 255})

	// --- 4 centered cards ---
	slots := g.handRects()
	for i, r := range slots {
		ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{0x2b, 0x2b, 0x3e, 0xff})
		if i == g.selectedIdx {
			ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), 2, color.NRGBA{240, 196, 25, 255})
		}
		if i < len(g.hand) {
			c := g.hand[i]
			if img := g.ensureMiniImageByName(c.Name); img != nil {
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
				pw := float64(r.w - 8)
				ph := float64(r.h - 24)
				s := mathMin(pw/float64(iw), ph/float64(ih))
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(s, s)
				op.GeoM.Translate(float64(r.x)+4, float64(r.y)+4)
				screen.DrawImage(img, op)
			}
			if c.Cost > g.gold {
				ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{0, 0, 0, 90})
			}
			text.Draw(screen, fmt.Sprintf("%d", c.Cost), basicfont.Face7x13, r.x+6, r.y+14, color.NRGBA{240, 196, 25, 255})
			text.Draw(screen, trim(c.Name, 14), basicfont.Face7x13, r.x+6, r.y+r.h-6, color.NRGBA{239, 229, 182, 255})
		}
	}

	// --- Next (right side) ---
	nextW := 70
	nextX := protocol.ScreenW - nextW - 16
	nextY := protocol.ScreenH - battleHUDH + 60
	nextR := rect{x: nextX, y: nextY, w: nextW, h: 116}
	text.Draw(screen, "Next", basicfont.Face7x13, nextX, nextY-6, color.NRGBA{140, 138, 163, 255})
	ebitenutil.DrawRect(screen, float64(nextR.x), float64(nextR.y), float64(nextR.w), float64(nextR.h), color.NRGBA{0x2b, 0x2b, 0x3e, 0xff})
	ebitenutil.DrawRect(screen, float64(nextR.x), float64(nextR.y), float64(nextR.w), float64(nextR.h), color.NRGBA{0, 0, 0, 70})
	if g.next.Name != "" {
		if img := g.ensureMiniImageByName(g.next.Name); img != nil {
			iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
			s := mathMin(float64(nextR.w-8)/float64(iw), float64(116-24)/float64(ih))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(float64(nextR.x)+4, float64(nextR.y)+4)
			screen.DrawImage(img, op)
		} else {
			text.Draw(screen, g.next.Name, basicfont.Face7x13, nextR.x+6, nextR.y+30, color.White)
		}
	}

	// --- Drag ghost ---
	if g.dragActive && g.dragIdx >= 0 && g.dragIdx < len(g.hand) {
		mx, my := ebiten.CursorPosition()
		if img := g.ensureMiniImageByName(g.hand[g.dragIdx].Name); img != nil {
			iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
			s := 0.5 * mathMin(1, 48.0/float64(maxInt(iw, ih)))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(float64(mx)-float64(iw)*s/2, float64(my)-float64(ih)*s/2)
			op.ColorScale.Scale(1, 1, 1, 0.75)
			screen.DrawImage(img, op)
		} else {
			ebitenutil.DrawRect(screen, float64(mx-12), float64(my-12), 24, 24, color.NRGBA{220, 220, 220, 200})
		}
	}
}

func (g *Game) handRects() []rect {
	// 4 cards centered at the bottom (like old hud.go)
	cardW, cardH := 100, 116
	gap := 10
	rowW := 4*cardW + 3*gap
	rowX := (protocol.ScreenW - rowW) / 2
	rowY := protocol.ScreenH - battleHUDH + 60

	n := len(g.hand)
	if n > 4 {
		n = 4
	}

	out := make([]rect, n)
	for i := 0; i < n; i++ {
		cx := rowX + i*(cardW+gap)
		cy := rowY
		out[i] = rect{x: cx, y: cy, w: cardW, h: cardH}
	}
	return out
}

// ---------- WS handlers & actions ----------

func (g *Game) handle(env Msg) {
	switch env.Type {
	case "Profile":
		var p protocol.Profile
		json.Unmarshal(env.Data, &p)
		g.playerID = p.PlayerID
		g.pvpRating = p.PvPRating
		g.pvpRank = p.PvPRank
		g.avatar = p.Avatar
		// latest
		g.send("ListMinis", protocol.ListMinis{})
		g.send("ListMaps",  protocol.ListMaps{})
		if len(g.avatars) == 0 {
			g.avatars = g.listAvatars()
		}
		// Build per-champion cache from server
		if g.champToMinis == nil {
			g.champToMinis = map[string]map[string]bool{}
		}
		g.champToMinis = map[string]map[string]bool{} // reset and rebuild
		for ch, minis := range p.Armies {
			set := map[string]bool{}
			for _, m := range minis {
				set[m] = true
			}
			g.champToMinis[ch] = set
		}

		// Active army -> selected champion + selected minis
		g.selectedChampion = ""
		g.selectedMinis = map[string]bool{}
		if len(p.Army) > 0 {
			g.selectedChampion = p.Army[0]
			for _, n := range p.Army[1:] {
				g.selectedMinis[n] = true
			}
			// Ensure champ also exists in local cache
			if _, ok := g.champToMinis[g.selectedChampion]; !ok {
				set := map[string]bool{}
				for k := range g.selectedMinis {
					set[k] = true
				}
				g.champToMinis[g.selectedChampion] = set
			}
		} else {
			// No active army yet: if we have any saved champ armies, pick one to display
			for ch, set := range g.champToMinis {
				g.selectedChampion = ch
				g.selectedMinis = map[string]bool{}
				for m := range set {
					g.selectedMinis[m] = true
				}
				break
			}
		}
	case "RatingUpdate":
		var ru protocol.RatingUpdate
		json.Unmarshal(env.Data, &ru)
		if ru.MatchType == "queue" {
			g.pvpRating = ru.NewRating
			g.pvpRank = ru.Rank
			sign := "+"
			if ru.Delta < 0 {
				sign = ""
			}
			g.pvpStatus = fmt.Sprintf("Rating %s%d => %d (%s vs %s %d)",
				sign, ru.Delta, ru.NewRating, ru.Rank, trim(ru.OppName, 14), ru.OppRating)
		}
	case "Leaderboard":
		var lb protocol.Leaderboard
		json.Unmarshal(env.Data, &lb)
		g.pvpLeaders = lb.Items
		g.lbLastStamp = lb.GeneratedAt
	case "Minis":
		var m protocol.Minis
		json.Unmarshal(env.Data, &m)
		g.minisAll = m.Items
		g.nameToMini = make(map[string]protocol.MiniInfo, len(g.minisAll))
		// split into champions & minis-only (no spells)
		g.champions = g.champions[:0]
		g.minisOnly = g.minisOnly[:0]
		g.nameToMini = make(map[string]protocol.MiniInfo, len(g.minisAll))
		for _, it := range g.minisAll {
			g.nameToMini[it.Name] = it
			if strings.EqualFold(it.Role, "champion") || strings.EqualFold(it.Class, "champion") {
				g.champions = append(g.champions, it)
			} else if strings.EqualFold(it.Role, "mini") && !strings.EqualFold(it.Class, "spell") {
				g.minisOnly = append(g.minisOnly, it)
			}
		}
		g.champScroll, g.miniScroll = 0, 0

	case "Maps":
		var m protocol.Maps
		json.Unmarshal(env.Data, &m)
		g.maps = m.Items
		// Keep the Map tab’s background fixed to the world map.
		if g.currentMap == "" {
			g.currentMap = defaultMapID
		}
		g.ensureMapHotspots()
		g.hoveredHS, g.selectedHS = -1, -1

	case "Init":
		var m protocol.Init
		json.Unmarshal(env.Data, &m)
		g.playerID = m.PlayerID
		g.hand = m.Hand
		g.next = m.Next
		// If server doesn’t send MapID, use what we requested:
		if g.pendingArena != "" {
			g.currentArena = g.pendingArena
			g.pendingArena = ""
		}
		// fresh battle state
		g.selectedIdx = -1
		g.dragActive = false
		g.endActive = false
		g.endVictory = false
		g.gameOver = false
		g.victory = false
		g.world = &World{
			Units: make(map[int64]*RenderUnit),
			Bases: make(map[int64]protocol.BaseState),
		}

		g.scr = screenBattle

	case "GoldUpdate":
		var m protocol.GoldUpdate
		json.Unmarshal(env.Data, &m)
		if m.PlayerID == g.playerID {
			g.gold = m.Gold
		}

	case "StateDelta":
		var d protocol.StateDelta
		json.Unmarshal(env.Data, &d)
		g.world.ApplyDelta(d)

	case "FullSnapshot":
		var s protocol.FullSnapshot
		json.Unmarshal(env.Data, &s)
		g.world = buildWorldFromSnapshot(s)

	case "Error":
		var em protocol.ErrorMsg
		json.Unmarshal(env.Data, &em)
		log.Println("server error:", em.Message)

	case "HandUpdate":
		var hu protocol.HandUpdate
		json.Unmarshal(env.Data, &hu)
		g.hand = hu.Hand
		g.next = hu.Next

	case "GameOver":
		var m protocol.GameOver
		json.Unmarshal(env.Data, &m)
		g.gameOver = true
		g.victory = (m.WinnerID == g.playerID)
	case "FriendlyCode":
		var m protocol.FriendlyCode
		json.Unmarshal(env.Data, &m)
		g.pvpCode = strings.ToUpper(strings.TrimSpace(m.Code))
		g.pvpStatus = "Share this code: " + g.pvpCode

	case "RoomCreated":
		var rc protocol.RoomCreated
		json.Unmarshal(env.Data, &rc)
		g.roomID = rc.RoomID
		// Clear lobby flags so UI looks right
		g.pvpQueued = false
		g.pvpHosting = false
		// (rest of your existing RoomCreated handling if any)
	case "LoggedOut":
		// server acknowledged logout; reset everything and return to login
		g.resetToLogin()
		return
	}
}

func (g *Game) onArmySave(cards []string) {
	g.send("SaveArmy", protocol.SaveArmy{Cards: cards})
	g.send("GetProfile", protocol.GetProfile{})
}

func (g *Game) onMapClicked(arenaID string) {
	g.currentArena = arenaID // arena selection
	g.send("CreatePve", protocol.CreatePve{MapID: arenaID})
}
func (g *Game) onStartBattle() { g.send("StartBattle", protocol.StartBattle{}) }
func (g *Game) onLeaveRoom() {
	g.send("LeaveRoom", protocol.LeaveRoom{})
	g.currentArena = "" // optional reset
}

// ---------- Helpers ----------

func safeName(hand []protocol.MiniCardView, i int) string {
	if i < 0 || i >= len(hand) {
		return ""
	}
	return hand[i].Name
}
func safeCost(hand []protocol.MiniCardView, i int) int {
	if i < 0 || i >= len(hand) {
		return 0
	}
	return hand[i].Cost
}
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (g *Game) Layout(w, h int) (int, int) { return protocol.ScreenW, protocol.ScreenH }

func fitToScreen() {
	mw, mh := ebiten.ScreenSizeInFullscreen() // monitor size
	w, h := protocol.ScreenW, protocol.ScreenH

	margin := 48 // space for titlebar/taskbar so it never clips
	maxW, maxH := mw-margin, mh-margin

	scale := 1.0
	if w > maxW || h > maxH {
		sx := float64(maxW) / float64(w)
		sy := float64(maxH) / float64(h)
		if sx < sy {
			scale = sx
		} else {
			scale = sy
		}
	}

	ww := int(float64(w) * scale)
	wh := int(float64(h) * scale)

	if ww < 800 {
		ww = 800
	}
	if wh < 600 {
		wh = 600
	}

	ebiten.SetWindowSize(ww, wh)
}

func (g *Game) logicalCursor() (int, int) {
	// kept for reference; not used in battle anymore
	winW, winH := ebiten.WindowSize()
	mx, my := ebiten.CursorPosition()
	if winW == 0 || winH == 0 {
		return mx, my
	}
	sx := float64(protocol.ScreenW) / float64(winW)
	sy := float64(protocol.ScreenH) / float64(winH)
	lx := int(float64(mx) * sx)
	ly := int(float64(my) * sy)
	return lx, ly
}

func mathMin(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func desktopMain() {
	fitToScreen()
	ebiten.SetWindowResizable(true)
	ebiten.SetWindowTitle("War Rumble — by s.robev")
	if err := ebiten.RunGame(New("Player")); err != nil {
		log.Fatal(err)
	}
}

func trim(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func (g *Game) selectedMinisList() []string {
	// deterministic order based on minisOnly order
	out := make([]string, 0, 6)
	for _, m := range g.minisOnly {
		if g.selectedMinis[m.Name] {
			out = append(out, m.Name)
			if len(out) == 6 {
				break
			}
		}
	}
	return out
}

func (g *Game) setChampArmyFromSelected() {
	if g.selectedChampion == "" {
		return
	}
	if g.champToMinis[g.selectedChampion] == nil {
		g.champToMinis[g.selectedChampion] = map[string]bool{}
	}
	// replace with current selectedMinis snapshot
	dst := map[string]bool{}
	for k := range g.selectedMinis {
		dst[k] = true
	}
	g.champToMinis[g.selectedChampion] = dst
}

func (g *Game) loadSelectedForChampion(name string) {
	g.selectedChampion = name
	// load saved 6 minis for that champion (if any)
	g.selectedMinis = map[string]bool{}
	if set, ok := g.champToMinis[name]; ok {
		for k := range set {
			g.selectedMinis[k] = true
		}
	}
}

func (g *Game) autoSaveCurrentChampionArmy() {
	// Only save when we have exactly 1 champion + 6 minis (matches server)
	if g.selectedChampion == "" {
		return
	}
	minis := g.selectedMinisList()
	if len(minis) != 6 {
		return
	}
	names := append([]string{g.selectedChampion}, minis...)
	if msg := g.validateArmy(names); msg != "" {
		return
	}
	// server-wide "active" army becomes this champion's army
	g.onArmySave(names)
}

func (g *Game) drawTopBarHome(screen *ebiten.Image) {
	// bar bg + separator
	ebitenutil.DrawRect(screen, 0, 0, float64(protocol.ScreenW), float64(topBarH), color.NRGBA{32, 32, 40, 255})
	ebitenutil.DrawRect(screen, 0, float64(topBarH), float64(protocol.ScreenW), 1, color.NRGBA{90, 90, 120, 255})

	// Make sure rects match what Update sees
	g.computeTopBarLayout()

	// --- User button (9-slice) ---
	mx, my := ebiten.CursorPosition()
	hoveredUser := g.userBtn.hit(mx, my)
	g.drawNineBtn(screen, g.userBtn, hoveredUser)

	// avatar placeholder on the left
	const avW, avPad = 28, 10
	avH := g.userBtn.h - 8
	avX := g.userBtn.x + avPad
	avY := g.userBtn.y + (g.userBtn.h-avH)/2
	if img := g.ensureAvatarImage(g.avatar); img != nil {
		iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
		s := mathMin(float64(avW)/float64(iw), float64(avH)/float64(ih))
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(s, s)
		op.GeoM.Translate(float64(avX), float64(avY))
		screen.DrawImage(img, op)
	} else {
		ebitenutil.DrawRect(screen, float64(avX), float64(avY), float64(avW), float64(avH), color.NRGBA{70, 70, 90, 255})
	}

	// label (no emoji) vertically centered to the button
	name := g.name
	if name == "" { name = "Player" }
	lb := text.BoundString(basicfont.Face7x13, name)
	baselineY := g.userBtn.y + (g.userBtn.h+lb.Dy())/2 - 2
	text.Draw(screen, name, basicfont.Face7x13, g.userBtn.x+avPad+avW+8, baselineY, color.White)
	//baselineY := g.userBtn.y + (g.userBtn.h+13)/2 - 1 // 13 is font height; tweak -2 for optical center
	//text.Draw(screen, label, basicfont.Face7x13, g.userBtn.x+avPad+avW+8, baselineY, color.White)

	// hover tooltip for user (name + avatar)
	if hoveredUser {
		tipW, tipH := 240, 70
		tx := clampInt(mx+14, 0, protocol.ScreenW-tipW)
		ty := clampInt(my+12, 0, protocol.ScreenH-tipH)
		ebitenutil.DrawRect(screen, float64(tx), float64(ty), float64(tipW), float64(tipH), color.NRGBA{30, 30, 45, 240})

		// avatar big placeholder
		ax := tx + 10
		ay := ty + 10
		aw, ah := 56, 56
		if img := g.ensureAvatarImage(g.avatar); img != nil {
			iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
			s := mathMin(float64(aw)/float64(iw), float64(ah)/float64(ih))
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(float64(ax)+(float64(aw)-float64(iw)*s)/2, float64(ay)+(float64(ah)-float64(ih)*s)/2)
			screen.DrawImage(img, op)
		} else {
			ebitenutil.DrawRect(screen, float64(ax), float64(ay), float64(aw), float64(ah), color.NRGBA{70, 70, 90, 255})
		}
		text.Draw(screen, "Account", basicfont.Face7x13, tx+68, ty+22, color.NRGBA{240, 196, 25, 255})
		label := g.name
		if label == "" {
			label = "Player"
		}
		text.Draw(screen, label, basicfont.Face7x13, tx+68, ty+40, color.White)
		text.Draw(screen, "PvP rating", basicfont.Face7x13, tx+68, ty+58, color.NRGBA{240, 196, 25, 255})
		txt := fmt.Sprintf("%d  (%s)", g.pvpRating, defaultIfEmpty(g.pvpRank, "Unranked"))
		text.Draw(screen, txt, basicfont.Face7x13, tx+150, ty+58, color.White)
	}

	// --- Title center ---
	title := "War Rumble"
	tb := text.BoundString(basicfont.Face7x13, title)
	ty := g.titleArea.y + (topBarH+tb.Dy())/2 - 2
	text.Draw(screen, title, basicfont.Face7x13, g.titleArea.x+8, ty, color.White)

	// --- Gold (right) ---
	goldStr := fmt.Sprintf("Gold: %d", g.accountGold)
	gb := text.BoundString(basicfont.Face7x13, goldStr)
	gy := g.goldArea.y + (topBarH+gb.Dy())/2 - 2
	text.Draw(screen, goldStr, basicfont.Face7x13, g.goldArea.x+6, gy, color.NRGBA{240,196,25,255})
	//goldStr := fmt.Sprintf("Gold: %d", g.accountGold)
	//text.Draw(screen, goldStr, basicfont.Face7x13, g.goldArea.x+6, g.goldArea.y+topBarH-14, color.NRGBA{240, 196, 25, 255})

	// hover tooltip for gold
	hoveredGold := g.goldArea.hit(mx, my)
	if hoveredGold {
		tip := []string{
			"Your account gold.",
			"Earn gold from battles, events,",
			"or rewards. Spend it in the shop.",
		}
		tipW, tipH := 260, 66
		tx := clampInt(mx-tipW-10, 0, protocol.ScreenW-tipW)
		ty := clampInt(my+12, 0, protocol.ScreenH-tipH)
		ebitenutil.DrawRect(screen, float64(tx), float64(ty), float64(tipW), float64(tipH), color.NRGBA{30, 30, 45, 240})
		text.Draw(screen, "Gold", basicfont.Face7x13, tx+10, ty+18, color.NRGBA{240, 196, 25, 255})
		text.Draw(screen, tip[0], basicfont.Face7x13, tx+10, ty+34, color.White)
		text.Draw(screen, tip[1], basicfont.Face7x13, tx+10, ty+48, color.NRGBA{210, 210, 220, 255})
		text.Draw(screen, tip[2], basicfont.Face7x13, tx+10, ty+62, color.NRGBA{210, 210, 220, 255})
	}
}

func (g *Game) beginChampDrag(px int) {
	g.champDragActive = true
	g.champDragStartX = px
	g.champDragLastX = px
	g.champDragAccum = 0
}

func (g *Game) moveChampDrag(px int, stepPx int, maxStart int) {
	dx := px - g.champDragLastX
	g.champDragLastX = px
	g.champDragAccum += dx

	// When accumulated movement exceeds one card width step, scroll by columns
	for g.champDragAccum <= -stepPx && g.champStripScroll < maxStart {
		g.champStripScroll++
		g.champDragAccum += stepPx
	}
	for g.champDragAccum >= stepPx && g.champStripScroll > 0 {
		g.champStripScroll--
		g.champDragAccum -= stepPx
	}
}

func (g *Game) endChampDrag() {
	g.champDragActive = false
	g.activeTouchID = -1
	g.champDragAccum = 0
}

const defaultMapID = "rumble_world"

// Which map image to display right now.
func (g *Game) displayMapID() string {
	if g.currentMap != "" {
		return g.currentMap
	}
	if len(g.maps) > 0 {
		return g.maps[0].ID
	}
	return defaultMapID
}

// Compute a *letterboxed* draw rect that preserves aspect ratio (no stretch).
// The image is scaled with s = min(ScreenW/iw, ScreenH/ih).
// Returns the on-screen offset (offX,offY), displayed size (dispW,dispH),
// and the scale factor s.
func (g *Game) mapRenderRect(img *ebiten.Image) (offX, offY, dispW, dispH int, s float64) {
	if img == nil {
		return 0, 0, protocol.ScreenW, protocol.ScreenH, 1
	}
	iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
	if iw == 0 || ih == 0 {
		return 0, 0, protocol.ScreenW, protocol.ScreenH, 1
	}
	sw := float64(protocol.ScreenW) / float64(iw)
	sh := float64(protocol.ScreenH) / float64(ih)
	if sw < sh {
		s = sw
	} else {
		s = sh
	}
	dispW = int(float64(iw) * s)
	dispH = int(float64(ih) * s)
	offX = (protocol.ScreenW - dispW) / 2
	offY = (protocol.ScreenH - dispH) / 2
	return
}

// Letterboxed destination for a bg image (fit to width, keep aspect)
func (g *Game) mapDstRect(imgW, imgH int) rect {
	W, H := protocol.ScreenW, protocol.ScreenH
	scale := float64(W) / float64(imgW)
	dh := int(float64(imgH) * scale)
	y := (H - dh) / 2
	return rect{x: 0, y: y, w: W, h: dh}
}

// Average a few pixels across top & bottom rows to pick a bar color
func (g *Game) mapEdgeColor(mapID string, _ *ebiten.Image) color.NRGBA {
	if c, ok := g.assets.edgeCol[mapID]; ok {
		return c
	}
	if c, ok := g.computeEdgeColorFromFS(mapID); ok {
		g.assets.edgeCol[mapID] = c
		return c
	}
	c := color.NRGBA{20, 20, 24, 255}
	g.assets.edgeCol[mapID] = c
	return c
}

func (g *Game) computeEdgeColorFromFS(mapID string) (color.NRGBA, bool) {
	// Try common names first
	candidates := []string{
		"assets/maps/" + strings.ToLower(mapID) + ".png",
		"assets/maps/" + strings.ToLower(mapID) + ".jpg",
	}

	// If ensureBgForMap fell back to "first file" in the dir, we’ll
	// try to find *any* image as a last resort.
	tryDecode := func(p string) (color.NRGBA, bool) {
		f, err := assetsFS.Open(p)
		if err != nil {
			return color.NRGBA{}, false
		}
		defer f.Close()
		img, _, err := image.Decode(f)
		if err != nil {
			return color.NRGBA{}, false
		}

		b := img.Bounds()
		iw, ih := b.Dx(), b.Dy()
		if iw == 0 || ih == 0 {
			return color.NRGBA{}, false
		}

		samples := 16
		sumR, sumG, sumB := 0, 0, 0
		for i := 0; i < samples; i++ {
			x := b.Min.X + i*(iw-1)/maxInt(samples-1, 1)
			r1, g1, b1, _ := img.At(x, b.Min.Y).RGBA()
			r2, g2, b2, _ := img.At(x, b.Max.Y-1).RGBA()
			sumR += int(r1>>8) + int(r2>>8)
			sumG += int(g1>>8) + int(g2>>8)
			sumB += int(b1>>8) + int(b2>>8)
		}
		n := samples * 2
		c := color.NRGBA{
			uint8(sumR / n),
			uint8(sumG / n),
			uint8(sumB / n),
			255,
		}
		// Slight darken so bars don’t pop
		c.R = uint8(float64(c.R) * 0.9)
		c.G = uint8(float64(c.G) * 0.9)
		c.B = uint8(float64(c.B) * 0.9)
		return c, true
	}

	for _, p := range candidates {
		if c, ok := tryDecode(p); ok {
			return c, true
		}
	}

	// As a last resort, scan the directory once to find an image
	if entries, err := assetsFS.ReadDir("assets/maps"); err == nil {
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := strings.ToLower(e.Name())
			if strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".jpg") {
				if c, ok := tryDecode("assets/maps/" + e.Name()); ok {
					return c, true
				}
			}
		}
	}
	return color.NRGBA{}, false
}

// Letterbox img inside a target rect (x0,y0,w,h)
func (g *Game) mapRenderRectInBounds(x0, y0, w, h int, img *ebiten.Image) (offX, offY, dispW, dispH int, s float64) {
	if img == nil || w <= 0 || h <= 0 {
		return x0, y0, w, h, 1
	}
	iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
	if iw == 0 || ih == 0 {
		return x0, y0, w, h, 1
	}
	sw := float64(w) / float64(iw)
	sh := float64(h) / float64(ih)
	if sw < sh {
		s = sw
	} else {
		s = sh
	}
	dispW = int(float64(iw) * s)
	dispH = int(float64(ih) * s)
	offX = x0 + (w-dispW)/2
	offY = y0 + (h-dispH)/2
	return
}

func (g *Game) createRoomFor(mapID string) {
	g.pendingArena = mapID
	g.send("CreatePve", protocol.CreatePve{MapID: mapID})
}

func (g *Game) drawArenaBG(screen *ebiten.Image) {
	if g.currentArena == "" {
		return
	}
	bg := g.ensureBgForMap(g.currentArena)
	if bg == nil {
		return
	}
	offX, offY, dispW, dispH, s := g.mapRenderRect(bg)

	// Fill letterbox bars with the edge-matched color
	col := g.mapEdgeColor(g.currentArena, bg)

	// Top/Bottom bars
	if offY > 0 {
		ebitenutil.DrawRect(screen, 0, 0, float64(protocol.ScreenW), float64(offY), col)
		ebitenutil.DrawRect(screen, 0, float64(offY+dispH),
			float64(protocol.ScreenW), float64(protocol.ScreenH-(offY+dispH)), col)
	}
	// Left/Right bars (in case the image is narrower than the screen)
	if offX > 0 {
		ebitenutil.DrawRect(screen, 0, float64(offY), float64(offX), float64(dispH), col)
		ebitenutil.DrawRect(screen, float64(offX+dispW), float64(offY),
			float64(protocol.ScreenW-(offX+dispW)), float64(dispH), col)
	}

	// Draw the arena image centered with preserved aspect
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(float64(offX), float64(offY))
	screen.DrawImage(bg, op)
}

func (g *Game) hpfxStep(m map[int64]*hpFx, id int64, curHP int, nowMs int64) *hpFx {
	fx := m[id]
	if fx == nil {
		fx = &hpFx{lastHP: curHP, ghostHP: curHP}
		m[id] = fx
		return fx
	}

	// Took damage → set yellow chip to old HP, hold for 1s
	if curHP < fx.lastHP {
		fx.ghostHP = fx.lastHP
		fx.holdUntilMs = nowMs + 500 // 0.5s hold
		fx.lerpStartMs = 0           // reset animation
		fx.lerpDurMs = 300           // 0.3s collapse after hold
		fx.lerpStartHP = fx.ghostHP
	}

	// After hold, collapse yellow chip down to current HP
	if fx.ghostHP > curHP && nowMs > fx.holdUntilMs {
		if fx.lerpStartMs == 0 {
			fx.lerpStartMs = nowMs
			fx.lerpStartHP = fx.ghostHP
		}
		t := float64(nowMs-fx.lerpStartMs) / float64(fx.lerpDurMs)
		if t >= 1 {
			fx.ghostHP = curHP
			fx.lerpStartMs = 0
		} else {
			// linear interpolate ghost towards cur
			gh := float64(fx.lerpStartHP) + (float64(curHP)-float64(fx.lerpStartHP))*t
			fx.ghostHP = int(math.Round(gh))
		}
	}

	fx.lastHP = curHP
	return fx
}

func (g *Game) drawHPBar(screen *ebiten.Image, x, y, w, h float64, cur, max, ghost int) {
	if max <= 0 || w <= 0 || h <= 0 {
		return
	}
	pCur := float64(cur) / float64(max)
	pGhost := float64(ghost) / float64(max)
	if pCur < 0 {
		pCur = 0
	} else if pCur > 1 {
		pCur = 1
	}
	if pGhost < 0 {
		pGhost = 0
	} else if pGhost > 1 {
		pGhost = 1
	}

	// Red background (missing health)
	ebitenutil.DrawRect(screen, x, y, w, h, color.NRGBA{160, 40, 40, 255})

	// Yellow recent-damage segment
	if pGhost > pCur {
		startX := x + w*pCur
		ebitenutil.DrawRect(screen, startX, y, w*(pGhost-pCur), h, color.NRGBA{240, 196, 25, 255})
	}

	// Green current HP
	ebitenutil.DrawRect(screen, x, y, w*pCur, h, color.NRGBA{40, 200, 60, 255})
}

type NineSlice struct{ Left, Right, Top, Bottom int }

// draw a 9-slice image into (x,y,w,h). Falls back to uniform scale if too small.
func drawNineSlice(dst *ebiten.Image, src *ebiten.Image, x, y, w, h int, cap NineSlice) {
	if src == nil || w <= 0 || h <= 0 {
		return
	}
	b := src.Bounds()
	iw, ih := b.Dx(), b.Dy()
	l, r, t, btm := cap.Left, cap.Right, cap.Top, cap.Bottom

	// If destination is too small to slice, uniformly scale the whole image.
	if w < l+r || h < t+btm {
		sx := float64(w) / float64(iw)
		sy := float64(h) / float64(ih)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(sx, sy)
		op.GeoM.Translate(float64(x), float64(y))
		dst.DrawImage(src, op)
		return
	}

	// Source rects
	sx := func(x0, y0, x1, y1 int) *ebiten.Image {
		return src.SubImage(image.Rect(b.Min.X+x0, b.Min.Y+y0, b.Min.X+x1, b.Min.Y+y1)).(*ebiten.Image)
	}

	// Dest positions/sizes
	cw := w - l - r
	ch := h - t - btm

	// 4 corners (no scale)
	type piece struct {
		s      *ebiten.Image
		dx, dy int
		sw, sh int
	}
	parts := []piece{
		{sx(0, 0, l, t), x, y, l, t},                               // TL
		{sx(iw-r, 0, iw, t), x + w - r, y, r, t},                   // TR
		{sx(0, ih-btm, l, ih), x, y + h - btm, l, btm},             // BL
		{sx(iw-r, ih-btm, iw, ih), x + w - r, y + h - btm, r, btm}, // BR
	}
	for _, p := range parts {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(p.dx), float64(p.dy))
		dst.DrawImage(p.s, op)
	}

	// 4 edges (scale in one axis)
	// Top
	if t > 0 && cw > 0 {
		sTop := sx(l, 0, iw-r, t)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(cw)/float64(iw-l-r), 1)
		op.GeoM.Translate(float64(x+l), float64(y))
		dst.DrawImage(sTop, op)
	}
	// Bottom
	if btm > 0 && cw > 0 {
		sBot := sx(l, ih-btm, iw-r, ih)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(cw)/float64(iw-l-r), 1)
		op.GeoM.Translate(float64(x+l), float64(y+h-btm))
		dst.DrawImage(sBot, op)
	}
	// Left
	if l > 0 && ch > 0 {
		sLeft := sx(0, t, l, ih-btm)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(1, float64(ch)/float64(ih-t-btm))
		op.GeoM.Translate(float64(x), float64(y+t))
		dst.DrawImage(sLeft, op)
	}
	// Right
	if r > 0 && ch > 0 {
		sRight := sx(iw-r, t, iw, ih-btm)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(1, float64(ch)/float64(ih-t-btm))
		op.GeoM.Translate(float64(x+w-r), float64(y+t))
		dst.DrawImage(sRight, op)
	}

	// Center (scale both)
	if cw > 0 && ch > 0 {
		sMid := sx(l, t, iw-r, ih-btm)
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(float64(cw)/float64(iw-l-r), float64(ch)/float64(ih-t-btm))
		op.GeoM.Translate(float64(x+l), float64(y+t))
		dst.DrawImage(sMid, op)
	}
}

func (g *Game) drawNineBtn(screen *ebiten.Image, r rect, hovered bool) {
	g.assets.ensureInit()
	img := g.assets.btn9Base
	if hovered && g.assets.btn9Hover != nil {
		img = g.assets.btn9Hover
	}
	if img != nil {
		drawNineSlice(screen, img, r.x, r.y, r.w, r.h, NineSlice{Left: 6, Right: 6, Top: 6, Bottom: 6})
		return
	}
	// Fallback color that stands out from the top bar
	col := color.NRGBA{54, 63, 88, 255}
	if hovered {
		col = color.NRGBA{74, 86, 120, 255}
	}
	ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), col)
}

func (g *Game) computeTopBarLayout() {
		// Button paddings and avatar size
		const padX = 10
		const avW  = 28

		// Vertical margins inside the bar
		const userBtnVMargin = 4

		userBtnH := topBarH - 2*userBtnVMargin // renamed (don't collide with global btnH const)

		// measure label with the real face
		uname := g.name
		if uname == "" { uname = "Player" }
			nameBounds := text.BoundString(basicfont.Face7x13, uname)
			nameW := nameBounds.Dx()

			btnW := padX*2 + avW + 8 + nameW
		if btnW < 132 { btnW = 132 } // slightly wider min so it breathes

		g.userBtn = rect{
			x: pad,
			y: userBtnVMargin,
			w: btnW,
			h: userBtnH,
		}

	// Right: gold area
	goldStr := fmt.Sprintf("Gold: %d", g.accountGold)
	gb := text.BoundString(basicfont.Face7x13, goldStr)
	g.goldArea = rect{
		x: protocol.ScreenW - pad - gb.Dx() - 12,
		y: 0,
		w: gb.Dx() + 12,
		h: topBarH,
	}

	// Center title
	title := "War Rumble"
	tb := text.BoundString(basicfont.Face7x13, title)
	tx := (protocol.ScreenW - tb.Dx()) / 2
	g.titleArea = rect{x: tx - 8, y: 0, w: tb.Dx() + 16, h: topBarH}
}

func (g *Game) computeBottomBarLayout() {
	type item struct {
		label string
		out   *rect
	}
	items := []item{
		{"Army", &g.armyBtn},
		{"Map", &g.mapBtn},
		{"PvP", &g.pvpBtn},
		{"Social", &g.socialBtn},
		{"Settings", &g.settingsBtn},
	}

	// Base geometry
	availW := protocol.ScreenW - 2*pad
	spacing := pad
	btnH := btnH // use your existing const
	y0 := protocol.ScreenH - menuBarH + (menuBarH-btnH)/2

	// Text metrics + natural widths
	basePadX := 16
	minPadX := 10
	minW := 64

	widths := make([]int, len(items))
	for i, it := range items {
		tw := text.BoundString(basicfont.Face7x13, it.label).Dx()
		w := tw + basePadX*2
		if w < minW {
			w = minW
		}
		widths[i] = w
	}

	// Cap by per-button budget so the row fits
	maxPer := (availW - spacing*(len(items)-1)) / len(items)
	if maxPer < minW {
		maxPer = minW
	}
	for i := range widths {
		if widths[i] > maxPer {
			widths[i] = maxPer
		}
	}

	// If still overflowing, reduce spacing then shrink to a tighter padding
	total := 0
	for _, w := range widths {
		total += w
	}
	need := total + spacing*(len(items)-1)
	if need > availW {
		// 1) squeeze spacing (down to 4px)
		spacing2 := spacing - (need-availW)/(len(items)-1) - 1
		if spacing2 < 4 {
			spacing2 = 4
		}
		spacing = spacing2
		need = total + spacing*(len(items)-1)
		// 2) if still too wide, shrink to text + minPadX
		if need > availW {
			for i := range widths {
				tw := text.BoundString(basicfont.Face7x13, items[i].label).Dx()
				minWi := tw + minPadX*2
				if minWi > widths[i] {
					minWi = widths[i]
				} // don’t grow
				widths[i] = minWi
			}
			// recompute totals (now it *will* fit on typical 800px+)
		}
	}

	// Assign rects left→right
	x := pad
	for i, it := range items {
		*it.out = rect{x: x, y: y0, w: widths[i], h: btnH}
		x += widths[i] + spacing
	}
}

func (g *Game) pvpLayout() (queueBtn, leaveBtn, createBtn, cancelBtn, joinInput, joinBtn rect) {
	const fieldW, fieldH = 180, 24
	const btnH = 28

	x := pad
	y := topBarH + pad + 22

	// Row 1: queue/leave
	queueBtn = rect{x: x, y: y, w: 150, h: btnH}
	leaveBtn = rect{x: x + 160, y: y, w: 150, h: btnH}

	// Row 2: create/cancel
	createBtn = rect{x: x, y: y + 44, w: 220, h: btnH}
	cancelBtn = rect{x: x + 230, y: y + 44, w: 150, h: btnH}

	// --- Dynamic vertical offset if the "Your code" pill is visible ---
	// Pill geometry in Draw: by = createBtn.y + btnH + 12, height = 26.
	// Keep at least 16px gap below it.
	joinY := y + 100
	if g.pvpHosting && g.pvpCode != "" {
		codeBottom := (createBtn.y + btnH + 12) + 26
		want := codeBottom + 16
		if want > joinY {
			joinY = want
		}
	}

	// Row 3: join-with-code
	joinInput = rect{x: x, y: joinY, w: fieldW, h: fieldH}
	joinBtn = rect{x: x + fieldW + 10, y: joinY - 4, w: 120, h: btnH}
	return
}
func defaultIfEmpty(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (g *Game) listAvatars() []string {
	buildEmbIndex() // you already have this
	entries, err := assetsFS.ReadDir("assets/ui/avatars")
	if err != nil {
		return []string{"default.png"} // fallback
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		low := strings.ToLower(name)
		if strings.HasSuffix(low, ".png") || strings.HasSuffix(low, ".jpg") || strings.HasSuffix(low, ".jpeg") {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	if len(out) == 0 {
		out = []string{"default.png"}
	}
	return out
}

func (g *Game) ensureAvatarImage(file string) *ebiten.Image {
	g.assets.ensureInit()
	if file == "" {
		file = "default.png"
	}
	key := "avatars/" + strings.ToLower(file)
	if img, ok := g.assets.minis[key]; ok {
		return img
	}
	img := loadImage("assets/ui/" + key)
	g.assets.minis[key] = img
	return img
}

func (g *Game) drawProfileOverlay(screen *ebiten.Image) {
    // --- Dim background (semi-transparent, blends over UI) ---
    ebitenutil.DrawRect(
        screen, 0, 0,
        float64(protocol.ScreenW), float64(protocol.ScreenH),
        color.NRGBA{10, 10, 18, 140},
    )

    // ---- Panel sizing (dynamic height based on avatar grid) ----
    if len(g.avatars) == 0 {
        g.avatars = g.listAvatars()
    }
    const cols = 6
    const cell = 60
    const gap  = 10

    rows := (len(g.avatars) + cols - 1) / cols
    gridH := 0
    if rows > 0 {
        gridH = rows*cell + (rows-1)*gap
    }

    const headerH  = 64  // title + subtitle + close
    const statsH   = 72  // pvp stats block height
    const padOut   = 16

    w := 520
    // base height: header + grid + stats + vertical padding
    h := headerH + gridH + statsH + padOut*2 + 8 // +8 extra breathing room

    // Keep inside the screen
    if h > protocol.ScreenH - 80 {
        h = protocol.ScreenH - 80
    }
    x := (protocol.ScreenW - w) / 2
    y := (protocol.ScreenH - h) / 2

    // ---- Panel background ----
    ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), color.NRGBA{32, 32, 44, 230})

    // ---- Title: Player name ----
    title := g.name
    if title == "" { title = "Player" }
    ty := y + 22
    text.Draw(screen, title, basicfont.Face7x13, x+padOut, ty, color.White)

    // Subtitle
    text.Draw(screen, "Choose your avatar", basicfont.Face7x13, x+padOut, ty+20, color.NRGBA{210,210,220,255})

    // ---- Close button ----
    g.profCloseBtn = rect{x: x + w - 80, y: y + 14, w: 64, h: 24}
    ebitenutil.DrawRect(screen, float64(g.profCloseBtn.x), float64(g.profCloseBtn.y),
        float64(g.profCloseBtn.w), float64(g.profCloseBtn.h), color.NRGBA{70, 70, 90, 255})
    text.Draw(screen, "X", basicfont.Face7x13, g.profCloseBtn.x+16, g.profCloseBtn.y+16, color.White)

    // ---- Avatar grid ----
    gridX := x + padOut
    gridY := y + headerH
    g.avatarRects = g.avatarRects[:0]

    for i, name := range g.avatars {
        c := i % cols
        r := i / cols
        cx := gridX + c*(cell+gap)
        cy := gridY + r*(cell+gap)
        rct := rect{x: cx, y: cy, w: cell, h: cell}
        g.avatarRects = append(g.avatarRects, rct)

        // card
        ebitenutil.DrawRect(screen, float64(cx), float64(cy), float64(cell), float64(cell),
            color.NRGBA{43,43,62,255})

        // image centered
        if img := g.ensureAvatarImage(name); img != nil {
            iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
            s := mathMin(float64(cell-8)/float64(iw), float64(cell-8)/float64(ih))
            op := &ebiten.DrawImageOptions{}
            op.GeoM.Scale(s, s)
            op.GeoM.Translate(
                float64(cx) + (float64(cell)-float64(iw)*s)/2,
                float64(cy) + (float64(cell)-float64(ih)*s)/2,
            )
            screen.DrawImage(img, op)
        } else {
            text.Draw(screen, "?", basicfont.Face7x13, cx+cell/2-4, cy+cell/2+4, color.White)
        }

        // selected highlight
        if strings.EqualFold(name, g.avatar) {
            ebitenutil.DrawRect(screen, float64(cx), float64(cy), float64(cell), 2,
                color.NRGBA{240,196,25,255})
        }
    }

    // ---- PvP Stats block (below grid) ----
    statsTop := gridY + gridH + 16
    if rows == 0 {
        statsTop = gridY + 8
    }

    // background bar
    ebitenutil.DrawRect(screen,
        float64(x+padOut-2), float64(statsTop-10),
        float64(w-2*padOut+4), float64(statsH), color.NRGBA{36, 36, 52, 255})

    // labels
    y0 := statsTop + 10
    text.Draw(screen, "PvP Stats", basicfont.Face7x13, x+padOut, y0, color.White)

    // Rating + Rank
    lineY := y0 + 18
    text.Draw(screen, fmt.Sprintf("Rating: %d", g.pvpRating),
        basicfont.Face7x13, x+padOut, lineY, color.NRGBA{220,220,230,255})
    text.Draw(screen, fmt.Sprintf("Rank:   %s", g.pvpRank),
        basicfont.Face7x13, x+padOut+160, lineY, color.NRGBA{240,196,25,255})

    // (Optional) You can add more lines here later: games played, winrate, best rank, season ends, etc.

	// ---- Logout button ----
	btnW, btnH := 96, 28
	bx := x + w - btnW - padOut
	by := y + h - btnH - padOut

	g.profLogoutBtn = rect{x: bx, y: by, w: btnW, h: btnH}

	// background
	ebitenutil.DrawRect(screen,
		float64(bx), float64(by),
		float64(btnW), float64(btnH),
		color.NRGBA{120, 40, 40, 255},
	)

	// label
	text.Draw(screen, "Logout",
	basicfont.Face7x13, bx+20, by+18, color.White)
}


func (g *Game) connectAsync() {
	// Run in a goroutine: never block the render/update thread.
	n, err := NewNet(netcfg.ServerURL)
	if err != nil {
		g.connCh <- connResult{nil, err}
		return
	}
	g.connCh <- connResult{n, nil}
}

func (g *Game) retryConnect() {
	if g.connSt == stateConnecting {
		return
	}
	g.connSt = stateConnecting
	g.connErrMsg = ""
	go g.connectAsync()
}

// ensureNet: make sure we have a live socket
func (g *Game) ensureNet() bool {
	if g.net != nil && !g.net.IsClosed() {
		return true
	}
	n, err := NewNet(netcfg.ServerURL)
	if err != nil {
		log.Printf("NET: dial failed: %v", err)
		return false
	}
	g.net = n
	log.Println("NET: connected")
	return true
}

// safe send wrapper
func (g *Game) send(typ string, payload interface{}) {
	if !g.ensureNet() {
		return
	}
	if err := g.net.Send(typ, payload); err != nil {
		log.Printf("NET: send(%s) failed: %v", typ, err)
	}
}


func (g *Game) resetToLogin() {
	// Fully reset client-side state and force a new WS on next login
	if g.net != nil {
		g.net.Close()
		g.net = nil
	}

	// Clear session-ish state so we render/login cleanly
	g.scr = screenLogin
	g.nameInput = ""
	g.name = ""
	g.playerID = 0

	// Clear lobby data so Army tab won’t try to render stale slices
	g.minisAll = nil
	g.minisOnly = nil
	g.champions = nil
	g.nameToMini = nil
	g.maps = nil
	g.champToMinis = map[string]map[string]bool{}
	g.selectedChampion = ""
	g.selectedMinis = map[string]bool{}

	// Clear PvP UI
	g.pvpQueued = false
	g.pvpHosting = false
	g.pvpCode = ""
	g.pvpCodeInput = ""
	g.pvpStatus = "Logged out."

	// Clear room/battle
	g.roomID = ""
	g.currentArena = ""
	g.pendingArena = ""
	g.world = &World{Units: map[int64]*RenderUnit{}, Bases: map[int64]protocol.BaseState{}}
	g.endActive, g.endVictory, g.gameOver, g.victory = false, false, false, false
}

