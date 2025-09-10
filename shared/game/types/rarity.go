package types

type Rarity int

const (
	RarityCommon Rarity = iota
	RarityRare
	RarityEpic
	RarityLegendary
)

func (r Rarity) ShardsPerRank() int {
	switch r {
	case RarityCommon:
		return 3
	case RarityRare:
		return 10
	case RarityEpic, RarityLegendary:
		return 25
	default:
		return 999999
	}
}

const (
	ShopPriceMiniGold     = int64(90)
	ShopPriceChampionGold = int64(120)
)
