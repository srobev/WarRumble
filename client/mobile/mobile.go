package yourgamemobile

import (
	"rumble/client/internal/game"

	"github.com/hajimehoshi/ebiten/v2/mobile"
)

func init() {
	// game.New() must return ebiten.Game
	mobile.SetGame(game.New())
}

// Dummy is required so gomobile/ebitenmobile will bind this package.
func Dummy() {}
