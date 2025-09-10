package combat

import (
	"testing"
	"time"
)

// MockWorld is a test implementation of the World interface
type MockWorld struct {
	units         []*UnitRuntime
	distanceCalls []struct {
		a, b Vec2
	}
	slowCalls []struct {
		tag     string
		slowPct float64
		dur     time.Duration
	}
	buffCalls []struct {
		tag    string
		fields map[string]float64
	}
}

func (w *MockWorld) UnitsAll() []*UnitRuntime { return w.units }
func (w *MockWorld) AlliesOf(u *UnitRuntime) []*UnitRuntime {
	var allies []*UnitRuntime
	for _, unit := range w.units {
		if unit.TeamID == u.TeamID && unit.ID != u.ID {
			allies = append(allies, unit)
		}
	}
	return allies
}
func (w *MockWorld) EnemiesOf(u *UnitRuntime) []*UnitRuntime {
	var enemies []*UnitRuntime
	for _, unit := range w.units {
		if unit.TeamID != u.TeamID {
			enemies = append(enemies, unit)
		}
	}
	return enemies
}
func (w *MockWorld) Distance(a, b Vec2) float64 {
	w.distanceCalls = append(w.distanceCalls, struct{ a, b Vec2 }{a, b})
	return distance(a, b)
}
func (w *MockWorld) ApplyTimedSlow(tag string, target *UnitRuntime, slowPct float64, dur time.Duration) {
	w.slowCalls = append(w.slowCalls, struct {
		tag     string
		slowPct float64
		dur     time.Duration
	}{tag, slowPct, dur})
}
func (w *MockWorld) ApplyOrUpdateSlow(tag string, target *UnitRuntime, slowPct float64) {
	w.slowCalls = append(w.slowCalls, struct {
		tag     string
		slowPct float64
		dur     time.Duration
	}{tag, slowPct, 0})
}
func (w *MockWorld) ApplyOrUpdateBuff(tag string, target *UnitRuntime, fields map[string]float64) {
	w.buffCalls = append(w.buffCalls, struct {
		tag    string
		fields map[string]float64
	}{tag, fields})
}
func (w *MockWorld) SameTeam(a, b *UnitRuntime) bool { return a.TeamID == b.TeamID }

func TestTickPerkAuras_GlaciaChillAura(t *testing.T) {
	w := &MockWorld{}
	w.units = []*UnitRuntime{
		{ID: 1, TeamID: "blue", Pos: Vec2{X: 100, Y: 100}, Alive: true, ActivePerk: &Perk{
			Effects: PerkEffects{Type: "aura_slow", Radius: 50, SlowPct: 0.20},
		}},
		{ID: 2, TeamID: "red", Pos: Vec2{X: 130, Y: 100}, Alive: true}, // 30 units away, within radius
		{ID: 3, TeamID: "red", Pos: Vec2{X: 180, Y: 100}, Alive: true}, // 80 units away, outside radius
	}

	TickPerkAuras(w, time.Millisecond*200)
	assert.Len(t, w.slowCalls, 1, "Should apply slow to enemy within radius")
	assert.Equal(t, "perk_aura_slow", w.slowCalls[0].tag)
	assert.Equal(t, 0.20, w.slowCalls[0].slowPct)
}

func TestTickPerkAuras_SwordsmanInspireAlly(t *testing.T) {
	w := &MockWorld{}
	w.units = []*UnitRuntime{
		{ID: 1, TeamID: "blue", Pos: Vec2{X: 100, Y: 100}, Alive: true, ActivePerk: &Perk{
			Effects: PerkEffects{Type: "aura_ally_dmg", Radius: 60, AllyDmgPct: 0.10},
		}},
		{ID: 2, TeamID: "blue", Pos: Vec2{X: 120, Y: 100}, Alive: true}, // 20 units away, within radius
		{ID: 3, TeamID: "red", Pos: Vec2{X: 100, Y: 180}, Alive: true},  // enemy, should not be buffed
	}

	TickPerkAuras(w, time.Millisecond*200)
	assert.Len(t, w.buffCalls, 1, "Should buff ally within radius")
	assert.Equal(t, "perk_ally_dmg", w.buffCalls[0].tag)
	assert.Equal(t, 1.10, w.buffCalls[0].fields["atk_mult"])
}

