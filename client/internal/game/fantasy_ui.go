package game

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"time"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

// FantasyUI manages the application of fantasy themes across all UI elements
type FantasyUI struct {
	Theme         *FantasyTheme
	AnimationTime time.Time

	// Cached UI elements
	buttonCache     map[string]*FantasyButton
	backgroundCache map[string]*ebiten.Image

	// Animation states
	hoverStates map[string]float64
	pressStates map[string]float64
}

// NewFantasyUI creates a new FantasyUI manager
func NewFantasyUI(theme *FantasyTheme) *FantasyUI {
	return &FantasyUI{
		Theme:           theme,
		AnimationTime:   time.Now(),
		buttonCache:     make(map[string]*FantasyButton),
		backgroundCache: make(map[string]*ebiten.Image),
		hoverStates:     make(map[string]float64),
		pressStates:     make(map[string]float64),
	}
}

// Update handles UI animations and state updates
func (ui *FantasyUI) Update() {
	ui.AnimationTime = time.Now()

	// Update hover/press states with smooth transitions
	for key, state := range ui.hoverStates {
		if state > 0 {
			ui.hoverStates[key] = math.Max(0, state-0.05)
		}
	}
	for key, state := range ui.pressStates {
		if state > 0 {
			ui.pressStates[key] = math.Max(0, state-0.1)
		}
	}
}

// DrawThemedButton draws a themed button with enhanced styling
func (ui *FantasyUI) DrawThemedButton(screen *ebiten.Image, x, y, width, height int, label string, state ButtonState) {
	key := label + "_btn"

	// Get or create cached button
	btn, exists := ui.buttonCache[key]
	if !exists {
		btn = NewFantasyButton(x, y, width, height, label, ui.Theme, nil)
		ui.buttonCache[key] = btn
	}

	// Update button position and state
	btn.X, btn.Y = x, y
	btn.Width, btn.Height = width, height
	btn.State = state

	// Update animation states
	switch state {
	case ButtonHover:
		ui.hoverStates[key] = 1.0
	case ButtonPressed:
		ui.pressStates[key] = 1.0
	}

	btn.Draw(screen)
}

// DrawThemedCard draws a themed card with ornate borders
func (ui *FantasyUI) DrawThemedCard(screen *ebiten.Image, x, y, width, height int, title string, content []string) {
	// Draw card background with theme colors
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), float32(height), ui.Theme.CardBackground, true)

	// Draw ornate border
	borderWidth := float32(3)
	vector.StrokeRect(screen, float32(x), float32(y), float32(width), float32(height), borderWidth, ui.Theme.Border, true)

	// Add inner highlight
	innerOffset := borderWidth + 1
	vector.StrokeRect(screen, float32(x)+innerOffset, float32(y)+innerOffset,
		float32(width)-innerOffset*2, float32(height)-innerOffset*2, 1, ui.Theme.Glow, true)

	// Draw title
	if title != "" {
		titleY := y + 8
		text.Draw(screen, title, basicfont.Face7x13, x+12, titleY+12, ui.Theme.TextPrimary)

		// Title underline
		underlineY := titleY + 16
		vector.DrawFilledRect(screen, float32(x+12), float32(underlineY), float32(width-24), 2, ui.Theme.Secondary, true)
	}

	// Draw content
	contentY := y + 30
	for i, line := range content {
		text.Draw(screen, line, basicfont.Face7x13, x+12, contentY+i*16, ui.Theme.TextSecondary)
	}
}

// DrawThemedPanel draws a themed panel with background and borders
func (ui *FantasyUI) DrawThemedPanel(screen *ebiten.Image, x, y, width, height int, alpha float64) {
	// Semi-transparent background
	backgroundColor := color.NRGBA{
		R: uint8(float64(ui.Theme.Background.R) * alpha),
		G: uint8(float64(ui.Theme.Background.G) * alpha),
		B: uint8(float64(ui.Theme.Background.B) * alpha),
		A: uint8(float64(ui.Theme.Background.A) * alpha),
	}

	vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), float32(height), backgroundColor, true)

	// Subtle border
	borderColor := color.NRGBA{
		R: uint8(float64(ui.Theme.Border.R) * alpha),
		G: uint8(float64(ui.Theme.Border.G) * alpha),
		B: uint8(float64(ui.Theme.Border.B) * alpha),
		A: uint8(float64(ui.Theme.Border.A) * alpha),
	}

	vector.StrokeRect(screen, float32(x), float32(y), float32(width), float32(height), 1, borderColor, true)
}

