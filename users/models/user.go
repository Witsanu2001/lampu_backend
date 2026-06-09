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
