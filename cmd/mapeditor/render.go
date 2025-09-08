package main

import (
	"fmt"
	"image/color"
	"path/filepath"
	"rumble/shared/protocol"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

// renderLoginScreen handles drawing the login interface
func (e *editor) renderLoginScreen(screen *ebiten.Image) {
	vw, vh := ebiten.WindowSize()

	// Background
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
}

// renderBackground handles drawing the background image and canvas setup
func (e *editor) renderBackground(screen *ebiten.Image) {
	if e.bg == nil {
		return
	}

	vw, vh := ebiten.WindowSize()
	vh -= 120 // Account for UI height

	// Game battle area dimensions (exact same as game client)
	gameBattleTopUI := 50
	gameBattleBottomUI := 160
	gameBattleAvailableHeight := protocol.ScreenH - gameBattleTopUI - gameBattleBottomUI
	gameBattleAvailableWidth := protocol.ScreenW

	// Calculate scale exactly like the game client - fit to width
	scaleX := float64(gameBattleAvailableWidth) / float64(e.bg.Bounds().Dx())
	scaleY := float64(gameBattleAvailableHeight) / float64(e.bg.Bounds().Dy())
	scale := scaleX
	if scaleY < scaleX {
		scale = scaleY // Use smaller scale to maintain aspect ratio
	}

	// Calculate display dimensions exactly like game
	dispW := int(float64(e.bg.Bounds().Dx()) * scale)
	dispH := int(float64(e.bg.Bounds().Dy()) * scale)

	// Game-style positioning: center in window
	offX := (vw - dispW) / 2
	offY := 120 + (vh-dispH)/2

	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale) // No extra zoom overlay
	op.GeoM.Translate(float64(offX), float64(offY))
	screen.DrawImage(e.bg, op)

	// Draw border frame showing the actual map boundaries (0-1 area)
	borderThickness := 6
	borderOffY := offY

	// Enable frame manipulation handles when Frame layer is active
	if e.currentLayer == 1 {
		// Apply configurable frame position and scale
		const topUIH = 120
		sw, sh := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
		vh -= topUIH
		sx := float64(vw) / float64(sw)
		sy := float64(vh) / float64(sh)
		s := sx
		if sy < sx {
			s = sy
		}
		extendedW := int(float64(sw) * s * 1.2)
		extendedH := int(float64(sh) * s * 1.2)
		borderOffX := (vw - extendedW) / 2
		borderOffY = topUIH + (vh-extendedH)/2
		mapBorderW := int(float64(extendedW) / 1.2)
		mapBorderH := int(float64(extendedH) / 1.2)
		borderOffX += (extendedW - mapBorderW) / 2
		borderOffY += (extendedH - mapBorderH) / 2

		frameBorderW := int(float64(mapBorderW) * e.frameScale)
		frameBorderH := int(float64(mapBorderH) * e.frameScale)
		frameOffX := borderOffX + int(float64(mapBorderW)*(e.frameX-0.5)*e.frameScale) + (mapBorderW-frameBorderW)/2
		frameOffY := borderOffY + int(float64(mapBorderH)*(e.frameY-0.5)*e.frameScale) + (mapBorderH-frameBorderH)/2

		// Draw border frame with thick lines
		borderColor := color.NRGBA{255, 255, 255, 255}

		// Top border
		ebitenutil.DrawRect(screen, float64(frameOffX), float64(frameOffY), float64(frameBorderW), float64(borderThickness), borderColor)
		// Bottom border
		ebitenutil.DrawRect(screen, float64(frameOffX), float64(frameOffY+frameBorderH-borderThickness), float64(frameBorderW), float64(borderThickness), borderColor)
		// Left border
		ebitenutil.DrawRect(screen, float64(frameOffX), float64(frameOffY), float64(borderThickness), float64(frameBorderH), borderColor)
		// Right border
		ebitenutil.DrawRect(screen, float64(frameOffX+frameBorderW-borderThickness), float64(frameOffY), float64(borderThickness), float64(frameBorderH), borderColor)

		// Draw frame manipulation handles only when not dragging and Frame layer is active
		if !e.frameDragging {
			handleSize := 8
			handleColor := color.NRGBA{100, 200, 255, 255}
			// Corner handles for scaling
			corners := [][2]int{
				{frameOffX - handleSize/2, frameOffY - handleSize/2},                               // Top-left
				{frameOffX + frameBorderW - handleSize/2, frameOffY - handleSize/2},                // Top-right
				{frameOffX + frameBorderW - handleSize/2, frameOffY + frameBorderH - handleSize/2}, // Bottom-right
				{frameOffX - handleSize/2, frameOffY + frameBorderH - handleSize/2},                // Bottom-left
			}
			for _, corner := range corners {
				ebitenutil.DrawRect(screen, float64(corner[0]), float64(corner[1]), float64(handleSize), float64(handleSize), handleColor)
			}
			// Center handle for moving
			centerX := frameOffX + frameBorderW/2 - handleSize/2
			centerY := frameOffY + frameBorderH/2 - handleSize/2
			ebitenutil.DrawRect(screen, float64(centerX), float64(centerY), float64(handleSize), float64(handleSize), color.NRGBA{255, 200, 100, 255})
		}
	}
}

