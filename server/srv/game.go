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

	"rumble/shared"
	"rumble/shared/protocol"
)

type MiniCard struct {
	Name        string   `json:"name"`
	DMG         int      `json:"dmg"`
	HP          int      `json:"hp"`
	Heal        int      `json:"heal"`
	Hps         int      `json:"hps"`
	Portrait    string   `json:"portrait"`
	Class       string   `json:"class"`
	SubClass    string   `json:"subclass"`
	Role        string   `json:"role"`
	Cost        int      `json:"cost"`
	Speed       float64  `json:"speed"`
	Range       int      `json:"range"`
	Particle    string   `json:"particle"`
	Cooldown    float64  `json:"cooldown,omitempty"`     // Attack cooldown in seconds
	AttackSpeed float64  `json:"attack_speed,omitempty"` // Attacks per second (alternative to cooldown)
	SpawnCount  int      `json:"spawn_count,omitempty"`  // Number of units to spawn (default: 1)
	Features    []string `json:"features,omitempty"`     // Special features like "siege"
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
	Features       []string          // Special features like "siege"
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

	// Unit targeting system
	targetValidator *shared.UnitTargetValidator

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

	// Initialize unit targeting system
	if validator, err := shared.NewUnitTargetValidator(); err != nil {
		log.Printf("Failed to initialize unit target validator: %v", err)
	} else {
		g.targetValidator = validator
	}

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
			// Calculate movement toward target
			nx, ny := dx/dist, dy/dist
			desiredX := u.X + nx*u.Speed*dt
			desiredY := u.Y + ny*u.Speed*dt

			// Declare variables for final position
			var newX, newY float64

			// Debug: Check map definition and obstacles
			if g.mapDef == nil {
				log.Printf("DEBUG: Unit %s - No map definition loaded", u.Name)
			} else if len(g.mapDef.Obstacles) == 0 {
				log.Printf("DEBUG: Unit %s - No obstacles in map", u.Name)
			} else {
				log.Printf("DEBUG: Unit %s at (%.1f,%.1f) moving to (%.1f,%.1f), %d obstacles",
					u.Name, u.X, u.Y, desiredX, desiredY, len(g.mapDef.Obstacles))
			}

			// Check if desired position would collide with obstacle (with safety margin)
			if g.isPointNearObstacle(desiredX, desiredY, 25) { // 25 pixel safety margin for proactive avoidance
				log.Printf("DEBUG: Unit %s - COLLISION DETECTED at (%.1f,%.1f)", u.Name, desiredX, desiredY)

				// First try: Move directly toward target but stop before obstacle
				obsCenterX, obsCenterY := g.getObstacleCenter(desiredX, desiredY)
				if obsCenterX != -1 && obsCenterY != -1 {
					// Calculate vector from current position to obstacle center
					obsDx := obsCenterX - u.X
					obsDy := obsCenterY - u.Y
					distToObs := math.Hypot(obsDx, obsDy)

					if distToObs > 0 {
						// Normalize
						obsDx /= distToObs
						obsDy /= distToObs

						// Calculate safe distance (stop before entering obstacle)

						// Calculate a position along the path to target, but stopping before obstacle
						// Instead of moving away from obstacle center, move toward target but stop early
						targetDx := tx - u.X
						targetDy := ty - u.Y
						distToTarget := math.Hypot(targetDx, targetDy)

						if distToTarget > 0 {
							// Normalize target direction
							targetDx /= distToTarget
							targetDy /= distToTarget

							// Calculate how far we can safely move toward target
							maxSafeDist := u.Speed * dt
							safeDist := maxSafeDist

							// Check if moving the full distance would hit obstacle
							testX := u.X + targetDx*maxSafeDist
							testY := u.Y + targetDy*maxSafeDist

							if g.isPointNearObstacle(testX, testY, 5) { // 5 pixel safety margin for binary search
								// Find the maximum distance we can move without hitting obstacle
								// Use binary search to find safe distance
								minDist := 0.0
								maxDist := maxSafeDist
								bestDist := 0.0

								for i := 0; i < 8; i++ { // 8 iterations for good precision
									midDist := (minDist + maxDist) / 2
									testX := u.X + targetDx*midDist
									testY := u.Y + targetDy*midDist

									if g.isPointInObstacle(testX, testY) {
										maxDist = midDist
									} else {
										minDist = midDist
										bestDist = midDist
									}
								}

								safeDist = bestDist * 0.9 // Use 90% of safe distance for extra margin
							}

							// Calculate final safe position
							safeX := u.X + targetDx*safeDist
							safeY := u.Y + targetDy*safeDist

							// Ensure we moved at least a minimum distance to avoid getting stuck
							minMoveDist := u.Speed * dt * 0.005 // At least 0.5% of max movement (very lenient)
							actualDist := math.Hypot(safeX-u.X, safeY-u.Y)

							if actualDist >= minMoveDist && !g.isPointNearObstacle(safeX, safeY, 2) { // 2 pixel safety margin for final check
								log.Printf("DEBUG: Unit %s - Using progressive safe approach: (%.1f,%.1f) dist=%.3f", u.Name, safeX, safeY, actualDist)
								newX, newY = safeX, safeY
							} else {
								log.Printf("DEBUG: Unit %s - Progressive approach too short (%.3f < %.3f), trying back-away strategy", u.Name, actualDist, minMoveDist)
								// Try to back away from obstacle first, then try perpendicular
								newX, newY = g.tryBackAwayStrategy(u, obsCenterX, obsCenterY, dt)
								if newX == u.X && newY == u.Y {
									// Back away failed, try perpendicular
									newX, newY = g.tryPerpendicularMovement(u, obsCenterX, obsCenterY, dt)
								}
							}
						} else {
							newX, newY = g.tryPerpendicularMovement(u, obsCenterX, obsCenterY, dt)
						}
					} else {
						newX, newY = g.tryPerpendicularMovement(u, obsCenterX, obsCenterY, dt)
					}
				} else {
					log.Printf("DEBUG: Unit %s - Could not find obstacle center", u.Name)
					newX, newY = desiredX, desiredY // Fallback to original desired position
				}
			} else {
				newX, newY = desiredX, desiredY
			}

			// Update position and facing
			u.Facing = math.Atan2(ny, nx)
			u.X = newX
			u.Y = newY
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
	// Special case: air-vs-base units always target the enemy base directly
	if strings.ToLower(u.SubClass) == "air-vs-base" {
		for _, p := range g.players {
			if p.ID != u.OwnerID {
				return float64(p.Base.X + p.Base.W/2), float64(p.Base.Y + p.Base.H/2)
			}
		}
	}

	var best *Unit
	bestDist := math.MaxFloat64
	for _, v := range g.units {
		if v.OwnerID == u.OwnerID || v.HP <= 0 {
			continue
		}
		// Check if attacker can target this unit
		if g.targetValidator != nil {
			canAttack := g.targetValidator.CanAttackTarget(u.Class, u.SubClass, "unit", v.Class, v.SubClass)
			if !canAttack {
				continue
			}
		}
		if d := hypot(u.X, u.Y, v.X, v.Y); d < bestDist {
			bestDist, best = d, v
		}
	}
	if best != nil {
		return best.X, best.Y
	}
	// enemy base - check if attacker can target bases
	if g.targetValidator != nil {
		canAttack := g.targetValidator.CanAttackTarget(u.Class, u.SubClass, "base", "base", "")
		if canAttack {
			for _, p := range g.players {
				if p.ID != u.OwnerID {
					return float64(p.Base.X + p.Base.W/2), float64(p.Base.Y + p.Base.H/2)
				}
			}
		}
	} else {
		// Fallback if no validator
		for _, p := range g.players {
			if p.ID != u.OwnerID {
				return float64(p.Base.X + p.Base.W/2), float64(p.Base.Y + p.Base.H/2)
			}
		}
	}
	return float64(g.width / 2), float64(g.height / 2)
}

