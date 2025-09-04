package game

import (
	"image/color"
	"math"
	"math/rand"
	"time"

	"strings"

	"rumble/shared/protocol"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Particle represents a single particle in the system
type Particle struct {
	X, Y       float64     // Position
	VX, VY     float64     // Velocity
	AX, AY     float64     // Acceleration
	Life       float64     // Current life (0-1)
	MaxLife    float64     // Maximum life
	Size       float64     // Current size
	StartSize  float64     // Initial size
	EndSize    float64     // Final size
	StartColor color.NRGBA // Initial color
	EndColor   color.NRGBA // Final color
	Rotation   float64     // Current rotation
	RotSpeed   float64     // Rotation speed
	Shape      string      // "circle", "square", "star"
	Active     bool        // Whether particle is active
}

// ParticleEmitter manages a collection of particles
type ParticleEmitter struct {
	Particles     []*Particle
	MaxParticles  int
	Active        bool
	X, Y          float64     // Emitter position
	Spread        float64     // Emission spread angle (radians)
	Speed         float64     // Initial particle speed
	SpeedVariance float64     // Speed variation
	Life          float64     // Particle lifetime
	LifeVariance  float64     // Lifetime variation
	Size          float64     // Particle size
	SizeVariance  float64     // Size variation
	StartColor    color.NRGBA // Initial color
	EndColor      color.NRGBA // Final color
	Shape         string      // Particle shape
	EmissionRate  float64     // Particles per second
	TimeSinceEmit float64     // Time accumulator for emission
	Duration      float64     // How long emitter should run (-1 for infinite)
	TimeAlive     float64     // How long emitter has been running
	Gravity       float64     // Gravity effect
	Drag          float64     // Air resistance
}

// ParticleSystem manages multiple emitters
type ParticleSystem struct {
	Emitters []*ParticleEmitter
}

// NewParticleSystem creates a new particle system
func NewParticleSystem() *ParticleSystem {
	return &ParticleSystem{
		Emitters: make([]*ParticleEmitter, 0),
	}
}

// NewParticleEmitter creates a new particle emitter
func NewParticleEmitter(x, y float64, maxParticles int) *ParticleEmitter {
	emitter := &ParticleEmitter{
		X:             x,
		Y:             y,
		MaxParticles:  maxParticles,
		Active:        true,
		Particles:     make([]*Particle, maxParticles),
		Spread:        2 * math.Pi, // Full circle
		Speed:         100,
		SpeedVariance: 20,
		Life:          1.0,
		LifeVariance:  0.2,
		Size:          4,
		SizeVariance:  1,
		StartColor:    color.NRGBA{200, 220, 255, 255},
		EndColor:      color.NRGBA{150, 180, 255, 0},
		Shape:         "circle",
		EmissionRate:  50,
		Duration:      -1, // Infinite
		Gravity:       0,
		Drag:          0.98,
	}

	// Initialize particles
	for i := 0; i < maxParticles; i++ {
		emitter.Particles[i] = &Particle{Active: false}
	}

	return emitter
}

// Update updates the particle system
func (ps *ParticleSystem) Update(deltaTime float64) {
	// Update all emitters
	for i := len(ps.Emitters) - 1; i >= 0; i-- {
		emitter := ps.Emitters[i]

		if !emitter.Active {
			// Remove inactive emitters
			ps.Emitters = append(ps.Emitters[:i], ps.Emitters[i+1:]...)
			continue
		}

		emitter.Update(deltaTime)
	}
}

// Draw renders all particles
func (ps *ParticleSystem) Draw(screen *ebiten.Image) {
	for _, emitter := range ps.Emitters {
		emitter.Draw(screen)
	}
}

// AddEmitter adds a new emitter to the system
func (ps *ParticleSystem) AddEmitter(emitter *ParticleEmitter) {
	ps.Emitters = append(ps.Emitters, emitter)
}

// Update updates the emitter and its particles
func (e *ParticleEmitter) Update(deltaTime float64) {
	e.TimeAlive += deltaTime

	// Check if emitter should stop
	if e.Duration > 0 && e.TimeAlive >= e.Duration {
		e.Active = false
		return
	}

	// Emit new particles
	e.TimeSinceEmit += deltaTime
	particlesToEmit := int(e.TimeSinceEmit * e.EmissionRate)
	if particlesToEmit > 0 {
		e.TimeSinceEmit -= float64(particlesToEmit) / e.EmissionRate
		for i := 0; i < particlesToEmit; i++ {
			e.emitParticle()
		}
	}

	// Update existing particles
	for _, particle := range e.Particles {
		if !particle.Active {
			continue
		}

		// Update position
		particle.VX += particle.AX * deltaTime
		particle.VY += particle.AY * deltaTime
		particle.X += particle.VX * deltaTime
		particle.Y += particle.VY * deltaTime

		// Apply gravity
		particle.VY += e.Gravity * deltaTime

		// Apply drag
		particle.VX *= e.Drag
		particle.VY *= e.Drag

		// Update rotation
		particle.Rotation += particle.RotSpeed * deltaTime

		// Update life and size
		particle.Life -= deltaTime / particle.MaxLife
		if particle.Life <= 0 {
			particle.Active = false
			continue
		}

		// Interpolate size
		t := 1.0 - particle.Life
		particle.Size = particle.StartSize + (particle.EndSize-particle.StartSize)*t
	}
}

// Draw renders the emitter's particles
func (e *ParticleEmitter) Draw(screen *ebiten.Image) {
	for _, particle := range e.Particles {
		if !particle.Active {
			continue
		}

		// Interpolate color
		t := 1.0 - particle.Life
		r := uint8(float64(e.StartColor.R) + float64(e.EndColor.R-e.StartColor.R)*t)
		g := uint8(float64(e.StartColor.G) + float64(e.EndColor.G-e.StartColor.G)*t)
		b := uint8(float64(e.StartColor.B) + float64(e.EndColor.B-e.StartColor.B)*t)
		a := uint8(float64(e.StartColor.A) + float64(e.EndColor.A-e.StartColor.A)*t)
		col := color.NRGBA{r, g, b, a}

		// Draw based on shape
		switch particle.Shape {
		case "circle":
			vector.DrawFilledCircle(screen, float32(particle.X), float32(particle.Y), float32(particle.Size), col, true)
		case "square":
			halfSize := particle.Size
			vector.DrawFilledRect(screen, float32(particle.X-halfSize), float32(particle.Y-halfSize), float32(particle.Size*2), float32(particle.Size*2), col, true)
		case "star":
			e.drawStar(screen, particle.X, particle.Y, particle.Size, particle.Rotation, col)
		}
	}
}

// emitParticle creates a new particle
func (e *ParticleEmitter) emitParticle() {
	// Find inactive particle
	var particle *Particle
	for _, p := range e.Particles {
		if !p.Active {
			particle = p
			break
		}
	}

	if particle == nil {
		return // No available particles
	}

	// Initialize particle
	particle.Active = true
	particle.X = e.X
	particle.Y = e.Y

	// Random direction within spread
	angle := rand.Float64()*e.Spread - e.Spread/2
	speed := e.Speed + rand.Float64()*e.SpeedVariance - e.SpeedVariance/2

	particle.VX = math.Cos(angle) * speed
	particle.VY = math.Sin(angle) * speed

	particle.AX = 0
	particle.AY = 0

	particle.Life = 1.0
	particle.MaxLife = e.Life + rand.Float64()*e.LifeVariance - e.LifeVariance/2

	particle.StartSize = e.Size + rand.Float64()*e.SizeVariance - e.SizeVariance/2
	particle.EndSize = particle.StartSize * 0.1
	particle.Size = particle.StartSize

	particle.StartColor = e.StartColor
	particle.EndColor = e.EndColor

	particle.Rotation = 0
	particle.RotSpeed = (rand.Float64() - 0.5) * 10

	particle.Shape = e.Shape
}

// drawStar draws a star shape
func (e *ParticleEmitter) drawStar(screen *ebiten.Image, x, y, size, rotation float64, col color.NRGBA) {
	points := 5
	outerRadius := size
	innerRadius := size * 0.4

	for i := 0; i < points*2; i++ {
		angle := (float64(i) * math.Pi / float64(points)) + rotation
		radius := outerRadius
		if i%2 == 1 {
			radius = innerRadius
		}

		px := x + math.Cos(angle)*radius
		py := y + math.Sin(angle)*radius

		if i == 0 {
			continue // Skip first point for line drawing
		}

		// Draw star points
		vector.DrawFilledCircle(screen, float32(px), float32(py), 1, col, true)
	}
}

// Stop deactivates the emitter
func (e *ParticleEmitter) Stop() {
	e.Active = false
}

// DrawMirrored renders the emitter's particles with Y-axis mirroring for PvP
func (e *ParticleEmitter) DrawMirrored(screen *ebiten.Image, mirrorY func(float64) float64) {
	for _, particle := range e.Particles {
		if !particle.Active {
			continue
		}

		// Apply mirroring to particle position
		mirroredX := particle.X
		mirroredY := mirrorY(particle.Y)

		// Interpolate color
		t := 1.0 - particle.Life
		r := uint8(float64(e.StartColor.R) + float64(e.EndColor.R-e.StartColor.R)*t)
		g := uint8(float64(e.StartColor.G) + float64(e.EndColor.G-e.StartColor.G)*t)
		b := uint8(float64(e.StartColor.B) + float64(e.EndColor.B-e.StartColor.B)*t)
		a := uint8(float64(e.StartColor.A) + float64(e.EndColor.A-e.StartColor.A)*t)
		col := color.NRGBA{r, g, b, a}

		// Draw based on shape with mirrored coordinates
		switch particle.Shape {
		case "circle":
			vector.DrawFilledCircle(screen, float32(mirroredX), float32(mirroredY), float32(particle.Size), col, true)
		case "square":
			halfSize := particle.Size
			vector.DrawFilledRect(screen, float32(mirroredX-halfSize), float32(mirroredY-halfSize), float32(particle.Size*2), float32(particle.Size*2), col, true)
		case "star":
			e.drawStarMirrored(screen, mirroredX, mirroredY, particle.Size, particle.Rotation, col, mirrorY)
		}
	}
}

// drawStarMirrored draws a star shape with Y-axis mirroring
func (e *ParticleEmitter) drawStarMirrored(screen *ebiten.Image, x, y, size, rotation float64, col color.NRGBA, mirrorY func(float64) float64) {
	points := 5
	outerRadius := size
	innerRadius := size * 0.4

	for i := 0; i < points*2; i++ {
		angle := (float64(i) * math.Pi / float64(points)) + rotation
		radius := outerRadius
		if i%2 == 1 {
			radius = innerRadius
		}

		px := x + math.Cos(angle)*radius
		py := mirrorY(y + math.Sin(angle)*radius)

		if i == 0 {
			continue // Skip first point for line drawing
		}

		// Draw star points with mirrored coordinates
		vector.DrawFilledCircle(screen, float32(px), float32(py), 1, col, true)
	}
}

// DrawWithCamera renders the emitter's particles with camera transformations
func (e *ParticleEmitter) DrawWithCamera(screen *ebiten.Image, cameraX, cameraY, cameraZoom float64) {
	for _, particle := range e.Particles {
		if !particle.Active {
			continue
		}

		// Apply camera transformations to particle position
		screenX := particle.X*cameraZoom + cameraX
		screenY := particle.Y*cameraZoom + cameraY

		// Interpolate color
		t := 1.0 - particle.Life
		r := uint8(float64(e.StartColor.R) + float64(e.EndColor.R-e.StartColor.R)*t)
		g := uint8(float64(e.StartColor.G) + float64(e.EndColor.G-e.StartColor.G)*t)
		b := uint8(float64(e.StartColor.B) + float64(e.EndColor.B-e.StartColor.B)*t)
		a := uint8(float64(e.StartColor.A) + float64(e.EndColor.A-e.StartColor.A)*t)
		col := color.NRGBA{r, g, b, a}

		// Draw based on shape with camera-transformed coordinates
		switch particle.Shape {
		case "circle":
			vector.DrawFilledCircle(screen, float32(screenX), float32(screenY), float32(particle.Size), col, true)
		case "square":
			halfSize := particle.Size
			vector.DrawFilledRect(screen, float32(screenX-halfSize), float32(screenY-halfSize), float32(particle.Size*2), float32(particle.Size*2), col, true)
		case "star":
			e.drawStar(screen, screenX, screenY, particle.Size, particle.Rotation, col)
		}
	}
}

// DrawMirroredWithCamera renders the emitter's particles with Y-axis mirroring and camera transformations
func (e *ParticleEmitter) DrawMirroredWithCamera(screen *ebiten.Image, mirrorY func(float64) float64, cameraX, cameraY, cameraZoom float64) {
	for _, particle := range e.Particles {
		if !particle.Active {
			continue
		}

		// Apply mirroring to particle position
		mirroredX := particle.X
		mirroredY := mirrorY(particle.Y)

		// Apply camera transformations to mirrored coordinates
		screenX := mirroredX*cameraZoom + cameraX
		screenY := mirroredY*cameraZoom + cameraY

		// Interpolate color
		t := 1.0 - particle.Life
		r := uint8(float64(e.StartColor.R) + float64(e.EndColor.R-e.StartColor.R)*t)
		g := uint8(float64(e.StartColor.G) + float64(e.EndColor.G-e.StartColor.G)*t)
		b := uint8(float64(e.StartColor.B) + float64(e.EndColor.B-e.StartColor.B)*t)
		a := uint8(float64(e.StartColor.A) + float64(e.EndColor.A-e.StartColor.A)*t)
		col := color.NRGBA{r, g, b, a}

		// Draw based on shape with mirrored and camera-transformed coordinates
		switch particle.Shape {
		case "circle":
			vector.DrawFilledCircle(screen, float32(screenX), float32(screenY), float32(particle.Size), col, true)
		case "square":
			halfSize := particle.Size
			vector.DrawFilledRect(screen, float32(screenX-halfSize), float32(screenY-halfSize), float32(particle.Size*2), float32(particle.Size*2), col, true)
		case "star":
			e.drawStarMirrored(screen, screenX, screenY, particle.Size, particle.Rotation, col, mirrorY)
		}
	}
}

// CreateExplosionEffect creates an explosion effect
func (ps *ParticleSystem) CreateExplosionEffect(x, y float64, intensity float64) {
	emitter := NewParticleEmitter(x, y, int(20*intensity))

	emitter.Spread = 2 * math.Pi
	emitter.Speed = 150 * intensity
	emitter.SpeedVariance = 50
	emitter.Life = 0.8
	emitter.LifeVariance = 0.3
	emitter.Size = 3 * intensity
	emitter.SizeVariance = 1
	emitter.StartColor = color.NRGBA{255, 200, 100, 255} // Orange
	emitter.EndColor = color.NRGBA{255, 50, 0, 0}        // Red to transparent
	emitter.Shape = "circle"
	emitter.EmissionRate = 200 * intensity
	emitter.Duration = 0.2
	emitter.Gravity = 200
	emitter.Drag = 0.95

	ps.AddEmitter(emitter)
}

// CreateSpellEffect creates a spell casting effect
func (ps *ParticleSystem) CreateSpellEffect(x, y float64, spellType string) {
	emitter := NewParticleEmitter(x, y, 30)

	switch spellType {
	case "fire":
		emitter.StartColor = color.NRGBA{255, 100, 0, 255} // Orange
		emitter.EndColor = color.NRGBA{255, 0, 0, 100}     // Red
		emitter.Shape = "circle"
	case "ice":
		emitter.StartColor = color.NRGBA{100, 200, 255, 255} // Light blue
		emitter.EndColor = color.NRGBA{0, 100, 255, 100}     // Blue
		emitter.Shape = "star"
	case "lightning":
		emitter.StartColor = color.NRGBA{255, 255, 100, 255} // Yellow
		emitter.EndColor = color.NRGBA{255, 255, 255, 100}   // White
		emitter.Shape = "star"
	default:
		emitter.StartColor = color.NRGBA{200, 200, 255, 255} // Light blue
		emitter.EndColor = color.NRGBA{100, 100, 255, 100}   // Blue
		emitter.Shape = "circle"
	}

	emitter.Spread = math.Pi / 2 // 90 degrees
	emitter.Speed = 80
	emitter.SpeedVariance = 20
	emitter.Life = 1.2
	emitter.LifeVariance = 0.4
	emitter.Size = 2
	emitter.SizeVariance = 0.5
	emitter.EmissionRate = 30
	emitter.Duration = 0.5
	emitter.Gravity = -50 // Float upward
	emitter.Drag = 0.97

	ps.AddEmitter(emitter)
}

// CreateProjectileTrail creates a trail effect for projectiles
func (ps *ParticleSystem) CreateProjectileTrail(x, y float64, targetX, targetY float64) {
	emitter := NewParticleEmitter(x, y, 15)

	emitter.StartColor = color.NRGBA{255, 255, 100, 200} // Yellow trail
	emitter.EndColor = color.NRGBA{255, 100, 0, 0}       // Orange to transparent
	emitter.Shape = "circle"
	emitter.Spread = math.Pi / 6 // Narrow spread
	emitter.Speed = 20
	emitter.SpeedVariance = 10
	emitter.Life = 0.3
	emitter.LifeVariance = 0.1
	emitter.Size = 1
	emitter.SizeVariance = 0.5
	emitter.EmissionRate = 100
	emitter.Duration = 0.1
	emitter.Gravity = 0
	emitter.Drag = 0.9

	// Point emitter toward target (narrow spread for trail effect)
	emitter.Spread = math.Pi / 8 // Very narrow
	emitter.Speed = 30

	ps.AddEmitter(emitter)
}

// CreateImpactEffect creates an impact effect when projectiles hit
func (ps *ParticleSystem) CreateImpactEffect(x, y float64, impactType string) {
	emitter := NewParticleEmitter(x, y, 25)

	switch impactType {
	case "fire":
		emitter.StartColor = color.NRGBA{255, 150, 0, 255} // Bright orange
		emitter.EndColor = color.NRGBA{255, 50, 0, 0}      // Red to transparent
	case "ice":
		emitter.StartColor = color.NRGBA{150, 200, 255, 255} // Light blue
		emitter.EndColor = color.NRGBA{50, 100, 255, 0}      // Blue to transparent
	default:
		emitter.StartColor = color.NRGBA{200, 200, 200, 255} // Gray
		emitter.EndColor = color.NRGBA{100, 100, 100, 0}     // Dark gray to transparent
	}

	emitter.Shape = "circle"
	emitter.Spread = 2 * math.Pi
	emitter.Speed = 120
	emitter.SpeedVariance = 40
	emitter.Life = 0.6
	emitter.LifeVariance = 0.2
	emitter.Size = 2
	emitter.SizeVariance = 1
	emitter.EmissionRate = 150
	emitter.Duration = 0.15
	emitter.Gravity = 150
	emitter.Drag = 0.92

	ps.AddEmitter(emitter)
}

// CreateHealingEffect creates a healing effect
func (ps *ParticleSystem) CreateHealingEffect(x, y float64) {
	emitter := NewParticleEmitter(x, y, 30)

	emitter.StartColor = color.NRGBA{0, 255, 0, 255} // Pure bright green
	emitter.EndColor = color.NRGBA{0, 255, 0, 0}     // Green to transparent
	emitter.Shape = "star"
	emitter.Spread = math.Pi / 2 // 90 degrees
	emitter.Speed = 80
	emitter.SpeedVariance = 20
	emitter.Life = 1.5
	emitter.LifeVariance = 0.4
	emitter.Size = 4
	emitter.SizeVariance = 1.5
	emitter.EmissionRate = 60
	emitter.Duration = 1.2
	emitter.Gravity = -60 // Float upward
	emitter.Drag = 0.95

	ps.AddEmitter(emitter)
}

// CreateTargetHealingEffect creates green particles on the healed target
func (ps *ParticleSystem) CreateTargetHealingEffect(x, y float64) {
	emitter := NewParticleEmitter(x, y, 25)

	emitter.StartColor = color.NRGBA{150, 255, 150, 220} // Bright healing green
	emitter.EndColor = color.NRGBA{100, 255, 100, 0}     // Green to transparent
	emitter.Shape = "circle"
	emitter.Spread = 2 * math.Pi // Full circle around target
	emitter.Speed = 40
	emitter.SpeedVariance = 10
	emitter.Life = 1.5
	emitter.LifeVariance = 0.4
	emitter.Size = 2.5
	emitter.SizeVariance = 0.8
	emitter.EmissionRate = 35
	emitter.Duration = 1.2
	emitter.Gravity = -30 // Gentle float upward
	emitter.Drag = 0.94

	ps.AddEmitter(emitter)
}

// CreateAuraEffect creates an aura effect around units
func (ps *ParticleSystem) CreateAuraEffect(x, y float64, auraType string) {
	emitter := NewParticleEmitter(x, y, 15)

	switch auraType {
	case "buff":
		emitter.StartColor = color.NRGBA{100, 255, 100, 150} // Bright green
		emitter.EndColor = color.NRGBA{50, 255, 50, 50}      // Green to transparent
	case "debuff":
		emitter.StartColor = color.NRGBA{255, 100, 100, 150} // Red glow
		emitter.EndColor = color.NRGBA{200, 50, 50, 50}      // Dark red glow
	default:
		emitter.StartColor = color.NRGBA{100, 100, 255, 150} // Blue glow
		emitter.EndColor = color.NRGBA{50, 50, 200, 50}      // Dark blue glow
	}

	emitter.Shape = "circle"
	emitter.Spread = 2 * math.Pi
	emitter.Speed = 30
	emitter.SpeedVariance = 10
	emitter.Life = 2.0
	emitter.LifeVariance = 0.5
	emitter.Size = 2
	emitter.SizeVariance = 0.5
	emitter.EmissionRate = 20
	emitter.Duration = 1.5 // Short duration for healing aura
	emitter.Gravity = 0
	emitter.Drag = 0.95

	ps.AddEmitter(emitter)
}

// CreateUnitAbilityEffect creates visual effects for unit special abilities
func (ps *ParticleSystem) CreateUnitAbilityEffect(x, y float64, abilityType string) {
	switch abilityType {
	case "heal":
		ps.createHealingWaveEffect(x, y)
	case "stun":
		ps.createStunEffect(x, y)
	case "shield":
		ps.createShieldEffect(x, y)
	case "teleport":
		ps.createTeleportEffect(x, y)
	case "summon":
		ps.createSummonEffect(x, y)
	case "rage":
		ps.createRageEffect(x, y)
	case "stealth":
		ps.createStealthEffect(x, y)
	case "poison":
		ps.createPoisonEffect(x, y)
	}
}

// createHealingWaveEffect creates a healing wave that expands outward
func (ps *ParticleSystem) createHealingWaveEffect(x, y float64) {
	emitter := NewParticleEmitter(x, y, 30)

	emitter.StartColor = color.NRGBA{100, 255, 150, 200} // Bright healing green
	emitter.EndColor = color.NRGBA{50, 255, 100, 0}      // Green to transparent
	emitter.Shape = "circle"
	emitter.Spread = 2 * math.Pi
	emitter.Speed = 80
	emitter.SpeedVariance = 20
	emitter.Life = 1.5
	emitter.LifeVariance = 0.3
	emitter.Size = 4
	emitter.SizeVariance = 1
	emitter.EmissionRate = 60
	emitter.Duration = 0.8
	emitter.Gravity = 0
	emitter.Drag = 0.9

	ps.AddEmitter(emitter)
}

// createStunEffect creates a stunning effect with stars and flashes
func (ps *ParticleSystem) createStunEffect(x, y float64) {
	emitter := NewParticleEmitter(x, y, 20)

	emitter.StartColor = color.NRGBA{255, 255, 100, 255} // Bright yellow
	emitter.EndColor = color.NRGBA{255, 200, 0, 0}       // Yellow to transparent
	emitter.Shape = "star"
	emitter.Spread = math.Pi / 2 // 90 degrees
	emitter.Speed = 60
	emitter.SpeedVariance = 15
	emitter.Life = 1.2
	emitter.LifeVariance = 0.4
	emitter.Size = 3
	emitter.SizeVariance = 1
	emitter.EmissionRate = 40
	emitter.Duration = 1.0
	emitter.Gravity = -30 // Float upward
	emitter.Drag = 0.95

	ps.AddEmitter(emitter)
}

// createShieldEffect creates a protective shield barrier
func (ps *ParticleSystem) createShieldEffect(x, y float64) {
	emitter := NewParticleEmitter(x, y, 25)

	emitter.StartColor = color.NRGBA{100, 150, 255, 180} // Light blue
	emitter.EndColor = color.NRGBA{50, 100, 255, 50}     // Blue to transparent
	emitter.Shape = "circle"
	emitter.Spread = 2 * math.Pi
	emitter.Speed = 40
	emitter.SpeedVariance = 10
	emitter.Life = 2.5
	emitter.LifeVariance = 0.5
	emitter.Size = 2
	emitter.SizeVariance = 0.5
	emitter.EmissionRate = 30
	emitter.Duration = -1 // Continuous shield
	emitter.Gravity = 0
	emitter.Drag = 0.97

	ps.AddEmitter(emitter)
}

// createTeleportEffect creates a magical teleportation effect
func (ps *ParticleSystem) createTeleportEffect(x, y float64) {
	emitter := NewParticleEmitter(x, y, 35)

	emitter.StartColor = color.NRGBA{150, 100, 255, 220} // Purple
	emitter.EndColor = color.NRGBA{100, 50, 255, 0}      // Purple to transparent
	emitter.Shape = "circle"
	emitter.Spread = 2 * math.Pi
	emitter.Speed = 100
	emitter.SpeedVariance = 30
	emitter.Life = 1.0
	emitter.LifeVariance = 0.2
	emitter.Size = 3
	emitter.SizeVariance = 1
	emitter.EmissionRate = 80
	emitter.Duration = 0.6
	emitter.Gravity = 0
	emitter.Drag = 0.9

	ps.AddEmitter(emitter)
}

// createSummonEffect creates a summoning ritual effect
func (ps *ParticleSystem) createSummonEffect(x, y float64) {
	emitter := NewParticleEmitter(x, y, 40)

	emitter.StartColor = color.NRGBA{200, 100, 255, 200} // Magenta
	emitter.EndColor = color.NRGBA{150, 50, 255, 0}      // Magenta to transparent
	emitter.Shape = "star"
	emitter.Spread = 2 * math.Pi
	emitter.Speed = 70
	emitter.SpeedVariance = 20
	emitter.Life = 2.0
	emitter.LifeVariance = 0.4
	emitter.Size = 4
	emitter.SizeVariance = 1.5
	emitter.EmissionRate = 50
	emitter.Duration = 1.5
	emitter.Gravity = -20 // Float upward
	emitter.Drag = 0.92

	ps.AddEmitter(emitter)
}

// createRageEffect creates a berserker rage effect
func (ps *ParticleSystem) createRageEffect(x, y float64) {
	emitter := NewParticleEmitter(x, y, 25)

	emitter.StartColor = color.NRGBA{255, 100, 100, 220} // Bright red
	emitter.EndColor = color.NRGBA{255, 50, 50, 0}       // Red to transparent
	emitter.Shape = "circle"
	emitter.Spread = math.Pi // 180 degrees forward
	emitter.Speed = 90
	emitter.SpeedVariance = 25
	emitter.Life = 1.8
	emitter.LifeVariance = 0.3
	emitter.Size = 3
	emitter.SizeVariance = 1
	emitter.EmissionRate = 45
	emitter.Duration = 2.0
	emitter.Gravity = 0
	emitter.Drag = 0.88

	ps.AddEmitter(emitter)
}

// createStealthEffect creates an invisibility/cloaking effect
func (ps *ParticleSystem) createStealthEffect(x, y float64) {
	emitter := NewParticleEmitter(x, y, 20)

	emitter.StartColor = color.NRGBA{150, 150, 200, 150} // Gray-blue
	emitter.EndColor = color.NRGBA{100, 100, 150, 0}     // Gray-blue to transparent
	emitter.Shape = "circle"
	emitter.Spread = 2 * math.Pi
	emitter.Speed = 25
	emitter.SpeedVariance = 8
	emitter.Life = 3.0
	emitter.LifeVariance = 0.8
	emitter.Size = 2
	emitter.SizeVariance = 0.5
	emitter.EmissionRate = 25
	emitter.Duration = -1 // Continuous stealth
	emitter.Gravity = 0
	emitter.Drag = 0.96

	ps.AddEmitter(emitter)
}

// createPoisonEffect creates a toxic/poison effect
func (ps *ParticleSystem) createPoisonEffect(x, y float64) {
	emitter := NewParticleEmitter(x, y, 22)

	emitter.StartColor = color.NRGBA{100, 255, 100, 180} // Bright green
	emitter.EndColor = color.NRGBA{50, 150, 50, 0}       // Green to transparent
	emitter.Shape = "circle"
	emitter.Spread = math.Pi / 3 // 60 degrees
	emitter.Speed = 50
	emitter.SpeedVariance = 15
	emitter.Life = 2.2
	emitter.LifeVariance = 0.5
	emitter.Size = 2.5
	emitter.SizeVariance = 0.8
	emitter.EmissionRate = 35
	emitter.Duration = 2.5
	emitter.Gravity = 20 // Sink down
	emitter.Drag = 0.93

	ps.AddEmitter(emitter)
}

// CreateBattleBuffEffect creates enhanced buff effects for battle
func (ps *ParticleSystem) CreateBattleBuffEffect(x, y float64, buffType string) {
	emitter := NewParticleEmitter(x, y, 18)

	switch buffType {
	case "attack":
		emitter.StartColor = color.NRGBA{255, 150, 150, 200} // Red-orange
		emitter.EndColor = color.NRGBA{255, 100, 100, 50}
	case "defense":
		emitter.StartColor = color.NRGBA{150, 150, 255, 200} // Blue
		emitter.EndColor = color.NRGBA{100, 100, 255, 50}
	case "speed":
		emitter.StartColor = color.NRGBA{255, 255, 150, 200} // Yellow
		emitter.EndColor = color.NRGBA{255, 255, 100, 50}
	case "health":
		emitter.StartColor = color.NRGBA{150, 255, 150, 200} // Green
		emitter.EndColor = color.NRGBA{100, 255, 100, 50}
	default:
		emitter.StartColor = color.NRGBA{200, 200, 255, 200} // Light blue
		emitter.EndColor = color.NRGBA{150, 150, 255, 50}
	}

	emitter.Shape = "star"
	emitter.Spread = 2 * math.Pi
	emitter.Speed = 35
	emitter.SpeedVariance = 12
	emitter.Life = 3.0
	emitter.LifeVariance = 0.7
	emitter.Size = 2.5
	emitter.SizeVariance = 0.7
	emitter.EmissionRate = 22
	emitter.Duration = -1 // Continuous buff
	emitter.Gravity = -15 // Float upward
	emitter.Drag = 0.94

	ps.AddEmitter(emitter)
}

// CreateCriticalHitEffect creates a spectacular critical hit effect
func (ps *ParticleSystem) CreateCriticalHitEffect(x, y float64) {
	emitter := NewParticleEmitter(x, y, 40)

	emitter.StartColor = color.NRGBA{255, 215, 0, 255} // Pure gold
	emitter.EndColor = color.NRGBA{255, 140, 0, 0}     // Gold to transparent
	emitter.Shape = "star"
	emitter.Spread = 2 * math.Pi
	emitter.Speed = 120
	emitter.SpeedVariance = 40
	emitter.Life = 1.5
	emitter.LifeVariance = 0.3
	emitter.Size = 5
	emitter.SizeVariance = 2
	emitter.EmissionRate = 100
	emitter.Duration = 0.4
	emitter.Gravity = -50 // Shoot upward
	emitter.Drag = 0.85

	ps.AddEmitter(emitter)
}

// CreateLevelUpEffect creates a celebration effect for leveling up
func (ps *ParticleSystem) CreateLevelUpEffect(x, y float64) {
	emitter := NewParticleEmitter(x, y, 50)

	emitter.StartColor = color.NRGBA{255, 255, 100, 255} // Bright yellow
	emitter.EndColor = color.NRGBA{255, 200, 0, 0}       // Yellow to transparent
	emitter.Shape = "star"
	emitter.Spread = 2 * math.Pi
	emitter.Speed = 150
	emitter.SpeedVariance = 50
	emitter.Life = 2.5
	emitter.LifeVariance = 0.5
	emitter.Size = 4
	emitter.SizeVariance = 1.5
	emitter.EmissionRate = 80
	emitter.Duration = 1.2
	emitter.Gravity = -80 // Celebrate upward
	emitter.Drag = 0.8

	ps.AddEmitter(emitter)
}

// CreateUnitDeathEffect creates spectacular death effects based on unit type
func (ps *ParticleSystem) CreateUnitDeathEffect(x, y float64, unitClass, unitSubclass string) {
	// Create multiple effects for a spectacular death animation
	class := strings.ToLower(unitClass)
	subclass := strings.ToLower(unitSubclass)

	// Main explosion effect (all units get this)
	emitter := NewParticleEmitter(x, y, 25)
	emitter.StartColor = color.NRGBA{255, 100, 0, 255} // Bright orange
	emitter.EndColor = color.NRGBA{255, 50, 0, 0}      // Orange to transparent
	emitter.Shape = "circle"
	emitter.Spread = 2 * math.Pi
	emitter.Speed = 120
	emitter.SpeedVariance = 40
	emitter.Life = 0.8
	emitter.LifeVariance = 0.3
	emitter.Size = 3
	emitter.SizeVariance = 1
	emitter.EmissionRate = 150
	emitter.Duration = 0.3
	emitter.Gravity = 100
	emitter.Drag = 0.9
	ps.AddEmitter(emitter)

	// Secondary debris effect
	emitter2 := NewParticleEmitter(x, y, 20)
	emitter2.StartColor = color.NRGBA{150, 150, 150, 200} // Gray
	emitter2.EndColor = color.NRGBA{50, 50, 50, 0}        // Gray to transparent
	emitter2.Shape = "circle"
	emitter2.Spread = 2 * math.Pi
	emitter2.Speed = 80
	emitter2.SpeedVariance = 30
	emitter2.Life = 1.2
	emitter2.LifeVariance = 0.4
	emitter2.Size = 2
	emitter2.SizeVariance = 0.8
	emitter2.EmissionRate = 80
	emitter2.Duration = 0.5
	emitter2.Gravity = 150
	emitter2.Drag = 0.95
	ps.AddEmitter(emitter2)

	// Class-specific effects
	switch class {
	case "range":
		// Ranged units get magical energy burst
		emitter3 := NewParticleEmitter(x, y, 15)
		emitter3.StartColor = color.NRGBA{100, 200, 255, 220} // Light blue
		emitter3.EndColor = color.NRGBA{50, 100, 255, 0}      // Blue to transparent
		emitter3.Shape = "star"
		emitter3.Spread = 2 * math.Pi
		emitter3.Speed = 100
		emitter3.SpeedVariance = 25
		emitter3.Life = 1.0
		emitter3.LifeVariance = 0.3
		emitter3.Size = 3
		emitter3.SizeVariance = 1
		emitter3.EmissionRate = 60
		emitter3.Duration = 0.4
		emitter3.Gravity = -50
		emitter3.Drag = 0.9
		ps.AddEmitter(emitter3)

		// Healers get special healing energy release
		if subclass == "healer" {
			emitter4 := NewParticleEmitter(x, y, 18)
			emitter4.StartColor = color.NRGBA{100, 255, 150, 200} // Bright green
			emitter4.EndColor = color.NRGBA{50, 255, 100, 0}      // Green to transparent
			emitter4.Shape = "star"
			emitter4.Spread = 2 * math.Pi
			emitter4.Speed = 90
			emitter4.SpeedVariance = 20
			emitter4.Life = 1.5
			emitter4.LifeVariance = 0.4
			emitter4.Size = 4
			emitter4.SizeVariance = 1.5
			emitter4.EmissionRate = 50
			emitter4.Duration = 0.6
			emitter4.Gravity = -30
			emitter4.Drag = 0.88
			ps.AddEmitter(emitter4)
		}

	case "melee":
		// Melee units get metallic debris
		emitter3 := NewParticleEmitter(x, y, 12)
		emitter3.StartColor = color.NRGBA{200, 200, 200, 220} // Silver
		emitter3.EndColor = color.NRGBA{100, 100, 100, 0}     // Gray to transparent
		emitter3.Shape = "circle"
		emitter3.Spread = math.Pi // 180 degrees forward
		emitter3.Speed = 70
		emitter3.SpeedVariance = 20
		emitter3.Life = 1.8
		emitter3.LifeVariance = 0.5
		emitter3.Size = 2.5
		emitter3.SizeVariance = 0.8
		emitter3.EmissionRate = 40
		emitter3.Duration = 0.7
		emitter3.Gravity = 120
		emitter3.Drag = 0.92
		ps.AddEmitter(emitter3)

	default:
		// Generic units get smoke effect
		emitter3 := NewParticleEmitter(x, y, 16)
		emitter3.StartColor = color.NRGBA{80, 80, 80, 180} // Dark gray
		emitter3.EndColor = color.NRGBA{30, 30, 30, 0}     // Dark gray to transparent
		emitter3.Shape = "circle"
		emitter3.Spread = 2 * math.Pi
		emitter3.Speed = 60
		emitter3.SpeedVariance = 15
		emitter3.Life = 2.0
		emitter3.LifeVariance = 0.6
		emitter3.Size = 3
		emitter3.SizeVariance = 1
		emitter3.EmissionRate = 35
		emitter3.Duration = 0.8
		emitter3.Gravity = -20
		emitter3.Drag = 0.96
		ps.AddEmitter(emitter3)
	}
}

// CreateUnitSpawnEffect creates spectacular spawn effects that look like units are dropped from the sky
func (ps *ParticleSystem) CreateUnitSpawnEffect(x, y float64, unitClass, unitSubclass string) {
	// Create effects that simulate units falling from above
	class := strings.ToLower(unitClass)
	subclass := strings.ToLower(unitSubclass)

	// Main drop effect - particles falling from above the spawn point
	emitter := NewParticleEmitter(x, y-100, 20)          // Start 100 pixels above
	emitter.StartColor = color.NRGBA{200, 220, 255, 180} // Light blue-white
	emitter.EndColor = color.NRGBA{150, 180, 255, 0}     // Blue to transparent
	emitter.Shape = "circle"
	emitter.Spread = math.Pi / 3 // Narrow downward spread
	emitter.Speed = 150
	emitter.SpeedVariance = 30
	emitter.Life = 1.2
	emitter.LifeVariance = 0.4
	emitter.Size = 2
	emitter.SizeVariance = 0.8
	emitter.EmissionRate = 80
	emitter.Duration = 0.6
	emitter.Gravity = 200 // Heavy gravity to simulate falling
	emitter.Drag = 0.85
	ps.AddEmitter(emitter)

	// Impact effect at spawn location
	emitter2 := NewParticleEmitter(x, y, 15)
	emitter2.StartColor = color.NRGBA{200, 220, 255, 200} // Light blue
	emitter2.EndColor = color.NRGBA{150, 180, 255, 0}     // Blue to transparent
	emitter2.Shape = "circle"
	emitter2.Spread = 2 * math.Pi
	emitter2.Speed = 80
	emitter2.SpeedVariance = 25
	emitter2.Life = 0.8
	emitter2.LifeVariance = 0.3
	emitter2.Size = 1.5
	emitter2.SizeVariance = 0.5
	emitter2.EmissionRate = 100
	emitter2.Duration = 0.3
	emitter2.Gravity = 50
	emitter2.Drag = 0.9
	ps.AddEmitter(emitter2)

	// Class-specific spawn effects
	switch class {
	case "range":
		// Ranged units get magical summoning circle
		emitter3 := NewParticleEmitter(x, y, 18)
		emitter3.StartColor = color.NRGBA{150, 200, 255, 160} // Light blue
		emitter3.EndColor = color.NRGBA{100, 150, 255, 0}     // Blue to transparent
		emitter3.Shape = "star"
		emitter3.Spread = 2 * math.Pi
		emitter3.Speed = 60
		emitter3.SpeedVariance = 15
		emitter3.Life = 1.5
		emitter3.LifeVariance = 0.4
		emitter3.Size = 3
		emitter3.SizeVariance = 1
		emitter3.EmissionRate = 50
		emitter3.Duration = 0.8
		emitter3.Gravity = -20 // Float upward slightly
		emitter3.Drag = 0.92
		ps.AddEmitter(emitter3)

		// Healers get special green summoning effect
		if subclass == "healer" {
			emitter4 := NewParticleEmitter(x, y, 12)
			emitter4.StartColor = color.NRGBA{150, 255, 180, 180} // Bright green
			emitter4.EndColor = color.NRGBA{100, 255, 150, 0}     // Green to transparent
			emitter4.Shape = "star"
			emitter4.Spread = 2 * math.Pi
			emitter4.Speed = 70
			emitter4.SpeedVariance = 20
			emitter4.Life = 1.8
			emitter4.LifeVariance = 0.5
			emitter4.Size = 4
			emitter4.SizeVariance = 1.2
			emitter4.EmissionRate = 40
			emitter4.Duration = 1.0
			emitter4.Gravity = -30
			emitter4.Drag = 0.88
			ps.AddEmitter(emitter4)
		}

	case "melee":
		// Melee units get metallic landing effect
		emitter3 := NewParticleEmitter(x, y, 16)
		emitter3.StartColor = color.NRGBA{220, 220, 220, 200} // Silver
		emitter3.EndColor = color.NRGBA{150, 150, 150, 0}     // Gray to transparent
		emitter3.Shape = "circle"
		emitter3.Spread = math.Pi // 180 degrees outward
		emitter3.Speed = 90
		emitter3.SpeedVariance = 25
		emitter3.Life = 1.0
		emitter3.LifeVariance = 0.3
		emitter3.Size = 2.5
		emitter3.SizeVariance = 0.8
		emitter3.EmissionRate = 60
		emitter3.Duration = 0.5
		emitter3.Gravity = 80
		emitter3.Drag = 0.9
		ps.AddEmitter(emitter3)

	default:
		// Generic units get energy burst
		emitter3 := NewParticleEmitter(x, y, 14)
		emitter3.StartColor = color.NRGBA{255, 255, 200, 180} // Yellow-white
		emitter3.EndColor = color.NRGBA{255, 200, 100, 0}     // Yellow to transparent
		emitter3.Shape = "circle"
		emitter3.Spread = 2 * math.Pi
		emitter3.Speed = 100
		emitter3.SpeedVariance = 30
		emitter3.Life = 1.2
		emitter3.LifeVariance = 0.4
		emitter3.Size = 2
		emitter3.SizeVariance = 0.7
		emitter3.EmissionRate = 45
		emitter3.Duration = 0.7
		emitter3.Gravity = 0
		emitter3.Drag = 0.95
		ps.AddEmitter(emitter3)
	}

	// Add a final "landing" effect that appears briefly at the exact spawn location
	emitterFinal := NewParticleEmitter(x, y, 8)
	emitterFinal.StartColor = color.NRGBA{200, 220, 255, 255} // Light blue
	emitterFinal.EndColor = color.NRGBA{150, 180, 255, 0}     // Blue to transparent
	emitterFinal.Shape = "circle"
	emitterFinal.Spread = 2 * math.Pi
	emitterFinal.Speed = 40
	emitterFinal.SpeedVariance = 10
	emitterFinal.Life = 0.5
	emitterFinal.LifeVariance = 0.2
	emitterFinal.Size = 1
	emitterFinal.SizeVariance = 0.3
	emitterFinal.EmissionRate = 60
	emitterFinal.Duration = 0.2
	emitterFinal.Gravity = 0
	emitterFinal.Drag = 0.98
	ps.AddEmitter(emitterFinal)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

// CreateVictoryCelebration creates a spectacular victory celebration effect
func (ps *ParticleSystem) CreateVictoryCelebration() {
	// Create multiple emitters for a spectacular celebration

	// Golden fireworks from center
	emitter1 := NewParticleEmitter(float64(protocol.ScreenW/2), float64(protocol.ScreenH/2), 60)
	emitter1.StartColor = color.NRGBA{255, 215, 0, 255} // Pure gold
	emitter1.EndColor = color.NRGBA{255, 140, 0, 0}     // Gold to transparent
	emitter1.Shape = "star"
	emitter1.Spread = 2 * math.Pi
	emitter1.Speed = 200
	emitter1.SpeedVariance = 80
	emitter1.Life = 3.0
	emitter1.LifeVariance = 0.8
	emitter1.Size = 6
	emitter1.SizeVariance = 3
	emitter1.EmissionRate = 120
	emitter1.Duration = 2.0
	emitter1.Gravity = -100 // Shoot upward
	emitter1.Drag = 0.8
	ps.AddEmitter(emitter1)

	// Colorful confetti from top
	emitter2 := NewParticleEmitter(float64(protocol.ScreenW/2), 50, 80)
	emitter2.StartColor = color.NRGBA{255, 100, 255, 220} // Magenta
	emitter2.EndColor = color.NRGBA{255, 50, 150, 0}      // Magenta to transparent
	emitter2.Shape = "square"
	emitter2.Spread = math.Pi // Downward spread
	emitter2.Speed = 150
	emitter2.SpeedVariance = 50
	emitter2.Life = 4.0
	emitter2.LifeVariance = 1.0
	emitter2.Size = 4
	emitter2.SizeVariance = 2
	emitter2.EmissionRate = 100
	emitter2.Duration = 3.0
	emitter2.Gravity = 50 // Fall down
	emitter2.Drag = 0.9
	ps.AddEmitter(emitter2)

	// Blue victory sparks
	emitter3 := NewParticleEmitter(float64(protocol.ScreenW/2), float64(protocol.ScreenH/2), 40)
	emitter3.StartColor = color.NRGBA{100, 200, 255, 255} // Bright blue
	emitter3.EndColor = color.NRGBA{50, 100, 255, 0}      // Blue to transparent
	emitter3.Shape = "circle"
	emitter3.Spread = 2 * math.Pi
	emitter3.Speed = 180
	emitter3.SpeedVariance = 60
	emitter3.Life = 2.5
	emitter3.LifeVariance = 0.6
	emitter3.Size = 3
	emitter3.SizeVariance = 1.5
	emitter3.EmissionRate = 80
	emitter3.Duration = 1.5
	emitter3.Gravity = -80
	emitter3.Drag = 0.85
	ps.AddEmitter(emitter3)

	// Green celebration particles
	emitter4 := NewParticleEmitter(float64(protocol.ScreenW/4), float64(protocol.ScreenH/2), 35)
	emitter4.StartColor = color.NRGBA{100, 255, 150, 240} // Bright green
	emitter4.EndColor = color.NRGBA{50, 255, 100, 0}      // Green to transparent
	emitter4.Shape = "star"
	emitter4.Spread = math.Pi / 2 // 90 degrees
	emitter4.Speed = 160
	emitter4.SpeedVariance = 40
	emitter4.Life = 3.5
	emitter4.LifeVariance = 0.9
	emitter4.Size = 5
	emitter4.SizeVariance = 2
	emitter4.EmissionRate = 60
	emitter4.Duration = 2.5
	emitter4.Gravity = -60
	emitter4.Drag = 0.88
	ps.AddEmitter(emitter4)

	// Red celebration particles from other side
	emitter5 := NewParticleEmitter(float64(3*protocol.ScreenW/4), float64(protocol.ScreenH/2), 35)
	emitter5.StartColor = color.NRGBA{255, 100, 150, 240} // Bright red-pink
	emitter5.EndColor = color.NRGBA{255, 50, 100, 0}      // Red-pink to transparent
	emitter5.Shape = "star"
	emitter5.Spread = math.Pi / 2 // 90 degrees
	emitter5.Speed = 160
	emitter5.SpeedVariance = 40
	emitter5.Life = 3.5
	emitter5.LifeVariance = 0.9
	emitter5.Size = 5
	emitter5.SizeVariance = 2
	emitter5.EmissionRate = 60
	emitter5.Duration = 2.5
	emitter5.Gravity = -60
	emitter5.Drag = 0.88
	ps.AddEmitter(emitter5)

	// Final golden burst from center
	emitter6 := NewParticleEmitter(float64(protocol.ScreenW/2), float64(protocol.ScreenH/2), 100)
	emitter6.StartColor = color.NRGBA{255, 255, 150, 255} // Bright yellow-gold
	emitter6.EndColor = color.NRGBA{255, 200, 0, 0}       // Yellow-gold to transparent
	emitter6.Shape = "star"
	emitter6.Spread = 2 * math.Pi
	emitter6.Speed = 250
	emitter6.SpeedVariance = 100
	emitter6.Life = 2.0
	emitter6.LifeVariance = 0.5
	emitter6.Size = 8
	emitter6.SizeVariance = 4
	emitter6.EmissionRate = 200
	emitter6.Duration = 1.0
	emitter6.Gravity = -120
	emitter6.Drag = 0.75
	ps.AddEmitter(emitter6)
}

// CreateDefeatEffect creates a somber defeat effect
func (ps *ParticleSystem) CreateDefeatEffect() {
	// Create multiple emitters for a dramatic defeat sequence

	// Gray falling particles from top
	emitter1 := NewParticleEmitter(float64(protocol.ScreenW/2), 50, 50)
	emitter1.StartColor = color.NRGBA{150, 150, 150, 180} // Gray
	emitter1.EndColor = color.NRGBA{80, 80, 80, 0}        // Gray to transparent
	emitter1.Shape = "circle"
	emitter1.Spread = math.Pi / 2 // Downward spread
	emitter1.Speed = 100
	emitter1.SpeedVariance = 30
	emitter1.Life = 4.0
	emitter1.LifeVariance = 1.0
	emitter1.Size = 3
	emitter1.SizeVariance = 1.5
	emitter1.EmissionRate = 60
	emitter1.Duration = 3.0
	emitter1.Gravity = 80 // Fall down
	emitter1.Drag = 0.95
	ps.AddEmitter(emitter1)

	// Dark particles drifting downward
	emitter2 := NewParticleEmitter(float64(protocol.ScreenW/3), float64(protocol.ScreenH/4), 30)
	emitter2.StartColor = color.NRGBA{100, 100, 100, 160} // Dark gray
	emitter2.EndColor = color.NRGBA{50, 50, 50, 0}        // Dark gray to transparent
	emitter2.Shape = "circle"
	emitter2.Spread = math.Pi // 180 degrees downward
	emitter2.Speed = 70
	emitter2.SpeedVariance = 20
	emitter2.Life = 3.5
	emitter2.LifeVariance = 0.8
	emitter2.Size = 2.5
	emitter2.SizeVariance = 1
	emitter2.EmissionRate = 40
	emitter2.Duration = 2.5
	emitter2.Gravity = 40
	emitter2.Drag = 0.97
	ps.AddEmitter(emitter2)

	// More dark particles from other side
	emitter3 := NewParticleEmitter(float64(2*protocol.ScreenW/3), float64(protocol.ScreenH/4), 30)
	emitter3.StartColor = color.NRGBA{120, 120, 120, 160} // Medium gray
	emitter3.EndColor = color.NRGBA{60, 60, 60, 0}        // Medium gray to transparent
	emitter3.Shape = "circle"
	emitter3.Spread = math.Pi // 180 degrees downward
	emitter3.Speed = 70
	emitter3.SpeedVariance = 20
	emitter3.Life = 3.5
	emitter3.LifeVariance = 0.8
	emitter3.Size = 2.5
	emitter3.SizeVariance = 1
	emitter3.EmissionRate = 40
	emitter3.Duration = 2.5
	emitter3.Gravity = 40
	emitter3.Drag = 0.97
	ps.AddEmitter(emitter3)

	// Subtle red defeat particles
	emitter4 := NewParticleEmitter(float64(protocol.ScreenW/2), float64(protocol.ScreenH/2), 25)
	emitter4.StartColor = color.NRGBA{150, 50, 50, 140} // Dark red
	emitter4.EndColor = color.NRGBA{100, 30, 30, 0}     // Dark red to transparent
	emitter4.Shape = "circle"
	emitter4.Spread = 2 * math.Pi
	emitter4.Speed = 50
	emitter4.SpeedVariance = 15
	emitter4.Life = 2.5
	emitter4.LifeVariance = 0.6
	emitter4.Size = 2
	emitter4.SizeVariance = 0.8
	emitter4.EmissionRate = 30
	emitter4.Duration = 2.0
	emitter4.Gravity = 20
	emitter4.Drag = 0.98
	ps.AddEmitter(emitter4)

	// Final somber particles
	emitter5 := NewParticleEmitter(float64(protocol.ScreenW/2), float64(protocol.ScreenH*3/4), 20)
	emitter5.StartColor = color.NRGBA{80, 80, 80, 120} // Very dark gray
	emitter5.EndColor = color.NRGBA{40, 40, 40, 0}     // Very dark gray to transparent
	emitter5.Shape = "circle"
	emitter5.Spread = 2 * math.Pi
	emitter5.Speed = 30
	emitter5.SpeedVariance = 10
	emitter5.Life = 3.0
	emitter5.LifeVariance = 0.7
	emitter5.Size = 1.5
	emitter5.SizeVariance = 0.5
	emitter5.EmissionRate = 20
	emitter5.Duration = 2.5
	emitter5.Gravity = 10
	emitter5.Drag = 0.99
	ps.AddEmitter(emitter5)
}

// CreateEnhancedProjectileTrail creates elemental projectile trails
func (ps *ParticleSystem) CreateEnhancedProjectileTrail(x1, y1, x2, y2 float64, projectileType string) {
	switch projectileType {
	case "fire":
		emitter := NewParticleEmitter(x1, y1, 15)
		emitter.StartColor = color.NRGBA{255, 150, 0, 200} // Orange
		emitter.EndColor = color.NRGBA{255, 50, 0, 0}      // Red to transparent
		emitter.Shape = "circle"
		emitter.Spread = math.Pi / 4 // Narrow spread toward target
		emitter.Speed = 120
		emitter.SpeedVariance = 30
		emitter.Life = 0.4
		emitter.LifeVariance = 0.1
		emitter.Size = 2
		emitter.SizeVariance = 0.8
		emitter.EmissionRate = 80
		emitter.Duration = 0.2
		emitter.Gravity = 0
		emitter.Drag = 0.9
		ps.AddEmitter(emitter)

	case "frost":
		emitter := NewParticleEmitter(x1, y1, 12)
		emitter.StartColor = color.NRGBA{150, 200, 255, 180} // Light blue
		emitter.EndColor = color.NRGBA{100, 150, 255, 0}     // Blue to transparent
		emitter.Shape = "circle"
		emitter.Spread = math.Pi / 6 // Very narrow spread
		emitter.Speed = 100
		emitter.SpeedVariance = 20
		emitter.Life = 0.6
		emitter.LifeVariance = 0.2
		emitter.Size = 1.5
		emitter.SizeVariance = 0.5
		emitter.EmissionRate = 60
		emitter.Duration = 0.3
		emitter.Gravity = 0
		emitter.Drag = 0.95
		ps.AddEmitter(emitter)

	case "lightning":
		emitter := NewParticleEmitter(x1, y1, 10)
		emitter.StartColor = color.NRGBA{255, 255, 150, 220} // Yellow
		emitter.EndColor = color.NRGBA{255, 200, 0, 0}       // Yellow to transparent
		emitter.Shape = "star"
		emitter.Spread = math.Pi / 3 // Moderate spread
		emitter.Speed = 150
		emitter.SpeedVariance = 40
		emitter.Life = 0.3
		emitter.LifeVariance = 0.1
		emitter.Size = 2.5
		emitter.SizeVariance = 1
		emitter.EmissionRate = 70
		emitter.Duration = 0.15
		emitter.Gravity = 0
		emitter.Drag = 0.85
		ps.AddEmitter(emitter)

	case "holy":
		emitter := NewParticleEmitter(x1, y1, 18)
		emitter.StartColor = color.NRGBA{255, 235, 100, 200} // Bright gold
		emitter.EndColor = color.NRGBA{255, 200, 0, 0}       // Gold to transparent
		emitter.Shape = "star"
		emitter.Spread = math.Pi / 2 // Wide spread
		emitter.Speed = 80
		emitter.SpeedVariance = 25
		emitter.Life = 0.8
		emitter.LifeVariance = 0.3
		emitter.Size = 3
		emitter.SizeVariance = 1.2
		emitter.EmissionRate = 50
		emitter.Duration = 0.4
		emitter.Gravity = -20
		emitter.Drag = 0.92
		ps.AddEmitter(emitter)

	case "dark":
		emitter := NewParticleEmitter(x1, y1, 14)
		emitter.StartColor = color.NRGBA{150, 50, 200, 180} // Purple
		emitter.EndColor = color.NRGBA{100, 0, 150, 0}      // Purple to transparent
		emitter.Shape = "circle"
		emitter.Spread = math.Pi / 4 // Narrow spread
		emitter.Speed = 90
		emitter.SpeedVariance = 20
		emitter.Life = 0.7
		emitter.LifeVariance = 0.2
		emitter.Size = 2
		emitter.SizeVariance = 0.7
		emitter.EmissionRate = 45
		emitter.Duration = 0.35
		emitter.Gravity = 10
		emitter.Drag = 0.94
		ps.AddEmitter(emitter)

	case "nature":
		emitter := NewParticleEmitter(x1, y1, 16)
		emitter.StartColor = color.NRGBA{100, 200, 100, 190} // Green
		emitter.EndColor = color.NRGBA{50, 150, 50, 0}       // Green to transparent
		emitter.Shape = "circle"
		emitter.Spread = math.Pi / 3 // Moderate spread
		emitter.Speed = 70
		emitter.SpeedVariance = 15
		emitter.Life = 0.9
		emitter.LifeVariance = 0.3
		emitter.Size = 2.5
		emitter.SizeVariance = 0.9
		emitter.EmissionRate = 40
		emitter.Duration = 0.45
		emitter.Gravity = 0
		emitter.Drag = 0.96
		ps.AddEmitter(emitter)

	case "arcane":
		emitter := NewParticleEmitter(x1, y1, 20)
		emitter.StartColor = color.NRGBA{200, 100, 255, 200} // Purple
		emitter.EndColor = color.NRGBA{150, 50, 255, 0}      // Purple to transparent
		emitter.Shape = "star"
		emitter.Spread = math.Pi / 2 // Wide spread
		emitter.Speed = 100
		emitter.SpeedVariance = 30
		emitter.Life = 0.6
		emitter.LifeVariance = 0.2
		emitter.Size = 2
		emitter.SizeVariance = 0.8
		emitter.EmissionRate = 60
		emitter.Duration = 0.3
		emitter.Gravity = 0
		emitter.Drag = 0.9
		ps.AddEmitter(emitter)

	default:
		// Default enhanced trail
		emitter := NewParticleEmitter(x1, y1, 12)
		emitter.StartColor = color.NRGBA{255, 255, 150, 180} // Yellow
		emitter.EndColor = color.NRGBA{255, 200, 0, 0}       // Yellow to transparent
		emitter.Shape = "circle"
		emitter.Spread = math.Pi / 4 // Narrow spread
		emitter.Speed = 110
		emitter.SpeedVariance = 25
		emitter.Life = 0.5
		emitter.LifeVariance = 0.15
		emitter.Size = 2
		emitter.SizeVariance = 0.6
		emitter.EmissionRate = 55
		emitter.Duration = 0.25
		emitter.Gravity = 0
		emitter.Drag = 0.93
		ps.AddEmitter(emitter)
	}
}

// CreateEnhancedImpactEffect creates elemental impact effects
func (ps *ParticleSystem) CreateEnhancedImpactEffect(x, y float64, projectileType string) {
	switch projectileType {
	case "fire":
		emitter := NewParticleEmitter(x, y, 25)
		emitter.StartColor = color.NRGBA{255, 150, 0, 255} // Bright orange
		emitter.EndColor = color.NRGBA{255, 50, 0, 0}      // Orange to transparent
		emitter.Shape = "circle"
		emitter.Spread = 2 * math.Pi
		emitter.Speed = 120
		emitter.SpeedVariance = 40
		emitter.Life = 0.8
		emitter.LifeVariance = 0.3
		emitter.Size = 3
		emitter.SizeVariance = 1.2
		emitter.EmissionRate = 100
		emitter.Duration = 0.3
		emitter.Gravity = 80
		emitter.Drag = 0.9
		ps.AddEmitter(emitter)

	case "frost":
		emitter := NewParticleEmitter(x, y, 20)
		emitter.StartColor = color.NRGBA{150, 200, 255, 255} // Light blue
		emitter.EndColor = color.NRGBA{100, 150, 255, 0}     // Blue to transparent
		emitter.Shape = "star"
		emitter.Spread = 2 * math.Pi
		emitter.Speed = 100
		emitter.SpeedVariance = 30
		emitter.Life = 1.0
		emitter.LifeVariance = 0.4
		emitter.Size = 3.5
		emitter.SizeVariance = 1.5
		emitter.EmissionRate = 80
		emitter.Duration = 0.4
		emitter.Gravity = 0
		emitter.Drag = 0.95
		ps.AddEmitter(emitter)

	case "lightning":
		emitter := NewParticleEmitter(x, y, 30)
		emitter.StartColor = color.NRGBA{255, 255, 150, 255} // Bright yellow
		emitter.EndColor = color.NRGBA{255, 200, 0, 0}       // Yellow to transparent
		emitter.Shape = "star"
		emitter.Spread = 2 * math.Pi
		emitter.Speed = 150
		emitter.SpeedVariance = 50
		emitter.Life = 0.6
		emitter.LifeVariance = 0.2
		emitter.Size = 4
		emitter.SizeVariance = 1.8
		emitter.EmissionRate = 120
		emitter.Duration = 0.25
		emitter.Gravity = -30
		emitter.Drag = 0.85
		ps.AddEmitter(emitter)

	case "holy":
		emitter := NewParticleEmitter(x, y, 35)
		emitter.StartColor = color.NRGBA{255, 235, 100, 255} // Bright gold
		emitter.EndColor = color.NRGBA{255, 200, 0, 0}       // Gold to transparent
		emitter.Shape = "star"
		emitter.Spread = 2 * math.Pi
		emitter.Speed = 110
		emitter.SpeedVariance = 35
		emitter.Life = 1.2
		emitter.LifeVariance = 0.4
		emitter.Size = 4.5
		emitter.SizeVariance = 2
		emitter.EmissionRate = 90
		emitter.Duration = 0.5
		emitter.Gravity = -40
		emitter.Drag = 0.88
		ps.AddEmitter(emitter)

	case "dark":
		emitter := NewParticleEmitter(x, y, 22)
		emitter.StartColor = color.NRGBA{150, 50, 200, 255} // Bright purple
		emitter.EndColor = color.NRGBA{100, 0, 150, 0}      // Purple to transparent
		emitter.Shape = "circle"
		emitter.Spread = 2 * math.Pi
		emitter.Speed = 90
		emitter.SpeedVariance = 25
		emitter.Life = 1.1
		emitter.LifeVariance = 0.3
		emitter.Size = 3
		emitter.SizeVariance = 1
		emitter.EmissionRate = 70
		emitter.Duration = 0.45
		emitter.Gravity = 20
		emitter.Drag = 0.92
		ps.AddEmitter(emitter)

	case "nature":
		emitter := NewParticleEmitter(x, y, 28)
		emitter.StartColor = color.NRGBA{100, 200, 100, 255} // Bright green
		emitter.EndColor = color.NRGBA{50, 150, 50, 0}       // Green to transparent
		emitter.Shape = "circle"
		emitter.Spread = 2 * math.Pi
		emitter.Speed = 80
		emitter.SpeedVariance = 20
		emitter.Life = 1.3
		emitter.LifeVariance = 0.5
		emitter.Size = 3.5
		emitter.SizeVariance = 1.5
		emitter.EmissionRate = 60
		emitter.Duration = 0.55
		emitter.Gravity = 0
		emitter.Drag = 0.96
		ps.AddEmitter(emitter)

	case "arcane":
		emitter := NewParticleEmitter(x, y, 32)
		emitter.StartColor = color.NRGBA{200, 100, 255, 255} // Bright purple
		emitter.EndColor = color.NRGBA{150, 50, 255, 0}      // Purple to transparent
		emitter.Shape = "star"
		emitter.Spread = 2 * math.Pi
		emitter.Speed = 130
		emitter.SpeedVariance = 40
		emitter.Life = 0.9
		emitter.LifeVariance = 0.3
		emitter.Size = 4
		emitter.SizeVariance = 1.6
		emitter.EmissionRate = 100
		emitter.Duration = 0.35
		emitter.Gravity = 0
		emitter.Drag = 0.9
		ps.AddEmitter(emitter)

	default:
		emitter := NewParticleEmitter(x, y, 20)
		emitter.StartColor = color.NRGBA{255, 255, 150, 255} // Bright yellow
		emitter.EndColor = color.NRGBA{255, 200, 0, 0}       // Yellow to transparent
		emitter.Shape = "circle"
		emitter.Spread = 2 * math.Pi
		emitter.Speed = 110
		emitter.SpeedVariance = 30
		emitter.Life = 0.7
		emitter.LifeVariance = 0.2
		emitter.Size = 3
		emitter.SizeVariance = 1
		emitter.EmissionRate = 85
		emitter.Duration = 0.3
		emitter.Gravity = 0
		emitter.Drag = 0.93
		ps.AddEmitter(emitter)
	}
}

// CreateBaseDamageEffect creates spectacular base damage effects
func (ps *ParticleSystem) CreateBaseDamageEffect(x, y float64, damage, baseHP, baseMaxHP int) {
	// Calculate damage severity (0-1 scale)
	damageRatio := float64(damage) / float64(baseMaxHP)
	if damageRatio > 1.0 {
		damageRatio = 1.0
	}

	// Scale effect intensity based on damage severity
	intensity := 0.5 + damageRatio*1.5 // 0.5 to 2.0 scale

	// Main impact effect - shockwave
	emitter1 := NewParticleEmitter(x, y, int(30*intensity))
	emitter1.StartColor = color.NRGBA{255, 200, 100, 255} // Bright orange
	emitter1.EndColor = color.NRGBA{255, 150, 50, 0}      // Orange to transparent
	emitter1.Shape = "circle"
	emitter1.Spread = 2 * math.Pi
	emitter1.Speed = 150 * intensity
	emitter1.SpeedVariance = 50
	emitter1.Life = 0.8
	emitter1.LifeVariance = 0.3
	emitter1.Size = 4 * intensity
	emitter1.SizeVariance = 1.5
	emitter1.EmissionRate = 120 * intensity
	emitter1.Duration = 0.3
	emitter1.Gravity = 0
	emitter1.Drag = 0.9
	ps.AddEmitter(emitter1)

	// Debris particles - flying outward
	emitter2 := NewParticleEmitter(x, y, int(25*intensity))
	emitter2.StartColor = color.NRGBA{200, 150, 100, 220} // Brown-orange
	emitter2.EndColor = color.NRGBA{100, 75, 50, 0}       // Brown to transparent
	emitter2.Shape = "circle"
	emitter2.Spread = 2 * math.Pi
	emitter2.Speed = 120 * intensity
	emitter2.SpeedVariance = 40
	emitter2.Life = 1.2
	emitter2.LifeVariance = 0.4
	emitter2.Size = 2.5 * intensity
	emitter2.SizeVariance = 1
	emitter2.EmissionRate = 80 * intensity
	emitter2.Duration = 0.5
	emitter2.Gravity = 150
	emitter2.Drag = 0.95
	ps.AddEmitter(emitter2)

	// Spark effects - bright flashes
	emitter3 := NewParticleEmitter(x, y, int(20*intensity))
	emitter3.StartColor = color.NRGBA{255, 255, 150, 255} // Bright yellow
	emitter3.EndColor = color.NRGBA{255, 100, 0, 0}       // Yellow to transparent
	emitter3.Shape = "star"
	emitter3.Spread = 2 * math.Pi
	emitter3.Speed = 100 * intensity
	emitter3.SpeedVariance = 30
	emitter3.Life = 0.6
	emitter3.LifeVariance = 0.2
	emitter3.Size = 3 * intensity
	emitter3.SizeVariance = 1
	emitter3.EmissionRate = 100 * intensity
	emitter3.Duration = 0.25
	emitter3.Gravity = -50
	emitter3.Drag = 0.88
	ps.AddEmitter(emitter3)

	// Base health indicator - color changes based on remaining health
	healthRatio := float64(baseHP) / float64(baseMaxHP)

	// Critical damage effect (if base health is low)
	if healthRatio < 0.3 {
		emitter4 := NewParticleEmitter(x, y, int(15*intensity))
		emitter4.StartColor = color.NRGBA{255, 50, 50, 200} // Bright red
		emitter4.EndColor = color.NRGBA{150, 25, 25, 0}     // Red to transparent
		emitter4.Shape = "circle"
		emitter4.Spread = 2 * math.Pi
		emitter4.Speed = 80 * intensity
		emitter4.SpeedVariance = 25
		emitter4.Life = 2.0
		emitter4.LifeVariance = 0.5
		emitter4.Size = 2 * intensity
		emitter4.SizeVariance = 0.8
		emitter4.EmissionRate = 40 * intensity
		emitter4.Duration = 1.5
		emitter4.Gravity = 30
		emitter4.Drag = 0.96
		ps.AddEmitter(emitter4)
	}

	// Screen shake effect for major damage
	if damageRatio > 0.5 {
		// Add a subtle screen shake by creating particles that move the camera
		// This would require camera shake implementation in the main game loop
		// For now, we'll just add more intense effects
		emitter5 := NewParticleEmitter(x, y, int(35*intensity))
		emitter5.StartColor = color.NRGBA{255, 150, 0, 240} // Bright orange
		emitter5.EndColor = color.NRGBA{255, 75, 0, 0}      // Orange to transparent
		emitter5.Shape = "circle"
		emitter5.Spread = 2 * math.Pi
		emitter5.Speed = 180 * intensity
		emitter5.SpeedVariance = 60
		emitter5.Life = 1.0
		emitter5.LifeVariance = 0.3
		emitter5.Size = 3.5 * intensity
		emitter5.SizeVariance = 1.2
		emitter5.EmissionRate = 150 * intensity
		emitter5.Duration = 0.4
		emitter5.Gravity = 80
		emitter5.Drag = 0.85
		ps.AddEmitter(emitter5)
	}

	// Damage number effect - floating damage numbers
	// This would require text rendering, but we can simulate with particles
	emitter6 := NewParticleEmitter(x, y-20, int(8*intensity))
	emitter6.StartColor = color.NRGBA{255, 200, 100, 255} // Orange
	emitter6.EndColor = color.NRGBA{255, 150, 50, 0}      // Orange to transparent
	emitter6.Shape = "circle"
	emitter6.Spread = math.Pi / 6 // Narrow upward spread
	emitter6.Speed = 60
	emitter6.SpeedVariance = 15
	emitter6.Life = 1.5
	emitter6.LifeVariance = 0.3
	emitter6.Size = 1.5
	emitter6.SizeVariance = 0.5
	emitter6.EmissionRate = 25
	emitter6.Duration = 1.0
	emitter6.Gravity = -40 // Float upward
	emitter6.Drag = 0.95
	ps.AddEmitter(emitter6)
}

// CreateAoEImpactEffect creates a visual AoE circle effect at the impact location
func (ps *ParticleSystem) CreateAoEImpactEffect(x, y float64, damage int) {
	// Create a ring effect that shows the AoE radius (60 pixels)
	// Use multiple concentric circles that fade out to show the area

	// Outer ring - shows the full AoE radius
	emitter1 := NewParticleEmitter(x, y, 40)
	emitter1.StartColor = color.NRGBA{255, 100, 0, 200} // Bright orange
	emitter1.EndColor = color.NRGBA{255, 50, 0, 0}      // Orange to transparent
	emitter1.Shape = "circle"
	emitter1.Spread = 2 * math.Pi
	emitter1.Speed = 0 // Stationary
	emitter1.SpeedVariance = 0
	emitter1.Life = 0.8
	emitter1.LifeVariance = 0.2
	emitter1.Size = 60 // 60 pixel radius
	emitter1.SizeVariance = 0
	emitter1.EmissionRate = 1
	emitter1.Duration = 0.8
	emitter1.Gravity = 0
	emitter1.Drag = 1.0
	ps.AddEmitter(emitter1)

	// Middle ring - slightly smaller
	emitter2 := NewParticleEmitter(x, y, 30)
	emitter2.StartColor = color.NRGBA{255, 150, 0, 180} // Lighter orange
	emitter2.EndColor = color.NRGBA{255, 100, 0, 0}     // Orange to transparent
	emitter2.Shape = "circle"
	emitter2.Spread = 2 * math.Pi
	emitter2.Speed = 0
	emitter2.SpeedVariance = 0
	emitter2.Life = 0.6
	emitter2.LifeVariance = 0.1
	emitter2.Size = 55
	emitter2.SizeVariance = 0
	emitter2.EmissionRate = 1
	emitter2.Duration = 0.6
	emitter2.Gravity = 0
	emitter2.Drag = 1.0
	ps.AddEmitter(emitter2)

	// Inner ring - even smaller
	emitter3 := NewParticleEmitter(x, y, 20)
	emitter3.StartColor = color.NRGBA{255, 200, 0, 160} // Yellow-orange
	emitter3.EndColor = color.NRGBA{255, 150, 0, 0}     // Yellow to transparent
	emitter3.Shape = "circle"
	emitter3.Spread = 2 * math.Pi
	emitter3.Speed = 0
	emitter3.SpeedVariance = 0
	emitter3.Life = 0.4
	emitter3.LifeVariance = 0.1
	emitter3.Size = 50
	emitter3.SizeVariance = 0
	emitter3.EmissionRate = 1
	emitter3.Duration = 0.4
	emitter3.Gravity = 0
	emitter3.Drag = 1.0
	ps.AddEmitter(emitter3)

	// Add some spark particles around the AoE circle
	emitter4 := NewParticleEmitter(x, y, 25)
	emitter4.StartColor = color.NRGBA{255, 150, 0, 255} // Bright orange sparks
	emitter4.EndColor = color.NRGBA{255, 50, 0, 0}      // Orange to transparent
	emitter4.Shape = "circle"
	emitter4.Spread = 2 * math.Pi
	emitter4.Speed = 80
	emitter4.SpeedVariance = 20
	emitter4.Life = 0.5
	emitter4.LifeVariance = 0.2
	emitter4.Size = 2
	emitter4.SizeVariance = 1
	emitter4.EmissionRate = 50
	emitter4.Duration = 0.3
	emitter4.Gravity = 0
	emitter4.Drag = 0.95
	ps.AddEmitter(emitter4)
}