// renderMapElements handles drawing all map objects based on current layer visibility
func (e *editor) renderMapElements(screen *ebiten.Image) {
	if e.bg == nil {
		return
	}

	// Use consistent coordinate transformation matching Draw function
	toX := func(nx float64) int { return e.getCoordinateTransformation(nx, 0).x }
	toY := func(ny float64) int { return e.getCoordinateTransformation(0, ny).y }

	// Layer-based visibility
	showDeploy := e.showAllLayers || e.currentLayer == 2 || e.currentLayer == 7
	showBG := e.showAllLayers || e.currentLayer == 0 || e.currentLayer == 7
	showLanes := e.showAllLayers || e.currentLayer == 3 || e.currentLayer == 7
	showObstacles := e.showAllLayers || e.currentLayer == 4 || e.currentLayer == 7
	showDecorative := e.showAllLayers || e.currentLayer == 5 || e.currentLayer == 7
	showBases := e.showAllLayers || e.currentLayer == 6 || e.currentLayer == 7

	// Draw deploy zones
	if showDeploy {
		e.renderDeployZones(screen, toX, toY)
	}

	// Draw background elements (meeting stones and gold mines)
	if showBG {
		e.renderBackgroundElements(screen, toX, toY)
	}

	// Draw lanes
	if showLanes {
		e.renderLanes(screen, toX, toY)
	}

	// Draw obstacles
	if showObstacles {
		e.renderObstacles(screen, toX, toY)
	}

	// Draw decorative elements
	if showDecorative {
		e.renderDecorativeElements(screen, toX, toY)
	}

	// Draw player and enemy bases
	if showBases {
		e.renderBases(screen, toX, toY)
	}
}

// renderUI handles drawing the main UI elements
func (e *editor) renderUI(screen *ebiten.Image) {
	vw, _ := ebiten.WindowSize()

	// Top UI bar background to prevent overlap with canvas - optimized single draw
	ebitenutil.DrawRect(screen, 0, 0, float64(vw), 120, color.NRGBA{28, 28, 40, 255})

	// Layer selection buttons
	e.renderLayerButtons(screen)

	// Map name input field
	e.renderMapNameInput(screen)

	// Main toolbar buttons (Save, Clear, Assets, Load Map, Help)
	e.renderToolbarButtons(screen)

	// Background info display
	e.renderBackgroundInfo(screen)

	// Grid overlay if enabled
	if e.showGrid {
		e.renderGrid(screen)
	}

	// Mouse coordinates display
	e.renderCoordinates(screen)
}

