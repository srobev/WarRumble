package srv

import (
	"encoding/json"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"rumble/shared/protocol"
)

type MiniCard struct {
	Name        string  `json:"name"`
	DMG         int     `json:"dmg"`
	HP          int     `json:"hp"`
	Heal        int     `json:"heal"`
	Hps         int     `json:"hps"`
	Portrait    string  `json:"portrait"`
	Class       string  `json:"class"`
	SubClass    string  `json:"subclass"`
	Role        string  `json:"role"`
	Cost        int     `json:"cost"`
	Speed       float64 `json:"speed"`
	Range       int     `json:"range"`
	Particle    string  `json:"particle"`
	Cooldown    float64 `json:"cooldown,omitempty"`     // Attack cooldown in seconds
	AttackSpeed float64 `json:"attack_speed,omitempty"` // Attacks per second (alternative to cooldown)
}

type Player struct {
	ID     int64
	Name   string
	Gold   int
	GoldT  float64
	Hand   []MiniCard
	Queue  []MiniCard
	Next   *MiniCard
	Ready  bool
	Base   Base
	Rating int    // NEW: PvP Elo
	Rank   string // NEW: derived name
}

type Base struct {
	OwnerID    int64
	HP, MaxHP  int
	X, Y, W, H int
}

type Unit struct {
	ID             int64
	Name           string
	X, Y           float64
	VX, VY         float64
	HP, MaxHP      int
	DMG            int
	Heal           int
	Hps            int
	Speed          float64 // px/s
	BaseSpeed      float64 // Original speed before debuffs
	OwnerID        int64
	Class          string
	SubClass       string
	Range          int
	Particle       string
	Facing, CD     float64
	HealCD         float64
	AttackCooldown float64           // Configurable attack cooldown duration
	SpeedDebuffs   map[int64]float64 // Active speed debuffs (blizzardID -> speedMultiplier)
}

type Projectile struct {
	ID             int64
	X, Y           float64 // Current position
	TX, TY         float64 // Target position
	VX, VY         float64 // Velocity
	Speed          float64 // Movement speed
	Damage         int     // Damage to deal on impact
	OwnerID        int64   // Who fired this projectile
	TargetID       int64   // Target unit ID (0 if targeting base)
	TargetX        float64 // Target X coordinate (for base targeting)
	TargetY        float64 // Target Y coordinate (for base targeting)
	Active         bool    // Whether projectile is still active
	ProjectileType string  // Type for visual effects
}

type BlizzardEffect struct {
	ID           int64
	CasterID     int64
	X            float64
	Y            float64
	Radius       float64
	Damage       int
	Duration     float64
	TickInterval float64
	LastTick     float64
	StartTime    float64
	TotalTime    float64 // Track total elapsed time for duration
}

type Game struct {
	minis       []MiniCard
	units       map[int64]*Unit
	projectiles map[int64]*Projectile
	players     map[int64]*Player
	width       int
	height      int
	mapDef      *protocol.MapDef // Current map definition

	// Blizzard effects
	blizzards map[int64]*BlizzardEffect

	// Timer system
	timerActive   bool
	timeRemaining float64 // in seconds
	timeLimit     int     // configured time limit in seconds
	isPaused      bool
	matchEnded    bool
	timerWinnerID int64 // winner when timer expires

	// Event broadcasting callback
	broadcastEvent func(eventType string, event interface{})
}

func NewGame() *Game {
	g := &Game{
		units:       make(map[int64]*Unit),
		projectiles: make(map[int64]*Projectile),
		players:     make(map[int64]*Player),
		width:       protocol.ScreenW,
		height:      protocol.ScreenH,
		blizzards:   make(map[int64]*BlizzardEffect),
		// init maps, players, etc.
	}
	g.loadMinis()
	return g
}

func (g *Game) loadMinis() {
	// Try sensible paths relative to the running binary and CWD.
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	candidates := []string{
		filepath.Join(exeDir, "data", "minis.json"), // server/data/minis.json next to server binary
		filepath.Join("data", "minis.json"),         // when running `go run .` inside server/
		filepath.Join(exeDir, "..", "internal", "game", "assets", "minis.json"),
		filepath.Join("..", "internal", "game", "assets", "minis.json"),
	}

	for _, p := range candidates {
		if b, err := os.ReadFile(p); err == nil {
			if err := json.Unmarshal(b, &g.minis); err == nil {
				log.Printf("loaded %d minis from %s", len(g.minis), p)
				return
			}
			log.Printf("failed to parse minis from %s: %v", p, err)
		}
	}
	log.Printf("WARNING: no minis.json found — using empty set (fallback cards will be used)")
}

func toBaseState(b *Base) protocol.BaseState {
	return protocol.BaseState{
		OwnerID: b.OwnerID,
		HP:      b.HP,
		MaxHP:   b.MaxHP,
		X:       b.X, Y: b.Y, W: b.W, H: b.H,
	}
}

