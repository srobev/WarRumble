// server/srv/hub.go
package srv

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"rumble/server/account"
	"rumble/server/currency"
	"rumble/server/shop"
	"rumble/shared/game/types"
	"rumble/shared/protocol"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// converters between Account and Profile for backward compatibility

// accountToProfile converts an Account to Profile for network transmission
func accountToProfile(acc *account.Account) protocol.Profile {
	profile := protocol.Profile{
		PlayerID:  0, // Will be set from session later
		Name:      acc.Name,
		Army:      make([]string, len(acc.Army)),
		Armies:    make(map[string][]string),
		Gold:      int(acc.Gold),
		AccountXP: acc.AccountXP,
		UnitXP:    make(map[string]int),
		Resources: make(map[string]int),
		PvPRating: acc.PvPRating,
		PvPRank:   acc.PvPRank,
		Avatar:    acc.Avatar,
		GuildID:   acc.GuildID,
		Dust:      acc.Dust,
		Capsules: protocol.CapsuleStock{
			Rare:      acc.Capsules.Rare,
			Epic:      acc.Capsules.Epic,
			Legendary: acc.Capsules.Legendary,
		},
	}

	// Copy army and armies
	copy(profile.Army, acc.Army)
	for k, v := range acc.Armies {
		profile.Armies[k] = make([]string, len(v))
		copy(profile.Armies[k], v)
	}

	// Copy UnitXP
	for k, v := range acc.UnitXP {
		profile.UnitXP[k] = v
	}

	// Copy Resources
	for k, v := range acc.Resources {
		profile.Resources[k] = v
	}

	return profile
}

// sendUnitProgressionData sends unit progression data to client for UI display
func (h *Hub) sendUnitProgressionData(acc *account.Account, client *client) {
	// Collect all unique unit IDs from the account's armies
	unitIDs := make(map[string]bool)

	// Add units from active army
	for _, unitID := range acc.Army {
		unitIDs[unitID] = true
	}

	// Add units from saved armies
	for _, units := range acc.Armies {
		for _, unitID := range units {
			unitIDs[unitID] = true
		}
	}

	// For each unique unit, load progression (creates defaults if missing) and send
	for unitID := range unitIDs {
		progress, err := acc.LoadUnitProgress(unitID)
		if err != nil {
			log.Printf("Failed to load progress for unit %s: %v", unitID, err)
			continue
		}
		unitProgressSync := protocol.UnitProgressSynced{
			UnitID:                unitID,
			Rank:                  progress.Rank,
			ShardsOwned:           progress.ShardsOwned,
			PerkSlotsUnlocked:     progress.Rank, // Approximation - can be calculated based on rank
			LegendaryPerkUnlocked: false,         // Calculate based on rank requirements
		}
		sendJSON(client, "UnitProgressSynced", unitProgressSync)
	}
}

// profileToAccount converts a Profile to Account for persistent storage
func profileToAccount(prof protocol.Profile) *account.Account {
	acc := &account.Account{
		ID:        prof.Name, // Use name as ID fallback
		Name:      prof.Name,
		Gold:      int64(prof.Gold),
		AccountXP: prof.AccountXP,
		Resources: make(map[string]int),
		PvPRating: prof.PvPRating,
		PvPRank:   prof.PvPRank,
		Avatar:    prof.Avatar,
		GuildID:   prof.GuildID,
		UnitXP:    make(map[string]int),
		Army:      make([]string, len(prof.Army)),
		Armies:    make(map[string][]string),
	}

	// Copy army and armies
	copy(acc.Army, prof.Army)
	for k, v := range prof.Armies {
		acc.Armies[k] = make([]string, len(v))
		copy(acc.Armies[k], v)
	}

	// Copy UnitXP
	for k, v := range prof.UnitXP {
		acc.UnitXP[k] = v
	}

	// Copy Resources
	for k, v := range prof.Resources {
		acc.Resources[k] = v
	}

	return acc
}

type client struct {
	conn *websocket.Conn
	send chan []byte
	id   int64
	room *Room
	name string
}

type Session struct {
	PlayerID int64
	Name     string
	Army     []string         // legacy active army [champ, 6 minis]
	RoomID   string           // current room
	Profile  protocol.Profile // full persisted profile (Armies, Gold, XP, etc.)
}

func NewSession() *Session {
	return &Session{
		PlayerID: protocol.NewID(),
		Name:     "Guest",
		Army:     nil,
		Profile: protocol.Profile{
			PlayerID:  0,
			Name:      "Guest",
			Army:      nil,
			Armies:    map[string][]string{},
			Gold:      0,
			AccountXP: 0,
			UnitXP:    map[string]int{},
			Resources: map[string]int{},
			PvPRating: 1200,
			PvPRank:   "Knight",      // 1200 threshold name from your table
			Avatar:    "default.png", //Default avatar
		},
	}
}

type Hub struct {
	mu       sync.Mutex
	clients  map[*client]struct{}
	rooms    map[string]*Room
	sessions map[*client]*Session

	// NEW:
	pvpQueue       []*client
	friendly       map[string]*client
	friendByClient map[*client]string // host client -> code (for cancel/cleanup)

	// Guilds and chat
	guilds    *Guilds
	guildSubs map[string]map[*client]struct{} // guildID -> clients subscribed

	social      *Social
	shopService *shop.Service
}

func NewHub() *Hub {
	h := &Hub{
		clients:  make(map[*client]struct{}),
		rooms:    make(map[string]*Room),
		sessions: make(map[*client]*Session),

		// NEW:
		pvpQueue:       make([]*client, 0, 64),
		friendly:       make(map[string]*client),
		friendByClient: make(map[*client]string),
		guildSubs:      make(map[string]map[*client]struct{}),
	}
	// guilds set by main() via setter to pass data dir
	return h
}

func (h *Hub) SetGuilds(g *Guilds)            { h.guilds = g }
func (h *Hub) SetSocial(s *Social)            { h.social = s }
func (h *Hub) SetShopService(s *shop.Service) { h.shopService = s }

