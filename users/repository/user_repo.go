package repository

import (
	"context"
	"fmt"
	"time"
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

func (r *UserRepository) GetRiders(ctx context.Context) ([]models.RiderWithJobsResponse, error) {
	var riders []models.RiderWithJobsResponse
	iter := r.client.Collection("users").Where("Role", "==", "rider").Documents(ctx)
	defer iter.Stop()

	// หาค่าวันที่ของวันนี้ (เช่น "2026-06-18")
	todayDateStr := time.Now().Format("2006-01-02")

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var user models.UserProfile
		if err := doc.DataTo(&user); err != nil {
			return nil, err
		}

		// 🌟 1. สร้างตัวแปรรับค่า (ถ้าไม่มีงานจะเป็น nil อัตโนมัติ)
		var currentJobsEvent *models.JobsEvent

		// 🌟 2. ประกอบ Document ID (รูปแบบเดียวกับตอนที่เราบันทึกข้อมูล)
		jobsEventID := fmt.Sprintf("%s_%s", user.UID, todayDateStr)

		// 🌟 3. หยิบ Document นั้นขึ้นมาตรงๆ (เร็วกว่าการใช้ Where)
		jobDoc, err := r.client.Collection("jobs_event").Doc(jobsEventID).Get(ctx)

		// ถ้าหยิบมาได้และมีข้อมูลอยู่จริง ให้แปลงใส่ Struct
		if err == nil && jobDoc.Exists() {
			var event models.JobsEvent
			if err := jobDoc.DataTo(&event); err == nil {
				// ชี้ pointer ไปที่ข้อมูลที่ดึงมาได้
				currentJobsEvent = &event
			}
		}

		// 🌟 4. ประกอบร่างส่งกลับ
		riders = append(riders, models.RiderWithJobsResponse{
			UserProfile: user,
			JobsEvent:   currentJobsEvent, // ส่งเป็น Object เดี่ยวๆ (ถ้าไม่มีข้อมูลจะเป็น null)
		})
	}

	return riders, nil
}

// func (r *UserRepository) GetRiders_JobsEvent(ctx context.Context) ([]models.RiderWithJobsResponse, error) {
// 	var riders []models.RiderWithJobsResponse
// 	iter := r.client.Collection("users").Where("Role", "==", "rider").Documents(ctx)
// 	defer iter.Stop()

// 	for {
// 		doc, err := iter.Next()
// 		if err == iterator.Done {
// 			break
// 		}
// 		if err != nil {
// 			return nil, err
// 		}

// 		var user models.UserProfile
// 		if err := doc.DataTo(&user); err != nil {
// 			return nil, err
// 		}

// 		var riderJobsEvents []models.JobsEvent

// 		// 🌟 ดึงข้อมูลจากตาราง jobs_event
// 		jobsIter := r.client.Collection("jobs_event").Where("rider_id", "==", user.UID).Documents(ctx)

// 		for {
// 			jobDoc, jobErr := jobsIter.Next()
// 			if jobErr == iterator.Done {
// 				break
// 			}
// 			if jobErr != nil {
// 				log.Printf("Error fetching jobs_event for rider %s: %v", user.UID, jobErr)
// 				break
// 			}

// 			// 🌟 แมปข้อมูลเข้า Struct JobsEvent โดยตรง
// 			var event models.JobsEvent
// 			if err := jobDoc.DataTo(&event); err == nil {
// 				riderJobsEvents = append(riderJobsEvents, event)
// 			} else {
// 				log.Printf("Error parsing jobs_event data: %v", err)
// 			}
// 		}
// 		jobsIter.Stop()

// 		// 🌟 ประกอบร่างแล้วยัดเข้า Array
// 		riders = append(riders, models.RiderWithJobsResponse{
// 			UserProfile: user,
// 			JobsEvents:  riderJobsEvents,
// 		})
// 	}

// 	return riders, nil
// }