// AddPlayerWithArmy allows passing a preselected 7-card army (names).
func (g *Game) AddPlayerWithArmy(id int64, name string, armyNames []string) *Player {
	p := &Player{ID: id, Name: name, Gold: 4}

	baseW, baseH := 96, 96
	bottomMargin := 180 // leave space for 160px HUD + padding
	topMargin := 28

	// Use map-defined base positions if available
	if g.mapDef != nil {
		var baseX, baseY int
		if len(g.players) == 0 {
			// First player (human) - use playerBase
			if g.mapDef.PlayerBase.X >= 0 && g.mapDef.PlayerBase.Y >= 0 {
				baseX = int(g.mapDef.PlayerBase.X * float64(g.width))
				baseY = int(g.mapDef.PlayerBase.Y * float64(g.height))
			} else {
				// Fallback to bottom center
				baseX = g.width/2 - baseW/2
				baseY = g.height - baseH - bottomMargin
			}
		} else {
			// Second player (AI or enemy) - use enemyBase
			if g.mapDef.EnemyBase.X >= 0 && g.mapDef.EnemyBase.Y >= 0 {
				baseX = int(g.mapDef.EnemyBase.X * float64(g.width))
				baseY = int(g.mapDef.EnemyBase.Y * float64(g.height))
			} else {
				// Fallback to top center
				baseX = g.width/2 - baseW/2
				baseY = topMargin
			}
		}

		p.Base = Base{
			OwnerID: id,
			HP:      3000, MaxHP: 3000,
			W: baseW, H: baseH,
			X: baseX,
			Y: baseY,
		}
	} else {
		// Fallback to hardcoded positions when no map definition
		p.Base = Base{
			OwnerID: id,
			HP:      3000, MaxHP: 3000,
			W: baseW, H: baseH,
			X: g.width/2 - baseW/2,
			Y: g.height - baseH - bottomMargin, // Player base at bottom
		}
		if len(g.players) == 1 {
			p.Base.Y = topMargin // Enemy base at top
		}
	}

	g.players[id] = p

	if ok := g.tryBuildArmyByNames(p, armyNames); !ok {
		g.dealArmy(p)
	}
	return p
}

func (g *Game) AddPlayer(id int64, name string) *Player {
	return g.AddPlayerWithArmy(id, name, nil)
}

func (g *Game) tryBuildArmyByNames(p *Player, names []string) bool {
	if len(names) != 7 {
		return false
	}
	// index minis by name
	idx := map[string]MiniCard{}
	for _, m := range g.minis {
		idx[strings.ToLower(m.Name)] = m
	}
	// collect 7 cards
	cards := make([]MiniCard, 0, 7)
	for _, n := range names {
		m, ok := idx[strings.ToLower(n)]
		if !ok {
			return false
		}
		cards = append(cards, m)
	}
	// simple validation: 1 champion + 6 minis (spells allowed)
	champ := 0
	minis := 0
	for _, c := range cards {
		r := strings.ToLower(c.Role)
		cl := strings.ToLower(c.Class)
		if r == "champion" || cl == "champion" {
			champ++
		} else if r == "mini" {
			minis++
		}
	}
	if champ != 1 || minis != 6 {
		return false
	}

	p.Hand = append([]MiniCard{}, cards[:4]...)
	p.Queue = append([]MiniCard{}, cards[4:]...)
	nx := p.Queue[0]
	p.Next = &nx
	return true
}

func (g *Game) RemovePlayer(id int64) { delete(g.players, id) }
func (g *Game) MarkReady(id int64) {
	if p, ok := g.players[id]; ok {
		p.Ready = true
	}
}
func lower(s string) string { return strings.ToLower(s) }
func speedPx(tier float64) float64 {
	if tier <= 0 {
		return 60
	} // was 90
	return 48 + 24*(tier-1) // was 75 + 30*(tier-1)
}

func max1(a, b int) int {
	if a > b {
		return a
	}
	return b
}
func hypot(x1, y1, x2, y2 float64) float64 { dx, dy := x2-x1, y2-y1; return math.Hypot(dx, dy) }

func (g *Game) dealArmy(p *Player) {
	var champs, minis []MiniCard
	for _, m := range g.minis {
		r := strings.ToLower(m.Role)
		c := strings.ToLower(m.Class)
		switch {
		case r == "champion" || c == "champion":
			champs = append(champs, m)
		case r == "mini":
			minis = append(minis, m)
		}
	}

	// Fallbacks if minis.json is missing or too small
	if len(champs) == 0 && len(minis) > 0 {
		champs = append(champs, minis[rand.Intn(len(minis))])
	}
	if len(champs) == 0 {
		champs = append(champs, MiniCard{
			Name: "Dummy Champion", DMG: 50, HP: 800, Cost: 3, Speed: 2, Class: "melee", Role: "champion",
		})
	}
	// Ensure at least 6 minis (duplicate if needed; if none at all, seed with dummy)
	if len(minis) == 0 {
		minis = append(minis, MiniCard{
			Name: "Footman", DMG: 60, HP: 300, Cost: 2, Speed: 2, Class: "melee", Role: "mini",
		})
	}
	for len(minis) < 6 {
		minis = append(minis, minis[rand.Intn(len(minis))])
	}

	// Build the 7-card army: 1 champion + 6 minis
	rand.Shuffle(len(minis), func(i, j int) { minis[i], minis[j] = minis[j], minis[i] })
	army := make([]MiniCard, 0, 7)
	army = append(army, champs[rand.Intn(len(champs))])
	army = append(army, minis[:6]...)

	// Hand (4) + Queue (3) + Next
	p.Hand = append([]MiniCard{}, army[:4]...)
	p.Queue = append([]MiniCard{}, army[4:]...)
	if len(p.Queue) > 0 {
		nx := p.Queue[0]
		p.Next = &nx
	} else {
		p.Next = nil
	}
}

func (g *Game) InitFor(pid int64) protocol.Init {
	p := g.players[pid]
	hand := make([]protocol.MiniCardView, len(p.Hand))
	for i, c := range p.Hand {
		hand[i] = protocol.MiniCardView{Name: c.Name, Portrait: c.Portrait, Cost: c.Cost, Class: c.Class}
	}
	nx := protocol.MiniCardView{}
	if p.Next != nil {
		nx = protocol.MiniCardView{Name: p.Next.Name, Portrait: p.Next.Portrait, Cost: p.Next.Cost, Class: p.Next.Class}
	}
	return protocol.Init{PlayerID: pid, MapWidth: g.width, MapHeight: g.height, Hand: hand, Next: nx}
}

// InitializeTimer sets up the match timer based on map configuration
func (g *Game) InitializeTimer() {
	if g.mapDef != nil && g.mapDef.TimeLimit > 0 {
		g.timeLimit = g.mapDef.TimeLimit
	} else {
		g.timeLimit = 180 // default 3:00 minutes
	}
	g.timeRemaining = float64(g.timeLimit)
	g.timerActive = true
	g.isPaused = false
	g.matchEnded = false
}

