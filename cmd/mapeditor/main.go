package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"log"
	"os"
	"path/filepath"
	"rumble/shared/protocol"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

type Layer int

const (
	LayerBG Layer = iota
	LayerFrame
	LayerDeploy
	LayerLanes
	LayerObstacles
	LayerAssets
	LayerBases
	LayerAll
)

// Full-featured editor struct
type editor struct {
	def  protocol.MapDef
	name string

	// Background
	bg     *ebiten.Image
	bgPath string

	// Current layer
	currentLayer  Layer
	showAllLayers bool

	// Selection
	selKind string

	// Status tracking
	status            string
	statusMessages    []string
	maxStatusMessages int

	// Browsers
	showMapBrowser    bool
	showAssetsBrowser bool
	availableMaps     []string
	availableAssets   []string
	mapBrowserSel     int
	assetsBrowserSel  int
	assetsCurrentPath string

	// Map name input
	nameFocus bool

	// Bases tracking
	playerBaseExists bool
	enemyBaseExists  bool

	// Mouse state for asset browser interaction
	assetsBrowserScroll int

	// Object manipulation state
	selectedDeployIndex int
	selectedObjectKind  string
	isDragging          bool
	isResizing          bool
	resizeHandle        int // 0: top-left, 1: top-right, 2: bottom-left, 3: bottom-right

	// Visual scaling for UI elements
	uiScale float64

	// Object images
	playerBaseImg *ebiten.Image
	enemyBaseImg  *ebiten.Image
	obstacleImg   *ebiten.Image
}

// generateID converts a display name to snake_case ID
func (e *editor) generateID(name string) string {
	if name == "" {
		return ""
	}

	// Simple snake_case conversion: lowercase and replace spaces with underscores
	result := strings.ToLower(name)
	result = strings.ReplaceAll(result, " ", "_")
	return result
}

func (e *editor) save() {
	if e.name == "" {
		e.status = "Set map name first"
		return
	}

	// Update the map definition with proper name and ID
	e.def.Name = e.name
	e.def.ID = e.generateID(e.name)

	os.MkdirAll("local_maps", 0755)
	filename := filepath.Join("local_maps", e.def.ID+".json")

	// Use pretty JSON formatting
	data, _ := json.MarshalIndent(e.def, "", "  ")
	os.WriteFile(filename, data, 0644)
	e.status = "Saved: " + filename + fmt.Sprintf(" (ID: %s)", e.def.ID)
}

func (e *editor) loadMap(filename string) {
	paths := []string{
		filename + ".json",
		filepath.Join("local_maps", filename+".json"),
		filepath.Join("server", "data", "maps", filename+".json"),
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		json.Unmarshal(data, &e.def)
		e.name = e.def.Name

		// Load background if specified
		if e.def.Bg != "" {
			path := filepath.Join("assets", "maps", e.def.Bg)
			img, _, err := ebitenutil.NewImageFromFile(path)
			if err == nil {
				e.bg = img
				e.bgPath = path
			}
		}

		e.status = "Loaded: " + filename
		e.playerBaseExists = e.def.PlayerBase.X != 0
		e.enemyBaseExists = e.def.EnemyBase.X != 0
		return
	}

	e.status = "Map not found: " + filename
}

func (e *editor) insertObject(layer Layer) {
	if e.bg == nil {
		e.status = "Load background first"
		return
	}

	// Get mouse position
	mx, my := ebiten.CursorPosition()
	vw, vh := ebiten.WindowSize()

	// Calculate map coordinates from mouse position
	imageW, imageH := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
	scaleX := float64(vw) / float64(imageW)
	scaleY := float64(vh-120) / float64(imageH)
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	offX := (vw - int(float64(imageW)*scale)) / 2
	offY := 80 + (vh-120-int(float64(imageH)*scale))/2

	// Calculate the actual viewport bounds (the in-game playable area)
	scaledViewportW := float64(protocol.ScreenW) * scale
	scaledViewportH := float64(protocol.ScreenH) * scale
	viewportX := float64(offX) + (float64(imageW)*scale-scaledViewportW)/2
	viewportY := float64(offY) + (float64(imageH)*scale-scaledViewportH)/2

	// Convert mouse position to normalized coordinates within the viewport
	mapX := (float64(mx) - viewportX) / scaledViewportW
	mapY := (float64(my) - viewportY) / scaledViewportH

	// Clamp coordinates to viewport bounds (0.0-1.0 is playable area)
	if mapX < 0 {
		mapX = 0
	} else if mapX > 1 {
		mapX = 1
	}
	if mapY < 0 {
		mapY = 0
	} else if mapY > 1 {
		mapY = 1
	}

	switch layer {
	case LayerDeploy:
		e.def.DeployZones = append(e.def.DeployZones, protocol.DeployZone{
			X: mapX, Y: mapY, W: 0.1, H: 0.1, Owner: "player"})
		e.status = fmt.Sprintf("Deploy zone added at (%.2f, %.2f)", mapX, mapY)
	case LayerObstacles:
		e.def.Obstacles = append(e.def.Obstacles, protocol.Obstacle{
			X: mapX, Y: mapY, Width: 0.05, Height: 0.05, Image: "tree.png"})
		e.status = fmt.Sprintf("Obstacle added at (%.2f, %.2f)", mapX, mapY)
	case LayerAssets:
		// Add decorative element (asset) at the calculated position
		element := protocol.DecorativeElement{
			X:      mapX,
			Y:      mapY,
			Width:  0.1,        // Default width
			Height: 0.1,        // Default height
			Image:  "tree.png", // Default image
			Layer:  1,          // Middle layer
		}
		e.def.DecorativeElements = append(e.def.DecorativeElements, element)
		e.status = fmt.Sprintf("Asset added at (%.2f, %.2f)", mapX, mapY)
	case LayerBases:
		if !e.playerBaseExists {
			e.def.PlayerBase = protocol.PointF{X: mapX, Y: mapY}
			e.playerBaseExists = true
			e.status = fmt.Sprintf("Player base added at (%.2f, %.2f)", mapX, mapY)
		} else if !e.enemyBaseExists {
			e.def.EnemyBase = protocol.PointF{X: mapX, Y: mapY}
			e.enemyBaseExists = true
			e.status = fmt.Sprintf("Enemy base added at (%.2f, %.2f)", mapX, mapY)
		} else {
			e.status = "Both bases exist - delete first"
		}
	}
}

