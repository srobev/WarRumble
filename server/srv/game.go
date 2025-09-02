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
	Name     string  `json:"name"`
	DMG      int     `json:"dmg"`
	HP       int     `json:"hp"`
	Portrait string  `json:"portrait"`
	Class    string  `json:"class"`
	Role     string  `json:"role"`
	Cost     int     `json:"cost"`
	Speed    float64 `json:"speed"`
}

type Player struct {
	ID    int64
	Name  string
	Gold  int
	GoldT float64
	Hand  []MiniCard
	Queue []MiniCard
	Next  *MiniCard
	Ready bool
	Base  Base
	Rating int    // NEW: PvP Elo
	Rank   string // NEW: derived name
}

type Base struct {
	OwnerID    int64
	HP, MaxHP  int
	X, Y, W, H int
}

type Unit struct {
	ID         int64
	Name       string
	X, Y       float64
	VX, VY     float64
	HP, MaxHP  int
	DMG        int
	Speed      float64 // px/s
	OwnerID    int64
	Class      string
	Facing, CD float64
}

type Game struct {
	minis   []MiniCard
	units   map[int64]*Unit
	players map[int64]*Player
	width   int
	height  int
}

func NewGame() *Game {
	g := &Game{
		units:   make(map[int64]*Unit),
		players: make(map[int64]*Player),
		width:   protocol.ScreenW,
		height:  protocol.ScreenH,
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

	p.Base = Base{
		OwnerID: id,
		HP:      3000, MaxHP: 3000,
		W: baseW, H: baseH,
		X: g.width/2 - baseW/2,
		Y: g.height - baseH - bottomMargin,
	}
	if len(g.players) == 1 {
		p.Base.Y = topMargin
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

func (g *Game) Step(dt float64) protocol.StateDelta {
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
			removed = append(removed, id)
			delete(g.units, id)
			continue
		}

		// target: nearest enemy unit, else enemy base
		tx, ty := g.findTarget(u)
		dx, dy := tx-u.X, ty-u.Y
		dist := math.Hypot(dx, dy)

		rng := 28.0 // melee
		if c := lower(u.Class); c == "range" || c == "ranged" {
			rng = 120
		}

		if dist <= rng {
			if u.CD <= 0 {
				g.damageAt(u, tx, ty, u.DMG)
				u.CD = 1.0
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

		upserts = append(upserts, protocol.UnitState{
			ID: u.ID, Name: u.Name, X: u.X, Y: u.Y, HP: u.HP, MaxHP: u.MaxHP,
			OwnerID: u.OwnerID, Facing: u.Facing, Class: u.Class,
		})
	}

	bases := make([]protocol.BaseState, 0, len(g.players))
	for _, p := range g.players {
		bases = append(bases, protocol.BaseState{
			OwnerID: p.ID, HP: p.Base.HP, MaxHP: p.Base.MaxHP, X: p.Base.X, Y: p.Base.Y, W: p.Base.W, H: p.Base.H,
		})
	}
	return protocol.StateDelta{UnitsUpsert: upserts, UnitsRemoved: removed, Bases: bases}
}

func (g *Game) FullSnapshot() protocol.FullSnapshot {
	units := make([]protocol.UnitState, 0, len(g.units))
	for _, u := range g.units {
		units = append(units, protocol.UnitState{
			ID: u.ID, Name: u.Name, X: u.X, Y: u.Y, HP: u.HP, MaxHP: u.MaxHP, OwnerID: u.OwnerID, Facing: u.Facing, Class: u.Class,
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
	for _, v := range g.units {
		if v.OwnerID == u.OwnerID || v.HP <= 0 {
			continue
		}
		if hypot(tx, ty, v.X, v.Y) <= 30 {
			v.HP -= dmg
			if v.HP < 0 {
				v.HP = 0
			}
			return
		}
	}
	for _, p := range g.players {
		if p.ID == u.OwnerID {
			continue
		}
		bx := float64(p.Base.X + p.Base.W/2)
		by := float64(p.Base.Y + p.Base.H/2)
		if hypot(tx, ty, bx, by) <= 40 {
			p.Base.HP -= dmg
			if p.Base.HP < 0 {
				p.Base.HP = 0
			}
			return
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

	u := &Unit{
		ID:   protocol.NewID(),
		Name: card.Name,
		X:    d.X, Y: d.Y,
		HP: max1(card.HP, 1), MaxHP: max1(card.HP, 1),
		DMG:     card.DMG,
		Speed:   speedPx(card.Speed),
		OwnerID: pid,
		Class:   card.Class,
	}
	g.units[u.ID] = u

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
            Name:     m.Name,
            Class:    m.Class,
            Role:     m.Role,
            Cost:     m.Cost,
            Portrait: m.Portrait,
            Dmg:      m.DMG,
            Hp:       m.HP,
            Speed:    int(math.Round(m.Speed)),
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
