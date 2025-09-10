package ui

import (
	"fmt"
	"image/color"

	"rumble/client/internal/game/progression"
	"rumble/shared/game/types"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"golang.org/x/image/font/basicfont"
)

type PerksPanel struct {
	x, y          int
	width, height int

	// Unit data
	unitProgress   *types.UnitProgress
	availablePerks []types.Perk

	// UI state
	visible      bool
	selectedPerk int

	// Colors
	backgroundColor color.RGBA
	textColor       color.RGBA
	buttonColor     color.RGBA
}

func NewPerksPanel(x, y, width, height int) *PerksPanel {
	return &PerksPanel{
		x: x, y: y,
		width: width, height: height,
		backgroundColor: color.RGBA{50, 50, 50, 200},
		textColor:       color.RGBA{255, 255, 255, 255},
		buttonColor:     color.RGBA{100, 100, 100, 255},
	}
}

func (p *PerksPanel) SetUnitProgress(progress *types.UnitProgress) {
	p.unitProgress = progress
}

func (p *PerksPanel) SetAvailablePerks(perks []types.Perk) {
	p.availablePerks = perks
}

func (p *PerksPanel) Show() {
	p.visible = true
}

func (p *PerksPanel) Hide() {
	p.visible = false
}

func (p *PerksPanel) IsVisible() bool {
	return p.visible
}

func (p *PerksPanel) Update() {
	if !p.visible || p.unitProgress == nil {
		return
	}

	// Handle mouse input for perk selection
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		mx, my := ebiten.CursorPosition()

		// Check if click is within panel bounds
		if mx >= p.x && mx <= p.x+p.width && my >= p.y && my <= p.y+p.height {
			// Calculate which perk was clicked (simple vertical layout)
			perkIndex := (my - p.y - 50) / 30
			if perkIndex >= 0 && perkIndex < len(p.availablePerks) {
				p.selectedPerk = perkIndex
				p.ActivateSelectedPerk()
			}
		}
	}
}

func (p *PerksPanel) ActivateSelectedPerk() {
	if p.selectedPerk < 0 || p.selectedPerk >= len(p.availablePerks) {
		return
	}

	perkID := p.availablePerks[p.selectedPerk].ID

	// Use the progression logic to validate and set the perk
	if progression.SetActivePerk(p.unitProgress, types.PerkID(perkID), p.availablePerks) {
		// Success - would send message to server here
		fmt.Printf("Activated perk: %s\n", perkID)
	} else {
		fmt.Printf("Failed to activate perk: %s\n", perkID)
	}
}

func (p *PerksPanel) Draw(screen *ebiten.Image) {
	if !p.visible || p.unitProgress == nil {
		return
	}

	// Draw background
	backgroundImg := ebiten.NewImage(p.width, p.height)
	backgroundImg.Fill(p.backgroundColor)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(float64(p.x), float64(p.y))
	screen.DrawImage(backgroundImg, op)

	// Draw title
	title := "Unit Perks"
	text.Draw(screen, title, basicfont.Face7x13, p.x+10, p.y+20, p.textColor)

	// Draw unit info
	if p.unitProgress != nil {
		rankText := fmt.Sprintf("Rank: %d", p.unitProgress.Rank)
		text.Draw(screen, rankText, basicfont.Face7x13, p.x+10, p.y+40, p.textColor)

		shardsText := fmt.Sprintf("Shards: %d / %d", p.unitProgress.ShardsOwned, p.unitProgress.Rarity.ShardsPerRank())
		text.Draw(screen, shardsText, basicfont.Face7x13, p.x+10, p.y+55, p.textColor)

		slotsText := fmt.Sprintf("Perk Slots: %d", progression.PerkSlotsUnlocked(p.unitProgress))
		text.Draw(screen, slotsText, basicfont.Face7x13, p.x+10, p.y+70, p.textColor)
	}

	// Draw available perks
	yOffset := 95
	for i, perk := range p.availablePerks {
		perkText := perk.Name
		if i == p.selectedPerk {
			perkText = "> " + perkText
		}

		// Check if this perk can be activated
		canActivate := false
		if perk.Legendary {
			canActivate = progression.LegendaryPerkUnlocked(p.unitProgress)
		} else {
			canActivate = progression.PerkSlotsUnlocked(p.unitProgress) > 0
		}

		textColor := p.textColor
		if !canActivate {
			textColor = color.RGBA{128, 128, 128, 255} // Gray out unavailable perks
		}

		text.Draw(screen, perkText, basicfont.Face7x13, p.x+20, p.y+yOffset, textColor)
		yOffset += 25

		// Draw perk description (truncated if too long)
		desc := perk.Description
		if len(desc) > 50 {
			desc = desc[:47] + "..."
		}
		text.Draw(screen, desc, basicfont.Face7x13, p.x+40, p.y+yOffset, color.RGBA{200, 200, 200, 255})
		yOffset += 20
	}
}
