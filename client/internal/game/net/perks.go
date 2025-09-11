package game

import (
	"net/url"
)

// PerkView represents perk information on the client
type PerkView struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Desc         string `json:"desc"`
	Purchased    bool   `json:"purchased"`
	Locked       bool   `json:"locked"`
	Active       bool   `json:"active"`
	UnlockRarity string `json:"unlockRarity"`
}

// ActivatePerkRequest represents an activate perk request
type ActivatePerkRequest struct {
	Unit   string `json:"unit"`
	PerkID string `json:"perk_id"`
}

// ListUnitPerks retrieves perks available for a unit
func ListUnitPerks(unit string) ([]PerkView, error) {
	path := "/api/units/perks?unit=" + url.QueryEscape(unit)
	return GetJSON[[]PerkView](path)
}

// ActivatePerk activates a perk for a unit
func ActivatePerk(unit, perkID string) error {
	req := ActivatePerkRequest{Unit: unit, PerkID: perkID}
	_, err := PostJSON[ActivatePerkRequest, struct{}](req, "/api/units/perk/activate")
	return err
}