// UpdateTimer updates the timer and checks for expiration
func (g *Game) UpdateTimer(dt float64) (timerExpired bool, winnerID int64) {
	if !g.timerActive || g.isPaused || g.matchEnded {
		return false, 0
	}

	g.timeRemaining -= dt
	if g.timeRemaining <= 0 {
		g.timeRemaining = 0
		g.timerActive = false
		g.matchEnded = true

		// Determine winner based on base health
		var player1, player2 *Player
		for _, p := range g.players {
			if player1 == nil {
				player1 = p
			} else {
				player2 = p
			}
		}

		if player1 != nil && player2 != nil {
			if player1.Base.HP > player2.Base.HP {
				return true, player1.ID
			} else if player2.Base.HP > player1.Base.HP {
				return true, player2.ID
			} else {
				// Draw - both lose
				return true, -1
			}
		}
	}
	return false, 0
}

// PauseTimer pauses the match timer
func (g *Game) PauseTimer() {
	if g.timerActive {
		g.isPaused = true
	}
}

// ResumeTimer resumes the match timer
func (g *Game) ResumeTimer() {
	if g.timerActive {
		g.isPaused = false
	}
}

// RestartMatch resets the entire match
func (g *Game) RestartMatch() {
	// Reset timer
	g.InitializeTimer()

	// Reset bases
	for _, p := range g.players {
		p.Base.HP = p.Base.MaxHP
	}

	// Clear all units and projectiles
	g.units = make(map[int64]*Unit)
	g.projectiles = make(map[int64]*Projectile)

	// Reset players' state
	for _, p := range g.players {
		p.Gold = 4
		p.GoldT = 0
		p.Ready = false
		// Re-deal army
		if ok := g.tryBuildArmyByNames(p, nil); !ok {
			g.dealArmy(p)
		}
	}

	g.matchEnded = false
}

// SurrenderMatch ends the match as a loss for the surrendering player
func (g *Game) SurrenderMatch(playerID int64) int64 {
	g.matchEnded = true
	g.timerActive = false

	// Find the winner (the other player)
	for _, p := range g.players {
		if p.ID != playerID {
			return p.ID
		}
	}
	return 0 // Should not happen
}

// GetTimerState returns current timer information
func (g *Game) GetTimerState() (remainingSeconds int, isPaused bool) {
	return int(math.Ceil(g.timeRemaining)), g.isPaused
}

// recalculateUnitSpeed updates a unit's effective speed based on active debuffs
func (g *Game) recalculateUnitSpeed(unit *Unit) {
	// Start with base speed
	unit.Speed = unit.BaseSpeed

	// Apply all active speed debuffs (multiply by debuff multipliers)
	for _, multiplier := range unit.SpeedDebuffs {
		unit.Speed *= multiplier
	}
}

func (g *Game) tickBlizzard(blizzardID int64, dt float64) {
	blizzard, exists := g.blizzards[blizzardID]
	if !exists {
		return
	}

	// Set start time on first tick
	if blizzard.StartTime == 0 {
		blizzard.StartTime = blizzard.LastTick
	}

	// Update total time elapsed
	blizzard.TotalTime += dt

	// Check if blizzard has expired using TotalTime
	if blizzard.TotalTime >= blizzard.Duration {
		// Remove speed debuffs from all units affected by this blizzard
		for _, unit := range g.units {
			if _, hasDebuff := unit.SpeedDebuffs[blizzardID]; hasDebuff {
				delete(unit.SpeedDebuffs, blizzardID)
				g.recalculateUnitSpeed(unit)
			}
		}
		delete(g.blizzards, blizzardID)
		return
	}

	// Apply/Remove speed debuffs based on unit positions
	for _, unit := range g.units {
		if unit.HP <= 0 {
			continue
		}

		dist := hypot(blizzard.X, blizzard.Y, unit.X, unit.Y)
		_, hasDebuff := unit.SpeedDebuffs[blizzardID]

		if dist <= blizzard.Radius && unit.OwnerID != blizzard.CasterID {
			// Unit is in blizzard area and is an enemy - apply debuff
			if !hasDebuff {
				unit.SpeedDebuffs[blizzardID] = 0.5 // 50% speed reduction
				g.recalculateUnitSpeed(unit)
			}
		} else {
			// Unit is outside blizzard area or is friendly - remove debuff
			if hasDebuff {
				delete(unit.SpeedDebuffs, blizzardID)
				g.recalculateUnitSpeed(unit)
			}
		}
	}

	// Check if it's time for a damage tick using LastTick for intervals
	if blizzard.LastTick-blizzard.StartTime >= blizzard.TickInterval {
		// Deal damage to all enemy units in radius
		for _, unit := range g.units {
			if unit.OwnerID == blizzard.CasterID || unit.HP <= 0 {
				continue
			}
			dist := hypot(blizzard.X, blizzard.Y, unit.X, unit.Y)
			if dist <= blizzard.Radius {
				unit.HP -= blizzard.Damage
				if unit.HP < 0 {
					unit.HP = 0
				}
			}
		}

		// Deal damage to all enemy bases in radius
		for _, player := range g.players {
			if player.ID == blizzard.CasterID {
				continue
			}
			// Calculate distance from blizzard center to base center
			baseCenterX := float64(player.Base.X + player.Base.W/2)
			baseCenterY := float64(player.Base.Y + player.Base.H/2)
			dist := hypot(blizzard.X, blizzard.Y, baseCenterX, baseCenterY)
			if dist <= blizzard.Radius {
				player.Base.HP -= blizzard.Damage
				if player.Base.HP < 0 {
					player.Base.HP = 0
				}
			}
		}

		// Reset tick timer for next damage interval
		blizzard.LastTick = blizzard.StartTime
	}

	// Update last tick time for damage intervals
	blizzard.LastTick += dt
}

