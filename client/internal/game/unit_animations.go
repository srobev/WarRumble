package game

import (
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
)

// UnitAnimationState represents the current animation state of a unit
type UnitAnimationState int

const (
	UnitStateIdle UnitAnimationState = iota
	UnitStateWalking
	UnitStateAttacking
	UnitStateHit
	UnitStateDeath
	UnitStateCasting
	UnitStateDefending
)

// UnitAnimationData holds animation data for a unit
type UnitAnimationData struct {
	State          UnitAnimationState
	StateStartTime time.Time
	AnimationTime  float64 // Current animation time (0-1)
	AnimationSpeed float64 // Speed multiplier for animations

	// Transform values (applied to unit rendering)
	ScaleX     float64
	ScaleY     float64
	Rotation   float64
	TranslateX float64
	TranslateY float64

	// Visual effects
	GlowColor      ColorWithAlpha
	GlowIntensity  float64
	ShakeIntensity float64
	ShakeDuration  float64

	// Movement tracking
	LastX         float64
	LastY         float64
	MovementSpeed float64

	// Special effects
	ParticleEmitter *ParticleEmitter
	TrailEffect     bool
}

// ColorWithAlpha represents a color with alpha channel
type ColorWithAlpha struct {
	R, G, B, A uint8
}

// NewUnitAnimationData creates a new animation data instance
func NewUnitAnimationData() *UnitAnimationData {
	return &UnitAnimationData{
		State:          UnitStateIdle,
		StateStartTime: time.Now(),
		AnimationTime:  0,
		AnimationSpeed: 1.0,
		ScaleX:         1.0,
		ScaleY:         1.0,
		Rotation:       0,
		TranslateX:     0,
		TranslateY:     0,
		GlowColor:      ColorWithAlpha{255, 255, 255, 0},
		GlowIntensity:  0,
		ShakeIntensity: 0,
		ShakeDuration:  0,
		LastX:          0,
		LastY:          0,
		MovementSpeed:  0,
		TrailEffect:    false,
	}
}

// UpdateAnimation updates the unit's animation state and calculates transforms
func (anim *UnitAnimationData) UpdateAnimation(unit *RenderUnit, deltaTime float64) {
	currentTime := time.Now()
	anim.AnimationTime = currentTime.Sub(anim.StateStartTime).Seconds() * anim.AnimationSpeed

	// Calculate movement speed
	dx := unit.X - anim.LastX
	dy := unit.Y - anim.LastY
	anim.MovementSpeed = math.Sqrt(dx*dx+dy*dy) / deltaTime
	anim.LastX = unit.X
	anim.LastY = unit.Y

	// Determine animation state based on unit conditions
	anim.updateAnimationState(unit)

	// Apply animation transforms based on current state
	anim.applyAnimationTransforms(deltaTime)

	// Update shake effect
	if anim.ShakeDuration > 0 {
		anim.ShakeDuration -= deltaTime
		if anim.ShakeDuration <= 0 {
			anim.ShakeIntensity = 0
		}
	}
}

// updateAnimationState determines the current animation state
func (anim *UnitAnimationData) updateAnimationState(unit *RenderUnit) {
	// Check for movement (walking state)
	if anim.MovementSpeed > 10 { // Moving faster than 10 pixels/second
		if anim.State != UnitStateWalking {
			anim.changeState(UnitStateWalking)
		}
	} else if anim.State == UnitStateWalking {
		// Stopped moving, go back to idle
		anim.changeState(UnitStateIdle)
	}

	// TODO: Add logic for attacking, hit, death states based on unit data
	// This would require additional data from the server about unit actions
}

// changeState changes the animation state and resets timing
func (anim *UnitAnimationData) changeState(newState UnitAnimationState) {
	if anim.State != newState {
		anim.State = newState
		anim.StateStartTime = time.Now()
		anim.AnimationTime = 0
	}
}

