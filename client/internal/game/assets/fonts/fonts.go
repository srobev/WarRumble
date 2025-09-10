package fonts

import (
	"embed"
	"image/color"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
)

// Embed all TTF files in this folder.
//
//go:embed *.ttf
var files embed.FS

// CHANGE this to the exact filename you added (e.g., "Cinzel-Black.ttf").
const PrimaryFile = "Cinzel-Black.ttf"

var (
	mu    sync.Mutex
	cache = map[float64]font.Face{} // size -> face
)

// Face returns a cached face for the given size (pixels).
func Face(size float64) font.Face {
	mu.Lock()
	defer mu.Unlock()

	if f, ok := cache[size]; ok {
		return f
	}

	data, err := files.ReadFile(PrimaryFile)
	if err != nil {
		panic("fonts: cannot read " + PrimaryFile + ": " + err.Error())
	}
	ft, err := opentype.Parse(data)
	if err != nil {
		panic("fonts: parse: " + err.Error())
	}
	f, err := opentype.NewFace(ft, &opentype.FaceOptions{
		Size:    size,
		DPI:     96,
		Hinting: font.HintingFull,
	})
	if err != nil {
		panic("fonts: face: " + err.Error())
	}
	cache[size] = f
	return f
}

// DrawOutlined draws outlined text (soft shadow/outline + fill).
// Use for headers/buttons to get a "fantasy UI" vibe.
func DrawOutlined(dst *ebiten.Image, s string, x, y int, size float64, fill color.Color) {
	shadow := color.RGBA{0, 0, 0, 200}
	face := Face(size)
	for dx := -2; dx <= 2; dx++ {
		for dy := -2; dy <= 2; dy++ {
			if dx == 0 && dy == 0 {
				continue
			}
			text.Draw(dst, s, face, x+dx, y+dy, shadow)
		}
	}
	text.Draw(dst, s, face, x, y, fill)
}
