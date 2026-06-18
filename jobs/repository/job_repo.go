package repository

import (
	"context"
	"jobs/models" // 🌟 เปลี่ยนเป็น path โมดูลของคุณ
	"sort"
	"time"

	"cloud.google.com/go/firestore"
	"firebase.google.com/go/v4/db"
	"google.golang.org/api/iterator"
)

type JobRepository struct {
	client     *firestore.Client
	RTDBClient *db.Client
}

func NewJobRepository(client *firestore.Client, rtdbClient *db.Client) *JobRepository {
	return &JobRepository{
		client:     client,
		RTDBClient: rtdbClient,
	}
}

func (r *JobRepository) GetJobsByUser(ctx context.Context, userID string) ([]models.Order, error) {
	responseList := make([]models.Order, 0)
	jobsRef := r.RTDBClient.NewRef("live_jobs/" + userID)

	var liveJobs map[string]struct {
		Status      string `json:"status"`
		QueueNumber int    `json:"queue_number"`
		UpdatedAt   int64  `json:"updated_at"`
	}

	if err := jobsRef.Get(ctx, &liveJobs); err != nil {
		return nil, err
	}

	if len(liveJobs) == 0 {
		return responseList, nil
	}

	var docRefs []*firestore.DocumentRef
	for orderID := range liveJobs {
		docRefs = append(docRefs, r.client.Collection("orders").Doc(orderID))
	}

	orderSnaps, err := r.client.GetAll(ctx, docRefs)
	if err != nil {
		return nil, err
	}

	type jobTemp struct {
		QueueNumber int
		OrderData   models.Order
	}
	var tempJobs []jobTemp

	for _, snap := range orderSnaps {
		if !snap.Exists() {
			continue
		}

		var order models.Order
		if err := snap.DataTo(&order); err == nil {
			if order.ID == "" {
				order.ID = snap.Ref.ID
			}

			if order.Status == "ready" || order.Status == "shipping" {
				queueNum := liveJobs[order.ID].QueueNumber

				tempJobs = append(tempJobs, jobTemp{
					QueueNumber: queueNum,
					OrderData:   order,
				})
			}
		}
	}

	sort.SliceStable(tempJobs, func(i, j int) bool {
		statusI := tempJobs[i].OrderData.Status
		statusJ := tempJobs[j].OrderData.Status

		// ดัน shipping ขึ้นไปก่อน
		if statusI == "shipping" && statusJ != "shipping" {
			return true
		}
		if statusI != "shipping" && statusJ == "shipping" {
			return false
		}

		// กรณีสถานะเหมือนกัน ให้เรียงตาม QueueNumber จากน้อยไปมาก
		return tempJobs[i].QueueNumber < tempJobs[j].QueueNumber
	})

	for _, tj := range tempJobs {
		responseList = append(responseList, tj.OrderData)
	}

	return responseList, nil
}

func (r *JobRepository) GetJobByID(ctx context.Context, orderID string) (*models.Order, error) {
	// ใช้ .Doc().Get() เพื่อดึงข้อมูลเอกสารแบบเจาะจง ID
	snap, err := r.client.Collection("orders").Doc(orderID).Get(ctx)
	if err != nil {
		return nil, err
	}

	var order models.Order
	if err := snap.DataTo(&order); err != nil {
		return nil, err
	}
	return &order, nil
}

