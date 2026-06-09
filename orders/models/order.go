package models

import "time"

type Order struct {
	ID         string    `json:"id" firestore:"id,omitempty"`
	UserID     string    `json:"user_id" firestore:"user_id"`
	Items      []Item    `json:"items" firestore:"items"`
	TotalPrice float64   `json:"total_price" firestore:"total_price"`
	Status     string    `json:"status" firestore:"status"` // เช่น pending, paid, completed, cancelled
	CreatedAt  time.Time `json:"created_at" firestore:"created_at"`
}

type Item struct {
	ProductID   string  `json:"product_id" firestore:"product_id"`
	ProductName string  `json:"product_name" firestore:"product_name"`
	Quantity    int     `json:"quantity" firestore:"quantity"`
	Price       float64 `json:"price" firestore:"price"`
}
