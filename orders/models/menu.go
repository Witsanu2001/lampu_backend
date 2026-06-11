package models

import "time"

type Menu struct {
	ID              string    `json:"id" firestore:"id,omitempty"`
	NameMenu        string    `json:"name_menu" firestore:"name_menu"`
	DescriptionMenu string    `json:"description_menu" firestore:"description_menu"`
	PriceMenu       float64   `json:"price_menu" firestore:"price_menu"`
	TypeMenu        string    `json:"type_menu" firestore:"type_menu"`
	Available       bool      `json:"available" firestore:"available"`
	ImageURLMenu    string    `json:"image_url_menu" firestore:"image_url_menu"`
	CreatedAt       time.Time `json:"created_at" firestore:"created_at"`
}