func (g *Game) Step(dt float64) protocol.StateDelta {
	// If game is paused, don't update anything
	if g.isPaused {
		// Return empty delta to indicate no changes
		return protocol.StateDelta{
			UnitsUpsert:  []protocol.UnitState{},
			UnitsRemoved: []int64{},
			Projectiles:  []protocol.ProjectileState{},
			Bases:        []protocol.BaseState{},
		}
	}

	// Update blizzards
	for blizzardID := range g.blizzards {
		g.tickBlizzard(blizzardID, dt)
	}

	// Update projectiles
	g.updateProjectiles(dt)

	// gold
	for _, p := range g.players {
		p.GoldT += dt
		for p.GoldT >= protocol.GoldTickSec && p.Gold < protocol.GoldMax {
			p.Gold++
			p.GoldT -= protocol.GoldTickSec
		}
		if p.Gold > protocol.GoldMax {
			p.Gold = protocol.GoldMax
		}
	}

	removed := make([]int64, 0, 8)
	upserts := make([]protocol.UnitState, 0, len(g.units))

	for id, u := range g.units {
		if u.HP <= 0 {
			// Broadcast unit death event before removing
			if g.broadcastEvent != nil {
				deathEvent := protocol.UnitDeathEvent{
					UnitID:       u.ID,
					UnitX:        u.X,
					UnitY:        u.Y,
					UnitName:     u.Name,
					UnitClass:    u.Class,
					UnitSubclass: u.SubClass,
					KillerID:     0, // TODO: Track which unit dealt the killing blow
				}
				g.broadcastEvent("UnitDeathEvent", deathEvent)
			}

			removed = append(removed, id)
			delete(g.units, id)
			continue
		}

		// target: nearest enemy unit, else enemy base
		tx, ty := g.findTarget(u)
		dx, dy := tx-u.X, ty-u.Y
		dist := math.Hypot(dx, dy)

		rng := 28.0 // melee
		if lower(u.Class) == "range" {
			rng = float64(u.Range)
		}

		// Attack logic for all ranged units (including healers)
		if dist <= rng {
			if u.CD <= 0 {
				// Healers attack if they have damage, otherwise they just stay at range
				if lower(u.SubClass) != "healer" || u.DMG > 0 {
					g.damageAt(u, tx, ty, u.DMG)
				}
				u.CD = u.AttackCooldown
			}
		} else if dist > 0 {
			nx, ny := dx/dist, dy/dist
			u.Facing = math.Atan2(ny, nx)
			u.X += nx * u.Speed * dt
			u.Y += ny * u.Speed * dt
		}
		if u.CD > 0 {
			u.CD -= dt
		}

		// Healing for healers
		if lower(u.Class) == "range" && lower(u.SubClass) == "healer" {
			if u.HealCD <= 0 {
				for _, v := range g.units {
					if v.OwnerID == u.OwnerID && v.HP < v.MaxHP && hypot(u.X, u.Y, v.X, v.Y) <= float64(u.Range) {
						// Send healing event to all clients
						healingEvent := protocol.HealingEvent{
							HealerID:   u.ID,
							HealerX:    u.X,
							HealerY:    u.Y,
							TargetID:   v.ID,
							TargetX:    v.X,
							TargetY:    v.Y,
							HealAmount: u.Heal,
							HealerName: u.Name,
							TargetName: v.Name,
						}

						// Broadcast healing event to all connected clients
						if g.broadcastEvent != nil {
							g.broadcastEvent("HealingEvent", healingEvent)
						}

						v.HP += u.Heal
						if v.HP > v.MaxHP {
							v.HP = v.MaxHP
						}
						u.HealCD = 4.0
						break
					}
				}
			} else {
				u.HealCD -= dt
			}
		}

		upserts = append(upserts, protocol.UnitState{
			ID: u.ID, Name: u.Name, X: u.X, Y: u.Y, HP: u.HP, MaxHP: u.MaxHP,
			OwnerID: u.OwnerID, Facing: u.Facing, Class: u.Class, Range: u.Range, Particle: u.Particle,
		})
	}

	// Build projectile states for clients
	projectiles := make([]protocol.ProjectileState, 0, len(g.projectiles))
	for _, proj := range g.projectiles {
		projectiles = append(projectiles, protocol.ProjectileState{
			ID:             proj.ID,
			X:              proj.X,
			Y:              proj.Y,
			TX:             proj.TX,
			TY:             proj.TY,
			Damage:         proj.Damage,
			OwnerID:        proj.OwnerID,
			TargetID:       proj.TargetID,
			ProjectileType: proj.ProjectileType,
			Active:         proj.Active,
		})
	}

	bases := make([]protocol.BaseState, 0, len(g.players))
	for _, p := range g.players {
		bases = append(bases, protocol.BaseState{
			OwnerID: p.ID, HP: p.Base.HP, MaxHP: p.Base.MaxHP, X: p.Base.X, Y: p.Base.Y, W: p.Base.W, H: p.Base.H,
		})
	}
	return protocol.StateDelta{UnitsUpsert: upserts, UnitsRemoved: removed, Projectiles: projectiles, Bases: bases}
}

func (g *Game) FullSnapshot() protocol.FullSnapshot {
	units := make([]protocol.UnitState, 0, len(g.units))
	for _, u := range g.units {
		units = append(units, protocol.UnitState{
			ID: u.ID, Name: u.Name, X: u.X, Y: u.Y, HP: u.HP, MaxHP: u.MaxHP, OwnerID: u.OwnerID, Facing: u.Facing, Class: u.Class, Range: u.Range, Particle: u.Particle,
		})
	}
	bases := make([]protocol.BaseState, 0, len(g.players))
	for _, p := range g.players {
		bases = append(bases, protocol.BaseState{OwnerID: p.ID, HP: p.Base.HP, MaxHP: p.Base.MaxHP, X: p.Base.X, Y: p.Base.Y, W: p.Base.W, H: p.Base.H})
	}
	return protocol.FullSnapshot{Units: units, Bases: bases}
}