// calculateObstacleAvoidingTarget calculates a pathfinding-adjusted target that avoids obstacles
func (g *Game) calculateObstacleAvoidingTarget(u *Unit, targetX, targetY float64) (float64, float64) {
	// If no map definition with obstacles, return direct target
	if g.mapDef == nil {
		log.Printf("DEBUG: No map definition loaded")
		return targetX, targetY
	}

	if len(g.mapDef.Obstacles) == 0 {
		log.Printf("DEBUG: No obstacles in map definition")
		return targetX, targetY
	}

	log.Printf("DEBUG: Unit at (%.1f, %.1f) targeting (%.1f, %.1f), %d obstacles", u.X, u.Y, targetX, targetY, len(g.mapDef.Obstacles))

	// Check if the direct path is clear
	if g.isDirectPathClear(u.X, u.Y, targetX, targetY) {
		return targetX, targetY
	}

	// Path is blocked, find obstacle-avoiding path
	return g.findObstacleAvoidingPath(u.X, u.Y, targetX, targetY)
}

// isDirectPathClear checks if the direct path from start to target is clear of obstacles
func (g *Game) isDirectPathClear(startX, startY, targetX, targetY float64) bool {
	if g.mapDef == nil {
		return true
	}

	dx := targetX - startX
	dy := targetY - startY
	dist := math.Hypot(dx, dy)

	if dist == 0 {
		return true
	}

	// Check points along the path
	steps := int(dist / 10) // Check every 10 pixels
	if steps < 3 {
		steps = 3
	}

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		checkX := startX + dx*t
		checkY := startY + dy*t

		// Check if this point collides with any obstacle
		for _, obstacle := range g.mapDef.Obstacles {
			obsX := obstacle.X * float64(g.width)
			obsY := obstacle.Y * float64(g.height)
			obsW := obstacle.Width * float64(g.width)
			obsH := obstacle.Height * float64(g.height)

			if checkX >= obsX && checkX <= obsX+obsW &&
				checkY >= obsY && checkY <= obsY+obsH {
				return false // Path is blocked
			}
		}
	}

	return true // Path is clear
}

