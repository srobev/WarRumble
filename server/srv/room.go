package srv

import (
	"log"
	"math/rand"
	"rumble/shared/protocol"
	"time"
)

type Room struct {
	id       string
	g        *Game
	lastSnap time.Time
	players  []*client
	active   bool   // gameplay ticks only when true
	Mode     string // "queue" | "friendly" | "pve"
	hub      *Hub   // back-reference so we can send and persist at game end
	// ---- PvE bot
	aiActive bool
	aiID     int64
	aiTimer  float64

	tick int
}

func NewRoom(id string, h *Hub) *Room {
	r := &Room{id: id, g: NewGame(), hub: h, Mode: "pve"}
	// Set up event broadcasting callback
	r.g.broadcastEvent = func(eventType string, event interface{}) {
		for _, c := range r.players {
			sendJSON(c, eventType, event)
		}
	}
	return r
}

// ---- Lobby join without starting the battle
// Uses the player's saved profile (ID/Name/Army) and DOES NOT send Init/snapshot yet.
func (r *Room) JoinClient(c *client, s *Session) {
	if c.room != nil {
		return
	}
	c.room = r
	r.players = append(r.players, c)

	// Attach client identity from session
	c.id = s.PlayerID
	c.name = s.Name

	// Add the player into the game with their saved army (fallback inside if invalid)
	r.g.AddPlayerWithArmy(c.id, s.Name, s.Army)
	// Scale player's cards by level (10% per level over base) using UnitXP from session
	if pl := r.g.players[c.id]; pl != nil {
		scaleFor := func(name string) float64 {
			if s == nil {
				return 1
			}
			xp := s.Profile.UnitXP[name]
			lvl, _, _ := computeLevel(xp)
			if lvl < 1 {
				lvl = 1
			}
			return 1.0 + 0.10*float64(lvl-1)
		}
		// Hand
		for i := range pl.Hand {
			f := scaleFor(pl.Hand[i].Name)
			pl.Hand[i].HP = int(float64(pl.Hand[i].HP) * f)
			pl.Hand[i].DMG = int(float64(pl.Hand[i].DMG) * f)
		}
		// Queue
		for i := range pl.Queue {
			f := scaleFor(pl.Queue[i].Name)
			pl.Queue[i].HP = int(float64(pl.Queue[i].HP) * f)
			pl.Queue[i].DMG = int(float64(pl.Queue[i].DMG) * f)
		}
		// Next
		if pl.Next != nil {
			f := scaleFor(pl.Next.Name)
			hp := int(float64(pl.Next.HP) * f)
			dmg := int(float64(pl.Next.DMG) * f)
			pl.Next.HP = hp
			pl.Next.DMG = dmg
		}
		// Scale base HP by average army level (rounded .5 up)
		// Average includes champion + 6 minis from player's saved Army
		if len(s.Army) == 7 {
			sum := 0.0
			for _, nm := range s.Army {
				xp := s.Profile.UnitXP[nm]
				lvl, _, _ := computeLevel(xp)
				if lvl < 1 {
					lvl = 1
				}
				sum += float64(lvl)
			}
			avg := sum / 7.0
			// round .5 up
			round := int(avg + 0.5)
			if round < 1 {
				round = 1
			}
			f := 1.0 + 0.10*float64(round-1)
			if pl.Base.MaxHP > 0 {
				pl.Base.MaxHP = int(float64(pl.Base.MaxHP) * f)
				pl.Base.HP = pl.Base.MaxHP
			}
		}
	}
	// carry over rating/rank into the game player record
	if pl := r.g.players[c.id]; pl != nil {
		pl.Rating = s.Profile.PvPRating
		pl.Rank = s.Profile.PvPRank
		// (Avatar not used in combat right now, but you could carry it here if needed)

	}
}

