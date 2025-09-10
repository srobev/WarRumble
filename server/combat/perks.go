package combat

import (
	"math"
	"time"
)

// Vec2 represents 2D coordinates
type Vec2 struct {
	X, Y float64
}

// PerkEffects represents the effect data for a perk
type PerkEffects struct {
	Type           string  `json:"type"`
	Radius         float64 `json:"radius,omitempty"`
	SlowPct        float64 `json:"slow_pct,omitempty"`
	Nth            int     `json:"nth,omitempty"`
	BonusDmgPct    float64 `json:"bonus_dmg_pct,omitempty"`
	DurationMs     int     `json:"duration_ms,omitempty"`
	AllyRadius     float64 `json:"ally_radius,omitempty"`
	DrPct          float64 `json:"dr_pct,omitempty"`
	ArmorBonusPct  float64 `json:"armor_bonus_pct,omitempty"`
	HpThresholdPct float64 `json:"hp_threshold_pct,omitempty"`
	AllyDmgPct     float64 `json:"ally_dmg_pct,omitempty"`
}

// Perk represents a single perk
type Perk struct {
	ID      string      `json:"id"`
	Name    string      `json:"name"`
	Desc    string      `json:"desc"`
	Effects PerkEffects `json:"effects"`
}

// UnitRuntime represents a unit in combat with perk state
type UnitRuntime struct {
	ID         int64
	TeamID     string
	Pos        Vec2
	HP         int
	MaxHP      int
	Alive      bool
	ActivePerk *Perk
	PerkState  struct {
		AttackCount  int
		LastTargetID int64 // Track target switches for nth-attack reset
	}
	// Squad mechanics (for Swordsman spawn_count=4)
	SquadID       int64
	IsSquadLeader bool
	// Combat bonuses
	SpeedModifier         float64
	AttackMultiplier      float64
	DamageTakenMultiplier float64
}

// World interface for combat environment
type World interface {
	UnitsAll() []*UnitRuntime
	AlliesOf(u *UnitRuntime) []*UnitRuntime
	EnemiesOf(u *UnitRuntime) []*UnitRuntime
	ApplyTimedSlow(tag string, target *UnitRuntime, slowPct float64, duration time.Duration)
	ApplyOrUpdateSlow(tag string, target *UnitRuntime, slowPct float64)           // strongest-wins inside
	ApplyOrUpdateBuff(tag string, target *UnitRuntime, fields map[string]float64) // multiplies attack, etc.
	Distance(a, b Vec2) float64
	SameTeam(a, b *UnitRuntime) bool
}

// TickPerkAuras processes aura effects every tick
func TickPerkAuras(w World, dt time.Duration) {
	for _, u := range w.UnitsAll() {
		if u.ActivePerk == nil || !u.Alive {
			continue
		}
		e := u.ActivePerk.Effects
		switch e.Type {
		case "aura_slow":
			for _, enemy := range w.EnemiesOf(u) {
				if w.Distance(u.Pos, enemy.Pos) <= e.Radius {
					w.ApplyOrUpdateSlow("perk_aura_slow", enemy, e.SlowPct)
				}
			}
		case "aura_ally_dmg":
			for _, ally := range w.AlliesOf(u) {
				if ally.ID == u.ID {
					continue
				}
				if w.Distance(u.Pos, ally.Pos) <= e.Radius {
					w.ApplyOrUpdateBuff("perk_ally_dmg", ally, map[string]float64{"atk_mult": 1.0 + e.AllyDmgPct})
				}
			}
		}
	}
}

// OnAttackDamageMultiplier returns damage multiplier for attacker
func OnAttackDamageMultiplier(attacker, target *UnitRuntime) float64 {
	m := attacker.AttackMultiplier
	if attacker.ActivePerk == nil {
		return m
	}

	e := attacker.ActivePerk.Effects
	switch e.Type {
	case "nth_attack_bonus":
		attacker.PerkState.AttackCount++
		if attacker.PerkState.AttackCount >= e.Nth {
			m *= (1.0 + e.BonusDmgPct)
			attacker.PerkState.AttackCount = 0
		}
	}

	return m
}

