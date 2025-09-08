package main

import (
	"fmt"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// handleEditorInput handles all input for the main editor interface
func (e *editor) handleEditorInput() error {
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

	// Enhanced toolbar click handling
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		// Check if clicking on toolbar buttons first
		toolbarClicked := false

		// Handle layer selection buttons
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

	return nil
}
