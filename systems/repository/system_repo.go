package repository

import (
	"context"
	"fmt"
	"systems/models"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/db"
)

// 🌟 รับ Client มาทั้ง 2 ตัว
type SystemRepository struct {
	firestoreClient *firestore.Client
	rtdbClient      *db.Client
}

// 🌟 ตอน New ให้ส่งเข้ามาทั้ง 2 ตัว
func NewSystemRepository(firestoreClient *firestore.Client, rtdbClient *db.Client) *SystemRepository {
	return &SystemRepository{
		firestoreClient: firestoreClient,
		rtdbClient:      rtdbClient,
	}
}

func (r *SystemRepository) GetUserRole(ctx context.Context, uid string) (string, error) {
	doc, err := r.firestoreClient.Collection("users").Doc(uid).Get(ctx)
	if err != nil {
		return "", err
	}

	data := doc.Data()

	// 🌟 เปลี่ยนจาก data["role"] เป็น data["Role"]
	role, ok := data["Role"].(string)
	if !ok {
		// ถ้าหาไม่เจอหรือค่าไม่ใช่ string ให้คืนค่า Default หรือ Error
		return "", fmt.Errorf("role field not found or invalid for user: %s", uid)
	}

	return role, nil
}

func (r *SystemRepository) SaveSystemSettings(ctx context.Context, payload models.SystemSettingsPayload) error {
	_, err := r.firestoreClient.Collection("settings").Doc(payload.Project).Set(ctx, payload)
	if err != nil {
		return err
	}

	ref := r.rtdbClient.NewRef("live_settings/" + payload.Project)
	return ref.Set(ctx, payload)
}

func (r *SystemRepository) GetSystemSettings(ctx context.Context, project string) (*models.SystemSettingsPayload, error) {
	doc, err := r.firestoreClient.Collection("settings").Doc(project).Get(ctx)
	if err != nil {
		return nil, err
	}

	var settings models.SystemSettingsPayload
	if err := doc.DataTo(&settings); err != nil {
		return nil, err
	}

	return &settings, nil
}

func (r *SystemRepository) UpdatePIN(ctx context.Context, project string, newPIN string) error {
	_, err := r.firestoreClient.Collection("settings").Doc(project).Update(ctx, []firestore.Update{
		{
			Path:  "PIN",
			Value: newPIN,
		},
	})
	return err
}

func (r *SystemRepository) GetSettings(ctx context.Context, project string) (*models.SystemSettingsPayload, error) {
	doc, err := r.firestoreClient.Collection("settings").Doc(project).Get(ctx)
	if err != nil {
		return nil, err
	}
	var settings models.SystemSettingsPayload
	doc.DataTo(&settings)
	return &settings, nil
}
