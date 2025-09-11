package game

// Offer represents a single shop offer
type Offer struct {
	OfferID   string  `json:"offer_id"`
	Type      string  `json:"offer_type"` // "mini"|"champion"|"perk"
	Unit      string  `json:"unit"`
	PerkID    *string `json:"perk_id,omitempty"`
	PriceGold int     `json:"price_gold"`
	Portrait  string  `json:"portrait"`
	Desc      string  `json:"desc,omitempty"`
}

// Grid represents the shop grid of offers
type Grid struct {
	Offers []Offer `json:"offers"`
}

// Ensure Grid type is exported and available in game package
var _ Grid = Grid{} // Type check

// FetchGrid retrieves current shop offers
func FetchGrid() (Grid, error) {
	return GetJSON[Grid]("/api/shop/grid")
}

// PurchaseRequest represents a purchase request
type PurchaseRequest struct {
	OfferID string `json:"offer_id"`
}

// Purchase purchases a shop offer
func Purchase(offerID string) error {
	req := PurchaseRequest{OfferID: offerID}
	_, err := PostJSON[PurchaseRequest, struct{}](req, "/api/shop/purchase")
	return err
}

// Reroll performs a shop reroll
func Reroll() error {
	_, err := PostJSON[struct{}, struct{}](struct{}{}, "/api/shop/reroll")
	return err
}
