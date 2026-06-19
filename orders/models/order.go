package models

import "time"

type Order struct {
	ID           string      `json:"id" firestore:"id"`
	UserID       string      `json:"user_id" firestore:"user_id"`
	MainItems    []Item      `json:"mainItems" firestore:"mainItems"`
	AddOnItems   []Item      `json:"addOnItems" firestore:"addOnItems"`
	Equipment    Equipment   `json:"equipment" firestore:"equipment"`
	Shipping     Shipping    `json:"shipping" firestore:"shipping"`
	Payment      Payment     `json:"payment" firestore:"payment"`
	Totals       Totals      `json:"totals" firestore:"totals"`
	SlipURL      string      `json:"slip_url" firestore:"slip_url"`
	OldSlipURL   string      `json:"old_slip_url" firestore:"old_slip_url"`
	HomeImageURL string      `json:"home_image_url" firestore:"home_image_url"`
	CreatedAt    time.Time   `json:"created_at"`
	UpdatedAt    time.Time   `json:"updated_at"`
	Status       string      `json:"status" firestore:"status"`
	RiderID      string      `json:"rider_id" firestore:"rider_id"`
	RiderName    UserProfile `json:"rider_name" firestore:"rider_name"`
	CancelReason string      `json:"cancel_reason" firestore:"cancel_reason"`
	IsEditedSlip bool        `json:"is_edited_slip" firestore:"is_edited_slip"`
}

type Item struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
	Subtotal float64 `json:"subtotal"`
}

type Equipment struct {
	NeedEquipment bool    `json:"needEquipment" firestore:"needEquipment"`
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
	Lat float64 `json:"lat" firestore:"lat"`
	Lng float64 `json:"lng" firestore:"lng"`
}

type Shipping struct {
	LocationID string   `json:"location_id" firestore:"location_id"`
	Recipient  string   `json:"recipient" firestore:"recipient"` // เก็บชื่อ
	Phone      string   `json:"phone" firestore:"phone"`         // เก็บเบอร์
	Address    string   `json:"address" firestore:"address"`     // เก็บรายละเอียดที่อยู่
	Note       string   `json:"note" firestore:"note"`           // เก็บจุดสังเกต
	Location   Location `json:"location" firestore:"location"`
	FeePerSet  float64  `json:"feePerSet" firestore:"feePerSet"`
	TotalFee   float64  `json:"totalFee" firestore:"totalFee"`
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

type UserLocation struct {
	Name     string `firestore:"name"`
	Phone    string `firestore:"phone"`
	Details  string `firestore:"details"`
	Note     string `firestore:"note"`
	Location struct {
		Lat float64 `firestore:"lat"`
		Lng float64 `firestore:"lng"`
	} `firestore:"location"`
}

type UpdateStatusRequest struct {
	UserID string `json:"user_id" form:"user_id"`
	Status string `json:"status" form:"status"`
}

type AssignJobPayload struct {
	OrderID     string  `json:"order_id"`
	RiderID     string  `json:"rider_id"`
	QueueNumber int     `json:"queue_number"`
	OrderSetQty int     `json:"order_set_qty"`
	DeliveryFee float64 `json:"delivery_fee"`
}

type BulkAssignRequest struct {
	Jobs []AssignJobPayload `json:"jobs"`
}

type SuccessOrderSummary struct {
	OrderID    string    `json:"order_id"`
	Status     string    `json:"status"`
	Recipient  string    `json:"recipient"`
	Address    string    `json:"address"`
	GrandTotal float64   `json:"grand_total"`
	CreatedAt  time.Time `json:"created_at"`
}

type UserProfile struct {
	UID         string    `json:"uid"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	PhotoURL    string    `json:"photoURL"`
	Provider    string    `json:"provider"`
	LastLogin   time.Time `json:"lastLogin"`
	Role        string    `json:"role"`
}
