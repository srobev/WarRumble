package game

import (
	"fmt"
	"image/color"
	_ "image/jpeg" // (optional) register JPEG decoder
	_ "image/png"  // register PNG decoder"
	"strings"
	"time"

	"rumble/client/internal/game/assets/fonts"
	"rumble/shared/protocol"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.org/x/image/font/basicfont"
)

// ---------- Home (Army / Map tabs) ----------

func (g *Game) drawHomeContent(screen *ebiten.Image) {
	switch g.activeTab {
	case tabArmy:
		// Draw map background first
		g.ensureArmyBgLayer()
		if g.armyBgLayer != nil {
			var op ebiten.DrawImageOptions
			op.GeoM.Translate(0, float64(topBarH))
			screen.DrawImage(g.armyBgLayer, &op)
		}

		// Layout numbers (mirror Update)
		const champCardW, champCardH = 100, 116
		const miniCardW, miniCardH = 100, 116
		const gap = 10
		const stripH = champCardH + 8

		stripX := pad
		stripY := topBarH + pad
		stripW := protocol.ScreenW - 2*pad
		g.champStripArea = rect{x: stripX, y: stripY, w: stripW, h: stripH}

		cols := maxInt(1, (stripW+gap)/(champCardW+gap))
		start := clampInt(g.champStripScroll, 0, maxInt(0, len(g.champions)-cols))
		// Center the visible strip block
		visW := cols*champCardW + maxInt(0, cols-1)*gap
		baseX := stripX + (stripW-visW)/2
		g.champStripRects = g.champStripRects[:0]
		for i := 0; i < cols && start+i < len(g.champions); i++ {
			x := baseX + i*(champCardW+gap)
			r := rect{x: x, y: stripY, w: champCardW, h: champCardH}
			g.champStripRects = append(g.champStripRects, r)

			it := g.champions[start+i]

			// Use FantasyUI themed card (remove cost display)
			if g.fantasyUI != nil {
				isSelected := g.selectedChampion == it.Name
				isHovered := g.hoveredChampionCard >= 0 && g.hoveredChampionCard == start+i

				g.fantasyUI.DrawEnhancedUnitCard(screen, r.x, r.y, r.w, r.h,
					it.Name, strings.Title(it.Class), 1, 0, nil, isSelected, isHovered)
			} else {
				// Fallback to basic styling
				ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{0x2b, 0x2b, 0x3e, 0xff})
			}

			// Cost display in top-right for champion strip
			if it.Cost > 0 {
				costStr := fmt.Sprintf("%d", it.Cost)
				costW := len(costStr) * 7
				costX := r.x + r.w - costW - 8
				costY := r.y + 8

				// Cost badge with gold theme
				vector.DrawFilledRect(screen, float32(costX-4), float32(costY-2), float32(costW+8), 14, color.NRGBA{255, 215, 0, 255}, true)
				vector.StrokeRect(screen, float32(costX-4), float32(costY-2), float32(costW+8), 14, 1, color.NRGBA{200, 160, 20, 255}, true)
				text.Draw(screen, costStr, fonts.Face(13), costX, costY+10, color.NRGBA{64, 64, 64, 255})
			}

			if img := g.ensureMiniImageByName(it.Name); img != nil {
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
				pw, ph := float64(r.w-8), float64(r.h-24)
				s := mathMin(pw/float64(iw), ph/float64(ih))
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(s, s)
				op.GeoM.Translate(float64(r.x)+4, float64(r.y)+4)
				op.Filter = ebiten.FilterLinear // High-quality filtering
				screen.DrawImage(img, op)
			}

			// Level bottom-left (moved from top-left)
			lvl := 1
			if g.unitXP != nil {
				if xp, ok := g.unitXP[it.Name]; ok {
					l, _, _ := computeLevel(xp)
					if l > 0 {
						lvl = l
					}
				}
			}
			levelBadgeX := r.x + 8
			levelBadgeY := r.y + r.h - 20

			// Level badge with purple theme (same size as gold cost badges)
			if lvl > 0 {
				levelStr := fmt.Sprintf("%d", lvl)
				levelW := len(levelStr) * 7
				vector.DrawFilledRect(screen, float32(levelBadgeX-4), float32(levelBadgeY-2), float32(levelW+8), 14, color.NRGBA{138, 43, 226, 200}, true)
				vector.StrokeRect(screen, float32(levelBadgeX-4), float32(levelBadgeY-2), float32(levelW+8), 14, 1, color.NRGBA{100, 20, 150, 255}, true)
				text.Draw(screen, levelStr, fonts.Face(13), levelBadgeX, levelBadgeY+10, color.NRGBA{255, 255, 255, 255})
			}

			// Selection indicator with theme color
			if g.selectedChampion == it.Name {
				selectionColor := color.NRGBA{240, 196, 25, 255}
				if g.fantasyUI != nil {
					selectionColor = g.fantasyUI.Theme.Secondary
				}
				ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), 2, selectionColor)
			}
		}

		topY := stripY + stripH + 12
		bigW := 200
		bigH := miniCardH*2 + gap
		// Center champion card + 2x3 grid as a block
		totalW := bigW + 16 + 3*miniCardW + 2*gap
		startX := (protocol.ScreenW - totalW) / 2
		leftX := startX

		chRect := rect{x: leftX, y: topY, w: bigW, h: bigH}
		if g.selectedChampion != "" {
			// Get champion cost from champion data
			championCost := 0
			for _, champ := range g.champions {
				if champ.Name == g.selectedChampion {
					championCost = champ.Cost
					break
				}
			}

			// Use FantasyUI themed card for selected champion
			if g.fantasyUI != nil {
				g.fantasyUI.DrawEnhancedUnitCard(screen, chRect.x, chRect.y, chRect.w, chRect.h,
					g.selectedChampion, "Champion", 1, 0, nil, true, g.hoveredSelectedChampionCard)
			} else {
				ebitenutil.DrawRect(screen, float64(chRect.x), float64(chRect.y), float64(chRect.w), float64(chRect.h), color.NRGBA{0x2b, 0x2b, 0x3e, 0xff})
			}

			if img := g.ensureMiniImageByName(g.selectedChampion); img != nil {
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
				pw, ph := float64(chRect.w-8), float64(chRect.h-24)
				s := mathMin(pw/float64(iw), ph/float64(ih))
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(s, s)
				op.GeoM.Translate(float64(chRect.x)+4, float64(chRect.y)+4)
				op.Filter = ebiten.FilterLinear // High-quality filtering
				screen.DrawImage(img, op)
			}

			// Cost display in top-right
			if championCost > 0 {
				costStr := fmt.Sprintf("%d", championCost)
				costW := len(costStr) * 7
				costX := chRect.x + chRect.w - costW - 8
				costY := chRect.y + 8

				// Cost badge with gold theme
				vector.DrawFilledRect(screen, float32(costX-4), float32(costY-2), float32(costW+8), 14, color.NRGBA{255, 215, 0, 255}, true)
				vector.StrokeRect(screen, float32(costX-4), float32(costY-2), float32(costW+8), 14, 1, color.NRGBA{200, 160, 20, 255}, true)
				text.Draw(screen, costStr, basicfont.Face7x13, costX, costY+10, color.NRGBA{64, 64, 64, 255})
			}

			// Level and XP bar at bottom-left
			lvl := 1
			xp := 0
			if g.unitXP != nil {
				if xpVal, ok := g.unitXP[g.selectedChampion]; ok {
					xp = xpVal
					if l, _, _ := computeLevel(xp); l > 0 {
						lvl = l
					}
				}
			}

			// Level badge bottom-left
			levelBadgeX := chRect.x + 8
			levelBadgeY := chRect.y + chRect.h - 20

			// Level badge with purple theme (same size as gold cost badges)
			if lvl > 0 {
				levelStr := fmt.Sprintf("%d", lvl)
				levelW := len(levelStr) * 7
				vector.DrawFilledRect(screen, float32(levelBadgeX-4), float32(levelBadgeY-2), float32(levelW+8), 14, color.NRGBA{138, 43, 226, 200}, true)
				vector.StrokeRect(screen, float32(levelBadgeX-4), float32(levelBadgeY-2), float32(levelW+8), 14, 1, color.NRGBA{100, 20, 150, 255}, true)
				text.Draw(screen, levelStr, basicfont.Face7x13, levelBadgeX, levelBadgeY+10, color.NRGBA{255, 255, 255, 255})
			}

			// XP bar next to level badge (centered vertically with level badge)
			xpBarX := levelBadgeX + 28
			xpBarY := levelBadgeY + 3 // Center with level badge (level badge is 14px, so +3 centers the 8px bar)
			xpBarW := chRect.w - 48   // Leave space for level badge and margins
			xpBarH := 8

			// XP bar background
			vector.DrawFilledRect(screen, float32(xpBarX), float32(xpBarY), float32(xpBarW), float32(xpBarH), color.NRGBA{32, 34, 48, 200}, true)

			// XP bar fill (purple, same as unit profile overlay)
			if xp > 0 {
				_, cur, next := computeLevel(xp)
				if next > 0 {
					frac := float64(cur) / float64(next)
					fillW := int(float64(xpBarW) * frac)
					if fillW > 0 {
						vector.DrawFilledRect(screen, float32(xpBarX), float32(xpBarY), float32(fillW), float32(xpBarH), color.NRGBA{138, 43, 226, 200}, true)
					}
				}
			}

			// XP bar border
			vector.StrokeRect(screen, float32(xpBarX), float32(xpBarY), float32(xpBarW), float32(xpBarH), 1, color.NRGBA{100, 90, 0, 255}, true)

		} else {
			// Empty champion slot with themed styling
			if g.fantasyUI != nil {
				g.fantasyUI.DrawThemedCard(screen, chRect.x, chRect.y, chRect.w, chRect.h,
					"", []string{"Select a champion from above"})
			} else {
				ebitenutil.DrawRect(screen, float64(chRect.x), float64(chRect.y), float64(chRect.w), float64(chRect.h), color.NRGBA{0x2b, 0x2b, 0x3e, 0xff})
				text.Draw(screen, "Champion (select above)", basicfont.Face7x13, chRect.x+6, chRect.y+18, color.NRGBA{200, 200, 200, 255})
			}
		}

		gridX := leftX + bigW + 16
		gridY := topY
		k := 0
		for row := 0; row < 2; row++ {
			for col := 0; col < 3; col++ {
				r := rect{
					x: gridX + col*(miniCardW+gap),
					y: gridY + row*(miniCardH+gap),
					w: miniCardW, h: miniCardH,
				}

				if k < 6 && g.selectedOrder[k] != "" {
					name := g.selectedOrder[k]

					// Get mini cost from mini data
					miniCost := 0
					if info, ok := g.nameToMini[name]; ok {
						miniCost = info.Cost
					}

					// Use FantasyUI themed card for equipped minis
					if g.fantasyUI != nil {
						isHovered := g.hoveredMiniSlotCard >= 0 && g.hoveredMiniSlotCard == k

						g.fantasyUI.DrawEnhancedUnitCard(screen, r.x, r.y, r.w, r.h,
							name, "Mini", 1, 0, nil, false, isHovered)
					} else {
						ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{0x26, 0x26, 0x35, 0xff})
					}

					if img := g.ensureMiniImageByName(name); img != nil {
						iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
						pw, ph := float64(r.w-8), float64(r.h-24)
						s := mathMin(pw/float64(iw), ph/float64(ih))
						op := &ebiten.DrawImageOptions{}
						op.GeoM.Scale(s, s)
						op.GeoM.Translate(float64(r.x)+4, float64(r.y)+4)
						op.Filter = ebiten.FilterLinear // High-quality filtering
						screen.DrawImage(img, op)
					}

					// Cost display in top-right for equipped minis
					if miniCost > 0 {
						costStr := fmt.Sprintf("%d", miniCost)
						costW := len(costStr) * 7
						costX := r.x + r.w - costW - 8
						costY := r.y + 8

						// Cost badge with gold theme
						vector.DrawFilledRect(screen, float32(costX-4), float32(costY-2), float32(costW+8), 14, color.NRGBA{255, 215, 0, 255}, true)
						vector.StrokeRect(screen, float32(costX-4), float32(costY-2), float32(costW+8), 14, 1, color.NRGBA{200, 160, 20, 255}, true)
						text.Draw(screen, costStr, basicfont.Face7x13, costX, costY+10, color.NRGBA{64, 64, 64, 255})
					}

					// Level and XP bar at bottom-left
					lvl := 1
					xp := 0
					if g.unitXP != nil {
						if xpVal, ok := g.unitXP[name]; ok {
							xp = xpVal
							if l, _, _ := computeLevel(xp); l > 0 {
								lvl = l
							}
						}
					}

					// Level badge bottom-left
					levelBadgeX := r.x + 8
					levelBadgeY := r.y + r.h - 20

					// Level badge with purple theme (same size as gold cost badges)
					if lvl > 0 {
						levelStr := fmt.Sprintf("%d", lvl)
						levelW := len(levelStr) * 7
						vector.DrawFilledRect(screen, float32(levelBadgeX-4), float32(levelBadgeY-2), float32(levelW+8), 14, color.NRGBA{138, 43, 226, 200}, true)
						vector.StrokeRect(screen, float32(levelBadgeX-4), float32(levelBadgeY-2), float32(levelW+8), 14, 1, color.NRGBA{100, 20, 150, 255}, true)
						text.Draw(screen, levelStr, basicfont.Face7x13, levelBadgeX, levelBadgeY+10, color.NRGBA{255, 255, 255, 255})
					}

					// XP bar next to level badge (centered vertically with level badge)
					xpBarX := levelBadgeX + 28
					xpBarY := levelBadgeY + 3 // Center with level badge (level badge is 14px, so +3 centers the 8px bar)
					xpBarW := r.w - 48        // Leave space for level badge and margins
					xpBarH := 8

					// XP bar background
					vector.DrawFilledRect(screen, float32(xpBarX), float32(xpBarY), float32(xpBarW), float32(xpBarH), color.NRGBA{32, 34, 48, 200}, true)

					// XP bar fill (purple, same as unit profile overlay)
					if xp > 0 {
						_, cur, next := computeLevel(xp)
						if next > 0 {
							frac := float64(cur) / float64(next)
							fillW := int(float64(xpBarW) * frac)
							if fillW > 0 {
								vector.DrawFilledRect(screen, float32(xpBarX), float32(xpBarY), float32(fillW), float32(xpBarH), color.NRGBA{138, 43, 226, 200}, true)
							}
						}
					}

					// XP bar border
					vector.StrokeRect(screen, float32(xpBarX), float32(xpBarY), float32(xpBarW), float32(xpBarH), 1, color.NRGBA{100, 90, 0, 255}, true)
				} else {
					// Empty mini slot with themed styling
					if g.fantasyUI != nil {
						g.fantasyUI.DrawThemedCard(screen, r.x, r.y, r.w, r.h,
							"", []string{"Empty slot"})
					} else {
						ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{0x26, 0x26, 0x35, 0xff})
						text.Draw(screen, "Mini", basicfont.Face7x13, r.x+6, r.y+14, color.NRGBA{200, 200, 200, 255})
						text.Draw(screen, "(empty)", basicfont.Face7x13, r.x+6, r.y+r.h-6, color.NRGBA{160, 160, 160, 255})
					}
				}
				k++
			}
		}
		// count equipped
		cnt := 0
		for i := 0; i < 6; i++ {
			if g.selectedOrder[i] != "" {
				cnt++
			}
		}
		text.Draw(screen, fmt.Sprintf("Minis: %d/6", cnt), fonts.Face(13), gridX, gridY-6, color.White)

		gridTop := topY + bigH + 16
		// Center the collection grid rows
		maxW := protocol.ScreenW - 2*pad
		cols2 := maxInt(1, (maxW+gap)/(miniCardW+gap))
		contentW := cols2*miniCardW + maxInt(0, cols2-1)*gap
		gridLeft := (protocol.ScreenW - contentW) / 2
		gridW := contentW
		gridH := protocol.ScreenH - menuBarH - pad - gridTop
		visRows := maxInt(1, (gridH+gap)/(miniCardH+gap))
		g.collArea = rect{x: gridLeft, y: gridTop, w: gridW, h: gridH}

		start2 := g.collScroll * cols2
		// Build available (non-equipped) minis list
		avail := make([]protocol.MiniInfo, 0, len(g.minisOnly))
		for _, mi := range g.minisOnly {
			if !g.selectedMinis[mi.Name] {
				avail = append(avail, mi)
			}
		}
		g.collRects = g.collRects[:0]
		maxItems := visRows * cols2
		for i := 0; i < maxItems && start2+i < len(avail); i++ {
			c := i % cols2
			rw := i / cols2
			x := gridLeft + c*(miniCardW+gap)
			y := gridTop + rw*(miniCardH+gap)
			r := rect{x: x, y: y, w: miniCardW, h: miniCardH}
			g.collRects = append(g.collRects, r)

			it := avail[start2+i]

			// Use FantasyUI themed card for collection items
			if g.fantasyUI != nil {
				isSelected := g.selectedMinis[it.Name]
				isHovered := g.hoveredCollectionCard >= 0 && g.hoveredCollectionCard == start2+i

				g.fantasyUI.DrawEnhancedUnitCard(screen, r.x, r.y, r.w, r.h,
					it.Name, strings.Title(it.Class), 1, 0, nil, isSelected, isHovered)
			} else {
				// Fallback to basic styling
				ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{0x2b, 0x2b, 0x3e, 0xff})
			}

			if img := g.ensureMiniImageByName(it.Name); img != nil {
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
				pw, ph := float64(r.w-8), float64(r.h-24)
				s := mathMin(pw/float64(iw), ph/float64(ih))
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(s, s)
				op.GeoM.Translate(float64(r.x)+4, float64(r.y)+4)
				op.Filter = ebiten.FilterLinear // High-quality filtering
				screen.DrawImage(img, op)
			}

			// Cost display in top-right for collection items
			if it.Cost > 0 {
				costStr := fmt.Sprintf("%d", it.Cost)
				costW := len(costStr) * 7
				costX := r.x + r.w - costW - 8
				costY := r.y + 8

				// Cost badge with gold theme
				vector.DrawFilledRect(screen, float32(costX-4), float32(costY-2), float32(costW+8), 14, color.NRGBA{255, 215, 0, 255}, true)
				vector.StrokeRect(screen, float32(costX-4), float32(costY-2), float32(costW+8), 14, 1, color.NRGBA{200, 160, 20, 255}, true)
				text.Draw(screen, costStr, basicfont.Face7x13, costX, costY+10, color.NRGBA{64, 64, 64, 255})
			}

			// Level badge at bottom-left for collection items
			lvl := 1
			if g.unitXP != nil {
				if xp, ok := g.unitXP[it.Name]; ok {
					l, _, _ := computeLevel(xp)
					if l > 0 {
						lvl = l
					}
				}
			}
			levelBadgeX := r.x + 8
			levelBadgeY := r.y + r.h - 20

			// Level badge with purple theme (same size as gold cost badges)
			if lvl > 0 {
				levelStr := fmt.Sprintf("%d", lvl)
				levelW := len(levelStr) * 7
				vector.DrawFilledRect(screen, float32(levelBadgeX-4), float32(levelBadgeY-2), float32(levelW+8), 14, color.NRGBA{138, 43, 226, 200}, true)
				vector.StrokeRect(screen, float32(levelBadgeX-4), float32(levelBadgeY-2), float32(levelW+8), 14, 1, color.NRGBA{100, 20, 150, 255}, true)
				text.Draw(screen, levelStr, basicfont.Face7x13, levelBadgeX, levelBadgeY+10, color.NRGBA{255, 255, 255, 255})
			}

			// XP bar at bottom-center (adjusted for level badge)
			xp := 0
			if g.unitXP != nil {
				if xpVal, ok := g.unitXP[it.Name]; ok {
					xp = xpVal
				}
			}

			// XP bar centered at bottom, accounting for level badge
			xpBarW := r.w - 48 // Leave space for level badge on left
			xpBarH := 8
			xpBarX := r.x + 36       // Start after level badge
			xpBarY := r.y + r.h - 15 // Center with level badge (level badge is 14px, so +3 centers the 8px bar)

			// XP bar background
			vector.DrawFilledRect(screen, float32(xpBarX), float32(xpBarY), float32(xpBarW), float32(xpBarH), color.NRGBA{32, 34, 48, 200}, true)

			// XP bar fill (purple, same as unit profile overlay)
			if xp > 0 {
				_, cur, next := computeLevel(xp)
				if next > 0 {
					frac := float64(cur) / float64(next)
					fillW := int(float64(xpBarW) * frac)
					if fillW > 0 {
						vector.DrawFilledRect(screen, float32(xpBarX), float32(xpBarY), float32(fillW), float32(xpBarH), color.NRGBA{138, 43, 226, 200}, true)
					}
				}
			}

			// XP bar border
			vector.StrokeRect(screen, float32(xpBarX), float32(xpBarY), float32(xpBarW), float32(xpBarH), 1, color.NRGBA{100, 90, 0, 255}, true)

			// Selection indicator with theme color
			if g.selectedMinis[it.Name] {
				selectionColor := color.NRGBA{240, 196, 25, 255}
				if g.fantasyUI != nil {
					selectionColor = g.fantasyUI.Theme.Secondary
				}
				ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), 2, selectionColor)
			}
		}

		// Draw hover tooltips for champion cards
		mx, my := ebiten.CursorPosition()

		// Draw tooltips for champion strip (top section)
		for i, r := range g.champStripRects {
			if r.hit(mx, my) {
				idx := start + i
				if idx >= 0 && idx < len(g.champions) {
					it := g.champions[idx]

					// Level tooltip
					levelBadgeRect := rect{x: r.x + 8, y: r.y + r.h - 20, w: 20, h: 20}
					if levelBadgeRect.hit(mx, my) {
						lvl := 1
						if g.unitXP != nil {
							if xp, ok := g.unitXP[it.Name]; ok {
								l, _, _ := computeLevel(xp)
								if l > 0 {
									lvl = l
								}
							}
						}

						tooltipText := fmt.Sprintf("Level %d", lvl)
						tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
						th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

						// Position tooltip above the level badge
						tooltipX := mx - tw/2
						tooltipY := my - th - 8

						// Keep tooltip on screen
						if tooltipX < 4 {
							tooltipX = 4
						}
						if tooltipX+tw+8 > protocol.ScreenW {
							tooltipX = protocol.ScreenW - tw - 8
						}
						if tooltipY < 4 {
							tooltipY = my + 16
						}

						// Draw tooltip background
						ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

						// Draw tooltip text
						text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.White)
					}

					// Cost tooltip
					costS := fmt.Sprintf("%d", it.Cost)
					cw := text.BoundString(basicfont.Face7x13, costS).Dx()
					costRect := rect{x: r.x + r.w - cw - 8, y: r.y + 8, w: cw, h: 14}
					if costRect.hit(mx, my) {
						tooltipText := fmt.Sprintf("Deploy cost %d", it.Cost)
						tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
						th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

						// Position tooltip above the cost text
						tooltipX := mx - tw/2
						tooltipY := my - th - 8

						// Keep tooltip on screen
						if tooltipX < 4 {
							tooltipX = 4
						}
						if tooltipX+tw+8 > protocol.ScreenW {
							tooltipX = protocol.ScreenW - tw - 8
						}
						if tooltipY < 4 {
							tooltipY = my + 16
						}

						// Draw tooltip background
						ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

						// Draw tooltip text
						text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.NRGBA{255, 215, 0, 255}) // Gold color for cost
					}
				}
				break
			}
		}

		// Draw tooltips for selected champion
		if g.hoveredSelectedChampionLevel && g.selectedChampion != "" {
			lvl := 1
			if g.unitXP != nil {
				if xp, ok := g.unitXP[g.selectedChampion]; ok {
					l, _, _ := computeLevel(xp)
					if l > 0 {
						lvl = l
					}
				}
			}

			tooltipText := fmt.Sprintf("Level %d", lvl)
			tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
			th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

			// Position tooltip above the level badge
			tooltipX := mx - tw/2
			tooltipY := my - th - 8

			// Keep tooltip on screen
			if tooltipX < 4 {
				tooltipX = 4
			}
			if tooltipX+tw+8 > protocol.ScreenW {
				tooltipX = protocol.ScreenW - tw - 8
			}
			if tooltipY < 4 {
				tooltipY = my + 16
			}

			// Draw tooltip background
			ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

			// Draw tooltip text
			text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.White)
		}

		// Draw XP bar tooltips for selected champion
		if g.hoveredSelectedChampionXP && g.selectedChampion != "" {
			xp := 0
			if g.unitXP != nil {
				if xpVal, ok := g.unitXP[g.selectedChampion]; ok {
					xp = xpVal
				}
			}
			_, cur, next := computeLevel(xp)

			tooltipText := fmt.Sprintf("%d/%d", cur, next)
			tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
			th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

			// Position tooltip above the XP bar
			tooltipX := mx - tw/2
			tooltipY := my - th - 8

			// Keep tooltip on screen
			if tooltipX < 4 {
				tooltipX = 4
			}
			if tooltipX+tw+8 > protocol.ScreenW {
				tooltipX = protocol.ScreenW - tw - 8
			}
			if tooltipY < 4 {
				tooltipY = my + 16
			}

			// Draw tooltip background
			ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

			// Draw tooltip text
			text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.White)
		}

		if g.hoveredSelectedChampionCost && g.selectedChampion != "" {
			if info, ok := g.nameToMini[g.selectedChampion]; ok {
				tooltipText := fmt.Sprintf("Deploy cost %d", info.Cost)
				tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
				th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

				// Position tooltip above the cost text
				tooltipX := mx - tw/2
				tooltipY := my - th - 8

				// Keep tooltip on screen
				if tooltipX < 4 {
					tooltipX = 4
				}
				if tooltipX+tw+8 > protocol.ScreenW {
					tooltipX = protocol.ScreenW - tw - 8
				}
				if tooltipY < 4 {
					tooltipY = my + 16
				}

				// Draw tooltip background
				ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

				// Draw tooltip text
				text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.NRGBA{255, 215, 0, 255}) // Gold color for cost
			}
		}

		// Draw tooltips for equipped mini slots
		if g.hoveredMiniSlotLevel >= 0 && g.hoveredMiniSlotLevel < len(g.selectedOrder) && g.selectedOrder[g.hoveredMiniSlotLevel] != "" {
			name := g.selectedOrder[g.hoveredMiniSlotLevel]
			lvl := 1
			if g.unitXP != nil {
				if xp, ok := g.unitXP[name]; ok {
					l, _, _ := computeLevel(xp)
					if l > 0 {
						lvl = l
					}
				}
			}

			tooltipText := fmt.Sprintf("Level %d", lvl)
			tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
			th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

			// Position tooltip above the level badge
			tooltipX := mx - tw/2
			tooltipY := my - th - 8

			// Keep tooltip on screen
			if tooltipX < 4 {
				tooltipX = 4
			}
			if tooltipX+tw+8 > protocol.ScreenW {
				tooltipX = protocol.ScreenW - tw - 8
			}
			if tooltipY < 4 {
				tooltipY = my + 16
			}

			// Draw tooltip background
			ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

			// Draw tooltip text
			text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.White)
		}

		// Draw XP bar tooltips for equipped mini slots
		if g.hoveredMiniSlotXP >= 0 && g.hoveredMiniSlotXP < len(g.selectedOrder) && g.selectedOrder[g.hoveredMiniSlotXP] != "" {
			name := g.selectedOrder[g.hoveredMiniSlotXP]
			xp := 0
			if g.unitXP != nil {
				if xpVal, ok := g.unitXP[name]; ok {
					xp = xpVal
				}
			}
			_, cur, next := computeLevel(xp)

			tooltipText := fmt.Sprintf("%d/%d", cur, next)
			tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
			th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

			// Position tooltip above the XP bar
			tooltipX := mx - tw/2
			tooltipY := my - th - 8

			// Keep tooltip on screen
			if tooltipX < 4 {
				tooltipX = 4
			}
			if tooltipX+tw+8 > protocol.ScreenW {
				tooltipX = protocol.ScreenW - tw - 8
			}
			if tooltipY < 4 {
				tooltipY = my + 16
			}

			// Draw tooltip background
			ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

			// Draw tooltip text
			text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.White)
		}

		if g.hoveredMiniSlotCost >= 0 && g.hoveredMiniSlotCost < len(g.selectedOrder) && g.selectedOrder[g.hoveredMiniSlotCost] != "" {
			name := g.selectedOrder[g.hoveredMiniSlotCost]
			if info, ok := g.nameToMini[name]; ok {
				tooltipText := fmt.Sprintf("Deploy cost %d", info.Cost)
				tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
				th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

				// Position tooltip above the cost text
				tooltipX := mx - tw/2
				tooltipY := my - th - 8

				// Keep tooltip on screen
				if tooltipX < 4 {
					tooltipX = 4
				}
				if tooltipX+tw+8 > protocol.ScreenW {
					tooltipX = protocol.ScreenW - tw - 8
				}
				if tooltipY < 4 {
					tooltipY = my + 16
				}

				// Draw tooltip background
				ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

				// Draw tooltip text
				text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.NRGBA{255, 215, 0, 255}) // Gold color for cost
			}
		}

		// Draw tooltips for mini overlay
		if g.hoveredOverlayLevel && g.miniOverlayName != "" {
			lvl := 1
			if g.unitXP != nil {
				if xp, ok := g.unitXP[g.miniOverlayName]; ok {
					l, _, _ := computeLevel(xp)
					if l > 0 {
						lvl = l
					}
				}
			}

			tooltipText := fmt.Sprintf("Level %d", lvl)
			tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
			th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

			// Position tooltip above the level badge
			tooltipX := mx - tw/2
			tooltipY := my - th - 8

			// Keep tooltip on screen
			if tooltipX < 4 {
				tooltipX = 4
			}
			if tooltipX+tw+8 > protocol.ScreenW {
				tooltipX = protocol.ScreenW - tw - 8
			}
			if tooltipY < 4 {
				tooltipY = my + 16
			}

			// Draw tooltip background
			ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

			// Draw tooltip text
			text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.White)
		}

		if g.hoveredOverlayCost && g.miniOverlayName != "" {
			if info, ok := g.nameToMini[g.miniOverlayName]; ok {
				tooltipText := fmt.Sprintf("Deploy cost %d", info.Cost)
				tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
				th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

				// Position tooltip above the cost text
				tooltipX := mx - tw/2
				tooltipY := my - th - 8

				// Keep tooltip on screen
				if tooltipX < 4 {
					tooltipX = 4
				}
				if tooltipX+tw+8 > protocol.ScreenW {
					tooltipX = protocol.ScreenW - tw - 8
				}
				if tooltipY < 4 {
					tooltipY = my + 16
				}

				// Draw tooltip background
				ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

				// Draw tooltip text
				text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.NRGBA{255, 215, 0, 255}) // Gold color for cost
			}
		}

		// Draw tooltips for collection XP bars
		if g.hoveredCollectionXP >= 0 && g.hoveredCollectionXP < len(avail) {
			it := avail[g.hoveredCollectionXP]
			xp := 0
			if g.unitXP != nil {
				if xpVal, ok := g.unitXP[it.Name]; ok {
					xp = xpVal
				}
			}
			_, cur, next := computeLevel(xp)

			tooltipText := fmt.Sprintf("%d/%d", cur, next)
			tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
			th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

			// Position tooltip above the XP bar
			tooltipX := mx - tw/2
			tooltipY := my - th - 8

			// Keep tooltip on screen
			if tooltipX < 4 {
				tooltipX = 4
			}
			if tooltipX+tw+8 > protocol.ScreenW {
				tooltipX = protocol.ScreenW - tw - 8
			}
			if tooltipY < 4 {
				tooltipY = my + 16
			}

			// Draw tooltip background
			ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

			// Draw tooltip text
			text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.White)
		}

		if g.armyMsg != "" {
			text.Draw(screen, g.armyMsg, basicfont.Face7x13, pad, protocol.ScreenH-menuBarH-24, color.White)
		}

		// Right-click selected mini slots to open XP overlay handled in Update
		// Mini XP overlay drawing + actions
		if g.miniOverlayOpen && g.miniOverlayName != "" {
			// Declare STAR bar variables here
			var starBarX, starBarY, starBarW, starBarH int

			ebitenutil.DrawRect(screen, 0, 0, float64(protocol.ScreenW), float64(protocol.ScreenH), color.NRGBA{0, 0, 0, 120})
			w, h := 580, 300
			x := (protocol.ScreenW - w) / 2
			// Position a bit lower so 2x3 slots remain visible when selecting
			const miniCardH = 116
			const gap = 10
			stripY := topBarH + pad
			stripH := 116 + 8
			topY := stripY + stripH + 12
			slotsBottom := topY + 2*(miniCardH+gap) - gap
			y := slotsBottom + 28
			if y+h > protocol.ScreenH-12 {
				y = (protocol.ScreenH - h) / 2
			}
			ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h), color.NRGBA{32, 34, 48, 245})
			// close box (draw only; click handled in Update)
			closeR := rect{x: x + w - 28, y: y + 8, w: 20, h: 20}
			ebitenutil.DrawRect(screen, float64(closeR.x), float64(closeR.y), float64(closeR.w), float64(closeR.h), color.NRGBA{60, 60, 80, 255})
			text.Draw(screen, "X", basicfont.Face7x13, closeR.x+6, closeR.y+14, color.White)
			// portrait
			if img := g.ensureMiniImageByName(g.miniOverlayName); img != nil {
				iw, ih := img.Bounds().Dx(), img.Bounds().Dy()
				pw, ph := 120, 140
				px := x + 16
				py := y + 36
				s := mathMin(float64(pw)/float64(iw), float64(ph)/float64(ih))
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Scale(s, s)
				ox := float64(px) + (float64(pw)-float64(iw)*s)/2
				oy := float64(py) + (float64(ph)-float64(ih)*s)/2
				op.GeoM.Translate(ox, oy)
				screen.DrawImage(img, op)
				// Level badge over portrait top-left (bigger)
				lvl := 1
				if g.unitXP != nil {
					if v, ok := g.unitXP[g.miniOverlayName]; ok {
						if l, _, _ := computeLevel(v); l > 0 {
							lvl = l
						}
					}
				}

				// Level badge with purple theme (same size as gold cost badges)
				if lvl > 0 {
					levelStr := fmt.Sprintf("%d", lvl)
					levelW := len(levelStr) * 7
					vector.DrawFilledRect(screen, float32(px+0-4), float32(py+0-2), float32(levelW+8), 14, color.NRGBA{138, 43, 226, 200}, true)
					vector.StrokeRect(screen, float32(px+0-4), float32(py+0-2), float32(levelW+8), 14, 1, color.NRGBA{100, 20, 150, 255}, true)
					text.Draw(screen, levelStr, basicfont.Face7x13, px+0, py+0+10, color.NRGBA{255, 255, 255, 255})
				}
			}
			// Unit name as title
			text.Draw(screen, g.miniOverlayName, basicfont.Face7x13, x+170, y+24, color.NRGBA{255, 215, 0, 255}) // Gold color for title
			// Stats block (right side)
			if info, ok := g.nameToMini[g.miniOverlayName]; ok {
				sy := y + 50
				startY := sy // Remember the starting Y position
				statCount := 0
				switchedToRight := false
				stat := func(label, val string, isRightColumn bool) {
					colX := x + 170
					if isRightColumn {
						colX = x + 320 // About 30 characters distance from left column
						if !switchedToRight {
							switchedToRight = true
							sy = startY // Reset to first stat's Y position for right column
						}
					}
					text.Draw(screen, label+": "+val, basicfont.Face7x13, colX, sy, color.NRGBA{220, 220, 230, 255})
					if !isRightColumn {
						statCount++
					}
					sy += 16
				}
				// Class / Cost first
				stat("Class", strings.Title(info.Class), false)
				stat("Cost", fmt.Sprintf("%d", info.Cost), false)

				// STAR (shards) progress bar - positioned below stats
				starBarX = x + 160
				starBarY = startY + 180
				starBarW = w - 170 - 24
				starBarH = 24

				// STAR title above progress bar
				text.Draw(screen, "⭐ STAR Progress", basicfont.Face7x13, starBarX, starBarY-8, color.NRGBA{255, 215, 0, 255}) // Gold color

				// Get unit progression data
				// Get rarity threshold based on unit
				threshold := 3 // default (Common)
				if info.Class == "champion" {
					threshold = 25 // Legendary
				} else {
					// Simple mapping based on class/name patterns for minis
					unitName := strings.ToLower(g.miniOverlayName)
					if strings.Contains(unitName, "rare") ||
						strings.Contains(unitName, "elite") ||
						strings.Contains(unitName, "knight") ||
						strings.Contains(unitName, "archer") {
						threshold = 10 // Rare
					} else if strings.Contains(unitName, "legendary") ||
						strings.Contains(unitName, "mythic") ||
						strings.Contains(unitName, "lord") ||
						strings.Contains(unitName, "king") {
						threshold = 25 // Epic/Legendary
					}
				}

				// Get current progress from unitProgression
				currentShards := 0
				if g.unitProgression != nil {
					if progress, exists := g.unitProgression[g.miniOverlayName]; exists {
						currentShards = progress.ShardsOwned
					}
				}

				// Star progress bar background
				vector.DrawFilledRect(screen, float32(starBarX), float32(starBarY), float32(starBarW), float32(starBarH), color.NRGBA{20, 22, 30, 200}, true)

				// Star progress bar fill (gold/star themed color for progress)
				var fillW int
				if threshold > 0 {
					progressRatio := float64(currentShards) / float64(threshold)
					if progressRatio > 1.0 {
						progressRatio = 1.0 // Cap at 100%
					}
					fillW = int(float64(starBarW) * progressRatio)
					if fillW > 0 {
						vector.DrawFilledRect(screen, float32(starBarX), float32(starBarY), float32(fillW), float32(starBarH), color.NRGBA{255, 200, 0, 220}, true)
					}
				}

				// Star progress bar border (gold-themed)
				vector.StrokeRect(screen, float32(starBarX), float32(starBarY), float32(starBarW), float32(starBarH), 2, color.NRGBA{180, 140, 0, 255}, true)

				// Star progress text centered over bar (X/Y format)
				starProgressText := fmt.Sprintf("%d / %d", currentShards, threshold)
				var tw int = text.BoundString(basicfont.Face7x13, starProgressText).Dx()
				var tx int = starBarX + (starBarW-tw)/2
				var ty int = starBarY + starBarH/2 + 6

				// Shadow effect for better visibility
				text.Draw(screen, starProgressText, basicfont.Face7x13, tx+1, ty, color.NRGBA{0, 0, 0, 200})
				text.Draw(screen, starProgressText, basicfont.Face7x13, tx-1, ty, color.NRGBA{0, 0, 0, 200})
				text.Draw(screen, starProgressText, basicfont.Face7x13, tx, ty+1, color.NRGBA{0, 0, 0, 200})
				text.Draw(screen, starProgressText, basicfont.Face7x13, tx, ty-1, color.NRGBA{0, 0, 0, 200})
				text.Draw(screen, starProgressText, basicfont.Face7x13, tx, ty, color.White)

				// Damage / Health
				// Scale by level: 10% per level above 1
				lvlStat := 1
				if g.unitXP != nil {
					if v, ok2 := g.unitXP[g.miniOverlayName]; ok2 {
						if l, _, _ := computeLevel(v); l > 0 {
							lvlStat = l
						}
					}
				}
				scale := 1.0 + 0.10*float64(lvlStat-1)
				if info.Dmg > 0 {
					stat("Damage", fmt.Sprintf("%d", int(float64(info.Dmg)*scale)), false)
					// Calculate DPS (damage per second based on attack speed)
					attackSpeed := 1.0 // default
					if info.AttackSpeed > 0 {
						attackSpeed = info.AttackSpeed
					}
					dps := float64(info.Dmg) * scale / attackSpeed
					stat("DPS", fmt.Sprintf("%.1f", dps), false)
				}
				if info.Hp > 0 {
					stat("Health", fmt.Sprintf("%d", int(float64(info.Hp)*scale)), false)
				}
				// Healer stats
				if info.SubClass == "healer" {
					if info.Hps > 0 {
						stat("HPS", fmt.Sprintf("%d", int(float64(info.Hps)*scale)), statCount >= 7)
					}
					if info.Heal > 0 {
						stat("Heal", fmt.Sprintf("%d", int(float64(info.Heal)*scale)), statCount >= 7)
					}
				}
				// Speed scale
				if info.Speed > 0 {
					sp := map[int]string{1: "Slow", 2: "Medium", 3: "Mid-fast", 4: "Fast"}
					sv := sp[info.Speed]
					if sv == "" {
						sv = fmt.Sprintf("%d", info.Speed)
					}
					stat("Speed", sv, statCount >= 7)
				}
				// After stats, place XP bar
				// Position XP bar below the stats area, not after the last stat
				barX, barY := x+160, startY+120
				barW, barH := w-170-24, 30
				// Draw very dark yellow frame around the whole progress bar dimension
				ebitenutil.DrawRect(screen, float64(barX-2), float64(barY-2), float64(barW+4), float64(barH+4), color.NRGBA{100, 90, 0, 255})
				// Fill the entire bar with overlay background color (transparent/missing XP)
				ebitenutil.DrawRect(screen, float64(barX), float64(barY), float64(barW), float64(barH), color.NRGBA{32, 34, 48, 245})
				// Level/XP calc
				xp := 0
				if g.unitXP != nil {
					if v, ok := g.unitXP[g.miniOverlayName]; ok {
						xp = v
					}
				}
				lvl, cur, next := computeLevel(xp)
				if lvl > 20 {
					lvl = 20
				}
				frac := 1.0
				if next > 0 {
					frac = float64(cur) / float64(next)
				}
				fillW = int(float64(barW) * frac)
				// Purpleish blue fill only for the filled portion
				if fillW > 0 {
					ebitenutil.DrawRect(screen, float64(barX), float64(barY), float64(fillW), float64(barH), color.NRGBA{138, 43, 226, 200})
				}
				// XP text centered over the bar (no 'Level X' text)
				var s string
				if next > 0 {
					s = fmt.Sprintf("%d/%d", cur, next)
				} else {
					s = "max"
				}
				tw = text.BoundString(basicfont.Face7x13, s).Dx()
				tx = barX + (barW-tw)/2
				ty = barY + barH/2 + 6
				text.Draw(screen, s, basicfont.Face7x13, tx+1, ty, color.NRGBA{0, 0, 0, 200})
				text.Draw(screen, s, basicfont.Face7x13, tx-1, ty, color.NRGBA{0, 0, 0, 200})
				text.Draw(screen, s, basicfont.Face7x13, tx, ty+1, color.NRGBA{0, 0, 0, 200})
				text.Draw(screen, s, basicfont.Face7x13, tx, ty-1, color.NRGBA{0, 0, 0, 200})
				text.Draw(screen, s, basicfont.Face7x13, tx, ty, color.White)

				// XP bar hover tooltip
				if g.xpBarHovered {
					mx, my := ebiten.CursorPosition()
					// Calculate next level and XP required
					nextLevel := lvl + 1
					if nextLevel > 20 {
						nextLevel = 20
					}
					xpRequired := 0
					if next > 0 {
						xpRequired = next - cur
					}

					// Tooltip text
					tooltipText := fmt.Sprintf("XP to level %d: %d", nextLevel, xpRequired)

					// Measure text for tooltip box
					tw := text.BoundString(basicfont.Face7x13, tooltipText).Dx()
					th := text.BoundString(basicfont.Face7x13, tooltipText).Dy()

					// Position tooltip above cursor with some offset
					tooltipX := mx - tw/2
					tooltipY := my - th - 8

					// Keep tooltip on screen
					if tooltipX < 4 {
						tooltipX = 4
					}
					if tooltipX+tw+8 > protocol.ScreenW {
						tooltipX = protocol.ScreenW - tw - 8
					}
					if tooltipY < 4 {
						tooltipY = my + 16
					}

					// Draw tooltip background
					ebitenutil.DrawRect(screen, float64(tooltipX-4), float64(tooltipY-4), float64(tw+8), float64(th+8), color.NRGBA{30, 30, 45, 240})

					// Draw tooltip text
					text.Draw(screen, tooltipText, basicfont.Face7x13, tooltipX, tooltipY+th, color.White)
				}
			}

			// Action button (draw only; handled in Update)
			btn := func(rx, ry int, label string) rect {
				r := rect{x: rx, y: ry, w: 110, h: 26}
				ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), float64(r.h), color.NRGBA{70, 110, 70, 255})
				text.Draw(screen, label, basicfont.Face7x13, r.x+10, r.y+18, color.White)
				return r
			}
			if g.miniOverlayFrom == "collection" {
				// Place Equip button below the portrait, centered under it
				pbx := x + 16 + (120-110)/2 // portrait width=120, button width=110
				pby := y + 36 + 140 + 10    // portrait top + height + gap
				_ = btn(pbx, pby, "Equip")
			}
			// Target selection modes
			if g.miniOverlayMode == "switch_target_slot" {
				// Highlight slots and accept click
				// Recompute slot rects
				stripY := topBarH + pad
				stripH := 116 + 8
				topY := stripY + stripH + 12
				bigW := 200
				// Center champion + slots block
				const miniCardW, miniCardH = 100, 116
				const gap = 10
				totalW := bigW + 16 + 3*miniCardW + 2*gap
				startX := (protocol.ScreenW - totalW) / 2
				leftX := startX
				rightX := leftX + bigW + 16
				k := 1
				slots := make([]rect, 6)
				for row := 0; row < 2; row++ {
					for col := 0; col < 3; col++ {
						slots[k-1] = rect{x: rightX + col*(miniCardW+gap), y: topY + row*(miniCardH+gap), w: miniCardW, h: miniCardH}
						k++
					}
				}
				// draw faint highlight
				for _, r := range slots {
					ebitenutil.DrawRect(screen, float64(r.x), float64(r.y), float64(r.w), 2, color.NRGBA{240, 196, 25, 200})
				}
				if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
					mx, my := ebiten.CursorPosition()
					for si, r := range slots {
						if r.hit(mx, my) {
							// place into exact slot si
							prev := g.selectedOrder[si]
							if prev != "" {
								delete(g.selectedMinis, prev)
							}
							g.selectedOrder[si] = g.miniOverlayName
							g.selectedMinis[g.miniOverlayName] = true
							g.setChampArmyFromSelected()
							g.autoSaveCurrentChampionArmy()
							g.miniOverlayOpen = false
							g.miniOverlayMode = ""
							break
						}
					}
				}
			}
		}
	case tabMap:
		disp := g.displayMapID()

		mapBg := g.ensureBgForMap(disp)
		offX, offY, dispW, dispH, s := g.mapRenderRect(mapBg)

		if mapBg != nil {
			c := g.mapEdgeColor(disp, mapBg)
			if offY > 0 {
				ebitenutil.DrawRect(screen, 0, 0, float64(protocol.ScreenW), float64(offY), c)
				ebitenutil.DrawRect(screen, 0, float64(offY+dispH),
					float64(protocol.ScreenW), float64(protocol.ScreenH-(offY+dispH)), c)
			}
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(s, s)
			op.GeoM.Translate(float64(offX), float64(offY))
			// Use linear filtering for smoother scaling on high-resolution displays
			op.Filter = ebiten.FilterLinear
			screen.DrawImage(mapBg, op)
		}
		// Initialize world map particles when switching to map tab
		if g.particleSystem == nil {
			g.initWorldMapParticles()
		}

		// Ensure particles are initialized even if particle system exists but is empty
		if g.particleSystem != nil && len(g.particleSystem.Emitters) == 0 {
			g.initWorldMapParticles()
		}

		// Draw particles AFTER the map so they appear on top!
		g.drawParticlesWithoutCamera(screen, 0, 0, protocol.ScreenW, protocol.ScreenH)

		text.Draw(screen, "Map — click a location, then press Start",
			basicfont.Face7x13, pad, topBarH-6, color.White)

		g.ensureMapHotspots()
		hsList := g.mapHotspots[disp]
		if hsList == nil {
			hsList = g.mapHotspots[defaultMapID]
		}
		for i, hs := range hsList {
			cx := offX + int(hs.X*float64(dispW))
			cy := offY + int(hs.Y*float64(dispH))

			col := color.NRGBA{0x66, 0x99, 0xcc, 0xff}
			if i == g.hoveredHS {
				col = color.NRGBA{0xa0, 0xd0, 0xff, 0xff}
			}
			if i == g.selectedHS {
				col = color.NRGBA{240, 196, 25, 255}
			}

			ebitenutil.DrawRect(screen, float64(cx-2), float64(cy-2), 4, 4, col)
		}

		if g.hoveredHS >= 0 && g.hoveredHS < len(hsList) {
			hs := hsList[g.hoveredHS]
			mx, my := ebiten.CursorPosition()
			w, h := 260, 46
			x := clampInt(mx+14, 0, protocol.ScreenW-w)
			y := clampInt(my-8-h, 0, protocol.ScreenH-h)
			ebitenutil.DrawRect(screen, float64(x), float64(y), float64(w), float64(h),
				color.NRGBA{30, 30, 45, 240})
			text.Draw(screen, hs.Name, basicfont.Face7x13, x+8, y+18, color.White)
			if hs.Info != "" {
				text.Draw(screen, hs.Info, basicfont.Face7x13, x+8, y+34, color.NRGBA{200, 200, 200, 255})
			}
		}

		if g.selectedHS >= 0 && g.selectedHS < len(hsList) {
			hs := hsList[g.selectedHS]
			cx := offX + int(hs.X*float64(dispW))
			cy := offY + int(hs.Y*float64(dispH))
			g.startBtn = rect{x: cx + 22, y: cy - 16, w: 90, h: 28}

			btnCol := color.NRGBA{70, 110, 70, 255}
			label := "Start"
			if g.roomID == "" {
				btnCol = color.NRGBA{110, 110, 70, 255}
				label = "Start…"
			}
			ebitenutil.DrawRect(screen, float64(g.startBtn.x), float64(g.startBtn.y),
				float64(g.startBtn.w), float64(g.startBtn.h), btnCol)
			text.Draw(screen, label, basicfont.Face7x13, g.startBtn.x+18, g.startBtn.y+18, color.White)
		}
	case tabPvp:

		contentY := topBarH
		contentH := protocol.ScreenH - menuBarH - contentY

		// Draw themed background panel with enhanced styling
		if g.fantasyUI != nil {
			g.fantasyUI.DrawThemedPanel(screen, 0, contentY, protocol.ScreenW, contentH, 0.9)

			// Add subtle pattern overlay for PvP section
			g.fantasyUI.CreateBackgroundPattern(protocol.ScreenW, contentH)
			pattern := g.fantasyUI.CreateBackgroundPattern(protocol.ScreenW, contentH)
			if pattern != nil {
				op := &ebiten.DrawImageOptions{}
				op.GeoM.Translate(0, float64(contentY))
				op.ColorM.Scale(1, 1, 1, 0.3) // Very subtle opacity
				screen.DrawImage(pattern, op)
			}
		} else {
			ebitenutil.DrawRect(screen, 0, float64(contentY), float64(protocol.ScreenW), float64(contentH), color.NRGBA{0x20, 0x20, 0x28, 0xFF})
		}

		// Add PvP section title with enhanced styling
		titleY := contentY + 20
		if g.fantasyUI != nil {
			// Draw ornate title background
			vector.DrawFilledRect(screen, float32(pad), float32(titleY-8), float32(protocol.ScreenW-2*pad), 40, g.fantasyUI.Theme.CardBackground, true)
			vector.StrokeRect(screen, float32(pad), float32(titleY-8), float32(protocol.ScreenW-2*pad), 40, 2, g.fantasyUI.Theme.Border, true)

			// Add title glow effect
			vector.StrokeRect(screen, float32(pad+2), float32(titleY-6), float32(protocol.ScreenW-2*pad-4), 36, 1, g.fantasyUI.Theme.Glow, true)

			text.Draw(screen, "⚔️ Player vs Player Arena", basicfont.Face7x13, pad+12, titleY+6, g.fantasyUI.Theme.TextPrimary)
			text.Draw(screen, "Battle other players in ranked matches", basicfont.Face7x13, pad+12, titleY+22, g.fantasyUI.Theme.TextSecondary)
		} else {
			text.Draw(screen, "⚔️ Player vs Player Arena", basicfont.Face7x13, pad+8, titleY, color.NRGBA{240, 196, 25, 255})
			text.Draw(screen, "Battle other players in ranked matches", basicfont.Face7x13, pad+8, titleY+16, color.NRGBA{200, 200, 200, 255})
		}

		queueBtn, leaveBtn, createBtn, cancelBtn, joinInput, joinBtn := g.pvpLayout()

		// Enhanced button states with hover detection
		mx, my := ebiten.CursorPosition()
		queueHovered := queueBtn.hit(mx, my)
		leaveHovered := leaveBtn.hit(mx, my)
		createHovered := createBtn.hit(mx, my)
		cancelHovered := cancelBtn.hit(mx, my)
		joinHovered := joinBtn.hit(mx, my)

		// Draw themed buttons with enhanced states and conditional visibility
		if g.fantasyUI != nil {
			// Queue PvP button - only show when not queued
			if !g.pvpQueued {
				queueState := ButtonNormal
				if queueHovered {
					queueState = ButtonHover
				}
				g.fantasyUI.DrawThemedButtonWithStyle(screen, queueBtn.x, queueBtn.y, queueBtn.w, queueBtn.h, "Queue PvP", queueState, true)
			}

			// Leave Queue button - only show when queued
			if g.pvpQueued {
				leaveState := ButtonNormal
				if leaveHovered {
					leaveState = ButtonHover
				}
				g.fantasyUI.DrawThemedButtonWithStyle(screen, leaveBtn.x, leaveBtn.y, leaveBtn.w, leaveBtn.h, "Leave Queue", leaveState, true)
			}

			// Create Friendly Code button - only show when not hosting
			if !g.pvpHosting {
				createState := ButtonNormal
				if createHovered {
					createState = ButtonHover
				}
				g.fantasyUI.DrawThemedButtonWithStyle(screen, createBtn.x, createBtn.y, createBtn.w, createBtn.h, "Create Friendly Code", createState, true)
			}

			// Cancel button - only show when hosting
			if g.pvpHosting {
				cancelState := ButtonNormal
				if cancelHovered {
					cancelState = ButtonHover
				}
				g.fantasyUI.DrawThemedButtonWithStyle(screen, cancelBtn.x, cancelBtn.y, cancelBtn.w, cancelBtn.h, "Cancel Friendly", cancelState, true)
			}

			// Join button - always visible
			joinState := ButtonNormal
			if joinHovered {
				joinState = ButtonHover
			}
			g.fantasyUI.DrawThemedButtonWithStyle(screen, joinBtn.x, joinBtn.y, joinBtn.w, joinBtn.h, "Join", joinState, true)
		} else {
			// Fallback to basic buttons
			ebitenutil.DrawRect(screen, float64(queueBtn.x), float64(queueBtn.y), float64(queueBtn.w), float64(queueBtn.h),
				map[bool]color.NRGBA{true: {80, 110, 80, 255}, false: {60, 60, 80, 255}}[g.pvpQueued])
			text.Draw(screen, "Queue PvP", basicfont.Face7x13, queueBtn.x+16, queueBtn.y+18, color.White)

			ebitenutil.DrawRect(screen, float64(leaveBtn.x), float64(leaveBtn.y), float64(leaveBtn.w), float64(leaveBtn.h),
				color.NRGBA{90, 70, 70, 255})
			text.Draw(screen, "Leave Queue", basicfont.Face7x13, leaveBtn.x+16, leaveBtn.y+18, color.White)

			ebitenutil.DrawRect(screen, float64(createBtn.x), float64(createBtn.y), float64(createBtn.w), float64(createBtn.h),
				map[bool]color.NRGBA{true: {80, 110, 80, 255}, false: {60, 60, 80, 255}}[g.pvpHosting])
			text.Draw(screen, "Create Friendly Code", basicfont.Face7x13, createBtn.x+16, createBtn.y+18, color.White)

			ebitenutil.DrawRect(screen, float64(cancelBtn.x), float64(cancelBtn.y), float64(cancelBtn.w), float64(cancelBtn.h),
				color.NRGBA{90, 70, 70, 255})
			text.Draw(screen, "Cancel", basicfont.Face7x13, cancelBtn.x+16, cancelBtn.y+18, color.White)
		}

		g.pvpCodeArea = rect{}
		if g.pvpHosting && g.pvpCode != "" {
			msg := "Your code: " + g.pvpCode

			lb := text.BoundString(basicfont.Face7x13, msg)
			bx := createBtn.x
			by := createBtn.y + createBtn.h + 12
			bw := lb.Dx() + 18
			bh := 26

			g.pvpCodeArea = rect{x: bx, y: by, w: bw, h: bh}

			// Draw themed code display area
			if g.fantasyUI != nil {
				// Draw the card background and border
				vector.DrawFilledRect(screen, float32(bx), float32(by), float32(bw), float32(bh), g.fantasyUI.Theme.CardBackground, true)
				vector.StrokeRect(screen, float32(bx), float32(by), float32(bw), float32(bh), 3, g.fantasyUI.Theme.Border, true)
				vector.StrokeRect(screen, float32(bx+3), float32(by+3), float32(bw-6), float32(bh-6), 1, g.fantasyUI.Theme.Glow, true)

				// Draw the code text centered in the card
				text.Draw(screen, msg, basicfont.Face7x13, bx+9, by+18, g.fantasyUI.Theme.TextPrimary)
			} else {
				ebitenutil.DrawRect(screen, float64(bx), float64(by), float64(bw), float64(bh), color.NRGBA{54, 63, 88, 255})
				text.Draw(screen, msg, basicfont.Face7x13, bx+9, by+18, color.White)
			}

			hintX := bx + bw + 12
			hintY := by + (bh+13)/2 - 2
			text.Draw(screen, "Click to copy", basicfont.Face7x13, hintX, hintY, color.NRGBA{160, 160, 170, 255})
		}

		// Enhanced input field with themed styling
		if g.fantasyUI != nil {
			// Draw themed input field background - use darker colors for better text visibility
			inputColor := color.NRGBA{45, 45, 60, 255} // Darker background
			if g.pvpInputActive {
				inputColor = color.NRGBA{55, 55, 75, 255} // Slightly lighter when active
			}
			vector.DrawFilledRect(screen, float32(joinInput.x), float32(joinInput.y),
				float32(joinInput.w), float32(joinInput.h), inputColor, true)
			vector.StrokeRect(screen, float32(joinInput.x), float32(joinInput.y),
				float32(joinInput.w), float32(joinInput.h), 2, g.fantasyUI.Theme.Border, true)
		} else {
			ebitenutil.DrawRect(screen, float64(joinInput.x), float64(joinInput.y), float64(joinInput.w), float64(joinInput.h),
				color.NRGBA{38, 38, 53, 255})
		}

		label := g.pvpCodeInput
		if label == "" && !g.pvpInputActive {
			label = "Enter code..."
			text.Draw(screen, label, basicfont.Face7x13, joinInput.x+8, joinInput.y+17, color.NRGBA{150, 150, 160, 255})
		} else {
			text.Draw(screen, label, basicfont.Face7x13, joinInput.x+8, joinInput.y+17, color.White)
		}

		bottomY := joinBtn.y + joinBtn.h
		sepY := bottomY + 20

		// Draw themed separator
		if g.fantasyUI != nil {
			vector.DrawFilledRect(screen, float32(pad), float32(sepY), float32(protocol.ScreenW-2*pad), 2, g.fantasyUI.Theme.Border, true)
		} else {
			ebitenutil.DrawRect(screen, float64(pad), float64(sepY), float64(protocol.ScreenW-2*pad), 1, color.NRGBA{90, 90, 120, 255})
		}

		// Status panel with themed styling
		panelY := sepY + 14
		panelH := 54

		if g.fantasyUI != nil {
			// Draw status panel manually to avoid the underline that passes through the text
			vector.DrawFilledRect(screen, float32(pad), float32(panelY), float32(protocol.ScreenW-2*pad), float32(panelH), g.fantasyUI.Theme.CardBackground, true)
			vector.StrokeRect(screen, float32(pad), float32(panelY), float32(protocol.ScreenW-2*pad), float32(panelH), 3, g.fantasyUI.Theme.Border, true)
			vector.StrokeRect(screen, float32(pad+3), float32(panelY+3), float32(protocol.ScreenW-2*pad-6), float32(panelH-6), 1, g.fantasyUI.Theme.Glow, true)

			// Draw title and content manually without the underline
			text.Draw(screen, "Status", basicfont.Face7x13, pad+12, panelY+18, g.fantasyUI.Theme.TextPrimary)

			msg := g.pvpStatus
			if msg == "" {
				msg = "—"
			}
			text.Draw(screen, msg, basicfont.Face7x13, pad+12, panelY+36, g.fantasyUI.Theme.TextSecondary)
		} else {
			ebitenutil.DrawRect(screen, float64(pad), float64(panelY), float64(protocol.ScreenW-2*pad), float64(panelH), color.NRGBA{0x24, 0x24, 0x30, 0xFF})
			text.Draw(screen, "Status", basicfont.Face7x13, pad+8, panelY+18, color.NRGBA{240, 196, 25, 255})

			msg := g.pvpStatus
			if msg == "" {
				msg = "—"
			}
			text.Draw(screen, msg, basicfont.Face7x13, pad+8, panelY+36, color.White)
		}

		// Leaderboard panel with enhanced styling
		panelPad := pad
		rows := minInt(50, len(g.pvpLeaders))
		const rowH = 16
		leaderboardPanelH := 16 + 16 + rows*rowH + 8
		if leaderboardPanelH < 120 {
			leaderboardPanelH = 120
		}

		leaderboardPanelTop := protocol.ScreenH - menuBarH - leaderboardPanelH - 8
		if leaderboardPanelTop < topBarH+180 {
			leaderboardPanelTop = topBarH + 180
		}

		// Draw themed leaderboard panel
		if g.fantasyUI != nil {
			g.fantasyUI.DrawThemedCard(screen, panelPad, leaderboardPanelTop,
				protocol.ScreenW-2*panelPad, leaderboardPanelH, "Top 50 - PvP Leaderboard", []string{})

			// Add timestamp if available
			if g.lbLastStamp != 0 {
				ts := time.UnixMilli(g.lbLastStamp).Format("15:04:05")
				text.Draw(screen, "as of "+ts, basicfont.Face7x13, panelPad+240, leaderboardPanelTop+18, color.NRGBA{170, 170, 180, 255})
			}
		} else {
			ebitenutil.DrawRect(screen, float64(panelPad), float64(leaderboardPanelTop),
				float64(protocol.ScreenW-2*panelPad), float64(leaderboardPanelH), color.NRGBA{0x24, 0x24, 0x30, 0xFF})

			text.Draw(screen, "Top 50 - PvP Leaderboard", basicfont.Face7x13, panelPad+8, leaderboardPanelTop+18, color.White)
			if g.lbLastStamp != 0 {
				ts := time.UnixMilli(g.lbLastStamp).Format("15:04:05")
				text.Draw(screen, "as of "+ts, basicfont.Face7x13, panelPad+240, leaderboardPanelTop+18, color.NRGBA{170, 170, 180, 255})
			}
		}

		colRankX := panelPad + 8
		colNameX := panelPad + 58
		colRatX := panelPad + 360
		colTierX := panelPad + 440

		hdrY := leaderboardPanelTop + 36
		text.Draw(screen, "#", basicfont.Face7x13, colRankX, hdrY, color.NRGBA{200, 200, 210, 255})
		text.Draw(screen, "Player", basicfont.Face7x13, colNameX, hdrY, color.NRGBA{200, 200, 210, 255})
		text.Draw(screen, "Rating", basicfont.Face7x13, colRatX, hdrY, color.NRGBA{200, 200, 210, 255})
		text.Draw(screen, "Rank", basicfont.Face7x13, colTierX, hdrY, color.NRGBA{200, 200, 210, 255})

		rowY := hdrY + 16
		maxRows := minInt(50, len(g.pvpLeaders))
		for i := 0; i < maxRows; i++ {
			e := g.pvpLeaders[i]
			y := rowY + i*rowH

			// Alternate row colors with theme integration
			if i%2 == 0 {
				if g.fantasyUI != nil {
					vector.DrawFilledRect(screen, float32(panelPad+4), float32(y-12),
						float32(protocol.ScreenW-2*panelPad-8), float32(rowH), g.fantasyUI.Theme.Surface, true)
				} else {
					ebitenutil.DrawRect(screen, float64(panelPad+4), float64(y-12),
						float64(protocol.ScreenW-2*panelPad-8), rowH, color.NRGBA{0x28, 0x28, 0x36, 0xFF})
				}
			}

			text.Draw(screen, fmt.Sprintf("%2d.", i+1), basicfont.Face7x13, colRankX, y, color.White)
			text.Draw(screen, trim(e.Name, 22), basicfont.Face7x13, colNameX, y, color.White)
			text.Draw(screen, fmt.Sprintf("%d", e.Rating), basicfont.Face7x13, colRatX, y, color.White)
			text.Draw(screen, e.Rank, basicfont.Face7x13, colTierX, y, color.NRGBA{240, 196, 25, 255})
		}

	case tabSocial:
		g.drawSocial(screen)
	case tabShop:
		// Draw the actual shop UI
		if g.shopView == nil {
			g.initShopUI()
		}
		g.shopView.Draw(screen)
	case tabSettings:
		// Settings panel background
		contentY := topBarH + pad
		contentH := protocol.ScreenH - menuBarH - contentY
		ebitenutil.DrawRect(screen, float64(pad), float64(contentY), float64(protocol.ScreenW-2*pad), float64(contentH), color.NRGBA{0x20, 0x20, 0x28, 0xFF})

		// Title
		y := contentY + 20
		text.Draw(screen, "⚙️ Settings", basicfont.Face7x13, pad+8, y, color.NRGBA{240, 196, 25, 255})

		// Display Settings Section
		y += 40
		text.Draw(screen, "Display Settings", basicfont.Face7x13, pad+8, y, color.NRGBA{200, 200, 210, 255})

		y += 25
		text.Draw(screen, "Fullscreen:", basicfont.Face7x13, pad+16, y, color.White)

		g.fsOnBtn = rect{x: pad + 140, y: y - 14, w: 80, h: 20}
		g.fsOffBtn = rect{x: g.fsOnBtn.x + 90, y: y - 14, w: 80, h: 20}

		onCol := color.NRGBA{70, 110, 70, 255}
		offCol := color.NRGBA{110, 70, 70, 255}
		neutral := color.NRGBA{60, 60, 80, 255}

		ebitenutil.DrawRect(screen, float64(g.fsOnBtn.x), float64(g.fsOnBtn.y), float64(g.fsOnBtn.w), float64(g.fsOnBtn.h),
			map[bool]color.NRGBA{true: onCol, false: neutral}[g.fullscreen])
		text.Draw(screen, "ON", basicfont.Face7x13, g.fsOnBtn.x+26, g.fsOnBtn.y+14, color.White)

		ebitenutil.DrawRect(screen, float64(g.fsOffBtn.x), float64(g.fsOffBtn.y), float64(g.fsOffBtn.w), float64(g.fsOffBtn.h),
			map[bool]color.NRGBA{true: neutral, false: offCol}[g.fullscreen])
		text.Draw(screen, "OFF", basicfont.Face7x13, g.fsOffBtn.x+24, g.fsOffBtn.y+14, color.White)

		// Game Settings Section
		y += 50
		text.Draw(screen, "Game Settings", basicfont.Face7x13, pad+8, y, color.NRGBA{200, 200, 210, 255})

		y += 25
		text.Draw(screen, "Auto-save Army:", basicfont.Face7x13, pad+16, y, color.White)

		// Auto-save toggle (for now, just show current state)
		autoSaveStatus := "Enabled"
		if !g.autoSaveEnabled {
			autoSaveStatus = "Disabled"
		}
		text.Draw(screen, autoSaveStatus, basicfont.Face7x13, pad+140, y, color.NRGBA{180, 180, 190, 255})

		// Account Settings Section
		y += 50
		text.Draw(screen, "Account Settings", basicfont.Face7x13, pad+8, y, color.NRGBA{200, 200, 210, 255})

		y += 25
		text.Draw(screen, "Player:", basicfont.Face7x13, pad+16, y, color.White)
		text.Draw(screen, g.name, basicfont.Face7x13, pad+140, y, color.NRGBA{240, 196, 25, 255})

		y += 20
		text.Draw(screen, "Gold:", basicfont.Face7x13, pad+16, y, color.White)
		goldStr := fmt.Sprintf("%d", g.accountGold)
		text.Draw(screen, goldStr, basicfont.Face7x13, pad+140, y, color.NRGBA{255, 215, 0, 255})

		// Logout button
		y += 30
		g.logoutBtn = rect{x: pad + 16, y: y - 6, w: 100, h: 24}
		ebitenutil.DrawRect(screen, float64(g.logoutBtn.x), float64(g.logoutBtn.y), float64(g.logoutBtn.w), float64(g.logoutBtn.h), color.NRGBA{110, 70, 70, 255})
		text.Draw(screen, "Logout", basicfont.Face7x13, g.logoutBtn.x+20, g.logoutBtn.y+16, color.White)

		// Controls Section
		y += 50
		text.Draw(screen, "Controls", basicfont.Face7x13, pad+8, y, color.NRGBA{200, 200, 210, 255})

		y += 18
		text.Draw(screen, "Alt+F - Toggle Fullscreen", basicfont.Face7x13, pad+16, y, color.NRGBA{180, 180, 190, 255})
		y += 18
		text.Draw(screen, "Mouse - Navigate & Select", basicfont.Face7x13, pad+16, y, color.NRGBA{180, 180, 190, 255})
		y += 18
		text.Draw(screen, "Touch - Mobile Controls", basicfont.Face7x13, pad+16, y, color.NRGBA{180, 180, 190, 255})

		// Version/Info
		y = protocol.ScreenH - menuBarH - 40
		text.Draw(screen, protocol.GameName+" v"+protocol.GameVersion, basicfont.Face7x13, pad+8, y, color.NRGBA{150, 150, 160, 255})
		text.Draw(screen, "by S. Robev", basicfont.Face7x13, pad+8, y+16, color.NRGBA{120, 120, 130, 255})
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (g *Game) createRoomFor(mapID string) {
	g.pendingArena = mapID
	g.send("CreatePve", protocol.CreatePve{MapID: mapID})
}

