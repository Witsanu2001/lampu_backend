package repository

import (
	"context"
	"fmt"
	"log"
	"time"
	"users/models"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/auth"
	"firebase.google.com/go/v4/db"
	"google.golang.org/api/iterator"
)

type UserRepository struct {
	client     *firestore.Client
	rtdbClient *db.Client
}

func NewUserRepository(client *firestore.Client, rtdbClient *db.Client) *UserRepository {
	return &UserRepository{client: client, rtdbClient: rtdbClient}
}

func (r *UserRepository) SyncUserToRTDB(ctx context.Context, uid string) error {
	// ดึงข้อมูลล่าสุดจาก Firestore
	user, err := r.GetByID(ctx, uid)
	if err != nil {
		return err
	}

	// เขียนลง RTDB ที่ path: live_users/{uid}
	ref := r.rtdbClient.NewRef("live_users/" + uid)
	return ref.Set(ctx, user)
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

func (r *UserRepository) Save(ctx context.Context, user models.UserProfile) error {
	_, err := r.client.Collection("users").Doc(user.UID).Set(ctx, user)
	return err
}
func (r *UserRepository) GetAll(ctx context.Context) ([]models.UserProfile, error) {
	var users []models.UserProfile
	iter := r.client.Collection("users").Documents(ctx)
	defer iter.Stop()

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
		users = append(users, user)
	}

	return users, nil
}

func (r *UserRepository) UpdateUser(ctx context.Context, uid string, newRole string) error {
	_, err := r.client.Collection("users").Doc(uid).Update(ctx, []firestore.Update{
		{Path: "Role", Value: newRole}, // 🌟 แก้จาก "role" เป็น "Role"
	})
	return err
}

func (r *UserRepository) DeleteUser(ctx context.Context, uid string) error {
	_, err := r.client.Collection("users").Doc(uid).Delete(ctx)
	return err
}

func (r *UserRepository) BlockUser(ctx context.Context, authClient *auth.Client, uid string) error {
	// 1. อัปเดตใน Firestore
	_, err := r.client.Collection("users").Doc(uid).Update(ctx, []firestore.Update{
		{Path: "is_blocked", Value: true},
	})
	if err != nil {
		return err
	}

	// 2. อัปเดต Custom Claims
	claims := map[string]interface{}{
		"is_blocked": true,
	}
	if err := authClient.SetCustomUserClaims(ctx, uid, claims); err != nil {
		log.Printf("Failed to set custom claims: %v", err)
	}

	// 3. สั่งยกเลิก Token
	if err := authClient.RevokeRefreshTokens(ctx, uid); err != nil {
		log.Printf("Failed to revoke refresh tokens: %v", err)
	}

	// 🌟 4. ซิงค์ข้อมูลล่าสุดลง RTDB (live_users/{uid}) เพื่อให้หน้าบ้านรับรู้ทันที
	if err := r.SyncUserToRTDB(ctx, uid); err != nil {
		log.Printf("Failed to sync blocked status to RTDB: %v", err)
	}

	return nil
}

// -----------------------------------------
// ส่วนฟังก์ชันปลดบล็อค (Unblock)
func (r *UserRepository) UnblockUser(ctx context.Context, authClient *auth.Client, uid string) error {
	// 1. อัปเดตใน Firestore
	_, err := r.client.Collection("users").Doc(uid).Update(ctx, []firestore.Update{
		{Path: "is_blocked", Value: false},
	})
	if err != nil {
		return err
	}

	// 2. เคลียร์ Custom Claims
	claims := map[string]interface{}{
		"is_blocked": false,
	}
	if err := authClient.SetCustomUserClaims(ctx, uid, claims); err != nil {
		log.Printf("Failed to remove custom claims: %v", err)
	}

	// 🌟 3. ซิงค์ข้อมูลล่าสุดลง RTDB (live_users/{uid}) เพื่อปลดล็อกการมองเห็น
	if err := r.SyncUserToRTDB(ctx, uid); err != nil {
		log.Printf("Failed to sync unblocked status to RTDB: %v", err)
	}

	return nil
}

func (r *UserRepository) GetRiders(ctx context.Context) ([]models.RiderWithJobsResponse, error) {
	var riders []models.RiderWithJobsResponse
	iter := r.client.Collection("users").Where("Role", "==", "rider").Documents(ctx)
	defer iter.Stop()

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
		var currentJobsEvent *models.JobsEvent
		jobsEventID := fmt.Sprintf("%s_%s", user.UID, todayDateStr)
		jobDoc, err := r.client.Collection("jobs_event").Doc(jobsEventID).Get(ctx)
		if err == nil && jobDoc.Exists() {
			var event models.JobsEvent
			if err := jobDoc.DataTo(&event); err == nil {
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