func (e *editor) refreshAssetsBrowser() {
	// Look for images in client assets directory
	patterns := []string{
		"client/internal/game/assets/maps/*.png",
	}

	e.availableAssets = []string{}
	for _, pattern := range patterns {
		files, _ := filepath.Glob(pattern)
		for _, file := range files {
			e.availableAssets = append(e.availableAssets, filepath.Base(file))
		}
	}

	if len(e.availableAssets) == 0 {
		e.availableAssets = []string{"No assets found"}
	}
}

func (e *editor) Update() error {
	// Handle shortcuts
	for _, k := range inpututil.AppendJustPressedKeys(nil) {
		ctrl := ebiten.IsKeyPressed(ebiten.KeyControl)
		shift := ebiten.IsKeyPressed(ebiten.KeyShift)

		if k == ebiten.KeyS && ctrl {
			e.save()
		}
		if k == ebiten.KeyInsert {
			e.insertObject(e.currentLayer)
		}
		if k == ebiten.KeyDelete || (k == ebiten.KeyBackspace && shift) {
			e.deleteSelectedObject()
		}
		if k == ebiten.KeyEscape {
			e.selKind = ""
			e.showAssetsBrowser = false
			e.showMapBrowser = false
		}
		if k >= ebiten.Key1 && k <= ebiten.Key8 {
			index := int(k - ebiten.Key1)
			e.currentLayer = Layer(index)
			e.showAllLayers = (index == 7)
			layers := []string{"BG", "Frame", "Deploy", "Lanes", "Assets", "Obstacles", "Bases", "All"}
			e.status = "Layer: " + layers[index]
		}
	}

	// Handle mouse interactions
	mx, my := ebiten.CursorPosition()
	vw, vh := ebiten.WindowSize()

	// Handle asset browser interactions if it's open
	if e.showAssetsBrowser {
		e.updateAssetsBrowser(mx, my, vh)
		return nil
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Save button
		if mx >= 450 && mx < 550 && my >= 8 && my < 32 {
			e.save()
			return nil
		}

		// Background image button
		if mx >= 530 && mx < 630 && my >= 8 && my < 32 {
			e.showAssetsBrowser = !e.showAssetsBrowser
			e.refreshAssetsBrowser()
			return nil
		}

		// Load Map button
		if mx >= 650 && mx < 750 && my >= 8 && my < 32 {
			e.showMapBrowser = true
			e.refreshMapBrowser()
			return nil
		}

		// BG button (Set current background as map background)
		if mx >= 750 && mx < 850 && my >= 8 && my < 32 && e.bg != nil {
			e.setBackground()
			return nil
		}

		// Clear selection button
		if mx >= 380 && mx < 480 && my >= 8 && my < 32 {
			e.selKind = ""
			e.status = "Selection cleared"
			return nil
		}

		// Map name field
		if mx >= vw-300 && mx < vw-20 && my >= 8 && my < 32 {
			e.nameFocus = true
			return nil
		}

		// Layer buttons
		for i := 0; i < 8; i++ {
			x, y, w, h := 150, 8+i*35, 80, 28
			if mx >= x && mx < x+w && my >= y && my < y+h {
				e.currentLayer = Layer(i)
				e.showAllLayers = (i == 7)
				layers := []string{"BG", "Frame", "Deploy", "Lanes", "Obstacles", "Assets", "Bases", "All"}
				e.status = "Layer: " + layers[i]
				return nil
			}
		}

		// Clear focus
		if mx < 120 || mx > 780 {
			e.nameFocus = false
		}

		// Handle object selection on map
		if mx >= 120 && my >= 80 && e.bg != nil {
			e.handleMapSelection(mx, my, vw, vh)
		}
	}

	// Handle dragging for selected objects while mouse is held down
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) && e.selKind != "" && e.selectedDeployIndex >= 0 && mx >= 120 && my >= 80 {
		e.handleObjectDragging(mx, my)
	}

	// Handle mouse button releases
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		e.isDragging = false
	}

	// Handle text input
	if e.nameFocus {
		for _, r := range ebiten.AppendInputChars(nil) {
			if r >= 32 && r < 127 {
				e.name += string(r)
			}
		}
		for _, k := range inpututil.AppendJustPressedKeys(nil) {
			if k == ebiten.KeyBackspace && len(e.name) > 0 {
				e.name = e.name[:len(e.name)-1]
			}
			if k == ebiten.KeyEnter {
				e.nameFocus = false
			}
		}
	}

	return nil
}

