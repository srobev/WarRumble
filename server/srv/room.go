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
	return &Room{id: id, g: NewGame(), hub: h, Mode: "pve"}
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

	// Send Init + initial Gold + immediate snapshot
	for _, p := range r.players {
		sendJSON(p, "Init", r.g.InitFor(p.id))

		if pl := r.g.players[p.id]; pl != nil {
			sendJSON(p, "GoldUpdate", protocol.GoldUpdate{
				PlayerID: pl.ID,
				Gold:     pl.Gold, // send 4 immediately so UI shows it right away
			})
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
	// detect game over
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
		for _, c := range r.players {
			sendJSON(c, "GameOver", protocol.GameOver{WinnerID: winnerID})
			// Rating only for Open Queue (two humans)
			if r.Mode == "queue" && r.hub != nil {
				applyQueueRating(r, winnerID, r.hub)
			}
		}
		r.active = false
	}
	if !r.active {
		return
	}
	const tickRate = 20.0
	dt := 1.0 / tickRate
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
