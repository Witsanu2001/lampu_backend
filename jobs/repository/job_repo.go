package repository

import (
	"context"
	"jobs/models" // 🌟 เปลี่ยนเป็น path โมดูลของคุณ

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
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
			OrderID: jobItem.OrderID,
			// Status:       jobItem.Status,
			QueueNumber:  jobItem.QueueNumber,
			AssignedAt:   jobItem.AssignedAt,
			OrderDetails: orderDetails,
		}
		responseList = append(responseList, detailRes)
	}

	return responseList, nil
}

func (r *JobRepository) GetStove(ctx context.Context, userID string) ([]models.StoveDetailResponse, error) {
	// 🌟 ป้องกันไม่ให้ Return ออกไปเป็น null ถ้าไม่มีข้อมูล
	responseList := make([]models.StoveDetailResponse, 0)

	iter := r.client.Collection("orders").Where("status", "==", "pending").Documents(ctx)
	defer iter.Stop() // ปิด iterator เมื่อทำงานเสร็จ

	// 1. สร้างตัวแปรพักข้อมูล และ Map เก็บ Ref ของ Rider
	var ordersData []models.Order
	riderRefsMap := make(map[string]*firestore.DocumentRef)

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break // อ่านจบแล้ว ออกจากลูป
		}
		if err != nil {
			return nil, err
		}

		var order models.Order
		if err := doc.DataTo(&order); err != nil {
			continue // ข้ามอันที่แปลงข้อมูลไม่ได้ไป
		}

		// เผื่อในเอกสารไม่มีฟิลด์ ID บันทึกไว้ ให้ใช้ Document ID เป็น OrderID
		if order.ID == "" {
			order.ID = doc.Ref.ID
		}

		ordersData = append(ordersData, order)

		// ถ้ารายการไหนมีการแอสไซน์ไรเดอร์ไว้ ให้เก็บ Ref เอาไว้ไปดึงข้อมูล
		if order.RiderID != "" {
			riderRefsMap[order.RiderID] = r.client.Collection("users").Doc(order.RiderID)
		}
	}

	// 2. 🌟 กวาดดึงข้อมูล Rider Profiles ทั้งหมดในครั้งเดียว (ลดการสืบค้น DB ซ้ำซ้อน)
	riderProfiles := make(map[string]models.UserProfile)
	if len(riderRefsMap) > 0 {
		var refs []*firestore.DocumentRef
		for _, ref := range riderRefsMap {
			refs = append(refs, ref)
		}

		userSnaps, err := r.client.GetAll(ctx, refs)
		if err == nil {
			for _, uSnap := range userSnaps {
				if uSnap.Exists() {
					var userProfile models.UserProfile
					if err := uSnap.DataTo(&userProfile); err == nil {
						// กันเหนียวเผื่อในตาราง users ไม่ได้เซฟ UID เอาไว้
						if userProfile.UID == "" {
							userProfile.UID = uSnap.Ref.ID
						}
						riderProfiles[uSnap.Ref.ID] = userProfile
					}
				}
			}
		}
	}

	// 3. 🌟 ประกอบร่างข้อมูลออเดอร์ เข้ากับข้อมูลไรเดอร์
	for _, order := range ordersData {
		detailRes := models.StoveDetailResponse{
			OrderID: order.ID,
			Status:  order.Status,
			Equipment: models.StoveEquipment{
				NeedEquipment: order.Equipment.NeedEquipment,
				StoveCount:    order.Equipment.StoveCount,
				PanCount:      order.Equipment.PanCount,
			},
			Shipping: order.Shipping,
		}

		// ถ้ารหัส Rider ตรงกับที่เราดึงมาได้ ให้ยัด Profile เข้าไปทั้งก้อนเลย
		if order.RiderID != "" {
			if profile, exists := riderProfiles[order.RiderID]; exists {
				// 🎯 เอาเครื่องหมาย & ออก เพื่อส่งเป็น Value ธรรมดา
				detailRes.RiderProfile = profile
			}
		}

		responseList = append(responseList, detailRes)
	}

	return responseList, nil
}

func (r *JobRepository) GetStoveByRiderId(ctx context.Context, userID string) ([]models.StoveDetailResponse, error) {
	responseList := make([]models.StoveDetailResponse, 0)

	// 🌟 เพิ่ม Where("rider_id", "==", userID) เพื่อกรองเฉพาะออเดอร์ของไรเดอร์คนนี้
	iter := r.client.Collection("orders").
		Where("status", "==", "pending").
		Where("rider_id", "==", userID).
		Documents(ctx)
	defer iter.Stop() // ปิด iterator เมื่อทำงานเสร็จ

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break // อ่านจบแล้ว ออกจากลูป
		}
		if err != nil {
			return nil, err
		}

		var order models.Order
		if err := doc.DataTo(&order); err != nil {
			continue // ข้ามอันที่แปลงข้อมูลไม่ได้ไป
		}

		// เผื่อในเอกสารไม่มีฟิลด์ ID บันทึกไว้ ให้ใช้ Document ID เป็น OrderID
		if order.ID == "" {
			order.ID = doc.Ref.ID
		}

		// 3. Map ข้อมูลใส่โครงสร้าง Response
		detailRes := models.StoveDetailResponse{
			OrderID: order.ID,
			Status:  order.Status,
			Equipment: models.StoveEquipment{
				NeedEquipment: order.Equipment.NeedEquipment,
				StoveCount:    order.Equipment.StoveCount,
				PanCount:      order.Equipment.PanCount,
			},
			Shipping: order.Shipping,
		}

		responseList = append(responseList, detailRes)
	}

	return responseList, nil
}