// renderMapNameInput handles drawing the map name input field
func (e *editor) renderMapNameInput(screen *ebiten.Image) {
	vw, _ := ebiten.WindowSize()
	mapNameY := 72
	mapNameInputW := 200
	mapNameInputX := vw - mapNameInputW - 8
	mapNameX := mapNameInputX - 40

	text.Draw(screen, "Map:", basicfont.Face7x13, mapNameX, mapNameY+16, color.White)

	// Draw map name input box
	ebitenutil.DrawRect(screen, float64(mapNameInputX), float64(mapNameY), float64(mapNameInputW), 24, color.NRGBA{40, 40, 50, 255})
	if e.nameFocus {
		ebitenutil.DrawRect(screen, float64(mapNameInputX), float64(mapNameY), float64(mapNameInputW), 24, color.NRGBA{60, 60, 80, 255})
	}

	nm := e.name
	if nm == "" {
		nm = "New Map"
	}
	text.Draw(screen, nm, basicfont.Face7x13, mapNameInputX+6, mapNameY+16, color.White)
	if e.nameFocus {
		text.Draw(screen, "|", basicfont.Face7x13, mapNameInputX+6+len(nm)*7, mapNameY+16, color.White)
	}
}

func (e *editor) renderToolbarButtons(screen *ebiten.Image) {
	saveX := 450
	saveY := 8
	clrX := saveX + 110
	clrY := saveY
	assetsX := clrX + 90
	assetsY := clrY
	loadX := assetsX + 90
	loadY := assetsY
	helpX := loadX + 100
	helpY := loadY

	// Save button
	ebitenutil.DrawRect(screen, float64(saveX), float64(saveY), 100, 24, color.NRGBA{70, 110, 70, 255})
	text.Draw(screen, "Save", basicfont.Face7x13, saveX+30, saveY+16, color.White)

	// Clear selection button
	ebitenutil.DrawRect(screen, float64(clrX), float64(clrY), 80, 24, color.NRGBA{90, 70, 70, 255})
	text.Draw(screen, "Clear", basicfont.Face7x13, clrX+20, clrY+16, color.White)

	// Assets button for decorative elements
	ebitenutil.DrawRect(screen, float64(assetsX), float64(assetsY), 80, 24, color.NRGBA{120, 90, 120, 255})
	text.Draw(screen, "Assets", basicfont.Face7x13, assetsX+15, assetsY+16, color.White)

	// Load Map button
	ebitenutil.DrawRect(screen, float64(loadX), float64(loadY), 90, 24, color.NRGBA{70, 90, 120, 255})
	text.Draw(screen, "Load Map", basicfont.Face7x13, loadX+8, loadY+16, color.White)

	// Help button
	ebitenutil.DrawRect(screen, float64(helpX), float64(helpY), 60, 24, color.NRGBA{120, 120, 70, 255})
	text.Draw(screen, "Help", basicfont.Face7x13, helpX+12, helpY+16, color.White)

	// Status display
	if e.status != "" {
		// Draw status message in a more visible location - below the buttons
		text.Draw(screen, e.status, basicfont.Face7x13, 8, 8+5*28+20, color.NRGBA{180, 220, 180, 255})
	}
}

// renderBackgroundInfo handles displaying background image information
func (e *editor) renderBackgroundInfo(screen *ebiten.Image) {
	bx := 8
	by := 48

	if e.bg != nil && e.bgPath != "" {
		filename := filepath.Base(e.bgPath)
		text.Draw(screen, "BG: "+filename, basicfont.Face7x13, bx, by+16, color.White)
	}
}

// renderGrid handles drawing the coordinate grid overlay
func (e *editor) renderGrid(screen *ebiten.Image) {
	if e.bg == nil {
		return
	}

	// Get coordinate transformation functions
	toX := func(nx float64) int { return e.getCoordinateTransformation(nx, 0).x }
	toY := func(ny float64) int { return e.getCoordinateTransformation(0, ny).y }

	const topUIH = 120

	// Calculate grid bounds based on current display area - using consistent calculation

	// Use the same coordinate transformation function that's used elsewhere
	transform := e.getCoordinateTransformation(0, 0)
	gridTop := float64(transform.y)
	gridLeft := float64(transform.x)
	gridRight := gridLeft + float64(e.getCoordinateTransformation(1, 0).x-transform.x)
	gridBottom := gridTop + float64(e.getCoordinateTransformation(0, 1).y-transform.y)

	// Draw grid lines every 10% interval
	for i := 1; i < 10; i++ {
		x := float64(toX(float64(i) / 10.0))
		y := float64(toY(float64(i) / 10.0))
		ebitenutil.DrawLine(screen, x, gridTop, x, gridBottom, color.NRGBA{60, 60, 70, 120})
		ebitenutil.DrawLine(screen, gridLeft, y, gridRight, y, color.NRGBA{60, 60, 70, 120})
	}
}

