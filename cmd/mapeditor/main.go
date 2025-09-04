package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image/color"
	"io"
	"log"
	"math"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"rumble/shared/protocol"

	"github.com/gorilla/websocket"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

type wsMsg struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type editor struct {
	// connection
	ws   *websocket.Conn
	inCh chan wsMsg

	// state
	bgPath   string
	bg       *ebiten.Image
	def      protocol.MapDef
	tmpLane  []protocol.PointF
	tool     int // 0 deploy, 1 stone, 2 mine, 3 lane, 4 obstacle
	name     string
	status   string
	savePath string

	// selection & editing
	selKind   string // ""|"deploy"|"stone"|"mine"|"lane"|"obstacle"
	selIndex  int
	selHandle int // for deploy corners: 0=TL,1=TR,2=BR,3=BL, -1=body; for obstacles: 0=TL,1=TR,2=BR,3=BL, -1=body
	dragging  bool
	lastMx    int
	lastMy    int

	// bg management
	showGrid bool
	bgInput  string
	bgFocus  bool

	// assets browser
	showAssetsBrowser   bool
	availableAssets     []string
	assetsBrowserSel    int
	assetsBrowserScroll int
	assetsCurrentPath   string

	// obstacles browser
	showObstaclesBrowser   bool
	availableObstacles     []string
	obstaclesBrowserSel    int
	obstaclesBrowserScroll int
	obstaclesCurrentPath   string

	// removed name input focus - no longer needed

	// login UI
	showLogin   bool
	loginUser   string
	loginPass   string
	loginFocus  int // 0=username, 1=password
	loginStatus string

	// help system
	showHelp     bool
	helpText     string
	tooltipTimer int
	tooltipX     int
	tooltipY     int

	// map browser
	showMapBrowser   bool
	availableMaps    []string
	mapBrowserSel    int
	mapBrowserScroll int

	// enhanced help system
	showWelcome  bool
	helpMode     bool
	keyboardHelp bool

	// undo/redo system
	undoStack    []protocol.MapDef
	redoStack    []protocol.MapDef
	maxUndoSteps int

	// better status and notifications
	lastAction  string
	actionTimer int

	// chat window for status messages
	statusMessages    []string
	maxStatusMessages int
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// ConfigDir: mimic client logic so we can reuse token.json
func sanitize(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, " ", "_")
	re := regexp.MustCompile(`[^a-z0-9._-]`)
	return re.ReplaceAllString(s, "")
}
func profileID() string {
	if p := strings.TrimSpace(os.Getenv("WAR_PROFILE")); p != "" {
		return sanitize(p)
	}
	exe, _ := os.Executable()
	base := strings.TrimSuffix(filepath.Base(exe), filepath.Ext(exe))
	h := sha1.Sum([]byte(exe))
	return sanitize(base) + "-" + hex.EncodeToString(h[:])[:8]
}
func configDir() string {
	root, _ := os.UserConfigDir()
	if root == "" {
		if home, _ := os.UserHomeDir(); home != "" {
			root = filepath.Join(home, ".config")
		}
	}
	d := filepath.Join(root, "WarRumble", profileID())
	_ = os.MkdirAll(d, 0o755)
	return d
}
func loadToken() string {
	b, _ := os.ReadFile(filepath.Join(configDir(), "token.json"))
	return strings.TrimSpace(string(b))
}

func dialWS(url, token string) (*websocket.Conn, error) {
	d := websocket.Dialer{HandshakeTimeout: 5 * time.Second,
		Proxy: func(*http.Request) (*neturl.URL, error) { return nil, nil }}
	// add token as header and query param (server accepts either)
	if token != "" {
		if u, err := neturl.Parse(url); err == nil {
			q := u.Query()
			q.Set("token", token)
			u.RawQuery = q.Encode()
			url = u.String()
		}
	}
	hdr := http.Header{}
	if token != "" {
		hdr.Set("Authorization", "Bearer "+token)
	}
	c, _, err := d.Dial(url, hdr)
	return c, err
}

func (e *editor) runReader() {
	for {
		_, data, err := e.ws.ReadMessage()
		if err != nil {
			close(e.inCh)
			return
		}
		var m wsMsg
		if json.Unmarshal(data, &m) == nil {
			e.inCh <- m
		}
	}
}

func (e *editor) Update() error {
	// Handle login screen
	if e.showLogin {
		mx, my := ebiten.CursorPosition()
		vw, vh := ebiten.WindowSize()

		// Handle mouse clicks for login UI
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Username field
			ux := (vw - 200) / 2
			uy := vh/2 - 40
			if mx >= ux && mx < ux+200 && my >= uy && my < uy+24 {
				e.loginFocus = 0
			}
			// Password field
			px := (vw - 200) / 2
			py := vh / 2
			if mx >= px && mx < px+200 && my >= py && my < py+24 {
				e.loginFocus = 1
			}
			// Login button
			bx := (vw - 100) / 2
			by := vh/2 + 60
			if mx >= bx && mx < bx+100 && my >= by && my < by+30 {
				e.attemptLogin()
			}
		}

		// Handle keyboard input
		for _, k := range inpututil.AppendJustPressedKeys(nil) {
			if k == ebiten.KeyTab {
				e.loginFocus = (e.loginFocus + 1) % 2
			}
			if k == ebiten.KeyEnter {
				e.attemptLogin()
			}
			if k == ebiten.KeyBackspace {
				if e.loginFocus == 0 && len(e.loginUser) > 0 {
					e.loginUser = e.loginUser[:len(e.loginUser)-1]
				} else if e.loginFocus == 1 && len(e.loginPass) > 0 {
					e.loginPass = e.loginPass[:len(e.loginPass)-1]
				}
			}
		}
		for _, r := range ebiten.AppendInputChars(nil) {
			if r >= 32 {
				if e.loginFocus == 0 {
					e.loginUser += string(r)
				} else if e.loginFocus == 1 {
					e.loginPass += string(r)
				}
			}
		}
		return nil
	}

	// pump messages
	for {
		select {
		case m := <-e.inCh:
			switch m.Type {
			case "MapDef":
				var md protocol.MapDefMsg
				_ = json.Unmarshal(m.Data, &md)
				e.def = md.Def
				if strings.TrimSpace(e.name) == "" {
					e.name = md.Def.Name
				}
				// Automatically load the map's background if specified
				if md.Def.Bg != "" {
					// Try multiple paths for background images
					paths := []string{
						md.Def.Bg, // Original path
						filepath.Join("..", "..", "client", "internal", "game", "assets", "maps", md.Def.Bg), // From mapeditor to client assets
						filepath.Join("maps", md.Def.Bg), // Local maps directory
					}

					var loaded bool
					for _, path := range paths {
						if img, _, err := ebitenutil.NewImageFromFile(path); err == nil {
							e.bg = img
							e.bgPath = path
							e.status = fmt.Sprintf("Loaded map and BG: %s", md.Def.Name)
							loaded = true
							break
						}
					}

					if !loaded {
						e.status = fmt.Sprintf("Loaded map: %s (BG not found)", md.Def.Name)
					}
				} else {
					e.status = fmt.Sprintf("Loaded map: %s", md.Def.Name)
				}
			case "Maps":
				// ignore
			case "Error":
				var em protocol.ErrorMsg
				_ = json.Unmarshal(m.Data, &em)
				e.status = em.Message
			}
		default:
			goto done
		}
	}