func defaultIfEmpty(s, d string) string {
	if s == "" {
		return d
	}
	return s
}
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// call every frame with latest hp + max
func (b *battleHPBar) Set(cur, max int) {
	if max <= 0 {
		max = 1
	}
	if b.maxHP != max {
		b.maxHP = max
		if b.displayHP > max {
			b.displayHP = max
		}
	}
	if cur < 0 {
		cur = 0
	}
	if cur > max {
		cur = max
	}

	if cur < b.targetHP {
		b.flashTicks = 22

	}

	if cur > b.displayHP {
		b.displayHP = cur
	}

	b.targetHP = cur
}

// Returns the image to show as the enemy "avatar" — base for now, boss later.
func (g *Game) enemyTargetAvatarImage() *ebiten.Image {
	if g.enemyTargetThumb != nil {
		return g.enemyTargetThumb
	}

	if g.enemyBossPortrait != "" {
		if img := g.ensureAvatarImage(g.enemyBossPortrait); img != nil {
			g.enemyTargetThumb = img
			return img
		}
	}

	tryPaths := []string{
		"assets/ui/base.png",
		"assets/ui/base_avatar.png",
	}
	for _, p := range tryPaths {
		if img := loadImage(p); img != nil {
			g.enemyTargetThumb = img
			return img
		}
	}

	if img := g.ensureAvatarImage("default.png"); img != nil {
		g.enemyTargetThumb = img
		return img
	}
	return nil
}