// findObstacleAvoidingPath finds a path around obstacles using simple detour logic
func (g *Game) findObstacleAvoidingPath(startX, startY, targetX, targetY float64) (float64, float64) {
	if g.mapDef == nil {
		return targetX, targetY
	}

	// Calculate the main direction vector
	dx := targetX - startX
	dy := targetY - startY
	distToTarget := math.Hypot(dx, dy)

	if distToTarget == 0 {
		return targetX, targetY
	}

	// Normalize direction
	dx /= distToTarget
	dy /= distToTarget

	// Find the closest blocking obstacle
	var closestObstacle *protocol.Obstacle
	minDist := float64(999999)

	for _, obstacle := range g.mapDef.Obstacles {
		obsX := obstacle.X * float64(g.width)
		obsY := obstacle.Y * float64(g.height)
		obsW := obstacle.Width * float64(g.width)
		obsH := obstacle.Height * float64(g.height)

		// Calculate distance from start to obstacle center
		obsCenterX := obsX + obsW/2
		obsCenterY := obsY + obsH/2
		dist := math.Hypot(obsCenterX-startX, obsCenterY-startY)

		if dist < minDist {
			minDist = dist
			obs := obstacle // Copy to avoid reference issues
			closestObstacle = &obs
		}
	}

	if closestObstacle == nil {
		return targetX, targetY
	}

	// Calculate obstacle center
	obsCenterX := closestObstacle.X*float64(g.width) + (closestObstacle.Width*float64(g.width))/2
	obsCenterY := closestObstacle.Y*float64(g.height) + (closestObstacle.Height*float64(g.height))/2

	// Calculate perpendicular vector for steering
	obsDx := obsCenterX - startX
	obsDy := obsCenterY - startY
	distToObs := math.Hypot(obsDx, obsDy)

	if distToObs == 0 {
		return targetX, targetY
	}

	// Normalize obstacle direction
	obsDx /= distToObs
	obsDy /= distToObs

	// Calculate perpendicular vector (90 degrees rotation)
	perpDx := -obsDy
	perpDy := obsDx

	// Choose detour direction based on target alignment
	dotProduct := dx*perpDx + dy*perpDy
	steerDx := perpDx
	steerDy := perpDy
	if dotProduct < 0 {
		steerDx = -perpDx
		steerDy = -perpDy
	}

	// Calculate detour point
	obstacleRadius := math.Max(closestObstacle.Width*float64(g.width), closestObstacle.Height*float64(g.height)) / 2
	detourDistance := obstacleRadius + 60 // 60 pixel buffer for more distance
	detourX := obsCenterX + steerDx*detourDistance
	detourY := obsCenterY + steerDy*detourDistance

	// Ensure detour point is not in another obstacle
	if !g.isPointInObstacle(detourX, detourY) {
		return detourX, detourY
	}

	// If detour point is blocked, try the opposite direction
	detourX = obsCenterX - steerDx*detourDistance
	detourY = obsCenterY - steerDy*detourDistance

	if !g.isPointInObstacle(detourX, detourY) {
		return detourX, detourY
	}

	// If both directions are blocked, return original target
	return targetX, targetY
}

