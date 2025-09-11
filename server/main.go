package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"rumble/server/auth"
	"rumble/server/shop"
	"rumble/server/srv"
	"rumble/shared/protocol"

	"github.com/gorilla/websocket"
)

type Player struct {
	ID      int64
	Name    string
	Profile protocol.Profile
	// ...
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  2048,
	WriteBufferSize: 2048,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func wsHandler(h *srv.Hub, authz *auth.Auth) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// --- AUTH for WebSocket (header or ?token=) ---
		var tok string
		if ah := r.Header.Get("Authorization"); strings.HasPrefix(ah, "Bearer ") {
			tok = strings.TrimPrefix(ah, "Bearer ")
		} else {
			tok = r.URL.Query().Get("token")
		}
		user, errTok := authz.ParseToken(tok)
		if errTok != nil || user == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("upgrade:", err)
			return
		}
		h.HandleWSAuth(conn, user)
	}
}

func main() {
	// Seed RNG once at startup for any randomization (AI, XP targets, etc.)
	rand.Seed(time.Now().UnixNano())
	hub := srv.NewHub()
	go hub.Run()

	authz, err := auth.NewAuth("./data")
	if err != nil {
		panic(err)
	}
	guilds, err := srv.NewGuilds("./data")
	if err != nil {
		panic(err)
	}
	hub.SetGuilds(guilds)
	social, err := srv.NewSocial("./data")
	if err != nil {
		panic(err)
	}
	hub.SetSocial(social)
	shopService := shop.NewService()
	hub.SetShopService(shopService)

	// Load static perks data
	if err := LoadPerks("./data"); err != nil {
		log.Printf("Failed to load perks: %v", err)
	} else {
		shopService.PerkCatalogFunc = GetPerksForUnit // set the func
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/ws", wsHandler(hub, authz))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("ok")) })
	mux.HandleFunc("/api/register", authz.HandleRegister)
	mux.HandleFunc("/api/login", authz.HandleLogin)
	// /api/profile — read current user's profile JSON from disk
	mux.Handle("/api/profile", authz.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract username again (RequireAuth validated it already)
		var tok string
		if ah := r.Header.Get("Authorization"); strings.HasPrefix(ah, "Bearer ") {
			tok = strings.TrimPrefix(ah, "Bearer ")
		} else {
			tok = r.URL.Query().Get("token")
		}
		username, err := authz.ParseToken(tok)
		if err != nil || username == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Build path using existing helpers from persist.go
		path := profilePath(username)
		_ = os.MkdirAll(filepath.Dir(path), 0o755)

		// Defaults for new users
		prof := protocol.Profile{
			Name:      username,
			Armies:    map[string][]string{},
			UnitXP:    map[string]int{},
			Resources: map[string]int{},
		}
		if b, err := os.ReadFile(path); err == nil && len(b) > 0 {
			_ = json.Unmarshal(b, &prof)
		}
		if prof.Avatar == "" {
			prof.Avatar = "default.png"
		}
		if prof.PvPRating == 0 {
			prof.PvPRating = 1200
		}
		if prof.PvPRank == "" {
			prof.PvPRank = "Knight"
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(prof)
	})))

	srvAddr := ":8080"
	s := &http.Server{
		Addr:         srvAddr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	log.Printf("WarRumble Server v%s starting...", protocol.GameVersion)
	log.Println("server listening on", srvAddr)
	log.Fatal(s.ListenAndServe())
}

// ------- minimal local helpers to read profiles (mirrors srv/hub.go logic) -------
func safeFileName(name string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9]+`)
	s := re.ReplaceAllString(name, "_")
	if s == "" {
		s = "player"
	}
	return s
}
func profilePath(name string) string {
	return filepath.Join("data", "profiles", safeFileName(name)+".json")
}
func readProfile(name string) protocol.Profile {
	_ = os.MkdirAll(filepath.Join("data", "profiles"), 0o755)
	p := protocol.Profile{
		Name:      name,
		Army:      nil,
		Armies:    map[string][]string{},
		Gold:      0,
		AccountXP: 0,
		UnitXP:    map[string]int{},
		Resources: map[string]int{},
	}
	b, err := os.ReadFile(profilePath(name))
	if err != nil {
		// defaults for new users
		if p.PvPRating == 0 {
			p.PvPRating = 1200
		}
		if p.PvPRank == "" {
			p.PvPRank = "Knight"
		}
		if p.Avatar == "" {
			p.Avatar = "default.png"
		}
		return p
	}
	_ = json.Unmarshal(b, &p)
	if p.PvPRating == 0 {
		p.PvPRating = 1200
	}
	if p.PvPRank == "" {
		p.PvPRank = "Knight"
	}
	if p.Avatar == "" {
		p.Avatar = "default.png"
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
	return p
}
