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

//go:embed *.ttf
var files embed.FS

// If you already have a TitleFile (e.g., Rakkas), keep it.
// For Home screen we'll use UIFile = SignikaNegative-SemiBold.ttf.
const (
	TitleFile = "Cinzel-Black.ttf"             // keep Cinzel for titles
	UIFile    = "SignikaNegative-SemiBold.ttf" // use this for Home screen
)

type key struct {
	file string
	size float64
}

var (
	mu    sync.Mutex
	cache = map[key]font.Face{}
)

func face(file string, size float64) font.Face {
	k := key{file, size}
	mu.Lock()
	defer mu.Unlock()

	if f, ok := cache[k]; ok {
		return f
	}
	data, err := files.ReadFile(file)
	if err != nil {
		panic("fonts: read " + file + ": " + err.Error())
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
	cache[k] = f
	return f
}

func Title(size float64) font.Face { return face(TitleFile, size) }
func UI(size float64) font.Face    { return face(UIFile, size) }

// Outlined text using the UI (Signika) face – for Home screen headings/buttons
func DrawOutlinedUI(dst *ebiten.Image, s string, x, y int, size float64, fill color.Color) {
	shadow := color.RGBA{0, 0, 0, 200}
	ff := UI(size)
	for dx := -2; dx <= 2; dx++ {
		for dy := -2; dy <= 2; dy++ {
			if dx == 0 && dy == 0 {
				continue
			}
			text.Draw(dst, s, ff, x+dx, y+dy, shadow)
		}
	}
	text.Draw(dst, s, ff, x, y, fill)
}
