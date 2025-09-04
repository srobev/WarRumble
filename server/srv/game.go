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
	OwnerID        int64
	Class          string
	SubClass       string
	Range          int
	Particle       string
	Facing, CD     float64
	HealCD         float64
	AttackCooldown float64 // Configurable attack cooldown duration
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

type Game struct {
	minis       []MiniCard
	units       map[int64]*Unit
	projectiles map[int64]*Projectile
	players     map[int64]*Player
	width       int
	height      int
	mapDef      *protocol.MapDef // Current map definition

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
	// simple validation: 1 champion + 6 non-spell minis
	champ := 0
	minis := 0
	for _, c := range cards {
		r := strings.ToLower(c.Role)
		cl := strings.ToLower(c.Class)
		if r == "champion" || cl == "champion" {
			champ++
		} else if r == "mini" && cl != "spell" {
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
		case r == "mini" && c != "spell":
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
		// Damage unit
		if targetUnit, exists := g.units[proj.TargetID]; exists {
			targetUnit.HP -= proj.Damage
			if targetUnit.HP < 0 {
				targetUnit.HP = 0
			}
		}
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
		OwnerID:        pid,
		Class:          card.Class,
		SubClass:       card.SubClass,
		Range:          card.Range,
		Particle:       card.Particle,
		AttackCooldown: attackCooldown,
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
		} else if role == "mini" && class != "spell" {
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
