package game

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"math/rand"
	"rumble/client/internal/netcfg"
	"rumble/shared/protocol"
	"strings"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

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
		particleSystem:   NewParticleSystem(),

		connCh: make(chan connResult, 4),
		connSt: stateConnected,
	}

	// Initialize fullscreen state
	g.fullscreen = false

	// Initialize Fantasy UI System
	g.fantasyUI = NewFantasyUI(DefaultFantasyTheme())

	// Initialize new interaction defaults
	g.slotDragFrom = -1

	// Initialize camera system - start with 20% zoom in
	g.cameraX = 0
	g.cameraY = 0
	g.cameraZoom = 1.2    // Start with 20% zoom in
	g.cameraMinZoom = 1.2 // Can't zoom out beyond 20% (prevents seeing too much)
	g.cameraMaxZoom = 1.4 // Allow zooming in up to 300%
	g.cameraDragging = false
	g.cameraDragStartX = 0
	g.cameraDragStartY = 0
	g.cameraDragInitialX = 0
	g.cameraDragInitialY = 0

	if !HasToken() {

		apiBase := netcfg.APIBase
		g.auth = NewAuthUI(apiBase, func(username string) {
			g.name = username

		})
		g.connSt = stateIdle
	} else {
		// Validate the existing token before attempting to connect
		if err := ValidateToken(); err != nil {
			// Token is invalid (likely version mismatch), clear it and show login
			ClearToken()
			ClearUsername()

			apiBase := netcfg.APIBase
			g.auth = NewAuthUI(apiBase, func(username string) {
				g.name = username
			})
			g.connSt = stateIdle
		} else {
			// Token is valid, proceed with auto-connect
			g.name = LoadUsername()
			g.retryConnect()
		}
	}
	return g
}

