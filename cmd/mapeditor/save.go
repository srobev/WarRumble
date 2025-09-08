package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"rumble/shared/protocol"

	"github.com/gorilla/websocket"
	"github.com/hajimehoshi/ebiten/v2"
)

// save handles saving the current map both locally and to the server
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

	// Save background positioning and scaling to preserve current camera state
	if e.bg != nil {
		// Calculate the base scale factor for the game (map editor uses extended scaling)
		sw, sh := e.bg.Bounds().Dx(), e.bg.Bounds().Dy()
		vw, vh := ebiten.WindowSize()
		vh -= 120 // Account for UI height
		sx := float64(vw) / float64(sw)
		sy := float64(vh) / float64(sh)
		baseScale := sx
		if sy < sx {
			baseScale = sy
		}

		// Save background scale and position from current camera state
		e.def.BgScale = baseScale * e.cameraZoom
		e.def.BgOffsetX = e.cameraX
		e.def.BgOffsetY = e.cameraY
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