func makeRoomID(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func (h *Hub) EnqueuePvp(c *client) {
	h.mu.Lock()
	// prevent duplicate
	for _, x := range h.pvpQueue {
		if x == c {
			h.mu.Unlock()
			return
		}
	}
	h.pvpQueue = append(h.pvpQueue, c)
	// try to match while we can
	for len(h.pvpQueue) >= 2 {
		a := h.pvpQueue[0]
		b := h.pvpQueue[1]
		h.pvpQueue = h.pvpQueue[2:]

		roomID := makeRoomID("pvp")
		r := NewRoom(roomID, h)
		r.Mode = "queue"
		h.rooms[roomID] = r

		// Randomly select an arena for PvP
		selectedArena := h.selectRandomArena()
		if selectedArena != "" {
			if mapDef, err := loadMapDef(selectedArena); err == nil {
				r.g.mapDef = &mapDef
				log.Printf("SYSTEM: Selected arena %s for PvP: playerBase=%.2f,%.2f enemyBase=%.2f,%.2f",
					selectedArena, mapDef.PlayerBase.X, mapDef.PlayerBase.Y, mapDef.EnemyBase.X, mapDef.EnemyBase.Y)
			} else {
				log.Printf("SYSTEM: Failed to load arena %s for PvP: %v", selectedArena, err)
			}
		}

		// Join with session identities (IDs, names, saved armies)
		sa := h.sessions[a]
		sb := h.sessions[b]
		r.JoinClient(a, sa)
		r.JoinClient(b, sb)
		sa.RoomID = roomID
		sb.RoomID = roomID

		// Tell both clients
		sendJSON(a, "RoomCreated", protocol.RoomCreated{RoomID: roomID})
		sendJSON(b, "RoomCreated", protocol.RoomCreated{RoomID: roomID})

		// Start the match (Init + Gold + Snapshot)
		r.StartBattle()
	}
	h.mu.Unlock()
}

func (h *Hub) DequeuePvp(c *client) {
	h.mu.Lock()
	for i, x := range h.pvpQueue {
		if x == c {
			h.pvpQueue = append(h.pvpQueue[:i], h.pvpQueue[i+1:]...)
			break
		}
	}
	h.mu.Unlock()
}

// selectRandomArena randomly selects an arena for PvP games
func (h *Hub) selectRandomArena() string {
	// Get all available arenas
	arenasDir := filepath.Join("data", "arenas")
	_ = os.MkdirAll(arenasDir, 0o755)

	var arenaIDs []string
	_ = filepath.WalkDir(arenasDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		low := strings.ToLower(d.Name())
		if strings.HasSuffix(low, ".json") {
			// Load the map to check if it's an arena
			id := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
			if def, err := loadMapDef(id); err == nil && def.IsArena {
				arenaIDs = append(arenaIDs, id)
			}
		}
		return nil
	})

	// If no arenas found, return empty string (will use default map)
	if len(arenaIDs) == 0 {
		log.Printf("SYSTEM: No arenas found, using default map for PvP")
		return ""
	}

	// Randomly select one
	selected := arenaIDs[rand.Intn(len(arenaIDs))]
	log.Printf("SYSTEM: Randomly selected arena: %s (from %d available arenas)", selected, len(arenaIDs))
	return selected
}

func (h *Hub) Run() {
	ticker := time.NewTicker(time.Second / 20)
	defer ticker.Stop()
	for range ticker.C {
		// snapshot
		h.mu.Lock()
		rooms := make([]*Room, 0, len(h.rooms))
		for _, r := range h.rooms {
			rooms = append(rooms, r)
		}
		h.mu.Unlock()

		// tick
		for _, r := range rooms {
			r.Tick()
		}

		// prune empties
		h.mu.Lock()
		for id, r := range h.rooms {
			if len(r.players) == 0 {
				delete(h.rooms, id)
			}
		}
		h.mu.Unlock()
	}
}

func (h *Hub) HandleWS(conn *websocket.Conn) {
	c := &client{conn: conn, send: make(chan []byte, 64), name: "Guest"}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	if h.sessions[c] == nil {
		h.sessions[c] = NewSession()
	}
	h.mu.Unlock()

	go c.writer()
	c.reader(h)
}

// HandleWSAuth upgrades a connection that is already authenticated and binds the session to 'username'.
// It also sends the Profile immediately so the client doesn't have to send SetName first.
func (h *Hub) HandleWSAuth(conn *websocket.Conn, username string) {
	c := &client{conn: conn, send: make(chan []byte, 64), name: username}
	h.mu.Lock()
	h.clients[c] = struct{}{}
	if h.sessions[c] == nil {
		s := NewSession()
		s.Name = username
		// Load existing profile (or defaults), bind identity
		prof, err := loadProfile(username)
		if err == nil {
			prof.PlayerID = s.PlayerID
			prof.Name = s.Name
			if prof.Avatar == "" {
				prof.Avatar = "default.png"
			}
			if prof.PvPRating == 0 {
				prof.PvPRating = 1200
			}
			if prof.PvPRank == "" {
				prof.PvPRank = rankName(prof.PvPRating)
			}
			if prof.Armies == nil {
				prof.Armies = map[string][]string{}
			}
			if prof.UnitXP == nil {
				prof.UnitXP = map[string]int{}
			}
			if prof.Resources == nil {
				prof.Resources = map[string]int{}
			}
			s.Profile = prof
			s.Army = append([]string{}, prof.Army...)
		}
		h.sessions[c] = s
	}
	h.mu.Unlock()

	go c.writer()
	if s := h.sessions[c]; s != nil {
		sendJSON(c, "Profile", s.Profile)
	}
	c.reader(h)
}

