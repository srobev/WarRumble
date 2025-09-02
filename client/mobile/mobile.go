package clientmobile

import (
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/mobile"
)

type emptyGame struct{}

func (emptyGame) Update() error              { return nil }
func (emptyGame) Draw(*ebiten.Image)         {}
func (emptyGame) Layout(int, int) (int, int) { return 480, 800 }

func init()  { mobile.SetGame(emptyGame{}) }
func Dummy() {}