func (r *Room) Join(c *client) {
	if r.g == nil {
		r.g = NewGame()
	}

	pid := c.id
	if pid == 0 {
		pid = protocol.NewID()
	}

	pl := r.g.AddPlayer(pid, c.name) // now returns *Player
	c.id = pl.ID                     // <-- CRITICAL: bind this websocket to that Player

	c.room = r

	r.players = append(r.players, c)
	log.Printf("JOIN room=%s player=%s id=%d players=%d", r.id, c.name, c.id, len(r.players))
	sendJSON(c, "Profile", protocol.Profile{PlayerID: c.id})
}

// ---- Begin gameplay: send Init + immediate snapshot, then enable ticking
func (r *Room) StartBattle() {
	log.Printf("ROOM %s StartBattle: players=%d", r.id, len(r.players))

	// Load map for friendly duels
	if r.Mode == "friendly" {
		duelMaps := []string{"friendly_duel1", "friendly_duel2"}
		randomIndex := rand.Intn(len(duelMaps))
		selectedMap := duelMaps[randomIndex]
		if mapDef, err := loadMapDef(selectedMap); err == nil {
			r.g.mapDef = &mapDef
			log.Printf("Loaded friendly duel map: %s", selectedMap)
		} else {
			log.Printf("Failed to load friendly duel map %s: %v", selectedMap, err)
		}
	}

	// Initialize timer
	r.g.InitializeTimer()

	// Send Init + initial Gold + immediate snapshot
	for _, p := range r.players {
		sendJSON(p, "Init", r.g.InitFor(p.id))

		if pl := r.g.players[p.id]; pl != nil {
			sendJSON(p, "GoldUpdate", protocol.GoldUpdate{
				PlayerID: pl.ID,
				Gold:     pl.Gold, // send 4 immediately so UI shows it right away
			})
		}

		// Send map definition if available
		if r.g.mapDef != nil {
			sendJSON(p, "MapDef", protocol.MapDefMsg{Def: *r.g.mapDef})
		}

		snap := r.g.FullSnapshot()
		sendJSON(p, "FullSnapshot", snap)
	}

	r.active = true
	r.lastSnap = time.Now()

	// If only 1 human, spawn a simple AI opponent
	ids := make([]int64, 0, len(r.g.players))
	for id := range r.g.players {
		ids = append(ids, id)
	}
	if len(ids) == 1 {
		r.aiActive = true
		r.aiID = protocol.NewID()
		r.g.AddPlayerWithArmy(r.aiID, "AI", r.g.DefaultAIArmy())
	}
}

// Optional (kept for future PvP readiness toggles)
func (r *Room) MarkReady(c *client) {
	r.g.MarkReady(c.id)
}

// Leave room & remove from game
func (r *Room) Leave(leaver *client) {
	// remove from players slice
	newList := r.players[:0]
	for _, p := range r.players {
		if p != leaver {
			newList = append(newList, p)
		}
	}
	r.players = newList

	// remove from authoritative game
	r.g.RemovePlayer(leaver.id)

	// If empty, stop ticking
	if len(r.players) == 0 {
		r.active = false
	}
}

// Deploy intent from a client -> mutate game, then unicast Hand/Gold updates
func (r *Room) HandleDeploy(c *client, d protocol.DeployMiniAt) {
	pl := r.g.players[c.id]
	if pl == nil {
		log.Printf("DEPLOY ignored: no player for client id=%d", c.id)
		return
	}
	if d.CardIndex < 0 || d.CardIndex >= len(pl.Hand) {
		log.Printf("DEPLOY ignored: bad card idx=%d (hand=%d) id=%d", d.CardIndex, len(pl.Hand), c.id)
		return
	}
	if pl.Gold < pl.Hand[d.CardIndex].Cost {
		log.Printf("DEPLOY ignored: not enough gold have=%d need=%d id=%d", pl.Gold, pl.Hand[d.CardIndex].Cost, c.id)
		return
	}
	// gold before
	before := 0
	if pl := r.g.players[c.id]; pl != nil {
		before = pl.Gold
	}

	r.g.HandleDeploy(c.id, d)

	// Unicast updated hand & gold
	if pl := r.g.players[c.id]; pl != nil {
		hu := protocol.HandUpdate{Hand: make([]protocol.MiniCardView, len(pl.Hand))}
		for i, card := range pl.Hand {
			hu.Hand[i] = protocol.MiniCardView{
				Name: card.Name, Portrait: card.Portrait, Cost: card.Cost, Class: card.Class,
			}
		}
		if pl.Next != nil {
			hu.Next = protocol.MiniCardView{
				Name: pl.Next.Name, Portrait: pl.Next.Portrait, Cost: pl.Next.Cost, Class: pl.Next.Class,
			}
		}
		sendJSON(c, "HandUpdate", hu)

		if pl.Gold != before {
			sendJSON(c, "GoldUpdate", protocol.GoldUpdate{PlayerID: pl.ID, Gold: pl.Gold})
		}
	}
}