done:
	mx, my := ebiten.CursorPosition()

	// Deselect tool when overlays are open to prevent accidental object placement
	if e.showMapBrowser || e.showAssetsBrowser || e.showObstaclesBrowser {
		e.tool = -1 // Deselect any tool
	}

	// Enhanced toolbar click handling (updated for new button positions)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Tool buttons
		for i := 0; i < 5; i++ {
			x, y, w, h := 8, 8+i*40, 100, 36
			if mx >= x && mx < x+w && my >= y && my < y+h {
				e.tool = i
				e.status = fmt.Sprintf("Selected: %s", []string{"Deploy Zones", "Meeting Stones", "Gold Mines", "Movement Lanes", "Obstacles"}[i])
			}
		}

		// Quick action buttons
		for i := 0; i < 4; i++ {
			x, y, w, h := 120, 8+i*40, 80, 36
			if mx >= x && mx < x+w && my >= y && my < y+h {
				switch i {
				case 0: // Grid
					e.showGrid = !e.showGrid
					if e.showGrid {
						e.status = "Grid overlay enabled"
					} else {
						e.status = "Grid overlay disabled"
					}
				case 1: // Snap
					e.showGrid = !e.showGrid // Using grid for snap functionality
					if e.showGrid {
						e.status = "Snap-to-grid enabled"
					} else {
						e.status = "Snap-to-grid disabled"
					}
				case 2: // Undo
					e.status = "Undo functionality coming soon"
				case 3: // Redo
					e.status = "Redo functionality coming soon"
				}
			}
		}
	}

	// Calculate button positions for tooltips
	nameLblX := 140
	nameX := nameLblX + 48
	nameY := 8
	saveX := nameX + 240
	saveY := 8
	clrX := saveX + 110
	clrY := saveY
	loadX := clrX + 90
	loadY := clrY
	helpX := loadX + 100
	helpY := loadY

	// BG buttons (second row)
	bx := 140
	by := 48
	lx, ly, lw, lh := bx+270, by, 64, 24
	sx, sy, sw, sh := bx+270+70, by, 64, 24
	cx, cy, cw, ch := bx+270+140, by, 64, 24
	ax, ay, aw, ah := bx+270+210, by, 64, 24
	ox, oy, ow, oh := bx+270+280, by, 64, 24
	// compute display rect consistent with Draw - match battle system scaling
	const topUIH = 120
	var offX, offY, dispW, dispH int
	if e.bg == nil {
		w, h := ebiten.WindowSize()
		offX, offY, dispW, dispH = 0, topUIH, w, h-topUIH
	} else {
		sw, sh := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
		vw, vh := ebiten.WindowSize()
		vh -= topUIH
		sx := float64(vw) / float64(sw)
		sy := float64(vh) / float64(sh)
		s := sx
		if sy < sx {
			s = sy
		}
		dispW = int(float64(sw) * s)
		dispH = int(float64(sh) * s)
		offX = (vw - dispW) / 2
		offY = topUIH + (vh-dispH)/2
	}
	inCanvas := func(x, y int) bool { return x >= offX && x < offX+dispW && y >= offY && y < offY+dispH }
	toNorm := func(px, py int) (float64, float64) {
		return float64(px-offX) / float64(dispW), float64(py-offY) / float64(dispH)
	}
	toPix := func(nx, ny float64) (int, int) { return offX + int(nx*float64(dispW)), offY + int(ny*float64(dispH)) }

	// Match battle system: use screen dimensions for consistent scaling
	screenW, screenH := ebiten.WindowSize()
	_ = screenW // Mark as used to avoid compiler warning
	_ = screenH // Mark as used to avoid compiler warning

	// hit tests
	hitDeploy := func(mx, my int) (idx int, handle int, ok bool) {
		for i, r := range e.def.DeployZones {
			x, y := toPix(r.X, r.Y)
			w := int(r.W * float64(dispW))
			h := int(r.H * float64(dispH))
			// corners
			corners := [][4]int{{x - 4, y - 4, 8, 8}, {x + w - 4, y - 4, 8, 8}, {x + w - 4, y + h - 4, 8, 8}, {x - 4, y + h - 4, 8, 8}}
			for ci, c := range corners {
				if mx >= c[0] && mx < c[0]+c[2] && my >= c[1] && my < c[1]+c[3] {
					return i, ci, true
				}
			}
			if mx >= x && mx < x+w && my >= y && my < y+h {
				return i, -1, true
			}
		}
		return -1, 0, false
	}
	hitPointList := func(pts []protocol.PointF, mx, my int, radius int) (int, bool) {
		r2 := radius * radius
		for i, p := range pts {
			px, py := toPix(p.X, p.Y)
			dx, dy := mx-px, my-py
			if dx*dx+dy*dy <= r2 {
				return i, true
			}
		}
		return -1, false
	}
	distToSegSq := func(x, y, x1, y1, x2, y2 int) float64 {
		dx := float64(x2 - x1)
		dy := float64(y2 - y1)
		if dx == 0 && dy == 0 {
			ex, ey := float64(x-x1), float64(y-y1)
			return ex*ex + ey*ey
		}
		t := (float64(x-x1)*dx + float64(y-y1)*dy) / (dx*dx + dy*dy)
		if t < 0 {
			t = 0
		} else if t > 1 {
			t = 1
		}
		px := float64(x1) + t*dx
		py := float64(y1) + t*dy
		ex, ey := float64(x)-px, float64(y)-py
		return ex*ex + ey*ey
	}
	hitLane := func(mx, my int) (int, bool) {
		th2 := 6.0 * 6.0
		for i, ln := range e.def.Lanes {
			for j := 1; j < len(ln.Points); j++ {
				x1, y1 := toPix(ln.Points[j-1].X, ln.Points[j-1].Y)
				x2, y2 := toPix(ln.Points[j].X, ln.Points[j].Y)
				if distToSegSq(mx, my, x1, y1, x2, y2) <= th2 {
					return i, true
				}
			}
		}
		return -1, false
	}
	hitObstacle := func(mx, my int) (idx int, handle int, ok bool) {
		for i, obs := range e.def.Obstacles {
			x, y := toPix(obs.X, obs.Y)
			w := int(obs.Width * float64(dispW))
			h := int(obs.Height * float64(dispH))
			// corners for resizing
			corners := [][4]int{{x - 4, y - 4, 8, 8}, {x + w - 4, y - 4, 8, 8}, {x + w - 4, y + h - 4, 8, 8}, {x - 4, y + h - 4, 8, 8}}
			for ci, c := range corners {
				if mx >= c[0] && mx < c[0]+c[2] && my >= c[1] && my < c[1]+c[3] {
					return i, ci, true
				}
			}
			if mx >= x && mx < x+w && my >= y && my < y+h {
				return i, -1, true
			}
		}
		return -1, 0, false
	}
	hitPlayerBase := func(mx, my int) bool {
		if e.def.PlayerBase.X == 0 && e.def.PlayerBase.Y == 0 {
			return false
		}
		x, y := toPix(e.def.PlayerBase.X, e.def.PlayerBase.Y)
		baseW := 96
		baseH := 96
		return mx >= x && mx < x+baseW && my >= y && my < y+baseH
	}
	hitEnemyBase := func(mx, my int) bool {
		if e.def.EnemyBase.X == 0 && e.def.EnemyBase.Y == 0 {
			return false
		}
		x, y := toPix(e.def.EnemyBase.X, e.def.EnemyBase.Y)
		baseW := 96
		baseH := 96
		return mx >= x && mx < x+baseW && my >= y && my < y+baseH
	}

	// Help system - check for hover tooltips
	e.tooltipTimer++
	if e.tooltipTimer > 30 { // Show tooltip after 0.5 seconds of hovering
		e.showHelp = false

		// Check enhanced toolbar buttons
		for i := 0; i < 5; i++ {
			x, y, w, h := 8, 8+i*40, 100, 36
			if mx >= x && mx < x+w && my >= y && my < y+h {
				tooltips := []string{
					"📦 Deploy Zones: Click to create deployment areas for units",
					"🏛️ Meeting Stones: Click to place rally points for units",
					"💰 Gold Mines: Click to place gold resource points",
					"🛤️ Movement Lanes: Click to start drawing unit movement paths",
					"🌳 Obstacles: Click to place blocking objects on the map",
				}
				e.helpText = tooltips[i]
				e.tooltipX = x + w + 10
				e.tooltipY = y
				e.showHelp = true
				break
			}
		}

		// Check quick action buttons
		if !e.showHelp {
			for i := 0; i < 4; i++ {
				x, y, w, h := 120, 8+i*40, 80, 36
				if mx >= x && mx < x+w && my >= y && my < y+h {
					quickTooltips := []string{
						"Grid: Toggle grid overlay for precise positioning",
						"Snap: Enable snap-to-grid for element placement",
						"Undo: Undo the last action (coming soon)",
						"Redo: Redo the last undone action (coming soon)",
					}
					e.helpText = quickTooltips[i]
					e.tooltipX = x + w + 10
					e.tooltipY = y
					e.showHelp = true
					break
				}
			}
		}

		// Check other UI elements if no toolbar tooltip
		if !e.showHelp {
			// Save button
			if mx >= saveX && mx < saveX+100 && my >= saveY && my < saveY+24 {
				e.helpText = "Save: Ctrl+S - Save current map to server"
				e.tooltipX = saveX
				e.tooltipY = saveY - 25
				e.showHelp = true
			} else
			// Clear button
			if mx >= clrX && mx < clrX+80 && my >= clrY && my < clrY+24 {
				e.helpText = "Clear Selection: Deselect current element"
				e.tooltipX = clrX
				e.tooltipY = clrY - 25
				e.showHelp = true
			} else
			// Load Map button
			if mx >= loadX && mx < loadX+90 && my >= loadY && my < loadY+24 {
				e.helpText = "Load Map: Browse and load existing maps"
				e.tooltipX = loadX
				e.tooltipY = loadY - 25
				e.showHelp = true
			} else
			// Help button
			if mx >= helpX && mx < helpX+60 && my >= helpY && my < helpY+24 {
				e.helpText = "Help: Toggle help screen with controls and features"
				e.tooltipX = helpX
				e.tooltipY = helpY - 25
				e.showHelp = true
			} else
			// BG buttons (Load, Set, Copy, Assets, Obstacles)
			if mx >= lx && mx < lx+lw && my >= ly && my < ly+lh {
				e.helpText = "Load BG: Load background image from file path"
				e.tooltipX = lx
				e.tooltipY = ly - 25
				e.showHelp = true
			} else if mx >= sx && mx < sx+sw && my >= sy && my < sy+sh {
				e.helpText = "Set BG: Set current background as map background"
				e.tooltipX = sx
				e.tooltipY = sy - 25
				e.showHelp = true
			} else if mx >= cx && mx < cx+cw && my >= cy && my < cy+ch {
				e.helpText = "Copy BG: Copy background to assets folder"
				e.tooltipX = cx
				e.tooltipY = cy - 25
				e.showHelp = true
			} else if mx >= ax && mx < ax+aw && my >= ay && my < ay+ah {
				e.helpText = "BG: Browse and load background images"
				e.tooltipX = ax
				e.tooltipY = ay - 25
				e.showHelp = true
			} else if mx >= ox && mx < ox+ow && my >= oy && my < oy+oh {
				e.helpText = "Obstacles: Browse and load obstacle images"
				e.tooltipX = ox
				e.tooltipY = oy - 25
				e.showHelp = true
			} else
			// Name field
			if mx >= nameX && mx < nameX+220 && my >= nameY && my < nameY+24 {
				e.helpText = "Map Name: Enter a name for your map"
				e.tooltipX = nameX
				e.tooltipY = nameY - 25
				e.showHelp = true
			} else
			// Check for deploy zone tooltips
			if e.bg != nil && inCanvas(mx, my) {
				// Check deploy zones
				for i, r := range e.def.DeployZones {
					x, y := toPix(r.X, r.Y)
					w := int(r.W * float64(dispW))
					h := int(r.H * float64(dispH))
					if mx >= x && mx < x+w && my >= y && my < y+h {
						owner := "Neutral"
						if r.Owner == "player" {
							owner = "Player"
						} else if r.Owner == "enemy" {
							owner = "Enemy"
						}
						e.helpText = fmt.Sprintf("Deploy Zone %d: %s (Click to change owner)", i+1, owner)
						e.tooltipX = mx + 10
						e.tooltipY = my - 25
						e.showHelp = true
						break
					}
				}
				// Check bases if no deploy zone tooltip
				if !e.showHelp {
					if hitPlayerBase(mx, my) {
						e.helpText = "Player Base"
						e.tooltipX = mx + 10
						e.tooltipY = my - 25
						e.showHelp = true
					} else if hitEnemyBase(mx, my) {
						e.helpText = "Enemy Base"
						e.tooltipX = mx + 10
						e.tooltipY = my - 25
						e.showHelp = true
					}
				}
			}
		}
	}

	// mouse press: select or create
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		e.dragging = false
		if inCanvas(mx, my) {
			if idx, h, ok := hitDeploy(mx, my); ok {
				// Check if this is a double-click to change owner
				if e.selKind == "deploy" && e.selIndex == idx && e.selHandle == h {
					// Change deploy zone owner
					if e.def.DeployZones[idx].Owner == "player" {
						e.def.DeployZones[idx].Owner = "enemy"
						e.status = fmt.Sprintf("Deploy Zone %d changed to Enemy", idx+1)
					} else if e.def.DeployZones[idx].Owner == "enemy" {
						e.def.DeployZones[idx].Owner = ""
						e.status = fmt.Sprintf("Deploy Zone %d changed to Neutral", idx+1)
					} else {
						e.def.DeployZones[idx].Owner = "player"
						e.status = fmt.Sprintf("Deploy Zone %d changed to Player", idx+1)
					}
				} else {
					e.selKind, e.selIndex, e.selHandle = "deploy", idx, h
					e.dragging, e.lastMx, e.lastMy = true, mx, my
				}
			} else if i, ok := hitPointList(e.def.MeetingStones, mx, my, 6); ok {
				e.selKind, e.selIndex, e.selHandle = "stone", i, -1
				e.dragging = true
				e.lastMx, e.lastMy = mx, my
			} else if i, ok := hitPointList(e.def.GoldMines, mx, my, 6); ok {
				e.selKind, e.selIndex, e.selHandle = "mine", i, -1
				e.dragging = true
				e.lastMx, e.lastMy = mx, my
			} else if i, ok := hitLane(mx, my); ok {
				e.selKind, e.selIndex, e.selHandle = "lane", i, -1
				e.dragging = true
				e.lastMx, e.lastMy = mx, my
			} else if idx, h, ok := hitObstacle(mx, my); ok {
				e.selKind, e.selIndex, e.selHandle = "obstacle", idx, h
				e.dragging, e.lastMx, e.lastMy = true, mx, my
			} else if hitPlayerBase(mx, my) {
				e.selKind, e.selIndex, e.selHandle = "playerbase", -1, -1
				e.dragging = true
				e.lastMx, e.lastMy = mx, my
			} else if hitEnemyBase(mx, my) {
				e.selKind, e.selIndex, e.selHandle = "enemybase", -1, -1
				e.dragging = true
				e.lastMx, e.lastMy = mx, my
			} else {
				nx, ny := toNorm(mx, my)
				switch e.tool {
				case 0:
					e.def.DeployZones = append(e.def.DeployZones, protocol.DeployZone{X: nx - 0.05, Y: ny - 0.05, W: 0.1, H: 0.1, Owner: "player"})
					e.selKind, e.selIndex, e.selHandle = "deploy", len(e.def.DeployZones)-1, -1
				case 1:
					e.def.MeetingStones = append(e.def.MeetingStones, protocol.PointF{X: nx, Y: ny})
					e.selKind, e.selIndex = "stone", len(e.def.MeetingStones)-1
				case 2:
					e.def.GoldMines = append(e.def.GoldMines, protocol.PointF{X: nx, Y: ny})
					e.selKind, e.selIndex = "mine", len(e.def.GoldMines)-1
				case 3:
					e.tmpLane = append(e.tmpLane, protocol.PointF{X: nx, Y: ny})
					e.selKind = ""
				case 4:
					e.def.Obstacles = append(e.def.Obstacles, protocol.Obstacle{X: nx - 0.05, Y: ny - 0.05, Type: "tree", Image: "tree.png", Width: 0.1, Height: 0.1})
					e.selKind, e.selIndex, e.selHandle = "obstacle", len(e.def.Obstacles)-1, -1
				}
				e.lastMx, e.lastMy = mx, my
			}
		}
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		e.dragging = false
	}

	// drag
	if e.dragging {
		dx := mx - e.lastMx
		dy := my - e.lastMy
		if dx != 0 || dy != 0 {
			e.lastMx, e.lastMy = mx, my
			ndx := float64(dx) / float64(dispW)
			ndy := float64(dy) / float64(dispH)
			switch e.selKind {
			case "deploy":
				if e.selIndex >= 0 && e.selIndex < len(e.def.DeployZones) {
					r := e.def.DeployZones[e.selIndex]
					if e.selHandle == -1 {
						r.X += ndx
						r.Y += ndy
					} else {
						switch e.selHandle {
						case 0:
							r.X += ndx
							r.Y += ndy
							r.W -= ndx
							r.H -= ndy
						case 1:
							r.Y += ndy
							r.W += ndx
							r.H -= ndy
						case 2:
							r.W += ndx
							r.H += ndy
						case 3:
							r.X += ndx
							r.W -= ndx
							r.H += ndy
						}
						if r.W < 0.02 {
							r.W = 0.02
						}
						if r.H < 0.02 {
							r.H = 0.02
						}
					}
					if r.X < 0 {
						r.X = 0
					}
					if r.Y < 0 {
						r.Y = 0
					}
					if r.X+r.W > 1 {
						r.X = 1 - r.W
					}
					if r.Y+r.H > 1 {
						r.Y = 1 - r.H
					}
					e.def.DeployZones[e.selIndex] = r
				}
			case "stone":
				if e.selIndex >= 0 && e.selIndex < len(e.def.MeetingStones) {
					p := e.def.MeetingStones[e.selIndex]
					p.X += ndx
					p.Y += ndy
					if p.X < 0 {
						p.X = 0
					}
					if p.Y < 0 {
						p.Y = 0
					}
					if p.X > 1 {
						p.X = 1
					}
					if p.Y > 1 {
						p.Y = 1
					}
					e.def.MeetingStones[e.selIndex] = p
				}
			case "mine":
				if e.selIndex >= 0 && e.selIndex < len(e.def.GoldMines) {
					p := e.def.GoldMines[e.selIndex]
					p.X += ndx
					p.Y += ndy
					if p.X < 0 {
						p.X = 0
					}
					if p.Y < 0 {
						p.Y = 0
					}
					if p.X > 1 {
						p.X = 1
					}
					if p.Y > 1 {
						p.Y = 1
					}
					e.def.GoldMines[e.selIndex] = p
				}
			case "lane":
				if e.selIndex >= 0 && e.selIndex < len(e.def.Lanes) {
					ln := e.def.Lanes[e.selIndex]
					for i := range ln.Points {
						ln.Points[i].X += ndx
						ln.Points[i].Y += ndy
					}
					e.def.Lanes[e.selIndex] = ln
				}
			case "obstacle":
				if e.selIndex >= 0 && e.selIndex < len(e.def.Obstacles) {
					obs := e.def.Obstacles[e.selIndex]
					if e.selHandle == -1 {
						obs.X += ndx
						obs.Y += ndy
					} else {
						switch e.selHandle {
						case 0:
							obs.X += ndx
							obs.Y += ndy
							obs.Width -= ndx
							obs.Height -= ndy
						case 1:
							obs.Y += ndy
							obs.Width += ndx
							obs.Height -= ndy
						case 2:
							obs.Width += ndx
							obs.Height += ndy
						case 3:
							obs.X += ndx
							obs.Width -= ndx
							obs.Height += ndy
						}
						if obs.Width < 0.02 {
							obs.Width = 0.02
						}
						if obs.Height < 0.02 {
							obs.Height = 0.02
						}
					}
					if obs.X < 0 {
						obs.X = 0
					}
					if obs.Y < 0 {
						obs.Y = 0
					}
					if obs.X+obs.Width > 1 {
						obs.X = 1 - obs.Width
					}
					if obs.Y+obs.Height > 1 {
						obs.Y = 1 - obs.Height
					}
					e.def.Obstacles[e.selIndex] = obs
				}
			case "playerbase":
				p := e.def.PlayerBase
				p.X += ndx
				p.Y += ndy
				if p.X < 0 {
					p.X = 0
				}
				if p.Y < 0 {
					p.Y = 0
				}
				if p.X > 1 {
					p.X = 1
				}
				if p.Y > 1 {
					p.Y = 1
				}
				e.def.PlayerBase = p
			case "enemybase":
				p := e.def.EnemyBase
				p.X += ndx
				p.Y += ndy
				if p.X < 0 {
					p.X = 0
				}
				if p.Y < 0 {
					p.Y = 0
				}
				if p.X > 1 {
					p.X = 1
				}
				if p.Y > 1 {
					p.Y = 1
				}
				e.def.EnemyBase = p
			}
		}
	}

	// Right click: finalize lane if drawing; otherwise clear selection
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonRight) {
		if len(e.tmpLane) > 0 {
			e.def.Lanes = append(e.def.Lanes, protocol.Lane{Points: append([]protocol.PointF(nil), e.tmpLane...), Dir: 1})
			e.tmpLane = nil
		} else {
			e.selKind, e.selIndex, e.selHandle = "", -1, -1
			e.dragging = false
		}
	}
	// Enhanced keyboard shortcuts
	for _, k := range inpututil.AppendJustPressedKeys(nil) {
		ctrlPressed := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyMeta)
		shiftPressed := ebiten.IsKeyPressed(ebiten.KeyShift)

		if k == ebiten.KeyS && ctrlPressed {
			e.save()
		}
		if k == ebiten.KeyN && ctrlPressed {
			// New map
			e.def = protocol.MapDef{Name: "New Map"}
			e.name = "New Map"
			e.status = "New map created"
		}
		if k == ebiten.KeyO && ctrlPressed {
			// Open map browser
			e.showMapBrowser = !e.showMapBrowser
			if e.showMapBrowser {
				e.availableMaps = []string{
					"east_gate", "mid_bridge", "north_tower", "south_gate", "west_keep", "tester",
					"colosseum", "forest_glade", "mountain_pass",
					"friendly_duel1", "friendly_duel2",
				}
				e.mapBrowserSel = 0
				e.mapBrowserScroll = 0
			}
		}
		if k == ebiten.KeyG && !e.bgFocus {
			e.showGrid = !e.showGrid
			if e.showGrid {
				e.status = "Grid overlay enabled"
			} else {
				e.status = "Grid overlay disabled"
			}
		}
		if k == ebiten.KeyC && ctrlPressed && !e.bgFocus {
			// Copy selected element (placeholder)
			e.status = "Copy functionality coming soon"
		}
		if k == ebiten.KeyV && ctrlPressed && !e.bgFocus {
			// Paste element (placeholder)
			e.status = "Paste functionality coming soon"
		}
		if k == ebiten.KeyZ && ctrlPressed && !e.bgFocus {
			// Undo (placeholder)
			e.status = "Undo functionality coming soon"
		}
		if k == ebiten.KeyY && ctrlPressed && !e.bgFocus {
			// Redo (placeholder)
			e.status = "Redo functionality coming soon"
		}
		if k == ebiten.KeyA && ctrlPressed && !e.bgFocus {
			// Select all (placeholder)
			e.status = "Select all functionality coming soon"
		}
		if k == ebiten.KeyEscape && !e.bgFocus {
			// Clear selection
			e.selKind = ""
			e.selIndex = -1
			e.selHandle = -1
			e.tmpLane = nil
			e.dragging = false
			e.status = "Selection cleared"
		}
		if k == ebiten.KeyDelete || (k == ebiten.KeyBackspace && shiftPressed) {
			if !e.bgFocus { // don't treat as delete when typing
				// Delete selected element
				switch e.selKind {
				case "deploy":
					if e.selIndex >= 0 && e.selIndex < len(e.def.DeployZones) {
						e.def.DeployZones = append(e.def.DeployZones[:e.selIndex], e.def.DeployZones[e.selIndex+1:]...)
						e.selKind = ""
						e.selIndex = -1
						e.status = "Deploy zone deleted"
					}
				case "stone":
					if e.selIndex >= 0 && e.selIndex < len(e.def.MeetingStones) {
						e.def.MeetingStones = append(e.def.MeetingStones[:e.selIndex], e.def.MeetingStones[e.selIndex+1:]...)
						e.selKind = ""
						e.selIndex = -1
						e.status = "Meeting stone deleted"
					}
				case "mine":
					if e.selIndex >= 0 && e.selIndex < len(e.def.GoldMines) {
						e.def.GoldMines = append(e.def.GoldMines[:e.selIndex], e.def.GoldMines[e.selIndex+1:]...)
						e.selKind = ""
						e.selIndex = -1
						e.status = "Gold mine deleted"
					}
				case "lane":
					if e.selIndex >= 0 && e.selIndex < len(e.def.Lanes) {
						e.def.Lanes = append(e.def.Lanes[:e.selIndex], e.def.Lanes[e.selIndex+1:]...)
						e.selKind = ""
						e.selIndex = -1
						e.status = "Movement lane deleted"
					}
				case "obstacle":
					if e.selIndex >= 0 && e.selIndex < len(e.def.Obstacles) {
						e.def.Obstacles = append(e.def.Obstacles[:e.selIndex], e.def.Obstacles[e.selIndex+1:]...)
						e.selKind = ""
						e.selIndex = -1
						e.status = "Obstacle deleted"
					}
				}
			}
		}
		if k == ebiten.KeyD && !e.bgFocus {
			if e.selKind == "lane" && e.selIndex >= 0 && e.selIndex < len(e.def.Lanes) {
				if e.def.Lanes[e.selIndex].Dir >= 0 {
					e.def.Lanes[e.selIndex].Dir = -1
					e.status = "Lane direction reversed"
				} else {
					e.def.Lanes[e.selIndex].Dir = 1
					e.status = "Lane direction normal"
				}
			}
		}
		if k == ebiten.KeyH && !e.bgFocus {
			e.helpMode = !e.helpMode
		}
		if k == ebiten.KeyF1 && !e.bgFocus {
			e.helpMode = !e.helpMode
		}

		// Number keys for tool selection
		if k >= ebiten.Key1 && k <= ebiten.Key5 && !e.bgFocus {
			toolIndex := int(k - ebiten.Key1)
			if toolIndex >= 0 && toolIndex < 5 {
				e.tool = toolIndex
				toolNames := []string{"Deploy Zones", "Meeting Stones", "Gold Mines", "Movement Lanes", "Obstacles"}
				e.status = fmt.Sprintf("Selected: %s", toolNames[toolIndex])
			}
		}
	}

	// Assets browser interactions
	// Toggle via button hit is handled in Draw click below (same top bar cluster)
	if e.showAssetsBrowser {
		// simple list on the right
		vw, vh := ebiten.WindowSize()
		panelX := vw - 240
		panelY := 40
		panelW := 232
		panelH := vh - panelY - 8
		rowH := 20
		maxRows := panelH / rowH
		// wheel scroll when hovering
		_, wy := ebiten.Wheel()
		if wy != 0 {
			if mx >= panelX && mx < panelX+panelW && my >= panelY && my < panelY+panelH {
				e.assetsBrowserScroll -= int(wy)
				if e.assetsBrowserScroll < 0 {
					e.assetsBrowserScroll = 0
				}
				if len(e.availableAssets) > maxRows {
					maxStart := len(e.availableAssets) - maxRows
					if e.assetsBrowserScroll > maxStart {
						e.assetsBrowserScroll = maxStart
					}
				}
			}
		}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if mx >= panelX && mx < panelX+panelW && my >= panelY && my < panelY+panelH {
				idx := (my-panelY)/rowH + e.assetsBrowserScroll
				if idx >= 0 && idx < len(e.availableAssets) {
					item := e.availableAssets[idx]
					if item == ".." {
						// go up
						parent := filepath.Dir(e.assetsCurrentPath)
						e.assetsCurrentPath = parent
						e.refreshAssetsBrowser()
					} else if strings.HasPrefix(item, "[DIR] ") {
						// enter directory
						dirName := strings.TrimPrefix(item, "[DIR] ")
						newPath := filepath.Join(e.assetsCurrentPath, dirName)
						e.assetsCurrentPath = newPath
						e.refreshAssetsBrowser()
					} else {
						// load file
						fullPath := filepath.Join(e.assetsCurrentPath, item)
						if img, _, err := ebitenutil.NewImageFromFile(fullPath); err == nil {
							e.bg = img
							e.bgPath = fullPath
							e.status = fmt.Sprintf("BG loaded: %s", filepath.Base(fullPath))
							e.showAssetsBrowser = false
						} else {
							e.status = fmt.Sprintf("Failed to load image: %v", err)
						}
					}
					e.assetsBrowserSel = idx
				}
			}
		}
	}
	return nil
}

