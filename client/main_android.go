//go:build android

package main

import (
	"log"
	"rumble/client/internal/game"

	"github.com/hajimehoshi/ebiten/v2/mobile"
)

func init() {
	log.Println("Android init: SetGame")
	mobile.SetGame(game.New())
}
func main() {}
