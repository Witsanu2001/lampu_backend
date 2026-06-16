package models

import "time"

// 🌟 วัตถุงานย่อยที่อยู่ในฟังก์ชัน active_jobs ของตาราง jobs
type ActiveJobItem struct {
	OrderID     string    `json:"order_id" firestore:"order_id"`
	Status      string    `json:"status" firestore:"status"`
	QueueNumber int       `json:"queue_number" firestore:"queue_number"`
	AssignedAt  time.Time `json:"assigned_at" firestore:"assigned_at"`
}

// 🌟 เอกสารหลักในตาราง jobs (ใช้ rider_id เป็น Doc ID)
type RiderJobDoc struct {
	RiderID    string          `firestore:"rider_id"`
	UpdatedAt  time.Time       `firestore:"updated_at"`
	ActiveJobs []ActiveJobItem `firestore:"active_jobs"`
}

type Order struct {
	ID           string       `json:"id" firestore:"id"`
	UserID       string       `json:"user_id" firestore:"user_id"`
	MainItems    []Item       `json:"mainItems" firestore:"mainItems"`
	AddOnItems   []Item       `json:"addOnItems" firestore:"addOnItems"`
	Equipment    Equipment    `json:"equipment" firestore:"equipment"`
	Shipping     Shipping     `json:"shipping" firestore:"shipping"`
	Payment      Payment      `json:"payment" firestore:"payment"`
	Totals       Totals       `json:"totals" firestore:"totals"`
	SlipURL      string       `json:"slip_url" firestore:"slip_url"`
	HomeImageURL string       `json:"home_image_url" firestore:"home_image_url"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	Status       string       `json:"status" firestore:"status"`
	RiderID      string       `json:"rider_id" firestore:"rider_id"`
	RiderProfile *UserProfile `json:"rider_profile,omitempty"`
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

// 🌟 โครงสร้างข้อมูลสำหรับส่งออก API (รวมข้อมูลคิวงาน และ รายละเอียดออเดอร์)
type JobDetailResponse struct {
	OrderID      string    `json:"order_id"`
	Status       string    `json:"status"`
	QueueNumber  int       `json:"queue_number"`
	AssignedAt   time.Time `json:"assigned_at"`
	Equipment    Equipment `json:"equipment"`
	Shipping     Shipping  `json:"shipping"`
	OrderDetails Order     `json:"order_details"`
}

type StoveDetailResponse struct {
	OrderID      string         `json:"order_id"`
	Status       string         `json:"status"`
	Equipment    StoveEquipment `json:"equipment"`
	Shipping     Shipping       `json:"shipping"`
	RiderProfile UserProfile    `json:"rider_profile,omitempty"`
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

type StoveEquipment struct {
	NeedEquipment bool `json:"needEquipment" firestore:"needEquipment"`
	StoveCount    int  `json:"stoveCount"`
	PanCount      int  `json:"panCount"`
}