// isPointInObstacle checks if a point collides with any obstacle (with safety margin)
func (g *Game) isPointInObstacle(x, y float64) bool {
	return g.isPointNearObstacle(x, y, 0) // 0 pixel safety margin for exact collision
}

// isPointNearObstacle checks if a point is near any obstacle within a safety margin
func (g *Game) isPointNearObstacle(x, y float64, safetyMargin float64) bool {
	if g.mapDef == nil {
		return false
	}

	for _, obstacle := range g.mapDef.Obstacles {
		obsX := obstacle.X * float64(g.width)
		obsY := obstacle.Y * float64(g.height)
		obsW := obstacle.Width * float64(g.width)
		obsH := obstacle.Height * float64(g.height)

		// Add safety margin around obstacle
		safeObsX := obsX - safetyMargin
		safeObsY := obsY - safetyMargin
		safeObsW := obsW + 2*safetyMargin
		safeObsH := obsH + 2*safetyMargin

		if x >= safeObsX && x <= safeObsX+safeObsW && y >= safeObsY && y <= safeObsY+safeObsH {
			return true
		}
	}
	return false
}

// isDetourPathSafe checks if the path from start to detour point is clear of obstacles
func (g *Game) isDetourPathSafe(startX, startY, detourX, detourY float64) bool {
	if g.mapDef == nil {
		return true
	}

	// Check if detour point itself is safe
	if g.isPointNearObstacle(detourX, detourY, 15) { // 15 pixel safety margin for detour points
		return false
	}

	// Check points along the path from start to detour
	dx := detourX - startX
	dy := detourY - startY
	dist := math.Hypot(dx, dy)

	if dist == 0 {
		return true
	}

	// Check every 8 pixels along the path
	steps := int(dist / 8)
	if steps < 2 {
		steps = 2
	}

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		checkX := startX + dx*t
		checkY := startY + dy*t

		// Check if this point on the path collides with any obstacle
		if g.isPointNearObstacle(checkX, checkY, 10) { // 10 pixel safety margin for path checking
			return false
		}
	}

	return true // Path is clear
}

// getObstacleCenter finds the center of the obstacle that contains the given point
func (g *Game) getObstacleCenter(x, y float64) (float64, float64) {
	if g.mapDef == nil {
		return -1, -1
	}

	for _, obstacle := range g.mapDef.Obstacles {
		obsX := obstacle.X * float64(g.width)
		obsY := obstacle.Y * float64(g.height)
		obsW := obstacle.Width * float64(g.width)
		obsH := obstacle.Height * float64(g.height)

		if x >= obsX && x <= obsX+obsW && y >= obsY && y <= obsY+obsH {
			// Return the center of this obstacle
			centerX := obsX + obsW/2
			centerY := obsY + obsH/2
			return centerX, centerY
		}
	}
	return -1, -1 // No obstacle found
}

// getObstacleRadius returns the radius of the obstacle that contains the given point
func (g *Game) getObstacleRadius(x, y float64) float64 {
	if g.mapDef == nil {
		return 0
	}

	for _, obstacle := range g.mapDef.Obstacles {
		obsX := obstacle.X * float64(g.width)
		obsY := obstacle.Y * float64(g.height)
		obsW := obstacle.Width * float64(g.width)
		obsH := obstacle.Height * float64(g.height)

		if x >= obsX && x <= obsX+obsW && y >= obsY && y <= obsY+obsH {
			// Return the radius (half the diagonal)
			return math.Hypot(obsW, obsH) / 2
		}
	}
	return 0
}

