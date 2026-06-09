package repository

import (
	"context"
	"users/models"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

type UserRepository struct {
	client *firestore.Client
}

// ฟังก์ชันสร้าง Repository instance ใหม่
func NewUserRepository(client *firestore.Client) *UserRepository {
	return &UserRepository{client: client}
}

// บันทึก หรือ อัปเดตข้อมูลผู้ใช้ลง Firestore
func (r *UserRepository) Save(ctx context.Context, user models.UserProfile) error {
	_, err := r.client.Collection("users").Doc(user.UID).Set(ctx, user)
	return err
}

// 🌟 ดึงรายชื่อผู้ใช้ทั้งหมดจากคอลเลกชัน "users"
func (r *UserRepository) GetAll(ctx context.Context) ([]models.UserProfile, error) {
	var users []models.UserProfile

	// ดึงเอกสารทั้งหมดในคอลเลกชัน users
	iter := r.client.Collection("users").Documents(ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break // อ่านครบทุกด็อคคิวเมนต์แล้ว ให้จบลูป
		}
		if err != nil {
			return nil, err
		}

		var user models.UserProfile
		// แปลงข้อมูลจาก Firestore เข้าสู่โครงสร้างตัวแปร Go
		if err := doc.DataTo(&user); err != nil {
			return nil, err
		}
		users = append(users, user)
	}

	return users, nil
}

func (r *UserRepository) GetByID(ctx context.Context, uid string) (*models.UserProfile, error) {
	doc, err := r.client.Collection("users").Doc(uid).Get(ctx)
	if err != nil {
		return nil, err
	}
	var user models.UserProfile
	if err := doc.DataTo(&user); err != nil {
		return nil, err
	}
	return &user, nil
}
