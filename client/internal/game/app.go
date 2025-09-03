package game

import (
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
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

	// Initialize new interaction defaults
	g.slotDragFrom = -1

	if !HasToken() {

		apiBase := netcfg.APIBase
		g.auth = NewAuthUI(apiBase, func(username string) {
			g.name = username

		})
		g.connSt = stateIdle
	} else {

		g.name = LoadUsername()
		g.retryConnect()
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
		g.updateHome()
	case screenBattle:
		if !g.timerPaused {
			g.updateBattle()
		}
		g.updateTimerInput()
	}

	// Skip world updates if game is paused
	if !g.timerPaused {
		g.world.LerpPositions()

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
	if g.scr == screenBattle {
		if inpututil.IsKeyJustPressed(ebiten.KeyE) {
			// Create explosion effect at center of screen
			if g.particleSystem != nil {
				g.particleSystem.CreateExplosionEffect(float64(protocol.ScreenW/2), float64(protocol.ScreenH/2), 1.0)
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyS) {
			// Create spell effect
			if g.particleSystem != nil {
				g.particleSystem.CreateSpellEffect(float64(protocol.ScreenW/2), float64(protocol.ScreenH/2), "fire")
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyH) {
			// Create healing effect
			if g.particleSystem != nil {
				g.particleSystem.CreateHealingEffect(float64(protocol.ScreenW/2), float64(protocol.ScreenH/2))
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyA) {
			// Create aura effect
			if g.particleSystem != nil {
				g.particleSystem.CreateAuraEffect(float64(protocol.ScreenW/2), float64(protocol.ScreenH/2), "buff")
			}
		}

		// New ability effect shortcuts
		if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
			// Healing ability
			if g.particleSystem != nil {
				g.particleSystem.CreateUnitAbilityEffect(float64(protocol.ScreenW/2), float64(protocol.ScreenH/2), "heal")
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyW) {
			// Stun ability
			if g.particleSystem != nil {
				g.particleSystem.CreateUnitAbilityEffect(float64(protocol.ScreenW/2), float64(protocol.ScreenH/2), "stun")
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyR) {
			// Rage ability
			if g.particleSystem != nil {
				g.particleSystem.CreateUnitAbilityEffect(float64(protocol.ScreenW/2), float64(protocol.ScreenH/2), "rage")
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyT) {
			// Teleport ability
			if g.particleSystem != nil {
				g.particleSystem.CreateUnitAbilityEffect(float64(protocol.ScreenW/2-50), float64(protocol.ScreenH/2), "teleport")
				time.Sleep(200 * time.Millisecond) // Small delay for effect
				g.particleSystem.CreateUnitAbilityEffect(float64(protocol.ScreenW/2+50), float64(protocol.ScreenH/2), "teleport")
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyY) {
			// Critical hit effect
			if g.particleSystem != nil {
				g.particleSystem.CreateCriticalHitEffect(float64(protocol.ScreenW/2), float64(protocol.ScreenH/2))
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyU) {
			// Level up celebration
			if g.particleSystem != nil {
				g.particleSystem.CreateLevelUpEffect(float64(protocol.ScreenW/2), float64(protocol.ScreenH/2))
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyI) {
			// Battle buff effect
			if g.particleSystem != nil {
				g.particleSystem.CreateBattleBuffEffect(float64(protocol.ScreenW/2), float64(protocol.ScreenH/2), "attack")
			}
		}
		if inpututil.IsKeyJustPressed(ebiten.KeyP) {
			// Target healing effect (green particles on healed unit)
			if g.particleSystem != nil {
				// Simulate healer at left side healing target at right side
				healerX := float64(protocol.ScreenW / 4)
				targetX := float64(3 * protocol.ScreenW / 4)
				targetY := float64(protocol.ScreenH / 2)

				// Healing wave from healer
				g.particleSystem.CreateUnitAbilityEffect(healerX, targetY, "heal")
				// Green particles on healed target
				g.particleSystem.CreateTargetHealingEffect(targetX, targetY)
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

		// Draw deploy zones (alpha rectangles)
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

			// Create base with mirrored position for rendering
			renderBase := protocol.BaseState{
				X:       renderBaseX,
				Y:       renderBaseY,
				W:       b.W,
				H:       b.H,
				HP:      b.HP,
				MaxHP:   b.MaxHP,
				OwnerID: b.OwnerID,
			}
			drawBaseImg(screen, img, renderBase)

			if b.MaxHP > 0 {
				fx := g.hpfxStep(g.hpFxBases, id, b.HP, nowMs)
				isPlayer := (b.OwnerID == g.playerID)
				x := float64(renderBaseX)
				y := float64(b.Y - 6)
				if shouldMirror && b.OwnerID == g.playerID {
					y = mirrorY(float64(b.Y - 6))
				}
				w := float64(b.W)
				h := 4.0
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

		// Draw projectiles (behind units) with consistent mirroring
		g.drawProjectiles(screen, shouldMirror, mirrorY, g.playerID)

		// Draw units
		const unitTargetPX = 42.0
		for id, u := range g.world.Units {
			// Apply mirroring to unit position (only player's own units)
			renderX := u.X
			renderY := u.Y
			if shouldMirror && u.OwnerID == g.playerID {
				renderY = float64(protocol.ScreenH) - u.Y
			}

			if img := g.ensureMiniImageByName(u.Name); img != nil {
				op := &ebiten.DrawImageOptions{}
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
				s := unitTargetPX / float64(maxInt(1, maxInt(iw, ih)))
				op.GeoM.Scale(s, s)
				op.GeoM.Translate(renderX-float64(iw)*s/2, renderY-float64(ih)*s/2)
				screen.DrawImage(img, op)
			} else {
				ebitenutil.DrawRect(screen, renderX-6, renderY-6, 12, 12, color.White)
			}

			if u.MaxHP > 0 {
				barW := 26.0 * 1.05
				bx := renderX - barW/2
				by := renderY - unitTargetPX/2 - 6

				fx := g.hpfxStep(g.hpFxUnits, id, u.HP, nowMs)
				g.DrawHPBarForOwner(screen, bx, by, barW, 3, u.HP, u.MaxHP, fx.ghostHP, fx.healGhostHP, u.OwnerID == g.playerID)
				// Level badge left of HP bar
				rx0 := int(bx + 0.5)
				ry0 := int(by + 0.5)
				rx1 := int(bx + barW + 0.5)
				ry1 := int(by + 3 + 0.5)
				barRect := image.Rect(rx0, ry0, rx1, ry1)
				lvl := g.levelForUnitName(u.Name)
				g.drawLevelBadge(screen, barRect, lvl)
			}
		}

		// Draw particle effects (after units, before UI)
		if g.particleSystem != nil {
			// Apply mirroring to particle system if needed
			if shouldMirror {
				// Create a temporary mirrored particle system for rendering
				g.drawMirroredParticles(screen, mirrorY)
			} else {
				g.particleSystem.Draw(screen)
			}
		}

		// Draw battle UI
		g.drawBattleBar(screen)

		myCur, myMax, enCur, enMax := g.battleHPs()
		g.drawBattleTopBars(screen, myCur, myMax, enCur, enMax)
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

	// Enhanced projectile rendering with particle effects
	for _, u := range g.world.Units {
		// Only draw projectiles for ranged units that belong to the player
		if strings.ToLower(u.Class) == "range" && u.OwnerID == playerID {
			// Find target
			tx, ty := g.findTargetForUnit(u)
			dist := math.Hypot(tx-u.X, ty-u.Y)

			// Only draw if in range
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

				// Create particle trail for projectile
				if g.particleSystem != nil {
					g.particleSystem.CreateProjectileTrail(renderUX, renderUY, renderTX, renderTY)
				}

				// Draw a simple line projectile (fallback)
				ebitenutil.DrawLine(screen, renderUX, renderUY, renderTX, renderTY, color.NRGBA{255, 255, 100, 200})

				// Draw a small circle at the projectile tip
				ebitenutil.DrawCircle(screen, renderTX, renderTY, 3, color.NRGBA{255, 255, 0, 255})

				// Create impact effect when projectile reaches target
				if dist < 15 && g.particleSystem != nil {
					impactType := "default"
					if strings.Contains(strings.ToLower(u.Name), "fire") {
						impactType = "fire"
					} else if strings.Contains(strings.ToLower(u.Name), "ice") {
						impactType = "ice"
					}
					g.particleSystem.CreateImpactEffect(renderTX, renderTY, impactType)
				}
			}
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
func (g *Game) drawMirroredParticles(screen *ebiten.Image, mirrorY func(float64) float64) {
	if g.particleSystem == nil {
		return
	}

	for _, emitter := range g.particleSystem.Emitters {
		if !emitter.Active {
			continue
		}

		emitter.DrawMirrored(screen, mirrorY)
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