func (c *client) reader(h *Hub) {
	defer func() {
		h.DequeuePvp(c)
		c.conn.Close()
		h.mu.Lock()
		delete(h.clients, c)
		if c.room != nil {
			c.room.Leave(c)
			c.room = nil
		}
		// remove from PvP queue if applicable
		for i, x := range h.pvpQueue {
			if x == c {
				h.pvpQueue = append(h.pvpQueue[:i], h.pvpQueue[i+1:]...)
				break
			}
		}
		// cancel friendly code if hosting
		if code, ok := h.friendByClient[c]; ok {
			delete(h.friendByClient, c)
			delete(h.friendly, code)
		}
		// remove from guild subscriptions
		for gid, set := range h.guildSubs {
			if _, ok := set[c]; ok {
				delete(set, c)
				if len(set) == 0 {
					delete(h.guildSubs, gid)
				}
			}
		}
		delete(h.sessions, c)
		h.mu.Unlock()
	}()

	// Helper function for logging with account name and timestamp
	logWithAccount := func(accountName, message string) {
		timestamp := time.Now().Format("2006-01-02 15:04:05")
		log.Printf("[%s] %s: %s", timestamp, accountName, message)
	}

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			logWithAccount(c.name, "WebSocket read error: "+err.Error())
			return
		}

		var env protocol.MsgEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			logWithAccount(c.name, "Failed to unmarshal WebSocket message")
			continue
		}
		logWithAccount(c.name, "WS msg type="+env.Type)

		switch env.Type {

		// ---------- Profile / Lobby ----------
		case "SetName":
			var msg protocol.SetName
			_ = json.Unmarshal(env.Data, &msg)

			h.mu.Lock()
			s := h.sessions[c]
			if s == nil {
				s = NewSession()
				h.sessions[c] = s
			}
			s.Name = msg.Name

			// Load Account data as the SINGLE source of truth for ALL progress
			acc, err := account.LoadAccount(s.Name)
			if err != nil {
				log.Printf("LoadAccount error for '%s': %v", s.Name, err)
				// Create new account with full progression data structure
				acc = &account.Account{
					ID:        s.Name, // Use name as ID
					Name:      s.Name,
					Gold:      1000, // Starting gold
					Avatar:    "default.png",
					PvPRating: 1200,
					PvPRank:   "Knight",
					UnitXP:    make(map[string]int),
					Resources: make(map[string]int),
					Armies:    make(map[string][]string),
					Progress:  make(map[string]*types.UnitProgress), // Full progression tracking
				}
				// Immediately save new account to establish single source of truth
				if saveErr := account.SaveAccount(acc); saveErr != nil {
					log.Printf("Initial save failed for '%s': %v", s.Name, saveErr)
				}
			}

			// Convert Account to Profile for session compatibility
			prof := accountToProfile(acc)
			prof.PlayerID = s.PlayerID
			s.Profile = prof
			s.Army = append([]string{}, prof.Army...) // keep legacy field in sync

			h.mu.Unlock()

			// Send Profile to client
			sendJSON(c, "Profile", s.Profile)

			// Send dust and capsule data to client
			sendJSON(c, "DustSynced", protocol.DustSynced{Dust: acc.Dust})
			sendJSON(c, "CapsulesSynced", protocol.CapsulesSynced{
				Capsules: protocol.CapsulesCount{
					Rare:      acc.Capsules.Rare,
					Epic:      acc.Capsules.Epic,
					Legendary: acc.Capsules.Legendary,
				},
			})

			// Send unit progression data for UI display IMMEDIATELY after profile loading
			// This ensures progression data is always up to date with server state
			h.sendUnitProgressionData(acc, c)
		case "GetGuild":
			h.mu.Lock()
			s := h.sessions[c]
			prof := protocol.Profile{}
			if s != nil {
				prof = s.Profile
			}
			gid := strings.TrimSpace(prof.GuildID)
			h.mu.Unlock()

			if gid == "" || h.guilds == nil {
				sendJSON(c, "GuildNone", protocol.GuildNone{})
			} else if gp, ok := h.guilds.BuildProfile(gid); ok {
				// fill online status
				h.mu.Lock()
				for i := range gp.Members {
					name := gp.Members[i].Name
					for cl := range h.clients {
						if ss := h.sessions[cl]; ss != nil && strings.EqualFold(ss.Profile.Name, name) {
							gp.Members[i].Online = true
							break
						}
					}
				}
				h.mu.Unlock()

				sendJSON(c, "GuildInfo", protocol.GuildInfo{Profile: gp})

				// Send guild chat history to the client
				h.mu.Lock()
				if s := h.sessions[c]; s != nil && s.Name != "" {
					if acc, err := account.LoadAccount(s.Name); err == nil {
						chatHistory := acc.GetGuildChatHistory(gid)
						if len(chatHistory) > 0 {
							sendJSON(c, "GuildChatHistory", protocol.GuildChatHistory{Messages: chatHistory})
						}
					}
				}

				// Continue with subscription setup
				subs := h.guildSubs[gid]
				if subs == nil {
					subs = map[*client]struct{}{}
					h.guildSubs[gid] = subs
				}
				subs[c] = struct{}{}
				h.mu.Unlock()
			} else {
				sendJSON(c, "GuildNone", protocol.GuildNone{})
			}
		case "CreateGuild":
			var m protocol.CreateGuild
			_ = json.Unmarshal(env.Data, &m)
			if h.guilds == nil {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Guilds disabled"})
				break
			}
			h.mu.Lock()
			s := h.sessions[c]
			user := ""
			if s != nil {
				user = s.Profile.Name
			}
			h.mu.Unlock()
			g, err := h.guilds.Create(m.Name, m.Desc, m.Privacy, m.Region, user)
			if err != nil {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: err.Error()})
				break
			}
			// Create system message for guild creation
			systemMsg := protocol.GuildChatMsg{
				From:   "",
				Text:   fmt.Sprintf("%s founded the guild", user),
				Ts:     time.Now().UnixMilli(),
				System: true,
			}
			// Send system message through account system
			if acc, loadErr := account.LoadAccount(user); loadErr == nil {
				if saveErr := acc.HandleGuildChatMessage(g.GuildID, systemMsg); saveErr != nil {
					log.Printf("Failed to save guild creation system message: %v", saveErr)
				} else {
					// Broadcast system message to all guild members currently online
					h.mu.Lock()
					for cl := range h.guildSubs[g.GuildID] {
						sendJSON(cl, "GuildChatMsg", systemMsg)
					}
					h.mu.Unlock()
				}
			}
			// store membership on profile
			h.mu.Lock()
			if s := h.sessions[c]; s != nil {
				s.Profile.GuildID = g.GuildID
				_ = saveProfile(s.Profile)
			}
			h.mu.Unlock()
			if gp, ok := h.guilds.BuildProfile(g.GuildID); ok {
				sendJSON(c, "GuildInfo", protocol.GuildInfo{Profile: gp})
			}
		case "JoinGuild":
			var m protocol.JoinGuild
			_ = json.Unmarshal(env.Data, &m)
			if h.guilds == nil {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Guilds disabled"})
				break
			}
			h.mu.Lock()
			s := h.sessions[c]
			user := ""
			if s != nil {
				user = s.Profile.Name
			}
			h.mu.Unlock()
			if err := h.guilds.Join(m.GuildID, user); err != nil {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: err.Error()})
				break
			}
			// Create system message for guild join
			systemMsg := protocol.GuildChatMsg{
				From:   "",
				Text:   fmt.Sprintf("%s joined the guild", user),
				Ts:     time.Now().UnixMilli(),
				System: true,
			}
			// Send system message through account system
			if acc, loadErr := account.LoadAccount(user); loadErr == nil {
				if saveErr := acc.HandleGuildChatMessage(m.GuildID, systemMsg); saveErr != nil {
					log.Printf("Failed to save guild join system message: %v", saveErr)
				} else {
					// Broadcast system message to all guild members currently online
					h.mu.Lock()
					for cl := range h.guildSubs[m.GuildID] {
						sendJSON(cl, "GuildChatMsg", systemMsg)
					}
					h.mu.Unlock()
				}
			}
			h.mu.Lock()
			if s := h.sessions[c]; s != nil {
				s.Profile.GuildID = m.GuildID
				_ = saveProfile(s.Profile)
			}
			h.mu.Unlock()
			if gp, ok := h.guilds.BuildProfile(m.GuildID); ok {
				sendJSON(c, "GuildInfo", protocol.GuildInfo{Profile: gp})
			}
		case "LeaveGuild":
			if h.guilds == nil {
				break
			}
			h.mu.Lock()
			s := h.sessions[c]
			gid := ""
			user := ""
			if s != nil {
				gid = s.Profile.GuildID
				user = s.Profile.Name
			}
			// also unsubscribe from guild chat set immediately
			if gid != "" {
				if set := h.guildSubs[gid]; set != nil {
					delete(set, c)
					if len(set) == 0 {
						delete(h.guildSubs, gid)
					}
				}
			}
			h.mu.Unlock()
			if gid != "" {
				// Create system message for guild leave before leaving
				systemMsg := protocol.GuildChatMsg{
					From:   "",
					Text:   fmt.Sprintf("%s left the guild", user),
					Ts:     time.Now().UnixMilli(),
					System: true,
				}
				// Send system message through account system
				if acc, loadErr := account.LoadAccount(user); loadErr == nil {
					if saveErr := acc.HandleGuildChatMessage(gid, systemMsg); saveErr != nil {
						log.Printf("Failed to save guild leave system message: %v", saveErr)
					} else {
						// Broadcast system message to all guild members currently online
						h.mu.Lock()
						for cl := range h.guildSubs[gid] {
							sendJSON(cl, "GuildChatMsg", systemMsg)
						}
						h.mu.Unlock()
					}
				}

				_ = h.guilds.Leave(gid, user)
				h.mu.Lock()
				if s := h.sessions[c]; s != nil {
					s.Profile.GuildID = ""
					_ = saveProfile(s.Profile)
				}
				h.mu.Unlock()
			}
			sendJSON(c, "GuildNone", protocol.GuildNone{})
		case "ListGuilds":
			var m protocol.ListGuilds
			_ = json.Unmarshal(env.Data, &m)
			if h.guilds == nil {
				sendJSON(c, "GuildList", protocol.GuildList{Items: nil})
				break
			}
			items := h.guilds.List(m.Query)
			sendJSON(c, "GuildList", protocol.GuildList{Items: items})
		case "PromoteMember":
			var m protocol.PromoteMember
			_ = json.Unmarshal(env.Data, &m)
			h.mu.Lock()
			s := h.sessions[c]
			gid := ""
			actor := ""
			if s != nil {
				gid = s.Profile.GuildID
				actor = s.Profile.Name
			}
			h.mu.Unlock()
			if gid == "" || h.guilds == nil {
				break
			}
			if err := h.guilds.SetRole(gid, actor, m.User, "officer"); err != nil {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: err.Error()})
			} else {
				// Create system message for promotion on success only
				systemMsg := protocol.GuildChatMsg{
					From:   "",
					Text:   fmt.Sprintf("%s was promoted to officer by %s", m.User, actor),
					Ts:     time.Now().UnixMilli(),
					System: true,
				}
				// Send system message through account system
				if acc, loadErr := account.LoadAccount(actor); loadErr == nil {
					if saveErr := acc.HandleGuildChatMessage(gid, systemMsg); saveErr != nil {
						log.Printf("Failed to save guild promotion system message: %v", saveErr)
					}
				}
			}
			if gp, ok := h.guilds.BuildProfile(gid); ok {
				sendJSON(c, "GuildInfo", protocol.GuildInfo{Profile: gp})
			}
		case "DemoteMember":
			var m protocol.DemoteMember
			_ = json.Unmarshal(env.Data, &m)
			h.mu.Lock()
			s := h.sessions[c]
			gid := ""
			actor := ""
			if s != nil {
				gid = s.Profile.GuildID
				actor = s.Profile.Name
			}
			h.mu.Unlock()
			if gid == "" || h.guilds == nil {
				break
			}
			if err := h.guilds.SetRole(gid, actor, m.User, "member"); err != nil {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: err.Error()})
			} else {
				// Create system message for demotion on success only
				systemMsg := protocol.GuildChatMsg{
					From:   "",
					Text:   fmt.Sprintf("%s was demoted to member by %s", m.User, actor),
					Ts:     time.Now().UnixMilli(),
					System: true,
				}
				// Send system message through account system
				if acc, loadErr := account.LoadAccount(actor); loadErr == nil {
					if saveErr := acc.HandleGuildChatMessage(gid, systemMsg); saveErr != nil {
						log.Printf("Failed to save guild demotion system message: %v", saveErr)
					}
				}
			}
			if gp, ok := h.guilds.BuildProfile(gid); ok {
				sendJSON(c, "GuildInfo", protocol.GuildInfo{Profile: gp})
			}
		case "KickMember":
			var m protocol.KickMember
			_ = json.Unmarshal(env.Data, &m)
			h.mu.Lock()
			s := h.sessions[c]
			gid := ""
			actor := ""
			if s != nil {
				gid = s.Profile.GuildID
				actor = s.Profile.Name
			}
			h.mu.Unlock()
			if gid == "" || h.guilds == nil {
				break
			}
			if err := h.guilds.Kick(gid, actor, m.User); err != nil {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: err.Error()})
			} else {
				// Create system message for kick on success only
				systemMsg := protocol.GuildChatMsg{
					From:   "",
					Text:   fmt.Sprintf("%s was kicked by %s", m.User, actor),
					Ts:     time.Now().UnixMilli(),
					System: true,
				}
				// Send system message through account system
				if acc, loadErr := account.LoadAccount(actor); loadErr == nil {
					if saveErr := acc.HandleGuildChatMessage(gid, systemMsg); saveErr != nil {
						log.Printf("Failed to save guild kick system message: %v", saveErr)
					}
				}
			}
			if gp, ok := h.guilds.BuildProfile(gid); ok {
				sendJSON(c, "GuildInfo", protocol.GuildInfo{Profile: gp})
			}
		case "TransferLeader":
			var m protocol.TransferLeader
			_ = json.Unmarshal(env.Data, &m)
			h.mu.Lock()
			s := h.sessions[c]
			gid := ""
			actor := ""
			if s != nil {
				gid = s.Profile.GuildID
				actor = s.Profile.Name
			}
			h.mu.Unlock()
			if gid == "" || h.guilds == nil {
				break
			}
			if err := h.guilds.SetRole(gid, actor, m.To, "leader"); err != nil {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: err.Error()})
			} else {
				// Create system message for leadership transfer on success only
				systemMsg := protocol.GuildChatMsg{
					From:   "",
					Text:   fmt.Sprintf("%s transferred leadership to %s", actor, m.To),
					Ts:     time.Now().UnixMilli(),
					System: true,
				}
				// Send system message through account system
				if acc, loadErr := account.LoadAccount(actor); loadErr == nil {
					if saveErr := acc.HandleGuildChatMessage(gid, systemMsg); saveErr != nil {
						log.Printf("Failed to save guild leadership transfer system message: %v", saveErr)
					}
				}
			}
			if gp, ok := h.guilds.BuildProfile(gid); ok {
				sendJSON(c, "GuildInfo", protocol.GuildInfo{Profile: gp})
			}
		case "SetGuildDesc":
			var m protocol.SetGuildDesc
			_ = json.Unmarshal(env.Data, &m)
			h.mu.Lock()
			s := h.sessions[c]
			gid := ""
			actor := ""
			if s != nil {
				gid = s.Profile.GuildID
				actor = s.Profile.Name
			}
			h.mu.Unlock()
			if gid == "" || h.guilds == nil {
				break
			}
			if err := h.guilds.SetDesc(gid, actor, strings.TrimSpace(m.Desc)); err != nil {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: err.Error()})
			} else {
				// Create system message for guild description update on success only
				systemMsg := protocol.GuildChatMsg{
					From:   "",
					Text:   fmt.Sprintf("%s updated the guild description", actor),
					Ts:     time.Now().UnixMilli(),
					System: true,
				}
				// Send system message through account system
				if acc, loadErr := account.LoadAccount(actor); loadErr == nil {
					if saveErr := acc.HandleGuildChatMessage(gid, systemMsg); saveErr != nil {
						log.Printf("Failed to save guild description update system message: %v", saveErr)
					}
				}
			}
			if gp, ok := h.guilds.BuildProfile(gid); ok {
				sendJSON(c, "GuildInfo", protocol.GuildInfo{Profile: gp})
			}
		case "GuildChatSend":
			var gm protocol.GuildChatSend
			_ = json.Unmarshal(env.Data, &gm)
			txt := strings.TrimSpace(gm.Text)
			if txt == "" {
				break
			}
			h.mu.Lock()
			s := h.sessions[c]
			gid := ""
			from := ""
			if s != nil {
				gid = s.Profile.GuildID
				from = s.Profile.Name
			}
			subs := h.guildSubs[gid]
			if subs == nil {
				subs = map[*client]struct{}{}
				h.guildSubs[gid] = subs
			}
			subs[c] = struct{}{}
			h.mu.Unlock()
			if gid == "" {
				break
			}
			msg := protocol.GuildChatMsg{From: from, Text: txt, Ts: time.Now().UnixMilli(), System: false}

			// Server-side guild chat persistence through account system
			if from != "" {
				// Load account and save guild chat message
				acc, err := account.LoadAccount(from)
				if err != nil {
					log.Printf("Failed to load account for guild chat persistence: %v", err)
				} else {
					if saveErr := acc.HandleGuildChatMessage(gid, msg); saveErr != nil {
						log.Printf("Failed to save guild chat message: %v", saveErr)
					}
				}
			}

			// Send to all subscribed guild members
			h.mu.Lock()
			for cl := range h.guildSubs[gid] {
				sendJSON(cl, "GuildChatMsg", msg)
			}
			h.mu.Unlock()

		// -------- Friends / DMs --------
		case "GetFriends":
			h.mu.Lock()
			s := h.sessions[c]
			user := ""
			if s != nil {
				user = s.Profile.Name
			}
			h.mu.Unlock()
			if h.social == nil {
				sendJSON(c, "FriendsList", protocol.FriendsList{})
				break
			}
			names := h.social.ListFriends(user)
			items := make([]protocol.FriendInfo, 0, len(names))
			for _, n := range names {
				online := false
				h.mu.Lock()
				for cl := range h.clients {
					if ss := h.sessions[cl]; ss != nil && strings.EqualFold(ss.Profile.Name, n) {
						online = true
						break
					}
				}
				h.mu.Unlock()
				items = append(items, protocol.FriendInfo{Name: n, Online: online})
			}
			sendJSON(c, "FriendsList", protocol.FriendsList{Items: items})
		case "AddFriend":
			var m protocol.AddFriend
			_ = json.Unmarshal(env.Data, &m)
			h.mu.Lock()
			s := h.sessions[c]
			user := ""
			if s != nil {
				user = s.Profile.Name
			}
			h.mu.Unlock()
			if h.social != nil && user != "" && strings.TrimSpace(m.Name) != "" {
				target := strings.TrimSpace(m.Name)
				// Disallow adding self
				if strings.EqualFold(user, target) {
					sendJSON(c, "Error", protocol.ErrorMsg{Message: "Cannot add yourself"})
					break
				}
				// Verify target exists: either has a stored profile file or currently online
				exists := false
				// Check online sessions
				h.mu.Lock()
				for cl := range h.clients {
					if ss := h.sessions[cl]; ss != nil && strings.EqualFold(ss.Profile.Name, target) {
						exists = true
						break
					}
				}
				h.mu.Unlock()
				if !exists {
					// Check persisted profile file without creating defaults
					path := profilePath(target)
					if _, err := os.Stat(path); err == nil {
						exists = true
					}
				}
				if !exists {
					sendJSON(c, "Error", protocol.ErrorMsg{Message: "Player not found"})
					break
				}
				// Proceed to add both ways
				h.social.AddFriend(user, target)
				// resend friends (with online flags)
				names := h.social.ListFriends(user)
				items := make([]protocol.FriendInfo, 0, len(names))
				for _, n := range names {
					online := false
					h.mu.Lock()
					for cl := range h.clients {
						if ss := h.sessions[cl]; ss != nil && strings.EqualFold(ss.Profile.Name, n) {
							online = true
							break
						}
					}
					h.mu.Unlock()
					items = append(items, protocol.FriendInfo{Name: n, Online: online})
				}
				sendJSON(c, "FriendsList", protocol.FriendsList{Items: items})
			}
		case "RemoveFriend":
			var m protocol.RemoveFriend
			_ = json.Unmarshal(env.Data, &m)
			h.mu.Lock()
			s := h.sessions[c]
			user := ""
			if s != nil {
				user = s.Profile.Name
			}
			h.mu.Unlock()
			if h.social != nil && user != "" && m.Name != "" {
				h.social.RemoveFriend(user, m.Name)
				names := h.social.ListFriends(user)
				items := make([]protocol.FriendInfo, 0, len(names))
				for _, n := range names {
					items = append(items, protocol.FriendInfo{Name: n, Online: false})
				}
				sendJSON(c, "FriendsList", protocol.FriendsList{Items: items})
			}
		case "SendFriendDM":
			var m protocol.SendFriendDM
			_ = json.Unmarshal(env.Data, &m)
			txt := strings.TrimSpace(m.Text)
			if txt == "" {
				break
			}
			h.mu.Lock()
			s := h.sessions[c]
			from := ""
			if s != nil {
				from = s.Profile.Name
			}
			h.mu.Unlock()
			if from == "" {
				break
			}
			dm := h.social.AppendDM(from, m.To, txt)
			// deliver to both participants if online
			h.mu.Lock()
			for cl := range h.clients {
				if ss := h.sessions[cl]; ss != nil && (strings.EqualFold(ss.Profile.Name, m.To) || strings.EqualFold(ss.Profile.Name, from)) {
					sendJSON(cl, "FriendDM", dm)
				}
			}
			h.mu.Unlock()
		case "GetFriendHistory":
			var m protocol.GetFriendHistory
			_ = json.Unmarshal(env.Data, &m)
			h.mu.Lock()
			s := h.sessions[c]
			user := ""
			if s != nil {
				user = s.Profile.Name
			}
			h.mu.Unlock()
			if user == "" || h.social == nil {
				break
			}
			items := h.social.History(user, m.With, m.Limit)
			sendJSON(c, "FriendHistory", protocol.FriendHistory{With: m.With, Items: items})
		case "SetAvatar":
			var m protocol.SetAvatar
			_ = json.Unmarshal(env.Data, &m)

			h.mu.Lock()
			s := h.sessions[c]
			username := ""
			if s != nil {
				username = s.Name
			}
			h.mu.Unlock()

			if username == "" {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Not authenticated"})
				break
			}

			// simple validation: filename-ish and png/jpg
			a := strings.TrimSpace(m.Avatar)
			a = strings.ReplaceAll(a, "\\", "/")
			if strings.Contains(a, "/") || strings.Contains(a, "..") || a == "" {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Invalid avatar"})
				break
			}
			low := strings.ToLower(a)
			if !(strings.HasSuffix(low, ".png") || strings.HasSuffix(low, ".jpg") || strings.HasSuffix(low, ".jpeg")) {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Avatar must be .png or .jpg"})
				break
			}

			// Update avatar through account system
			acc, err := account.LoadAccount(username)
			if err != nil {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Failed to load account: " + err.Error()})
				break
			}

			if avatarErr := acc.HandleSetAvatar(username, a); avatarErr != nil {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Failed to set avatar: " + avatarErr.Error()})
				break
			}

			// Update session profile with fresh account data
			h.mu.Lock()
			if s := h.sessions[c]; s != nil {
				// Convert account back to profile
				s.Profile = accountToProfile(acc)
				log.Printf("AVATAR: %s set avatar to %s", username, a)
			}
			prof := s.Profile
			h.mu.Unlock()

			// Send updated profile back to client
			sendJSON(c, "Profile", prof)

			// Send AvatarSet response to client for confirmation
			avatarSetMsg := protocol.AvatarSet{AvatarName: a}
			sendJSON(c, "AvatarSet", avatarSetMsg)

		case "GetProfile":
			h.mu.Lock()
			s := h.sessions[c]
			var prof protocol.Profile
			if s != nil {
				prof = s.Profile
			}
			h.mu.Unlock()
			sendJSON(c, "Profile", prof)

		case "GetUserProfile":
			var m protocol.GetUserProfile
			_ = json.Unmarshal(env.Data, &m)
			name := strings.TrimSpace(m.Name)
			if name == "" {
				break
			}
			prof, err := loadProfile(name)
			if err == nil {
				// fill defaults like elsewhere
				if prof.PvPRating == 0 {
					prof.PvPRating = 1200
				}
				if prof.PvPRank == "" {
					prof.PvPRank = rankName(prof.PvPRating)
				}
				if prof.Avatar == "" {
					prof.Avatar = "default.png"
				}
			}
			sendJSON(c, "UserProfile", protocol.UserProfile{Profile: prof})

		case "SaveArmy":
			var msg protocol.SaveArmy
			_ = json.Unmarshal(env.Data, &msg)
			if len(msg.Cards) != 7 {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "SaveArmy requires 7 cards: [champion, 6 minis]"})
				break
			}
			ch := msg.Cards[0]
			minis := msg.Cards[1:]

			h.mu.Lock()
			s := h.sessions[c]
			if s == nil {
				s = NewSession()
				h.sessions[c] = s
			}
			if s.Profile.Armies == nil {
				s.Profile.Armies = map[string][]string{}
			}
			// update active + per-champion
			s.Profile.Army = append([]string{}, msg.Cards...)
			s.Profile.Armies[ch] = append([]string{}, minis...)
			// legacy sync
			s.Army = append([]string{}, msg.Cards...)

			// Update account with army composition - DO NOT save profile here to avoid losing progression data
			// Progression data is stored in separate account files and should not be affected by army saves
			if acc, accountLoadErr := account.LoadAccount(s.Profile.Name); accountLoadErr == nil {
				// Update army in account (preserves progression data)
				acc.Army = append([]string{}, msg.Cards...)
				if acc.Armies == nil {
					acc.Armies = make(map[string][]string)
				}
				acc.Armies[ch] = append([]string{}, minis...)

				// Save account data through account system (preserves progression)
				if saveErr := account.SaveAccount(acc); saveErr != nil {
					log.Printf("Failed to save account army data for %s: %v", s.Profile.Name, saveErr)
				}
			}

			prof := s.Profile
			h.mu.Unlock()

			sendJSON(c, "Profile", prof)

		case "ListMinis":
			minis := LoadLobbyMinis()
			sendJSON(c, "Minis", protocol.Minis{Items: minis})

		case "ListMaps":
			maps := listMaps()
			sendJSON(c, "Maps", protocol.Maps{Items: maps})
		case "GetMap":
			var m protocol.GetMap
			_ = json.Unmarshal(env.Data, &m)
			if strings.TrimSpace(m.ID) == "" {
				break
			}
			if def, err := loadMapDef(m.ID); err == nil {
				sendJSON(c, "MapDef", protocol.MapDefMsg{Def: def})
			} else {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Map not found"})
			}
		case "SaveMap":
			var m protocol.SaveMap
			_ = json.Unmarshal(env.Data, &m)
			if strings.TrimSpace(m.Def.ID) == "" && strings.TrimSpace(m.Def.Name) == "" {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Map requires id or name"})
				break
			}
			if err := saveMapDef(m.Def); err != nil {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: err.Error()})
				break
			}
			// Return updated list and the saved def
			sendJSON(c, "MapDef", protocol.MapDefMsg{Def: m.Def})
			maps := listMaps()
			sendJSON(c, "Maps", protocol.Maps{Items: maps})

		// ---------- Room / PvE ----------
		case "CreatePve":
			var m protocol.CreatePve
			_ = json.Unmarshal(env.Data, &m)

			roomID := fmt.Sprintf("pve-%d", protocol.NewID())

			h.mu.Lock()
			r := NewRoom(roomID, h)
			r.Mode = "pve"
			h.rooms[roomID] = r
			s := h.sessions[c]
			if s == nil {
				s = NewSession()
				h.sessions[c] = s
			}
			// Load map definition fresh each time (no caching)
			if mapDef, err := loadMapDef(m.MapID); err == nil {
				r.g.mapDef = &mapDef
				logWithAccount(c.name, fmt.Sprintf("SYSTEM: Player started PVE battle - loaded map %s.json", m.MapID))
			} else {
				logWithAccount(c.name, "Failed to load map "+m.MapID+" for PvE: "+err.Error())
			}
			// Join with the session identity so c.id == s.PlayerID
			r.JoinClient(c, s)
			s.RoomID = roomID
			h.mu.Unlock()

			sendJSON(c, "RoomCreated", protocol.RoomCreated{RoomID: roomID})
		case "JoinPvpQueue":
			h.EnqueuePvp(c)
		case "LeavePvpQueue":
			h.DequeuePvp(c)
		case "FriendlyCreate":
			h.FriendlyCreate(c)
		case "FriendlyCancel":
			h.FriendlyCancel(c)
		case "FriendlyJoin":
			var m protocol.FriendlyJoin
			_ = json.Unmarshal(env.Data, &m)
			h.FriendlyJoin(c, m.Code)
		case "GetLeaderboard":
			lb := h.buildLeaderboardTop50()
			sendJSON(c, "Leaderboard", lb)
		case "StartBattle":
			h.mu.Lock()
			r := c.room
			// if somehow not in r.g yet, join with session first
			if r != nil && r.g.players[c.id] == nil {
				if s := h.sessions[c]; s != nil {
					r.JoinClient(c, s)
				}
			}
			h.mu.Unlock()

			if r == nil {
				log.Printf("StartBattle requested by player=%d but no room", c.id)
				break
			}
			log.Printf("StartBattle requested by player=%d room=%s", c.id, r.id)
			r.StartBattle()

		case "LeaveRoom":
			h.mu.Lock()
			if c.room != nil {
				c.room.Leave(c)
				c.room = nil
			}
			if s := h.sessions[c]; s != nil {
				s.RoomID = ""
			}
			h.mu.Unlock()

		// ---------- Timer and Pause Controls (PvE only) ----------
		case "PauseGame":
			if c.room != nil && c.room.Mode == "pve" {
				c.room.g.PauseTimer()
				// Send timer update to all players in room
				for _, p := range c.room.players {
					remaining, paused := c.room.g.GetTimerState()
					sendJSON(p, "TimerUpdate", protocol.TimerUpdate{
						RemainingSeconds: remaining,
						IsPaused:         paused,
					})
				}
			}
		case "ResumeGame":
			if c.room != nil && c.room.Mode == "pve" {
				c.room.g.ResumeTimer()
				// Send timer update to all players in room
				for _, p := range c.room.players {
					remaining, paused := c.room.g.GetTimerState()
					sendJSON(p, "TimerUpdate", protocol.TimerUpdate{
						RemainingSeconds: remaining,
						IsPaused:         paused,
					})
				}
			}
		case "RestartMatch":
			if c.room != nil && c.room.Mode == "pve" {
				c.room.g.RestartMatch()
				// Send updated snapshots to all players
				for _, p := range c.room.players {
					snap := c.room.g.FullSnapshot()
					sendJSON(p, "FullSnapshot", snap)
					if pl := c.room.g.players[p.id]; pl != nil {
						sendJSON(p, "GoldUpdate", protocol.GoldUpdate{
							PlayerID: pl.ID,
							Gold:     pl.Gold,
						})
					}
					remaining, paused := c.room.g.GetTimerState()
					sendJSON(p, "TimerUpdate", protocol.TimerUpdate{
						RemainingSeconds: remaining,
						IsPaused:         paused,
					})
				}
			}
		case "SurrenderMatch":
			if c.room != nil && c.room.Mode == "pve" {
				winnerID := c.room.g.SurrenderMatch(c.id)
				for _, p := range c.room.players {
					sendJSON(p, "GameOver", protocol.GameOver{WinnerID: winnerID})
				}
				c.room.active = false
			}

		// ---------- Gameplay ----------
		case "DeployMiniAt":
			var m protocol.DeployMiniAt
			_ = json.Unmarshal(env.Data, &m)
			if c.room != nil {
				c.room.HandleDeploy(c, m)
			}

		case "CastSpell":
			var m protocol.CastSpell
			_ = json.Unmarshal(env.Data, &m)
			if c.room != nil {
				c.room.HandleSpellCast(c, m)
			}

		// ---------- Currency Operations ----------
		case "GrantGold":
			var m protocol.GrantGold
			_ = json.Unmarshal(env.Data, &m)
			h.mu.Lock()
			s := h.sessions[c]
			if s != nil {
				ctx := currency.SessionCtx{AccountID: fmt.Sprintf("%d", s.PlayerID)}
				if err := currency.HandleGrantGold(&ctx, m); err != nil {
					if currencyErr, ok := err.(*currency.CurrencyError); ok {
						sendJSON(c, "Error", protocol.Error{Code: currencyErr.Code, Message: currencyErr.Message})
					} else {
						sendJSON(c, "Error", protocol.ErrorMsg{Message: err.Error()})
					}
				} else {
					// If successful, push updated gold to client
					if err := currency.PushGoldSynced(&ctx, 0); err != nil {
						log.Printf("push gold sync failed: %v", err)
					}
				}
			}
			h.mu.Unlock()

		case "SpendGold":
			var m protocol.SpendGold
			_ = json.Unmarshal(env.Data, &m)
			h.mu.Lock()
			s := h.sessions[c]
			if s != nil {
				ctx := currency.SessionCtx{AccountID: fmt.Sprintf("%d", s.PlayerID)}
				if err := currency.HandleSpendGold(&ctx, m); err != nil {
					if currencyErr, ok := err.(*currency.CurrencyError); ok {
						sendJSON(c, "Error", protocol.Error{Code: currencyErr.Code, Message: currencyErr.Message})
					} else {
						sendJSON(c, "Error", protocol.ErrorMsg{Message: err.Error()})
					}
				} else {
					// If successful, push updated gold to client
					if err := currency.PushGoldSynced(&ctx, 0); err != nil {
						log.Printf("push gold sync failed: %v", err)
					}
				}
			}
			h.mu.Unlock()

		case "Ready":
			if c.room != nil {
				c.room.MarkReady(c)
			}

		case "Logout":
			h.mu.Lock()
			// best-effort cleanups
			if c.room != nil {
				c.room.Leave(c)
				c.room = nil
			}
			// Remove from PvP queue WITHOUT re-locking (we already hold h.mu)
			for i, x := range h.pvpQueue {
				if x == c {
					h.pvpQueue = append(h.pvpQueue[:i], h.pvpQueue[i+1:]...)
					break
				}
			}
			if code, ok := h.friendByClient[c]; ok {
				delete(h.friendByClient, c)
				delete(h.friendly, code)
			}
			delete(h.sessions, c) // drop session so next login gets a fresh one
			h.mu.Unlock()

			// tell the client it's ok to close from their side
			sendJSON(c, "LoggedOut", struct{}{})
			// DO NOT close c.conn here; just continue the read loop.
			// The client will Close(); our reader will then get an error,
			// the defer will run, and we’ll clean up this client safely.

		// Shop handlers
		case "GetShopRoll":
			var req protocol.GetShopRollReq
			_ = json.Unmarshal(env.Data, &req)
			h.mu.Lock()
			s := h.sessions[c]
			username := ""
			if s != nil {
				username = s.Name // Use username from session
			}
			h.mu.Unlock()

			if username == "" {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Not authenticated"})
				break
			}

			// Call shop service through hub's shop handler
			if h.shopService != nil {
				err := h.shopService.HandleGetShopRoll(username, func(eventType string, event interface{}) {
					sendJSON(c, eventType, event)
				})
				if err != nil {
					sendJSON(c, "Error", protocol.ErrorMsg{Message: err.Error()})
				}
			} else {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Shop service unavailable"})
			}

		case "RerollShop":
			var req protocol.RerollShopReq
			_ = json.Unmarshal(env.Data, &req)
			h.mu.Lock()
			s := h.sessions[c]
			username := ""
			if s != nil {
				username = s.Name // Use username from session
			}
			h.mu.Unlock()

			if username == "" {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Not authenticated"})
				break
			}

			if h.shopService != nil {
				// Convert protocol type to types type
				reqTypes := types.RerollShopReq{req.Nonce}
				err := h.shopService.HandleRerollShop(username, reqTypes, func(eventType string, event interface{}) {
					sendJSON(c, eventType, event)
				})
				if err != nil {
					sendJSON(c, "Error", protocol.ErrorMsg{Message: err.Error()})
				}
			} else {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Shop service unavailable"})
			}

		case "BuyShopSlot":
			var req protocol.BuyShopSlotReq
			_ = json.Unmarshal(env.Data, &req)
			h.mu.Lock()
			s := h.sessions[c]
			username := ""
			if s != nil {
				username = s.Name // Use username from session
			}
			h.mu.Unlock()

			if username == "" {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Not authenticated"})

				break
			}

			if h.shopService != nil {
				// Convert protocol type to types type
				reqTypes := types.BuyShopSlotReq{Slot: req.Slot, Nonce: req.Nonce}
				err := h.shopService.HandleBuyShopSlot(username, reqTypes, func(eventType string, event interface{}) {
					sendJSON(c, eventType, event)
				})
				if err != nil {
					sendJSON(c, "Error", protocol.ErrorMsg{Message: err.Error()})
				}
			} else {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Shop service unavailable"})
			}

		case "UpgradeUnit":
			var req protocol.UpgradeUnit
			_ = json.Unmarshal(env.Data, &req)

			h.mu.Lock()
			s := h.sessions[c]
			username := ""
			if s != nil {
				username = s.Name // Use username from session
			}
			h.mu.Unlock()

			if username == "" {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Not authenticated"})
				break
			}

			// Handle unit upgrade directly through account service
			// Load the account to perform the upgrade
			acc, accountErr := account.LoadAccount(username)
			if accountErr != nil {
				sendJSON(c, "Error", protocol.ErrorMsg{Message: "Failed to load account: " + accountErr.Error()})
				break
			}

			if err := acc.HandleUpgradeUnit(username, req.UnitID); err != nil {
				sendJSON(c, "UpgradeResult", protocol.UpgradeResult{
					Success: false,
					UnitID:  req.UnitID,
					Reason:  err.Error(),
				})
			} else {
				// Reload account after upgrade to get fresh data
				acc, accountErr = account.LoadAccount(username)
				if accountErr != nil {
					sendJSON(c, "Error", protocol.ErrorMsg{Message: "Failed to reload account: " + accountErr.Error()})
					break
				}

				// Update session profile with fresh account data
				h.mu.Lock()
				if s := h.sessions[c]; s != nil {
					s.Profile = accountToProfile(acc)
					logWithAccount(username, fmt.Sprintf("Upgrade success: %s rank %d, shards %d, dust %d, capsules (%d rare, %d epic, %d legendary)",
						req.UnitID, acc.Progress[req.UnitID].Rank, acc.Progress[req.UnitID].ShardsOwned, acc.Dust, acc.Capsules.Rare, acc.Capsules.Epic, acc.Capsules.Legendary))
				}
				h.mu.Unlock()

				// Send success response with upgraded unit details
				sendJSON(c, "UpgradeResult", protocol.UpgradeResult{
					Success:   true,
					UnitID:    req.UnitID,
					NewRank:   acc.Progress[req.UnitID].Rank,
					NewShards: acc.Progress[req.UnitID].ShardsOwned,
				})

				// IMPORTANT: Sync the unit progression data back to client
				// This prevents the client from showing stale shard counts
				h.sendUnitProgressionData(acc, c)

				// Send updated UnitXP to client so level increases are visible immediately
				sendJSON(c, "UnitXPUpdate", protocol.UnitXPUpdate{
					UnitXP: acc.UnitXP,
				})

				// Send updated gold to client (dust costs consume gold equivalent)
				sendJSON(c, "GoldUpdate", protocol.GoldUpdate{
					PlayerID: s.PlayerID,
					Gold:     int(acc.Gold),
				})

				// Send updated dust count to client
				sendJSON(c, "DustSynced", protocol.DustSynced{Dust: acc.Dust})

				// Send updated capsule counts to client
				sendJSON(c, "CapsulesSynced", protocol.CapsulesSynced{
					Capsules: protocol.CapsulesCount{
						Rare:      acc.Capsules.Rare,
						Epic:      acc.Capsules.Epic,
						Legendary: acc.Capsules.Legendary,
					},
				})
			}

		default:
			sendJSON(c, "Error", protocol.ErrorMsg{Message: "Unknown message type: " + env.Type})
		}
	}
}

