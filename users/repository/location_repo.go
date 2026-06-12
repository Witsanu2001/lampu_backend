package repository

import (
	"context"
	"users/models"

	"cloud.google.com/go/firestore"
)

type LocationRepository struct {
	Client *firestore.Client
}

func NewLocationRepository(client *firestore.Client) *LocationRepository {
	return &LocationRepository{Client: client}
}

func (r *LocationRepository) SaveLocation(ctx context.Context, location models.Location) error {
	// ถ้ามี ID ส่งมา ให้ใช้ ID เดิม (เพื่ออัปเดต), ถ้าไม่มี ให้ Firestore สร้าง ID ใหม่ให้
	if location.ID != "" {
		_, err := r.Client.Collection("locations").Doc(location.ID).Set(ctx, location)
		return err
	}

	// สร้างใหม่
	docRef, _, err := r.Client.Collection("locations").Add(ctx, location)
	if err == nil {
		// อัปเดต ID กลับเข้าไปใน Document
		_, err = docRef.Set(ctx, map[string]interface{}{"id": docRef.ID}, firestore.MergeAll)
	}
	return err
}
