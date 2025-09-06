package main

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
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
	tool     int // 0 deploy, 1 stone, 2 mine, 3 lane, 4 obstacle, 5 decorative
	name     string
	status   string
	savePath string

	// layer system
	currentLayer  int  // 0=background, 1=frame, 2=deploy, 3=lanes, 4=obstacles, 5=decorative, 6=bases
	showAllLayers bool // toggle to show all layers simultaneously

	// base management
	playerBaseExists bool
	enemyBaseExists  bool

	// selection & editing
	selKind   string // ""|"deploy"|"stone"|"mine"|"lane"|"obstacle"|"decorative"|"frame"
	selIndex  int
	selHandle int // for deploy corners: 0=TL,1=TR,2=BR,3=BL, -1=body; for obstacles: 0=TL,1=TR,2=BR,3=BL, -1=body
	dragging  bool
	lastMx    int
	lastMy    int

	// camera system for map scrolling and zooming
	cameraX          float64
	cameraY          float64
	cameraZoom       float64
	cameraMinZoom    float64
	cameraMaxZoom    float64
	cameraDragging   bool
	cameraDragStartX int
	cameraDragStartY int

	// extended camera space visualization
	showExtendedCameraSpace bool

	// configurable map frame
	frameX          float64 // normalized position (0-1)
	frameY          float64
	frameWidth      float64 // normalized width (0-1)
	frameHeight     float64 // normalized height (0-1)
	frameScale      float64 // scale factor (for backward compatibility)
	frameDragging   bool
	frameDragStartX int
	frameDragStartY int
	frameDragHandle int // 0=center, 1=top-left, 2=top-right, 3=bottom-right, 4=bottom-left

	// bg management
	showGrid  bool
	bgInput   string
	bgFocus   bool
	nameFocus bool

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

	// decorative elements browser
	showDecorativeBrowser   bool
	availableDecorative     []string
	decorativeBrowserSel    int
	decorativeBrowserScroll int
	decorativeCurrentPath   string

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
	statusMessages      []string
	maxStatusMessages   int
	showStatusLog       bool
	statusLogX          int
	statusLogY          int
	statusLogDragging   bool
	statusLogDragStartX int
	statusLogDragStartY int
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

func (e *editor) save() {
	nm := strings.TrimSpace(e.name)
	if nm == "" {
		nm = "New Map"
	}
	e.def.Name = nm
	if strings.TrimSpace(e.def.ID) == "" {
		e.def.ID = strings.ReplaceAll(strings.ToLower(nm), " ", "-")
	}

	// Save frame boundaries to the map definition
	e.def.FrameX = e.frameX
	e.def.FrameY = e.frameY
	e.def.FrameWidth = e.frameWidth
	e.def.FrameHeight = e.frameHeight

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
