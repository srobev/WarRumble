package game

import (
    "fmt"
    "image"
    "image/color"
    "log"
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

        connCh: make(chan connResult, 4),
        connSt: stateConnected,
    }

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

	if g.profileOpen && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()
		if g.profCloseBtn.hit(mx, my) {
			g.profileOpen = false
			g.profCloseBtn = rect{}
			g.profLogoutBtn = rect{}
		}
	}
afterMessages:

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
			g.endActive = (myHP <= 0 || enemyHP <= 0)
			g.endVictory = (enemyHP <= 0 && myHP > 0)
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
		g.updateBattle()
	}

	g.world.LerpPositions()
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

	if g.activeTab == tabArmy {
		g.ensureArmyBgLayer()
		if g.armyBgLayer != nil {
			var op ebiten.DrawImageOptions
			op.GeoM.Translate(0, float64(topBarH))
			screen.DrawImage(g.armyBgLayer, &op)
		}
	}

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
	if g.scr == screenBattle {
		g.drawArenaBG(screen)
	}
	nowMs := time.Now().UnixMilli()

	for id, b := range g.world.Bases {
		g.assets.ensureInit()
		img := g.assets.baseEnemy
		if b.OwnerID == g.playerID {
			img = g.assets.baseMe
		}
		drawBaseImg(screen, img, b)

		if b.MaxHP > 0 {
			fx := g.hpfxStep(g.hpFxBases, id, b.HP, nowMs)
			isPlayer := (b.OwnerID == g.playerID)
			x := float64(b.X); y := float64(b.Y-6); w := float64(b.W); h := 4.0
			g.DrawHPBarForOwner(screen, x, y, w, h, b.HP, b.MaxHP, fx.ghostHP, isPlayer)
			// Base level badge left of bar
			rx0 := int(x + 0.5); ry0 := int(y + 0.5); rx1 := int(x+w + 0.5); ry1 := int(y+h + 0.5)
			barRect := image.Rect(rx0, ry0, rx1, ry1)
			lvl := 1
			if isPlayer { lvl = g.currentArmyRoundedLevel() }
			g.drawLevelBadge(screen, barRect, lvl)
		}
	}

	// Units
	const unitTargetPX = 42.0
	for id, u := range g.world.Units {
		if img := g.ensureMiniImageByName(u.Name); img != nil {
			op := &ebiten.DrawImageOptions{}
			iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
			s := unitTargetPX / float64(maxInt(1, maxInt(iw, ih)))
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(u.X-float64(iw)*s/2, u.Y-float64(ih)*s/2)
			screen.DrawImage(img, op)
		} else {
			ebitenutil.DrawRect(screen, u.X-6, u.Y-6, 12, 12, color.White)
		}

		if u.MaxHP > 0 {
            barW := 26.0 * 1.05
            bx := u.X - barW/2
            by := u.Y - unitTargetPX/2 - 6

            fx := g.hpfxStep(g.hpFxUnits, id, u.HP, nowMs)
            g.DrawHPBarForOwner(screen, bx, by, barW, 3, u.HP, u.MaxHP, fx.ghostHP, u.OwnerID == g.playerID)
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

	switch g.scr {
	case screenLogin:
		text.Draw(screen, "Enter username, then press Enter:", basicfont.Face7x13, pad, 48, color.White)
		ebitenutil.DrawRect(screen, float64(pad), 54, 260, 18, color.NRGBA{40, 40, 40, 255})
		text.Draw(screen, g.nameInput, basicfont.Face7x13, pad+4, 68, color.White)

	case screenHome:
		g.drawHomeContent(screen)
		g.drawTopBarHome(screen)
		g.drawBottomBar(screen)

		if g.showProfile {
			g.drawProfileOverlay(screen)
		}

	case screenBattle:
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
		if g.endVictory || g.victory { title = "Victory!" }
		text.Draw(screen, title, basicfont.Face7x13, x+20, y+40, color.White)

        // XP gains list (if computed)
        if g.xpGains != nil {
            names := g.battleArmy
            if len(names) == 0 {
                if g.selectedChampion != "" { names = append(names, g.selectedChampion) }
                for i := 0; i < 6; i++ { if g.selectedOrder[i] != "" { names = append(names, g.selectedOrder[i]) } }
            }
            // Reorder so the champion stays first and minis with +XP are shown next
            if len(names) > 0 {
                champ := names[0]
                pos := make([]string, 0, 6)
                zero := make([]string, 0, 6)
                for _, n := range names[1:] {
                    if g.xpGains[n] > 0 { pos = append(pos, n) } else { zero = append(zero, n) }
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
				if d > 0 { line += fmt.Sprintf("  +%d XP", d) } else { line += "  +0 XP" }
				text.Draw(screen, line, basicfont.Face7x13, x+40, yy, color.White)
				yy += 18
				if yy > y+h-64 { break }
			}
		}

		g.continueBtn = rect{x: x + (w-120)/2, y: y + h - 50, w: 120, h: 32}
		ebitenutil.DrawRect(screen, float64(g.continueBtn.x), float64(g.continueBtn.y), float64(g.continueBtn.w), float64(g.continueBtn.h), color.NRGBA{70, 110, 70, 255})
		text.Draw(screen, "Continue", basicfont.Face7x13, g.continueBtn.x+18, g.continueBtn.y+20, color.White)
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
	ebiten.SetWindowTitle("War Rumble — by s.robev")
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