func (e *editor) updateAssetsBrowser(mx, my, vh int) {
	vw, _ := ebiten.WindowSize()

	// Calculate asset browser panel position (same as drawAssetsBrowser)
	panelX := vw/2 - 200
	panelY := vh/2 - 150
	panelW := 400
	panelH := 300

	// Maximum visible rows (panel height - header - footer)
	maxRows := 12

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Close button bounds
		closeBtnX := panelX + panelW - 40
		closeBtnY := panelY + 2
		if mx >= closeBtnX && mx < closeBtnX+30 && my >= closeBtnY && my < closeBtnY+20 {
			e.showAssetsBrowser = false
			return
		}

		// Select button bounds
		selectBtnX := panelX + panelW/2 - 40
		selectBtnY := panelY + panelH - 35
		if mx >= selectBtnX && mx < selectBtnX+80 && my >= selectBtnY && my < selectBtnY+24 {
			if e.assetsBrowserSel >= 0 && e.assetsBrowserSel < len(e.availableAssets) {
				assetName := e.availableAssets[e.assetsBrowserSel]
				e.setAssetBackground(assetName)
			}
			return
		}

		// Click on asset items
		// Asset list starts after the title bar
		listStartY := panelY + 40
		itemHeight := 20

		// Check if click is within the asset list area
		if mx >= panelX+5 && mx < panelX+panelW-5 {
			for i := 0; i < maxRows; i++ {
				actualIndex := i + e.assetsBrowserScroll
				if actualIndex >= len(e.availableAssets) {
					break
				}

				itemY := listStartY + i*itemHeight - 10 // -10 to account for vertical padding
				if my >= itemY && my < itemY+18 {
					e.assetsBrowserSel = actualIndex
					e.status = fmt.Sprintf("Selected: %s", e.availableAssets[actualIndex])
					return
				}
			}
		}
	}

	// Handle mouse wheel scrolling
	_, wy := ebiten.Wheel()
	if wy != 0 {
		scrollAmount := int(wy) * 2 // Scroll 2 items at a time
		e.assetsBrowserScroll -= scrollAmount

		if e.assetsBrowserScroll < 0 {
			e.assetsBrowserScroll = 0
		}

		maxScroll := len(e.availableAssets) - maxRows
		if maxScroll < 0 {
			maxScroll = 0
		}
		if e.assetsBrowserScroll > maxScroll {
			e.assetsBrowserScroll = maxScroll
		}
	}
}

func (e *editor) setAssetBackground(assetName string) {
	// Construct the full path to load the asset
	fullPath := filepath.Join("client", "internal", "game", "assets", "maps", assetName)
	img, _, err := ebitenutil.NewImageFromFile(fullPath)
	if err != nil {
		// Fallback: try loading without directory structure
		img, _, err = ebitenutil.NewImageFromFile(assetName)
		if err != nil {
			e.status = fmt.Sprintf("Failed to load asset: %v", err)
			return
		}
		e.bgPath = assetName
	} else {
		e.bgPath = fullPath
	}

	e.bg = img
	e.status = fmt.Sprintf("Asset loaded: %s", assetName)
	e.showAssetsBrowser = false
}

func (e *editor) deleteSelectedObject() {
	switch e.selKind {
	case "deploy":
		if len(e.def.DeployZones) > 0 {
			e.def.DeployZones = e.def.DeployZones[:len(e.def.DeployZones)-1]
			e.selKind = ""
			e.status = "Deploy zone deleted"
		}
	case "obstacle":
		if len(e.def.Obstacles) > 0 {
			e.def.Obstacles = e.def.Obstacles[:len(e.def.Obstacles)-1]
			e.selKind = ""
			e.status = "Obstacle deleted"
		}
	case "base":
		if e.playerBaseExists {
			e.def.PlayerBase.X = 0
			e.def.PlayerBase.Y = 0
			e.playerBaseExists = false
			e.enemyBaseExists = false
			e.def.EnemyBase.X = 0
			e.def.EnemyBase.Y = 0
			e.selKind = ""
			e.status = "Bases deleted"
		}
	default:
		e.status = "No object selected to delete"
	}

	e.statusMessages = append(e.statusMessages, e.status)
	if len(e.statusMessages) > e.maxStatusMessages {
		e.statusMessages = e.statusMessages[1:]
	}
}

