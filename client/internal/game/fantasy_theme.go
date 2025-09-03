package game

import "image/color"

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
		// Core colors - Much darker theme
		Primary:   color.NRGBA{20, 20, 30, 255},    // Very dark slate
		Secondary: color.NRGBA{255, 215, 0, 255},   // Bright gold
		Accent:    color.NRGBA{120, 120, 140, 255}, // Dark gray-blue
		Danger:    color.NRGBA{139, 0, 0, 255},     // Blood red
		Success:   color.NRGBA{34, 139, 34, 255},   // Emerald green
		Warning:   color.NRGBA{255, 140, 0, 255},   // Amber

		// Backgrounds - Much darker theme
		Background:     color.NRGBA{15, 15, 20, 255}, // Extremely dark
		Surface:        color.NRGBA{25, 25, 35, 255}, // Very dark surface
		CardBackground: color.NRGBA{30, 30, 40, 255}, // Very dark card background
		Glass:          color.NRGBA{35, 35, 45, 180}, // Very dark glass

		// Text
		TextPrimary:   color.NRGBA{240, 240, 250, 255}, // Very light text
		TextSecondary: color.NRGBA{200, 200, 210, 255}, // Light muted text
		TextMuted:     color.NRGBA{100, 100, 110, 255}, // Very dark text

		// Effects - Enhanced golden theme
		Glow:      color.NRGBA{255, 215, 0, 200}, // Bright golden glow
		Shadow:    color.NRGBA{0, 0, 0, 150},     // Deeper shadow
		Border:    color.NRGBA{60, 60, 70, 255},  // Darker border
		Highlight: color.NRGBA{255, 215, 0, 120}, // Bright golden highlight
	}
}
