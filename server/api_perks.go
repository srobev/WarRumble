package main

import (
	"encoding/json"
	"net/http"
)

// activatePerkRequest represents the JSON request for activating a perk
type activatePerkRequest struct {
	Unit   string `json:"unit"`
	PerkID string `json:"perk_id"`
}

// perksResponse represents the JSON response for listing unit perks
type perksResponse struct {
	Perks []perkInfo `json:"perks"`
}

type perkInfo struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Desc         string `json:"desc"`
	Purchased    bool   `json:"purchased"`
	Locked       bool   `json:"locked"`
	Active       bool   `json:"active"`
	UnlockRarity string `json:"unlockRarity"`
}

// handleActivatePerk handles POST /api/units/perk/activate
func handleActivatePerk(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	var req activatePerkRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", 400)
		return
	}

	// TODO: authenticate user and find unit progress
	// For now, stub implementation

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"ok": true})
}

// handleListPerks handles GET /api/units/perks?unit=UnitName
func handleListPerks(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", 405)
		return
	}

	unitName := r.URL.Query().Get("unit")
	if unitName == "" {
		http.Error(w, "Unit parameter required", 400)
		return
	}

	// TODO: authenticate user

	// Get perks for unit
	perks := GetPerksForUnit(unitName)

	// Build response
	var response []perkInfo
	for _, perk := range perks {
		// For now, mark all as available
		info := perkInfo{
			ID:           string(perk.ID),
			Name:         perk.Name,
			Desc:         perk.Description,
			Purchased:    false, // TODO: check from account
			Locked:       false,
			Active:       false,  // TODO: check from account
			UnlockRarity: "Rare", // Default, TODO: calculate properly
		}
		response = append(response, info)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(perksResponse{Perks: response})
}