func TestOnAttackDamageMultiplier_GlaciaIceSpike(t *testing.T) {
	attacker := &UnitRuntime{
		ActivePerk: &Perk{
			Effects: PerkEffects{Type: "nth_attack_bonus", Nth: 4, BonusDmgPct: 0.40},
		},
		AttackMultiplier: 1.0,
		PerkState: struct {
			AttackCount int
		}{AttackCount: 3}, // 4th attack should trigger bonus
	}
	target := &UnitRuntime{}

	mult := OnAttackDamageMultiplier(attacker, target)
	assert.Equal(t, 1.40, mult, "4th attack should have 40% bonus")
	assert.Equal(t, 0, attacker.PerkState.AttackCount, "Attack count should reset after 4th attack")
}

func TestOnDamageTakenMultiplier_SwordsmanShieldWall(t *testing.T) {
	w := &MockWorld{
		units: []*UnitRuntime{
			{ID: 1, TeamID: "blue", Pos: Vec2{X: 100, Y: 100}},
			{ID: 2, TeamID: "blue", Pos: Vec2{X: 120, Y: 100}}, // 20 units away, within 140 radius
		},
	}

	target := &UnitRuntime{
		TeamID:                "blue",
		Pos:                   Vec2{X: 100, Y: 100},
		DamageTakenMultiplier: 1.0,
		ActivePerk: &Perk{
			Effects: PerkEffects{Type: "conditional_dr_near_ally", DrPct: 0.15, AllyRadius: 140},
		},
	}
	attacker := &UnitRuntime{TeamID: "red"}

	mult := OnDamageTakenMultiplier(target, attacker)
	assert.Equal(t, 0.85, mult, "Should grant 15% damage reduction with ally nearby")
}

func TestOnDamageTakenMultiplier_SwordsmanLastStand(t *testing.T) {
	target := &UnitRuntime{
		HP:                    25,
		MaxHP:                 100, // 25% HP
		DamageTakenMultiplier: 1.0,
		ActivePerk: &Perk{
			Effects: PerkEffects{Type: "low_hp_armor", ArmorBonusPct: 0.25, HpThresholdPct: 0.30},
		},
	}
	attacker := &UnitRuntime{}

	mult := OnDamageTakenMultiplier(target, attacker)
	assert.Equal(t, 0.75, mult, "Should grant 25% damage reduction below 30% HP")
}

func TestOnUnitDeath_GlaciaFrozenVeil(t *testing.T) {
	w := &MockWorld{}
	u := &UnitRuntime{
		Pos: Vec2{X: 100, Y: 100},
		ActivePerk: &Perk{
			Effects: PerkEffects{Type: "ondeath_aoe_slow", Radius: 80, SlowPct: 0.50, DurationMs: 2000},
		},
	}

	OnUnitDeath(u, w)
	assert.Len(t, w.slowCalls, 1, "Should apply AOE slow on death")
	assert.Equal(t, "perk_frozen_veil", w.slowCalls[0].tag)
	assert.Equal(t, 0.50, w.slowCalls[0].slowPct)
	assert.Equal(t, time.Duration(2000)*time.Millisecond, w.slowCalls[0].dur)
}

func TestLoadUnitPerk_Glacia(t *testing.T) {
	perk := LoadUnitPerk("Sorceress Glacia", "glacia_chill_aura")
	assert.NotNil(t, perk)
	assert.Equal(t, "glacia_chill_aura", perk.ID)
	assert.Equal(t, "Chill Aura", perk.Name)
	assert.Equal(t, "aura_slow", perk.Effects.Type)
	assert.Equal(t, 160.0, perk.Effects.Radius)
	assert.Equal(t, 0.10, perk.Effects.SlowPct)
}

func TestLoadUnitPerk_Swordsman(t *testing.T) {
	perk := LoadUnitPerk("Swordsman", "sword_shield_wall")
	assert.NotNil(t, perk)
	assert.Equal(t, "sword_shield_wall", perk.ID)
	assert.Equal(t, "Shield Wall", perk.Name)
	assert.Equal(t, "conditional_dr_near_ally", perk.Effects.Type)
	assert.Equal(t, 0.15, perk.Effects.DrPct)
	assert.Equal(t, 140.0, perk.Effects.AllyRadius)
}

func TestLoadUnitPerk_Invalid(t *testing.T) {
	perk := LoadUnitPerk("Invalid Unit", "invalid_perk")
	assert.Nil(t, perk)
}
