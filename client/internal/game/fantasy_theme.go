package game

import (
	"image"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// FantasyTheme defines the color palette and visual style for the fantasy UI
type FantasyTheme struct {
	// Primary colors
	Primary   color.NRGBA // Deep royal purple
	Secondary color.NRGBA // Antique gold
	Accent    color.NRGBA // Crimson red
	Danger    color.NRGBA // Blood red
	Success   color.NRGBA // Emerald green
	Warning   color.NRGBA // Amber

	// Background colors
	Background     color.NRGBA // Dark stone
	Surface        color.NRGBA // Slightly lighter stone
	CardBackground color.NRGBA // Parchment-like
	Glass          color.NRGBA // Enchanted glass

	// Text colors
	TextPrimary   color.NRGBA // Light parchment
	TextSecondary color.NRGBA // Muted gold
	TextMuted     color.NRGBA // Dark stone

	// Special effects
	Glow      color.NRGBA // Magical glow
	Shadow    color.NRGBA // Deep shadow
	Border    color.NRGBA // Runed border
	Highlight color.NRGBA // Selection highlight
}

// DefaultFantasyTheme creates the standard fantasy color palette
func DefaultFantasyTheme() *FantasyTheme {
	return &FantasyTheme{
		// Core colors
		Primary:   color.NRGBA{45, 27, 105, 255},  // Deep royal purple
		Secondary: color.NRGBA{212, 175, 55, 255}, // Antique gold
		Accent:    color.NRGBA{220, 20, 60, 255},  // Crimson red
		Danger:    color.NRGBA{139, 0, 0, 255},    // Blood red
		Success:   color.NRGBA{34, 139, 34, 255},  // Emerald green
		Warning:   color.NRGBA{255, 140, 0, 255},  // Amber

		// Backgrounds
		Background:     color.NRGBA{28, 28, 28, 255},    // Dark stone
		Surface:        color.NRGBA{42, 42, 42, 255},    // Slightly lighter stone
		CardBackground: color.NRGBA{245, 245, 220, 255}, // Parchment
		Glass:          color.NRGBA{74, 85, 104, 180},   // Enchanted glass

		// Text
		TextPrimary:   color.NRGBA{250, 250, 250, 255}, // Light parchment
		TextSecondary: color.NRGBA{212, 175, 55, 255},  // Muted gold
		TextMuted:     color.NRGBA{120, 120, 120, 255}, // Dark stone

		// Effects
		Glow:      color.NRGBA{255, 215, 0, 200},  // Magical glow
		Shadow:    color.NRGBA{0, 0, 0, 120},      // Deep shadow
		Border:    color.NRGBA{160, 130, 90, 255}, // Runed border
		Highlight: color.NRGBA{255, 215, 0, 100},  // Selection highlight
	}
}

// FantasyButton represents an enhanced button with fantasy styling and animations
type FantasyButton struct {
	X, Y          int
	Width, Height int
	Text          string
	State         ButtonState
	Theme         *FantasyTheme

	// Animation properties
	pressScale    float64
	hoverGlow     float64
	animationTime time.Time

	// Touch handling
	touchID int
	onClick func()
}

// ButtonState represents the current state of a button
type ButtonState int

const (
	ButtonNormal ButtonState = iota
	ButtonHover
	ButtonPressed
	ButtonDisabled
)

// NewFantasyButton creates a new fantasy-styled button
func NewFantasyButton(x, y, width, height int, text string, theme *FantasyTheme, onClick func()) *FantasyButton {
	return &FantasyButton{
		X:             x,
		Y:             y,
		Width:         width,
		Height:        height,
		Text:          text,
		State:         ButtonNormal,
		Theme:         theme,
		pressScale:    1.0,
		hoverGlow:     0.0,
		animationTime: time.Now(),
		touchID:       -1,
		onClick:       onClick,
	}
}

// Update handles button state updates and animations
func (b *FantasyButton) Update() {
	// Handle touch input for mobile
	touches := ebiten.AppendTouchIDs(nil)
	mousePressed := ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	mx, my := ebiten.CursorPosition()

	// Check if button is being touched/clicked
	isPressed := false
	if mousePressed && b.containsPoint(mx, my) {
		isPressed = true
	} else {
		for _, id := range touches {
			tx, ty := ebiten.TouchPosition(id)
			if b.containsPoint(tx, ty) {
				isPressed = true
				b.touchID = int(id)
				break
			}
		}
	}

	// Update button state
	if b.State == ButtonDisabled {
		return
	}

	if isPressed {
		b.State = ButtonPressed
		b.pressScale = 0.95 // Slight scale down when pressed
		b.hoverGlow = 1.0
	} else if b.containsPoint(mx, my) {
		b.State = ButtonHover
		b.pressScale = 1.0
		b.hoverGlow = math.Min(b.hoverGlow+0.1, 0.8) // Smooth glow increase
	} else {
		b.State = ButtonNormal
		b.pressScale = 1.0
		b.hoverGlow = math.Max(b.hoverGlow-0.05, 0.0) // Smooth glow decrease
	}

	// Handle click/tap release
	if b.touchID != -1 && len(touches) == 0 && !mousePressed {
		if b.onClick != nil {
			b.onClick()
		}
		b.touchID = -1
	}
}

// Draw renders the fantasy button
func (b *FantasyButton) Draw(screen *ebiten.Image) {
	// Calculate animation values
	scale := b.pressScale
	glowIntensity := b.hoverGlow

	// Scale transformation
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate(-float64(b.Width)*(scale-1)/2, -float64(b.Height)*(scale-1)/2)
	op.GeoM.Translate(float64(b.X), float64(b.Y))

	// Draw button background
	vector.DrawFilledRect(screen, float32(b.X), float32(b.Y),
		float32(b.Width), float32(b.Height), b.Theme.Surface, true)

	// Draw border with runes
	b.drawRunedBorder(screen, glowIntensity)

	// Draw glow effect when hovered/pressed
	if glowIntensity > 0 {
		b.drawGlowEffect(screen, glowIntensity)
	}

	// Draw text
	b.drawText(screen)
}

// containsPoint checks if a point is within the button bounds
func (b *FantasyButton) containsPoint(x, y int) bool {
	return x >= b.X && x <= b.X+b.Width && y >= b.Y && y <= b.Y+b.Height
}

// drawRunedBorder draws an enchanted border with subtle runes
func (b *FantasyButton) drawRunedBorder(screen *ebiten.Image, glowIntensity float64) {
	borderWidth := float32(2.0)
	borderColor := b.Theme.Border

	// Adjust border color based on state
	if b.State == ButtonPressed {
		borderColor = b.Theme.Accent
	} else if b.State == ButtonHover {
		// Blend with glow color
		r := uint8(float64(b.Theme.Border.R)*(1-glowIntensity) + float64(b.Theme.Glow.R)*glowIntensity)
		g := uint8(float64(b.Theme.Border.G)*(1-glowIntensity) + float64(b.Theme.Glow.G)*glowIntensity)
		b := uint8(float64(b.Theme.Border.B)*(1-glowIntensity) + float64(b.Theme.Glow.B)*glowIntensity)
		borderColor = color.NRGBA{r, g, b, 255}
	}

	// Draw border
	vector.StrokeRect(screen, float32(b.X), float32(b.Y),
		float32(b.Width), float32(b.Height), borderWidth, borderColor, true)
}

// drawGlowEffect creates a magical glow around the button
func (b *FantasyButton) drawGlowEffect(screen *ebiten.Image, intensity float64) {
	glowColor := color.NRGBA{
		R: uint8(float64(b.Theme.Glow.R) * intensity),
		G: uint8(float64(b.Theme.Glow.G) * intensity),
		B: uint8(float64(b.Theme.Glow.B) * intensity),
		A: uint8(100 * intensity),
	}

	// Draw multiple glow layers for depth
	for i := 0; i < 3; i++ {
		offset := float32(i * 2)
		alpha := uint8(float64(glowColor.A) * (1.0 - float64(i)*0.3))

		vector.StrokeRect(screen,
			float32(b.X)-offset, float32(b.Y)-offset,
			float32(b.Width)+offset*2, float32(b.Height)+offset*2,
			1.0, color.NRGBA{glowColor.R, glowColor.G, glowColor.B, alpha}, true)
	}
}

// drawText renders the button text with fantasy styling
func (b *FantasyButton) drawText(screen *ebiten.Image) {
	// Use basic font for now - can be enhanced with fantasy fonts later
	textColor := b.Theme.TextPrimary

	if b.State == ButtonDisabled {
		textColor = b.Theme.TextMuted
	} else if b.State == ButtonPressed {
		textColor = b.Theme.Secondary
	}

	// Center text
	textWidth := len(b.Text) * 7 // Approximate width for basic font
	textX := b.X + (b.Width-textWidth)/2
	textY := b.Y + b.Height/2 + 4

	// Draw text shadow for depth
	shadowColor := color.NRGBA{0, 0, 0, 120}
	vector.DrawFilledRect(screen, float32(textX-1), float32(textY+1),
		float32(textWidth), 8, shadowColor, true)

	// Draw main text
	vector.DrawFilledRect(screen, float32(textX), float32(textY),
		float32(textWidth), 8, textColor, true)
}

// SetOnClick sets the click handler for the button
func (b *FantasyButton) SetOnClick(handler func()) {
	b.onClick = handler
}

// SetState changes the button state
func (b *FantasyButton) SetState(state ButtonState) {
	b.State = state
}

// GetBounds returns the button bounds as an image.Rectangle
func (b *FantasyButton) GetBounds() image.Rectangle {
	return image.Rect(b.X, b.Y, b.X+b.Width, b.Y+b.Height)
}
