package repository

import (
	"context"
	"jobs/models" // 🌟 เปลี่ยนเป็น path โมดูลของคุณ

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type JobRepository struct {
	client *firestore.Client
}

func NewJobRepository(client *firestore.Client) *JobRepository {
	return &JobRepository{client: client}
}

// GetJobsByUser ดึงรายการคิวงานของไรเดอร์ พร้อมทั้งแมปข้อมูลรายละเอียดออเดอร์มาให้ครบถ้วน
func (r *JobRepository) GetJobsByUser(ctx context.Context, userID string) ([]models.JobDetailResponse, error) {
	var responseList []models.JobDetailResponse

	// 1. ดึงเอกสารงานของไรเดอร์คนนี้ (ใช้ userID เป็น Document ID โดยตรง)
	docSnap, err := r.client.Collection("jobs").Doc(userID).Get(ctx)
	if err != nil {
		// ถ้าไม่พบเอกสาร (ยังไม่มีงาน) ให้รีเทิร์นอาเรย์ว่างกลับไป (ไม่นับเป็น error ของระบบ)
		if status.Code(err) == codes.NotFound {
			return []models.JobDetailResponse{}, nil
		}
		return nil, err
	}

	var riderDoc models.RiderJobDoc
	if err := docSnap.DataTo(&riderDoc); err != nil {
		return nil, err
	}

	// หากไม่มีคิวงานค้างอยู่ ให้รีเทิร์นอาเรย์ว่าง
	if len(riderDoc.ActiveJobs) == 0 {
		return []models.JobDetailResponse{}, nil
	}

	// 2. ⚡ เตรียมดึงรายละเอียดจากตาราง "orders" แบบมัดรวมก้อนเดียว (GetAll)
	var docRefs []*firestore.DocumentRef
	for _, jobItem := range riderDoc.ActiveJobs {
		orderRef := r.client.Collection("orders").Doc(jobItem.OrderID)
		docRefs = append(docRefs, orderRef)
	}

	// ยิงไปดึงข้อมูลคำสั่งซื้อทั้งหมดในครั้งเดียว
	orderSnaps, err := r.client.GetAll(ctx, docRefs)
	if err != nil {
		return nil, err
	}

	// แปลงข้อมูลคำสั่งซื้อที่ได้มาเก็บไว้ในรูปของ Map [order_id] -> ข้อมูลออเดอร์
	ordersMap := make(map[string]models.Order)
	for _, snap := range orderSnaps {
		if !snap.Exists() {
			continue
		}
		var order models.Order
		if err := snap.DataTo(&order); err == nil {
			ordersMap[snap.Ref.ID] = order
		}
	}

	// 3. ประกอบร่างข้อมูลคิวงานเข้ากับรายละเอียดของออเดอร์นั้นๆ
	for _, jobItem := range riderDoc.ActiveJobs {
		orderDetails, exists := ordersMap[jobItem.OrderID]

		// กรณีฉุกเฉิน: ถ้าหากออเดอร์ตัวจริงถูกลบออกจากระบบไปแล้ว
		if !exists {
			orderDetails = models.Order{ID: jobItem.OrderID, Status: "unknown"}
		}

		// รวมร่างเข้าโครงสร้างข้อมูลสำหรับ Response
		detailRes := models.JobDetailResponse{
			OrderID:      jobItem.OrderID,
			Status:       jobItem.Status,
			QueueNumber:  jobItem.QueueNumber,
			AssignedAt:   jobItem.AssignedAt,
			OrderDetails: orderDetails,
		}
		responseList = append(responseList, detailRes)
	}

	return responseList, nil
}
