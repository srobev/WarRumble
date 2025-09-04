package game

import (
	"math"
	"time"
)

// UnitAbility represents a special ability that a unit can use
type UnitAbility struct {
	Name        string        // Ability name
	Description string        // Ability description
	Cooldown    time.Duration // Cooldown between uses
	LastUsed    time.Time     // When it was last used
	Damage      int           // Damage dealt (if any)
	Healing     int           // Healing amount (if any)
	Range       float64       // Ability range
	Duration    time.Duration // How long the effect lasts
	EffectType  string        // Type of visual effect
	TargetType  string        // "self", "enemy", "ally", "area"
	IsActive    bool          // Whether ability is currently active
}

// UnitAbilities manages all abilities for a unit
type UnitAbilities struct {
	Abilities    map[string]*UnitAbility
	UnitX, UnitY float64 // Unit position for effects
}

// NewUnitAbilities creates a new ability system for a unit
func NewUnitAbilities() *UnitAbilities {
	return &UnitAbilities{
		Abilities: make(map[string]*UnitAbility),
	}
}

// AddAbility adds a new ability to the unit
func (ua *UnitAbilities) AddAbility(name string, ability *UnitAbility) {
	ua.Abilities[name] = ability
}

// UseAbility attempts to use an ability
func (ua *UnitAbilities) UseAbility(name string, targetX, targetY float64, particleSystem *ParticleSystem) bool {
	ability, exists := ua.Abilities[name]
	if !exists {
		return false
	}

	// Check cooldown
	if time.Since(ability.LastUsed) < ability.Cooldown {
		return false // Still on cooldown
	}

	// Check range if applicable
	if ability.Range > 0 {
		distance := math.Hypot(targetX-ua.UnitX, targetY-ua.UnitY)
		if distance > ability.Range {
			return false // Out of range
		}
	}

	// Use the ability
	ability.LastUsed = time.Now()
	ability.IsActive = true

	// Trigger visual effects
	ua.triggerAbilityEffect(name, targetX, targetY, particleSystem)

	// Auto-deactivate after duration
	if ability.Duration > 0 {
		time.AfterFunc(ability.Duration, func() {
			ability.IsActive = false
		})
	}

	return true
}

// triggerAbilityEffect creates the appropriate visual effect for the ability
func (ua *UnitAbilities) triggerAbilityEffect(abilityName string, targetX, targetY float64, particleSystem *ParticleSystem) {
	if particleSystem == nil {
		return
	}

	switch abilityName {
	case "heal":
		// Green particles on healer and target (same as pressing H key)
		particleSystem.CreateHealingEffect(ua.UnitX, ua.UnitY)
		particleSystem.CreateHealingEffect(targetX, targetY)
	case "stun":
		particleSystem.CreateUnitAbilityEffect(targetX, targetY, "stun")
	case "shield":
		particleSystem.CreateUnitAbilityEffect(targetX, targetY, "shield")
	case "teleport":
		// Teleport effect at both start and end positions
		particleSystem.CreateUnitAbilityEffect(ua.UnitX, ua.UnitY, "teleport")
		particleSystem.CreateUnitAbilityEffect(targetX, targetY, "teleport")
	case "summon":
		particleSystem.CreateUnitAbilityEffect(targetX, targetY, "summon")
	case "rage":
		particleSystem.CreateUnitAbilityEffect(ua.UnitX, ua.UnitY, "rage")
	case "stealth":
		particleSystem.CreateUnitAbilityEffect(ua.UnitX, ua.UnitY, "stealth")
	case "poison":
		particleSystem.CreateUnitAbilityEffect(targetX, targetY, "poison")
	case "critical_strike":
		particleSystem.CreateCriticalHitEffect(targetX, targetY)
	case "level_up":
		particleSystem.CreateLevelUpEffect(ua.UnitX, ua.UnitY)
	}
}

// UpdatePosition updates the unit's position for ability targeting
func (ua *UnitAbilities) UpdatePosition(x, y float64) {
	ua.UnitX = x
	ua.UnitY = y
}