func (e *editor) attemptLogin() {
	if strings.TrimSpace(e.loginUser) == "" || strings.TrimSpace(e.loginPass) == "" {
		e.loginStatus = "Please enter username and password"
		return
	}

	e.loginStatus = "Logging in..."

	// Run login in a goroutine to avoid blocking the UI
	go func() {
		req := map[string]interface{}{
			"username": e.loginUser,
			"password": e.loginPass,
			"version":  protocol.GameVersion,
		}
		b, _ := json.Marshal(req)

		resp, err := http.Post("http://localhost:8080/api/login", "application/json", strings.NewReader(string(b)))
		if err != nil {
			e.loginStatus = "Connection failed: " + err.Error()
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			if resp.StatusCode == 426 {
				e.loginStatus = "Version mismatch: please update your game"
			} else {
				e.loginStatus = "Invalid credentials"
			}
			return
		}

		var result map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			e.loginStatus = "Failed to parse response"
			return
		}

		token, ok := result["token"].(string)
		if !ok {
			e.loginStatus = "Invalid response format"
			return
		}

		// Save token
		if err := os.WriteFile(filepath.Join(configDir(), "token.json"), []byte(strings.TrimSpace(token)), 0o600); err != nil {
			e.loginStatus = "Failed to save token"
			return
		}

		// Save username
		if err := os.WriteFile(filepath.Join(configDir(), "username.txt"), []byte(strings.TrimSpace(e.loginUser)), 0o600); err != nil {
			e.loginStatus = "Failed to save username"
			return
		}

		e.loginStatus = "Login successful! Connecting..."

		// Connect to WebSocket
		ws, err := dialWS(getenv("WAR_WS_URL", "ws://127.0.0.1:8080/ws"), token)
		if err != nil {
			e.loginStatus = "WebSocket connection failed: " + err.Error()
			return
		}

		e.ws = ws
		e.inCh = make(chan wsMsg, 128)
		go e.runReader()
		e.showLogin = false
		e.loginStatus = ""
	}()
}

