package models

import "time"

type GeoLocation struct {
	Lat float64 `json:"lat" firestore:"lat"`
	Lng float64 `json:"lng" firestore:"lng"`
}

type Location struct {
	ID          string       `json:"id" firestore:"id,omitempty"`
	UserID      string       `json:"user_id" firestore:"user_id"`
	Name        string       `json:"name" firestore:"name"`
	Phone       string       `json:"phone" firestore:"phone"`
	Details     string       `json:"details" firestore:"details"`
	Note        string       `json:"note" firestore:"note"`
	Location    *GeoLocation `json:"location" firestore:"location"`
	DeliveryFee float64      `json:"deliveryFee" firestore:"delivery_fee"`
	Distance    float64      `json:"distance" firestore:"distance"`
	IsMeetup    bool         `json:"isMeetup" firestore:"is_meetup"`
	IsDefault   bool         `json:"isDefault" firestore:"is_default"`
	CreatedAt   time.Time    `json:"created_at" firestore:"created_at"`
}