// GetAbility returns an ability by name
func (ua *UnitAbilities) GetAbility(name string) *UnitAbility {
	return ua.Abilities[name]
}

// IsAbilityReady checks if an ability is ready to use
func (ua *UnitAbilities) IsAbilityReady(name string) bool {
	ability, exists := ua.Abilities[name]
	if !exists {
		return false
	}

	return time.Since(ability.LastUsed) >= ability.Cooldown
}

// GetCooldownRemaining returns remaining cooldown time for an ability
func (ua *UnitAbilities) GetCooldownRemaining(name string) time.Duration {
	ability, exists := ua.Abilities[name]
	if !exists {
		return 0
	}

	elapsed := time.Since(ability.LastUsed)
	if elapsed >= ability.Cooldown {
		return 0
	}

	return ability.Cooldown - elapsed
}

// CreatePresetAbilities creates common ability presets
func CreatePresetAbilities() map[string]*UnitAbility {
	return map[string]*UnitAbility{
		"heal": {
			Name:        "Healing Wave",
			Description: "Heals nearby allies",
			Cooldown:    8 * time.Second,
			Healing:     200,
			Range:       100,
			Duration:    1 * time.Second,
			EffectType:  "heal",
			TargetType:  "area",
		},
		"stun": {
			Name:        "Stunning Strike",
			Description: "Stuns the target for 2 seconds",
			Cooldown:    12 * time.Second,
			Damage:      50,
			Range:       80,
			Duration:    2 * time.Second,
			EffectType:  "stun",
			TargetType:  "enemy",
		},
		"shield": {
			Name:        "Protective Shield",
			Description: "Creates a protective barrier",
			Cooldown:    15 * time.Second,
			Range:       0, // Self-target
			Duration:    5 * time.Second,
			EffectType:  "shield",
			TargetType:  "self",
		},
		"teleport": {
			Name:        "Blink",
			Description: "Teleport to target location",
			Cooldown:    10 * time.Second,
			Range:       150,
			Duration:    0,
			EffectType:  "teleport",
			TargetType:  "area",
		},
		"rage": {
			Name:        "Berserker Rage",
			Description: "Enter a rage state increasing damage",
			Cooldown:    20 * time.Second,
			Range:       0, // Self-target
			Duration:    8 * time.Second,
			EffectType:  "rage",
			TargetType:  "self",
		},
		"stealth": {
			Name:        "Shadow Step",
			Description: "Become temporarily invisible",
			Cooldown:    25 * time.Second,
			Range:       0, // Self-target
			Duration:    6 * time.Second,
			EffectType:  "stealth",
			TargetType:  "self",
		},
		"poison": {
			Name:        "Toxic Strike",
			Description: "Poison the target over time",
			Cooldown:    10 * time.Second,
			Damage:      30, // Initial damage
			Range:       70,
			Duration:    5 * time.Second,
			EffectType:  "poison",
			TargetType:  "enemy",
		},
	}
}

// CreateUnitAbilitySystem creates a complete ability system for a unit
func CreateUnitAbilitySystem(unitType string) *UnitAbilities {
	abilities := NewUnitAbilities()
	presets := CreatePresetAbilities()

	switch unitType {
	case "healer":
		abilities.AddAbility("heal", presets["heal"])
		abilities.AddAbility("shield", presets["shield"])
	case "warrior":
		abilities.AddAbility("rage", presets["rage"])
		abilities.AddAbility("stun", presets["stun"])
	case "assassin":
		abilities.AddAbility("stealth", presets["stealth"])
		abilities.AddAbility("teleport", presets["teleport"])
	case "mage":
		abilities.AddAbility("teleport", presets["teleport"])
		abilities.AddAbility("poison", presets["poison"])
	case "champion":
		// Champions get multiple abilities
		abilities.AddAbility("rage", presets["rage"])
		abilities.AddAbility("stun", presets["stun"])
		abilities.AddAbility("teleport", presets["teleport"])
	default:
		// Basic units get one ability
		abilities.AddAbility("stun", presets["stun"])
	}

	return abilities
}