// tryBackAwayStrategy tries to move away from the obstacle first, then tries perpendicular movement
func (g *Game) tryBackAwayStrategy(u *Unit, obsCenterX, obsCenterY float64, dt float64) (float64, float64) {
	// Calculate vector from obstacle center to unit (opposite of normal)
	awayDx := u.X - obsCenterX
	awayDy := u.Y - obsCenterY
	distFromObs := math.Hypot(awayDx, awayDy)

	if distFromObs == 0 {
		// Unit is exactly at obstacle center, try perpendicular instead
		return g.tryPerpendicularMovement(u, obsCenterX, obsCenterY, dt)
	}

	// Normalize away direction
	awayDx /= distFromObs
	awayDy /= distFromObs

	maxMove := u.Speed * dt

	// Try to move away from obstacle
	awayX := u.X + awayDx*maxMove
	awayY := u.Y + awayDy*maxMove

	// Check if moving away is safe
	if !g.isPointNearObstacle(awayX, awayY, 15) { // 15 pixel safety margin for back-away
		log.Printf("DEBUG: Unit %s - Using back-away strategy: (%.1f,%.1f)", u.Name, awayX, awayY)
		return awayX, awayY
	}

	// If moving directly away is blocked, try perpendicular movement
	log.Printf("DEBUG: Unit %s - Back-away blocked, trying perpendicular", u.Name)
	return g.tryPerpendicularMovement(u, obsCenterX, obsCenterY, dt)
}

// tryPerpendicularMovement attempts to move perpendicular to the obstacle when direct movement fails
func (g *Game) tryPerpendicularMovement(u *Unit, obsCenterX, obsCenterY float64, dt float64) (float64, float64) {
	// Calculate vector from current position to obstacle center
	obsDx := obsCenterX - u.X
	obsDy := obsCenterY - u.Y
	distToObs := math.Hypot(obsDx, obsDy)

	if distToObs == 0 {
		return u.X, u.Y
	}

	// Normalize obstacle direction
	obsDx /= distToObs
	obsDy /= distToObs

	// Calculate perpendicular vector (90 degrees rotation)
	perpDx := -obsDy
	perpDy := obsDx

	// Try both perpendicular directions
	maxMove := u.Speed * dt
	testX1 := u.X + perpDx*maxMove
	testY1 := u.Y + perpDy*maxMove
	testX2 := u.X - perpDx*maxMove
	testY2 := u.Y - perpDy*maxMove

	log.Printf("DEBUG: Unit %s - Testing perpendicular: (%.1f,%.1f) and (%.1f,%.1f)",
		u.Name, testX1, testY1, testX2, testY2)

	// Choose the direction that's not blocked
	if !g.isPointNearObstacle(testX1, testY1, 12) { // 12 pixel safety margin for perpendicular movement
		log.Printf("DEBUG: Unit %s - Using perpendicular direction 1", u.Name)
		return testX1, testY1
	} else if !g.isPointNearObstacle(testX2, testY2, 12) { // 12 pixel safety margin for perpendicular movement
		log.Printf("DEBUG: Unit %s - Using perpendicular direction 2", u.Name)
		return testX2, testY2
	} else {
		log.Printf("DEBUG: Unit %s - Both perpendicular directions blocked, trying detour pathfinding", u.Name)
		// If both perpendicular directions are blocked, try a more sophisticated detour
		return g.findDetourPath(u, dt)
	}
}