func (c *client) writer() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}

func sendJSON(c *client, typ string, v interface{}) {
	b, _ := json.Marshal(v)
	env := protocol.MsgEnvelope{Type: typ, Data: b}
	out, _ := json.Marshal(env)
	select {
	case c.send <- out:
	default:
	}
}

/* ------------------- simple JSON persistence (per account) ------------------- */

var profilesDir = filepath.Join("data", "profiles")

func safeFileName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	s := re.ReplaceAllString(name, "_")
	if s == "" {
		s = "player"
	}
	return s
}

func ensureProfilesDir() error {
	return os.MkdirAll(profilesDir, 0o755)
}

func profilePath(name string) string {
	return filepath.Join(profilesDir, safeFileName(name)+".json")
}

func loadProfile(name string) (protocol.Profile, error) {
	_ = ensureProfilesDir()
	path := profilePath(name)
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return protocol.Profile{
			Name:      name,
			Army:      nil,
			Armies:    map[string][]string{},
			Gold:      0,
			AccountXP: 0,
			UnitXP:    map[string]int{},
			Resources: map[string]int{},
		}, nil
	}
	if err != nil {
		return protocol.Profile{}, err
	}
	var p protocol.Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return protocol.Profile{}, err
	}
	if p.Avatar == "" {
		p.Avatar = "default.png"
	}
	// PvP defaults if missing/zero
	if p.PvPRating == 0 {
		p.PvPRating = 1200
	}
	if p.PvPRank == "" {
		p.PvPRank = rankName(p.PvPRating)
	}
	if p.Armies == nil {
		p.Armies = map[string][]string{}
	}
	if p.UnitXP == nil {
		p.UnitXP = map[string]int{}
	}
	if p.Resources == nil {
		p.Resources = map[string]int{}
	}
	return p, nil
}

