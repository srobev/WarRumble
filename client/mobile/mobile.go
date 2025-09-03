package mobile

import (
	"rumble/client/internal/game"

	"github.com/hajimehoshi/ebiten/v2/mobile"
)

func init() {
	// Initialize the actual WarRumble game instead of empty game
	mobile.SetGame(game.New("android"))
}

func Dummy() {}