// applyAnimationTransforms applies procedural transforms based on animation state
func (anim *UnitAnimationData) applyAnimationTransforms(deltaTime float64) {
	// Reset transforms to defaults
	anim.ScaleX = 1.0
	anim.ScaleY = 1.0
	anim.Rotation = 0
	anim.TranslateX = 0
	anim.TranslateY = 0
	anim.GlowIntensity = 0

	switch anim.State {
	case UnitStateIdle:
		// Gentle breathing animation
		breathRate := 2.0    // breaths per second
		breathAmount := 0.02 // 2% size variation
		anim.ScaleX = 1.0 + math.Sin(anim.AnimationTime*breathRate*math.Pi*2)*breathAmount
		anim.ScaleY = anim.ScaleX

		// Subtle glow for idle units
		anim.GlowColor = ColorWithAlpha{200, 220, 255, 30}
		anim.GlowIntensity = 0.3

	case UnitStateWalking:
		// Walking bobbing animation
		walkRate := 4.0       // steps per second
		bobAmount := 0.05     // 5% vertical movement
		rotationAmount := 3.0 // 3 degrees rotation

		// Vertical bobbing
		anim.TranslateY = math.Sin(anim.AnimationTime*walkRate*math.Pi*2) * bobAmount * 42 // 42px is unit size

		// Slight rotation for walking motion
		anim.Rotation = math.Sin(anim.AnimationTime*walkRate*math.Pi*2) * rotationAmount * math.Pi / 180

		// Scale pulsing for walking rhythm
		scalePulse := 0.03
		anim.ScaleX = 1.0 + math.Sin(anim.AnimationTime*walkRate*math.Pi*2)*scalePulse
		anim.ScaleY = anim.ScaleX

		// Movement trail effect
		anim.TrailEffect = true

	case UnitStateAttacking:
		// Forward lunge animation
		attackDuration := 0.5 // 0.5 second attack animation
		attackProgress := math.Min(anim.AnimationTime/attackDuration, 1.0)

		// Forward movement during attack
		lungeDistance := 8.0
		anim.TranslateX = math.Sin(attackProgress*math.Pi) * lungeDistance

		// Scale pulse during attack
		scalePulse := 0.1
		anim.ScaleX = 1.0 + math.Sin(attackProgress*math.Pi)*scalePulse
		anim.ScaleY = anim.ScaleX

		// Bright glow during attack
		anim.GlowColor = ColorWithAlpha{255, 150, 150, 100}
		anim.GlowIntensity = 0.8

	case UnitStateHit:
		// Hit reaction animation
		hitDuration := 0.3
		hitProgress := math.Min(anim.AnimationTime/hitDuration, 1.0)

		// Shake effect
		anim.ShakeIntensity = (1.0 - hitProgress) * 3.0

		// Quick scale down then up
		scaleEffect := math.Sin(hitProgress*math.Pi) * 0.1
		anim.ScaleX = 1.0 - scaleEffect
		anim.ScaleY = anim.ScaleX

		// Red glow for damage
		anim.GlowColor = ColorWithAlpha{255, 100, 100, 150}
		anim.GlowIntensity = 1.0 - hitProgress

	case UnitStateDeath:
		// Death animation - dramatic scaling and rotation
		deathDuration := 1.0
		deathProgress := math.Min(anim.AnimationTime/deathDuration, 1.0)

		// Scale down to nothing
		scaleFactor := 1.0 - deathProgress*deathProgress
		anim.ScaleX = scaleFactor
		anim.ScaleY = scaleFactor

		// Dramatic rotation
		anim.Rotation = deathProgress * math.Pi * 4 // 4 full rotations

		// Fade out glow
		anim.GlowColor = ColorWithAlpha{255, 100, 100, uint8((1.0 - deathProgress) * 100)}
		anim.GlowIntensity = 1.0 - deathProgress

	case UnitStateCasting:
		// Spell casting animation
		// Magical pulsing
		pulseRate := 8.0
		pulseAmount := 0.05
		anim.ScaleX = 1.0 + math.Sin(anim.AnimationTime*pulseRate*math.Pi*2)*pulseAmount
		anim.ScaleY = anim.ScaleX

		// Magical glow
		anim.GlowColor = ColorWithAlpha{150, 200, 255, 120}
		anim.GlowIntensity = 0.6 + math.Sin(anim.AnimationTime*4*math.Pi*2)*0.2

	case UnitStateDefending:
		// Defensive stance animation
		// Slight crouching effect
		anim.ScaleY = 0.95
		anim.TranslateY = 2.0

		// Protective glow
		anim.GlowColor = ColorWithAlpha{100, 150, 255, 80}
		anim.GlowIntensity = 0.5
	}
}

// ApplyTransforms applies the animation transforms to the draw options
func (anim *UnitAnimationData) ApplyTransforms(op *ebiten.DrawImageOptions, centerX, centerY float64) {
	// Apply shake effect
	shakeX := 0.0
	shakeY := 0.0
	if anim.ShakeIntensity > 0 {
		timeValue := float64(time.Now().UnixNano()) / 10000000.0
		shakeX = (math.Sin(timeValue) * anim.ShakeIntensity)
		shakeY = (math.Cos(timeValue) * anim.ShakeIntensity)
	}

	// Apply transforms in correct order: translate to origin, scale, rotate, translate back
	op.GeoM.Translate(-centerX, -centerY)
	op.GeoM.Scale(anim.ScaleX, anim.ScaleY)
	op.GeoM.Rotate(anim.Rotation)
	op.GeoM.Translate(centerX+anim.TranslateX+shakeX, centerY+anim.TranslateY+shakeY)
}

// GetGlowColor returns the current glow color as ebiten color
func (anim *UnitAnimationData) GetGlowColor() (float64, float64, float64, float64) {
	intensity := anim.GlowIntensity
	return float64(anim.GlowColor.R) / 255.0 * intensity,
		float64(anim.GlowColor.G) / 255.0 * intensity,
		float64(anim.GlowColor.B) / 255.0 * intensity,
		float64(anim.GlowColor.A) / 255.0 * intensity
}

// TriggerHit triggers the hit animation
func (anim *UnitAnimationData) TriggerHit() {
	anim.changeState(UnitStateHit)
	anim.ShakeDuration = 0.3
	anim.ShakeIntensity = 3.0
}

// TriggerAttack triggers the attack animation
func (anim *UnitAnimationData) TriggerAttack() {
	anim.changeState(UnitStateAttacking)
}

// TriggerDeath triggers the death animation
func (anim *UnitAnimationData) TriggerDeath() {
	anim.changeState(UnitStateDeath)
}

// TriggerCast triggers the casting animation
func (anim *UnitAnimationData) TriggerCast() {
	anim.changeState(UnitStateCasting)
}

// TriggerDefend triggers the defend animation
func (anim *UnitAnimationData) TriggerDefend() {
	anim.changeState(UnitStateDefending)
}

// IsAnimationComplete returns true if the current animation state is complete
func (anim *UnitAnimationData) IsAnimationComplete() bool {
	switch anim.State {
	case UnitStateAttacking:
		return anim.AnimationTime >= 0.5
	case UnitStateHit:
		return anim.AnimationTime >= 0.3
	case UnitStateDeath:
		return anim.AnimationTime >= 1.0
	case UnitStateCasting:
		return anim.AnimationTime >= 1.0
	default:
		return false
	}
}

// ResetToIdle resets the animation to idle state
func (anim *UnitAnimationData) ResetToIdle() {
	anim.changeState(UnitStateIdle)
}