func saveProfile(p protocol.Profile) error {
	_ = ensureProfilesDir()
	path := profilePath(p.Name)
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func genCode(n int) string {
	const alphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O/1/I to avoid confusion
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[rand.Intn(len(alphabet))]
	}
	return string(b)
}

func (h *Hub) FriendlyCreate(c *client) {
	h.mu.Lock()
	// already hosting? re-send same code
	if code, ok := h.friendByClient[c]; ok {
		h.mu.Unlock()
		sendJSON(c, "FriendlyCode", protocol.FriendlyCode{Code: code})
		return
	}

	// generate unique code
	var code string
	for {
		code = genCode(6)
		if _, exists := h.friendly[code]; !exists {
			break
		}
	}
	h.friendly[code] = c
	h.friendByClient[c] = code
	h.mu.Unlock()

	sendJSON(c, "FriendlyCode", protocol.FriendlyCode{Code: code})
}

func (h *Hub) FriendlyCancel(c *client) {
	h.mu.Lock()
	if code, ok := h.friendByClient[c]; ok {
		delete(h.friendByClient, c)
		delete(h.friendly, code)
	}
	h.mu.Unlock()
}

func (h *Hub) FriendlyJoin(c *client, code string) {
	code = strings.ToUpper(strings.TrimSpace(code))

	h.mu.Lock()
	host := h.friendly[code]
	if host == nil {
		h.mu.Unlock()
		sendJSON(c, "Error", protocol.ErrorMsg{Message: "Code not found"})
		return
	}
	// simple guards: don’t let people join if either side is already in a room
	if sh := h.sessions[host]; sh != nil && sh.RoomID != "" {
		h.mu.Unlock()
		sendJSON(c, "Error", protocol.ErrorMsg{Message: "Host is already in a room"})
		return
	}
	if sc := h.sessions[c]; sc != nil && sc.RoomID != "" {
		h.mu.Unlock()
		sendJSON(c, "Error", protocol.ErrorMsg{Message: "You are already in a room"})
		return
	}

	// consume the code
	delete(h.friendly, code)
	delete(h.friendByClient, host)

	// create & register room
	roomID := makeRoomID("frd")
	r := NewRoom(roomID, h)
	r.Mode = "friendly"
	h.rooms[roomID] = r

	// join both with session identity (IDs, names, saved armies)
	sa := h.sessions[host]
	sb := h.sessions[c]
	r.JoinClient(host, sa)
	r.JoinClient(c, sb)
	if sa != nil {
		sa.RoomID = roomID
	}
	if sb != nil {
		sb.RoomID = roomID
	}
	h.mu.Unlock()

	// notify & start
	sendJSON(host, "RoomCreated", protocol.RoomCreated{RoomID: roomID})
	sendJSON(c, "RoomCreated", protocol.RoomCreated{RoomID: roomID})
	r.StartBattle()
}