// Tick the room ONLY when active (after StartBattle)
func (r *Room) Tick() {
	// Do nothing when room is inactive (e.g., after GameOver)
	if !r.active {
		return
	}

	const tickRate = 20.0
	dt := 1.0 / tickRate

	// Update timer and check for expiration
	if timerExpired, timerWinnerID := r.g.UpdateTimer(dt); timerExpired {
		// Timer expired - end game based on timer winner
		if r.Mode == "pve" && timerWinnerID != -1 {
			r.awardPveXPServer(timerWinnerID)
		}

		// Send victory/defeat events before GameOver
		r.sendVictoryDefeatEvents(timerWinnerID)

		for _, c := range r.players {
			sendJSON(c, "GameOver", protocol.GameOver{WinnerID: timerWinnerID})
			// Rating only for Open Queue (two humans)
			if r.Mode == "queue" && r.hub != nil {
				applyQueueRating(r, timerWinnerID, r.hub)
			}
		}
		r.active = false
		return
	}

	// detect game over by base destruction
	var loser *Player
	for _, p := range r.g.players {
		if p.Base.HP <= 0 {
			loser = p
			break
		}
	}
	if loser != nil {
		// winner = the other one (if single-player with AI, that’ll be the bot)
		var winnerID int64
		for id := range r.g.players {
			if id != loser.ID {
				winnerID = id
				break
			}
		}
		// Server-authoritative XP for PvE
		if r.Mode == "pve" {
			r.awardPveXPServer(winnerID)
		}

		// Send victory/defeat events before GameOver
		r.sendVictoryDefeatEvents(winnerID)

		for _, c := range r.players {
			sendJSON(c, "GameOver", protocol.GameOver{WinnerID: winnerID})
			// Rating only for Open Queue (two humans)
			if r.Mode == "queue" && r.hub != nil {
				applyQueueRating(r, winnerID, r.hub)
			}
		}
		r.active = false
		return
	}

	r.tick++

	// --- Simple AI: slower & capped
	if r.aiActive {
		r.aiTimer += dt
		if r.aiTimer >= 3.5 { // spawn every ~3.5s
			r.aiTimer = 0

			// cap AI units at 5
			aiCount := 0
			for _, u := range r.g.units {
				if u.OwnerID == r.aiID {
					aiCount++
				}
			}
			if aiCount < 5 {
				if pl := r.g.players[r.aiID]; pl != nil {
					idx := -1
					for i, c := range pl.Hand {
						if c.Cost <= pl.Gold {
							idx = i
							break
						}
					}
					if idx >= 0 {
						x := float64(r.g.width/2 + (rand.Intn(120) - 60))
						y := float64(90 + rand.Intn(40))
						r.g.HandleDeploy(r.aiID, protocol.DeployMiniAt{CardIndex: idx, X: x, Y: y})
					}
				}
			}
		}
	}

	// --- Sim step
	delta := r.g.Step(dt)

	// --- Broadcast delta (includes bases every tick from g.Step)
	for _, c := range r.players {
		sendJSON(c, "StateDelta", delta)
		// also send each player's gold
		if p := r.g.players[c.id]; p != nil {
			sendJSON(c, "GoldUpdate", protocol.GoldUpdate{PlayerID: p.ID, Gold: p.Gold})
		}
	}

	// Optional: resync occasionally
	if r.tick%60 == 0 { // every ~3s
		snap := r.g.FullSnapshot()
		for _, c := range r.players {
			sendJSON(c, "FullSnapshot", snap)
		}
	}
}