func (g *Game) Update() error {

	if g.auth != nil {
		g.auth.Update()
		if g.auth.Done() {
			user := g.auth.Username()
			g.auth = nil
			g.name = user
			g.retryConnect()
		}
		return nil
	}

	if g.connSt == stateFailed && time.Now().After(g.connRetryAt) {
		if !g.connectInFlight {
			g.connSt = stateConnecting
			g.connErrMsg = ""
			g.connRetryAt = time.Now().Add(2 * time.Second)
			g.connectInFlight = true
			go g.connectAsync()
		}
	}

	select {
	case res := <-g.connCh:
		if res.err != nil {
			g.connSt = stateFailed
			g.connErrMsg = res.err.Error()
			g.connRetryAt = time.Now().Add(2 * time.Second)
			break
		}
		g.net = res.n
		g.connSt = stateConnected

		if strings.TrimSpace(g.name) == "" {
			g.name = "Player"
		}
		g.send("SetName", protocol.SetName{Name: g.name})
		g.send("GetProfile", protocol.GetProfile{})
		g.send("ListMinis", protocol.ListMinis{})
		g.send("ListMaps", protocol.ListMaps{})
		g.send("GetGuild", protocol.GetGuild{})
		g.send("GetFriends", protocol.GetFriends{})

		g.activeTab = tabArmy
		g.requestLobbyDataOnce()
		g.lobbyRequested = true
		g.scr = screenHome

		// Clear particle effects when transitioning to home screen
		if g.particleSystem != nil {
			g.particleSystem = NewParticleSystem()
		}

	default:
	}

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

afterMessages:

	if g.profileOpen && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if g.profCloseBtn.hit(mx, my) {
			g.profileOpen = false
			g.profCloseBtn = rect{}
			g.profLogoutBtn = rect{}
		}
	}

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
			g.endActive = (myHP <= 0 || enemyHP <= 0 || g.timerRemainingSeconds <= 0)
			g.endVictory = (enemyHP <= 0 && myHP > 0) || (enemyHP <= 0 && g.timerRemainingSeconds <= 0 && myHP > 0)
		} else {
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

				if !g.ensureNet() {
					break
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
		// Update Fantasy UI system
		if g.fantasyUI != nil {
			g.fantasyUI.Update()
		}
		g.updateHome()
	case screenBattle:
		// Always update battle (for camera controls) but skip deployment when paused
		g.updateBattle()
		g.updateTimerInput()
	}

	// Skip world updates if game is paused
	if !g.timerPaused {
		g.world.LerpPositions()

		// Update unit animations
		for _, unit := range g.world.Units {
			if unit.AnimationData != nil {
				unit.AnimationData.UpdateAnimation(unit, 1.0/60.0)
			}
		}

		// Update particle system
		if g.particleSystem != nil {
			g.particleSystem.Update(1.0 / 60.0) // Assuming 60 FPS
		}
	}

	// Update timer countdown (every second)
	if g.scr == screenBattle && !g.timerPaused && !g.gameOver && !g.endActive {
		currentTime := time.Now().Unix()
		if g.lastTimerUpdate == 0 {
			g.lastTimerUpdate = currentTime
		}

		if currentTime > g.lastTimerUpdate {
			// Decrement timer by 1 second
			if g.timerRemainingSeconds > 0 {
				g.timerRemainingSeconds--
			}
			g.lastTimerUpdate = currentTime
		}
	}

	// Test particle effects with keyboard shortcuts (for development)
	// Works in battle screen and home screen for testing
	if g.scr == screenBattle || g.scr == screenHome {
		// Get mouse position for particle effects
		mouseX, mouseY := ebiten.CursorPosition()

		if inpututil.IsKeyJustPressed(ebiten.KeyE) {
			// Create explosion effect at mouse position
			if g.particleSystem != nil {
				g.particleSystem.CreateExplosionEffect(float64(mouseX), float64(mouseY), 1.0)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyS) {
			// Create spell effect at mouse position
			if g.particleSystem != nil {
				g.particleSystem.CreateSpellEffect(float64(mouseX), float64(mouseY), "fire")
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyH) {
			// Create healing effect at mouse position
			if g.particleSystem != nil {
				g.particleSystem.CreateHealingEffect(float64(mouseX), float64(mouseY))
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyA) {
			// Create aura effect at mouse position
			if g.particleSystem != nil {
				g.particleSystem.CreateAuraEffect(float64(mouseX), float64(mouseY), "buff")
			}
		}

		// New ability effect shortcuts
		if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
			// Healing ability at mouse position
			if g.particleSystem != nil {
				g.particleSystem.CreateUnitAbilityEffect(float64(mouseX), float64(mouseY), "heal")
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyW) {
			// Stun ability at mouse position
			if g.particleSystem != nil {
				g.particleSystem.CreateUnitAbilityEffect(float64(mouseX), float64(mouseY), "stun")
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			// Rage ability at mouse position
			if g.particleSystem != nil {
				g.particleSystem.CreateUnitAbilityEffect(float64(mouseX), float64(mouseY), "rage")
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyT) {
			// Teleport ability at mouse position and offset
			if g.particleSystem != nil {
				g.particleSystem.CreateUnitAbilityEffect(float64(mouseX-50), float64(mouseY), "teleport")
				time.Sleep(200 * time.Millisecond) // Small delay for effect
				g.particleSystem.CreateUnitAbilityEffect(float64(mouseX+50), float64(mouseY), "teleport")
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyY) {
			// Critical hit effect at mouse position
			if g.particleSystem != nil {
				g.particleSystem.CreateCriticalHitEffect(float64(mouseX), float64(mouseY))
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyU) {
			// Level up celebration at mouse position
			if g.particleSystem != nil {
				g.particleSystem.CreateLevelUpEffect(float64(mouseX), float64(mouseY))
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyI) {
			// Battle buff effect at mouse position
			if g.particleSystem != nil {
				g.particleSystem.CreateBattleBuffEffect(float64(mouseX), float64(mouseY), "attack")
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyP) {
			// Target healing effect at mouse position and offset
			if g.particleSystem != nil {
				// Healing wave from healer (left of mouse)
				healerX := float64(mouseX - 100)
				targetX := float64(mouseX + 100)
				targetY := float64(mouseY)

				// Healing wave from healer
				g.particleSystem.CreateUnitAbilityEffect(healerX, targetY, "heal")
				// Green particles on healed target
				g.particleSystem.CreateTargetHealingEffect(targetX, targetY)
			}
		}

		// Unit Animation Test Shortcuts
		if inpututil.IsKeyJustPressed(ebiten.KeyDigit1) {
			// Trigger hit animation on first unit
			for _, unit := range g.world.Units {
				if unit.AnimationData != nil {
					unit.AnimationData.TriggerHit()
					break
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyDigit2) {
			// Trigger attack animation on first unit
			for _, unit := range g.world.Units {
				if unit.AnimationData != nil {
					unit.AnimationData.TriggerAttack()
					break
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyDigit3) {
			// Trigger death animation on first unit
			for _, unit := range g.world.Units {
				if unit.AnimationData != nil {
					unit.AnimationData.TriggerDeath()
					break
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyDigit4) {
			// Trigger cast animation on first unit
			for _, unit := range g.world.Units {
				if unit.AnimationData != nil {
					unit.AnimationData.TriggerCast()
					break
				}
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyDigit5) {
			// Trigger defend animation on first unit
			for _, unit := range g.world.Units {
				if unit.AnimationData != nil {
					unit.AnimationData.TriggerDefend()
					break
				}
			}
		}
	}

	return nil
}

// ---------- Login ----------
func (g *Game) updateLogin() {

	for _, k := range inpututil.AppendJustPressedKeys(nil) {
		switch k {
		case ebiten.KeyEnter:

			g.name = strings.TrimSpace(g.nameInput)
			if g.name == "" {
				g.name = "Player"
			}
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

func (g *Game) Draw(screen *ebiten.Image) {
	// Clear screen to prevent rendering artifacts
	screen.Clear()

	if g.auth != nil {
		g.auth.Draw(screen)
		return
	}

	if g.connSt != stateConnected {

		ebitenutil.DrawRect(
			screen,
			0, 0,
			float64(protocol.ScreenW), float64(protocol.ScreenH),
			color.NRGBA{0, 0, 0, 140},
		)

		w, h := 420, 140
		x := (protocol.ScreenW - w) / 2
		y := (protocol.ScreenH - h) / 2
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), color.NRGBA{32, 32, 44, 255})

		if g.connSt == stateConnecting {
			text.Draw(screen, "Connecting to server...", basicfont.Face7x13, x+20, y+46, color.White)

		} else {
			text.Draw(screen, "Unable to connect to server", basicfont.Face7x13, x+20, y+46, color.NRGBA{255, 120, 120, 255})
			if g.connErrMsg != "" {
				text.Draw(screen, g.connErrMsg, basicfont.Face7x13, x+20, y+66, color.NRGBA{220, 200, 200, 255})
			}
			text.Draw(screen, "Press R (desktop) or Tap (mobile) to retry", basicfont.Face7x13, x+20, y+96, color.NRGBA{220, 220, 220, 255})
		}

		return
	}
	if !g.profileOpen {
		g.profCloseBtn = rect{}
		g.profLogoutBtn = rect{}
	}

	switch g.scr {
	case screenLogin:
		text.Draw(screen, "Enter username, then press Enter:", basicfont.Face7x13, pad, 48, color.White)
		ebitenutil.DrawRect(screen, float64(pad), 54, 260, 18, color.NRGBA{40, 40, 40, 255})
		text.Draw(screen, g.nameInput, basicfont.Face7x13, pad+4, 68, color.White)

	case screenHome:
		// Draw home screen background
		if g.activeTab == tabArmy {
			g.ensureArmyBgLayer()
			if g.armyBgLayer != nil {
				var op ebiten.DrawImageOptions
				op.GeoM.Translate(0, float64(topBarH))
				screen.DrawImage(g.armyBgLayer, &op)
			}
		}

		g.drawHomeContent(screen)
		g.drawTopBarHome(screen)
		g.drawBottomBar(screen)

		if g.showProfile {
			g.drawProfileOverlay(screen)
		}

	case screenBattle:
		nowMs := time.Now().UnixMilli()

		// Check if this is PvP (has exactly 2 bases with different owners)
		isPvP := false
		playerBaseY := 0.0
		playerBaseCount := 0
		enemyBaseCount := 0
		for _, b := range g.world.Bases {
			if b.OwnerID == g.playerID {
				playerBaseY = float64(b.Y)
				playerBaseCount++
			} else {
				enemyBaseCount++
			}
		}
		// Only consider it PvP if there are exactly 2 bases AND it's actually a PvP room
		// (PvE also has 2 bases but should not be mirrored)
		if playerBaseCount == 1 && enemyBaseCount == 1 && strings.Contains(g.roomID, "pvp-") {
			isPvP = true
		}

		// PvP Mirroring: Ensure both players see their base at bottom
		// Mirror if player's base is at the TOP (needs to be moved to bottom)
		// Only apply mirroring in actual PvP scenarios (2+ players)
		// PVE should never be mirrored
		shouldMirror := false
		if isPvP && g.currentMapDef != nil {
			shouldMirror = playerBaseY < float64(protocol.ScreenH)/2
		}

		// Draw battle arena background first
		g.drawArenaBG(screen)

		// Draw deploy zones (alpha rectangles) - apply camera transformation
		g.drawDeployZones(screen, shouldMirror)

		// Helper function to mirror Y coordinate
		mirrorY := func(y float64) float64 {
			if shouldMirror {
				return float64(protocol.ScreenH) - y
			}
			return y
		}

		// Draw bases
		for id, b := range g.world.Bases {
			g.assets.ensureInit()
			img := g.assets.baseEnemy
			if b.OwnerID == g.playerID {
				img = g.assets.baseMe
			}

			// Apply mirroring to base position for PvP (only player's own base)
			renderBaseX := b.X
			renderBaseY := b.Y
			if shouldMirror && b.OwnerID == g.playerID {
				renderBaseY = int(mirrorY(float64(b.Y)))
			}

			// Apply camera transformations
			renderBaseX = int(float64(renderBaseX)*g.cameraZoom + g.cameraX)
			renderBaseY = int(float64(renderBaseY)*g.cameraZoom + g.cameraY)

			// Create base with transformed position for rendering
			renderBase := protocol.BaseState{
				X:       renderBaseX,
				Y:       renderBaseY,
				W:       int(float64(b.W) * g.cameraZoom),
				H:       int(float64(b.H) * g.cameraZoom),
				HP:      b.HP,
				MaxHP:   b.MaxHP,
				OwnerID: b.OwnerID,
			}
			drawBaseImg(screen, img, renderBase)

			if b.MaxHP > 0 {
				fx := g.hpfxStep(g.hpFxBases, id, b.HP, nowMs)
				isPlayer := (b.OwnerID == g.playerID)
				x := float64(renderBaseX)
				y := float64(renderBaseY - 6) // Fixed size, not scaled
				w := float64(b.W)             // Fixed size, not scaled
				h := 4.0                      // Fixed size, not scaled
				g.DrawHPBarForOwner(screen, x, y, w, h, b.HP, b.MaxHP, fx.ghostHP, fx.healGhostHP, isPlayer)
				// Base level badge left of bar
				rx0 := int(x + 0.5)
				ry0 := int(y + 0.5)
				rx1 := int(x + w + 0.5)
				ry1 := int(y + h + 0.5)
				barRect := image.Rect(rx0, ry0, rx1, ry1)
				lvl := 1
				if isPlayer {
					lvl = g.currentArmyRoundedLevel()
				}
				g.drawLevelBadge(screen, barRect, lvl)
			}
		}

		// Draw projectiles (ON TOP of bases but behind units) with consistent mirroring
		g.drawProjectiles(screen, shouldMirror, mirrorY, g.playerID)

		// Draw obstacles
		g.drawObstacles(screen, shouldMirror, mirrorY)

		// Update spawn animations
		if g.world != nil {
			g.world.UpdateSpawnAnimations()
		}

		// Draw units
		const unitTargetPX = 42.0

		// Draw spawn animations (behind units)
		if g.world != nil {
			for _, animation := range g.world.SpawnAnimations {
				if !animation.Active {
					continue
				}

				// Calculate current position based on animation progress
				currentX := animation.StartX + (animation.TargetX-animation.StartX)*animation.Progress
				currentY := animation.StartY + (animation.TargetY-animation.StartY)*animation.Progress

				// Apply mirroring to animation position
				renderX := currentX
				renderY := currentY
				if shouldMirror {
					renderY = float64(protocol.ScreenH) - currentY
				}

				// Apply camera transformations
				renderX = renderX*g.cameraZoom + g.cameraX
				renderY = renderY*g.cameraZoom + g.cameraY

				// Get unit image for animation
				if img := g.ensureMiniImageByName(animation.UnitName); img != nil {
					op := &ebiten.DrawImageOptions{}
					iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
					s := unitTargetPX / float64(maxInt(1, maxInt(iw, ih))) * g.cameraZoom * animation.CurrentScale
					op.GeoM.Scale(s, s)
					op.GeoM.Translate(renderX-float64(iw)*s/2, renderY-float64(ih)*s/2)
					screen.DrawImage(img, op)
				} else {
					// Fallback: draw a scaled rectangle
					size := 12 * g.cameraZoom * animation.CurrentScale
					ebitenutil.DrawRect(screen, renderX-size/2, renderY-size/2, size, size, color.White)
				}
			}
		}
		for id, u := range g.world.Units {
			// Check if this unit has an active spawn animation
			hasSpawnAnimation := false
			if g.world != nil {
				if animation := g.world.GetActiveSpawnAnimation(id); animation != nil {
					hasSpawnAnimation = true
				}
			}

			// Skip drawing actual unit if it has an active spawn animation
			if hasSpawnAnimation {
				continue
			}

			// Apply mirroring to unit position (only player's own units)
			renderX := u.X
			renderY := u.Y
			if shouldMirror && u.OwnerID == g.playerID {
				renderY = float64(protocol.ScreenH) - u.Y
			}

			// Apply camera transformations
			renderX = renderX*g.cameraZoom + g.cameraX
			renderY = renderY*g.cameraZoom + g.cameraY

			if img := g.ensureMiniImageByName(u.Name); img != nil {
				op := &ebiten.DrawImageOptions{}
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
				s := unitTargetPX / float64(maxInt(1, maxInt(iw, ih))) * g.cameraZoom

				// Apply animation transforms if available
				if u.AnimationData != nil {
					// Apply animation transforms first (center of original image)
					u.AnimationData.ApplyTransforms(op, float64(iw)/2, float64(ih)/2)

					// Then apply base scaling and positioning
					op.GeoM.Scale(s, s)
					op.GeoM.Translate(renderX-float64(iw)*s/2, renderY-float64(ih)*s/2)

					// Glow effects removed to eliminate white dots

					// Movement trail particles removed to eliminate white dots
				} else {
					// No animation data, use basic positioning
					op.GeoM.Scale(s, s)
					op.GeoM.Translate(renderX-float64(iw)*s/2, renderY-float64(ih)*s/2)
				}

				screen.DrawImage(img, op)
			} else {
				// Fallback: draw a subtle gray dot for missing unit images
				ebitenutil.DrawRect(screen, renderX-3*g.cameraZoom, renderY-3*g.cameraZoom, 6*g.cameraZoom, 6*g.cameraZoom, color.NRGBA{100, 100, 100, 128})
			}

			if u.MaxHP > 0 {
				lvl := g.levelForUnitName(u.Name)
				isFullHealth := u.HP >= u.MaxHP

				if isFullHealth {
					// Full health: Show level badge above unit head (same as damaged units)
					barW := 26.0 * 1.05 // Fixed size, not scaled
					bx := renderX - barW/2
					by := renderY - unitTargetPX/2 - 6 // Fixed size, not scaled

					// Level badge above unit (same style as damaged units)
					rx0 := int(bx + 0.5)
					ry0 := int(by + 0.5)
					rx1 := int(bx + barW + 0.5)
					ry1 := int(by + 3 + 0.5) // Fixed size, not scaled
					barRect := image.Rect(rx0, ry0, rx1, ry1)
					g.drawLevelBadge(screen, barRect, lvl)
				} else {
					// Damaged: Show health bar with level badge
					barW := 26.0 * 1.05 // Fixed size, not scaled
					bx := renderX - barW/2
					by := renderY - unitTargetPX/2 - 6 // Fixed size, not scaled

					fx := g.hpfxStep(g.hpFxUnits, id, u.HP, nowMs)
					g.DrawHPBarForOwner(screen, bx, by, barW, 3, u.HP, u.MaxHP, fx.ghostHP, fx.healGhostHP, u.OwnerID == g.playerID) // Fixed size, not scaled
					// Level badge left of HP bar
					rx0 := int(bx + 0.5)
					ry0 := int(by + 0.5)
					rx1 := int(bx + barW + 0.5)
					ry1 := int(by + 3 + 0.5) // Fixed size, not scaled
					barRect := image.Rect(rx0, ry0, rx1, ry1)
					g.drawLevelBadge(screen, barRect, lvl)
				}
			}
		}

		// Draw battle UI
		g.drawBattleBar(screen)

		myCur, myMax, enCur, enMax := g.battleHPs()
		g.drawBattleTopBars(screen, myCur, myMax, enCur, enMax)

		// Draw particle effects (after UI, before victory/defeat overlay)
		if g.particleSystem != nil {
			// Apply mirroring to particle system if needed
			if shouldMirror {
				// Create a temporary mirrored particle system for rendering
				g.drawMirroredParticles(screen, mirrorY, g.cameraX, g.cameraY, g.cameraZoom)
			} else {
				g.drawParticlesWithCamera(screen, g.cameraX, g.cameraY, g.cameraZoom)
			}
		}
	}

	if g.scr == screenBattle && (g.endActive || g.gameOver) {
		overlay := ebiten.NewImage(protocol.ScreenW, protocol.ScreenH)
		overlay.Fill(color.NRGBA{0, 0, 0, 140})
		screen.DrawImage(overlay, nil)

		w, h := 360, 160
		x := (protocol.ScreenW - w) / 2
		y := (protocol.ScreenH - h) / 2
		ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), color.NRGBA{30, 30, 45, 240})

		title := "Defeat"
		if g.endVictory || g.victory {
			title = "Victory!"
		}
		text.Draw(screen, title, basicfont.Face7x13, x+20, y+40, color.White)

		// XP gains list (if computed)
		if g.xpGains != nil {
			names := g.battleArmy
			if len(names) == 0 {
				if g.selectedChampion != "" {
					names = append(names, g.selectedChampion)
				}
				for i := 0; i < 6; i++ {
					if g.selectedOrder[i] != "" {
						names = append(names, g.selectedOrder[i])
					}
				}
			}
			// Reorder so the champion stays first and minis with +XP are shown next
			if len(names) > 0 {
				champ := names[0]
				pos := make([]string, 0, 6)
				zero := make([]string, 0, 6)
				for _, n := range names[1:] {
					if g.xpGains[n] > 0 {
						pos = append(pos, n)
					} else {
						zero = append(zero, n)
					}
				}
				// Keep relative order among positives/zeros stable
				names = append([]string{champ}, append(pos, zero...)...)
			}
			yy := y + 62
			for _, n := range names {
				// small portrait
				if img := g.ensureMiniImageByName(n); img != nil {
					iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
					s := 20.0 / float64(maxInt(1, maxInt(iw, ih)))
					op := &ebiten.DrawImageOptions{}
					op.GeoM.Scale(s, s)
					op.GeoM.Translate(float64(x+14), float64(yy-14))
					screen.DrawImage(img, op)
				}
				d := g.xpGains[n]
				line := n
				if d > 0 {
					line += fmt.Sprintf("  +%d XP", d)
				} else {
					line += "  +0 XP"
				}
				text.Draw(screen, line, basicfont.Face7x13, x+40, yy, color.White)
				yy += 18
				if yy > y+h-64 {
					break
				}
			}
		}

		continueBtnX := x + (w-120)/2
		continueBtnY := y + h - 50
		ebitenutil.DrawRect(screen, float64(continueBtnX), float64(continueBtnY), 120, 32, color.NRGBA{70, 110, 70, 255})
		text.Draw(screen, "Continue", basicfont.Face7x13, continueBtnX+18, continueBtnY+20, color.White)

		// Store continue button rect for click detection
		g.continueBtn = rect{x: int(continueBtnX), y: int(continueBtnY), w: 120, h: 32}

		// Handle continue button click
		if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
			mx, my := ebiten.CursorPosition()
			if g.continueBtn.hit(mx, my) {
				g.onLeaveRoom()
				g.roomID = ""
				g.scr = screenHome

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

	}
}

func (g *Game) Layout(w, h int) (int, int) { return protocol.ScreenW, protocol.ScreenH }

func fitToScreen() {
	mw, mh := ebiten.ScreenSizeInFullscreen()
	w, h := protocol.ScreenW, protocol.ScreenH

	margin := 48
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
	ebiten.SetWindowTitle(protocol.GameName + " — by s.robev")
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

func (b *battleHPBar) Update() {

	if b.displayHP > b.targetHP {
		delta := b.displayHP - b.targetHP

		step := delta / 6
		if step < 2 {
			step = 2
		}
		b.displayHP -= step
		if b.displayHP < b.targetHP {
			b.displayHP = b.targetHP
		}
	}
	if b.flashTicks > 0 {
		b.flashTicks--
	}
}

func (b *battleHPBar) Draw(dst *ebiten.Image) {
	if b.maxHP <= 0 || b.w <= 0 || b.h <= 0 {
		return
	}
	fw := float64(b.w)

	curW := int(fw * float64(b.targetHP) / float64(b.maxHP))
	dispW := int(fw * float64(b.displayHP) / float64(b.maxHP))
	if dispW < curW {
		dispW = curW
	}
	if dispW > b.w {
		dispW = b.w
	}

	ebitenutil.DrawRect(dst, float64(b.x), float64(b.y), float64(b.w), float64(b.h), color.NRGBA{18, 18, 22, 255})

	if curW > 0 {
		ebitenutil.DrawRect(dst, float64(b.x), float64(b.y), float64(curW), float64(b.h), b.colBase)
	}

	if dispW > curW {
		if b.flashTicks == 0 || (b.flashTicks%6) < 3 {
			ebitenutil.DrawRect(dst, float64(b.x+curW), float64(b.y), float64(dispW-curW), float64(b.h), b.colFlash)
		}
	}

	if dispW < b.w {
		ebitenutil.DrawRect(dst, float64(b.x+dispW), float64(b.y), float64(b.w-dispW), float64(b.h), b.colMissing)
	}

	ebitenutil.DrawRect(dst, float64(b.x), float64(b.y), float64(b.w), 1, color.NRGBA{0, 0, 0, 160})
	ebitenutil.DrawRect(dst, float64(b.x), float64(b.y+b.h-1), float64(b.w), 1, color.NRGBA{0, 0, 0, 160})
	ebitenutil.DrawRect(dst, float64(b.x), float64(b.y), 1, float64(b.h), color.NRGBA{0, 0, 0, 160})
	ebitenutil.DrawRect(dst, float64(b.x+b.w-1), float64(b.y), 1, float64(b.h), color.NRGBA{0, 0, 0, 160})
}

func (g *Game) drawProjectiles(screen *ebiten.Image, shouldMirror bool, mirrorY func(float64) float64, playerID int64) {

	// Draw actual flying projectiles from the world state
	if g.world != nil && g.world.Projectiles != nil {
		for _, proj := range g.world.Projectiles {
			if !proj.Active {
				continue
			}

			// Apply mirroring to projectile coordinates
			renderX := proj.X
			renderY := proj.Y
			renderTX := proj.TX
			renderTY := proj.TY
			if shouldMirror {
				renderY = mirrorY(proj.Y)
				renderTY = mirrorY(proj.TY)
			}

			// Apply camera transformations
			renderX = renderX*g.cameraZoom + g.cameraX
			renderY = renderY*g.cameraZoom + g.cameraY
			renderTX = renderTX*g.cameraZoom + g.cameraX
			renderTY = renderTY*g.cameraZoom + g.cameraY

			// Draw projectile as a single flying bolt at current position
			g.drawProjectileByType(screen, renderX, renderY, renderTX, renderTY, proj.ProjectileType, g.cameraZoom)
		}
	}

	// Legacy projectile rendering for units that are attacking (fallback)
	for _, u := range g.world.Units {
		// Only draw projectiles for ranged units that belong to the player
		if strings.ToLower(u.Class) == "range" && u.OwnerID == playerID {
			// Find target
			tx, ty := g.findTargetForUnit(u)
			dist := math.Hypot(tx-u.X, ty-u.Y)

			// Only draw if in range and attacking
			if dist <= float64(u.Range) && dist > 10 {
				// Apply mirroring to projectile coordinates
				renderUX := u.X
				renderUY := u.Y
				renderTX := tx
				renderTY := ty
				if shouldMirror {
					renderUY = mirrorY(u.Y)
					renderTY = mirrorY(ty)
				}

				// Apply camera transformations
				renderUX = renderUX*g.cameraZoom + g.cameraX
				renderUY = renderUY*g.cameraZoom + g.cameraY
				renderTX = renderTX*g.cameraZoom + g.cameraX
				renderTY = renderTY*g.cameraZoom + g.cameraY

				// Determine projectile type based on unit lore/name
				projectileType := g.determineProjectileType(u.Name)

				// Draw projectile based on type (only if no actual projectile exists)
				if g.world.Projectiles == nil || len(g.world.Projectiles) == 0 {
					g.drawProjectileByType(screen, renderUX, renderUY, renderTX, renderTY, projectileType, g.cameraZoom)
				}

				// Create impact effect when projectile reaches target
				if dist < 15 && g.particleSystem != nil {
					g.particleSystem.CreateEnhancedImpactEffect(renderTX, renderTY, projectileType)
				}
			}
		}
	}
}

// determineProjectileType analyzes unit name to determine elemental projectile type
func (g *Game) determineProjectileType(unitName string) string {
	name := strings.ToLower(unitName)

	// Fire-themed projectiles
	if strings.Contains(name, "blaze") || strings.Contains(name, "fire") ||
		strings.Contains(name, "magma") || strings.Contains(name, "flame") ||
		strings.Contains(name, "bloodmage") || strings.Contains(name, "firedrake") {
		return "fire"
	}

	// Ice/Frost-themed projectiles
	if strings.Contains(name, "glacia") || strings.Contains(name, "blizzard") ||
		strings.Contains(name, "frost") || strings.Contains(name, "ice") ||
		strings.Contains(name, "arctic") || strings.Contains(name, "winter") {
		return "frost"
	}

	// Lightning-themed projectiles
	if strings.Contains(name, "lightning") || strings.Contains(name, "chain") ||
		strings.Contains(name, "storm") || strings.Contains(name, "thunder") {
		return "lightning"
	}

	// Holy/Light-themed projectiles
	if strings.Contains(name, "holy") || strings.Contains(name, "light") ||
		strings.Contains(name, "divine") || strings.Contains(name, "angel") ||
		strings.Contains(name, "nova") || strings.Contains(name, "radiant") {
		return "holy"
	}

	// Dark/Shadow-themed projectiles
	if strings.Contains(name, "shadow") || strings.Contains(name, "dark") ||
		strings.Contains(name, "night") || strings.Contains(name, "void") ||
		strings.Contains(name, "death") || strings.Contains(name, "necro") {
		return "dark"
	}

	// Nature-themed projectiles
	if strings.Contains(name, "spirit") || strings.Contains(name, "nature") ||
		strings.Contains(name, "earth") || strings.Contains(name, "wind") ||
		strings.Contains(name, "jungle") || strings.Contains(name, "forest") {
		return "nature"
	}

	// Arcane/Magic-themed projectiles
	if strings.Contains(name, "arcane") || strings.Contains(name, "mana") ||
		strings.Contains(name, "magic") || strings.Contains(name, "sorcerer") ||
		strings.Contains(name, "wizard") || strings.Contains(name, "mage") {
		return "arcane"
	}

	// Default projectile type
	return "default"
}

// drawProjectileByType renders different projectile visuals based on type
func (g *Game) drawProjectileByType(screen *ebiten.Image, currentX, currentY, targetX, targetY float64, projectileType string, zoom float64) {
	// Realistic projectile sizes - 15-20% of unit size (unit is ~42px, so projectile ~6-8px)
	projectileSize := 6 * zoom // Main projectile (15% of unit size)
	coreSize := 3 * zoom       // Bright core
	trailSize := 2 * zoom      // Trail particles

	switch projectileType {
	case "fire":
		// FIREBALL - Realistic flaming projectile
		// Outer flame layer (orange-red)
		ebitenutil.DrawCircle(screen, currentX, currentY, projectileSize, color.NRGBA{255, 80, 0, 220})
		// Inner flame core (bright orange)
		ebitenutil.DrawCircle(screen, currentX, currentY, projectileSize*0.7, color.NRGBA{255, 150, 0, 255})
		// Bright white-hot core
		ebitenutil.DrawCircle(screen, currentX, currentY, coreSize, color.NRGBA{255, 220, 100, 255})

		// Flaming trail particles
		for i := 0; i < 8; i++ {
			trailX := currentX + (rand.Float64()-0.5)*20*zoom
			trailY := currentY + (rand.Float64()-0.5)*20*zoom
			ebitenutil.DrawCircle(screen, trailX, trailY, trailSize, color.NRGBA{255, 120, 0, 180})
		}
		// Smoke trail
		for i := 0; i < 3; i++ {
			smokeX := currentX + (rand.Float64()-0.5)*15*zoom
			smokeY := currentY + (rand.Float64()-0.5)*15*zoom
			ebitenutil.DrawCircle(screen, smokeX, smokeY, trailSize*1.5, color.NRGBA{100, 100, 100, 120})
		}

	case "frost":
		// FROSTBOLT - Narrow bullet-shaped projectile tapering to a point
		// Calculate direction vector from current to target
		dx := targetX - currentX
		dy := targetY - currentY
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist > 0 {
			dx /= dist
			dy /= dist
		}

		// Bullet length and width
		bulletLength := projectileSize * 2.5 // Elongated shape
		bulletWidth := projectileSize * 0.6  // Narrow width

		// Draw the main bullet body as a series of connected circles
		segments := 8
		for i := 0; i < segments; i++ {
			progress := float64(i) / float64(segments-1)
			segmentX := currentX + dx*bulletLength*progress
			segmentY := currentY + dy*bulletLength*progress

			// Taper the width from back to front
			taperFactor := 1.0 - progress*0.6 // Narrower at the front
			segmentSize := bulletWidth * taperFactor

			// Draw segment with gradient color (brighter at front)
			alpha := 0.8 - progress*0.3
			blue := 0.78 + progress*0.2
			ebitenutil.DrawCircle(screen, segmentX, segmentY, segmentSize, color.NRGBA{150, 220, uint8(blue * 255), uint8(alpha * 255)})
		}

		// Draw tapered point at the front
		pointX := currentX + dx*bulletLength*0.9
		pointY := currentY + dy*bulletLength*0.9
		ebitenutil.DrawCircle(screen, pointX, pointY, bulletWidth*0.2, color.NRGBA{200, 240, 255, 255})

		// Bright core at the center
		ebitenutil.DrawCircle(screen, currentX, currentY, coreSize, color.NRGBA{220, 250, 255, 255})

		// Ice crystal spikes along the bullet
		for i := 0; i < 4; i++ {
			spikePos := float64(i) * 0.25
			spikeX := currentX + dx*bulletLength*spikePos
			spikeY := currentY + dy*bulletLength*spikePos
			spikeAngle := math.Atan2(dy, dx) + math.Pi/2
			spikeDist := bulletWidth * 1.2
			spikeEndX := spikeX + math.Cos(spikeAngle)*spikeDist
			spikeEndY := spikeY + math.Sin(spikeAngle)*spikeDist
			ebitenutil.DrawLine(screen, spikeX, spikeY, spikeEndX, spikeEndY, color.NRGBA{220, 240, 255, 200})
		}

		// Frost mist trail behind the bullet
		for i := 0; i < 3; i++ {
			trailX := currentX - dx*bulletLength*0.5 + (rand.Float64()-0.5)*12*zoom
			trailY := currentY - dy*bulletLength*0.5 + (rand.Float64()-0.5)*12*zoom
			ebitenutil.DrawCircle(screen, trailX, trailY, trailSize, color.NRGBA{180, 220, 255, 150})
		}

	case "lightning":
		// LIGHTNING BOLT - Electric energy projectile
		// Outer electric field (bright yellow)
		ebitenutil.DrawCircle(screen, currentX, currentY, projectileSize, color.NRGBA{255, 255, 100, 180})
		// Inner electric core (pure white)
		ebitenutil.DrawCircle(screen, currentX, currentY, projectileSize*0.5, color.NRGBA{255, 255, 200, 255})
		// Electric core
		ebitenutil.DrawCircle(screen, currentX, currentY, coreSize, color.NRGBA{255, 255, 255, 255})

		// Electric arcs branching out
		for i := 0; i < 5; i++ {
			arcX := currentX + (rand.Float64()-0.5)*25*zoom
			arcY := currentY + (rand.Float64()-0.5)*25*zoom
			ebitenutil.DrawLine(screen, currentX, currentY, arcX, arcY, color.NRGBA{255, 255, 150, 220})
			// Small electric sparks
			ebitenutil.DrawCircle(screen, arcX, arcY, trailSize*0.3, color.NRGBA{255, 255, 200, 200})
		}

	case "holy":
		// HOLY MISSILE - Divine light projectile
		// Outer divine aura (golden)
		ebitenutil.DrawCircle(screen, currentX, currentY, projectileSize, color.NRGBA{255, 215, 0, 200})
		// Inner holy light (bright gold)
		ebitenutil.DrawCircle(screen, currentX, currentY, projectileSize*0.7, color.NRGBA{255, 235, 100, 255})
		// Radiant core
		ebitenutil.DrawCircle(screen, currentX, currentY, coreSize, color.NRGBA{255, 250, 150, 255})

		// Divine light rays
		for i := 0; i < 8; i++ {
			angle := float64(i) * math.Pi / 4
			rayX := currentX + math.Cos(angle)*projectileSize*1.3
			rayY := currentY + math.Sin(angle)*projectileSize*1.3
			ebitenutil.DrawLine(screen, currentX, currentY, rayX, rayY, color.NRGBA{255, 240, 100, 180})
		}
		// Holy sparkles
		for i := 0; i < 6; i++ {
			sparkleX := currentX + (rand.Float64()-0.5)*22*zoom
			sparkleY := currentY + (rand.Float64()-0.5)*22*zoom
			ebitenutil.DrawCircle(screen, sparkleX, sparkleY, trailSize*0.4, color.NRGBA{255, 250, 200, 220})
		}

	case "dark":
		// SHADOW BOLT - Dark magical projectile
		// Outer shadow veil (dark purple)
		ebitenutil.DrawCircle(screen, currentX, currentY, projectileSize, color.NRGBA{80, 20, 120, 200})
		// Inner dark energy (purple)
		ebitenutil.DrawCircle(screen, currentX, currentY, projectileSize*0.6, color.NRGBA{120, 50, 180, 255})
		// Void core
		ebitenutil.DrawCircle(screen, currentX, currentY, coreSize, color.NRGBA{150, 100, 200, 255})

		// Shadow tendrils
		for i := 0; i < 4; i++ {
			tendrilX := currentX + (rand.Float64()-0.5)*20*zoom
			tendrilY := currentY + (rand.Float64()-0.5)*20*zoom
			ebitenutil.DrawLine(screen, currentX, currentY, tendrilX, tendrilY, color.NRGBA{100, 30, 150, 160})
		}
		// Dark particles
		for i := 0; i < 5; i++ {
			darkX := currentX + (rand.Float64()-0.5)*16*zoom
			darkY := currentY + (rand.Float64()-0.5)*16*zoom
			ebitenutil.DrawCircle(screen, darkX, darkY, trailSize, color.NRGBA{80, 20, 120, 140})
		}

	case "nature":
		// NATURE'S LIGHTNING BOLT - Green lightning bolt projectile
		// Calculate direction vector from current to target
		dx := targetX - currentX
		dy := targetY - currentY
		dist := math.Sqrt(dx*dx + dy*dy)
		if dist > 0 {
			dx /= dist
			dy /= dist
		}

		// Main lightning bolt path
		boltLength := projectileSize * 3.0
		segments := 6

		// Draw main lightning bolt with zigzag pattern
		prevX := currentX
		prevY := currentY

		for i := 1; i <= segments; i++ {
			progress := float64(i) / float64(segments)
			segmentX := currentX + dx*boltLength*progress
			segmentY := currentY + dy*boltLength*progress

			// Add zigzag to lightning bolt
			zagOffset := (rand.Float64() - 0.5) * projectileSize * 0.8
			perpX := -dy * zagOffset
			perpY := dx * zagOffset

			segmentX += perpX
			segmentY += perpY

			// Draw lightning segment
			ebitenutil.DrawLine(screen, prevX, prevY, segmentX, segmentY, color.NRGBA{100, 255, 100, 255})

			// Add branching lightning arcs
			if i%2 == 0 && rand.Float64() < 0.7 {
				branchAngle := math.Atan2(dy, dx) + (rand.Float64()-0.5)*math.Pi*0.8
				branchLength := projectileSize * 1.5
				branchX := segmentX + math.Cos(branchAngle)*branchLength
				branchY := segmentY + math.Sin(branchAngle)*branchLength
				ebitenutil.DrawLine(screen, segmentX, segmentY, branchX, branchY, color.NRGBA{150, 255, 150, 200})
			}

			prevX = segmentX
			prevY = segmentY
		}

		// Bright core at the center
		ebitenutil.DrawCircle(screen, currentX, currentY, coreSize, color.NRGBA{200, 255, 200, 255})

		// Lightning spark effects along the bolt
		for i := 0; i < 4; i++ {
			sparkProgress := rand.Float64()
			sparkX := currentX + dx*boltLength*sparkProgress
			sparkY := currentY + dy*boltLength*sparkProgress
			sparkAngle := math.Atan2(dy, dx) + (rand.Float64()-0.5)*math.Pi
			sparkDist := projectileSize * 0.5
			sparkEndX := sparkX + math.Cos(sparkAngle)*sparkDist
			sparkEndY := sparkY + math.Sin(sparkAngle)*sparkDist
			ebitenutil.DrawLine(screen, sparkX, sparkY, sparkEndX, sparkEndY, color.NRGBA{180, 255, 180, 220})
		}

		// Nature energy particles
		for i := 0; i < 5; i++ {
			particleX := currentX + (rand.Float64()-0.5)*boltLength*0.8
			particleY := currentY + (rand.Float64()-0.5)*boltLength*0.8
			ebitenutil.DrawCircle(screen, particleX, particleY, trailSize*0.6, color.NRGBA{120, 255, 120, 180})
		}

	case "arcane":
		// ARCANE MISSILE - Magical energy projectile
		// Outer magical field (purple)
		ebitenutil.DrawCircle(screen, currentX, currentY, projectileSize, color.NRGBA{150, 100, 255, 200})
		// Inner arcane energy (bright purple)
		ebitenutil.DrawCircle(screen, currentX, currentY, projectileSize*0.7, color.NRGBA{200, 150, 255, 255})
		// Magical core
		ebitenutil.DrawCircle(screen, currentX, currentY, coreSize, color.NRGBA{220, 180, 255, 255})

		// Arcane runes/symbols
		for i := 0; i < 5; i++ {
			angle := float64(i) * 2 * math.Pi / 5
			runeX := currentX + math.Cos(angle)*projectileSize*1.2
			runeY := currentY + math.Sin(angle)*projectileSize*1.2
			ebitenutil.DrawCircle(screen, runeX, runeY, trailSize*0.5, color.NRGBA{240, 200, 255, 220})
		}
		// Magical sparkles
		for i := 0; i < 6; i++ {
			sparkleX := currentX + (rand.Float64()-0.5)*20*zoom
			sparkleY := currentY + (rand.Float64()-0.5)*20*zoom
			ebitenutil.DrawCircle(screen, sparkleX, sparkleY, trailSize*0.4, color.NRGBA{255, 220, 255, 200})
		}

	default:
		// ENERGY BOLT - Generic magical projectile
		// Outer energy field
		ebitenutil.DrawCircle(screen, currentX, currentY, projectileSize, color.NRGBA{200, 200, 255, 200})
		// Inner energy core
		ebitenutil.DrawCircle(screen, currentX, currentY, projectileSize*0.6, color.NRGBA{220, 220, 255, 255})
		// Bright core
		ebitenutil.DrawCircle(screen, currentX, currentY, coreSize, color.NRGBA{255, 255, 255, 255})

		// Energy particles
		for i := 0; i < 6; i++ {
			particleX := currentX + (rand.Float64()-0.5)*16*zoom
			particleY := currentY + (rand.Float64()-0.5)*16*zoom
			ebitenutil.DrawCircle(screen, particleX, particleY, trailSize, color.NRGBA{220, 240, 255, 180})
		}
	}

}

func (g *Game) findTargetForUnit(u *RenderUnit) (float64, float64) {
	var best *RenderUnit
	bestDist := math.MaxFloat64
	for _, v := range g.world.Units {
		if v.OwnerID == u.OwnerID || v.HP <= 0 {
			continue
		}
		d := math.Hypot(v.X-u.X, v.Y-u.Y)
		if d < bestDist {
			bestDist = d
			best = v
		}
	}
	if best != nil {
		return best.X, best.Y
	}
	// enemy base
	for _, b := range g.world.Bases {
		if b.OwnerID != u.OwnerID {
			return float64(b.X + b.W/2), float64(b.Y + b.H/2)
		}
	}
	return float64(protocol.ScreenW / 2), float64(protocol.ScreenH / 2)
}

// drawMirroredParticles draws particle effects with Y-axis mirroring for PvP
func (g *Game) drawMirroredParticles(screen *ebiten.Image, mirrorY func(float64) float64, cameraX, cameraY, cameraZoom float64) {
	if g.particleSystem == nil {
		return
	}

	for _, emitter := range g.particleSystem.Emitters {
		if !emitter.Active {
			continue
		}

		emitter.DrawMirroredWithCamera(screen, mirrorY, cameraX, cameraY, cameraZoom)
	}
}

// drawParticlesWithCamera draws particle effects with camera transformations
func (g *Game) drawParticlesWithCamera(screen *ebiten.Image, cameraX, cameraY, cameraZoom float64) {
	if g.particleSystem == nil {
		return
	}

	for _, emitter := range g.particleSystem.Emitters {
		if !emitter.Active {
			continue
		}

		emitter.DrawWithCamera(screen, cameraX, cameraY, cameraZoom)
	}
}

// updateTimerInput handles timer-related input (pause button, pause overlay buttons)
func (g *Game) updateTimerInput() {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()

		// Handle pause button click first (always active, even during pause overlay)
		if g.roomID != "" && !strings.Contains(g.roomID, "pvp-") && g.timerBtn.hit(mx, my) {
			if g.timerPaused {
				g.send("ResumeGame", protocol.ResumeGame{})
				// Immediately resume locally as fallback
				g.timerPaused = false
				g.pauseOverlay = false
			} else {
				g.send("PauseGame", protocol.PauseGame{})
				// Immediately pause locally as fallback
				g.timerPaused = true
				g.pauseOverlay = true
			}
			return // Pause button was clicked, don't process other inputs
		}

		// Handle pause overlay buttons (only when overlay is active)
		if g.pauseOverlay {
			// Define button positions for pause overlay
			menuW := 300
			menuH := 250
			menuX := (protocol.ScreenW - menuW) / 2
			menuY := (protocol.ScreenH - menuH) / 2

			resumeBtnX := menuX + 50
			resumeBtnY := menuY + 50
			restartBtnX := menuX + 50
			restartBtnY := menuY + 100
			surrenderBtnX := menuX + 50
			surrenderBtnY := menuY + 150

			if mx >= resumeBtnX && mx <= resumeBtnX+200 && my >= resumeBtnY && my <= resumeBtnY+40 {
				g.send("ResumeGame", protocol.ResumeGame{})
				g.pauseOverlay = false
			} else if mx >= restartBtnX && mx <= restartBtnX+200 && my >= restartBtnY && my <= restartBtnY+40 {
				g.send("RestartMatch", protocol.RestartMatch{})
				g.pauseOverlay = false
			} else if mx >= surrenderBtnX && mx <= surrenderBtnX+200 && my >= surrenderBtnY && my <= surrenderBtnY+40 {
				g.send("SurrenderMatch", protocol.SurrenderMatch{})
				g.pauseOverlay = false
			}
			return // Don't handle other clicks when pause overlay is active
		}
	}
}

// drawDeployZones draws the deploy zones as alpha rectangles
func (g *Game) drawDeployZones(screen *ebiten.Image, shouldMirror bool) {
	if g.currentMapDef == nil {
		return
	}

	// Check if this is PvP (has exactly 2 bases with different owners)
	isPvP := g.isPvPMode()

	// Only show deploy zones when a unit is selected or being dragged
	if g.selectedIdx == -1 && !g.dragActive {
		return
	}

	for _, zone := range g.currentMapDef.DeployZones {
		// In PvP mode, only show player's own deploy zones (not enemy zones)
		if isPvP && zone.Owner != "player" {
			continue // Skip enemy zones in PvP - players should only see their own zones
		}

		// In PvE mode, skip enemy deploy zones (they're not relevant to player)
		if !isPvP && zone.Owner != "player" {
			continue // Skip enemy zones in PvE
		}

		// Convert normalized coordinates to screen coordinates
		x := zone.X * float64(protocol.ScreenW)
		y := zone.Y * float64(protocol.ScreenH)
		w := zone.W * float64(protocol.ScreenW)
		h := zone.H * float64(protocol.ScreenH)

		// Apply mirroring if needed
		if shouldMirror {
			y = float64(protocol.ScreenH) - y - h
		}

		// Apply camera transformations
		x = x*g.cameraZoom + g.cameraX
		y = y*g.cameraZoom + g.cameraY
		w = w * g.cameraZoom
		h = h * g.cameraZoom

		// Deploy zone color - only blue for player zones (since enemy zones are hidden in PvP)
		var deployZoneColor color.NRGBA
		var borderColor color.NRGBA

		deployZoneColor = color.NRGBA{70, 130, 255, 100} // Player deploy zones - blue tint, more opaque
		borderColor = color.NRGBA{100, 160, 255, 180}

		// Draw the deploy zone rectangle
		ebitenutil.DrawRect(screen, x, y, w, h, deployZoneColor)

		// Draw a subtle border
		ebitenutil.DrawRect(screen, x, y, w, 1, borderColor)     // Top
		ebitenutil.DrawRect(screen, x, y+h-1, w, 1, borderColor) // Bottom
		ebitenutil.DrawRect(screen, x, y, 1, h, borderColor)     // Left
		ebitenutil.DrawRect(screen, x+w-1, y, 1, h, borderColor) // Right
	}
}

// isPvPMode checks if the current game is in PvP mode
func (g *Game) isPvPMode() bool {
	playerBaseCount := 0
	enemyBaseCount := 0
	for _, b := range g.world.Bases {
		if b.OwnerID == g.playerID {
			playerBaseCount++
		} else {
			enemyBaseCount++
		}
	}
	// PvP has exactly 2 bases (1 player + 1 enemy) AND it's a PvP room
	return playerBaseCount == 1 && enemyBaseCount == 1 && strings.Contains(g.roomID, "pvp-")
}

// drawObstacles draws obstacles from the current map definition
func (g *Game) drawObstacles(screen *ebiten.Image, shouldMirror bool, mirrorY func(float64) float64) {
	if g.currentMapDef == nil {
		return
	}

	for _, obstacle := range g.currentMapDef.Obstacles {
		// Convert normalized coordinates to screen coordinates
		x := obstacle.X * float64(protocol.ScreenW)
		y := obstacle.Y * float64(protocol.ScreenH)
		w := obstacle.Width * float64(protocol.ScreenW)
		h := obstacle.Height * float64(protocol.ScreenH)

		// Apply mirroring if needed
		if shouldMirror {
			y = mirrorY(y)
		}

		// Apply camera transformations
		x = x*g.cameraZoom + g.cameraX
		y = y*g.cameraZoom + g.cameraY
		w = w * g.cameraZoom
		h = h * g.cameraZoom

		// Try to load obstacle image
		if img := g.ensureObstacleImage(obstacle.Type); img != nil {
			// Scale image to fit the obstacle dimensions
			iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
			if iw > 0 && ih > 0 {
				scaleX := w / float64(iw)
				scaleY := h / float64(ih)
				scale := mathMin(scaleX, scaleY)

				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(scale, scale)
				op.GeoM.Translate(x, y)
				screen.DrawImage(img, op)
			}
		} else {
			// Fallback: draw a colored rectangle
			obstacleColor := color.NRGBA{100, 100, 100, 255} // Gray for unknown obstacles
			if obstacle.Type == "tree" {
				obstacleColor = color.NRGBA{34, 139, 34, 255} // Green for trees
			} else if obstacle.Type == "rock" {
				obstacleColor = color.NRGBA{105, 105, 105, 255} // Dark gray for rocks
			} else if obstacle.Type == "building" {
				obstacleColor = color.NRGBA{139, 69, 19, 255} // Brown for buildings
			}
			ebitenutil.DrawRect(screen, x, y, w, h, obstacleColor)
		}
	}
}