// renderCoordinates handles displaying mouse coordinates
func (e *editor) renderCoordinates(screen *ebiten.Image) {
	if e.bg == nil {
		return
	}

	const topUIH = 120
	vw, _ := ebiten.WindowSize()
	mx, my := ebiten.CursorPosition()

	sw, _ := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
	sx := float64(vw) / float64(sw)
	sy := float64(vw-topUIH) / float64(sw) // Use sw for height since aspect ratio should be maintained
	s := sx
	if sy < sx {
		s = sy
	}

	dispW := int(float64(sw) * s)
	offX := (vw - dispW) / 2
	offY := topUIH + ((vw - topUIH - dispW) / 2)

	dispH := int(float64(sw) * s) // Maintain aspect ratio

	// Adjust offY calculation to use available height properly
	_, vh := ebiten.WindowSize()
	offY = topUIH + (vh-topUIH-dispH)/2

	if mx >= offX && mx < offX+dispW && my >= offY && my < offY+dispH {
		nx := (float64(mx - offX)) / float64(dispW)
		ny := (float64(my - offY)) / float64(dispH)
		coordX := vw - 120
		coordText := fmt.Sprintf("(%.3f, %.3f)", nx, ny)
		text.Draw(screen, coordText, basicfont.Face7x13, coordX, 10, color.NRGBA{200, 200, 210, 255})
	}
}

// renderLayerButtons handles drawing the layer selection buttons
func (e *editor) renderLayerButtons(screen *ebiten.Image) {
	layerNames := []string{"BG", "Frame", "Deploy", "Lanes", "Obstacles", "Assets", "Player Bases", "All"}
	layerColors := []color.NRGBA{
		{0x6a, 0x6a, 0x7a, 0xff}, // Gray for BG
		{0xff, 0xff, 0x00, 0xff}, // Yellow for Frame
		{0x4a, 0x9e, 0xff, 0xff}, // Blue for Deploy
		{0x4a, 0xff, 0x7a, 0xff}, // Green for Lanes
		{0x8b, 0x45, 0x13, 0xff}, // Brown for Obstacles
		{0xff, 0x6b, 0x6b, 0xff}, // Pink for Assets
		{0xff, 0x8a, 0x4a, 0xff}, // Orange for Bases
		{0x8a, 0x8a, 0x9a, 0xff}, // Gray for All
	}

	for i := 0; i < len(layerNames); i++ {
		x, y, w, h := 150, 8+i*32, 80, 28

		// Background with gradient effect
		baseCol := color.NRGBA{0x2a, 0x2a, 0x3a, 0xff}
		if e.currentLayer == i {
			baseCol = layerColors[i]
		}

		// Draw main button with shadow effect
		ebitenutil.DrawRect(screen, float64(x+1), float64(y+1), float64(w), float64(h), color.NRGBA{0x1a, 0x1a, 0x2a, 0xff})
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), baseCol)

		// Add border
		borderCol := color.NRGBA{0x5a, 0x5a, 0x6a, 0xff}
		if e.currentLayer == i {
			borderCol = color.NRGBA{0xff, 0xff, 0xff, 0xff}
		}
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), 1, borderCol)
		ebitenutil.DrawRect(screen, float64(x), float64(y+h-1), float64(w), 1, borderCol)
		ebitenutil.DrawRect(screen, float64(x), float64(y), 1, float64(h), borderCol)
		ebitenutil.DrawRect(screen, float64(x+w-1), float64(y), 1, float64(h), borderCol)

		// Draw text (centered)
		textW := len(layerNames[i]) * 7
		textX := x + (w-textW)/2
		text.Draw(screen, layerNames[i], basicfont.Face7x13, textX, y+18, color.White)

		// Selection indicator
		if e.currentLayer == i {
			ebitenutil.DrawRect(screen, float64(x-2), float64(y), 3, float64(h), color.NRGBA{0xff, 0xff, 0xff, 0xff})
		}
	}
}