// อย่าลืมเช็ก import ว่ามี "time" อยู่ด้วยนะครับ
func (r *JobRepository) GetHistory(ctx context.Context, userID string, dateStr string, page int, limit int) ([]models.Order, error) {
	// ป้องกันการ Return เป็น null
	responseList := make([]models.Order, 0)

	// 🌟 1. สร้าง Base Query ค้นหาจาก Firestore
	query := r.client.Collection("orders").
		Where("rider_id", "==", userID).
		Where("status", "in", []string{"delivered", "success"})

	// 🌟 2. ถ้ามีการส่งวันที่มา ให้เพิ่มเงื่อนไขค้นหา
	if dateStr != "" {
		loc, _ := time.LoadLocation("Asia/Bangkok")
		parsedDate, err := time.ParseInLocation("2006-01-02", dateStr, loc)
		if err != nil {
			return nil, err
		}

		startOfDay := parsedDate
		endOfDay := parsedDate.Add(24 * time.Hour).Add(-time.Nanosecond)

		query = query.Where("updated_at", ">=", startOfDay).
			Where("updated_at", "<=", endOfDay)
	}

	// 🌟 3. เรียงลำดับจากล่าสุดไปเก่าสุด
	query = query.OrderBy("updated_at", firestore.Desc)

	// 🌟 4. ระบบ Pagination (หน้าละกี่รายการ)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10 // กำหนดค่าเริ่มต้นเป็น 10 เผื่อหน้าบ้านไม่ได้ส่งมา
	}

	// คำนวณข้ามรายการ (Offset) เช่น หน้า 2 (limit 10) จะต้องข้าม 10 รายการแรก
	offset := (page - 1) * limit
	query = query.Offset(offset).Limit(limit)

	// 🌟 5. สั่งดึงข้อมูลจาก Firestore
	iter := query.Documents(ctx)
	defer iter.Stop()

	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}

		var order models.Order
		if err := doc.DataTo(&order); err == nil {
			if order.ID == "" {
				order.ID = doc.Ref.ID
			}
			responseList = append(responseList, order)
		}
	}

	return responseList, nil
}

func (r *JobRepository) GetJobSummary(ctx context.Context, riderID string, dateStr string) (*models.JobSummaryResponse, error) {

	// 🌟 1. ค้นหาจากฟิลด์ "date" ตรงๆ ได้เลย ไม่ต้องแปลงเวลาเป็น startOfDay / startOfTomorrow แล้ว
	query := r.client.Collection("jobs_event").
		Where("rider_id", "==", riderID).
		Where("date", "==", dateStr) // ใช้ dateStr ("2026-06-19") เทียบได้เลย

	snapshots, err := query.Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	// ตัวแปรเก็บยอดรวม
	totalRounds := 0
	totalOrderSets := 0
	totalDeliveryFee := 0

	for _, snap := range snapshots {
		var eventData map[string]interface{}
		if err := snap.DataTo(&eventData); err != nil {
			continue
		}

		// 🌟 2. นับจำนวนรอบ จากขนาดของ Array "job_ids"
		if jobIDs, ok := eventData["job_ids"].([]interface{}); ok {
			totalRounds += len(jobIDs)
		}

		// 🌟 3. ดึงค่าเงินจาก "total_delivery_fee"
		if fee, ok := eventData["total_delivery_fee"].(int64); ok {
			totalDeliveryFee += int(fee)
		} else if feeFloat, ok := eventData["total_delivery_fee"].(float64); ok {
			totalDeliveryFee += int(feeFloat)
		}

		// 🌟 4. ดึงจำนวนชุดหมูกระทะจาก "total_order_sets"
		if sets, ok := eventData["total_order_sets"].(int64); ok {
			totalOrderSets += int(sets)
		} else if setsFloat, ok := eventData["total_order_sets"].(float64); ok {
			totalOrderSets += int(setsFloat)
		}
	}

	// 🌟 5. ส่งผลลัพธ์กลับไป (จัดแมปเข้า Struct ใหม่ที่แก้ไขแล้ว)
	result := &models.JobSummaryResponse{
		TotalRounds:      totalRounds,
		TotalOrderSets:   totalOrderSets,
		TotalDeliveryFee: totalDeliveryFee,
	}

	return result, nil
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

func (r *JobRepository) GetStoveSuccess(ctx context.Context, userID string, dateStr string, page int, limit int) ([]models.StoveDetailResponse, error) {
	// 🌟 ป้องกันไม่ให้ Return ออกไปเป็น null ถ้าไม่มีข้อมูล
	responseList := make([]models.StoveDetailResponse, 0)

	// 🌟 1. สร้าง Base Query ค้นหาจากตาราง "orders" ของไรเดอร์คนนี้
	query := r.client.Collection("orders").
		Where("rider_id", "==", userID).
		Where("status", "==", "success")

	// 🌟 2. กรองตามวันที่ (ถ้ามี)
	if dateStr != "" {
		loc, _ := time.LoadLocation("Asia/Bangkok")
		parsedDate, err := time.ParseInLocation("2006-01-02", dateStr, loc)
		if err != nil {
			return nil, err
		}

		startOfDay := parsedDate
		endOfDay := parsedDate.Add(24 * time.Hour).Add(-time.Nanosecond)

		// ค้นหาจากวันที่เก็บเตาสำเร็จ (updated_at)
		query = query.Where("updated_at", ">=", startOfDay).
			Where("updated_at", "<=", endOfDay)
	}

	// 🌟 3. เรียงลำดับจากล่าสุดไปเก่าสุด
	query = query.OrderBy("updated_at", firestore.Desc)

	// 🌟 4. ระบบ Pagination (หน้าละกี่รายการ)
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 10
	}
	offset := (page - 1) * limit
	query = query.Offset(offset).Limit(limit)

	// 🌟 5. ดึงข้อมูลจาก Firestore ตามเงื่อนไขที่เซ็ตไว้
	iter := query.Documents(ctx)
	defer iter.Stop()

	// สร้างตัวแปรพักข้อมูล และ Map เก็บ Ref ของ Rider
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

	// 6. 🌟 กวาดดึงข้อมูล Rider Profiles ทั้งหมดในครั้งเดียว (เฉพาะไรเดอร์ในหน้านี้)
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
						if userProfile.UID == "" {
							userProfile.UID = uSnap.Ref.ID
						}
						riderProfiles[uSnap.Ref.ID] = userProfile
					}
				}
			}
		}
	}

	// 7. 🌟 ประกอบร่างข้อมูลออเดอร์ เข้ากับข้อมูลไรเดอร์
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
				detailRes.RiderProfile = profile
			}
		}

		responseList = append(responseList, detailRes)
	}

	return responseList, nil
}

