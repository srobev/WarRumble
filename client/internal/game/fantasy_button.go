package game

import (
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

// ButtonState represents the current state of a button
type ButtonState int

const (
	ButtonNormal ButtonState = iota
	ButtonHover
	ButtonPressed
)

// FantasyButton represents an enhanced themed button with animations
type FantasyButton struct {
	X, Y          int
	Width, Height int
	Label         string
	State         ButtonState
	Theme         *FantasyTheme

	// Special styling flags
	IsBottomBarButton bool // For bottom bar specific styling
	IsPvpButton       bool // For PvP section buttons - uses bottom bar colors

	// Animation properties
	hoverProgress float64
	pressProgress float64
	lastState     ButtonState
	animationTime time.Time

	// Cached rendering
	cachedImage *ebiten.Image
	needsRedraw bool
}

// NewFantasyButton creates a new FantasyButton
func NewFantasyButton(x, y, width, height int, label string, theme *FantasyTheme, onClick func()) *FantasyButton {
	return &FantasyButton{
		X:             x,
		Y:             y,
		Width:         width,
		Height:        height,
		Label:         label,
		State:         ButtonNormal,
		Theme:         theme,
		animationTime: time.Now(),
		needsRedraw:   true,
	}
}

// Update updates the button's animation state
func (btn *FantasyButton) Update() {
	now := time.Now()
	dt := now.Sub(btn.animationTime).Seconds()
	btn.animationTime = now

	// Smooth state transitions
	targetHover := 0.0
	targetPress := 0.0

	switch btn.State {
	case ButtonHover:
		targetHover = 1.0
	case ButtonPressed:
		targetPress = 1.0
		targetHover = 1.0
	}

	// Animate hover progress
	if btn.hoverProgress < targetHover {
		btn.hoverProgress = math.Min(targetHover, btn.hoverProgress+dt*8)
		btn.needsRedraw = true
	} else if btn.hoverProgress > targetHover {
		btn.hoverProgress = math.Max(targetHover, btn.hoverProgress-dt*8)
		btn.needsRedraw = true
	}

	// Animate press progress
	if btn.pressProgress < targetPress {
		btn.pressProgress = math.Min(targetPress, btn.pressProgress+dt*12)
		btn.needsRedraw = true
	} else if btn.pressProgress > targetPress {
		btn.pressProgress = math.Max(targetPress, btn.pressProgress-dt*12)
		btn.needsRedraw = true
	}

	// Check if state changed
	if btn.State != btn.lastState {
		btn.needsRedraw = true
		btn.lastState = btn.State
	}
}

// Draw renders the button
func (btn *FantasyButton) Draw(screen *ebiten.Image) {
	btn.Update()

	// Create cached image if needed
	if btn.cachedImage == nil || btn.needsRedraw {
		btn.cachedImage = ebiten.NewImage(btn.Width, btn.Height)
		btn.renderToCache()
		btn.needsRedraw = false
	}

	// Draw cached image
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(btn.X), float64(btn.Y))
	screen.DrawImage(btn.cachedImage, op)
}