// DrawEnhancedUnitCard draws a high-quality unit card with theme integration
func (ui *FantasyUI) DrawEnhancedUnitCard(screen *ebiten.Image, x, y, width, height int, unitName, unitClass string, level, cost int, unitImage *ebiten.Image, isSelected, isHovered bool) {
	// Card background with theme - use different logic for champion vs mini cards
	var cardColor color.NRGBA
	if unitClass == "Champion" {
		// Champion cards use a different background logic
		if isSelected {
			// For selected champion, use a slightly different shade
			r := uint8(math.Min(255, float64(ui.Theme.Surface.R)*1.1))
			g := uint8(math.Min(255, float64(ui.Theme.Surface.G)*1.1))
			b := uint8(math.Min(255, float64(ui.Theme.Surface.B)*1.1))
			a := ui.Theme.Surface.A
			cardColor = color.NRGBA{r, g, b, a}
		} else {
			// Normal champion background
			cardColor = ui.Theme.Surface
		}
	} else {
		// Mini cards use the original logic
		if isSelected {
			// Blend primary with accent for selected state
			r := uint8((float64(ui.Theme.Primary.R) + float64(ui.Theme.Accent.R)) / 2)
			g := uint8((float64(ui.Theme.Primary.G) + float64(ui.Theme.Accent.G)) / 2)
			b := uint8((float64(ui.Theme.Primary.B) + float64(ui.Theme.Accent.B)) / 2)
			a := uint8((float64(ui.Theme.Primary.A) + float64(ui.Theme.Accent.A)) / 2)
			cardColor = color.NRGBA{r, g, b, a}
		} else if isHovered {
			// Slightly brighter for hover
			r := uint8(math.Min(255, float64(ui.Theme.Surface.R)*1.2))
			g := uint8(math.Min(255, float64(ui.Theme.Surface.G)*1.2))
			b := uint8(math.Min(255, float64(ui.Theme.Surface.B)*1.2))
			a := ui.Theme.Surface.A
			cardColor = color.NRGBA{r, g, b, a}
		} else {
			cardColor = ui.Theme.Surface
		}
	}

	vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), float32(height), cardColor, true)

	// Enhanced border with theme colors
	borderColor := ui.Theme.Border
	if isSelected {
		borderColor = ui.Theme.Secondary
	} else if isHovered {
		borderColor = ui.Theme.Glow
	}

	vector.StrokeRect(screen, float32(x), float32(y), float32(width), float32(height), 2, borderColor, true)

	// Unit image with high-quality rendering
	if unitImage != nil {
		imageX := x + 8
		imageY := y + 8
		imageSize := 80

		// High-quality scaling with linear filtering
		op := &ebiten.DrawImageOptions{}
		iw, ih := unitImage.Bounds().Dx(), unitImage.Bounds().Dy()
		scale := float64(imageSize) / math.Max(float64(iw), float64(ih))
		op.GeoM.Scale(scale, scale)
		op.GeoM.Translate(float64(imageX), float64(imageY))
		op.Filter = ebiten.FilterLinear // High-quality filtering
		screen.DrawImage(unitImage, op)

		// Add subtle glow for selected units
		if isSelected {
			glowColor := color.NRGBA{
				R: uint8(float64(ui.Theme.Glow.R) * 0.3),
				G: uint8(float64(ui.Theme.Glow.G) * 0.3),
				B: uint8(float64(ui.Theme.Glow.B) * 0.3),
				A: 100,
			}
			vector.StrokeRect(screen, float32(imageX-2), float32(imageY-2),
				float32(imageSize+4), float32(imageSize+4), 3, glowColor, true)
		}
	}

	// Cost display in bottom right with darker text for visibility
	if cost > 0 {
		costStr := fmt.Sprintf("%d", cost)
		costW := len(costStr) * 7
		costX := x + width - costW - 8
		costY := y + height - 16

		// Cost badge with gold theme
		vector.DrawFilledRect(screen, float32(costX-4), float32(costY-2), float32(costW+8), 14, ui.Theme.Secondary, true)
		vector.StrokeRect(screen, float32(costX-4), float32(costY-2), float32(costW+8), 14, 1, ui.Theme.Border, true)
		// Use dark gray text for better visibility
		text.Draw(screen, costStr, basicfont.Face7x13, costX, costY+10, color.NRGBA{64, 64, 64, 255})
	}
}

// DrawThemedTooltip draws a themed tooltip with enhanced styling
func (ui *FantasyUI) DrawThemedTooltip(screen *ebiten.Image, x, y, width, height int, title string, lines []string) {
	// Tooltip background with theme
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(width), float32(height), ui.Theme.CardBackground, true)

	// Border
	vector.StrokeRect(screen, float32(x), float32(y), float32(width), float32(height), 2, ui.Theme.Border, true)

	// Title
	if title != "" {
		text.Draw(screen, title, basicfont.Face7x13, x+8, y+16, ui.Theme.TextPrimary)

		// Title separator
		vector.DrawFilledRect(screen, float32(x+8), float32(y+20), float32(width-16), 1, ui.Theme.Border, true)
	}

	// Content lines
	lineY := y + 28
	for _, line := range lines {
		text.Draw(screen, line, basicfont.Face7x13, x+8, lineY, ui.Theme.TextSecondary)
		lineY += 14
	}
}

// GetButtonBounds returns the bounds of a themed button
func (ui *FantasyUI) GetButtonBounds(x, y, width, height int) image.Rectangle {
	return image.Rect(x, y, x+width, y+height)
}

// CreateBackgroundPattern creates a subtle background pattern
func (ui *FantasyUI) CreateBackgroundPattern(width, height int) *ebiten.Image {
	key := fmt.Sprintf("bg_%dx%d", width, height)

	// Check cache first
	if bg, exists := ui.backgroundCache[key]; exists {
		return bg
	}

	// Create new background
	img := ebiten.NewImage(width, height)

	// Fill with base background color
	vector.DrawFilledRect(img, 0, 0, float32(width), float32(height), ui.Theme.Background, true)

	// Add subtle pattern
	patternColor := color.NRGBA{
		R: uint8(math.Min(255, float64(ui.Theme.Background.R)*1.1)),
		G: uint8(math.Min(255, float64(ui.Theme.Background.G)*1.1)),
		B: uint8(math.Min(255, float64(ui.Theme.Background.B)*1.1)),
		A: 30,
	}

	// Simple dot pattern
	for i := 0; i < width; i += 20 {
		for j := 0; j < height; j += 20 {
			vector.DrawFilledCircle(img, float32(i), float32(j), 1, patternColor, true)
		}
	}

	// Cache the background
	ui.backgroundCache[key] = img
	return img
}