// findDetourPath finds a path around obstacles using a simple detour algorithm
func (g *Game) findDetourPath(u *Unit, dt float64) (float64, float64) {
	// Find target position
	tx, ty := g.findTarget(u)

	// Calculate direct path to target
	targetDx := tx - u.X
	targetDy := ty - u.Y
	distToTarget := math.Hypot(targetDx, targetDy)

	if distToTarget == 0 {
		return u.X, u.Y
	}

	// Normalize target direction
	targetDx /= distToTarget
	targetDy /= distToTarget

	maxMove := u.Speed * dt

	// Try different detour strategies
	detourStrategies := []struct {
		name        string
		getPosition func() (float64, float64)
	}{
		{
			name: "wide_detour_left",
			getPosition: func() (float64, float64) {
				// Move perpendicular to target direction, then toward target
				perpDx := -targetDy
				perpDy := targetDx
				detourX := u.X + perpDx*maxMove*0.7 + targetDx*maxMove*0.3
				detourY := u.Y + perpDy*maxMove*0.7 + targetDy*maxMove*0.3
				return detourX, detourY
			},
		},
		{
			name: "wide_detour_right",
			getPosition: func() (float64, float64) {
				// Move perpendicular to target direction (opposite), then toward target
				perpDx := targetDy
				perpDy := -targetDx
				detourX := u.X + perpDx*maxMove*0.7 + targetDx*maxMove*0.3
				detourY := u.Y + perpDy*maxMove*0.7 + targetDy*maxMove*0.3
				return detourX, detourY
			},
		},
		{
			name: "zigzag_left",
			getPosition: func() (float64, float64) {
				// Pure perpendicular movement
				perpDx := -targetDy
				perpDy := targetDx
				return u.X + perpDx*maxMove, u.Y + perpDy*maxMove
			},
		},
		{
			name: "zigzag_right",
			getPosition: func() (float64, float64) {
				// Pure perpendicular movement (opposite)
				perpDx := targetDy
				perpDy := -targetDx
				return u.X + perpDx*maxMove, u.Y + perpDy*maxMove
			},
		},
		{
			name: "diagonal_escape",
			getPosition: func() (float64, float64) {
				// Try diagonal movement
				return u.X + targetDx*maxMove*0.5 + (math.Sin(float64(u.ID)) * maxMove * 0.5),
					u.Y + targetDy*maxMove*0.5 + (math.Cos(float64(u.ID)) * maxMove * 0.5)
			},
		},
	}

	// Try each detour strategy with enhanced validation
	for _, strategy := range detourStrategies {
		detourX, detourY := strategy.getPosition()

		// Enhanced validation: check if detour point is safe and path to it is clear
		if g.isDetourPathSafe(u.X, u.Y, detourX, detourY) {
			distFromStart := math.Hypot(detourX-u.X, detourY-u.Y)
			if distFromStart <= maxMove*1.2 { // Allow slight overshoot for better pathfinding
				log.Printf("DEBUG: Unit %s - Using detour strategy: %s -> (%.1f,%.1f)",
					u.Name, strategy.name, detourX, detourY)
				return detourX, detourY
			}
		}
	}

	// If all detour strategies fail, try a simple random walk away from obstacles
	log.Printf("DEBUG: Unit %s - All detour strategies failed, trying random walk", u.Name)

	// Try random directions
	for attempt := 0; attempt < 8; attempt++ {
		angle := float64(attempt) * math.Pi / 4 // Try 8 different directions
		randomX := u.X + math.Cos(angle)*maxMove*0.5
		randomY := u.Y + math.Sin(angle)*maxMove*0.5

		if !g.isPointInObstacle(randomX, randomY) {
			log.Printf("DEBUG: Unit %s - Using random walk: (%.1f,%.1f)", u.Name, randomX, randomY)
			return randomX, randomY
		}
	}

	// If everything fails, try to move directly toward target with reduced speed
	reducedMove := maxMove * 0.3
	reducedX := u.X + targetDx*reducedMove
	reducedY := u.Y + targetDy*reducedMove

	if !g.isPointInObstacle(reducedX, reducedY) {
		log.Printf("DEBUG: Unit %s - Using reduced speed movement: (%.1f,%.1f)", u.Name, reducedX, reducedY)
		return reducedX, reducedY
	}

	// Last resort: stay put but log the issue
	log.Printf("DEBUG: Unit %s - All movement options blocked, staying put", u.Name)
	return u.X, u.Y
}

