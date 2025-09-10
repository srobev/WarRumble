package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

// PerkView represents the client-facing view of a perk
type PerkView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Desc         string `json:"desc"`
	Purchased    bool   `json:"purchased"`
	Locked       bool   `json:"locked"`
	Active       bool   `json:"active"`
	UnlockRarity string `json:"unlockRarity"`
}

// Response for listing perks
type PerksResponse struct {
	Perks []PerkView `json:"perks"`
}

// ActivatePerkRequest represents the request to activate a perk
type ActivatePerkRequest struct {
	Unit   string `json:"unit"`
	PerkID string `json:"perk_id"`
}

// HandleActivatePerk handles POST /api/units/perk/activate
func HandleActivatePerk(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	// Authenticate user
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		_ = strings.TrimPrefix(authHeader, "Bearer ")
	} else if tok := r.URL.Query().Get("token"); tok != "" {
		_ = tok
	} else {
		http.Error(w, "Authentication required", 401)
		return
	}

	// Parse request
	var req ActivatePerkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}

	// TODO: Validate user owns the unit and perk is purchased
	// TODO: Call progression service to set active perk

	log.Printf("User activation request: unit=%s, perk=%s", req.Unit, req.PerkID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
}

// HandleListPerks handles GET /api/units/perks?unit=UnitName
func HandleListPerks(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	// Authenticate user
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		_ = strings.TrimPrefix(authHeader, "Bearer ")
	} else if tok := r.URL.Query().Get("token"); tok != "" {
		_ = tok
	} else {
		http.Error(w, "Authentication required", 401)
		return
	}

	unitName := r.URL.Query().Get("unit")
	if unitName == "" {
		http.Error(w, "Unit parameter required", 400)
		return
	}

	// TODO: Validate user authentication using token
	log.Printf("User list perks request: unit=%s", unitName)

	// TODO: Query progression service for user's perk data for this unit
	// For now, return sample data

	perks := []PerkView{}

	// Generate sample perks based on unit type
	switch unitName {
	case "Sorceress Glacia":
		perks = []PerkView{
			{
				ID:           "glacia_chill_aura",
				Name:         "Chill Aura",
				Desc:         "Enemies in range move 10% slower.",
				Purchased:    true,
				Locked:       false,
				Active:       true,
				UnlockRarity: "Rare",
			},
			{
				ID:           "glacia_ice_spike",
				Name:         "Ice Spike",
				Desc:         "Every 4th attack deals +40% damage.",
				Purchased:    true,
				Locked:       false,
				Active:       false,
				UnlockRarity: "Rare",
			},
			{
				ID:           "glacia_frozen_veil",
				Name:         "Frozen Veil",
				Desc:         "On death, slows nearby enemies by 50% for 3s.",
				Purchased:    false,
				Locked:       false,
				Active:       false,
				UnlockRarity: "Epic",
			},
		}
	case "Swordsman":
		perks = []PerkView{
			{
				ID:           "sword_shield_wall",
				Name:         "Shield Wall",
				Desc:         "Takes 15% less damage while an ally is nearby.",
				Purchased:    false,
				Locked:       true,
				Active:       false,
				UnlockRarity: "Uncommon",
			},
			{
				ID:           "sword_last_stand",
				Name:         "Last Stand",
				Desc:         "Gain +25% Armor below 30% HP.",
				Purchased:    false,
				Locked:       false,
				Active:       false,
				UnlockRarity: "Rare",
			},
			{
				ID:           "sword_inspire",
				Name:         "Inspiring Presence",
				Desc:         "Allies in range deal +5% damage.",
				Purchased:    false,
				Locked:       false,
				Active:       false,
				UnlockRarity: "Epic",
			},
		}
	default:
		perks = []PerkView{} // No perks for this unit
	}

	response := PerksResponse{Perks: perks}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