func (e *editor) save() {
	nm := strings.TrimSpace(e.name)
	if nm == "" {
		nm = "New Map"
	}
	e.def.Name = nm
	if strings.TrimSpace(e.def.ID) == "" {
		e.def.ID = strings.ReplaceAll(strings.ToLower(nm), " ", "-")
	}

	// Save locally
	localDir := "local_maps"
	_ = os.MkdirAll(localDir, 0o755)
	filename := e.def.ID + ".json"
	filepath := filepath.Join(localDir, filename)
	b, _ := json.MarshalIndent(e.def, "", "  ")
	localErr := os.WriteFile(filepath, b, 0o644)

	if e.ws == nil {
		if localErr == nil {
			e.status = "Saved locally to " + filepath
		} else {
			e.status = "Local save failed: " + localErr.Error()
		}
		return
	}

	// Send to server
	b2, _ := json.Marshal(struct {
		Type string      `json:"type"`
		Data interface{} `json:"data"`
	}{Type: "SaveMap", Data: protocol.SaveMap{Def: e.def}})
	err := e.ws.WriteMessage(websocket.TextMessage, b2)
	if err != nil {
		if localErr == nil {
			e.status = "Local save ok, server save failed: " + err.Error()
		} else {
			e.status = "Save failed: " + err.Error()
		}
	} else {
		if localErr == nil {
			e.status = "Saved to server and locally"
		} else {
			e.status = "Saved to server, local save failed: " + localErr.Error()
		}
	}
}

func (e *editor) refreshAssetsBrowser() {
	var items []string
	if e.assetsCurrentPath != "." {
		items = append(items, "..")
	}
	entries, err := os.ReadDir(e.assetsCurrentPath)
	if err != nil {
		e.status = fmt.Sprintf("Error reading directory: %v", err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			items = append(items, "[DIR] "+entry.Name())
		} else {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				items = append(items, entry.Name())
			}
		}
	}
	e.availableAssets = items
	e.assetsBrowserScroll = 0
	e.assetsBrowserSel = -1
	e.status = fmt.Sprintf("Browsing: %s (%d items)", e.assetsCurrentPath, len(items))
}

func (e *editor) refreshObstaclesBrowser() {
	var items []string
	if e.obstaclesCurrentPath != "." {
		items = append(items, "..")
	}
	entries, err := os.ReadDir(e.obstaclesCurrentPath)
	if err != nil {
		e.status = fmt.Sprintf("Error reading directory: %v", err)
		return
	}
	for _, entry := range entries {
		if entry.IsDir() {
			items = append(items, "[DIR] "+entry.Name())
		} else {
			ext := strings.ToLower(filepath.Ext(entry.Name()))
			if ext == ".png" || ext == ".jpg" || ext == ".jpeg" {
				items = append(items, entry.Name())
			}
		}
	}
	e.availableObstacles = items
	e.obstaclesBrowserScroll = 0
	e.obstaclesBrowserSel = -1
	e.status = fmt.Sprintf("Browsing: %s (%d items)", e.obstaclesCurrentPath, len(items))
}