func (e *editor) handleMapSelection(mx, my, vw, vh int) {
	if e.bg == nil {
		return
	}

	imageW, imageH := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
	scaleX := float64(vw) / float64(imageW)
	scaleY := float64(vh-120) / float64(imageH)
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	offX := (vw - int(float64(imageW)*scale)) / 2
	offY := 80 + (vh-120-int(float64(imageH)*scale))/2

	// Clear previous selection
	e.selKind = ""
	e.selectedDeployIndex = -1

	// Convert mouse position to normalized map coordinates
	mapX := float64(mx-offX) / (float64(imageW) * scale)
	mapY := float64(my-offY) / (float64(imageH) * scale)

	// Check selection based on current layer
	if e.currentLayer == LayerDeploy || e.currentLayer == LayerAll {
		for i, zone := range e.def.DeployZones {
			// Check if click is within the zone rectangle
			if mapX >= zone.X && mapX <= zone.X+zone.W && mapY >= zone.Y && mapY <= zone.Y+zone.H {
				e.selKind = "deploy"
				e.selectedDeployIndex = i
				e.status = fmt.Sprintf("Selected Deploy Zone %d", i+1)
				return
			}
		}
	}

	if e.currentLayer == LayerObstacles || e.currentLayer == LayerAll {
		for i, obstacle := range e.def.Obstacles {
			// Calculate obstacle bounds (using default dimensions if not specified)
			width := obstacle.Width
			height := obstacle.Height
			if width == 0 {
				width = 0.05 // Default width
			}
			if height == 0 {
				height = 0.05 // Default height
			}

			// Check if click is within obstacle bounds
			if mapX >= obstacle.X && mapX <= obstacle.X+width && mapY >= obstacle.Y && mapY <= obstacle.Y+height {
				e.selKind = "obstacle"
				e.selectedDeployIndex = i
				e.status = fmt.Sprintf("Selected Obstacle %d", i+1)
				return
			}
		}
	}

	if e.currentLayer == LayerAssets || e.currentLayer == LayerAll {
		for i, element := range e.def.DecorativeElements {
			// Check if click is within the element bounds
			width := element.Width
			height := element.Height
			if width == 0 {
				width = 0.1 // Default width
			}
			if height == 0 {
				height = 0.1 // Default height
			}

			if mapX >= element.X && mapX <= element.X+width && mapY >= element.Y && mapY <= element.Y+height {
				e.selKind = "asset"
				e.selectedDeployIndex = i
				e.status = fmt.Sprintf("Selected Asset %d", i+1)
				return
			}
		}
	}

	if e.currentLayer == LayerBases || e.currentLayer == LayerAll {
		// Check player base first
		if e.playerBaseExists && e.def.PlayerBase.X > 0 {
			baseSize := 0.08 // Base size in normalized coordinates
			if mapX >= e.def.PlayerBase.X-baseSize/2 && mapX <= e.def.PlayerBase.X+baseSize/2 &&
				mapY >= e.def.PlayerBase.Y-baseSize/2 && mapY <= e.def.PlayerBase.Y+baseSize/2 {
				e.selKind = "base"
				e.selectedDeployIndex = 0
				e.status = "Selected Player Base"
				return
			}
		}

		// Check enemy base
		if e.enemyBaseExists && e.def.EnemyBase.X > 0 {
			baseSize := 0.08 // Base size in normalized coordinates
			if mapX >= e.def.EnemyBase.X-baseSize/2 && mapX <= e.def.EnemyBase.X+baseSize/2 &&
				mapY >= e.def.EnemyBase.Y-baseSize/2 && mapY <= e.def.EnemyBase.Y+baseSize/2 {
				e.selKind = "base"
				e.selectedDeployIndex = 1
				e.status = "Selected Enemy Base"
				return
			}
		}
	}
}

