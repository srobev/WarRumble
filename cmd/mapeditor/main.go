package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image/color"
	"log"
	"math"
	"os"
	"path/filepath"
	"rumble/shared/protocol"
	"strings"

	"github.com/gorilla/websocket"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

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
				// Initialize base existence flags based on loaded map
				e.playerBaseExists = e.def.PlayerBase.X != 0 || e.def.PlayerBase.Y != 0
				e.enemyBaseExists = e.def.EnemyBase.X != 0 || e.def.EnemyBase.Y != 0
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

				// Auto-scale camera to fit all objects in the map
				e.autoScaleToFitObjects()
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

	// Handle camera controls for map editor
	// Zoom with mouse wheel
	_, wy := ebiten.Wheel()
	if wy != 0 {
		oldZoom := e.cameraZoom
		e.cameraZoom *= (1.0 + wy*0.1)
		if e.cameraZoom < e.cameraMinZoom {
			e.cameraZoom = e.cameraMinZoom
		}
		if e.cameraZoom > e.cameraMaxZoom {
			e.cameraZoom = e.cameraMaxZoom
		}
		// Adjust camera position to zoom towards mouse cursor
		if oldZoom != e.cameraZoom {
			mx, my := ebiten.CursorPosition()
			zoomFactor := e.cameraZoom / oldZoom
			e.cameraX = float64(mx) - (float64(mx)-e.cameraX)*zoomFactor
			e.cameraY = float64(my) - (float64(my)-e.cameraY)*zoomFactor
		}
	}

	// Pan with middle mouse button or right mouse button when not over UI
	if ebiten.IsMouseButtonPressed(ebiten.MouseButtonMiddle) ||
		(ebiten.IsMouseButtonPressed(ebiten.MouseButtonRight) && !e.cameraDragging) {
		if !e.cameraDragging {
			e.cameraDragging = true
			e.cameraDragStartX, e.cameraDragStartY = ebiten.CursorPosition()
		} else {
			cx, cy := ebiten.CursorPosition()
			newCameraX := e.cameraX + float64(cx-e.cameraDragStartX)
			newCameraY := e.cameraY + float64(cy-e.cameraDragStartY)

			// Apply 20% boundary limits to match battle system
			vw, vh := ebiten.WindowSize()
			mapWidth := float64(vw) * e.cameraZoom
			mapHeight := float64(vh) * e.cameraZoom
			maxScrollX := mapWidth * 0.2  // 20% outside left/right borders
			maxScrollY := mapHeight * 0.2 // 20% outside top/bottom borders

			// Clamp camera position within boundaries
			if newCameraX > maxScrollX {
				newCameraX = maxScrollX
			} else if newCameraX < -maxScrollX {
				newCameraX = -maxScrollX
			}
			if newCameraY > maxScrollY {
				newCameraY = maxScrollY
			} else if newCameraY < -maxScrollY {
				newCameraY = -maxScrollY
			}

			e.cameraX = newCameraX
			e.cameraY = newCameraY
			e.cameraDragStartX, e.cameraDragStartY = cx, cy
		}
	} else {
		e.cameraDragging = false
	}

	// Handle frame manipulation with mouse
	if e.bg != nil && !e.frameDragging {
		// Calculate frame handle positions for hit testing
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

		// Apply configurable frame position and size
		frameBorderW := int(float64(mapBorderW) * e.frameWidth)
		frameBorderH := int(float64(mapBorderH) * e.frameHeight)
		frameOffX := borderOffX + int(float64(mapBorderW)*(e.frameX-0.5)*e.frameWidth) + (mapBorderW-frameBorderW)/2
		frameOffY := borderOffY + int(float64(mapBorderH)*(e.frameY-0.5)*e.frameHeight) + (mapBorderH-frameBorderH)/2

		handleSize := 8
		// Check if clicking on frame handles
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Corner handles for resizing
			corners := [][2]int{
				{frameOffX - handleSize/2, frameOffY - handleSize/2},                               // Top-left
				{frameOffX + frameBorderW - handleSize/2, frameOffY - handleSize/2},                // Top-right
				{frameOffX + frameBorderW - handleSize/2, frameOffY + frameBorderH - handleSize/2}, // Bottom-right
				{frameOffX - handleSize/2, frameOffY + frameBorderH - handleSize/2},                // Bottom-left
			}
			handleClicked := false
			for i, corner := range corners {
				if mx >= corner[0] && mx < corner[0]+handleSize && my >= corner[1] && my < corner[1]+handleSize {
					e.frameDragging = true
					e.frameDragStartX = mx
					e.frameDragStartY = my
					e.frameDragHandle = i + 1 // 1=top-left, 2=top-right, 3=bottom-right, 4=bottom-left
					e.status = fmt.Sprintf("Resizing frame from corner %d", i+1)
					handleClicked = true
					break
				}
			}
			// Center handle for moving (only if no corner was clicked)
			if !handleClicked {
				centerX := frameOffX + frameBorderW/2 - handleSize/2
				centerY := frameOffY + frameBorderH/2 - handleSize/2
				if mx >= centerX-handleSize && mx < centerX+handleSize*2 && my >= centerY-handleSize && my < centerY+handleSize*2 {
					e.frameDragging = true
					e.frameDragStartX = mx
					e.frameDragStartY = my
					e.frameDragHandle = 0 // 0=center
					e.status = "Dragging frame position"
				}
			}
		}
	}

	// Deselect tool when overlays are open to prevent accidental object placement
	if e.showMapBrowser || e.showAssetsBrowser || e.showObstaclesBrowser || e.showDecorativeBrowser {
		e.tool = -1 // Deselect any tool
	}

	// Decorative elements browser interactions
	if e.showDecorativeBrowser {
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
				e.decorativeBrowserScroll -= int(wy)
				if e.decorativeBrowserScroll < 0 {
					e.decorativeBrowserScroll = 0
				}
				if len(e.availableDecorative) > maxRows {
					maxStart := len(e.availableDecorative) - maxRows
					if e.decorativeBrowserScroll > maxStart {
						e.decorativeBrowserScroll = maxStart
					}
				}
			}
		}
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if mx >= panelX && mx < panelX+panelW && my >= panelY && my < panelY+panelH {
				idx := (my-panelY)/rowH + e.decorativeBrowserScroll
				if idx >= 0 && idx < len(e.availableDecorative) {
					item := e.availableDecorative[idx]
					if item == ".." {
						// go up
						parent := filepath.Dir(e.decorativeCurrentPath)
						e.decorativeCurrentPath = parent
						e.refreshDecorativeBrowser()
					} else if strings.HasPrefix(item, "[DIR] ") {
						// enter directory
						dirName := strings.TrimPrefix(item, "[DIR] ")
						newPath := filepath.Join(e.decorativeCurrentPath, dirName)
						e.decorativeCurrentPath = newPath
						e.refreshDecorativeBrowser()
					} else {
						// load file as decorative element
						fullPath := filepath.Join(e.decorativeCurrentPath, item)
						if _, _, err := ebitenutil.NewImageFromFile(fullPath); err == nil {
							if e.selKind == "decorative" && e.selIndex >= 0 && e.selIndex < len(e.def.DecorativeElements) {
								// Change image of selected decorative element
								e.def.DecorativeElements[e.selIndex].Image = filepath.Base(fullPath)
								e.status = fmt.Sprintf("Decorative element image changed: %s", filepath.Base(fullPath))
							} else {
								// Create new decorative element at center of canvas
								nx, ny := 0.5, 0.5 // center
								dec := protocol.DecorativeElement{
									X:      nx - 0.05,
									Y:      ny - 0.05,
									Image:  filepath.Base(fullPath),
									Width:  0.1,
									Height: 0.1,
									Layer:  1, // middle layer
								}
								e.def.DecorativeElements = append(e.def.DecorativeElements, dec)
								e.selKind, e.selIndex, e.selHandle = "decorative", len(e.def.DecorativeElements)-1, -1
								e.status = fmt.Sprintf("Decorative element added: %s", filepath.Base(fullPath))
							}
							e.showDecorativeBrowser = false
						} else {
							e.status = fmt.Sprintf("Failed to load decorative image: %v", err)
						}
					}
					e.decorativeBrowserSel = idx
				}
			}
		}
	}

	// Enhanced toolbar click handling (updated for new button positions)
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Check if clicking on toolbar buttons first
		toolbarClicked := false

		// Handle layer selection buttons (left side - compact)
		for i := 0; i < 8; i++ {
			x, y, w, h := 150, 8+i*32, 80, 28
			if mx >= x && mx < x+w && my >= y && my < y+h {
				if i == 7 {
					// "All" layer - enable show all layers
					e.showAllLayers = true
					e.currentLayer = 7
				} else {
					e.currentLayer = i
					e.showAllLayers = false // Disable show-all when selecting specific layer
				}
				layerNames := []string{"BG", "Frame", "Deploy", "Lanes", "Obstacles", "Assets", "Player Bases", "All"}
				e.status = fmt.Sprintf("Layer: %s", layerNames[i])
				toolbarClicked = true
				break
			}
		}

		// If toolbar was clicked, don't process object selection
		if toolbarClicked {
			return nil
		}
	}

	// Calculate button positions for tooltips - moved to avoid overlap
	saveX := 450
	saveY := 8
	clrX := saveX + 110
	clrY := saveY
	loadX := clrX + 90
	loadY := clrY
	helpX := loadX + 100
	helpY := loadY

	// Map name input positions - moved to right side
	vw, _ := ebiten.WindowSize()
	mapNameX := vw - 250
	mapNameY := 8
	mapNameInputX := mapNameX + 40
	mapNameInputY := mapNameY

	// BG buttons (streamlined - removed Load, Copy; kept Set and BG)
	bx := 8
	by := 48
	sx, sy, sw, sh := bx, by, 64, 24
	ax, ay, aw, ah := bx+70, by, 64, 24
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
	// Use consistent coordinate transformation matching Draw function
	toPix := func(nx, ny float64) (int, int) {
		if e.bg != nil {
			// Use the same transformation as Draw function
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
			return borderOffX + int(nx*float64(mapBorderW)), borderOffY + int(ny*float64(mapBorderH))
		}
		return offX + int(nx*float64(dispW)), offY + int(ny*float64(dispH))
	}

	// Match battle system: use screen dimensions for consistent scaling
	screenW, screenH := ebiten.WindowSize()
	_ = screenW // Mark as used to avoid compiler warning
	_ = screenH // Mark as used to avoid compiler warning

	// hit tests - use consistent coordinate system matching Draw function
	hitDeploy := func(mx, my int) (idx int, handle int, ok bool) {
		if e.bg != nil {
			// Use same calculations as Draw function
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

			for i, r := range e.def.DeployZones {
				x := borderOffX + int(r.X*float64(mapBorderW))
				y := borderOffY + int(r.Y*float64(mapBorderH))
				w := int(r.W * float64(mapBorderW))
				h := int(r.H * float64(mapBorderH))
				// corners - make them larger for easier clicking
				corners := [][4]int{{x - 6, y - 6, 12, 12}, {x + w - 6, y - 6, 12, 12}, {x + w - 6, y + h - 6, 12, 12}, {x - 6, y + h - 6, 12, 12}}
				for ci, c := range corners {
					if mx >= c[0] && mx < c[0]+c[2] && my >= c[1] && my < c[1]+c[3] {
						return i, ci, true
					}
				}
				if mx >= x && mx < x+w && my >= y && my < y+h {
					return i, -1, true
				}
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
		if e.bg != nil {
			// Use same calculations as Draw function
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

			for i, obs := range e.def.Obstacles {
				x := borderOffX + int(obs.X*float64(mapBorderW))
				y := borderOffY + int(obs.Y*float64(mapBorderH))
				w := int(obs.Width * float64(mapBorderW))
				h := int(obs.Height * float64(mapBorderH))
				// corners for resizing - make them larger for easier clicking
				corners := [][4]int{{x - 6, y - 6, 12, 12}, {x + w - 6, y - 6, 12, 12}, {x + w - 6, y + h - 6, 12, 12}, {x - 6, y + h - 6, 12, 12}}
				for ci, c := range corners {
					if mx >= c[0] && mx < c[0]+c[2] && my >= c[1] && my < c[1]+c[3] {
						return i, ci, true
					}
				}
				if mx >= x && mx < x+w && my >= y && my < y+h {
					return i, -1, true
				}
			}
		}
		return -1, 0, false
	}
	hitDecorative := func(mx, my int) (idx int, handle int, ok bool) {
		if e.bg != nil {
			// Use same calculations as Draw function
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

			for i, dec := range e.def.DecorativeElements {
				x := borderOffX + int(dec.X*float64(mapBorderW))
				y := borderOffY + int(dec.Y*float64(mapBorderH))
				w := int(dec.Width * float64(mapBorderW))
				h := int(dec.Height * float64(mapBorderH))
				// corners for resizing - make them larger for easier clicking
				corners := [][4]int{{x - 6, y - 6, 12, 12}, {x + w - 6, y - 6, 12, 12}, {x + w - 6, y + h - 6, 12, 12}, {x - 6, y + h - 6, 12, 12}}
				for ci, c := range corners {
					if mx >= c[0] && mx < c[0]+c[2] && my >= c[1] && my < c[1]+c[3] {
						return i, ci, true
					}
				}
				if mx >= x && mx < x+w && my >= y && my < y+h {
					return i, -1, true
				}
			}
		}
		return -1, 0, false
	}
	hitPlayerBase := func(mx, my int) bool {
		// Check if player base exists (use a flag or check if coordinates are valid)
		if e.def.PlayerBase.X == 0 && e.def.PlayerBase.Y == 0 && !e.playerBaseExists {
			return false
		}
		x, y := toPix(e.def.PlayerBase.X, e.def.PlayerBase.Y)
		baseW := 96
		baseH := 96
		return mx >= x && mx < x+baseW && my >= y && my < y+baseH
	}
	hitEnemyBase := func(mx, my int) bool {
		// Check if enemy base exists (use a flag or check if coordinates are valid)
		if e.def.EnemyBase.X == 0 && e.def.EnemyBase.Y == 0 && !e.enemyBaseExists {
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

		// Check layer buttons
		for i := 0; i < 8; i++ {
			x, y, w, h := 150, 8+i*32, 80, 28
			if mx >= x && mx < x+w && my >= y && my < y+h {
				layerTooltips := []string{
					"🏠 BG Layer: Show/hide background elements (meeting stones, gold mines)",
					"📐 Frame Layer: Edit map frame position, size and scale",
					"📦 Deploy Layer: Show/hide deployment zones",
					"🛤️ Lanes Layer: Show/hide movement lanes",
					"🌳 Obstacles Layer: Show/hide blocking objects",
					"🎨 Assets Layer: Show/hide decorative elements",
					"🏰 Player Bases Layer: Show/hide player and enemy bases",
					"👁️ All Layers: Show all map elements simultaneously",
				}
				e.helpText = layerTooltips[i]
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
			// BG buttons (Set, BG)
			if mx >= sx && mx < sx+sw && my >= sy && my < sy+sh {
				e.helpText = "Set BG: Set current background as map background"
				e.tooltipX = sx
				e.tooltipY = sy - 25
				e.showHelp = true
			} else if mx >= ax && mx < ax+aw && my >= ay && my < ay+ah {
				e.helpText = "BG: Browse and load background images"
				e.tooltipX = ax
				e.tooltipY = ay - 25
				e.showHelp = true
			} else
			// Name field
			if mx >= mapNameInputX && mx < mapNameInputX+200 && my >= mapNameInputY && my < mapNameInputY+24 {
				e.helpText = "Map Name: Enter a name for your map"
				e.tooltipX = mapNameInputX
				e.tooltipY = mapNameInputY - 25
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
			// Layer-based selection logic
			selected := false

			// Layer 1: Frame (only selectable when frame layer is active)
			if e.currentLayer == 1 && !selected {
				// Check frame handles first
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

				frameBorderW := int(float64(mapBorderW) * e.frameWidth)
				frameBorderH := int(float64(mapBorderH) * e.frameHeight)
				frameOffX := borderOffX + int(float64(mapBorderW)*(e.frameX-0.5)*e.frameWidth) + (mapBorderW-frameBorderW)/2
				frameOffY := borderOffY + int(float64(mapBorderH)*(e.frameY-0.5)*e.frameHeight) + (mapBorderH-frameBorderH)/2

				handleSize := 8
				corners := [][2]int{
					{frameOffX - handleSize/2, frameOffY - handleSize/2},
					{frameOffX + frameBorderW - handleSize/2, frameOffY - handleSize/2},
					{frameOffX + frameBorderW - handleSize/2, frameOffY + frameBorderH - handleSize/2},
					{frameOffX - handleSize/2, frameOffY + frameBorderH - handleSize/2},
				}

				for i, corner := range corners {
					if mx >= corner[0] && mx < corner[0]+handleSize && my >= corner[1] && my < corner[1]+handleSize {
						e.frameDragging = true
						e.frameDragStartX = mx
						e.frameDragStartY = my
						e.frameDragHandle = i + 1
						e.status = fmt.Sprintf("Resizing frame from corner %d", i+1)
						selected = true
						break
					}
				}

				if !selected {
					centerX := frameOffX + frameBorderW/2 - handleSize/2
					centerY := frameOffY + frameBorderH/2 - handleSize/2
					if mx >= centerX-handleSize && mx < centerX+handleSize*2 && my >= centerY-handleSize && my < centerY+handleSize*2 {
						e.frameDragging = true
						e.frameDragStartX = mx
						e.frameDragStartY = my
						e.frameDragHandle = 0
						e.status = "Dragging frame position"
						selected = true
					}
				}

				if !selected {
					// Check if clicking inside frame area
					if mx >= frameOffX && mx < frameOffX+frameBorderW && my >= frameOffY && my < frameOffY+frameBorderH {
						e.selKind, e.selIndex, e.selHandle = "frame", -1, -1
						e.dragging, e.lastMx, e.lastMy = true, mx, my
						selected = true
					}
				}
			}

			// Allow selection of any element regardless of current layer
			// Check in priority order: bases first (highest priority), then other elements
			if hitPlayerBase(mx, my) {
				e.selKind, e.selIndex, e.selHandle = "playerbase", -1, -1
				e.dragging = true
				e.lastMx, e.lastMy = mx, my
				selected = true
			} else if hitEnemyBase(mx, my) {
				e.selKind, e.selIndex, e.selHandle = "enemybase", -1, -1
				e.dragging = true
				e.lastMx, e.lastMy = mx, my
				selected = true
			} else if idx, h, ok := hitDecorative(mx, my); ok {
				e.selKind, e.selIndex, e.selHandle = "decorative", idx, h
				e.dragging, e.lastMx, e.lastMy = true, mx, my
				selected = true
			} else if idx, h, ok := hitObstacle(mx, my); ok {
				e.selKind, e.selIndex, e.selHandle = "obstacle", idx, h
				e.dragging, e.lastMx, e.lastMy = true, mx, my
				selected = true
			} else if i, ok := hitLane(mx, my); ok {
				e.selKind, e.selIndex, e.selHandle = "lane", i, -1
				e.dragging = true
				e.lastMx, e.lastMy = mx, my
				selected = true
			} else if idx, h, ok := hitDeploy(mx, my); ok {
				if e.selKind == "deploy" && e.selIndex == idx && e.selHandle == h {
					// Change deploy zone owner on double-click
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
				selected = true
			} else if i, ok := hitPointList(e.def.MeetingStones, mx, my, 6); ok {
				e.selKind, e.selIndex, e.selHandle = "stone", i, -1
				e.dragging = true
				e.lastMx, e.lastMy = mx, my
				selected = true
			} else if i, ok := hitPointList(e.def.GoldMines, mx, my, 6); ok {
				e.selKind, e.selIndex, e.selHandle = "mine", i, -1
				e.dragging = true
				e.lastMx, e.lastMy = mx, my
				selected = true
			}

			// If nothing was selected, create new objects based on current tool or layer
			if !selected {
				nx, ny := toNorm(mx, my)

				// Create objects based on current layer or tool
				switch e.currentLayer {
				case 2: // Deploy layer - create deploy zones
					e.def.DeployZones = append(e.def.DeployZones, protocol.DeployZone{X: nx - 0.05, Y: ny - 0.05, W: 0.1, H: 0.1, Owner: "player"})
					e.selKind, e.selIndex, e.selHandle = "deploy", len(e.def.DeployZones)-1, -1
					e.status = "Deploy zone created"
				case 3: // Lanes layer - start drawing lanes
					e.tmpLane = append(e.tmpLane, protocol.PointF{X: nx, Y: ny})
					e.selKind = ""
					e.status = "Lane point added"
				case 4: // Obstacles layer - create obstacles
					e.def.Obstacles = append(e.def.Obstacles, protocol.Obstacle{X: nx - 0.05, Y: ny - 0.05, Type: "tree", Image: "tree.png", Width: 0.1, Height: 0.1})
					e.selKind, e.selIndex, e.selHandle = "obstacle", len(e.def.Obstacles)-1, -1
					e.status = "Obstacle created"
				case 5: // Assets layer - create decorative elements
					e.def.DecorativeElements = append(e.def.DecorativeElements, protocol.DecorativeElement{X: nx - 0.05, Y: ny - 0.05, Image: "decorative.png", Width: 0.1, Height: 0.1, Layer: 1})
					e.selKind, e.selIndex, e.selHandle = "decorative", len(e.def.DecorativeElements)-1, -1
					e.status = "Decorative element created"
				case 6: // Bases layer - create bases
					// Create player base if it doesn't exist, otherwise create enemy base
					if !e.playerBaseExists {
						e.def.PlayerBase = protocol.PointF{X: nx, Y: ny}
						e.playerBaseExists = true
						e.selKind = "playerbase"
						e.status = "Player base created"
					} else if !e.enemyBaseExists {
						e.def.EnemyBase = protocol.PointF{X: nx, Y: ny}
						e.enemyBaseExists = true
						e.selKind = "enemybase"
						e.status = "Enemy base created"
					} else {
						e.status = "Both bases already exist. Select and delete one first."
					}
				default:
					// Fallback to tool-based creation for BG and Frame layers
					switch e.tool {
					case 0: // Deploy zones
						e.def.DeployZones = append(e.def.DeployZones, protocol.DeployZone{X: nx - 0.05, Y: ny - 0.05, W: 0.1, H: 0.1, Owner: "player"})
						e.selKind, e.selIndex, e.selHandle = "deploy", len(e.def.DeployZones)-1, -1
						e.tool = -1
					case 1: // Meeting stones
						e.def.MeetingStones = append(e.def.MeetingStones, protocol.PointF{X: nx, Y: ny})
						e.selKind, e.selIndex = "stone", len(e.def.MeetingStones)-1
						e.tool = -1
					case 2: // Gold mines
						e.def.GoldMines = append(e.def.GoldMines, protocol.PointF{X: nx, Y: ny})
						e.selKind, e.selIndex = "mine", len(e.def.GoldMines)-1
						e.tool = -1
					case 3: // Movement lanes
						e.tmpLane = append(e.tmpLane, protocol.PointF{X: nx, Y: ny})
						e.selKind = ""
					case 4: // Obstacles
						e.def.Obstacles = append(e.def.Obstacles, protocol.Obstacle{X: nx - 0.05, Y: ny - 0.05, Type: "tree", Image: "tree.png", Width: 0.1, Height: 0.1})
						e.selKind, e.selIndex, e.selHandle = "obstacle", len(e.def.Obstacles)-1, -1
						e.tool = -1
					case 5: // Decorative elements
						e.def.DecorativeElements = append(e.def.DecorativeElements, protocol.DecorativeElement{X: nx - 0.05, Y: ny - 0.05, Image: "decorative.png", Width: 0.1, Height: 0.1, Layer: 1})
						e.selKind, e.selIndex, e.selHandle = "decorative", len(e.def.DecorativeElements)-1, -1
						e.tool = -1
					}
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
					// Allow objects to be placed outside the 0-1 coordinate range
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
					// Allow objects to be placed outside the 0-1 coordinate range
					e.def.Obstacles[e.selIndex] = obs
				}
			case "playerbase":
				p := e.def.PlayerBase
				p.X += ndx
				p.Y += ndy
				// Allow objects to be placed outside the 0-1 coordinate range
				e.def.PlayerBase = p
			case "enemybase":
				p := e.def.EnemyBase
				p.X += ndx
				p.Y += ndy
				// Allow objects to be placed outside the 0-1 coordinate range
				e.def.EnemyBase = p
			case "decorative":
				if e.selIndex >= 0 && e.selIndex < len(e.def.DecorativeElements) {
					dec := e.def.DecorativeElements[e.selIndex]
					if e.selHandle == -1 {
						dec.X += ndx
						dec.Y += ndy
					} else {
						switch e.selHandle {
						case 0:
							dec.X += ndx
							dec.Y += ndy
							dec.Width -= ndx
							dec.Height -= ndy
						case 1:
							dec.Y += ndy
							dec.Width += ndx
							dec.Height -= ndy
						case 2:
							dec.Width += ndx
							dec.Height += ndy
						case 3:
							dec.X += ndx
							dec.Width -= ndx
							dec.Height += ndy
						}
						if dec.Width < 0.02 {
							dec.Width = 0.02
						}
						if dec.Height < 0.02 {
							dec.Height = 0.02
						}
					}
					// Allow objects to be placed outside the 0-1 coordinate range
					e.def.DecorativeElements[e.selIndex] = dec
				}
			}
		}
	}

	// Handle frame dragging
	if e.frameDragging {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			dx := mx - e.frameDragStartX
			dy := my - e.frameDragStartY

			// Convert screen delta to normalized frame coordinates
			vw, vh := ebiten.WindowSize()
			vh -= topUIH
			sw, sh := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
			sx := float64(vw) / float64(sw)
			sy := float64(vh) / float64(sh)
			s := sx
			if sy < sx {
				s = sy
			}
			extendedW := int(float64(sw) * s * 1.2)
			extendedH := int(float64(sh) * s * 1.2)
			mapBorderW := int(float64(extendedW) / 1.2)
			mapBorderH := int(float64(extendedH) / 1.2)

			// Calculate the change in normalized coordinates
			scaleFactor := 1.0 / (s * e.frameScale)
			dFrameX := float64(dx) * scaleFactor / float64(mapBorderW)
			dFrameY := float64(dy) * scaleFactor / float64(mapBorderH)

			if e.frameDragHandle == 0 {
				// Center handle: move the frame
				e.frameX += dFrameX
				e.frameY += dFrameY

				// Clamp frame position to reasonable bounds
				if e.frameX < -0.5 {
					e.frameX = -0.5
				}
				if e.frameX > 1.5 {
					e.frameX = 1.5
				}
				if e.frameY < -0.5 {
					e.frameY = -0.5
				}
				if e.frameY > 1.5 {
					e.frameY = 1.5
				}
				e.status = fmt.Sprintf("Frame moved to (%.3f, %.3f)", e.frameX, e.frameY)
			} else {
				// Corner handles: resize the frame
				switch e.frameDragHandle {
				case 1: // Top-left
					e.frameX += dFrameX
					e.frameY += dFrameY
					e.frameWidth -= dFrameX
					e.frameHeight -= dFrameY
				case 2: // Top-right
					e.frameY += dFrameY
					e.frameWidth += dFrameX
					e.frameHeight -= dFrameY
				case 3: // Bottom-right
					e.frameWidth += dFrameX
					e.frameHeight += dFrameY
				case 4: // Bottom-left
					e.frameX += dFrameX
					e.frameWidth -= dFrameX
					e.frameHeight += dFrameY
				}

				// Ensure minimum size
				if e.frameWidth < 0.1 {
					e.frameWidth = 0.1
				}
				if e.frameHeight < 0.1 {
					e.frameHeight = 0.1
				}

				// Clamp position to reasonable bounds
				if e.frameX < -0.5 {
					e.frameX = -0.5
				}
				if e.frameX > 1.5 {
					e.frameX = 1.5
				}
				if e.frameY < -0.5 {
					e.frameY = -0.5
				}
				if e.frameY > 1.5 {
					e.frameY = 1.5
				}

				e.status = fmt.Sprintf("Frame resized to (%.3f, %.3f)", e.frameWidth, e.frameHeight)
			}

			e.frameDragStartX = mx
			e.frameDragStartY = my
		} else {
			e.frameDragging = false
			e.frameDragHandle = 0
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
				case "decorative":
					if e.selIndex >= 0 && e.selIndex < len(e.def.DecorativeElements) {
						e.def.DecorativeElements = append(e.def.DecorativeElements[:e.selIndex], e.def.DecorativeElements[e.selIndex+1:]...)
						e.selKind = ""
						e.selIndex = -1
						e.status = "Decorative element deleted"
					}
				case "playerbase":
					// Reset player base to (0,0) and mark as not existing
					e.def.PlayerBase = protocol.PointF{X: 0, Y: 0}
					e.playerBaseExists = false
					e.selKind = ""
					e.selIndex = -1
					e.status = "Player base deleted"
				case "enemybase":
					// Reset enemy base to (0,0) and mark as not existing
					e.def.EnemyBase = protocol.PointF{X: 0, Y: 0}
					e.enemyBaseExists = false
					e.selKind = ""
					e.selIndex = -1
					e.status = "Enemy base deleted"
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

		// Frame manipulation shortcuts
		if k == ebiten.KeyF && !e.bgFocus {
			// Reset frame to center (0.5, 0.5) and scale 1.0
			e.frameX = 0.5
			e.frameY = 0.5
			e.frameScale = 1.0
			e.status = "Frame reset to center"
		}
		if k == ebiten.KeyArrowLeft && !e.bgFocus {
			e.frameX -= 0.01
			if e.frameX < 0 {
				e.frameX = 0
			}
			e.status = fmt.Sprintf("Frame X: %.3f", e.frameX)
		}
		if k == ebiten.KeyArrowRight && !e.bgFocus {
			e.frameX += 0.01
			if e.frameX > 1 {
				e.frameX = 1
			}
			e.status = fmt.Sprintf("Frame X: %.3f", e.frameX)
		}
		if k == ebiten.KeyArrowUp && !e.bgFocus {
			e.frameY -= 0.01
			if e.frameY < 0 {
				e.frameY = 0
			}
			e.status = fmt.Sprintf("Frame Y: %.3f", e.frameY)
		}
		if k == ebiten.KeyArrowDown && !e.bgFocus {
			e.frameY += 0.01
			if e.frameY > 1 {
				e.frameY = 1
			}
			e.status = fmt.Sprintf("Frame Y: %.3f", e.frameY)
		}
		if k == ebiten.KeyEqual && !e.bgFocus { // + key
			e.frameScale += 0.01
			if e.frameScale > 2.0 {
				e.frameScale = 2.0
			}
			e.status = fmt.Sprintf("Frame Scale: %.3f", e.frameScale)
		}
		if k == ebiten.KeyMinus && !e.bgFocus {
			e.frameScale -= 0.01
			if e.frameScale < 0.1 {
				e.frameScale = 0.1
			}
			e.status = fmt.Sprintf("Frame Scale: %.3f", e.frameScale)
		}

		// Number keys for tool selection
		if k >= ebiten.Key1 && k <= ebiten.Key6 && !e.bgFocus {
			toolIndex := int(k - ebiten.Key1)
			if toolIndex >= 0 && toolIndex < 6 {
				e.tool = toolIndex
				toolNames := []string{"Deploy Zones", "Meeting Stones", "Gold Mines", "Movement Lanes", "Obstacles", "Decorative"}
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
							relPath, _ := filepath.Rel(".", fullPath)
							e.status = fmt.Sprintf("BG loaded: %s", relPath)
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
	e.status = fmt.Sprintf("Browsing: %s (%d items)", filepath.Base(e.assetsCurrentPath), len(items))
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

func (e *editor) refreshDecorativeBrowser() {
	var items []string
	if e.decorativeCurrentPath != "." {
		items = append(items, "..")
	}
	entries, err := os.ReadDir(e.decorativeCurrentPath)
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
	e.availableDecorative = items
	e.decorativeBrowserScroll = 0
	e.decorativeBrowserSel = -1
	e.status = fmt.Sprintf("Browsing: %s (%d items)", e.decorativeCurrentPath, len(items))
}

func (e *editor) autoScaleToFitObjects() {
	if e.bg == nil {
		return
	}

	// Calculate bounds of all objects
	minX, minY := 1.0, 1.0
	maxX, maxY := 0.0, 0.0

	// Check deploy zones
	for _, r := range e.def.DeployZones {
		minX = math.Min(minX, r.X)
		minY = math.Min(minY, r.Y)
		maxX = math.Max(maxX, r.X+r.W)
		maxY = math.Max(maxY, r.Y+r.H)
	}

	// Check meeting stones
	for _, p := range e.def.MeetingStones {
		minX = math.Min(minX, p.X-0.01)
		minY = math.Min(minY, p.Y-0.01)
		maxX = math.Max(maxX, p.X+0.01)
		maxY = math.Max(maxY, p.Y+0.01)
	}

	// Check gold mines
	for _, p := range e.def.GoldMines {
		minX = math.Min(minX, p.X-0.01)
		minY = math.Min(minY, p.Y-0.01)
		maxX = math.Max(maxX, p.X+0.01)
		maxY = math.Max(maxY, p.Y+0.01)
	}

	// Check lanes
	for _, ln := range e.def.Lanes {
		for _, p := range ln.Points {
			minX = math.Min(minX, p.X-0.01)
			minY = math.Min(minY, p.Y-0.01)
			maxX = math.Max(maxX, p.X+0.01)
			maxY = math.Max(maxY, p.Y+0.01)
		}
	}

	// Check obstacles
	for _, obs := range e.def.Obstacles {
		minX = math.Min(minX, obs.X)
		minY = math.Min(minY, obs.Y)
		maxX = math.Max(maxX, obs.X+obs.Width)
		maxY = math.Max(maxY, obs.Y+obs.Height)
	}

	// Check decorative elements
	for _, dec := range e.def.DecorativeElements {
		minX = math.Min(minX, dec.X)
		minY = math.Min(minY, dec.Y)
		maxX = math.Max(maxX, dec.X+dec.Width)
		maxY = math.Max(maxY, dec.Y+dec.Height)
	}

	// Check bases
	if e.def.PlayerBase.X != 0 || e.def.PlayerBase.Y != 0 {
		minX = math.Min(minX, e.def.PlayerBase.X-0.05)
		minY = math.Min(minY, e.def.PlayerBase.Y-0.05)
		maxX = math.Max(maxX, e.def.PlayerBase.X+0.05)
		maxY = math.Max(maxY, e.def.PlayerBase.Y+0.05)
	}
	if e.def.EnemyBase.X != 0 || e.def.EnemyBase.Y != 0 {
		minX = math.Min(minX, e.def.EnemyBase.X-0.05)
		minY = math.Min(minY, e.def.EnemyBase.Y-0.05)
		maxX = math.Max(maxX, e.def.EnemyBase.X+0.05)
		maxY = math.Max(maxY, e.def.EnemyBase.Y+0.05)
	}

	// If no objects found, reset to default
	if minX >= maxX || minY >= maxY {
		e.cameraX = 0
		e.cameraY = 0
		e.cameraZoom = 1.0
		return
	}

	// Calculate required zoom to fit all objects
	vw, vh := ebiten.WindowSize()
	vh -= 120 // Account for UI height

	objectWidth := maxX - minX
	objectHeight := maxY - minY

	if objectWidth <= 0 {
		objectWidth = 0.1
	}
	if objectHeight <= 0 {
		objectHeight = 0.1
	}

	zoomX := float64(vw) / (objectWidth * float64(e.bg.Bounds().Dx()))
	zoomY := float64(vh) / (objectHeight * float64(e.bg.Bounds().Dy()))

	// Use the smaller zoom to ensure everything fits
	newZoom := math.Min(zoomX, zoomY)
	newZoom = math.Min(newZoom, e.cameraMaxZoom) // Don't exceed max zoom
	newZoom = math.Max(newZoom, e.cameraMinZoom) // Don't go below min zoom

	// Calculate camera position to center the objects
	centerX := (minX + maxX) / 2
	centerY := (minY + maxY) / 2

	// Convert normalized coordinates to screen coordinates
	screenCenterX := centerX * float64(e.bg.Bounds().Dx()) * newZoom
	screenCenterY := centerY * float64(e.bg.Bounds().Dy()) * newZoom

	// Center the camera on the objects
	e.cameraX = screenCenterX - float64(vw)/2
	e.cameraY = screenCenterY - float64(vh)/2
	e.cameraZoom = newZoom

	e.status = fmt.Sprintf("Auto-scaled to fit all objects (zoom: %.2f)", newZoom)
}

func (e *editor) Draw(screen *ebiten.Image) {
	vw, vh := ebiten.WindowSize()

	// Coordinates text position - bottom right corner
	coordX := vw - 120
	coordY := vh - 30

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

	// Top UI bar background to prevent overlap with canvas - optimized single draw
	ebitenutil.DrawRect(screen, 0, 0, float64(vw), float64(topUIH), color.NRGBA{28, 28, 40, 255})

	// Performance optimization: only draw elements when necessary
	if e.bg == nil {
		return
	}

	// BG
	if e.bg != nil {
		// Scale background to cover the extended camera space (120% to account for 20% scroll margins)
		sw, sh := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
		vw, vh = ebiten.WindowSize()
		vh -= topUIH

		// Calculate scale to fit the extended area (120% of original)
		extendedScale := 1.2
		sx := float64(vw) / (float64(sw) * extendedScale)
		sy := float64(vh) / (float64(sh) * extendedScale)
		s := sx
		if sy < sx {
			s = sy
		}

		// Calculate dimensions for the extended area
		extendedW := int(float64(sw) * s * extendedScale)
		extendedH := int(float64(sh) * s * extendedScale)
		offX := (vw - extendedW) / 2
		offY := topUIH + (vh-extendedH)/2

		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(s*e.cameraZoom, s*e.cameraZoom)
		op.GeoM.Translate(float64(offX)+e.cameraX, float64(offY)+e.cameraY)
		screen.DrawImage(e.bg, op)

		// Draw border frame showing the actual map boundaries (0-1 area)
		mapBorderW := int(float64(extendedW) / extendedScale)
		mapBorderH := int(float64(extendedH) / extendedScale)
		borderOffX := offX + (extendedW-mapBorderW)/2
		borderOffY := offY + (extendedH-mapBorderH)/2

		// Apply configurable frame position and scale
		frameBorderW := int(float64(mapBorderW) * e.frameScale)
		frameBorderH := int(float64(mapBorderH) * e.frameScale)
		frameOffX := borderOffX + int(float64(mapBorderW)*(e.frameX-0.5)*e.frameScale) + (mapBorderW-frameBorderW)/2
		frameOffY := borderOffY + int(float64(mapBorderH)*(e.frameY-0.5)*e.frameScale) + (mapBorderH-frameBorderH)/2

		// Draw border frame with thick lines
		borderColor := color.NRGBA{255, 255, 255, 255}
		borderThickness := 6

		// Top border
		ebitenutil.DrawRect(screen, float64(frameOffX), float64(frameOffY), float64(frameBorderW), float64(borderThickness), borderColor)
		// Bottom border
		ebitenutil.DrawRect(screen, float64(frameOffX), float64(frameOffY+frameBorderH-borderThickness), float64(frameBorderW), float64(borderThickness), borderColor)
		// Left border
		ebitenutil.DrawRect(screen, float64(frameOffX), float64(frameOffY), float64(borderThickness), float64(frameBorderH), borderColor)
		// Right border
		ebitenutil.DrawRect(screen, float64(frameOffX+frameBorderW-borderThickness), float64(frameOffY), float64(borderThickness), float64(frameBorderH), borderColor)

		// Draw frame manipulation handles when not dragging and Frame layer is active
		if !e.frameDragging && e.currentLayer == 1 {
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

		// overlays
		toX := func(nx float64) int { return borderOffX + int(nx*float64(mapBorderW)) }
		toY := func(ny float64) int { return borderOffY + int(ny*float64(mapBorderH)) }
		if e.showGrid {
			// draw a light 10% grid
			for i := 1; i < 10; i++ {
				x := toX(float64(i) / 10.0)
				y := toY(float64(i) / 10.0)
				ebitenutil.DrawLine(screen, float64(x), float64(offY), float64(x), float64(offY+extendedH), color.NRGBA{60, 60, 70, 120})
				ebitenutil.DrawLine(screen, float64(offX), float64(y), float64(offX+extendedW), float64(y), color.NRGBA{60, 60, 70, 120})
			}
		}
		// Draw layers based on current layer or show all layers
		showDeploy := e.showAllLayers || e.currentLayer == 2 || e.currentLayer == 7
		showBG := e.showAllLayers || e.currentLayer == 0 || e.currentLayer == 7
		showLanes := e.showAllLayers || e.currentLayer == 3 || e.currentLayer == 7
		showObstacles := e.showAllLayers || e.currentLayer == 4 || e.currentLayer == 7
		showDecorative := e.showAllLayers || e.currentLayer == 5 || e.currentLayer == 7
		showBases := e.showAllLayers || e.currentLayer == 6 || e.currentLayer == 7

		// Draw deploy zones if current layer is 2 (Deploy) or show all layers
		if showDeploy {
			for i, r := range e.def.DeployZones {
				x := toX(r.X)
				y := toY(r.Y)
				rw := int(r.W * float64(mapBorderW))
				rh := int(r.H * float64(mapBorderH))
				ebitenutil.DrawRect(screen, float64(x), float64(y), float64(rw), float64(rh), color.NRGBA{60, 150, 90, 90})
				if e.selKind == "deploy" && e.selIndex == i {
					// border
					ebitenutil.DrawRect(screen, float64(x), float64(y), float64(rw), 1, color.NRGBA{240, 196, 25, 255})
					ebitenutil.DrawRect(screen, float64(x), float64(y+rh-1), float64(rw), 1, color.NRGBA{240, 196, 25, 255})
					ebitenutil.DrawRect(screen, float64(x), float64(y), 1, float64(rh), color.NRGBA{240, 196, 25, 255})
					ebitenutil.DrawRect(screen, float64(x+rw-1), float64(y), 1, float64(rh), color.NRGBA{240, 196, 25, 255})
					// handles - make them larger to match hit areas
					handles := [][2]int{{x, y}, {x + rw, y}, {x + rw, y + rh}, {x, y + rh}}
					for _, h := range handles {
						ebitenutil.DrawRect(screen, float64(h[0]-6), float64(h[1]-6), 12, 12, color.NRGBA{240, 196, 25, 255})
					}
				}
			}
		}
		// Draw meeting stones and gold mines if BG layer is active (layer 0) or show all layers
		if showBG {
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
		}
		// Draw lanes if current layer is 3 (Lanes) or show all layers
		if showLanes {
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
		}
		// Draw obstacles if current layer is 4 (Obstacles) or show all layers
		if showObstacles {
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
						ebitenutil.DrawRect(screen, float64(h[0]-6), float64(h[1]-6), 12, 12, color.NRGBA{240, 196, 25, 255})
					}
				}
			}
		}

		// Draw decorative elements if current layer is 5 (Assets) or show all layers
		if showDecorative {
			for i, dec := range e.def.DecorativeElements {
				x := toX(dec.X)
				y := toY(dec.Y)
				w := int(dec.Width * float64(dispW))
				h := int(dec.Height * float64(dispH))

				// Try to draw the actual decorative image if available
				drewImage := false
				if dec.Image != "" {
					// Try multiple paths for decorative images
					decorativePaths := []string{
						filepath.Join("..", "..", "client", "internal", "game", "assets", "obstacles", dec.Image), // From mapeditor to client assets
						filepath.Join("decorative", dec.Image),                                                    // Local decorative directory
						dec.Image,                                                                                 // Original path
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

				if e.selKind == "decorative" && e.selIndex == i {
					// border
					ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), 1, color.NRGBA{240, 196, 25, 255})
					ebitenutil.DrawRect(screen, float64(x), float64(y+h-1), float64(w), 1, color.NRGBA{240, 196, 25, 255})
					ebitenutil.DrawRect(screen, float64(x), float64(y), 1, float64(h), color.NRGBA{240, 196, 25, 255})
					ebitenutil.DrawRect(screen, float64(x+w-1), float64(y), 1, float64(h), color.NRGBA{240, 196, 25, 255})
					// handles
					handles := [][2]int{{x, y}, {x + w, y}, {x + w, y + h}, {x, y + h}}
					for _, h := range handles {
						ebitenutil.DrawRect(screen, float64(h[0]-6), float64(h[1]-6), 12, 12, color.NRGBA{240, 196, 25, 255})
					}
				}
			}
		}
		// Draw player base (if Bases layer is active or show all layers)
		if showBases && (e.def.PlayerBase.X != 0 || e.def.PlayerBase.Y != 0) {
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
		// Draw enemy base (if Bases layer is active or show all layers)
		if showBases && (e.def.EnemyBase.X != 0 || e.def.EnemyBase.Y != 0) {
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

	// Layer selection buttons (center - more accessible)
	layerNames := []string{"BG", "Frame", "Deploy", "Lanes", "Obstacles", "Assets", "Player Bases", "All"}
	layerColors := []color.NRGBA{
		{0x6a, 0x6a, 0x7a, 0xff}, // Gray for BG
		{0xff, 0xff, 0x00, 0xff}, // Yellow for Frame
		{0x4a, 0x9e, 0xff, 0xff}, // Blue for Deploy
		{0x4a, 0xff, 0x7a, 0xff}, // Green for Lanes
		{0x8b, 0x45, 0x13, 0xff}, // Brown for Obstacles
		{0xff, 0x6b, 0x6b, 0xff}, // Pink for Assets
		{0xff, 0x8a, 0x4a, 0xff}, // Orange for Bases
		{0x8a, 0x8a, 0x9a, 0xff}, // Gray for All (matching other buttons)
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

	// Map name input (top right, under background info)
	mapNameY := 72
	mapNameInputW := 200
	mapNameInputX := vw - mapNameInputW - 8
	mapNameX := mapNameInputX - 40
	text.Draw(screen, "Map:", basicfont.Face7x13, mapNameX, mapNameY+16, color.White)
	mapNameInputY := mapNameY

	// Draw map name input box
	ebitenutil.DrawRect(screen, float64(mapNameInputX), float64(mapNameInputY), float64(mapNameInputW), 24, color.NRGBA{40, 40, 50, 255})
	if e.nameFocus {
		ebitenutil.DrawRect(screen, float64(mapNameInputX), float64(mapNameInputY), float64(mapNameInputW), 24, color.NRGBA{60, 60, 80, 255})
	}
	nm := e.name
	if nm == "" {
		nm = "New Map"
	}
	text.Draw(screen, nm, basicfont.Face7x13, mapNameInputX+6, mapNameInputY+16, color.White)
	if e.nameFocus {
		text.Draw(screen, "|", basicfont.Face7x13, mapNameInputX+6+len(nm)*7, mapNameInputY+16, color.White)
	}

	// Top buttons (left side)
	saveX := 450
	saveY := 8
	ebitenutil.DrawRect(screen, float64(saveX), float64(saveY), 100, 24, color.NRGBA{70, 110, 70, 255})
	text.Draw(screen, "Save", basicfont.Face7x13, saveX+30, saveY+16, color.White)
	// Clear selection button
	clrX := saveX + 110
	clrY := saveY
	ebitenutil.DrawRect(screen, float64(clrX), float64(clrY), 80, 24, color.NRGBA{90, 70, 70, 255})
	text.Draw(screen, "Clear", basicfont.Face7x13, clrX+20, clrY+16, color.White)

	// Assets button for decorative elements
	assetsX := clrX + 90
	assetsY := clrY
	ebitenutil.DrawRect(screen, float64(assetsX), float64(assetsY), 80, 24, color.NRGBA{120, 90, 120, 255})
	text.Draw(screen, "Assets", basicfont.Face7x13, assetsX+15, assetsY+16, color.White)

	// Load Map button
	loadX := assetsX + 90
	loadY := assetsY
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
		if mx >= assetsX && mx < assetsX+80 && my >= assetsY && my < assetsY+24 {
			e.showDecorativeBrowser = !e.showDecorativeBrowser
			if e.showDecorativeBrowser {
				e.refreshDecorativeBrowser()
			}
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

	// Background info display (top left)
	bx := 8
	by := 48
	if e.bg != nil && e.bgPath != "" {
		filename := filepath.Base(e.bgPath)
		text.Draw(screen, "BG: "+filename, basicfont.Face7x13, bx, by+16, color.White)
	}
	// buttons
	btn := func(x int, label string) (int, int, int, int) {
		w := 64
		h := 24
		ebitenutil.DrawRect(screen, float64(x), float64(by), float64(w), float64(h), color.NRGBA{60, 90, 120, 255})
		text.Draw(screen, label, basicfont.Face7x13, x+10, by+16, color.White)
		return x, by, w, h
	}
	sx, sy, sw, sh := btn(bx+270, "Set")
	ax, ay, aw, ah := btn(bx+270+70, "BG")
	// focus input when clicking the box
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		if mx >= bx && mx < bx+260 && my >= by && my < by+24 {
			e.bgFocus = true
		} else if !(mx >= sx && mx < sx+sw && my >= sy && my < sy+sh) {
			e.bgFocus = false
		}
		if mx >= sx && mx < sx+sw && my >= sy && my < sy+sh { // Set MapDef Bg
			if e.bgPath != "" {
				e.def.Bg = filepath.Base(e.bgPath)
				// Calculate the actual scale factor that should be used in the game
				// The map editor uses extended scaling (1.2x) for camera space, but game uses normal scaling
				vw, vh := ebiten.WindowSize()
				vh -= topUIH
				sw, sh := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
				sx := float64(vw) / float64(sw)
				sy := float64(vh) / float64(sh)
				baseScale := sx
				if sy < sx {
					baseScale = sy
				}
				// Save the base scale (what the game should use) multiplied by current zoom
				e.def.BgScale = baseScale * e.cameraZoom
				e.def.BgOffsetX = e.cameraX
				e.def.BgOffsetY = e.cameraY
				e.status = fmt.Sprintf("BG set in map: %s", filepath.Base(e.bgPath))
			} else {
				e.status = "No background loaded to set"
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

	// Show live normalized mouse coordinates (bottom right corner)
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
			// Draw coordinates in bottom right corner
			text.Draw(screen, fmt.Sprintf("(%.3f, %.3f)", nx, ny), basicfont.Face7x13, coordX, coordY, color.NRGBA{200, 200, 210, 255})
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

	// Decorative elements browser panel (draw)
	if e.showDecorativeBrowser {
		vw, vh := ebiten.WindowSize()
		panelX := vw/2 - 200
		panelY := vh/2 - 150
		panelW := 400
		panelH := 300
		ebitenutil.DrawRect(screen, float64(panelX), float64(panelY), float64(panelW), float64(panelH), color.NRGBA{20, 20, 30, 240})
		ebitenutil.DrawRect(screen, float64(panelX+2), float64(panelY+2), float64(panelW-4), 24, color.NRGBA{40, 40, 60, 255})
		text.Draw(screen, "Load Decorative Image", basicfont.Face7x13, panelX+10, panelY+18, color.White)

		// Close button
		closeX := panelX + panelW - 30
		closeY := panelY + 2
		ebitenutil.DrawRect(screen, float64(closeX), float64(closeY), 28, 20, color.NRGBA{100, 60, 60, 255})
		text.Draw(screen, "X", basicfont.Face7x13, closeX+10, closeY+15, color.White)

		rowH := 20
		maxRows := (panelH - 60) / rowH
		start := e.decorativeBrowserScroll
		for i := 0; i < maxRows && start+i < len(e.availableDecorative); i++ {
			yy := panelY + 30 + i*rowH
			if (start+i)%2 == 0 {
				ebitenutil.DrawRect(screen, float64(panelX+4), float64(yy-12), float64(panelW-8), 18, color.NRGBA{40, 40, 56, 255})
			}
			decorativePath := e.availableDecorative[start+i]
			show := filepath.Base(decorativePath)
			var acol color.Color = color.White
			if e.decorativeBrowserSel == start+i {
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

		// Handle decorative browser interactions
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if mx >= closeX && mx < closeX+28 && my >= closeY && my < closeY+20 {
				e.showDecorativeBrowser = false
			} else if mx >= loadBtnX && mx < loadBtnX+80 && my >= loadBtnY && my < loadBtnY+24 {
				if e.decorativeBrowserSel >= 0 && e.decorativeBrowserSel < len(e.availableDecorative) {
					item := e.availableDecorative[e.decorativeBrowserSel]
					if item == ".." {
						// go up
						parent := filepath.Dir(e.decorativeCurrentPath)
						e.decorativeCurrentPath = parent
						e.refreshDecorativeBrowser()
					} else if strings.HasPrefix(item, "[DIR] ") {
						// enter directory
						dirName := strings.TrimPrefix(item, "[DIR] ")
						newPath := filepath.Join(e.decorativeCurrentPath, dirName)
						e.decorativeCurrentPath = newPath
						e.refreshDecorativeBrowser()
					} else {
						// load file as decorative element
						fullPath := filepath.Join(e.decorativeCurrentPath, item)
						if _, _, err := ebitenutil.NewImageFromFile(fullPath); err == nil {
							if e.selKind == "decorative" && e.selIndex >= 0 && e.selIndex < len(e.def.DecorativeElements) {
								// Change image of selected decorative element
								e.def.DecorativeElements[e.selIndex].Image = filepath.Base(fullPath)
								e.status = fmt.Sprintf("Decorative element image changed: %s", filepath.Base(fullPath))
							} else {
								// Create new decorative element at center of canvas
								nx, ny := 0.5, 0.5 // center
								dec := protocol.DecorativeElement{
									X:      nx - 0.05,
									Y:      ny - 0.05,
									Image:  filepath.Base(fullPath),
									Width:  0.1,
									Height: 0.1,
									Layer:  1, // middle layer
								}
								e.def.DecorativeElements = append(e.def.DecorativeElements, dec)
								e.selKind, e.selIndex, e.selHandle = "decorative", len(e.def.DecorativeElements)-1, -1
								e.status = fmt.Sprintf("Decorative element added: %s", filepath.Base(fullPath))
							}
							e.showDecorativeBrowser = false
						} else {
							e.status = fmt.Sprintf("Failed to load decorative image: %v", err)
						}
					}
				}
			} else if mx >= panelX+4 && mx < panelX+panelW-4 && my >= panelY+30 && my < panelY+panelH-40 {
				idx := (my-(panelY+30))/rowH + start
				if idx >= 0 && idx < len(e.availableDecorative) {
					e.decorativeBrowserSel = idx
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

	// Draw status log window (movable, 30% width)
	if e.showStatusLog {
		// Set default position if not set
		if e.statusLogY == 0 {
			e.statusLogY = vh - 200
		}

		chatX := e.statusLogX
		chatY := e.statusLogY
		chatW := int(float64(vw) * 0.3) // 30% of workspace width
		chatH := 180

		// Handle dragging
		if e.statusLogDragging {
			if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
				// Update position based on mouse movement
				newX := mx - e.statusLogDragStartX
				newY := my - e.statusLogDragStartY

				// Keep window within screen bounds (allow dragging to full window height)
				if newX < 0 {
					newX = 0
				}
				if newX+chatW > vw {
					newX = vw - chatW
				}
				if newY < 0 {
					newY = 0
				}
				if newY+chatH > vh {
					newY = vh - chatH
				}

				e.statusLogX = newX
				e.statusLogY = newY
			} else {
				// Stop dragging when mouse button is released
				e.statusLogDragging = false
			}
		}

		// Background with border
		ebitenutil.DrawRect(screen, float64(chatX-2), float64(chatY-2), float64(chatW+4), float64(chatH+4), color.NRGBA{60, 60, 70, 255})
		ebitenutil.DrawRect(screen, float64(chatX), float64(chatY), float64(chatW), float64(chatH), color.NRGBA{20, 20, 30, 220})

		// Title bar
		ebitenutil.DrawRect(screen, float64(chatX), float64(chatY), float64(chatW), 20, color.NRGBA{40, 40, 60, 255})
		text.Draw(screen, "Status Log", basicfont.Face7x13, chatX+8, chatY+14, color.NRGBA{200, 200, 220, 255})

		// Minimize button
		minBtnX := chatX + chatW - 30
		minBtnY := chatY + 2
		ebitenutil.DrawRect(screen, float64(minBtnX), float64(minBtnY), 26, 16, color.NRGBA{100, 60, 60, 255})
		text.Draw(screen, "-", basicfont.Face7x13, minBtnX+10, minBtnY+12, color.White)

		// Handle mouse interactions
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			// Check minimize button first
			if mx >= minBtnX && mx < minBtnX+26 && my >= minBtnY && my < minBtnY+16 {
				e.showStatusLog = false
			} else if mx >= chatX && mx < chatX+chatW && my >= chatY && my < chatY+20 {
				// Start dragging from title bar (excluding minimize button area)
				e.statusLogDragging = true
				e.statusLogDragStartX = mx - chatX
				e.statusLogDragStartY = my - chatY
			}
		}

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
	} else {
		// Draw maximize button when minimized
		maxBtnX := 8
		maxBtnY := vh - 30
		ebitenutil.DrawRect(screen, float64(maxBtnX), float64(maxBtnY), 80, 24, color.NRGBA{60, 90, 120, 255})
		text.Draw(screen, "Status Log", basicfont.Face7x13, maxBtnX+8, maxBtnY+16, color.White)

		// Handle maximize button click
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			if mx >= maxBtnX && mx < maxBtnX+80 && my >= maxBtnY && my < maxBtnY+24 {
				e.showStatusLog = true
			}
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
		showLogin:             tok == "", // Show login if no token
		assetsCurrentPath:     filepath.Join(projectRoot, "client", "internal", "game", "assets", "maps"),
		obstaclesCurrentPath:  filepath.Join(projectRoot, "client", "internal", "game", "assets", "obstacles"),
		decorativeCurrentPath: filepath.Join(projectRoot, "client", "internal", "game", "assets", "decorative"),
		statusMessages:        []string{},
		maxStatusMessages:     10,
		showStatusLog:         true,
		statusLogX:            8,
		statusLogY:            0, // Will be set dynamically
		statusLogDragging:     false,
		// Initialize layer system
		currentLayer:  0, // Start with BG layer
		showAllLayers: false,
		// Initialize base management
		playerBaseExists: false,
		enemyBaseExists:  false,
		// Initialize camera for map editor
		cameraX:        0,
		cameraY:        0,
		cameraZoom:     1.0,
		cameraMinZoom:  0.1,
		cameraMaxZoom:  2.0,
		cameraDragging: false,
		// Initialize frame for map editor
		frameX:        0.5,
		frameY:        0.5,
		frameWidth:    1.0,
		frameHeight:   1.0,
		frameScale:    1.0,
		frameDragging: false,
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