// Helper struct for coordinate transformations
type coordPoint struct {
	x, y int
}

// getCoordinateTransformation provides consistent coordinate system
func (e *editor) getCoordinateTransformation(nx, ny float64) coordPoint {
	if e.bg != nil {
		const topUIH = 120
		sw, sh := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
		vw, vh := ebiten.WindowSize()
		vh -= topUIH
		sx := float64(vw) / float64(sw)
		sy := float64(vh) / float64(sh)
		s := sx
		if sy < sx {
			s = sy
		}
		extendedW := int(float64(sw) * s * 1.2)
		extendedH := int(float64(sh) * s * 1.2)
		borderOffX := (vw - extendedW) / 2
		borderOffY := topUIH + (vh-extendedH)/2
		mapBorderW := int(float64(extendedW) / 1.2)
		mapBorderH := int(float64(extendedH) / 1.2)
		borderOffX += (extendedW - mapBorderW) / 2
		borderOffY += (extendedH - mapBorderH) / 2

		if nx != 0 || ny != 0 {
			return coordPoint{
				x: borderOffX + int(nx*float64(mapBorderW)),
				y: borderOffY + int(ny*float64(mapBorderH)),
			}
		}
		return coordPoint{
			x: borderOffX,
			y: borderOffY,
		}
	}
	return coordPoint{x: 0, y: 0}
}

// renderDeployZones handles drawing deployment zones with selection highlighting
func (e *editor) renderDeployZones(screen *ebiten.Image, toX, toY func(float64) int) {
	for i, r := range e.def.DeployZones {
		x := toX(r.X)
		y := toY(r.Y)
		rw := e.getCoordinateTransformation(r.X+r.W, r.Y+r.H).x - x
		rh := e.getCoordinateTransformation(r.X+r.W, r.Y+r.H).y - y

		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(rw), float64(rh), color.NRGBA{60, 150, 90, 90})

		// Selection highlight
		if e.selKind == "deploy" && e.selIndex == i {
			// Draw border
			ebitenutil.DrawRect(screen, float64(x), float64(y), float64(rw), 1, color.NRGBA{240, 196, 25, 255})
			ebitenutil.DrawRect(screen, float64(x), float64(y+rh-1), float64(rw), 1, color.NRGBA{240, 196, 25, 255})
			ebitenutil.DrawRect(screen, float64(x), float64(y), 1, float64(rh), color.NRGBA{240, 196, 25, 255})
			ebitenutil.DrawRect(screen, float64(x+rw-1), float64(y), 1, float64(rh), color.NRGBA{240, 196, 25, 255})
			// Corner handles
			handles := [][2]int{{x, y}, {x + rw, y}, {x + rw, y + rh}, {x, y + rh}}
			for _, h := range handles {
				ebitenutil.DrawRect(screen, float64(h[0]-6), float64(h[1]-6), 12, 12, color.NRGBA{240, 196, 25, 255})
			}
		}
	}
}

// renderBackgroundElements handles drawing meeting stones and gold mines
func (e *editor) renderBackgroundElements(screen *ebiten.Image, toX, toY func(float64) int) {
	// Meeting stones
	for i, p := range e.def.MeetingStones {
		col := color.NRGBA{140, 120, 220, 255}
		if e.selKind == "stone" && e.selIndex == i {
			col = color.NRGBA{240, 196, 25, 255}
		}
		ebitenutil.DrawRect(screen, float64(toX(p.X)-2), float64(toY(p.Y)-2), 4, 4, col)
	}

	// Gold mines
	for i, p := range e.def.GoldMines {
		col := color.NRGBA{200, 170, 40, 255}
		if e.selKind == "mine" && e.selIndex == i {
			col = color.NRGBA{240, 196, 25, 255}
		}
		ebitenutil.DrawRect(screen, float64(toX(p.X)-3), float64(toY(p.Y)-3), 6, 6, col)
	}
}

