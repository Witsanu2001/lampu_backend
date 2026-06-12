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
	// 🌟 1. ถ้าที่อยู่นี้ถูกตั้งเป็นค่าเริ่มต้น (IsDefault = true)
	if location.IsDefault {
		// ให้ค้นหาที่อยู่เดิมของ User คนนี้ที่เคยเป็น true
		iter := r.Client.Collection("locations").
			Where("user_id", "==", location.UserID).
			Where("is_default", "==", true).
			Documents(ctx)

		for {
			doc, err := iter.Next()
			if err != nil {
				break // ถ้าหาไม่เจอหรือหมดแล้วให้ออกลูป
			}
			// เจอของเก่าแล้ว สั่งเปลี่ยนเป็น false ซะ!
			if doc.Ref.ID != location.ID {
				doc.Ref.Update(ctx, []firestore.Update{
					{Path: "is_default", Value: false},
				})
			}
		}
	}

	// 2. ค่อยบันทึกที่อยู่ใหม่/แก้ไขที่อยู่เดิม ตามโค้ดเดิมของคุณ
	if location.ID != "" {
		_, err := r.Client.Collection("locations").Doc(location.ID).Set(ctx, location)
		return err
	}

	docRef, _, err := r.Client.Collection("locations").Add(ctx, location)
	if err == nil {
		_, err = docRef.Set(ctx, map[string]interface{}{"id": docRef.ID}, firestore.MergeAll)
	}
	return err
}

// ฟังก์ชันสำหรับลบที่อยู่จัดส่ง
func (r *LocationRepository) DeleteLocation(ctx context.Context, id string) error {
	_, err := r.Client.Collection("locations").Doc(id).Delete(ctx)
	return err
}