func (r *JobRepository) GetStoveByRiderId(ctx context.Context, userID string) ([]models.StoveDetailResponse, error) {
	responseList := make([]models.StoveDetailResponse, 0)

	// 🌟 ใช้ "in" และส่งค่าเป็น Slice []string เพื่อบอกว่าเอาสถานะไหนบ้าง
	iter := r.client.Collection("orders").
		Where("status", "in", []string{"pending", "stoveFalse"}).
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

func (r *JobRepository) PostStoveStatusFalse(ctx context.Context, req models.UpdateStoveStatusRequest, riderID string) error {
	orderRef := r.client.Collection("orders").Doc(req.OrderID)

	if req.IsComplete {
		// 🌟 กรณีเก็บสำเร็จ (เก็บครบ)
		_, err := orderRef.Update(ctx, []firestore.Update{
			{Path: "status", Value: "success"},
			{Path: "updated_at", Value: time.Now()},
		})
		return err
	} else {
		// 🌟 กรณีเก็บไม่ครบ (Stove False)
		// ใช้ Batch เพื่อทำการอัปเดต Order และสร้าง Note ไปพร้อมๆ กัน
		batch := r.client.Batch()

		// 1. อัปเดตสถานะ Order เป็น stoveFalse
		batch.Update(orderRef, []firestore.Update{
			{Path: "status", Value: "stoveFalse"},
			{Path: "updated_at", Value: time.Now()},
		})

		// 2. สร้างเอกสารใหม่ในตาราง stoves_note
		noteRef := r.client.Collection("stoves_note").NewDoc()
		noteData := models.StoveNote{
			OrderID:         req.OrderID,
			RiderID:         riderID,
			CollectedStoves: req.CollectedStoves,
			CollectedPans:   req.CollectedPans,
			Reason:          req.Reason,
			CreatedAt:       time.Now(),
		}
		batch.Set(noteRef, noteData)

		// สั่งรันคำสั่งทั้งหมดใน Batch
		_, err := batch.Commit(ctx)
		return err
	}
}