// OnDamageTakenMultiplier returns damage taken multiplier for target
func OnDamageTakenMultiplier(target, attacker *UnitRuntime) float64 {
	m := target.DamageTakenMultiplier
	if target.ActivePerk != nil {
		e := target.ActivePerk.Effects
		switch e.Type {
		case "conditional_dr_near_ally":
			if hasAllyNearby(target, e.AllyRadius, attacker, attacker.TeamID) {
				m *= (1.0 - e.DrPct)
			}
		case "low_hp_armor":
			if float64(target.HP)/float64(target.MaxHP) <= e.HpThresholdPct {
				m *= (1.0 - e.ArmorBonusPct)
			}
		}
	}
	return m
}

// OnUnitDeath processes death effects
func OnUnitDeath(u *UnitRuntime, w World) {
	if u.ActivePerk == nil {
		return
	}
	e := u.ActivePerk.Effects
	if e.Type == "ondeath_aoe_slow" {
		dur := time.Duration(e.DurationMs) * time.Millisecond
		for _, enemy := range w.EnemiesOf(u) {
			if w.Distance(u.Pos, enemy.Pos) <= e.Radius {
				w.ApplyTimedSlow("perk_frozen_veil", enemy, e.SlowPct, dur)
			}
		}
	}
}

// hasAllyNearby checks if there's an ally nearby (excluding recently killed attacker)
//
// This is a simplified stub for testing. In production, this should integrate
// with the game engine's spatial partitioning system for efficient O(k) queries.
//
// TODO: Replace with World.QueryAlliesNearby(u *UnitRuntime, radius float64) []*UnitRuntime
func hasAllyNearby(u *UnitRuntime, radius float64, excludeUnit *UnitRuntime, teamID string) bool {
	// Stub implementation that always returns true for testing
	// In production, this should query and use teamID filtering
	return true
}

// distance calculates distance between two positions
func distance(a, b Vec2) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return math.Sqrt(dx*dx + dy*dy)
}

// LoadUnitPerk loads and converts perk data for runtime use
func LoadUnitPerk(unitName string, perkID string) *Perk {
	// This should load from the persisted perk data
	// For now, return stub data based on known perks

	switch unitName {
	case "Sorceress Glacia":
		switch perkID {
		case "glacia_chill_aura":
			return &Perk{
				ID:   "glacia_chill_aura",
				Name: "Chill Aura",
				Desc: "Enemies in range move 10% slower.",
				Effects: PerkEffects{
					Type:    "aura_slow",
					Radius:  160,
					SlowPct: 0.10,
				},
			}
		case "glacia_ice_spike":
			return &Perk{
				ID:   "glacia_ice_spike",
				Name: "Ice Spike",
				Desc: "Every 4th attack deals +40% damage.",
				Effects: PerkEffects{
					Type:        "nth_attack_bonus",
					Nth:         4,
					BonusDmgPct: 0.40,
				},
			}
		case "glacia_frozen_veil":
			return &Perk{
				ID:   "glacia_frozen_veil",
				Name: "Frozen Veil",
				Desc: "On death, slows nearby enemies by 50% for 3s.",
				Effects: PerkEffects{
					Type:       "ondeath_aoe_slow",
					Radius:     140,
					SlowPct:    0.50,
					DurationMs: 3000,
				},
			}
		}
	case "Swordsman":
		switch perkID {
		case "sword_shield_wall":
			return &Perk{
				ID:   "sword_shield_wall",
				Name: "Shield Wall",
				Desc: "Takes 15% less damage while an ally is nearby.",
				Effects: PerkEffects{
					Type:       "conditional_dr_near_ally",
					DrPct:      0.15,
					AllyRadius: 140,
				},
			}
		case "sword_last_stand":
			return &Perk{
				ID:   "sword_last_stand",
				Name: "Last Stand",
				Desc: "Gain +25% Armor below 30% HP.",
				Effects: PerkEffects{
					Type:           "low_hp_armor",
					ArmorBonusPct:  0.25,
					HpThresholdPct: 0.30,
				},
			}
		case "sword_inspire":
			return &Perk{
				ID:   "sword_inspire",
				Name: "Inspiring Presence",
				Desc: "Allies in range deal +5% damage.",
				Effects: PerkEffects{
					Type:       "aura_ally_dmg",
					Radius:     140,
					AllyDmgPct: 0.05,
				},
			}
		}
	}

	return nil
}