func (e *editor) Draw(screen *ebiten.Image) {
	vw, vh := ebiten.WindowSize()

	// compute display rect consistent with Draw
	const topUIH = 120
	var dispW, dispH int
	if e.bg == nil {
		dispW, dispH = vw, vh-topUIH
	} else {
		sw, sh := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
		vh -= topUIH
		sx := float64(vw) / float64(sw)
		sy := float64(vh) / float64(sh)
		s := sx
		if sy < sx {
			s = sy
		}
		dispW = int(float64(sw) * s)
		dispH = int(float64(sh) * s)
	}

	// Login screen
	if e.showLogin {
		ebitenutil.DrawRect(screen, 0, 0, float64(vw), float64(vh), color.NRGBA{20, 20, 30, 255})

		// Title
		title := "Map Editor Login"
		tw := len(title) * 7 // approximate width
		tx := (vw - tw) / 2
		ty := vh/2 - 100
		text.Draw(screen, title, basicfont.Face7x13, tx, ty, color.White)

		// Username field
		ux := (vw - 200) / 2
		uy := vh/2 - 40
		ebitenutil.DrawRect(screen, float64(ux), float64(uy), 200, 24, color.NRGBA{40, 40, 50, 255})
		if e.loginFocus == 0 {
			ebitenutil.DrawRect(screen, float64(ux), float64(uy), 200, 24, color.NRGBA{60, 60, 80, 255})
		}
		userText := e.loginUser
		if e.loginFocus == 0 {
			userText += "|"
		}
		text.Draw(screen, "Username: "+userText, basicfont.Face7x13, ux-80, uy+16, color.White)

		// Password field
		px := (vw - 200) / 2
		py := vh / 2
		ebitenutil.DrawRect(screen, float64(px), float64(py), 200, 24, color.NRGBA{40, 40, 50, 255})
		if e.loginFocus == 1 {
			ebitenutil.DrawRect(screen, float64(px), float64(py), 200, 24, color.NRGBA{60, 60, 80, 255})
		}
		passText := strings.Repeat("*", len(e.loginPass))
		if e.loginFocus == 1 {
			passText += "|"
		}
		text.Draw(screen, "Password: "+passText, basicfont.Face7x13, px-80, py+16, color.White)

		// Login button
		bx := (vw - 100) / 2
		by := vh/2 + 60
		ebitenutil.DrawRect(screen, float64(bx), float64(by), 100, 30, color.NRGBA{60, 90, 120, 255})
		text.Draw(screen, "Login", basicfont.Face7x13, bx+30, by+20, color.White)

		// Status
		if e.loginStatus != "" {
			statusCol := color.NRGBA{255, 100, 100, 255}
			if strings.Contains(e.loginStatus, "success") {
				statusCol = color.NRGBA{100, 255, 100, 255}
			}
			sw := len(e.loginStatus) * 7
			sx := (vw - sw) / 2
			sy := vh/2 + 120
			text.Draw(screen, e.loginStatus, basicfont.Face7x13, sx, sy, statusCol)
		}

		return
	}

	// Top UI bar background to prevent overlap with canvas
	ebitenutil.DrawRect(screen, 0, 0, float64(vw), float64(topUIH), color.NRGBA{28, 28, 40, 255})

	// BG
	if e.bg != nil {
		// simple fit top with 32px toolbar reserved
		sw, sh := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
		vw, vh = ebiten.WindowSize()
		vh -= topUIH
		sx := float64(vw) / float64(sw)
		sy := float64(vh) / float64(sh)
		s := sx
		if sy < sx {
			s = sy
		}
		dw := int(float64(sw) * s)
		dh := int(float64(sh) * s)
		offX := (vw - dw) / 2
		offY := topUIH + (vh-dh)/2
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(s, s)
		op.GeoM.Translate(float64(offX), float64(offY))
		screen.DrawImage(e.bg, op)
		// overlays
		toX := func(nx float64) int { return offX + int(nx*float64(dw)) }
		toY := func(ny float64) int { return offY + int(ny*float64(dh)) }
		if e.showGrid {
			// draw a light 10% grid
			for i := 1; i < 10; i++ {
				x := toX(float64(i) / 10.0)
				y := toY(float64(i) / 10.0)
				ebitenutil.DrawLine(screen, float64(x), float64(offY), float64(x), float64(offY+dh), color.NRGBA{60, 60, 70, 120})
				ebitenutil.DrawLine(screen, float64(offX), float64(y), float64(offX+dw), float64(y), color.NRGBA{60, 60, 70, 120})
			}
		}
		for i, r := range e.def.DeployZones {
			x := toX(r.X)
			y := toY(r.Y)
			rw := int(r.W * float64(dw))
			rh := int(r.H * float64(dh))
			ebitenutil.DrawRect(screen, float64(x), float64(y), float64(rw), float64(rh), color.NRGBA{60, 150, 90, 90})
			if e.selKind == "deploy" && e.selIndex == i {
				// border
				ebitenutil.DrawRect(screen, float64(x), float64(y), float64(rw), 1, color.NRGBA{240, 196, 25, 255})
				ebitenutil.DrawRect(screen, float64(x), float64(y+rh-1), float64(rw), 1, color.NRGBA{240, 196, 25, 255})
				ebitenutil.DrawRect(screen, float64(x), float64(y), 1, float64(rh), color.NRGBA{240, 196, 25, 255})
				ebitenutil.DrawRect(screen, float64(x+rw-1), float64(y), 1, float64(rh), color.NRGBA{240, 196, 25, 255})
				// handles
				handles := [][2]int{{x, y}, {x + rw, y}, {x + rw, y + rh}, {x, y + rh}}
				for _, h := range handles {
					ebitenutil.DrawRect(screen, float64(h[0]-4), float64(h[1]-4), 8, 8, color.NRGBA{240, 196, 25, 255})
				}
			}
		}
		for i, p := range e.def.MeetingStones {
			col := color.NRGBA{140, 120, 220, 255}
			if e.selKind == "stone" && e.selIndex == i {
				col = color.NRGBA{240, 196, 25, 255}
			}
			ebitenutil.DrawRect(screen, float64(toX(p.X)-2), float64(toY(p.Y)-2), 4, 4, col)
		}
		for i, p := range e.def.GoldMines {
			col := color.NRGBA{200, 170, 40, 255}
			if e.selKind == "mine" && e.selIndex == i {
				col = color.NRGBA{240, 196, 25, 255}
			}
			ebitenutil.DrawRect(screen, float64(toX(p.X)-3), float64(toY(p.Y)-3), 6, 6, col)
		}
		drawPath := func(pts []protocol.PointF, col color.NRGBA) {
			for i := 1; i < len(pts); i++ {
				x0, y0 := toX(pts[i-1].X), toY(pts[i-1].Y)
				x1, y1 := toX(pts[i].X), toY(pts[i].Y)
				ebitenutil.DrawLine(screen, float64(x0), float64(y0), float64(x1), float64(y1), col)
			}
		}
		for i, ln := range e.def.Lanes {
			col := color.NRGBA{90, 160, 220, 255}
			if ln.Dir < 0 {
				col = color.NRGBA{220, 110, 110, 255}
			}
			if e.selKind == "lane" && e.selIndex == i {
				col = color.NRGBA{240, 196, 25, 255}
			}
			drawPath(ln.Points, col)
		}
		if len(e.tmpLane) > 0 {
			drawPath(e.tmpLane, color.NRGBA{200, 220, 90, 255})
		}
		for i, obs := range e.def.Obstacles {
			x := toX(obs.X)
			y := toY(obs.Y)
			w := int(obs.Width * float64(dispW))
			h := int(obs.Height * float64(dispH))

			// Try to draw the actual obstacle image if available
			drewImage := false
			if obs.Image != "" {
				// Try multiple paths for obstacle images
				obstaclePaths := []string{
					filepath.Join("..", "..", "client", "internal", "game", "assets", "obstacles", obs.Image), // From mapeditor to client assets
					filepath.Join("obstacles", obs.Image),                                                     // Local obstacles directory
					obs.Image,                                                                                 // Original path
				}

				for _, obstaclePath := range obstaclePaths {
					if img, _, err := ebitenutil.NewImageFromFile(obstaclePath); err == nil {
						op := &ebiten.DrawImageOptions{}
						scaleX := float64(w) / float64(img.Bounds().Dx())
						scaleY := float64(h) / float64(img.Bounds().Dy())
						op.GeoM.Scale(scaleX, scaleY)
						op.GeoM.Translate(float64(x), float64(y))
						screen.DrawImage(img, op)
						drewImage = true
						break
					}
				}
			}

			// Fallback to rectangle if image couldn't be loaded
			if !drewImage {
				ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), color.NRGBA{120, 80, 40, 120})
			}

			if e.selKind == "obstacle" && e.selIndex == i {
				// border
				ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), 1, color.NRGBA{240, 196, 25, 255})
				ebitenutil.DrawRect(screen, float64(x), float64(y+h-1), float64(w), 1, color.NRGBA{240, 196, 25, 255})
				ebitenutil.DrawRect(screen, float64(x), float64(y), 1, float64(h), color.NRGBA{240, 196, 25, 255})
				ebitenutil.DrawRect(screen, float64(x+w-1), float64(y), 1, float64(h), color.NRGBA{240, 196, 25, 255})
				// handles
				handles := [][2]int{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}}
				for _, h := range handles {
					ebitenutil.DrawRect(screen, float64(h[0]-4), float64(h[1]-4), 8, 8, color.NRGBA{240, 196, 25, 255})
				}
			}
		}
		// Draw player base
		if e.def.PlayerBase.X != 0 || e.def.PlayerBase.Y != 0 {
			x := toX(e.def.PlayerBase.X)
			y := toY(e.def.PlayerBase.Y)

			// Try to load base image
			baseImageLoaded := false
			basePaths := []string{
				filepath.Join("..", "..", "client", "internal", "game", "assets", "ui", "base.png"),
				filepath.Join("ui", "base.png"),
				"base.png",
			}

			for _, basePath := range basePaths {
				if img, _, err := ebitenutil.NewImageFromFile(basePath); err == nil {
					// Use the actual base size from battle system (96x96 pixels)
					baseW := 96
					baseH := 96

					// Scale image to fit the base dimensions
					scaleX := float64(baseW) / float64(img.Bounds().Dx())
					scaleY := float64(baseH) / float64(img.Bounds().Dy())
					scale := math.Min(scaleX, scaleY)

					op := &ebiten.DrawImageOptions{}
					op.GeoM.Scale(scale, scale)
					op.GeoM.Translate(float64(x), float64(y))
					screen.DrawImage(img, op)
					baseImageLoaded = true
					break
				}
			}

			// Fallback to rectangle if image not found - use actual base size
			if !baseImageLoaded {
				baseW := 96
				baseH := 96
				col := color.NRGBA{0, 150, 255, 255} // Blue for player
				if e.selKind == "playerbase" {
					col = color.NRGBA{240, 196, 25, 255}
				}
				ebitenutil.DrawRect(screen, float64(x), float64(y), float64(baseW), float64(baseH), col)
			}

			// Draw "PLAYER BASE" label above the base
			label := "PLAYER BASE"
			labelW := len(label) * 7
			labelX := x + (96-labelW)/2
			labelY := y - 20
			ebitenutil.DrawRect(screen, float64(labelX-4), float64(labelY-2), float64(labelW+8), 16, color.NRGBA{0, 0, 0, 180})
			text.Draw(screen, label, basicfont.Face7x13, labelX, labelY+12, color.NRGBA{0, 150, 255, 255})

			// Draw selection border if selected
			if e.selKind == "playerbase" {
				ebitenutil.DrawRect(screen, float64(x-2), float64(y-2), float64(100), 2, color.NRGBA{240, 196, 25, 255})
				ebitenutil.DrawRect(screen, float64(x-2), float64(y+96), float64(100), 2, color.NRGBA{240, 196, 25, 255})
				ebitenutil.DrawRect(screen, float64(x-2), float64(y-2), 2, float64(100), color.NRGBA{240, 196, 25, 255})
				ebitenutil.DrawRect(screen, float64(x+96), float64(y-2), 2, float64(100), color.NRGBA{240, 196, 25, 255})
			}
		}
		// Draw enemy base
		if e.def.EnemyBase.X != 0 || e.def.EnemyBase.Y != 0 {
			x := toX(e.def.EnemyBase.X)
			y := toY(e.def.EnemyBase.Y)

			// Try to load base image
			baseImageLoaded := false
			basePaths := []string{
				filepath.Join("..", "..", "client", "internal", "game", "assets", "ui", "base.png"),
				filepath.Join("ui", "base.png"),
				"base.png",
			}

			for _, basePath := range basePaths {
				if img, _, err := ebitenutil.NewImageFromFile(basePath); err == nil {
					// Use the actual base size from battle system (96x96 pixels)
					baseW := 96
					baseH := 96

					// Scale image to fit the base dimensions
					scaleX := float64(baseW) / float64(img.Bounds().Dx())
					scaleY := float64(baseH) / float64(img.Bounds().Dy())
					scale := math.Min(scaleX, scaleY)

					op := &ebiten.DrawImageOptions{}
					op.GeoM.Scale(scale, scale)
					op.GeoM.Translate(float64(x), float64(y))
					screen.DrawImage(img, op)
					baseImageLoaded = true
					break
				}
			}

			// Fallback to rectangle if image not found - use actual base size
			if !baseImageLoaded {
				baseW := 96
				baseH := 96
				col := color.NRGBA{255, 0, 0, 255} // Red for enemy
				if e.selKind == "enemybase" {
					col = color.NRGBA{240, 196, 25, 255}
				}
				ebitenutil.DrawRect(screen, float64(x), float64(y), float64(baseW), float64(baseH), col)
			}

			// Draw "ENEMY BASE" label above the base
			label := "ENEMY BASE"
			labelW := len(label) * 7
			labelX := x + (96-labelW)/2
			labelY := y - 20
			ebitenutil.DrawRect(screen, float64(labelX-4), float64(labelY-2), float64(labelW+8), 16, color.NRGBA{0, 0, 0, 180})
			text.Draw(screen, label, basicfont.Face7x13, labelX, labelY+12, color.NRGBA{255, 0, 0, 255})

			// Draw selection border if selected
			if e.selKind == "enemybase" {
				ebitenutil.DrawRect(screen, float64(x-2), float64(y-2), float64(100), 2, color.NRGBA{240, 196, 25, 255})
				ebitenutil.DrawRect(screen, float64(x-2), float64(y+96), float64(100), 2, color.NRGBA{240, 196, 25, 255})
				ebitenutil.DrawRect(screen, float64(x-2), float64(y-2), 2, float64(100), color.NRGBA{240, 196, 25, 255})
				ebitenutil.DrawRect(screen, float64(x+96), float64(y-2), 2, float64(100), color.NRGBA{240, 196, 25, 255})
			}
		}
	}
	// Enhanced toolbar with better styling and fixed text positioning
	toolLabels := []string{"Deploy", "Stones", "Mines", "Lanes", "Obstacles"}
	toolIcons := []string{"📦", "🏛️", "💰", "🛤️", "🌳"}
	toolColors := []color.NRGBA{
		{0x4a, 0x9e, 0xff, 0xff}, // Blue for deploy
		{0x9c, 0x7a, 0xff, 0xff}, // Purple for stones
		{0xff, 0xd7, 0x00, 0xff}, // Gold for mines
		{0x4a, 0xff, 0x7a, 0xff}, // Green for lanes
		{0x8b, 0x45, 0x13, 0xff}, // Brown for obstacles
	}

	for i, lb := range toolLabels {
		x, y, w, h := 8, 8+i*40, 100, 36

		// Background with gradient effect
		baseCol := color.NRGBA{0x2a, 0x2a, 0x3a, 0xff}
		if e.tool == i {
			baseCol = toolColors[i]
		}

		// Draw main button with shadow effect
		ebitenutil.DrawRect(screen, float64(x+1), float64(y+1), float64(w), float64(h), color.NRGBA{0x1a, 0x1a, 0x2a, 0xff})
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), baseCol)

		// Add border
		borderCol := color.NRGBA{0x5a, 0x5a, 0x6a, 0xff}
		if e.tool == i {
			borderCol = color.NRGBA{0xff, 0xff, 0xff, 0xff}
		}
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), 1, borderCol)
		ebitenutil.DrawRect(screen, float64(x), float64(y+h-1), float64(w), 1, borderCol)
		ebitenutil.DrawRect(screen, float64(x), float64(y), 1, float64(h), borderCol)
		ebitenutil.DrawRect(screen, float64(x+w-1), float64(y), 1, float64(h), borderCol)

		// Draw icon and text (fixed positioning)
		iconX := x + 8
		textX := x + 8
		text.Draw(screen, toolIcons[i], basicfont.Face7x13, iconX, y+16, color.White)
		text.Draw(screen, lb, basicfont.Face7x13, textX, y+30, color.White)

		// Selection indicator
		if e.tool == i {
			ebitenutil.DrawRect(screen, float64(x-2), float64(y), 3, float64(h), color.NRGBA{0xff, 0xff, 0xff, 0xff})
		}
	}

	// Quick action buttons (repositioned)
	quickActions := []string{"Grid", "Snap", "Undo", "Redo"}
	quickColors := []color.NRGBA{
		{0x6a, 0x6a, 0x7a, 0xff},
		{0x7a, 0x6a, 0x8a, 0xff},
		{0x8a, 0x7a, 0x6a, 0xff},
		{0x6a, 0x8a, 0x7a, 0xff},
	}

	for i, action := range quickActions {
		x, y, w, h := 120, 8+i*40, 80, 36

		col := quickColors[i]
		if (action == "Grid" && e.showGrid) || (action == "Snap" && e.showGrid) {
			col = color.NRGBA{0xaa, 0x77, 0x55, 0xff}
		}

		// Shadow effect
		ebitenutil.DrawRect(screen, float64(x+1), float64(y+1), float64(w), float64(h), color.NRGBA{0x4a, 0x4a, 0x5a, 0xff})
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), col)
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), 1, color.NRGBA{0x9a, 0x9a, 0xaa, 0xff})
		ebitenutil.DrawRect(screen, float64(x), float64(y+h-1), float64(w), 1, color.NRGBA{0x5a, 0x5a, 0x6a, 0xff})
		ebitenutil.DrawRect(screen, float64(x), float64(y), 1, float64(h), color.NRGBA{0x9a, 0x9a, 0xaa, 0xff})
		ebitenutil.DrawRect(screen, float64(x+w-1), float64(y), 1, float64(h), color.NRGBA{0x5a, 0x5a, 0x6a, 0xff})

		text.Draw(screen, action, basicfont.Face7x13, x+8, y+20, color.White)
	}
	// name + save
	// name input box (first row)
	nameLblX := 140
	nameLblY := 8
	text.Draw(screen, "Name:", basicfont.Face7x13, nameLblX, nameLblY+16, color.White)
	nameX := nameLblX + 48
	nameY := 8
	nm := e.name
	text.Draw(screen, nm, basicfont.Face7x13, nameX+6, nameY+16, color.White)
	saveX := nameX + 240
	saveY := 8
	ebitenutil.DrawRect(screen, float64(saveX), float64(saveY), 100, 24, color.NRGBA{70, 110, 70, 255})
	text.Draw(screen, "Save", basicfont.Face7x13, saveX+30, saveY+16, color.White)
	// Clear selection button
	clrX := saveX + 110
	clrY := saveY
	ebitenutil.DrawRect(screen, float64(clrX), float64(clrY), 80, 24, color.NRGBA{90, 70, 70, 255})
	text.Draw(screen, "Clear", basicfont.Face7x13, clrX+20, clrY+16, color.White)

	// Load Map button
	loadX := clrX + 90
	loadY := clrY
	ebitenutil.DrawRect(screen, float64(loadX), float64(loadY), 90, 24, color.NRGBA{70, 90, 120, 255})
	text.Draw(screen, "Load Map", basicfont.Face7x13, loadX+8, loadY+16, color.White)

	// Help button
	helpX := loadX + 100
	helpY := loadY
	ebitenutil.DrawRect(screen, float64(helpX), float64(helpY), 60, 24, color.NRGBA{120, 120, 70, 255})
	text.Draw(screen, "Help", basicfont.Face7x13, helpX+12, helpY+16, color.White)

	mx, my := ebiten.CursorPosition()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if mx >= saveX && mx < saveX+100 && my >= saveY && my < saveY+24 {
			e.save()
		}
		if mx >= clrX && mx < clrX+80 && my >= clrY && my < clrY+24 {
			e.selKind = ""
			e.selIndex = -1
			e.selHandle = -1
			e.tmpLane = nil
			e.dragging = false
		}
		if mx >= loadX && mx < loadX+90 && my >= loadY && my < loadY+24 {
			e.showMapBrowser = !e.showMapBrowser
			if e.showMapBrowser {
				// Load available maps from all directories
				e.availableMaps = []string{
					// PVE Maps
					"east_gate", "mid_bridge", "north_tower", "south_gate", "west_keep", "tester",
					// PVP Arenas
					"colosseum", "forest_glade", "mountain_pass",
					// Friendly Duels
					"friendly_duel1", "friendly_duel2",
				}
				e.mapBrowserSel = 0
				e.mapBrowserScroll = 0
			}
		}
		if mx >= helpX && mx < helpX+60 && my >= helpY && my < helpY+24 {
			e.helpMode = !e.helpMode
		}
	}
	if e.status != "" {
		// Draw status message in a more visible location - below the buttons
		text.Draw(screen, e.status, basicfont.Face7x13, 8, 8+5*28+20, color.NRGBA{180, 220, 180, 255})
	}

	// BG path controls: input + Load + Set + Copy (second row)
	bx := 140
	by := 48
	// input box
	if e.bgFocus {
		ebitenutil.DrawRect(screen, float64(bx), float64(by), 260, 24, color.NRGBA{24, 28, 40, 220})
	}
	path := e.bgInput
	if path == "" {
		path = e.bgPath
	}
	show := path
	if len(show) > 34 {
		show = "…" + show[len(show)-33:]
	}
	text.Draw(screen, "BG: "+show, basicfont.Face7x13, bx+6, by+16, color.White)
	// buttons
	btn := func(x int, label string) (int, int, int, int) {
		w := 64
		h := 24
		ebitenutil.DrawRect(screen, float64(x), float64(by), float64(w), float64(h), color.NRGBA{60, 90, 120, 255})
		text.Draw(screen, label, basicfont.Face7x13, x+10, by+16, color.White)
		return x, by, w, h
	}
	lx, ly, lw, lh := btn(bx+270, "Load")
	sx, sy, sw, sh := btn(bx+270+70, "Set")
	cx, cy, cw, ch := btn(bx+270+140, "Copy")
	ax, ay, aw, ah := btn(bx+270+210, "BG")
	ox, oy, ow, oh := btn(bx+270+280, "Obstacles")
	// focus input when clicking the box
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if mx >= bx && mx < bx+260 && my >= by && my < by+24 {
			e.bgFocus = true
		} else if !(mx >= lx && mx < lx+lw && my >= ly && my < ly+lh) && !(mx >= sx && mx < sx+sw && my >= sy && my < sy+sh) && !(mx >= cx && mx < cx+cw && my >= cy && my < cy+ch) {
			e.bgFocus = false
		}
		if mx >= lx && mx < lx+lw && my >= ly && my < ly+lh { // Load
			e.status = fmt.Sprintf("Load button clicked at (%d,%d)", mx, my)
			p := strings.TrimSpace(e.bgInput)
			if p == "" {
				p = e.bgPath
			}
			if p != "" {
				if img, _, err := ebitenutil.NewImageFromFile(p); err == nil {
					e.bg = img
					e.bgPath = p
					e.status = fmt.Sprintf("BG loaded: %s", filepath.Base(p))
				} else {
					e.status = fmt.Sprintf("BG load failed: %v", err)
				}
			} else {
				e.status = "No background path specified"
			}
		}
		if mx >= sx && mx < sx+sw && my >= sy && my < sy+sh { // Set MapDef Bg
			if e.bgPath != "" {
				e.def.Bg = e.bgPath
				e.status = fmt.Sprintf("BG set in map: %s", filepath.Base(e.bgPath))
			} else {
				e.status = "No background loaded to set"
			}
		}
		if mx >= cx && mx < cx+cw && my >= cy && my < cy+ch { // Copy to assets/maps/<id>.*
			if e.bgPath != "" && strings.TrimSpace(e.def.ID) != "" {
				ext := filepath.Ext(e.bgPath)
				if ext == "" {
					ext = ".png"
				}
				dst := filepath.Join("client", "internal", "game", "assets", "maps", e.def.ID+ext)
				_ = os.MkdirAll(filepath.Dir(dst), 0o755)
				if in, err := os.Open(e.bgPath); err == nil {
					defer in.Close()
					if out, err2 := os.Create(dst); err2 == nil {
						defer out.Close()
						if _, err3 := io.Copy(out, in); err3 == nil {
							e.status = fmt.Sprintf("BG copied to %s", filepath.Base(dst))
						} else {
							e.status = fmt.Sprintf("Copy failed: %v", err3)
						}
					} else {
						e.status = fmt.Sprintf("Create failed: %v", err2)
					}
				} else {
					e.status = fmt.Sprintf("Open failed: %v", err)
				}
			} else {
				e.status = "No background or map ID to copy"
			}
		}
		if mx >= ax && mx < ax+aw && my >= ay && my < ay+ah { // Toggle assets browser and refresh
			e.showAssetsBrowser = !e.showAssetsBrowser
			if e.showAssetsBrowser {
				if e.assetsCurrentPath == "" {
					e.assetsCurrentPath = "../client/internal/game/assets/maps"
				}
				e.refreshAssetsBrowser()
			}
		}
		if mx >= ox && mx < ox+ow && my >= oy && my < oy+oh { // Toggle obstacles browser and refresh
			e.showObstaclesBrowser = !e.showObstaclesBrowser
			if e.showObstaclesBrowser {
				if e.obstaclesCurrentPath == "" {
					e.obstaclesCurrentPath = filepath.Join("..", "..", "client", "internal", "game", "assets", "obstacles")
				}
				e.refreshObstaclesBrowser()
			}
		}
	}
	// Handle input for focused text fields
	if e.bgFocus {
		for _, k := range inpututil.AppendJustPressedKeys(nil) {
			if k == ebiten.KeyBackspace && len(e.bgInput) > 0 {
				e.bgInput = e.bgInput[:len(e.bgInput)-1]
			}
			if k == ebiten.KeyEnter {
				p := strings.TrimSpace(e.bgInput)
				if p == "" {
					p = e.bgPath
				}
				if p != "" {
					if img, _, err := ebitenutil.NewImageFromFile(p); err == nil {
						e.bg = img
						e.bgPath = p
						e.status = "BG loaded"
					} else {
						e.status = "BG load failed"
					}
				}
			}
		}
		for _, r := range ebiten.AppendInputChars(nil) {
			if r >= 32 {
				e.bgInput += string(r)
			}
		}
	}

	// Show live normalized mouse coordinates (below toolbar)
	if e.bg != nil {
		const topUIH = 120
		vw, vh := ebiten.WindowSize()
		sw, sh := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
		sx := float64(vw) / float64(sw)
		sy := float64(vh-topUIH) / float64(sh)
		s := sx
		if sy < sx {
			s = sy
		}
		dw := int(float64(sw) * s)
		dh := int(float64(sh) * s)
		offX := (vw - dw) / 2
		offY := topUIH + (vh-topUIH-dh)/2
		if mx >= offX && mx < offX+dw && my >= offY && my < offY+dh {
			nx := (float64(mx - offX)) / float64(dw)
			ny := (float64(my - offY)) / float64(dh)
			text.Draw(screen, fmt.Sprintf("(%.3f, %.3f)", nx, ny), basicfont.Face7x13, 8, 8+4*28, color.NRGBA{200, 200, 210, 255})
		}
	}

	// Map browser panel (draw)
	if e.showMapBrowser {
		vw, vh := ebiten.WindowSize()
		panelX := vw/2 - 200
		panelY := vh/2 - 150
		panelW := 400
		panelH := 300
		ebitenutil.DrawRect(screen, float64(panelX), float64(panelY), float64(panelW), float64(panelH), color.NRGBA{20, 20, 30, 240})
		ebitenutil.DrawRect(screen, float64(panelX+2), float64(panelY+2), float64(panelW-4), 24, color.NRGBA{40, 40, 60, 255})
		text.Draw(screen, "Load Map", basicfont.Face7x13, panelX+10, panelY+18, color.White)

		// Close button
		closeX := panelX + panelW - 30
		closeY := panelY + 2
		ebitenutil.DrawRect(screen, float64(closeX), float64(closeY), 28, 20, color.NRGBA{100, 60, 60, 255})
		text.Draw(screen, "X", basicfont.Face7x13, closeX+10, closeY+15, color.White)

		rowH := 20
		maxRows := (panelH - 60) / rowH
		start := e.mapBrowserScroll
		for i := 0; i < maxRows && start+i < len(e.availableMaps); i++ {
			yy := panelY + 30 + i*rowH
			if (start+i)%2 == 0 {
				ebitenutil.DrawRect(screen, float64(panelX+4), float64(yy-12), float64(panelW-8), 18, color.NRGBA{40, 40, 56, 255})
			}
			mapName := e.availableMaps[start+i]
			var acol color.Color = color.White
			if e.mapBrowserSel == start+i {
				acol = color.NRGBA{240, 196, 25, 255}
				ebitenutil.DrawRect(screen, float64(panelX+4), float64(yy-12), float64(panelW-8), 18, color.NRGBA{60, 60, 80, 255})
			}
			text.Draw(screen, mapName, basicfont.Face7x13, panelX+8, yy, acol)
		}

		// Load button
		loadBtnX := panelX + panelW/2 - 40
		loadBtnY := panelY + panelH - 30
		ebitenutil.DrawRect(screen, float64(loadBtnX), float64(loadBtnY), 80, 24, color.NRGBA{60, 90, 120, 255})
		text.Draw(screen, "Load", basicfont.Face7x13, loadBtnX+25, loadBtnY+16, color.White)

		// Handle map browser interactions
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if mx >= closeX && mx < closeX+28 && my >= closeY && my < closeY+20 {
				e.showMapBrowser = false
			} else if mx >= loadBtnX && mx < loadBtnX+80 && my >= loadBtnY && my < loadBtnY+24 {
				if e.mapBrowserSel >= 0 && e.mapBrowserSel < len(e.availableMaps) {
					mapID := e.availableMaps[e.mapBrowserSel]
					if e.ws != nil {
						b, _ := json.Marshal(struct {
							Type string      `json:"type"`
							Data interface{} `json:"data"`
						}{Type: "GetMap", Data: protocol.GetMap{ID: mapID}})
						_ = e.ws.WriteMessage(websocket.TextMessage, b)
						e.showMapBrowser = false
						e.status = "Loading " + mapID + "..."
					}
				}
			} else if mx >= panelX+4 && mx < panelX+panelW-4 && my >= panelY+30 && my < panelY+panelH-40 {
				idx := (my-(panelY+30))/rowH + start
				if idx >= 0 && idx < len(e.availableMaps) {
					e.mapBrowserSel = idx
				}
			}
		}
	}

	// Assets browser panel (draw)
	if e.showAssetsBrowser {
		vw, vh := ebiten.WindowSize()
		panelX := vw/2 - 200
		panelY := vh/2 - 150
		panelW := 400
		panelH := 300
		ebitenutil.DrawRect(screen, float64(panelX), float64(panelY), float64(panelW), float64(panelH), color.NRGBA{20, 20, 30, 240})
		ebitenutil.DrawRect(screen, float64(panelX+2), float64(panelY+2), float64(panelW-4), 24, color.NRGBA{40, 40, 60, 255})
		text.Draw(screen, "Load Background Image", basicfont.Face7x13, panelX+10, panelY+18, color.White)

		// Close button
		closeX := panelX + panelW - 30
		closeY := panelY + 2
		ebitenutil.DrawRect(screen, float64(closeX), float64(closeY), 28, 20, color.NRGBA{100, 60, 60, 255})
		text.Draw(screen, "X", basicfont.Face7x13, closeX+10, closeY+15, color.White)

		rowH := 20
		maxRows := (panelH - 60) / rowH
		start := e.assetsBrowserScroll
		for i := 0; i < maxRows && start+i < len(e.availableAssets); i++ {
			yy := panelY + 30 + i*rowH
			if (start+i)%2 == 0 {
				ebitenutil.DrawRect(screen, float64(panelX+4), float64(yy-12), float64(panelW-8), 18, color.NRGBA{40, 40, 56, 255})
			}
			assetPath := e.availableAssets[start+i]
			show := filepath.Base(assetPath)
			var acol color.Color = color.White
			if e.assetsBrowserSel == start+i {
				acol = color.NRGBA{240, 196, 25, 255}
				ebitenutil.DrawRect(screen, float64(panelX+4), float64(yy-12), float64(panelW-8), 18, color.NRGBA{60, 60, 80, 255})
			}
			text.Draw(screen, show, basicfont.Face7x13, panelX+8, yy, acol)
		}

		// Load button
		loadBtnX := panelX + panelW/2 - 40
		loadBtnY := panelY + panelH - 30
		ebitenutil.DrawRect(screen, float64(loadBtnX), float64(loadBtnY), 80, 24, color.NRGBA{60, 90, 120, 255})
		text.Draw(screen, "Load", basicfont.Face7x13, loadBtnX+25, loadBtnY+16, color.White)

		// Handle assets browser interactions
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if mx >= closeX && mx < closeX+28 && my >= closeY && my < closeY+20 {
				e.showAssetsBrowser = false
			} else if mx >= loadBtnX && mx < loadBtnX+80 && my >= loadBtnY && my < loadBtnY+24 {
				if e.assetsBrowserSel >= 0 && e.assetsBrowserSel < len(e.availableAssets) {
					item := e.availableAssets[e.assetsBrowserSel]
					if item == ".." {
						// go up
						parent := filepath.Dir(e.assetsCurrentPath)
						e.assetsCurrentPath = parent
						e.refreshAssetsBrowser()
					} else if strings.HasPrefix(item, "[DIR] ") {
						// enter directory
						dirName := strings.TrimPrefix(item, "[DIR] ")
						newPath := filepath.Join(e.assetsCurrentPath, dirName)
						e.assetsCurrentPath = newPath
						e.refreshAssetsBrowser()
					} else {
						// load file
						fullPath := filepath.Join(e.assetsCurrentPath, item)
						if img, _, err := ebitenutil.NewImageFromFile(fullPath); err == nil {
							e.bg = img
							e.bgPath = fullPath
							e.status = fmt.Sprintf("BG loaded: %s", filepath.Base(fullPath))
							e.showAssetsBrowser = false
						} else {
							e.status = fmt.Sprintf("Failed to load image: %v", err)
						}
					}
				}
			} else if mx >= panelX+4 && mx < panelX+panelW-4 && my >= panelY+30 && my < panelY+panelH-40 {
				idx := (my-(panelY+30))/rowH + start
				if idx >= 0 && idx < len(e.availableAssets) {
					e.assetsBrowserSel = idx
				}
			}
		}
	}

	// Obstacles browser panel (draw)
	if e.showObstaclesBrowser {
		vw, vh := ebiten.WindowSize()
		panelX := vw/2 - 200
		panelY := vh/2 - 150
		panelW := 400
		panelH := 300
		ebitenutil.DrawRect(screen, float64(panelX), float64(panelY), float64(panelW), float64(panelH), color.NRGBA{20, 20, 30, 240})
		ebitenutil.DrawRect(screen, float64(panelX+2), float64(panelY+2), float64(panelW-4), 24, color.NRGBA{40, 40, 60, 255})
		text.Draw(screen, "Load Obstacle Image", basicfont.Face7x13, panelX+10, panelY+18, color.White)

		// Close button
		closeX := panelX + panelW - 30
		closeY := panelY + 2
		ebitenutil.DrawRect(screen, float64(closeX), float64(closeY), 28, 20, color.NRGBA{100, 60, 60, 255})
		text.Draw(screen, "X", basicfont.Face7x13, closeX+10, closeY+15, color.White)

		rowH := 20
		maxRows := (panelH - 60) / rowH
		start := e.obstaclesBrowserScroll
		for i := 0; i < maxRows && start+i < len(e.availableObstacles); i++ {
			yy := panelY + 30 + i*rowH
			if (start+i)%2 == 0 {
				ebitenutil.DrawRect(screen, float64(panelX+4), float64(yy-12), float64(panelW-8), 18, color.NRGBA{40, 40, 56, 255})
			}
			obstaclePath := e.availableObstacles[start+i]
			show := filepath.Base(obstaclePath)
			var acol color.Color = color.White
			if e.obstaclesBrowserSel == start+i {
				acol = color.NRGBA{240, 196, 25, 255}
				ebitenutil.DrawRect(screen, float64(panelX+4), float64(yy-12), float64(panelW-8), 18, color.NRGBA{60, 60, 80, 255})
			}
			text.Draw(screen, show, basicfont.Face7x13, panelX+8, yy, acol)
		}

		// Load button
		loadBtnX := panelX + panelW/2 - 40
		loadBtnY := panelY + panelH - 30
		ebitenutil.DrawRect(screen, float64(loadBtnX), float64(loadBtnY), 80, 24, color.NRGBA{60, 90, 120, 255})
		text.Draw(screen, "Load", basicfont.Face7x13, loadBtnX+25, loadBtnY+16, color.White)

		// Handle obstacles browser interactions
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if mx >= closeX && mx < closeX+28 && my >= closeY && my < closeY+20 {
				e.showObstaclesBrowser = false
			} else if mx >= loadBtnX && mx < loadBtnX+80 && my >= loadBtnY && my < loadBtnY+24 {
				if e.obstaclesBrowserSel >= 0 && e.obstaclesBrowserSel < len(e.availableObstacles) {
					item := e.availableObstacles[e.obstaclesBrowserSel]
					if item == ".." {
						// go up
						parent := filepath.Dir(e.obstaclesCurrentPath)
						e.obstaclesCurrentPath = parent
						e.refreshObstaclesBrowser()
					} else if strings.HasPrefix(item, "[DIR] ") {
						// enter directory
						dirName := strings.TrimPrefix(item, "[DIR] ")
						newPath := filepath.Join(e.obstaclesCurrentPath, dirName)
						e.obstaclesCurrentPath = newPath
						e.refreshObstaclesBrowser()
					} else {
						// load file as obstacle
						fullPath := filepath.Join(e.obstaclesCurrentPath, item)
						if _, _, err := ebitenutil.NewImageFromFile(fullPath); err == nil {
							if e.selKind == "obstacle" && e.selIndex >= 0 && e.selIndex < len(e.def.Obstacles) {
								// Change image of selected obstacle
								e.def.Obstacles[e.selIndex].Image = filepath.Base(fullPath)
								e.status = fmt.Sprintf("Obstacle image changed: %s", filepath.Base(fullPath))
							} else {
								// Create new obstacle at center of canvas
								nx, ny := 0.5, 0.5 // center
								obs := protocol.Obstacle{
									X:      nx - 0.05,
									Y:      ny - 0.05,
									Type:   "custom",
									Image:  filepath.Base(fullPath),
									Width:  0.1,
									Height: 0.1,
								}
								e.def.Obstacles = append(e.def.Obstacles, obs)
								e.selKind, e.selIndex, e.selHandle = "obstacle", len(e.def.Obstacles)-1, -1
								e.status = fmt.Sprintf("Obstacle added: %s", filepath.Base(fullPath))
							}
							e.showObstaclesBrowser = false
						} else {
							e.status = fmt.Sprintf("Failed to load obstacle image: %v", err)
						}
					}
				}
			} else if mx >= panelX+4 && mx < panelX+panelW-4 && my >= panelY+30 && my < panelY+panelH-40 {
				idx := (my-(panelY+30))/rowH + start
				if idx >= 0 && idx < len(e.availableObstacles) {
					e.obstaclesBrowserSel = idx
				}
			}
		}
	}

	// Help overlay
	if e.helpMode {
		vw, vh := ebiten.WindowSize()
		ebitenutil.DrawRect(screen, 0, 0, float64(vw), float64(vh), color.NRGBA{0, 0, 0, 180})

		helpText := []string{
			"Welcome to the Rumble Map Editor!",
			"",
			"TOOLS:",
			"  Deploy: Click to create deployment zones for units",
			"  Stone: Click to place meeting stones (rally points)",
			"  Mine: Click to place gold mines (resource points)",
			"  Lane: Click to start drawing unit movement paths",
			"  Obstacle: Click to place blocking objects",
			"",
			"CONTROLS:",
			"  Left Click: Select/create elements",
			"  Right Click: Finalize lane or clear selection",
			"  Drag: Move/resize selected elements",
			"  Delete: Remove selected element",
			"  Ctrl+S: Save map",
			"  G: Toggle grid overlay",
			"  D: Toggle lane direction (when lane selected)",
			"",
			"FEATURES:",
			"  Load Map: Browse and load existing maps",
			"  Assets: Browse background images",
			"  Help: Toggle this help screen",
			"",
			"Press H or click Help button to close",
		}

		y := 50
		for _, line := range helpText {
			text.Draw(screen, line, basicfont.Face7x13, 50, y, color.White)
			y += 16
		}
	}

	// Draw tooltips
	if e.showHelp && !e.helpMode {
		ebitenutil.DrawRect(screen, float64(e.tooltipX-4), float64(e.tooltipY-4), float64(len(e.helpText)*7+8), 20, color.NRGBA{40, 40, 60, 240})
		text.Draw(screen, e.helpText, basicfont.Face7x13, e.tooltipX, e.tooltipY+12, color.White)
	}

	// Draw chat window for status messages (bottom left)
	chatX := 8
	chatY := vh - 200
	chatW := 300
	chatH := 180

	// Background with border
	ebitenutil.DrawRect(screen, float64(chatX-2), float64(chatY-2), float64(chatW+4), float64(chatH+4), color.NRGBA{60, 60, 70, 255})
	ebitenutil.DrawRect(screen, float64(chatX), float64(chatY), float64(chatW), float64(chatH), color.NRGBA{20, 20, 30, 220})

	// Title bar
	ebitenutil.DrawRect(screen, float64(chatX), float64(chatY), float64(chatW), 20, color.NRGBA{40, 40, 60, 255})
	text.Draw(screen, "Status Log", basicfont.Face7x13, chatX+8, chatY+14, color.NRGBA{200, 200, 220, 255})

	// Draw status messages
	lineHeight := 14
	maxLines := (chatH - 25) / lineHeight
	startY := chatY + 25

	// Add current status to messages if it's new
	if e.status != "" && (len(e.statusMessages) == 0 || e.statusMessages[len(e.statusMessages)-1] != e.status) {
		e.statusMessages = append(e.statusMessages, e.status)
		if len(e.statusMessages) > e.maxStatusMessages {
			e.statusMessages = e.statusMessages[1:]
		}
		e.status = "" // Clear after adding to prevent duplicates
	}

	// Draw messages from bottom to top (most recent at bottom)
	for i := 0; i < maxLines && i < len(e.statusMessages); i++ {
		msgIndex := len(e.statusMessages) - 1 - i
		if msgIndex >= 0 {
			y := startY + (maxLines-1-i)*lineHeight
			text.Draw(screen, e.statusMessages[msgIndex], basicfont.Face7x13, chatX+8, y+12, color.NRGBA{180, 220, 180, 255})
		}
	}
}