func (e *editor) handleObjectDragging(mx, my int) {
	if e.bg == nil {
		return
	}

	// Only drag if we have a selected object
	if e.selKind == "" || e.selectedDeployIndex == -1 {
		return
	}

	vw, vh := ebiten.WindowSize()
	imageW, imageH := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
	scaleX := float64(vw) / float64(imageW)
	scaleY := float64(vh-120) / float64(imageH)
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	offX := (vw - int(float64(imageW)*scale)) / 2
	offY := 80 + (vh-120-int(float64(imageH)*scale))/2

	// Convert mouse position to normalized map coordinates
	mapX := float64(mx-offX) / (float64(imageW) * scale)
	mapY := float64(my-offY) / (float64(imageH) * scale)

	// Clamp coordinates to map bounds
	if mapX < 0 {
		mapX = 0
	} else if mapX > 1 {
		mapX = 1
	}
	if mapY < 0 {
		mapY = 0
	} else if mapY > 1 {
		mapY = 1
	}

	// Update object position based on type
	switch e.selKind {
	case "deploy":
		if e.selectedDeployIndex >= 0 && e.selectedDeployIndex < len(e.def.DeployZones) {
			zone := &e.def.DeployZones[e.selectedDeployIndex]
			zone.X = mapX - zone.W/2 // Center the zone on mouse position
			zone.Y = mapY - zone.H/2

			// Clamp deploy zone to stay within map bounds
			if zone.X < 0 {
				zone.X = 0
			} else if zone.X+zone.W > 1 {
				zone.X = 1 - zone.W
			}
			if zone.Y < 0 {
				zone.Y = 0
			} else if zone.Y+zone.H > 1 {
				zone.Y = 1 - zone.H
			}
		}
	case "obstacle":
		if e.selectedDeployIndex >= 0 && e.selectedDeployIndex < len(e.def.Obstacles) {
			obstacle := &e.def.Obstacles[e.selectedDeployIndex]
			width := obstacle.Width
			height := obstacle.Height
			if width == 0 {
				width = 0.05
			}
			if height == 0 {
				height = 0.05
			}

			obstacle.X = mapX - width/2 // Center obstacle on mouse position
			obstacle.Y = mapY - height/2

			// Clamp obstacle to stay within map bounds
			if obstacle.X < 0 {
				obstacle.X = 0
			} else if obstacle.X+width > 1 {
				obstacle.X = 1 - width
			}
			if obstacle.Y < 0 {
				obstacle.Y = 0
			} else if obstacle.Y+height > 1 {
				obstacle.Y = 1 - height
			}
		}
	case "base":
		baseSize := 0.08 // Base size in normalized coordinates
		newX := mapX
		newY := mapY

		// Clamp base to stay within map bounds
		if newX-baseSize/2 < 0 {
			newX = baseSize / 2
		} else if newX+baseSize/2 > 1 {
			newX = 1 - baseSize/2
		}
		if newY-baseSize/2 < 0 {
			newY = baseSize / 2
		} else if newY+baseSize/2 > 1 {
			newY = 1 - baseSize/2
		}

		if e.selectedDeployIndex == 0 && e.playerBaseExists {
			// Update player base position
			e.def.PlayerBase.X = newX
			e.def.PlayerBase.Y = newY
		} else if e.selectedDeployIndex == 1 && e.enemyBaseExists {
			// Update enemy base position
			e.def.EnemyBase.X = newX
			e.def.EnemyBase.Y = newY
		}
	}
}

func (e *editor) drawDeployZones(screen *ebiten.Image, vw, vh int) {
	if e.bg == nil {
		return
	}

	imageW, imageH := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
	scaleX := float64(vw) / float64(imageW)
	scaleY := float64(vh-120) / float64(imageH)
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	offX := (vw - int(float64(imageW)*scale)) / 2
	offY := 80 + (vh-120-int(float64(imageH)*scale))/2

	for i, zone := range e.def.DeployZones {
		// Convert normalized coordinates to screen coordinates
		x := float64(offX) + zone.X*float64(imageW)*scale
		y := float64(offY) + zone.Y*float64(imageH)*scale
		w := zone.W * float64(imageW) * scale
		h := zone.H * float64(imageH) * scale

		// Draw normal green rectangle (slightly brighter if selected)
		green := color.RGBA{0, 200, 0, 128}
		if e.selKind == "deploy" && e.selectedDeployIndex == i {
			green = color.RGBA{0, 255, 0, 160} // Brighter when selected
			// Subtle white border for selected objects
			ebitenutil.DrawRect(screen, x-1, y-1, w+2, 1, color.RGBA{255, 255, 255, 200}) // Top
			ebitenutil.DrawRect(screen, x-1, y+h, w+2, 1, color.RGBA{255, 255, 255, 200}) // Bottom
			ebitenutil.DrawRect(screen, x-1, y-1, 1, h+2, color.RGBA{255, 255, 255, 200}) // Left
			ebitenutil.DrawRect(screen, x+w, y-1, 1, h+2, color.RGBA{255, 255, 255, 200}) // Right
		}
		ebitenutil.DrawRect(screen, x, y, w, h, green)

		// Draw label for deploy zone
		label := fmt.Sprintf("Deploy %d", i+1)
		labelX := int(x + w/2 - float64(len(label)*7)/2)
		labelY := int(y + h + 15)
		ebitenutil.DrawRect(screen, float64(labelX-2), float64(labelY-10), float64(len(label)*7+4), 14, color.RGBA{0, 0, 0, 128})
		text.Draw(screen, label, basicfont.Face7x13, labelX, labelY, color.White)
	}
}

