package models

import "time"

type Menu struct {
	ID          string    `json:"id" firestore:"id,omitempty"`
	Name        string    `json:"name" firestore:"name"`
	Description string    `json:"description" firestore:"description"`
	Price       float64   `json:"price" firestore:"price"`
	Category    string    `json:"category" firestore:"category"`   // ✨ เพิ่มเข้ามา
	Available   bool      `json:"available" firestore:"available"` // ✨ เพิ่มเข้ามา
	ImageURL    string    `json:"image_url" firestore:"image_url"`
	CreatedAt   time.Time `json:"created_at" firestore:"created_at"`
}
