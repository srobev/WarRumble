package types

type UnitMeta struct {
	ID         string
	Rarity     Rarity
	IsChampion bool
	Icon       string // portrait filename
}

var unitRegistry = []UnitMeta{
	// Mini units with default Common rarity (can be adjusted later)
	{"Flesh Golem", RarityCommon, false, "flesh_golem.png"},
	{"Arcane Blast", RarityCommon, false, "arcane-blast.png"},
	{"Wraith", RarityCommon, false, "wraith.png"},
	{"Foulwing Rider", RarityCommon, false, "foulwing_rider.png"},
	{"Blazebinder", RarityRare, false, "blazebinder.png"},
	{"Blizzard", RarityRare, false, "blizzard.png"},
	{"Chain Lightning", RarityEpic, false, "chain-lightning.png"},
	{"Twin Wyrnn", RarityRare, false, "twin_wyrnn.png"},
	{"Magma Beasts", RarityCommon, false, "magma_beasts.png"},
	{"Redhand Thieves", RarityCommon, false, "redhand_thieves.png"},
	{"Firedrake", RarityEpic, false, "firedrake.png"},
	{"Execute", RarityEpic, false, "execute.png"},
	{"Jungle Headhunter", RarityRare, false, "jungle_headhunter.png"},
	{"Mana Wyrm", RarityRare, false, "mana_wyrm.png"},
	{"Spirit Binder", RarityRare, false, "spirit_binder.png"},
	{"Obsidian Bat", RarityCommon, false, "obsidian_bat.png"},
	{"Rampager", RarityCommon, false, "rampager.png"},
	{"Aerie Guard", RarityRare, false, "aerie_guard.png"},
	{"Winged Screechers", RarityCommon, false, "winged_screechers.png"},
	{"Holy Nova", RarityEpic, false, "holy-nova.png"},
	{"Night Sentinel", RarityCommon, false, "night_sentinel.png"},
	{"Living Bomb", RarityLegendary, false, "living-bomb.png"},
	{"Corpse Catapult", RarityRare, false, "corpse_catapult.png"},
	{"Magma Goliath", RarityEpic, false, "magma_goliath.png"},
	{"Earth and Moon", RarityLegendary, false, "earth-and-moon.png"},
	{"Reanimator", RarityCommon, false, "reanimator.png"},
	{"Polymorph", RarityRare, false, "polymorph.png"},
	{"Razorboar", RarityCommon, false, "razorboar.png"},
	{"Dino Steed", RarityCommon, false, "dino_steed.png"},
	{"Grizzly Marksman", RarityCommon, false, "grizzly_marksman.png"},
	{"Kamikaze Aviator", RarityRare, false, "kamikaze_aviator.png"},
	{"Bone Revenants", RarityCommon, false, "bone_revenants.png"},
	{"Smoke Bomb", RarityCommon, false, "smoke-bomb.png"},
	{"Earthhoof", RarityRare, false, "earthhoof.png"},
	{"Whelp Eggs", RarityRare, false, "whelp-eggs.png"},
	{"Voodoo Hexer", RarityRare, false, "voodoo_hexer.png"},
	{"Nightfang", RarityCommon, false, "nightfang.png"},
	{"Crimson Raider", RarityCommon, false, "crimson_raider.png"},

	// Champion units
	{"Bloodmage Thalor", RarityEpic, true, "bloodmage_thalor.png"},
	{"Warg Lord", RarityEpic, true, "warg_lord.png"},
	{"Sorceress Glacia", RarityEpic, true, "sorceress_glacia.png"},
	{"The Warden", RarityRare, true, "the_warden.png"},
	{"Aethelion Ragestorm", RarityLegendary, true, "aethelion.png"},
	{"Lord of Flame", RarityLegendary, true, "lord_of_flame.png"},
	{"Silvanus Windarrow", RarityEpic, true, "silvanus_windarrow.png"},
	{"Rusttooth", RarityRare, true, "rusttooth.png"},
	{"Sir Kaelen Lightbane", RarityEpic, true, "sir_kaelen_lightbane.png"},
}

func ListMinis() []UnitMeta {
	var minis []UnitMeta
	for _, unit := range unitRegistry {
		if !unit.IsChampion {
			minis = append(minis, unit)
		}
	}
	return minis
}

func ListChampions() []UnitMeta {
	var champions []UnitMeta
	for _, unit := range unitRegistry {
		if unit.IsChampion {
			champions = append(champions, unit)
		}
	}
	return champions
}

func GetUnitMeta(unitID string) *UnitMeta {
	for _, unit := range unitRegistry {
		if unit.ID == unitID {
			return &unit
		}
	}
	return nil
}