func (e *editor) drawObstacles(screen *ebiten.Image, vw, vh int) {
	if e.bg == nil {
		return
	}

	imageW, imageH := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
	scaleX := float64(vw) / float64(imageW)
	scaleY := float64(vh-120) / float64(imageH)
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	offX := (vw - int(float64(imageW)*scale)) / 2
	offY := 80 + (vh-120-int(float64(imageH)*scale))/2

	// Load obstacle image if not loaded
	if e.obstacleImg == nil {
		path := filepath.Join("client", "internal", "game", "assets", "obstacles", "tree.png")
		img, _, err := ebitenutil.NewImageFromFile(path)
		if err == nil {
			e.obstacleImg = img
		}
	}

	for i, obstacle := range e.def.Obstacles {
		x := float64(offX) + obstacle.X*float64(imageW)*scale
		y := float64(offY) + obstacle.Y*float64(imageH)*scale

		// Draw obstacle as image or rectangle if image not loaded
		if e.obstacleImg != nil {
			w := int(obstacle.Width * float64(imageW) * scale)
			h := int(obstacle.Height * float64(imageH) * scale)
			if w == 0 {
				w = 32
			}
			if h == 0 {
				h = 32
			}

			// Scale the obstacle image appropriately
			scale := float64(w) / float64(e.obstacleImg.Bounds().Dx())
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(scale, scale)
			op.GeoM.Translate(float64(int(x)), float64(int(y)))
			screen.DrawImage(e.obstacleImg, op)

		}

		// Draw label for obstacle
		label := fmt.Sprintf("Obstacle %d", i+1)
		labelX := int(x + 16) // Center on 32px wide image
		labelY := int(y - 5)
		ebitenutil.DrawRect(screen, float64(labelX-len(label)*7/2-2), float64(labelY-10), float64(len(label)*7+4), 14, color.RGBA{0, 0, 0, 128})
		text.Draw(screen, label, basicfont.Face7x13, labelX-len(label)*7/2, labelY, color.RGBA{255, 255, 0, 255})
	}
}

func (e *editor) drawBases(screen *ebiten.Image, vw, vh int) {
	if e.bg == nil {
		return
	}

	imageW, imageH := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
	scaleX := float64(vw) / float64(imageW)
	scaleY := float64(vh-120) / float64(imageH)
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY
	}

	offX := (vw - int(float64(imageW)*scale)) / 2
	offY := 80 + (vh-120-int(float64(imageH)*scale))/2

	// Load base images if not loaded
	if e.playerBaseImg == nil {
		playerPath := filepath.Join("client", "internal", "game", "assets", "ui", "base.png")
		playerImg, _, err := ebitenutil.NewImageFromFile(playerPath)
		if err == nil {
			e.playerBaseImg = playerImg
		}
	}

	if e.enemyBaseImg == nil && e.playerBaseImg != nil {
		// For now, use the same player base image for enemy
		e.enemyBaseImg = e.playerBaseImg
	}

	// Draw player base
	if e.playerBaseExists && e.def.PlayerBase.X > 0 {
		x := float64(offX) + e.def.PlayerBase.X*float64(imageW)*scale
		y := float64(offY) + e.def.PlayerBase.Y*float64(imageH)*scale

		if e.playerBaseImg != nil {
			bounds := e.playerBaseImg.Bounds()
			baseWidth := bounds.Dx()
			baseSize := 64.0 // Desired size
			baseScale := baseSize / float64(baseWidth)

			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(baseScale, baseScale)
			op.GeoM.Translate(float64(int(x-baseSize/2)), float64(int(y-baseSize/2)))
			screen.DrawImage(e.playerBaseImg, op)

			// Label for player base
			label := "Player Base"
			labelX := int(x)
			labelY := int(y - baseSize/2 - 5)
			ebitenutil.DrawRect(screen, float64(labelX-len(label)*7/2-2), float64(labelY-10), float64(len(label)*7+4), 14, color.RGBA{0, 0, 0, 128})
			text.Draw(screen, label, basicfont.Face7x13, labelX-len(label)*7/2, labelY, color.RGBA{0, 255, 255, 255})
		}
	}

	// Draw enemy base
	if e.enemyBaseExists && e.def.EnemyBase.X > 0 {
		x := float64(offX) + e.def.EnemyBase.X*float64(imageW)*scale
		y := float64(offY) + e.def.EnemyBase.Y*float64(imageH)*scale

		if e.enemyBaseImg != nil {
			bounds := e.enemyBaseImg.Bounds()
			baseWidth := bounds.Dx()
			baseSize := 64.0 // Desired size
			baseScale := baseSize / float64(baseWidth)

			// Add slight red tint for enemy base
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(baseScale, baseScale)
			op.GeoM.Translate(float64(int(x-baseSize/2)), float64(int(y-baseSize/2)))
			op.ColorM.Scale(1.0, 0.7, 0.7, 1.0) // Red tint
			screen.DrawImage(e.enemyBaseImg, op)

			// Label for enemy base
			label := "Enemy Base"
			labelX := int(x)
			labelY := int(y - baseSize/2 - 5)
			ebitenutil.DrawRect(screen, float64(labelX-len(label)*7/2-2), float64(labelY-10), float64(len(label)*7+4), 14, color.RGBA{0, 0, 0, 128})
			text.Draw(screen, label, basicfont.Face7x13, labelX-len(label)*7/2, labelY, color.RGBA{255, 0, 0, 255})
		}
	}
}

func (e *editor) setBackground() {
	if e.bgPath == "" {
		e.status = "No background loaded"
		return
	}

	e.def.Bg = filepath.Base(e.bgPath)
	e.status = "Background set for map: " + e.def.Bg
}

