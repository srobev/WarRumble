package game

import (
	"math"
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
	AnimationData      *UnitAnimationData // Animation system data
}

type RenderProjectile struct {
	ID             int64
	X, Y           float64 // Current position
	TX, TY         float64 // Target position
	Damage         int     // Damage to deal
	OwnerID        int64   // Who fired this projectile
	TargetID       int64   // Target unit ID (0 if targeting base)
	ProjectileType string  // Type for visual effects
	Active         bool    // Whether projectile is still active
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
	Projectiles     map[int64]*RenderProjectile
	Bases           map[int64]protocol.BaseState
	lastUpdate      time.Time
	Obstacles       []protocol.Obstacle // Current map obstacles
	Lanes           []protocol.Lane     // Current map lanes
	SpawnAnimations []*SpawnAnimation   // Active spawn animations
}

func buildWorldFromSnapshot(s protocol.FullSnapshot, currentMapDef *protocol.MapDef) *World {
	w := &World{
		Units:       make(map[int64]*RenderUnit),
		Projectiles: make(map[int64]*RenderProjectile),
		Bases:       make(map[int64]protocol.BaseState),
	}
	for _, u := range s.Units {
		w.Units[u.ID] = &RenderUnit{
			ID: u.ID, Name: u.Name,
			X: float64(u.X), Y: float64(u.Y),
			PrevX: float64(u.X), PrevY: float64(u.Y),
			TargetX: float64(u.X), TargetY: float64(u.Y),
			HP: u.HP, MaxHP: u.MaxHP, OwnerID: u.OwnerID, Class: u.Class, Range: u.Range, Particle: u.Particle,
			AnimationData: NewUnitAnimationData(), // Initialize animation system
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
			ru = &RenderUnit{
				ID: u.ID, Name: u.Name,
				AnimationData: NewUnitAnimationData(), // Initialize animation system
			}
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

	// Handle projectiles from server
	if len(d.Projectiles) > 0 {
		// Clear existing projectiles and replace with server state
		w.Projectiles = make(map[int64]*RenderProjectile)
		for _, p := range d.Projectiles {
			if p.Active {
				w.Projectiles[p.ID] = &RenderProjectile{
					ID:             p.ID,
					X:              p.X,
					Y:              p.Y,
					TX:             p.TX,
					TY:             p.TY,
					Damage:         p.Damage,
					OwnerID:        p.OwnerID,
					TargetID:       p.TargetID,
					ProjectileType: p.ProjectileType,
					Active:         p.Active,
				}
			}
		}
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
		// Calculate new position with smooth interpolation
		newX := u.PrevX + (u.TargetX-u.PrevX)*alpha
		newY := u.PrevY + (u.TargetY-u.PrevY)*alpha

		// Apply collision avoidance (only for units vs bases)
		// Removed real-time obstacle avoidance to prevent jerky movement
		newX, newY = w.applyBaseCollisionAvoidance(u.ID, newX, newY)

		u.X = newX
		u.Y = newY
	}

	// Update projectile positions
	w.updateProjectiles()
}

// updateProjectiles moves projectiles toward their targets and handles collisions
func (w *World) updateProjectiles() {
	const projectileSpeed = 400.0 // pixels per second

	for id, projectile := range w.Projectiles {
		if !projectile.Active {
			continue
		}

		// Calculate direction to target
		dx := projectile.TX - projectile.X
		dy := projectile.TY - projectile.Y
		dist := math.Sqrt(dx*dx + dy*dy)

		if dist < 5 { // Close enough to target
			// Projectile hit target - server will handle damage via delta updates
			// Client only handles visual effects, not HP modifications

			// Remove projectile
			delete(w.Projectiles, id)
			continue
		}

		// Move projectile toward target
		if dist > 0 {
			// Normalize direction
			dx /= dist
			dy /= dist

			// Move projectile
			projectile.X += dx * projectileSpeed * 0.016 // Approximate 60 FPS
			projectile.Y += dy * projectileSpeed * 0.016
		}
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

	// Path is blocked, use smart obstacle avoidance
	return w.FindSmartObstaclePath(startX, startY, targetX, targetY)
}

// FindSmartObstaclePath implements intelligent pathfinding around obstacles
// considering the direction toward the target to choose the optimal side
func (w *World) FindSmartObstaclePath(startX, startY, targetX, targetY float64) (float64, float64) {
	// Calculate the main direction vector from start to target
	dx := targetX - startX
	dy := targetY - startY
	distToTarget := math.Sqrt(dx*dx + dy*dy)

	if distToTarget == 0 {
		return targetX, targetY // Already at target
	}

	// Normalize the direction vector
	dx /= distToTarget
	dy /= distToTarget

	// Find obstacles that might block the direct path
	blockingObstacles := w.findBlockingObstacles(startX, startY, targetX, targetY)

	if len(blockingObstacles) == 0 {
		// No blocking obstacles, return direct path
		return targetX, targetY
	}

	// For each blocking obstacle, evaluate both sides and choose the best
	var bestPathX, bestPathY float64
	bestScore := float64(-999999)

	for _, obstacle := range blockingObstacles {
		// Get obstacle center in screen coordinates
		obsCenterX := obstacle.X*float64(protocol.ScreenW) + (obstacle.Width*float64(protocol.ScreenW))/2
		obsCenterY := obstacle.Y*float64(protocol.ScreenH) + (obstacle.Height*float64(protocol.ScreenH))/2

		// Calculate vectors from start to obstacle center
		obsDx := obsCenterX - startX
		obsDy := obsCenterY - startY
		distToObstacle := math.Sqrt(obsDx*obsDx + obsDy*obsDy)

		if distToObstacle == 0 {
			continue // Skip if exactly at obstacle center
		}

		// Normalize obstacle direction
		obsDx /= distToObstacle
		obsDy /= distToObstacle

		// Calculate perpendicular vector (90 degrees rotation)
		perpDx := -obsDy
		perpDy := obsDx

		// Calculate detour points on both sides of the obstacle
		// Use obstacle radius plus some buffer
		obstacleRadius := math.Max(obstacle.Width*float64(protocol.ScreenW), obstacle.Height*float64(protocol.ScreenH)) / 2
		detourDistance := obstacleRadius + 30 // 30 pixel buffer

		// Left side detour (relative to movement direction)
		leftDetourX := obsCenterX + perpDx*detourDistance
		leftDetourY := obsCenterY + perpDy*detourDistance

		// Right side detour (opposite side)
		rightDetourX := obsCenterX - perpDx*detourDistance
		rightDetourY := obsCenterY - perpDy*detourDistance

		// Evaluate both detour options
		for _, detourPoint := range []struct{ x, y float64 }{
			{leftDetourX, leftDetourY},
			{rightDetourX, rightDetourY},
		} {
			// Check if detour point is valid (not in another obstacle)
			if w.IsPointInObstacle(detourPoint.x, detourPoint.y) {
				continue
			}

			// Calculate path: start -> detour -> target
			pathDist := w.calculatePathDistance(startX, startY, detourPoint.x, detourPoint.y, targetX, targetY)

			// Calculate how well this path aligns with target direction
			detourDx := detourPoint.x - startX
			detourDy := detourPoint.y - startY
			detourDist := math.Sqrt(detourDx*detourDx + detourDy*detourDy)

			if detourDist > 0 {
				detourDx /= detourDist
				detourDy /= detourDist

				// Dot product measures alignment with target direction
				alignment := detourDx*dx + detourDy*dy

				// Score combines path efficiency and directional alignment
				score := alignment*100 - pathDist*0.1

				if score > bestScore {
					bestScore = score
					bestPathX = detourPoint.x
					bestPathY = detourPoint.y
				}
			}
		}
	}

	// If we found a good detour path, return it
	if bestScore > -999999 {
		return bestPathX, bestPathY
	}

	// Fallback to closest lane point if smart pathfinding fails
	return w.FindClosestLanePoint(targetX, targetY)
}

// findBlockingObstacles finds obstacles that block the direct path from start to target
func (w *World) findBlockingObstacles(startX, startY, targetX, targetY float64) []protocol.Obstacle {
	var blocking []protocol.Obstacle

	// Calculate line segment from start to target
	dx := targetX - startX
	dy := targetY - startY
	dist := math.Sqrt(dx*dx + dy*dy)

	if dist == 0 {
		return blocking
	}

	// Check points along the path
	steps := int(dist / 10) // Check every 10 pixels
	if steps < 3 {
		steps = 3
	}

	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		checkX := startX + dx*t
		checkY := startY + dy*t

		// Check if this point is inside any obstacle
		for _, obstacle := range w.Obstacles {
			obsX := obstacle.X * float64(protocol.ScreenW)
			obsY := obstacle.Y * float64(protocol.ScreenH)
			obsW := obstacle.Width * float64(protocol.ScreenW)
			obsH := obstacle.Height * float64(protocol.ScreenH)

			if checkX >= obsX && checkX <= obsX+obsW &&
				checkY >= obsY && checkY <= obsY+obsH {
				// Check if this obstacle is already in our list
				alreadyAdded := false
				for _, existing := range blocking {
					if existing.X == obstacle.X && existing.Y == obstacle.Y {
						alreadyAdded = true
						break
					}
				}
				if !alreadyAdded {
					blocking = append(blocking, obstacle)
				}
			}
		}
	}

	return blocking
}

// calculatePathDistance calculates the total distance of a path: start -> waypoint -> target
func (w *World) calculatePathDistance(startX, startY, waypointX, waypointY, targetX, targetY float64) float64 {
	dist1 := math.Sqrt((waypointX-startX)*(waypointX-startX) + (waypointY-startY)*(waypointY-startY))
	dist2 := math.Sqrt((targetX-waypointX)*(targetX-waypointX) + (targetY-waypointY)*(targetY-waypointY))
	return dist1 + dist2
}

// applySmartObstacleAvoidance applies real-time obstacle avoidance during unit movement
func (w *World) applySmartObstacleAvoidance(unit *RenderUnit, newX, newY float64) (float64, float64) {
	// Check if the new position would collide with an obstacle
	if !w.IsPointInObstacle(newX, newY) {
		return newX, newY // No collision, return original position
	}

	// Calculate the unit's movement direction toward its target
	targetDx := unit.TargetX - unit.X
	targetDy := unit.TargetY - unit.Y
	distToTarget := math.Sqrt(targetDx*targetDx + targetDy*targetDy)

	if distToTarget == 0 {
		return newX, newY // No movement direction, can't determine which way to steer
	}

	// Normalize target direction
	targetDx /= distToTarget
	targetDy /= distToTarget

	// Find the closest obstacle that's blocking the path
	var closestObstacle *protocol.Obstacle
	minDist := float64(999999)

	for _, obstacle := range w.Obstacles {
		obsX := obstacle.X * float64(protocol.ScreenW)
		obsY := obstacle.Y * float64(protocol.ScreenH)
		obsW := obstacle.Width * float64(protocol.ScreenW)
		obsH := obstacle.Height * float64(protocol.ScreenH)

		// Check if new position is inside this obstacle
		if newX >= obsX && newX <= obsX+obsW && newY >= obsY && newY <= obsY+obsH {
			// Calculate distance from unit's current position to obstacle center
			obsCenterX := obsX + obsW/2
			obsCenterY := obsY + obsH/2
			dist := math.Sqrt((obsCenterX-unit.X)*(obsCenterX-unit.X) + (obsCenterY-unit.Y)*(obsCenterY-unit.Y))

			if dist < minDist {
				minDist = dist
				obs := obstacle // Create a copy to avoid reference issues
				closestObstacle = &obs
			}
		}
	}

	if closestObstacle == nil {
		return newX, newY // No blocking obstacle found
	}

	// Calculate obstacle center
	obsCenterX := closestObstacle.X*float64(protocol.ScreenW) + (closestObstacle.Width*float64(protocol.ScreenW))/2
	obsCenterY := closestObstacle.Y*float64(protocol.ScreenH) + (closestObstacle.Height*float64(protocol.ScreenH))/2

	// Calculate vector from unit to obstacle center
	obsDx := obsCenterX - unit.X
	obsDy := obsCenterY - unit.Y
	distToObsCenter := math.Sqrt(obsDx*obsDx + obsDy*obsDy)

	if distToObsCenter == 0 {
		return newX, newY // Exactly at obstacle center, can't determine direction
	}

	// Normalize obstacle direction
	obsDx /= distToObsCenter
	obsDy /= distToObsCenter

	// Calculate perpendicular vector (90 degrees rotation) for steering
	perpDx := -obsDy
	perpDy := obsDx

	// Determine which side to steer based on target direction
	// Dot product with perpendicular vector determines which side aligns better with target
	dotProduct := targetDx*perpDx + targetDy*perpDy

	// Choose steering direction: positive dot product means steer in perp direction,
	// negative means steer in opposite direction
	steerDx := perpDx
	steerDy := perpDy
	if dotProduct < 0 {
		steerDx = -perpDx
		steerDy = -perpDy
	}

	// Calculate steering force based on distance to obstacle
	obstacleRadius := math.Max(closestObstacle.Width*float64(protocol.ScreenW), closestObstacle.Height*float64(protocol.ScreenH)) / 2
	distToObstacle := distToObsCenter - obstacleRadius

	// Steering strength decreases as we get farther from obstacle
	steerStrength := 1.0
	if distToObstacle > 20 {
		steerStrength = math.Max(0, 1.0-(distToObstacle-20)/30) // Fade out steering over 30 pixels
	}

	// Apply steering to the new position
	steerAmount := 25.0 * steerStrength // Maximum steering distance per frame
	newX += steerDx * steerAmount
	newY += steerDy * steerAmount

	// Ensure the steered position doesn't put us in another obstacle
	if w.IsPointInObstacle(newX, newY) {
		// If steering put us in another obstacle, try the opposite direction
		newX = unit.X + (-steerDx)*steerAmount
		newY = unit.Y + (-steerDy)*steerAmount

		// If that also fails, don't steer at all
		if w.IsPointInObstacle(newX, newY) {
			return unit.X, unit.Y // Stay in place rather than move into obstacle
		}
	}

	return newX, newY
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

// applyBaseCollisionAvoidance prevents units from overlapping with bases (no unit-to-unit collision)
func (w *World) applyBaseCollisionAvoidance(unitID int64, newX, newY float64) (float64, float64) {
	unit := w.Units[unitID]
	if unit == nil {
		return newX, newY
	}

	const unitRadius = 21.0 // Half of unit size (42px / 2)
	const baseBuffer = 25.0 // Extra buffer around bases

	// Check collision with bases only (no unit-to-unit collision)
	for _, base := range w.Bases {
		// Use rectangular collision with actual base dimensions (96x96)
		baseLeft := float64(base.X) - baseBuffer
		baseRight := float64(base.X+base.W) + baseBuffer
		baseTop := float64(base.Y) - baseBuffer
		baseBottom := float64(base.Y+base.H) + baseBuffer

		// Find closest point on expanded base rectangle
		closestX := math.Max(baseLeft, math.Min(newX, baseRight))
		closestY := math.Max(baseTop, math.Min(newY, baseBottom))

		// Check if unit is inside the expanded rectangle
		if newX >= baseLeft && newX <= baseRight && newY >= baseTop && newY <= baseBottom {
			// Unit is inside expanded base area, push it out
			dx := newX - closestX
			dy := newY - closestY

			if dx != 0 || dy != 0 {
				dist := math.Sqrt(dx*dx + dy*dy)
				if dist > 0 {
					// Push unit away from closest point
					pushDist := unitRadius + baseBuffer
					newX = closestX + (dx/dist)*pushDist
					newY = closestY + (dy/dist)*pushDist
				}
			}
		}
	}

	return newX, newY
}
