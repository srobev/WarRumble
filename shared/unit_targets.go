package shared

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// TargetMapping represents a single mapping rule for unit targeting
type TargetMapping struct {
	AttackerClass    string `json:"attacker_class"`
	AttackerSubclass string `json:"attacker_subclass"`
	TargetType       string `json:"target_type"`
	TargetClass      string `json:"target_class"`
	TargetSubclass   string `json:"target_subclass"`
	CanAttack        bool   `json:"can_attack"`
}

// UnitTargetConfig holds the complete target mapping configuration
type UnitTargetConfig struct {
	Mappings []TargetMapping `json:"mappings"`
}

// UnitTargetValidator handles target validation logic
type UnitTargetValidator struct {
	config UnitTargetConfig
}

// NewUnitTargetValidator creates a new validator with loaded configuration
func NewUnitTargetValidator() (*UnitTargetValidator, error) {
	validator := &UnitTargetValidator{}

	// Try to load from server/data/unit_targets.json
	configPath := filepath.Join("server", "data", "unit_targets.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Fallback to current directory
		configPath = "unit_targets.json"
	}

	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open unit targets config: %v", err)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&validator.config); err != nil {
		return nil, fmt.Errorf("failed to decode unit targets config: %v", err)
	}

	return validator, nil
}

// CanAttackTarget checks if an attacker unit can attack a specific target
func (v *UnitTargetValidator) CanAttackTarget(attackerClass, attackerSubclass string, targetType string, targetClass string, targetSubclass string) bool {
	// Look for exact matches first
	for _, mapping := range v.config.Mappings {
		if mapping.AttackerClass == attackerClass &&
			mapping.AttackerSubclass == attackerSubclass &&
			mapping.TargetType == targetType &&
			mapping.TargetClass == targetClass &&
			mapping.TargetSubclass == targetSubclass {
			return mapping.CanAttack
		}
	}

	// Look for wildcard matches (empty subclass)
	for _, mapping := range v.config.Mappings {
		if mapping.AttackerClass == attackerClass &&
			mapping.AttackerSubclass == "" &&
			mapping.TargetType == targetType &&
			mapping.TargetClass == targetClass &&
			mapping.TargetSubclass == targetSubclass {
			return mapping.CanAttack
		}
	}

	// Look for wildcard target subclass matches
	for _, mapping := range v.config.Mappings {
		if mapping.AttackerClass == attackerClass &&
			mapping.AttackerSubclass == attackerSubclass &&
			mapping.TargetType == targetType &&
			mapping.TargetClass == targetClass &&
			mapping.TargetSubclass == "" {
			return mapping.CanAttack
		}
	}

	// Look for both wildcard matches
	for _, mapping := range v.config.Mappings {
		if mapping.AttackerClass == attackerClass &&
			mapping.AttackerSubclass == "" &&
			mapping.TargetType == targetType &&
			mapping.TargetClass == targetClass &&
			mapping.TargetSubclass == "" {
			return mapping.CanAttack
		}
	}

	// Handle air-* wildcard matching
	if targetSubclass != "" && strings.HasPrefix(targetSubclass, "air-") {
		for _, mapping := range v.config.Mappings {
			if mapping.AttackerClass == attackerClass &&
				mapping.AttackerSubclass == attackerSubclass &&
				mapping.TargetType == targetType &&
				mapping.TargetClass == targetClass &&
				mapping.TargetSubclass == "air-*" {
				return mapping.CanAttack
			}
		}
	}

	// Default fallback - allow basic attacks
	if targetType == "base" {
		return true // Bases can generally be attacked
	}

	// For unit vs unit, allow same class attacks by default
	if targetType == "unit" && attackerClass == targetClass {
		return true
	}

	return false
}

// GetValidTargets returns a list of valid target types for a given unit
func (v *UnitTargetValidator) GetValidTargets(attackerClass, attackerSubclass string) []string {
	var validTargets []string

	for _, mapping := range v.config.Mappings {
		if mapping.AttackerClass == attackerClass &&
			(mapping.AttackerSubclass == attackerSubclass || mapping.AttackerSubclass == "") &&
			mapping.CanAttack {
			targetDesc := mapping.TargetType
			if mapping.TargetClass != "" {
				targetDesc += ":" + mapping.TargetClass
			}
			if mapping.TargetSubclass != "" {
				targetDesc += ":" + mapping.TargetSubclass
			}
			validTargets = append(validTargets, targetDesc)
		}
	}

	return validTargets
}

// HasSiegeFeature checks if a unit has the siege feature
func HasSiegeFeature(features []string) bool {
	if features == nil {
		return false
	}
	for _, feature := range features {
		if feature == "siege" {
			return true
		}
	}
	return false
}

// CalculateSiegeDamage calculates damage with siege bonus if applicable
func CalculateSiegeDamage(baseDamage int, isTargetingBase bool, hasSiegeFeature bool) int {
	if isTargetingBase && hasSiegeFeature {
		return baseDamage * 2 // Double damage to bases
	}
	return baseDamage
}
