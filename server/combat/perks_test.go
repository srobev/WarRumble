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
	if len(w.slowCalls) != 1 {
		t.Errorf("Should apply slow to enemy within radius, got %d calls", len(w.slowCalls))
	}
	if w.slowCalls[0].tag != "perk_aura_slow" {
		t.Errorf("Expected tag 'perk_aura_slow', got %s", w.slowCalls[0].tag)
	}
	if w.slowCalls[0].slowPct != 0.20 {
		t.Errorf("Expected slowPct 0.20, got %f", w.slowCalls[0].slowPct)
	}
}

func TestTickPerkAuras_SwordsmanInspireAlly(t *testing.T) {
	w := &MockWorld{}
	w.units = []*UnitRuntime{
		{ID: 1, TeamID: "blue", Pos: Vec2{X: 100, Y: 100}, Alive: true, IsSquadLeader: true, ActivePerk: &Perk{
			Effects: PerkEffects{Type: "aura_ally_dmg", Radius: 60, AllyDmgPct: 0.10},
		}},
		{ID: 2, TeamID: "blue", Pos: Vec2{X: 120, Y: 100}, Alive: true}, // 20 units away, within radius
		{ID: 3, TeamID: "red", Pos: Vec2{X: 100, Y: 180}, Alive: true},  // enemy, should not be buffed
	}

	TickPerkAuras(w, time.Millisecond*200)
	if len(w.buffCalls) != 1 {
		t.Errorf("Should buff ally within radius, got %d calls", len(w.buffCalls))
	}
	if len(w.buffCalls) > 0 {
		if w.buffCalls[0].tag != "perk_ally_dmg" {
			t.Errorf("Expected tag 'perk_ally_dmg', got %s", w.buffCalls[0].tag)
		}
		if w.buffCalls[0].fields["atk_mult"] != 1.10 {
			t.Errorf("Expected atk_mult 1.10, got %f", w.buffCalls[0].fields["atk_mult"])
		}
	}
}

func TestOnAttackDamageMultiplier_GlaciaIceSpike(t *testing.T) {
	attacker := &UnitRuntime{
		ActivePerk: &Perk{
			Effects: PerkEffects{Type: "nth_attack_bonus", Nth: 4, BonusDmgPct: 0.40},
		},
		AttackMultiplier: 1.0,
		PerkState: struct {
			AttackCount  int
			LastTargetID int64
		}{AttackCount: 3}, // 4th attack should trigger bonus
	}
	target := &UnitRuntime{}

	mult := OnAttackDamageMultiplier(attacker, target)
	if mult != 1.40 {
		t.Errorf("4th attack should have 40%% bonus, got %f", mult)
	}
	if attacker.PerkState.AttackCount != 0 {
		t.Errorf("Attack count should reset after 4th attack, got %d", attacker.PerkState.AttackCount)
	}
}

func TestOnDamageTakenMultiplier_SwordsmanShieldWall(t *testing.T) {
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
	if mult != 0.85 {
		t.Errorf("Should grant 15%% damage reduction with ally nearby, got %f", mult)
	}
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
	if mult != 0.75 {
		t.Errorf("Should grant 25%% damage reduction below 30%% HP, got %f", mult)
	}
}

func TestOnUnitDeath_GlaciaFrozenVeil(t *testing.T) {
	w := &MockWorld{}
	// Add targets for the AOE effect to work with
	w.units = []*UnitRuntime{
		{ID: 2, TeamID: "blue", Pos: Vec2{X: 150, Y: 100}, Alive: true}, // within 80 radius
		{ID: 3, TeamID: "blue", Pos: Vec2{X: 200, Y: 100}, Alive: true}, // 100 units away, outside radius
	}
	u := &UnitRuntime{
		TeamID: "red", // opposite team
		Pos:    Vec2{X: 100, Y: 100},
		Alive:  false, // unit is dead
		ActivePerk: &Perk{
			Effects: PerkEffects{Type: "ondeath_aoe_slow", Radius: 80, SlowPct: 0.50, DurationMs: 2000},
		},
	}

	OnUnitDeath(u, w)
	if len(w.slowCalls) != 1 {
		t.Errorf("Should apply AOE slow on death, got %d calls", len(w.slowCalls))
	}
	if len(w.slowCalls) > 0 {
		if w.slowCalls[0].tag != "perk_frozen_veil" {
			t.Errorf("Expected tag 'perk_frozen_veil', got %s", w.slowCalls[0].tag)
		}
		if w.slowCalls[0].slowPct != 0.50 {
			t.Errorf("Expected slowPct 0.50, got %f", w.slowCalls[0].slowPct)
		}
		if w.slowCalls[0].dur != time.Duration(2000)*time.Millisecond {
			t.Errorf("Expected duration %v, got %v", time.Duration(2000)*time.Millisecond, w.slowCalls[0].dur)
		}
	}
}

func TestLoadUnitPerk_Glacia(t *testing.T) {
	perk := LoadUnitPerk("Sorceress Glacia", "glacia_chill_aura")
	if perk == nil {
		t.Error("Expected perk to be not nil")
		return
	}
	if perk.ID != "glacia_chill_aura" {
		t.Errorf("Expected ID 'glacia_chill_aura', got %s", perk.ID)
	}
	if perk.Name != "Chill Aura" {
		t.Errorf("Expected Name 'Chill Aura', got %s", perk.Name)
	}
	if perk.Effects.Type != "aura_slow" {
		t.Errorf("Expected Type 'aura_slow', got %s", perk.Effects.Type)
	}
	if perk.Effects.Radius != 160.0 {
		t.Errorf("Expected Radius 160.0, got %f", perk.Effects.Radius)
	}
	if perk.Effects.SlowPct != 0.10 {
		t.Errorf("Expected SlowPct 0.10, got %f", perk.Effects.SlowPct)
	}
}

func TestLoadUnitPerk_Swordsman(t *testing.T) {
	perk := LoadUnitPerk("Swordsman", "sword_shield_wall")
	if perk == nil {
		t.Error("Expected perk to be not nil")
		return
	}
	if perk.ID != "sword_shield_wall" {
		t.Errorf("Expected ID 'sword_shield_wall', got %s", perk.ID)
	}
	if perk.Name != "Shield Wall" {
		t.Errorf("Expected Name 'Shield Wall', got %s", perk.Name)
	}
	if perk.Effects.Type != "conditional_dr_near_ally" {
		t.Errorf("Expected Type 'conditional_dr_near_ally', got %s", perk.Effects.Type)
	}
	if perk.Effects.DrPct != 0.15 {
		t.Errorf("Expected DrPct 0.15, got %f", perk.Effects.DrPct)
	}
	if perk.Effects.AllyRadius != 140.0 {
		t.Errorf("Expected AllyRadius 140.0, got %f", perk.Effects.AllyRadius)
	}
}

func TestLoadUnitPerk_Invalid(t *testing.T) {
	perk := LoadUnitPerk("Invalid Unit", "invalid_perk")
	if perk != nil {
		t.Error("Expected perk to be nil for invalid unit/perk")
	}
}
