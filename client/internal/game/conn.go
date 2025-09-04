package game

import (
	"log"
	"rumble/client/internal/netcfg"
	"rumble/shared/protocol"
	"time"
)

func (g *Game) retryConnect() {
	if g.connectInFlight {
		return
	}
	g.connSt = stateConnecting
	g.connErrMsg = ""
	g.connectInFlight = true
	go g.connectAsync()
}

func (g *Game) connectAsync() {
	// Single in-flight dial guarded by connectInFlight
	n, err := NewNet(netcfg.ServerURL)
	// send result without blocking forever; drop oldest on overflow
	select {
	case g.connCh <- connResult{n: n, err: err}:
	default:
		select {
		case <-g.connCh:
		default:
		}
		g.connCh <- connResult{n: n, err: err}
	}
	g.connectInFlight = false
}

// ensureNet: make sure we have a live socket
func (g *Game) ensureNet() bool {
	if g.net != nil && !g.net.IsClosed() {
		return true
	}
	n, err := NewNet(netcfg.ServerURL)
	if err != nil {
		log.Printf("NET: dial failed: %v", err)
		return false
	}
	g.net = n
	log.Println("NET: connected")
	return true
}

// safe send wrapper
func (g *Game) send(typ string, payload interface{}) {
	if !g.ensureNet() {
		return
	}
	if err := g.net.Send(typ, payload); err != nil {
		log.Printf("NET: send(%s) failed: %v", typ, err)
	}
}

func (g *Game) resetToLogin() {

	if g.net != nil {
		g.net.Close()
		g.net = nil
	}

	g.scr = screenLogin
	g.nameInput = ""
	g.name = ""
	g.playerID = 0

	g.minisAll = nil
	g.minisOnly = nil
	g.champions = nil
	g.nameToMini = nil
	g.maps = nil
	g.champToMinis = map[string]map[string]bool{}
	g.selectedChampion = ""
	g.selectedMinis = map[string]bool{}

	g.pvpQueued = false
	g.pvpHosting = false
	g.pvpCode = ""
	g.pvpCodeInput = ""
	g.pvpStatus = "Logged out."

	g.roomID = ""
	g.currentArena = ""
	g.pendingArena = ""
	g.world = &World{Units: map[int64]*RenderUnit{}, Bases: map[int64]protocol.BaseState{}}
	// Populate obstacles and lanes if available
	if g.currentMapDef != nil {
		g.world.Obstacles = g.currentMapDef.Obstacles
		g.world.Lanes = g.currentMapDef.Lanes
	}
	g.endActive, g.endVictory, g.gameOver, g.victory = false, false, false, false

	g.lobbyRequested = false
	g.lastLobbyReq = time.Time{}
	g.activeTab = tabArmy

	g.connSt = stateIdle
	g.connectInFlight = false
	apiBase := netcfg.APIBase
	g.auth = NewAuthUI(apiBase, func(username string) {
		g.name = username
		g.activeTab = tabArmy
		g.lobbyRequested = false

	})
}

func (g *Game) requestLobbyDataOnce() {
	if g.net == nil || g.net.IsClosed() {
		return
	}

	if time.Since(g.lastLobbyReq) < 1200*time.Millisecond {
		return
	}
	g.send("GetProfile", protocol.GetProfile{})
	g.send("ListMinis", protocol.ListMinis{})
	g.send("ListMaps", protocol.ListMaps{})
	g.lastLobbyReq = time.Now()
}

// resetToLoginNoAutoConnect cleanly tears down WS + UI and shows the Auth screen.
// It does NOT auto-connect; AuthUI's success callback will connect after login.
func (g *Game) resetToLoginNoAutoConnect() {

	if g.net != nil {
		_ = g.net.Close()
		g.net = nil
	}

	g.connSt = stateIdle
	g.connErrMsg = ""
	g.connRetryAt = time.Time{}
	g.connectInFlight = false

	for {
		select {
		case <-g.connCh:
		default:
			goto drained
		}
	}
drained:

	g.scr = screenHome
	g.showProfile = false
	g.profileOpen = false
	g.profCloseBtn = rect{}
	g.profLogoutBtn = rect{}

	g.roomID = ""
	g.pvpCode = ""
	g.pvpStatus = ""
	g.pvpCodeInput = ""
	g.hoveredHS, g.selectedHS = -1, -1

	g.lobbyRequested = false
	g.lastLobbyReq = time.Time{}

	g.pvpQueued, g.pvpHosting = false, false
	g.pvpCode, g.pvpStatus, g.pvpCodeInput = "", "", ""
	g.hoveredHS, g.selectedHS = -1, -1

	g.minisAll = nil
	g.champions = nil
	g.minisOnly = nil
	g.maps = nil
	g.nameToMini = nil

	g.world = &World{Units: make(map[int64]*RenderUnit), Bases: make(map[int64]protocol.BaseState)}
	// Populate obstacles and lanes if available
	if g.currentMapDef != nil {
		g.world.Obstacles = g.currentMapDef.Obstacles
		g.world.Lanes = g.currentMapDef.Lanes
	}
	g.endActive, g.endVictory, g.gameOver, g.victory = false, false, false, false
	g.hand = nil
	g.next = protocol.MiniCardView{}
	g.selectedIdx = -1
	g.dragActive = false

	apiBase := netcfg.APIBase
	g.auth = NewAuthUI(apiBase, func(username string) {

		g.name = username
		g.retryConnect()
	})
}