// renderLanes handles drawing movement lanes with direction indicators
func (e *editor) renderLanes(screen *ebiten.Image, toX, toY func(float64) int) {
	for i, ln := range e.def.Lanes {
		col := color.NRGBA{90, 160, 220, 255}
		if ln.Dir < 0 {
			col = color.NRGBA{220, 110, 110, 255}
		}
		if e.selKind == "lane" && e.selIndex == i {
			col = color.NRGBA{240, 196, 25, 255}
		}
		for j := 1; j < len(ln.Points); j++ {
			x0, y0 := toX(ln.Points[j-1].X), toY(ln.Points[j-1].Y)
			x1, y1 := toX(ln.Points[j].X), toY(ln.Points[j].Y)
			ebitenutil.DrawLine(screen, float64(x0), float64(y0), float64(x1), float64(y1), col)
		}
	}

	// Draw temporary lane if being drawn
	if len(e.tmpLane) > 0 {
		for i := 1; i < len(e.tmpLane); i++ {
			x0 := toX(e.tmpLane[i-1].X)
			y0 := toY(e.tmpLane[i-1].Y)
			x1 := toX(e.tmpLane[i].X)
			y1 := toY(e.tmpLane[i].Y)
			ebitenutil.DrawLine(screen, float64(x0), float64(y0), float64(x1), float64(y1), color.NRGBA{200, 220, 90, 255})
		}
	}
}

// renderObstacles handles drawing obstacles with images
func (e *editor) renderObstacles(screen *ebiten.Image, toX, toY func(float64) int) {
	for i, obs := range e.def.Obstacles {
		x := toX(obs.X)
		y := toY(obs.Y)
		w := e.getCoordinateTransformation(obs.X+obs.Width, obs.Y+obs.Height).x - x
		h := e.getCoordinateTransformation(obs.X+obs.Width, obs.Y+obs.Height).y - y

		// Try to draw the actual obstacle image if available
		drewImage := false
		if obs.Image != "" {
			obstaclePaths := []string{
				filepath.Join("..", "..", "client", "internal", "game", "assets", "obstacles", obs.Image),
				filepath.Join("obstacles", obs.Image),
				obs.Image,
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

		// Selection highlight
		if e.selKind == "obstacle" && e.selIndex == i {
			ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), 1, color.NRGBA{240, 196, 25, 255})
			ebitenutil.DrawRect(screen, float64(x), float64(y+h-1), float64(w), 1, color.NRGBA{240, 196, 25, 255})
			ebitenutil.DrawRect(screen, float64(x), float64(y), 1, float64(h), color.NRGBA{240, 196, 25, 255})
			ebitenutil.DrawRect(screen, float64(x+w-1), float64(y), 1, float64(h), color.NRGBA{240, 196, 25, 255})
			handles := [][2]int{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}}
			for _, h := range handles {
				ebitenutil.DrawRect(screen, float64(h[0]-6), float64(h[1]-6), 12, 12, color.NRGBA{240, 196, 25, 255})
			}
		}
	}
}

// renderDecorativeElements handles drawing decorative elements with images
func (e *editor) renderDecorativeElements(screen *ebiten.Image, toX, toY func(float64) int) {
	for i, dec := range e.def.DecorativeElements {
		x := toX(dec.X)
		y := toY(dec.Y)
		w := e.getCoordinateTransformation(dec.X+dec.Width, dec.Y+dec.Height).x - x
		h := e.getCoordinateTransformation(dec.X+dec.Width, dec.Y+dec.Height).y - y

		// Try to draw the actual decorative image if available
		drewImage := false
		if dec.Image != "" {
			decorativePaths := []string{
				filepath.Join("..", "..", "client", "internal", "game", "assets", "obstacles", dec.Image),
				filepath.Join("decorative", dec.Image),
				dec.Image,
			}

			for _, decorativePath := range decorativePaths {
				if img, _, err := ebitenutil.NewImageFromFile(decorativePath); err == nil {
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
			ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), color.NRGBA{200, 150, 200, 120})
		}

		// Selection highlight
		if e.selKind == "decorative" && e.selIndex == i {
			ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), 1, color.NRGBA{240, 196, 25, 255})
			ebitenutil.DrawRect(screen, float64(x), float64(y+h-1), float64(w), 1, color.NRGBA{240, 196, 25, 255})
			ebitenutil.DrawRect(screen, float64(x), float64(y), 1, float64(h), color.NRGBA{240, 196, 25, 255})
			ebitenutil.DrawRect(screen, float64(x+w-1), float64(y), 1, float64(h), color.NRGBA{240, 196, 25, 255})
			handles := [][2]int{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}}
			for _, h := range handles {
				ebitenutil.DrawRect(screen, float64(h[0]-6), float64(h[1]-6), 12, 12, color.NRGBA{240, 196, 25, 255})
			}
		}
	}
}

