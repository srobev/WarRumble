package game

import (
	"image/color"

	"github.com/hajimehoshi/ebiten/v2"
)

type Assets struct {
	btn9Base  *ebiten.Image
	btn9Hover *ebiten.Image

	minis               map[string]*ebiten.Image // key: portrait filename (or derived)
	baseMe              *ebiten.Image
	baseEnemy           *ebiten.Image
	baseDead            *ebiten.Image            // optional destroyed variant
	bg                  map[string]*ebiten.Image // key: mapID -> background
	coinFull, coinEmpty *ebiten.Image

	edgeCol map[string]color.NRGBA // <- new: per-map letterbox color
}
