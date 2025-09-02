package game

import (
    "rumble/shared/protocol"
    "strings"

    "github.com/hajimehoshi/ebiten/v2"
)

func (g *Game) ensureArmyBgLayer() {
    if g.armyBg == nil { g.armyBg = loadImage("assets/ui/army_bg.png") }
    if g.armyBg == nil {
        return
    }

	sw, sh := protocol.ScreenW, protocol.ScreenH
	viewW, viewH := sw, sh-topBarH-menuBarH
	if viewW < 1 {
		viewW = 1
	}
	if viewH < 1 {
		viewH = 1
	}

	if g.armyBgLayer == nil || g.armyBgLayer.Bounds().Dx() != viewW || g.armyBgLayer.Bounds().Dy() != viewH {
		g.armyBgLayer = ebiten.NewImage(viewW, viewH)
	}
	g.armyBgLayer.Clear()

	iw, ih := g.armyBg.Bounds().Dx(), g.armyBg.Bounds().Dy()
	scaleX := float64(viewW) / float64(iw)
	scaleY := float64(viewH) / float64(ih)
	scale := scaleX
	if scaleY > scale {
		scale = scaleY
	}

	w := float64(iw) * scale
	h := float64(ih) * scale
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(scale, scale)
	op.GeoM.Translate((float64(viewW)-w)/2, (float64(viewH)-h)/2)
	g.armyBgLayer.DrawImage(g.armyBg, op)
}

func (g *Game) rowsPerCol() int {
	contentTop := pad + 40
	contentH := protocol.ScreenH - menuBarH - pad - contentTop
	return maxInt(1, contentH/rowH)
}

func (g *Game) trySaveArmy() {
	names := g.buildArmyNames()
	if msg := g.validateArmy(names); msg != "" {
		g.armyMsg = msg
		return
	}
	g.armyMsg = "Saved!"
	g.onArmySave(names)
}

func (g *Game) buildArmyNames() []string {
    out := make([]string, 0, 7)
    if g.selectedChampion != "" {
        out = append(out, g.selectedChampion)
    }
    // use explicit slot order when available
    count := 0
    for i := 0; i < 6; i++ {
        if g.selectedOrder[i] != "" {
            out = append(out, g.selectedOrder[i])
            count++
        }
    }
    if count < 6 {
        // fallback to map order to fill
        for _, m := range g.minisOnly {
            if g.selectedMinis[m.Name] {
                // ensure not already included
                already := false
                for _, n := range out[1:] { if n == m.Name { already = true; break } }
                if !already {
                    out = append(out, m.Name)
                    count++
                    if count == 6 { break }
                }
            }
        }
    }
    return out
}

func (g *Game) validateArmy(names []string) string {
	if len(names) != 7 {
		return "Select exactly 1 Champion and 6 Minis."
	}
	info := map[string]protocol.MiniInfo{}
	for _, m := range g.minisAll {
		info[m.Name] = m
	}

	first := info[names[0]]
	if !(strings.EqualFold(first.Role, "champion") || strings.EqualFold(first.Class, "champion")) {
		return "First card must be a Champion."
	}

	for i := 1; i < 7; i++ {
		m := info[names[i]]
		if !strings.EqualFold(m.Role, "mini") || strings.EqualFold(m.Class, "spell") {
			return "Slots 2..7 must be Minis (non-spell)."
		}
	}
	return ""
}

func (g *Game) selectedMinisList() []string {
    // Return explicit slot order if defined
    out := make([]string, 0, 6)
    hasAny := false
    for i := 0; i < 6; i++ {
        if g.selectedOrder[i] != "" { hasAny = true; break }
    }
    if hasAny {
        for i := 0; i < 6; i++ { if g.selectedOrder[i] != "" { out = append(out, g.selectedOrder[i]) } }
        return out
    }
    // Fallback: derive from map order
    for _, m := range g.minisOnly {
        if g.selectedMinis[m.Name] {
            out = append(out, m.Name)
            if len(out) == 6 { break }
        }
    }
    return out
}

func (g *Game) setChampArmyFromSelected() {
	if g.selectedChampion == "" {
		return
	}
	if g.champToMinis[g.selectedChampion] == nil {
		g.champToMinis[g.selectedChampion] = map[string]bool{}
	}

    dst := map[string]bool{}
    for k := range g.selectedMinis {
        dst[k] = true
    }
    g.champToMinis[g.selectedChampion] = dst
    // also persist order
    if g.champToOrder == nil { g.champToOrder = map[string][6]string{} }
    g.champToOrder[g.selectedChampion] = g.selectedOrder
}

func (g *Game) loadSelectedForChampion(name string) {
    g.selectedChampion = name

    g.selectedMinis = map[string]bool{}
    if set, ok := g.champToMinis[name]; ok { for k := range set { g.selectedMinis[k] = true } }
    // load slot order if available
    g.selectedOrder = [6]string{}
    if ord, ok := g.champToOrder[name]; ok {
        g.selectedOrder = ord
    } else {
        // derive initial order from map order
        list := g.selectedMinisList()
        for i := 0; i < len(list) && i < 6; i++ { g.selectedOrder[i] = list[i] }
    }
}

func (g *Game) autoSaveCurrentChampionArmy() {

	if g.selectedChampion == "" {
		return
	}
    minis := make([]string, 0, 6)
    for i := 0; i < 6; i++ { if g.selectedOrder[i] != "" { minis = append(minis, g.selectedOrder[i]) } }
    if len(minis) != 6 {
        return
    }
    names := append([]string{g.selectedChampion}, minis...)
	if msg := g.validateArmy(names); msg != "" {
		return
	}

	g.onArmySave(names)
}

func (g *Game) beginChampDrag(px int) {
	g.champDragActive = true
	g.champDragStartX = px
	g.champDragLastX = px
	g.champDragAccum = 0
}

func (g *Game) moveChampDrag(px int, stepPx int, maxStart int) {
	dx := px - g.champDragLastX
	g.champDragLastX = px
	g.champDragAccum += dx

	for g.champDragAccum <= -stepPx && g.champStripScroll < maxStart {
		g.champStripScroll++
		g.champDragAccum += stepPx
	}
	for g.champDragAccum >= stepPx && g.champStripScroll > 0 {
		g.champStripScroll--
		g.champDragAccum -= stepPx
	}
}

func (g *Game) endChampDrag() {
	g.champDragActive = false
	g.activeTouchID = -1
	g.champDragAccum = 0
}
