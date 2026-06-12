package models

import "time"

type Order struct {
	ID           string    `json:"id" firestore:"id"`
	UserID       string    `json:"user_id" firestore:"user_id"`
	MainItems    []Item    `json:"mainItems" firestore:"mainItems"`
	AddOnItems   []Item    `json:"addOnItems" firestore:"addOnItems"`
	Equipment    Equipment `json:"equipment" firestore:"equipment"`
	Shipping     Shipping  `json:"shipping" firestore:"shipping"`
	Payment      Payment   `json:"payment" firestore:"payment"`
	Totals       Totals    `json:"totals" firestore:"totals"`
	SlipURL      string    `json:"slip_url" firestore:"slip_url"`
	HomeImageURL string    `json:"home_image_url" firestore:"home_image_url"`
	CreatedAt    time.Time `json:"created_at" firestore:"created_at"` // ✨ เพิ่มตรงนี้
	UpdatedAt    time.Time `json:"updated_at" firestore:"updated_at"` // ✨ เพิ่มตรงนี้
	Status       string    `json:"status" firestore:"status"`         // ✨ เพิ่มตรงนี้สำคัญมาก!
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