func (g *Game) findTarget(u *Unit) (float64, float64) {
	var best *Unit
	bestDist := math.MaxFloat64
	for _, v := range g.units {
		if v.OwnerID == u.OwnerID || v.HP <= 0 {
			continue
		}
		if d := hypot(u.X, u.Y, v.X, v.Y); d < bestDist {
			bestDist, best = d, v
		}
	}
	if best != nil {
		return best.X, best.Y
	}
	// enemy base
	for _, p := range g.players {
		if p.ID != u.OwnerID {
			return float64(p.Base.X + p.Base.W/2), float64(p.Base.Y + p.Base.H/2)
		}
	}
	return float64(g.width / 2), float64(g.height / 2)
}

func (g *Game) damageAt(u *Unit, tx, ty float64, dmg int) {
	// Determine projectile type based on unit name
	projectileType := g.determineProjectileType(u.Name)

	// Check if targeting a unit
	for _, v := range g.units {
		if v.OwnerID == u.OwnerID || v.HP <= 0 {
			continue
		}
		if hypot(tx, ty, v.X, v.Y) <= 30 {
			// Create projectile targeting this unit
			g.createProjectile(u.X, u.Y, v.X, v.Y, dmg, u.OwnerID, v.ID, 0, 0, projectileType)
			return
		}
	}

	// Check if targeting a base
	for _, p := range g.players {
		if p.ID == u.OwnerID {
			continue
		}
		bx := float64(p.Base.X + p.Base.W/2)
		by := float64(p.Base.Y + p.Base.H/2)
		if hypot(tx, ty, bx, by) <= 40 {
			// Create projectile targeting this base
			g.createProjectile(u.X, u.Y, bx, by, dmg, u.OwnerID, 0, bx, by, projectileType)
			return
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

// createProjectile creates a new projectile
func (g *Game) createProjectile(startX, startY, targetX, targetY float64, damage int, ownerID, targetID int64, targetBaseX, targetBaseY float64, projectileType string) {
	projectile := &Projectile{
		ID:             protocol.NewID(),
		X:              startX,
		Y:              startY,
		TX:             targetX,
		TY:             targetY,
		Damage:         damage,
		OwnerID:        ownerID,
		TargetID:       targetID,
		TargetX:        targetBaseX,
		TargetY:        targetBaseY,
		Active:         true,
		ProjectileType: projectileType,
		Speed:          400, // pixels per second
	}

	// Calculate velocity vector
	dx := targetX - startX
	dy := targetY - startY
	dist := math.Hypot(dx, dy)
	if dist > 0 {
		projectile.VX = (dx / dist) * projectile.Speed
		projectile.VY = (dy / dist) * projectile.Speed
	}

	g.projectiles[projectile.ID] = projectile
}

// updateProjectiles updates all active projectiles
func (g *Game) updateProjectiles(dt float64) {
	for id, proj := range g.projectiles {
		if !proj.Active {
			delete(g.projectiles, id)
			continue
		}

		// Move projectile
		proj.X += proj.VX * dt
		proj.Y += proj.VY * dt

		// Check if projectile reached its target
		var targetX, targetY float64
		if proj.TargetID != 0 {
			// Targeting a unit
			if targetUnit, exists := g.units[proj.TargetID]; exists && targetUnit.HP > 0 {
				targetX, targetY = targetUnit.X, targetUnit.Y
			} else {
				// Target unit is dead or gone, deactivate projectile
				proj.Active = false
				continue
			}
		} else {
			// Targeting a base
			targetX, targetY = proj.TargetX, proj.TargetY
		}

		// Check distance to target
		dist := math.Hypot(proj.X-targetX, proj.Y-targetY)
		if dist <= 15 { // Hit threshold
			// Deal damage
			g.applyProjectileDamage(proj)
			proj.Active = false
		}
	}
}

// applyProjectileDamage applies damage from a projectile to its target
func (g *Game) applyProjectileDamage(proj *Projectile) {
	if proj.TargetID != 0 {
		// Damage primary target unit
		if targetUnit, exists := g.units[proj.TargetID]; exists {
			targetUnit.HP -= proj.Damage
			if targetUnit.HP < 0 {
				targetUnit.HP = 0
			}
		}

		// Apply AoE damage for certain projectile types
		g.applyAoEDamage(proj)
	} else {
		// Damage base
		for _, p := range g.players {
			if p.ID != proj.OwnerID {
				bx := float64(p.Base.X + p.Base.W/2)
				by := float64(p.Base.Y + p.Base.H/2)
				if math.Hypot(proj.X-bx, proj.Y-by) <= 50 {
					originalHP := p.Base.HP
					p.Base.HP -= proj.Damage
					if p.Base.HP < 0 {
						p.Base.HP = 0
					}

					// Send base damage event
					if g.broadcastEvent != nil {
						damageEvent := protocol.BaseDamageEvent{
							BaseID:       p.ID,
							BaseX:        bx,
							BaseY:        by,
							Damage:       originalHP - p.Base.HP,
							AttackerID:   0, // Projectile doesn't have attacker unit ID
							AttackerName: "Projectile",
							BaseHP:       p.Base.HP,
							BaseMaxHP:    p.Base.MaxHP,
						}
						g.broadcastEvent("BaseDamageEvent", damageEvent)
					}
					break
				}
			}
		}
	}
}

// applyAoEDamage applies area of effect damage around the projectile impact
func (g *Game) applyAoEDamage(proj *Projectile) {
	// Only apply AoE for certain projectile types
	if proj.ProjectileType != "fire" {
		return
	}

	aoeRadius := 60.0                            // 60 pixel radius for AoE
	aoeDamage := int(float64(proj.Damage) * 0.5) // 50% of primary damage

	// Find all enemy units within AoE radius
	for _, unit := range g.units {
		if unit.OwnerID == proj.OwnerID || unit.HP <= 0 {
			continue
		}

		// Check if unit is within AoE radius
		dist := math.Hypot(unit.X-proj.X, unit.Y-proj.Y)
		if dist <= aoeRadius {
			// Don't damage the primary target again
			if unit.ID != proj.TargetID {
				originalHP := unit.HP
				unit.HP -= aoeDamage
				if unit.HP < 0 {
					unit.HP = 0
				}

				// Send AoE damage event
				if g.broadcastEvent != nil {
					aoeDamageEvent := protocol.AoEDamageEvent{
						TargetID:     unit.ID,
						TargetX:      unit.X,
						TargetY:      unit.Y,
						Damage:       originalHP - unit.HP,
						AttackerID:   0, // Projectile doesn't have attacker unit ID
						AttackerName: "AoE Fire",
						TargetName:   unit.Name,
						ImpactX:      proj.X,
						ImpactY:      proj.Y,
					}
					g.broadcastEvent("AoEDamageEvent", aoeDamageEvent)
				}
			}
		}
	}
}

func (g *Game) HandleDeploy(pid int64, d protocol.DeployMiniAt) {
	p, ok := g.players[pid]
	if !ok {
		log.Printf("deploy: unknown player %d", pid)
		return
	}
	if d.CardIndex < 0 || d.CardIndex >= len(p.Hand) {
		log.Printf("deploy: bad index %d (hand=%d)", d.CardIndex, len(p.Hand))
		return
	}
	card := p.Hand[d.CardIndex]
	if p.Gold < card.Cost {
		log.Printf("deploy: not enough gold pid=%d have=%d need=%d", pid, p.Gold, card.Cost)
		return
	}
	// TODO: strict spawn zones; for now allow anywhere your client permits
	p.Gold -= card.Cost

	// Calculate attack cooldown from JSON data
	attackCooldown := 2.0 // default
	if card.Cooldown > 0 {
		attackCooldown = card.Cooldown
	} else if card.AttackSpeed > 0 {
		attackCooldown = 1.0 / card.AttackSpeed
	}

	u := &Unit{
		ID:   protocol.NewID(),
		Name: card.Name,
		X:    d.X, Y: d.Y,
		HP: max1(card.HP, 1), MaxHP: max1(card.HP, 1),
		DMG:            card.DMG,
		Heal:           card.Heal,
		Hps:            card.Hps,
		Speed:          speedPx(card.Speed),
		BaseSpeed:      speedPx(card.Speed),
		OwnerID:        pid,
		Class:          card.Class,
		SubClass:       card.SubClass,
		Range:          card.Range,
		Particle:       card.Particle,
		AttackCooldown: attackCooldown,
		SpeedDebuffs:   make(map[int64]float64),
	}
	g.units[u.ID] = u

	// Broadcast unit spawn event for visual effects
	if g.broadcastEvent != nil {
		spawnEvent := protocol.UnitSpawnEvent{
			UnitID:       u.ID,
			UnitX:        u.X,
			UnitY:        u.Y,
			UnitName:     u.Name,
			UnitClass:    u.Class,
			UnitSubclass: u.SubClass,
			OwnerID:      u.OwnerID,
		}
		g.broadcastEvent("UnitSpawnEvent", spawnEvent)
	}

	// rotate handnow i see bases and enemy attacks mine but i still cannot place units
	played := p.Hand[d.CardIndex]
	if p.Next != nil {
		p.Hand[d.CardIndex] = *p.Next
	}
	if len(p.Queue) > 0 {
		p.Queue = append(p.Queue[1:], played)
	}
	if len(p.Queue) > 0 {
		nx := p.Queue[0]
		p.Next = &nx
	} else {
		p.Next = nil
	}

}

// HandleSpellCast processes spell casting from a player
func (g *Game) HandleSpellCast(pid int64, s protocol.CastSpell) {
	p, ok := g.players[pid]
	if !ok {
		log.Printf("spell cast: unknown player %d", pid)
		return
	}

	// Find the spell card in hand
	spellIndex := -1
	for i, card := range p.Hand {
		if card.Name == s.SpellName {
			spellIndex = i
			break
		}
	}

	if spellIndex == -1 {
		log.Printf("spell cast: spell %s not found in hand for player %d", s.SpellName, pid)
		return
	}

	spellCard := p.Hand[spellIndex]
	if p.Gold < spellCard.Cost {
		log.Printf("spell cast: not enough gold pid=%d have=%d need=%d", pid, p.Gold, spellCard.Cost)
		return
	}

	// Deduct gold
	p.Gold -= spellCard.Cost

	// Process spell effects based on spell name
	g.processSpellEffect(pid, s)

	// Broadcast spell cast event for visual effects
	if g.broadcastEvent != nil {
		spellEvent := protocol.SpellCastEvent{
			SpellName: s.SpellName,
			CasterID:  pid,
			TargetX:   s.X,
			TargetY:   s.Y,
		}
		g.broadcastEvent("SpellCastEvent", spellEvent)
	}

	// Remove spell from hand and rotate cards
	played := p.Hand[spellIndex]
	if p.Next != nil {
		p.Hand[spellIndex] = *p.Next
	}
	if len(p.Queue) > 0 {
		p.Queue = append(p.Queue[1:], played)
	}
	if len(p.Queue) > 0 {
		nx := p.Queue[0]
		p.Next = &nx
	} else {
		p.Next = nil
	}
}

// processSpellEffect applies the effects of a spell based on its name
func (g *Game) processSpellEffect(casterID int64, s protocol.CastSpell) {
	switch s.SpellName {
	case "Arcane Blast":
		// Single target damage spell
		g.castArcaneBlast(casterID, s.X, s.Y)

	case "Blizzard":
		// Area damage spell
		g.castBlizzard(casterID, s.X, s.Y)

	case "Chain Lightning":
		// Chain damage spell
		g.castChainLightning(casterID, s.X, s.Y)

	case "Execute":
		// Execute low HP targets
		g.castExecute(casterID, s.X, s.Y)

	case "Holy Nova":
		// Area healing/damage spell
		g.castHolyNova(casterID, s.X, s.Y)

	case "Living Bomb":
		// DoT spell
		g.castLivingBomb(casterID, s.X, s.Y)

	case "Earth and Moon":
		// Area damage spell
		g.castEarthAndMoon(casterID, s.X, s.Y)

	case "Polymorph":
		// Utility spell
		g.castPolymorph(casterID, s.X, s.Y)

	case "Smoke Bomb":
		// Area utility spell
		g.castSmokeBomb(casterID, s.X, s.Y)

	case "Whelp Eggs":
		// Summon spell
		g.castWhelpEggs(casterID, s.X, s.Y)
	}
}

// Individual spell casting functions
func (g *Game) castArcaneBlast(casterID int64, targetX, targetY float64) {
	damage := 200
	// Find closest enemy unit within range
	for _, unit := range g.units {
		if unit.OwnerID == casterID || unit.HP <= 0 {
			continue
		}
		dist := hypot(targetX, targetY, unit.X, unit.Y)
		if dist <= 50 { // 50 pixel range
			unit.HP -= damage
			if unit.HP < 0 {
				unit.HP = 0
			}
			break // Only hit first target
		}
	}
}

func (g *Game) castBlizzard(casterID int64, targetX, targetY float64) {
	damage := 50 // Damage per tick (total 250 over 5 seconds)
	radius := 120.0
	duration := 5.0     // 5 seconds
	tickInterval := 1.0 // 1 second ticks

	// Create blizzard area effect
	blizzardID := protocol.NewID()

	g.blizzards[blizzardID] = &BlizzardEffect{
		ID:           blizzardID,
		CasterID:     casterID,
		X:            targetX,
		Y:            targetY,
		Radius:       radius,
		Damage:       damage,
		Duration:     duration,
		TickInterval: tickInterval,
		LastTick:     0,
		StartTime:    0, // Will be set when first tick occurs
	}

	// Initial damage tick
	g.tickBlizzard(blizzardID, 0)
}

func (g *Game) castChainLightning(casterID int64, targetX, targetY float64) {
	damage := 120
	jumps := 3
	reduction := 0.6 // 60% damage reduction per jump

	// Find initial target
	var initialTarget *Unit
	minDist := math.MaxFloat64
	for _, unit := range g.units {
		if unit.OwnerID == casterID || unit.HP <= 0 {
			continue
		}
		dist := hypot(targetX, targetY, unit.X, unit.Y)
		if dist < minDist {
			minDist = dist
			initialTarget = unit
		}
	}

	if initialTarget == nil {
		return
	}

	// Chain to nearby enemies
	currentTarget := initialTarget
	currentDamage := damage

	for i := 0; i < jumps; i++ {
		currentTarget.HP -= int(currentDamage)
		if currentTarget.HP < 0 {
			currentTarget.HP = 0
		}

		// Find next target
		var nextTarget *Unit
		minDist = math.MaxFloat64
		for _, unit := range g.units {
			if unit.OwnerID == casterID || unit.HP <= 0 || unit.ID == currentTarget.ID {
				continue
			}
			dist := hypot(currentTarget.X, currentTarget.Y, unit.X, unit.Y)
			if dist < minDist && dist <= 100 { // 100 pixel chain range
				minDist = dist
				nextTarget = unit
			}
		}

		if nextTarget == nil {
			break
		}

		currentTarget = nextTarget
		currentDamage = int(float64(currentDamage) * reduction)
	}
}

func (g *Game) castExecute(casterID int64, targetX, targetY float64) {
	threshold := 0.25 // 25% HP threshold
	// Find enemy unit with lowest HP percentage
	for _, unit := range g.units {
		if unit.OwnerID == casterID || unit.HP <= 0 {
			continue
		}
		dist := hypot(targetX, targetY, unit.X, unit.Y)
		if dist <= 60 && float64(unit.HP)/float64(unit.MaxHP) <= threshold {
			unit.HP = 0 // Execute
			break
		}
	}
}

func (g *Game) castHolyNova(casterID int64, targetX, targetY float64) {
	healAmount := 350 // Heal amount for friendly units
	damage := 350     // Damage amount for enemy units
	aoeDamage := 105  // 30% of healing amount (350 * 0.3) - damages ALL units
	radius := 120.0   // Same radius as Blizzard

	// First apply AoE damage to ALL units in area
	for _, unit := range g.units {
		if unit.HP <= 0 {
			continue
		}
		dist := hypot(targetX, targetY, unit.X, unit.Y)
		if dist <= radius {
			// AoE damage affects all units regardless of ownership
			unit.HP -= aoeDamage
			if unit.HP < 0 {
				unit.HP = 0
			}
		}
	}

	// Then apply healing/damage effects
	for _, unit := range g.units {
		if unit.HP <= 0 {
			continue
		}
		dist := hypot(targetX, targetY, unit.X, unit.Y)
		if dist <= radius {
			if unit.OwnerID == casterID {
				// Heal friendly units (after AoE damage)
				unit.HP += healAmount
				if unit.HP > unit.MaxHP {
					unit.HP = unit.MaxHP
				}
			} else {
				// Additional damage to enemy units (after AoE damage)
				unit.HP -= damage
				if unit.HP < 0 {
					unit.HP = 0
				}
			}
		}
	}

	// Also heal friendly bases and damage enemy bases
	for _, player := range g.players {
		if player.ID == casterID {
			// Heal friendly base
			player.Base.HP += healAmount
			if player.Base.HP > player.Base.MaxHP {
				player.Base.HP = player.Base.MaxHP
			}
		} else {
			// Damage enemy bases
			baseCenterX := float64(player.Base.X + player.Base.W/2)
			baseCenterY := float64(player.Base.Y + player.Base.H/2)
			dist := hypot(targetX, targetY, baseCenterX, baseCenterY)
			if dist <= radius {
				player.Base.HP -= damage
				if player.Base.HP < 0 {
					player.Base.HP = 0
				}
			}
		}
	}
}

func (g *Game) castLivingBomb(casterID int64, targetX, targetY float64) {
	damage := 300
	duration := 3.0 // 3 seconds
	// Find target unit
	for _, unit := range g.units {
		if unit.OwnerID == casterID || unit.HP <= 0 {
			continue
		}
		dist := hypot(targetX, targetY, unit.X, unit.Y)
		if dist <= 50 {
			// Apply DoT effect (simplified - just damage after delay)
			go func(target *Unit) {
				time.Sleep(time.Duration(duration) * time.Second)
				if target.HP > 0 {
					target.HP -= damage
					if target.HP < 0 {
						target.HP = 0
					}
				}
			}(unit)
			break
		}
	}
}

func (g *Game) castEarthAndMoon(casterID int64, targetX, targetY float64) {
	damage := 180
	radius := 120.0
	// Damage all enemy units in large area
	for _, unit := range g.units {
		if unit.OwnerID == casterID || unit.HP <= 0 {
			continue
		}
		dist := hypot(targetX, targetY, unit.X, unit.Y)
		if dist <= radius {
			unit.HP -= damage
			if unit.HP < 0 {
				unit.HP = 0
			}
		}
	}
}

func (g *Game) castPolymorph(casterID int64, targetX, targetY float64) {
	duration := 5.0 // 5 seconds
	// Find target unit
	for _, unit := range g.units {
		if unit.OwnerID == casterID || unit.HP <= 0 {
			continue
		}
		dist := hypot(targetX, targetY, unit.X, unit.Y)
		if dist <= 60 {
			// Disable unit temporarily (simplified - just prevent movement/attacks)
			unit.Speed = 0
			unit.DMG = 0
			go func(target *Unit, originalSpeed float64, originalDMG int) {
				time.Sleep(time.Duration(duration) * time.Second)
				if target.HP > 0 {
					target.Speed = originalSpeed
					target.DMG = originalDMG
				}
			}(unit, unit.Speed, unit.DMG)
			break
		}
	}
}

func (g *Game) castSmokeBomb(casterID int64, targetX, targetY float64) {
	duration := 4.0 // 4 seconds
	radius := 60.0
	// Reduce accuracy/speed of enemy units in area
	for _, unit := range g.units {
		if unit.OwnerID == casterID || unit.HP <= 0 {
			continue
		}
		dist := hypot(targetX, targetY, unit.X, unit.Y)
		if dist <= radius {
			originalSpeed := unit.Speed
			unit.Speed *= 0.5 // 50% speed reduction
			go func(target *Unit, originalSpeed float64) {
				time.Sleep(time.Duration(duration) * time.Second)
				if target.HP > 0 {
					target.Speed = originalSpeed
				}
			}(unit, originalSpeed)
		}
	}
}

func (g *Game) castWhelpEggs(casterID int64, targetX, targetY float64) {
	// Summon 3 whelps
	for i := 0; i < 3; i++ {
		offsetX := (float64(i) - 1.0) * 30 // Spread them out
		whelp := &Unit{
			ID:             protocol.NewID(),
			Name:           "Whelp",
			X:              targetX + offsetX,
			Y:              targetY + float64(rand.Intn(20)-10),
			HP:             100,
			MaxHP:          100,
			DMG:            30,
			Speed:          80,
			BaseSpeed:      80,
			OwnerID:        casterID,
			Class:          "melee",
			Range:          20,
			AttackCooldown: 1.5,
			SpeedDebuffs:   make(map[int64]float64),
		}
		g.units[whelp.ID] = whelp
	}
}

// Info needed to render the army picker
type MiniInfo struct {
	Name     string `json:"name"`
	Class    string `json:"class"`
	Role     string `json:"role"`
	Cost     int    `json:"cost"`
	Portrait string `json:"portrait,omitempty"`
}

// LoadLobbyMinis returns a lightweight list of minis for the Army UI.
func LoadLobbyMinis() []protocol.MiniInfo {
	g := NewGame() // this loads minis.json via g.loadMinis()
	out := make([]protocol.MiniInfo, 0, len(g.minis))
	for _, m := range g.minis {
		out = append(out, protocol.MiniInfo{
			Name:        m.Name,
			Class:       m.Class,
			SubClass:    m.SubClass,
			Role:        m.Role,
			Cost:        m.Cost,
			Portrait:    m.Portrait,
			Dmg:         m.DMG,
			Hp:          m.HP,
			Heal:        m.Heal,
			Hps:         m.Hps,
			Speed:       int(math.Round(m.Speed)),
			AttackSpeed: m.AttackSpeed,
		})
	}
	return out
}

// DefaultAIArmy picks 1 champion + 6 cheapest non-spell minis from loaded minis.json
func (g *Game) DefaultAIArmy() []string {
	var champ string
	type miniCost struct {
		name string
		cost int
	}
	minis := []miniCost{}

	for _, m := range g.minis {
		role := strings.ToLower(m.Role)
		class := strings.ToLower(m.Class)
		if champ == "" && (role == "champion" || class == "champion") {
			champ = m.Name
		} else if role == "mini" {
			minis = append(minis, miniCost{m.Name, m.Cost})
		}
	}
	sort.Slice(minis, func(i, j int) bool { return minis[i].cost < minis[j].cost })
	army := []string{champ}
	for i := 0; i < len(minis) && len(army) < 7; i++ {
		army = append(army, minis[i].name)
	}
	if len(army) < 7 {
		// fallback: duplicate cheapest to fill
		for len(army) < 7 && len(minis) > 0 {
			army = append(army, minis[0].name)
		}
	}
	return army
}