// findSafePositionAroundObstacle finds a safe position around an obstacle when direct movement is blocked
func (g *Game) findSafePositionAroundObstacle(currentX, currentY, desiredX, desiredY, targetX, targetY, maxDistance float64) (float64, float64) {
	if g.mapDef == nil {
		return desiredX, desiredY
	}

	// Find the obstacle that's blocking the path
	var blockingObstacle *protocol.Obstacle
	minDist := float64(999999)

	for _, obstacle := range g.mapDef.Obstacles {
		obsX := obstacle.X * float64(g.width)
		obsY := obstacle.Y * float64(g.height)
		obsW := obstacle.Width * float64(g.width)
		obsH := obstacle.Height * float64(g.height)

		// Check if desired position is inside this obstacle
		if desiredX >= obsX && desiredX <= obsX+obsW && desiredY >= obsY && desiredY <= obsY+obsH {
			// Calculate distance from current position to obstacle center
			obsCenterX := obsX + obsW/2
			obsCenterY := obsY + obsH/2
			distToObs := math.Hypot(obsCenterX-currentX, obsCenterY-currentY)

			if distToObs < minDist {
				minDist = distToObs
				obs := obstacle // Copy to avoid reference issues
				blockingObstacle = &obs
			}
		}
	}

	if blockingObstacle == nil {
		return desiredX, desiredY
	}

	// Calculate obstacle center
	obsCenterX := blockingObstacle.X*float64(g.width) + (blockingObstacle.Width*float64(g.width))/2
	obsCenterY := blockingObstacle.Y*float64(g.height) + (blockingObstacle.Height*float64(g.height))/2

	// Calculate vector from current position to obstacle center
	obsDx := obsCenterX - currentX
	obsDy := obsCenterY - currentY
	distToObs := math.Hypot(obsDx, obsDy)

	if distToObs == 0 {
		return desiredX, desiredY
	}

	// Normalize obstacle direction
	obsDx /= distToObs
	obsDy /= distToObs

	// Calculate perpendicular vector (90 degrees rotation) - this gives us the two possible detour directions
	perpDx := -obsDy
	perpDy := obsDx

	// Evaluate both detour directions and choose the one with shorter total path
	obstacleRadius := math.Max(blockingObstacle.Width*float64(g.width), blockingObstacle.Height*float64(g.height)) / 2
	detourDistances := []float64{obstacleRadius + 30, obstacleRadius + 15} // Try larger detour first, then smaller

	var bestDetourX, bestDetourY float64
	bestPathLength := float64(999999)
	foundValidDetour := false

	// Try both detour directions
	for _, baseDetourDist := range detourDistances {
		for _, direction := range []struct{ dx, dy float64 }{
			{perpDx, perpDy},   // One direction
			{-perpDx, -perpDy}, // Opposite direction
		} {
			detourX := obsCenterX + direction.dx*baseDetourDist
			detourY := obsCenterY + direction.dy*baseDetourDist

			// Check if detour point is valid (not in another obstacle)
			if !g.isPointInObstacle(detourX, detourY) {
				// Calculate distance from current position to detour point
				detourDx := detourX - currentX
				detourDy := detourY - currentY
				distToDetour := math.Hypot(detourDx, detourDy)

				// Only consider detours within movement range
				if distToDetour <= maxDistance {
					// Calculate total path length: current -> detour -> target
					distDetourToTarget := math.Hypot(targetX-detourX, targetY-detourY)
					totalPathLength := distToDetour + distDetourToTarget

					// Choose the detour with shortest total path
					if totalPathLength < bestPathLength {
						bestPathLength = totalPathLength
						bestDetourX = detourX
						bestDetourY = detourY
						foundValidDetour = true
					}
				} else {
					// Detour is too far, but we can scale it to fit within movement range
					if distToDetour > 0 {
						scale := maxDistance / distToDetour
						scaledDetourX := currentX + (detourX-currentX)*scale
						scaledDetourY := currentY + (detourY-currentY)*scale

						// Make sure scaled position is still safe
						if !g.isPointInObstacle(scaledDetourX, scaledDetourY) {
							// Calculate total path length for scaled detour
							distScaledToTarget := math.Hypot(targetX-scaledDetourX, targetY-scaledDetourY)
							totalPathLength := maxDistance + distScaledToTarget

							if totalPathLength < bestPathLength {
								bestPathLength = totalPathLength
								bestDetourX = scaledDetourX
								bestDetourY = scaledDetourY
								foundValidDetour = true
							}
						}
					}
				}
			}
		}
	}

	// If we found a valid detour, return it
	if foundValidDetour {
		return bestDetourX, bestDetourY
	}

	// If no detour works, try to move directly toward target but stop before obstacle
	// Calculate a position that's closer to target but doesn't enter obstacle
	safeDistance := obstacleRadius + 5 // 5 pixel safety margin
	safeX := obsCenterX - obsDx*safeDistance
	safeY := obsCenterY - obsDy*safeDistance

	// Ensure this safe position is within movement range
	safeDx := safeX - currentX
	safeDy := safeY - currentY
	safeDist := math.Hypot(safeDx, safeDy)

	if safeDist <= maxDistance && !g.isPointInObstacle(safeX, safeY) {
		return safeX, safeY
	} else if safeDist > 0 {
		// Scale to fit within movement range
		scale := maxDistance / safeDist
		safeX = currentX + safeDx*scale
		safeY = currentY + safeDy*scale
		if !g.isPointNearObstacle(safeX, safeY, 3) { // 3 pixel safety margin for final position validation
			return safeX, safeY
		}
	}

	// If all attempts fail, return current position (unit stops moving)
	return currentX, currentY
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
			// Check if attacker can target this unit
			if g.targetValidator != nil {
				canAttack := g.targetValidator.CanAttackTarget(u.Class, u.SubClass, "unit", v.Class, v.SubClass)
				if !canAttack {
					// Cannot attack this target, skip
					continue
				}
			}
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
			// Check if attacker can target bases
			if g.targetValidator != nil {
				canAttack := g.targetValidator.CanAttackTarget(u.Class, u.SubClass, "base", "base", "")
				if !canAttack {
					// Cannot attack bases, skip
					continue
				}
			}
			// Create projectile targeting this base
			g.createProjectile(u.X, u.Y, bx, by, dmg, u.OwnerID, 0, bx, by, projectileType)
			return
		}
	}
}

