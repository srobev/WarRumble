package main

import (
	"encoding/json"
	"rumble/shared/protocol"

	"github.com/gorilla/websocket"
	"github.com/hajimehoshi/ebiten/v2"
)

// wsMsg represents WebSocket messages
type wsMsg struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// editor represents the main map editor state and UI
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

	// scrollbars
	scrollX    int
	scrollY    int
	scrollStep int

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

	// save dialog and enhanced map naming
	showSaveDialog bool
	saveDialogY    int
	nameInputWidth int
}