// renderBases handles drawing player and enemy bases
func (e *editor) renderBases(screen *ebiten.Image, toX, toY func(float64) int) {
	// Player base
	if (e.def.PlayerBase.X != 0 || e.def.PlayerBase.Y != 0) || e.playerBaseExists {
		x := toX(e.def.PlayerBase.X)
		y := toY(e.def.PlayerBase.Y)

		baseW := 96
		baseH := 96

		col := color.NRGBA{0, 150, 255, 255} // Blue for player
		if e.selKind == "playerbase" {
			col = color.NRGBA{240, 196, 25, 255}
		}

		// Fallback rectangle (when no image available)
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(baseW), float64(baseH), col)

		// Player Base label
		label := "PLAYER BASE"
		labelW := len(label) * 7
		labelX := x + (96-labelW)/2
		labelY := y - 20
		ebitenutil.DrawRect(screen, float64(labelX-4), float64(labelY-2), float64(labelW+8), 16, color.NRGBA{0, 0, 0, 180})
		text.Draw(screen, label, basicfont.Face7x13, labelX, labelY+12, color.NRGBA{0, 150, 255, 255})

		// Selection border
		if e.selKind == "playerbase" {
			ebitenutil.DrawRect(screen, float64(x-2), float64(y-2), float64(baseW+4), 2, color.NRGBA{240, 196, 25, 255})
			ebitenutil.DrawRect(screen, float64(x-2), float64(y+baseH), float64(baseW+4), 2, color.NRGBA{240, 196, 25, 255})
			ebitenutil.DrawRect(screen, float64(x-2), float64(y-2), 2, float64(baseH+4), color.NRGBA{240, 196, 25, 255})
			ebitenutil.DrawRect(screen, float64(x+baseW), float64(y-2), 2, float64(baseH+4), color.NRGBA{240, 196, 25, 255})
		}
	}

	// Enemy base
	if (e.def.EnemyBase.X != 0 || e.def.EnemyBase.Y != 0) || e.enemyBaseExists {
		x := toX(e.def.EnemyBase.X)
		y := toY(e.def.EnemyBase.Y)

		baseW := 96
		baseH := 96

		col := color.NRGBA{255, 0, 0, 255} // Red for enemy
		if e.selKind == "enemybase" {
			col = color.NRGBA{240, 196, 25, 255}
		}

		// Fallback rectangle (when no image available)
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(baseW), float64(baseH), col)

		// Enemy Base label
		label := "ENEMY BASE"
		labelW := len(label) * 7
		labelX := x + (96-labelW)/2
		labelY := y - 20
		ebitenutil.DrawRect(screen, float64(labelX-4), float64(labelY-2), float64(labelW+8), 16, color.NRGBA{0, 0, 0, 180})
		text.Draw(screen, label, basicfont.Face7x13, labelX, labelY+12, color.NRGBA{255, 0, 0, 255})

		// Selection border
		if e.selKind == "enemybase" {
			ebitenutil.DrawRect(screen, float64(x-2), float64(y-2), float64(baseW+4), 2, color.NRGBA{240, 196, 25, 255})
			ebitenutil.DrawRect(screen, float64(x-2), float64(y+baseH), float64(baseW+4), 2, color.NRGBA{240, 196, 25, 255})
			ebitenutil.DrawRect(screen, float64(x-2), float64(y-2), 2, float64(baseH+4), color.NRGBA{240, 196, 25, 255})
			ebitenutil.DrawRect(screen, float64(x+baseW), float64(y-2), 2, float64(baseH+4), color.NRGBA{240, 196, 25, 255})
		}
	}
}
