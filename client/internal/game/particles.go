package game

import (
	"image/color"
	"math"
	"math/rand"
	"time"

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
		StartColor:    color.NRGBA{255, 255, 255, 255},
		EndColor:      color.NRGBA{255, 255, 255, 0},
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
	emitter := NewParticleEmitter(x, y, 20)

	emitter.StartColor = color.NRGBA{100, 255, 100, 255} // Bright green
	emitter.EndColor = color.NRGBA{50, 255, 50, 0}       // Green to transparent
	emitter.Shape = "star"
	emitter.Spread = math.Pi / 3 // 60 degrees
	emitter.Speed = 60
	emitter.SpeedVariance = 15
	emitter.Life = 1.0
	emitter.LifeVariance = 0.3
	emitter.Size = 3
	emitter.SizeVariance = 1
	emitter.EmissionRate = 40
	emitter.Duration = 0.8
	emitter.Gravity = -80 // Float upward
	emitter.Drag = 0.96

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
	emitter.Duration = -1 // Continuous
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

func init() {
	rand.Seed(time.Now().UnixNano())
}
