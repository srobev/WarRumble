package progression

import (
	"log"

	"rumble/shared/game/types"
	"rumble/shared/protocol"
)

type Endpoints struct {
	service *Service
}

func NewEndpoints(service *Service) *Endpoints {
	return &Endpoints{service: service}
}

// HandleUnitProgressUpdate endpoint
func (e *Endpoints) HandleUnitProgressUpdate(msg protocol.UnitProgressUpdate, broadcaster func(eventType string, event interface{})) error {
	log.Printf("Processing UnitProgressUpdate: unitID=%s, deltaShards=%d", msg.UnitID, msg.DeltaShards)

	err := e.service.HandleUnitProgressUpdate(msg.UnitID, msg.DeltaShards, broadcaster)
	if err != nil {
		log.Printf("Failed to handle unit progress update: %v", err)
		return err
	}

	return nil
}

// HandleSetActivePerk endpoint
func (e *Endpoints) HandleSetActivePerk(msg protocol.SetActivePerk, available []types.Perk, broadcaster func(eventType string, event interface{})) error {
	log.Printf("Processing SetActivePerk: unitID=%s, perkID=%s", msg.UnitID, msg.PerkID)

	// Convert protocol PerkID to types PerkID
	perkID := types.PerkID(msg.PerkID)
	err := e.service.HandleSetActivePerk(msg.UnitID, perkID, available, broadcaster)
	if err != nil {
		log.Printf("Failed to handle set active perk: %v", err)
		return err
	}

	return nil
}
