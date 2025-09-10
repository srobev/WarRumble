package types

type ShopSlot struct {
	Slot       int    `json:"slot"` // 0..80
	UnitID     string `json:"unitId"`
	IsChampion bool   `json:"isChampion"`
	PriceGold  int64  `json:"priceGold"`
	Sold       bool   `json:"sold"`
}

type ShopRoll struct {
	Slots   []ShopSlot `json:"slots"`   // len 81
	Version int        `json:"version"` // bump when format changes
}

type GetShopRollReq struct{}
type RerollShopReq struct {
	Nonce string `json:"nonce"`
}
type BuyShopSlotReq struct {
	Slot  int    `json:"slot"`
	Nonce string `json:"nonce"`
}

type ShopRollSynced struct {
	Roll ShopRoll `json:"roll"`
}
type BuyShopResult struct {
	Slot      int    `json:"slot"`
	UnitID    string `json:"unitId"`
	Gold      int64  `json:"gold"`
	Shards    int    `json:"shards"`    // total shards for this unit after buy
	Rank      int    `json:"rank"`      // unit rank after buy
	Threshold int    `json:"threshold"` // rarity threshold (3/10/25/25)
}