func (e *editor) Layout(outsideWidth, outsideHeight int) (int, int) {
	// Match the window size so UI scales with resize
	return ebiten.WindowSize()
}

func main() {
	var wsURL, bg, mapID, flagToken string
	flag.StringVar(&wsURL, "ws", getenv("WAR_WS_URL", "ws://127.0.0.1:8080/ws"), "WebSocket URL")
	flag.StringVar(&bg, "bg", "", "Background image path (optional)")
	flag.StringVar(&mapID, "id", "", "Load existing map by ID (optional)")
	flag.StringVar(&flagToken, "token", "", "Bearer token for auth (optional)")
	flag.Parse()
	log.SetFlags(0)

	// Try to get token from various sources
	tok := strings.TrimSpace(flagToken)
	if tok == "" {
		tok = strings.TrimSpace(getenv("WAR_TOKEN", ""))
	}
	if tok == "" {
		tok = loadToken()
	}

	// Determine project root based on executable location
	exePath, err := os.Executable()
	projectRoot := "."
	if err == nil {
		exeDir := filepath.Dir(exePath)
		parentDir := filepath.Dir(exeDir)
		if filepath.Base(parentDir) == "cmd" {
			projectRoot = filepath.Dir(parentDir)
		} else {
			projectRoot = exeDir
		}
	}

	ed := &editor{
		showLogin:            tok == "", // Show login if no token
		assetsCurrentPath:    filepath.Join(projectRoot, "client", "internal", "game", "assets", "maps"),
		obstaclesCurrentPath: filepath.Join(projectRoot, "client", "internal", "game", "assets", "obstacles"),
		statusMessages:       []string{},
		maxStatusMessages:    10,
	}

	// If we have a token, try to connect immediately
	if tok != "" {
		c, err := dialWS(wsURL, tok)
		if err != nil {
			log.Printf("Failed to connect with existing token: %v", err)
			ed.showLogin = true // Fall back to login screen
		} else {
			ed.ws = c
			ed.inCh = make(chan wsMsg, 128)
			go ed.runReader()
		}
	}

	// Load initial background
	try := func(p string) {
		if ed.bg != nil || p == "" {
			return
		}
		if img, _, err := ebitenutil.NewImageFromFile(p); err == nil {
			ed.bg = img
			ed.bgPath = p
		}
	}
	try(bg)
	try(filepath.Join("assets", "maps", "default.png"))
	if ed.bg == nil { // fallback: empty image
		ed.bg = ebiten.NewImage(800, 600)
		ed.bg.Fill(color.NRGBA{0x22, 0x22, 0x33, 0xff})
	}

	// If we have a WebSocket connection and a map ID, load the map
	if ed.ws != nil && strings.TrimSpace(mapID) != "" {
		b, _ := json.Marshal(struct {
			Type string      `json:"type"`
			Data interface{} `json:"data"`
		}{Type: "GetMap", Data: protocol.GetMap{ID: mapID}})
		_ = ed.ws.WriteMessage(websocket.TextMessage, b)
	}

	ebiten.SetWindowTitle("Rumble Map Editor")
	ebiten.SetWindowSize(1200, 800)
	ebiten.SetWindowResizable(true)
	if err := ebiten.RunGame(ed); err != nil {
		log.Fatal(err)
	}
}