// renderToCache renders the button to the cached image
func (btn *FantasyButton) renderToCache() {
	if btn.cachedImage == nil {
		return
	}

	// Clear cache
	btn.cachedImage.Clear()

	// Calculate animated colors
	baseColor := btn.Theme.Primary
	hoverColor := btn.Theme.Secondary
	pressedColor := btn.Theme.Accent

	// For bottom bar buttons and PvP buttons, use darker greyish colors instead of gold
	if btn.IsBottomBarButton || btn.IsPvpButton {
		// Dark grey for hover/pressed states instead of gold
		hoverColor = color.NRGBA{70, 70, 80, 255}   // Dark grey
		pressedColor = color.NRGBA{50, 50, 60, 255} // Even darker grey
	}

	// Interpolate colors based on state
	r := uint8(float64(baseColor.R) + (float64(hoverColor.R)-float64(baseColor.R))*btn.hoverProgress)
	g := uint8(float64(baseColor.G) + (float64(hoverColor.G)-float64(baseColor.G))*btn.hoverProgress)
	b := uint8(float64(baseColor.B) + (float64(hoverColor.B)-float64(baseColor.B))*btn.hoverProgress)
	a := uint8(float64(baseColor.A) + (float64(pressedColor.A)-float64(baseColor.A))*btn.pressProgress)

	buttonColor := color.NRGBA{r, g, b, a}

	// Draw button background with rounded corners effect
	vector.DrawFilledRect(btn.cachedImage, 0, 0, float32(btn.Width), float32(btn.Height), buttonColor, true)

	// Add border with theme colors - enhanced golden frames for hover/pressed
	borderColor := btn.Theme.Border
	borderWidth := float32(2)

	if btn.State == ButtonHover {
		borderColor = btn.Theme.Secondary // Bright gold for hover
		borderWidth = 3
	} else if btn.State == ButtonPressed {
		borderColor = btn.Theme.Glow // Bright golden glow for pressed
		borderWidth = 4
	}

	vector.StrokeRect(btn.cachedImage, 0, 0, float32(btn.Width), float32(btn.Height), borderWidth, borderColor, true)

	// Add inner highlight for depth
	highlightColor := color.NRGBA{
		R: uint8(math.Min(255, float64(buttonColor.R)*1.3)),
		G: uint8(math.Min(255, float64(buttonColor.G)*1.3)),
		B: uint8(math.Min(255, float64(buttonColor.B)*1.3)),
		A: 100,
	}
	vector.StrokeRect(btn.cachedImage, 2, 2, float32(btn.Width-4), float32(btn.Height-4), 1, highlightColor, true)

	// Add enhanced glow effect for hover/press states
	if btn.hoverProgress > 0 {
		glowAlpha := uint8(btn.hoverProgress * 180)
		glowColor := color.NRGBA{
			R: btn.Theme.Glow.R,
			G: btn.Theme.Glow.G,
			B: btn.Theme.Glow.B,
			A: glowAlpha,
		}

		// Outer golden glow
		vector.StrokeRect(btn.cachedImage, -2, -2, float32(btn.Width+4), float32(btn.Height+4), 4, glowColor, true)

		// Inner golden frame for pressed state
		if btn.State == ButtonPressed {
			innerGlowColor := color.NRGBA{
				R: btn.Theme.Secondary.R,
				G: btn.Theme.Secondary.G,
				B: btn.Theme.Secondary.B,
				A: 220,
			}
			vector.StrokeRect(btn.cachedImage, 1, 1, float32(btn.Width-2), float32(btn.Height-2), 2, innerGlowColor, true)
		}
	}

	// Draw label
	labelColor := btn.Theme.TextPrimary
	if btn.State == ButtonPressed {
		// Slightly darker text when pressed
		labelColor = color.NRGBA{
			R: uint8(float64(btn.Theme.TextPrimary.R) * 0.8),
			G: uint8(float64(btn.Theme.TextPrimary.G) * 0.8),
			B: uint8(float64(btn.Theme.TextPrimary.B) * 0.8),
			A: btn.Theme.TextPrimary.A,
		}
	}

	// Center the text
	bounds := text.BoundString(basicfont.Face7x13, btn.Label)
	textX := (btn.Width - bounds.Dx()) / 2
	textY := (btn.Height + bounds.Dy()) / 2

	text.Draw(btn.cachedImage, btn.Label, basicfont.Face7x13, textX, textY, labelColor)

	// Add subtle shadow for depth
	if btn.State != ButtonPressed {
		shadowColor := color.NRGBA{0, 0, 0, 60}
		text.Draw(btn.cachedImage, btn.Label, basicfont.Face7x13, textX+1, textY+1, shadowColor)
	}
}

// Contains checks if a point is within the button bounds
func (btn *FantasyButton) Contains(x, y int) bool {
	return x >= btn.X && x <= btn.X+btn.Width && y >= btn.Y && y <= btn.Y+btn.Height
}

// SetState sets the button state and marks for redraw
func (btn *FantasyButton) SetState(state ButtonState) {
	if btn.State != state {
		btn.State = state
		btn.needsRedraw = true
	}
}

// GetBounds returns the button bounds as an image.Rectangle
func (btn *FantasyButton) GetBounds() (int, int, int, int) {
	return btn.X, btn.Y, btn.Width, btn.Height
}