// awardPveXPServer updates each human player's profile with XP after PvE battle.
func (r *Room) awardPveXPServer(winnerID int64) {
	if r.hub == nil {
		return
	}
	for _, c := range r.players {
		// Skip bot if present
		if r.aiActive && c.id == r.aiID {
			continue
		}
		r.hub.mu.Lock()
		s := r.hub.sessions[c]
		if s == nil {
			r.hub.mu.Unlock()
			continue
		}
		if s.Profile.UnitXP == nil {
			s.Profile.UnitXP = map[string]int{}
		}
		rate := 0.02
		if c.id == winnerID {
			rate = 0.05
		}
		// Use saved active army and award to champion + random minis
		army := s.Profile.Army
		if len(army) == 0 {
			// nothing to award
			_ = saveProfile(s.Profile)
			prof := s.Profile
			r.hub.mu.Unlock()
			sendJSON(c, "Profile", prof)
			continue
		}

		champ := army[0]
		minis := make([]string, 0, 6)
		for i := 1; i < 7 && i < len(army); i++ {
			if army[i] != "" {
				minis = append(minis, army[i])
			}
		}

		// Determine how many minis to award alongside champion
		// Keep it light: 1 mini on loss, 2 minis on win (or fewer if not enough minis)
		k := 1
		if c.id == winnerID {
			k = 2
		}
		if k > len(minis) {
			k = len(minis)
		}

		// Shuffle minis so selection is random
		if len(minis) > 1 {
			rand.Shuffle(len(minis), func(i, j int) { minis[i], minis[j] = minis[j], minis[i] })
		}

		// Targets: champion (if present) + first k shuffled minis
		targets := make([]string, 0, 1+k)
		if champ != "" {
			targets = append(targets, champ)
		}
		targets = append(targets, minis[:k]...)

		for _, name := range targets {
			cur := s.Profile.UnitXP[name]
			delta := xpDeltaForRate(cur, rate)
			if delta > 0 {
				s.Profile.UnitXP[name] = cur + delta
			}
		}
		_ = saveProfile(s.Profile)
		prof := s.Profile
		r.hub.mu.Unlock()
		sendJSON(c, "Profile", prof)
	}
}

// sendVictoryDefeatEvents sends victory/defeat events to all players based on the winner
func (r *Room) sendVictoryDefeatEvents(winnerID int64) {
	// Calculate match duration
	duration := 0
	if r.g != nil && r.g.timeLimit > 0 {
		duration = r.g.timeLimit - int(r.g.timeRemaining)
		if duration < 0 {
			duration = 0
		}
	}

	// Find winner and loser info
	var winnerName, loserName string
	var loserID int64
	var goldEarned, xpGained int

	for _, p := range r.g.players {
		if p.ID == winnerID {
			winnerName = p.Name
			// Calculate rewards for PvE
			if r.Mode == "pve" {
				goldEarned = 10 // Base gold reward
				xpGained = 50   // Base XP reward
			}
		} else {
			loserID = p.ID
			loserName = p.Name
		}
	}

	// Send victory event to winner
	victoryEvent := protocol.VictoryEvent{
		WinnerID:   winnerID,
		WinnerName: winnerName,
		MatchType:  r.Mode,
		Duration:   duration,
		GoldEarned: goldEarned,
		XPGained:   xpGained,
	}

	// Send defeat event to loser
	defeatEvent := protocol.DefeatEvent{
		LoserID:    loserID,
		LoserName:  loserName,
		WinnerID:   winnerID,
		WinnerName: winnerName,
		MatchType:  r.Mode,
		Duration:   duration,
	}

	// Broadcast events to all players
	for _, c := range r.players {
		if c.id == winnerID {
			sendJSON(c, "VictoryEvent", victoryEvent)
		} else {
			sendJSON(c, "DefeatEvent", defeatEvent)
		}
	}
}
