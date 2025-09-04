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

type SpawnAnimation struct {
	UnitID           int64
	UnitName         string
	UnitClass        string
	UnitSubclass     string
	StartX, StartY   float64   // Starting position (above spawn point)
	TargetX, TargetY float64   // Final position (spawn point)
	StartScale       float64   // Starting scale (zoomed in)
	EndScale         float64   // Ending scale (normal size)
	CurrentScale     float64   // Current animation scale
	Progress         float64   // Animation progress (0-1)
	Duration         float64   // Total animation duration in seconds
	StartTime        time.Time // When animation started
	Active           bool      // Whether animation is still running
}

type World struct {
	Units           map[int64]*RenderUnit
	Bases           map[int64]protocol.BaseState
	lastUpdate      time.Time
	Obstacles       []protocol.Obstacle // Current map obstacles
	Lanes           []protocol.Lane     // Current map lanes
	SpawnAnimations []*SpawnAnimation   // Active spawn animations
}

func buildWorldFromSnapshot(s protocol.FullSnapshot, currentMapDef *protocol.MapDef) *World {
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

	// Populate obstacles and lanes if available
	if currentMapDef != nil {
		w.Obstacles = currentMapDef.Obstacles
		w.Lanes = currentMapDef.Lanes
	}

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

// Check if a point collides with any obstacle
func (w *World) IsPointInObstacle(x, y float64) bool {
	for _, obstacle := range w.Obstacles {
		// Convert normalized coordinates to screen coordinates for collision check
		obsX := obstacle.X * float64(protocol.ScreenW)
		obsY := obstacle.Y * float64(protocol.ScreenH)
		obsW := obstacle.Width * float64(protocol.ScreenW)
		obsH := obstacle.Height * float64(protocol.ScreenH)

		// Simple AABB collision detection
		if x >= obsX && x <= obsX+obsW && y >= obsY && y <= obsY+obsH {
			return true
		}
	}
	return false
}

// Find the closest point on any lane to the given position
func (w *World) FindClosestLanePoint(x, y float64) (float64, float64) {
	if len(w.Lanes) == 0 {
		return x, y // No lanes, return original position
	}

	var closestX, closestY float64
	minDist := float64(999999)

	for _, lane := range w.Lanes {
		for _, point := range lane.Points {
			// Convert normalized coordinates to screen coordinates
			px := point.X * float64(protocol.ScreenW)
			py := point.Y * float64(protocol.ScreenH)

			dist := (px-x)*(px-x) + (py-y)*(py-y)
			if dist < minDist {
				minDist = dist
				closestX, closestY = px, py
			}
		}
	}

	return closestX, closestY
}

// Find a path along lanes from start to target, avoiding obstacles
func (w *World) FindLanePath(startX, startY, targetX, targetY float64) (float64, float64) {
	// First, check if the direct path is clear
	if !w.IsPointInObstacle(targetX, targetY) {
		// Check a few points along the path
		steps := 10
		for i := 1; i < steps; i++ {
			t := float64(i) / float64(steps)
			checkX := startX + (targetX-startX)*t
			checkY := startY + (targetY-startY)*t
			if w.IsPointInObstacle(checkX, checkY) {
				break // Path is blocked, need to find alternative
			}
		}
		// If we get here, path is clear
		return targetX, targetY
	}

	// Path is blocked, find closest lane point to target
	return w.FindClosestLanePoint(targetX, targetY)
}

// Update unit target position to avoid obstacles using lane pathfinding
func (w *World) UpdateUnitTargetWithObstacleAvoidance(unitID int64, targetX, targetY float64) {
	unit := w.Units[unitID]
	if unit == nil {
		return
	}

	// Use lane-based pathfinding to avoid obstacles
	newTargetX, newTargetY := w.FindLanePath(unit.X, unit.Y, targetX, targetY)

	// Update unit's target position
	unit.TargetX = newTargetX
	unit.TargetY = newTargetY
}

// StartSpawnAnimation creates a new spawn animation for a unit
func (w *World) StartSpawnAnimation(unitID int64, unitName, unitClass, unitSubclass string, spawnX, spawnY float64) {
	animation := &SpawnAnimation{
		UnitID:       unitID,
		UnitName:     unitName,
		UnitClass:    unitClass,
		UnitSubclass: unitSubclass,
		StartX:       spawnX,
		StartY:       spawnY - 40, // Start 40 pixels above spawn point (half the height)
		TargetX:      spawnX,
		TargetY:      spawnY,
		StartScale:   1.4, // Start 1.4x zoomed in (half the zoom)
		EndScale:     1.0, // End at normal size
		CurrentScale: 1.4,
		Progress:     0.0,
		Duration:     0.4, // 0.4 second animation (half the duration)
		StartTime:    time.Now(),
		Active:       true,
	}

	w.SpawnAnimations = append(w.SpawnAnimations, animation)
}

// UpdateSpawnAnimations updates all active spawn animations
func (w *World) UpdateSpawnAnimations() {
	currentTime := time.Now()

	// Update all animations and remove completed ones
	for i := len(w.SpawnAnimations) - 1; i >= 0; i-- {
		animation := w.SpawnAnimations[i]

		if !animation.Active {
			// Remove completed animation
			w.SpawnAnimations = append(w.SpawnAnimations[:i], w.SpawnAnimations[i+1:]...)
			continue
		}

		// Calculate progress
		elapsed := currentTime.Sub(animation.StartTime).Seconds()
		animation.Progress = elapsed / animation.Duration

		if animation.Progress >= 1.0 {
			// Animation completed
			animation.Progress = 1.0
			animation.Active = false
			continue
		}

		// Update current position and scale using easing
		// Use ease-out cubic for smooth animation
		t := animation.Progress
		easedT := 1.0 - (1.0-t)*(1.0-t)*(1.0-t) // Cubic ease-out

		// Interpolate position
		animation.CurrentScale = animation.StartScale + (animation.EndScale-animation.StartScale)*easedT
	}
}

// GetActiveSpawnAnimation returns the active spawn animation for a unit ID, if any
func (w *World) GetActiveSpawnAnimation(unitID int64) *SpawnAnimation {
	for _, animation := range w.SpawnAnimations {
		if animation.UnitID == unitID && animation.Active {
			return animation
		}
	}
	return nil
}
