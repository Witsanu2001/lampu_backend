package models

import "time"

type UserProfile struct {
	UID         string    `json:"uid"`
	Email       string    `json:"email"`
	DisplayName string    `json:"displayName"`
	PhotoURL    string    `json:"photoURL"`
	Provider    string    `json:"provider"`
	LastLogin   time.Time `json:"lastLogin"`
	Role        string    `json:"role"`
}

type JobsEvent struct {
	RiderId          string    `json:"rider_id" firestore:"rider_id"`
	Date             string    `json:"date" firestore:"date"`
	Status           string    `json:"status" firestore:"status"`
	TotalDeliveryFee float64   `json:"total_delivery_fee" firestore:"total_delivery_fee"`
	TotalOrderSets   int       `json:"total_order_sets" firestore:"total_order_sets"`
	UpdatedAt        time.Time `json:"updated_at" firestore:"updated_at"`
}

type RiderWithJobsResponse struct {
	UserProfile
	JobsEvent *JobsEvent `json:"jobs_event"`
}

// type RiderWithJobsResponse struct {
// 	UserProfile
// 	JobsEvents []JobsEvent `json:"jobs_events"`
// }
