package game

import (
	"time"

	"rumble/shared/protocol"
)

type RenderUnit struct {
	ID                 int64
	Name               string
	X, Y, PrevX, PrevY float64
	TargetX, TargetY   float64
	HP, MaxHP          int
	Facing             float64
	OwnerID            int64
	Class              string
	Range              int
	Particle           string
}

type World struct {
	Units      map[int64]*RenderUnit
	Bases      map[int64]protocol.BaseState
	lastUpdate time.Time
}

func buildWorldFromSnapshot(s protocol.FullSnapshot) *World {
	w := &World{Units: make(map[int64]*RenderUnit), Bases: make(map[int64]protocol.BaseState)}
	for _, u := range s.Units {
		w.Units[u.ID] = &RenderUnit{
			ID: u.ID, Name: u.Name,
			X: float64(u.X), Y: float64(u.Y),
			PrevX: float64(u.X), PrevY: float64(u.Y),
			TargetX: float64(u.X), TargetY: float64(u.Y),
			HP: u.HP, MaxHP: u.MaxHP, OwnerID: u.OwnerID, Class: u.Class, Range: u.Range, Particle: u.Particle,
		}
	}
	for _, b := range s.Bases {
		w.Bases[int64(b.OwnerID)] = b
	}
	w.lastUpdate = time.Now()
	return w
}

func (w *World) ApplyDelta(d protocol.StateDelta) {
	for _, u := range d.UnitsUpsert {
		ru := w.Units[u.ID]
		if ru == nil {
			// New unit: initialize at server position, no interpolation jump
			ru = &RenderUnit{ID: u.ID, Name: u.Name}
			ru.X, ru.Y = float64(u.X), float64(u.Y)
			ru.PrevX, ru.PrevY = ru.X, ru.Y
			ru.TargetX, ru.TargetY = ru.X, ru.Y
			w.Units[u.ID] = ru
		}
		// Smoothly interpolate toward new server position
		ru.PrevX, ru.PrevY = ru.X, ru.Y
		ru.TargetX, ru.TargetY = float64(u.X), float64(u.Y)
		ru.HP, ru.MaxHP = u.HP, u.MaxHP
		ru.OwnerID, ru.Class = u.OwnerID, u.Class
		ru.Range = u.Range
		ru.Particle = u.Particle
	}
	for _, id := range d.UnitsRemoved {
		delete(w.Units, id)
	}
	if len(d.Bases) > 0 {
		for _, b := range d.Bases {
			w.Bases[int64(b.OwnerID)] = b
		}
	}
	w.lastUpdate = time.Now()
}

func (w *World) LerpPositions() {
	if w.lastUpdate.IsZero() {
		return
	}
	alpha := time.Since(w.lastUpdate).Seconds() * 10.0
	if alpha > 1 {
		alpha = 1
	}
	for _, u := range w.Units {
		u.X = u.PrevX + (u.TargetX-u.PrevX)*alpha
		u.Y = u.PrevY + (u.TargetY-u.PrevY)*alpha
	}
}
