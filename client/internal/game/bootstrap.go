//go:build !android

package game

import (
	"rumble/shared/protocol"

	"github.com/hajimehoshi/ebiten/v2"
)

func init() {
	// Your default window size/title (optional)
	ebiten.SetWindowSize(600, 1000)
	ebiten.SetWindowTitle(protocol.GameName)

	// Ebiten v2.8+: preferred API
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	// If you're on an older Ebiten (<2.8) and the line above doesn't compile,
	// comment it and use this instead:
	// ebiten.SetWindowResizable(true)

	// Optional: set reasonable min size, no max (-1, -1)
	ebiten.SetWindowSizeLimits(400, 600, -1, -1)

	// Set higher internal resolution for better quality on high-resolution displays
	// This helps reduce pixelation on 2560x1440 and similar high-res displays
	ebiten.SetScreenClearedEveryFrame(false)
}
