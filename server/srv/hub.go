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
	"rumble/shared/protocol"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

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
}

func NewHub() *Hub {
	return &Hub{
		clients:  make(map[*client]struct{}),
		rooms:    make(map[string]*Room),
		sessions: make(map[*client]*Session),

		// NEW:
		pvpQueue:       make([]*client, 0, 64),
		friendly:       make(map[string]*client),
		friendByClient: make(map[*client]string),
	}
}

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
		delete(h.sessions, c)
		h.mu.Unlock()
	}()

	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			return
		}

		var env protocol.MsgEnvelope
		if err := json.Unmarshal(data, &env); err != nil {
			continue
		}
		log.Printf("WS msg type=%s", env.Type)

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
			prof, err := loadProfile(s.Name)
			if err != nil {
				log.Printf("loadProfile: %v", err)
			}
			// bind server-issued ID + name
			prof.PlayerID = s.PlayerID
			prof.Name = s.Name
			s.Profile = prof
			if s.Profile.Avatar == "" {
				s.Profile.Avatar = "default.png"
			}
			if s.Profile.PvPRating == 0 {
				s.Profile.PvPRating = 1200
			}
			if s.Profile.PvPRank == "" {
				s.Profile.PvPRank = rankName(s.Profile.PvPRating)
			}
			// keep legacy field in sync
			s.Army = append([]string{}, prof.Army...)
			h.mu.Unlock()

			sendJSON(c, "Profile", s.Profile)
		case "SetAvatar":
			var m protocol.SetAvatar
			_ = json.Unmarshal(env.Data, &m)

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

			h.mu.Lock()
			if s := h.sessions[c]; s != nil {
				s.Profile.Avatar = a
				_ = saveProfile(s.Profile)
				// send updated profile back so client refreshes UI
				prof := s.Profile
				h.mu.Unlock()
				sendJSON(c, "Profile", prof)
			} else {
				h.mu.Unlock()
			}

		case "GetProfile":
			h.mu.Lock()
			s := h.sessions[c]
			var prof protocol.Profile
			if s != nil {
				prof = s.Profile
			}
			h.mu.Unlock()
			sendJSON(c, "Profile", prof)

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
			// persist
			if err := saveProfile(s.Profile); err != nil {
				log.Printf("saveProfile: %v", err)
			}
			prof := s.Profile
			h.mu.Unlock()

			sendJSON(c, "Profile", prof)

		case "ListMinis":
			minis := LoadLobbyMinis()
			sendJSON(c, "Minis", protocol.Minis{Items: minis})

		case "ListMaps":
			maps := []protocol.MapInfo{
				{ID: "arena1", Name: "Arena I"},
				{ID: "arena2", Name: "Arena II"},
				{ID: "arena3", Name: "Arena III"},
			}
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

		// ---------- Gameplay ----------
		case "DeployMiniAt":
			var m protocol.DeployMiniAt
			_ = json.Unmarshal(env.Data, &m)
			if c.room != nil {
				c.room.HandleDeploy(c, m)
			}

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
			h.DequeuePvp(c)
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
