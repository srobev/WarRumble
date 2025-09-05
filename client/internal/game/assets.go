package game

import (
	"embed"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg" // (optional) register JPEG decoder
	_ "image/png"  // register PNG decoder
	"log"
	"math"
	"path"
	"rumble/client/internal/game/ui"
	"rumble/shared/protocol"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

//go:embed assets/ui/* assets/ui/avatars/* assets/minis/* assets/maps/* assets/obstacles/*
var assetsFS embed.FS

var ornate *ui.OrnateBar

// Assets struct holds all loaded game assets
type Assets struct {
	btn9Base  *ebiten.Image
	btn9Hover *ebiten.Image
	minis     map[string]*ebiten.Image
	obstacles map[string]*ebiten.Image
	bg        map[string]*ebiten.Image
	baseMe    *ebiten.Image
	baseEnemy *ebiten.Image
	baseDead  *ebiten.Image
	coinFull  *ebiten.Image
	coinEmpty *ebiten.Image
	edgeCol   map[string]color.NRGBA
}

// index of real filenames by lowercase name per directory
var embIndex = map[string]map[string]string{}

func buildEmbIndex() {
	if len(embIndex) != 0 {
		return
	}
	for _, dir := range []string{"assets/ui", "assets/minis", "assets/maps", "assets/obstacles"} {
		entries, err := assetsFS.ReadDir(dir)
		if err != nil {
			continue
		}
		m := make(map[string]string, len(entries))
		for _, e := range entries {
			m[strings.ToLower(e.Name())] = e.Name()
		}
		embIndex[dir] = m
	}
}

func loadImage(p string) *ebiten.Image {
	buildEmbIndex()

	f, err := assetsFS.Open(p)
	if err != nil {

		dir, file := path.Split(p)
		key := strings.ToLower(strings.TrimSuffix(file, ""))
		if idx, ok := embIndex[strings.TrimRight(dir, "/")]; ok {
			if real, ok2 := idx[key]; ok2 {
				f, err = assetsFS.Open(dir + real)
			}
		}
	}
	if err != nil {
		log.Println("asset not found in embed:", p)
		return nil
	}
	defer f.Close()

	img, _, err := ebitenutil.NewImageFromReader(f)
	if err != nil {
		log.Println("decode image failed:", p, err)
		return nil
	}
	return img
}

func (a *Assets) ensureInit() {
	if a.btn9Base == nil {
		a.btn9Base = loadImage("assets/ui/btn9_slim.png")
	}
	if a.btn9Hover == nil {
		a.btn9Hover = loadImage("assets/ui/btn9_slim_hover.png")
	}
	if a.minis == nil {
		a.minis = make(map[string]*ebiten.Image)
	}
	if a.obstacles == nil {
		a.obstacles = make(map[string]*ebiten.Image)
	}
	if a.bg == nil {
		a.bg = make(map[string]*ebiten.Image)
	}
	if a.baseMe == nil {
		a.baseMe = loadImage("assets/ui/base.png")
	}
	if a.baseEnemy == nil {
		a.baseEnemy = loadImage("assets/ui/base.png")
	}
	if a.baseDead == nil {
		a.baseDead = loadImage("assets/ui/base_destroyed.png")
	}
	if a.coinFull == nil {
		a.coinFull = loadImage("assets/ui/coin.png")
	}
	if a.coinEmpty == nil {
		a.coinEmpty = loadImage("assets/ui/coin_empty.png")
	}
	if a.edgeCol == nil {
		a.edgeCol = make(map[string]color.NRGBA)
	}
}

func (g *Game) portraitKeyFor(name string) string {

	if info, ok := g.nameToMini[name]; ok && info.Portrait != "" {
		return info.Portrait
	}

	return strings.ToLower(strings.ReplaceAll(name, " ", "_")) + ".png"
}

func (g *Game) ensureMiniImageByName(name string) *ebiten.Image {
	g.assets.ensureInit()
	key := g.portraitKeyFor(name)
	if img, ok := g.assets.minis[key]; ok {
		return img
	}
	img := loadImage("assets/minis/" + key)
	g.assets.minis[key] = img
	return img
}

func (g *Game) ensureObstacleImage(obstacleType string) *ebiten.Image {
	g.assets.ensureInit()
	if img, ok := g.assets.obstacles[obstacleType]; ok {
		return img
	}
	img := loadImage("assets/obstacles/" + obstacleType + ".png")
	g.assets.obstacles[obstacleType] = img
	return img
}

func (g *Game) ensureBgForMap(mapID string) *ebiten.Image {
	g.assets.ensureInit()
	if img, ok := g.assets.bg[mapID]; ok {
		return img
	}

	base := strings.ToLower(mapID)
	for _, p := range []string{
		"assets/maps/" + base + ".png",
		"assets/maps/" + base + ".jpg",
	} {
		if img := loadImage(p); img != nil {
			g.assets.bg[mapID] = img
			return img
		}
	}

	log.Println("map background not found for mapID:", mapID)
	g.assets.bg[mapID] = nil
	return nil
}

// drawSmallLevelBadgeSized draws the level inside level_bar.png (if present) at a given pixel height.
func drawSmallLevelBadgeSized(dst *ebiten.Image, x, y, level int, size int) {
	// center for text
	cx, cy := x+size/2, y+size/2
	if ornate == nil {
		// Build ornate bar from embedded assets (no disk dependency)
		ob := &ui.OrnateBar{}
		// Try primary names shipped in repo
		ob.Frame = loadImage("assets/ui/health_bar.png")
		if ob.Frame == nil {
			ob.Frame = loadImage("assets/ui/bar_frame.png")
		}
		ob.Badge = loadImage("assets/ui/level_bar.png")
		if ob.Badge == nil {
			ob.Badge = loadImage("assets/ui/level_badge.png")
		}
		// Defaults reasonable for our images; can be tuned via meta later
		ob.WellOfs = image.Pt(130, 30)
		ob.WellSize = image.Pt(350, 44)
		ob.Mode = "fitpad"
		ob.PadX, ob.PadY = 2, 2
		ob.BadgeScale = 1.0
		ornate = ob
	}
	if ornate != nil && ornate.Badge != nil {
		bb := ornate.Badge.Bounds()
		if bb.Dx() > 0 && bb.Dy() > 0 {
			s := float64(size) / float64(bb.Dy())
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(float64(x), float64(y))
			dst.DrawImage(ornate.Badge, op)
		}
	} else {
		// fallback: small circle
		vector.DrawFilledCircle(dst, float32(cx), float32(cy), float32(size)/2, color.NRGBA{240, 196, 25, 255}, true)
		vector.DrawFilledCircle(dst, float32(cx), float32(cy), float32(size)/2-1.2, color.NRGBA{200, 160, 20, 255}, true)
	}
	// level number with outline
	s := fmt.Sprintf("%d", level)
	tw := text.BoundString(basicfont.Face7x13, s).Dx()
	th := text.BoundString(basicfont.Face7x13, s).Dy()
	tx := cx - tw/2
	ty := cy + th/2 - 2
	text.Draw(dst, s, basicfont.Face7x13, tx+1, ty, color.NRGBA{0, 0, 0, 200})
	text.Draw(dst, s, basicfont.Face7x13, tx-1, ty, color.NRGBA{0, 0, 0, 200})
	text.Draw(dst, s, basicfont.Face7x13, tx, ty+1, color.NRGBA{0, 0, 0, 200})
	text.Draw(dst, s, basicfont.Face7x13, tx, ty-1, color.NRGBA{0, 0, 0, 200})
	text.Draw(dst, s, basicfont.Face7x13, tx, ty, color.NRGBA{250, 250, 250, 255})
}

// default Army/overlay size
func drawSmallLevelBadge(dst *ebiten.Image, x, y, level int) {
	drawSmallLevelBadgeSized(dst, x, y, level, 14)
}

func drawBaseImg(screen *ebiten.Image, img *ebiten.Image, b protocol.BaseState) {
	if img == nil {

		ebitenutil.DrawRect(screen, float64(b.X), float64(b.Y), float64(b.W), float64(b.H), color.NRGBA{90, 90, 120, 255})
		return
	}
	iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
	if iw == 0 || ih == 0 {
		return
	}

	sx := float64(b.W) / float64(iw)
	sy := float64(b.H) / float64(ih)
	s := math.Min(sx, sy)

	op := &ebiten.DrawImageOptions{}
	ox := float64(b.X) + (float64(b.W)-float64(iw)*s)/2
	oy := float64(b.Y) + (float64(b.H)-float64(ih)*s)/2
	op.GeoM.Scale(s, s)
	op.GeoM.Translate(ox, oy)
	// Use linear filtering for smoother scaling on high-resolution displays
	op.Filter = ebiten.FilterLinear
	screen.DrawImage(img, op)
}