// determineProjectileType analyzes unit name to determine elemental projectile type
func (g *Game) determineProjectileType(unitName string) string {
	name := strings.ToLower(unitName)

	// Special case: Voodoo Hexer uses channeling beam effect
	if strings.Contains(name, "voodoo") || strings.Contains(name, "hex") {
		return "voodoo_hex"
	}

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
		// Damage base - check for siege damage
		for _, p := range g.players {
			if p.ID != proj.OwnerID {
				bx := float64(p.Base.X + p.Base.W/2)
				by := float64(p.Base.Y + p.Base.H/2)
				if math.Hypot(proj.X-bx, proj.Y-by) <= 50 {
					originalHP := p.Base.HP

					// Find the attacking unit to check for siege feature
					var attackerUnit *Unit
					for _, unit := range g.units {
						if unit.OwnerID == proj.OwnerID {
							attackerUnit = unit
							break
						}
					}

					damage := proj.Damage
					// Apply siege damage multiplier if attacker has siege feature
					if attackerUnit != nil {
						for _, feature := range attackerUnit.Features {
							if feature == "siege" {
								damage *= 2 // Double damage to base
								break
							}
						}
					}

					p.Base.HP -= damage
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

	// Determine spawn count (default to 1 if not specified)
	spawnCount := 1
	if card.SpawnCount > 0 {
		spawnCount = card.SpawnCount
	}

	// Spawn multiple units
	for i := 0; i < spawnCount; i++ {
		// Calculate position offset for multiple units
		offsetX := 0.0
		offsetY := 0.0

		if spawnCount > 1 {
			// Spread units out in a circle pattern
			angle := 2 * math.Pi * float64(i) / float64(spawnCount)
			distance := 25.0 // pixels between units
			offsetX = math.Cos(angle) * distance
			offsetY = math.Sin(angle) * distance
		}

		u := &Unit{
			ID:   protocol.NewID(),
			Name: card.Name,
			X:    d.X + offsetX, Y: d.Y + offsetY,
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
			Features:       card.Features,
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
