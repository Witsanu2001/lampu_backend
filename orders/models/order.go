package models

import "time"

type Order struct {
	ID           string    `json:"id"`
	MainItems    []Item    `json:"mainItems"`
	AddOnItems   []Item    `json:"addOnItems"`
	Equipment    Equipment `json:"equipment"`
	Shipping     Shipping  `json:"shipping"`
	Payment      Payment   `json:"payment"`
	Totals       Totals    `json:"totals"`
	SlipURL      string    `json:"slip_url"`
	HomeImageURL string    `json:"home_image_url"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Item struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
	Subtotal float64 `json:"subtotal"`
}

type Equipment struct {
	StoveCount    int     `json:"stoveCount"`
	PanCount      int     `json:"panCount"`
	CharcoalCount int     `json:"charcoalCount"`
	ExtraStoves   int     `json:"extraStoves"`
	ExtraPans     int     `json:"extraPans"`
	StoveFee      float64 `json:"stoveFee"`
	PanFee        float64 `json:"panFee"`
	CharcoalFee   float64 `json:"charcoalFee"`
}

type Location struct {
	Lat float64 `json:"lat"`
	Lng float64 `json:"lng"`
}

type Shipping struct {
	Address   string   `json:"address"`
	Location  Location `json:"location"`
	FeePerSet float64  `json:"feePerSet"`
	TotalFee  float64  `json:"totalFee"`
}

type Payment struct {
	Method  string `json:"method"`
	HasSlip bool   `json:"hasSlip"`
}

type Totals struct {
	CartTotal   float64 `json:"cartTotal"`
	AddOnTotal  float64 `json:"addOnTotal"`
	ShippingFee float64 `json:"shippingFee"`
	GrandTotal  float64 `json:"grandTotal"`
}