var friendlyHosts = map[string]*client{}

func randCode() string {
	const letters = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // no 0/O/I/1
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

// send to a specific player id if online
func (h *Hub) send(id int64, typ string, v interface{}) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for c := range h.clients {
		if c.id == id {
			sendJSON(c, typ, v)
			return
		}
	}
}
func (h *Hub) buildLeaderboardTop50() protocol.Leaderboard {
	_ = ensureProfilesDir()
	entries := []protocol.LeaderboardEntry{}

	// Walk the profiles dir (flat)
	fs.WalkDir(os.DirFS(profilesDir), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(p), ".json") {
			return nil
		}
		b, err := os.ReadFile(filepath.Join(profilesDir, p))
		if err != nil {
			return nil
		}
		var prof protocol.Profile
		if json.Unmarshal(b, &prof) != nil {
			return nil
		}

		// Safety defaults (older files)
		if prof.PvPRating == 0 {
			prof.PvPRating = 1200
		}
		if prof.PvPRank == "" {
			prof.PvPRank = rankName(prof.PvPRating)
		}

		name := prof.Name
		if name == "" {
			name = strings.TrimSuffix(filepath.Base(p), ".json")
		}

		entries = append(entries, protocol.LeaderboardEntry{
			Name:   name,
			Rating: prof.PvPRating,
			Rank:   prof.PvPRank,
		})
		return nil
	})

	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Rating != entries[j].Rating {
			return entries[i].Rating > entries[j].Rating
		}
		// tie-break by name
		return strings.ToLower(entries[i].Name) < strings.ToLower(entries[j].Name)
	})
	if len(entries) > 50 {
		entries = entries[:50]
	}

	return protocol.Leaderboard{
		Items:       entries,
		GeneratedAt: time.Now().UnixMilli(),
	}
}
