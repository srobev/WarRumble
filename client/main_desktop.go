//go:build !android

package main

import (
    "log"
    "rumble/client/internal/game"

    "github.com/hajimehoshi/ebiten/v2"
)

func main() {
    game.SetPlatform("desktop") // optional
    log.Println("Desktop main() starting...")
    if err := ebiten.RunGame(game.New()); err != nil {
        log.Fatal(err)
    }
}