func (e *editor) refreshMapBrowser() {
	// Get all JSON files from local_maps
	files, _ := filepath.Glob("local_maps/*.json")
	for _, file := range files {
		e.availableMaps = append(e.availableMaps, strings.TrimSuffix(filepath.Base(file), ".json"))
	}
}

func (e *editor) Draw(screen *ebiten.Image) {
	vw, vh := ebiten.WindowSize()

	// Top UI bar
	uiColor := color.RGBA{40, 40, 64, 255}
	ebitenutil.DrawRect(screen, 0, 0, float64(vw), 80, uiColor)

	// Draw background if available
	if e.bg != nil {
		imageW, imageH := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
		scaleX := float64(vw) / float64(imageW)
		scaleY := float64(vh-120) / float64(imageH)
		scale := scaleX
		if scaleY < scaleX {
			scale = scaleY
		}

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(scale, scale)

		offX := (vw - int(float64(imageW)*scale)) / 2
		offY := 80 + (vh-120-int(float64(imageH)*scale))/2
		op.GeoM.Translate(float64(offX), float64(offY))

		screen.DrawImage(e.bg, op)

		// Draw viewport border (blue frame showing in-game viewing area)
		scaledViewportW := int(float64(protocol.ScreenW) * scale)
		scaledViewportH := int(float64(protocol.ScreenH) * scale)

		viewportX := offX + (int(float64(imageW)*scale)-scaledViewportW)/2
		viewportY := offY + (int(float64(imageH)*scale)-scaledViewportH)/2

		borderThickness := 3
		blueColor := color.RGBA{0, 100, 255, 128}
		ebitenutil.DrawRect(screen, float64(viewportX), float64(viewportY), float64(scaledViewportW), float64(borderThickness), blueColor)
		ebitenutil.DrawRect(screen, float64(viewportX), float64(viewportY+scaledViewportH-borderThickness), float64(scaledViewportW), float64(borderThickness), blueColor)
		ebitenutil.DrawRect(screen, float64(viewportX), float64(viewportY), float64(borderThickness), float64(scaledViewportH), blueColor)
		ebitenutil.DrawRect(screen, float64(viewportX+scaledViewportW-borderThickness), float64(viewportY), float64(borderThickness), float64(scaledViewportH), blueColor)

		// Label
		label := "IN-GAME CAMERA VIEWPORT"
		labelW := len(label) * 7
		labelX := viewportX + (scaledViewportW-labelW)/2
		labelY := viewportY - 25
		ebitenutil.DrawRect(screen, float64(labelX-4), float64(labelY-2), float64(labelW+8), 16, color.RGBA{0, 0, 0, 160})
		text.Draw(screen, label, basicfont.Face7x13, labelX, labelY+12, color.RGBA{0, 100, 255, 255})
	}

	// Draw deploy zones as green rectangles (visible on Deploy layer)
	if e.currentLayer == LayerDeploy || e.currentLayer == LayerAll {
		e.drawDeployZones(screen, vw, vh)
	}

	// Draw obstacles as images (visible on Assets/Obstacles layer)
	if e.currentLayer == LayerObstacles || e.currentLayer == LayerAll {
		e.drawObstacles(screen, vw, vh)
	}

	// Draw bases as images (visible on Bases layer or All)
	if e.currentLayer == LayerBases || e.currentLayer == LayerAll {
		e.drawBases(screen, vw, vh)
	}

	// Layer buttons (left side)
	layerNames := []string{"BG", "Frame", "Deploy", "Lanes", "Obstacles", "Assets", "Bases", "All"}
	for i, name := range layerNames {
		x, y, w, h := 150, 8+i*35, 80, 28

		buttonColor := color.RGBA{42, 42, 58, 255}
		if e.currentLayer == Layer(i) {
			buttonColor = color.RGBA{74, 158, 255, 255}
		}

		// Draw button
		borderColor := color.RGBA{90, 90, 106, 255}
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), 1, borderColor)
		ebitenutil.DrawRect(screen, float64(x), float64(y+h-1), float64(w), 1, borderColor)
		ebitenutil.DrawRect(screen, float64(x), float64(y), 1, float64(h), borderColor)
		ebitenutil.DrawRect(screen, float64(x+w-1), float64(y), 1, float64(h), borderColor)
		ebitenutil.DrawRect(screen, float64(x+1), float64(y+1), float64(w-2), float64(h-2), buttonColor)

		// Text
		textColor := color.White
		text.Draw(screen, name, basicfont.Face7x13, x+(w-len(name)*7)/2, y+20, textColor)
	}

	// Top buttons
	buttonY := 8
	buttonH := 24

	// Save button
	saveColor := color.RGBA{112, 180, 112, 255}
	ebitenutil.DrawRect(screen, 450, float64(buttonY), 60, float64(buttonH), saveColor)
	text.Draw(screen, "Save", basicfont.Face7x13, 465, 23, color.White)

	// Background button
	assetsColor := color.RGBA{144, 112, 144, 255}
	ebitenutil.DrawRect(screen, 530, float64(buttonY), 80, float64(buttonH), assetsColor)
	text.Draw(screen, "Load BG", basicfont.Face7x13, 545, 23, color.White)

	// Load button
	loadColor := color.RGBA{112, 144, 176, 255}
	ebitenutil.DrawRect(screen, 650, float64(buttonY), 80, float64(buttonH), loadColor)
	text.Draw(screen, "Load Map", basicfont.Face7x13, 660, 23, color.White)

	// BG button (Set current background as map background)
	if e.bg != nil {
		bgColor := color.RGBA{176, 144, 112, 255}
		ebitenutil.DrawRect(screen, 750, float64(buttonY), 60, float64(buttonH), bgColor)
		text.Draw(screen, "Set BG", basicfont.Face7x13, 760, 23, color.White)
	}

	// Clear Selection button
	clearColor := color.RGBA{112, 112, 160, 255}
	ebitenutil.DrawRect(screen, 380, float64(buttonY), 60, float64(buttonH), clearColor)
	text.Draw(screen, "Clear", basicfont.Face7x13, 390, 23, color.White)

	// Map name field
	mapNameX := vw - 300
	fieldColor := color.RGBA{64, 64, 80, 255}
	if e.nameFocus {
		fieldColor = color.RGBA{96, 96, 128, 255}
	}
	ebitenutil.DrawRect(screen, float64(mapNameX), float64(buttonY), 280, float64(buttonH), fieldColor)

	displayName := e.name
	if displayName == "" {
		displayName = "Enter map name..."
	}
	text.Draw(screen, fmt.Sprintf("Map: %s", displayName), basicfont.Face7x13, mapNameX+6, 23, color.White)
	if e.nameFocus {
		cursorX := mapNameX + 6 + len(fmt.Sprintf("Map: %s", displayName))*7
		text.Draw(screen, "|", basicfont.Face7x13, cursorX, 23, color.White)
	}

	// Status message
	if e.status != "" {
		text.Draw(screen, e.status, basicfont.Face7x13, 8, 75, color.RGBA{180, 220, 180, 255})
	}

	// Assets browser
	if e.showAssetsBrowser {
		e.drawAssetsBrowser(screen, vw, vh)
	}
}

func (e *editor) drawAssetsBrowser(screen *ebiten.Image, vw, vh int) {
	// Simple browser panel
	panelX, panelY := vw/2-200, vh/2-150
	panelW, panelH := 400, 300

	ebitenutil.DrawRect(screen, float64(panelX), float64(panelY), float64(panelW), float64(panelH), color.RGBA{255, 255, 255, 136})
	ebitenutil.DrawRect(screen, float64(panelX+2), float64(panelY+2), float64(panelW-4), 24, color.RGBA{64, 80, 80, 255})
	text.Draw(screen, "Assets Browser", basicfont.Face7x13, panelX+10, panelY+18, color.RGBA{255, 255, 0, 255})

	// Close button
	ebitenutil.DrawRect(screen, float64(panelX+panelW-40), float64(panelY+2), 30, 20, color.RGBA{100, 63, 63, 255})
	text.Draw(screen, "X", basicfont.Face7x13, panelX+panelW-25, panelY+17, color.RGBA{255, 255, 255, 255})

	// List assets with scrolling
	y := panelY + 40
	itemHeight := 20
	maxVisibleRows := 12

	for i := 0; i < maxVisibleRows; i++ {
		actualIndex := i + e.assetsBrowserScroll
		if actualIndex >= len(e.availableAssets) {
			break
		}

		asset := e.availableAssets[actualIndex]
		col := color.RGBA{32, 32, 40, 255}
		if actualIndex == e.assetsBrowserSel {
			col = color.RGBA{64, 64, 80, 255}
		}

		ebitenutil.DrawRect(screen, float64(panelX+5), float64(y-10), float64(panelW-10), 18, col)
		text.Draw(screen, asset, basicfont.Face7x13, panelX+10, y+3, color.RGBA{255, 255, 255, 255})
		y += itemHeight
	}

	// Load button
	loadBtnY := panelY + panelH - 35
	ebitenutil.DrawRect(screen, float64(panelX+panelW/2-40), float64(loadBtnY), 80, 24, color.RGBA{96, 144, 96, 255})
	text.Draw(screen, "Select", basicfont.Face7x13, panelX+panelW/2-30, loadBtnY+16, color.RGBA{255, 255, 255, 255})
}

func (e *editor) Layout(outsideWidth, outsideHeight int) (int, int) {
	return ebiten.WindowSize()
}

func main() {
	log.SetFlags(0)

	ed := &editor{
		name:   "New Map",
		status: "Map Editor Ready",
	}

	ebiten.SetWindowTitle("Rumble Map Editor")
	ebiten.SetWindowSize(1200, 800)
	ebiten.SetWindowResizable(true)

	if err := ebiten.RunGame(ed); err != nil {
		log.Fatal(err)
	}
}
