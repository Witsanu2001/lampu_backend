package repository

import (
	"context"
	"orders/models"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"firebase.google.com/go/v4/db"
)

type OrderRepository struct {
	Client     *firestore.Client
	RTDBClient *db.Client
}

// ฟังก์ชันนี้แหละที่ main.go มองหาอยู่!
func NewOrderRepository(client *firestore.Client, rtdbClient *db.Client) *OrderRepository {
	return &OrderRepository{
		Client:     client,
		RTDBClient: rtdbClient,
	}
}

func (r *OrderRepository) CreateOrder(ctx context.Context, order *models.Order) error {
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	// 1. บันทึกข้อมูลเต็มลง Firestore (ของเดิม)
	_, err := r.Client.Collection("orders").Doc(order.ID).Set(ctx, order)

	// 2. ✨ ถ้าบันทึกลง Firestore สำเร็จ ให้ยิง Signal ไปที่ RTDB
	if err == nil {
		// สร้าง Path: live_orders/ORD-20260612-xxxxxx
		ref := r.RTDBClient.NewRef("live_orders/" + order.ID)

		// เก็บแค่ข้อมูลสำคัญสั้นๆ เพื่อให้ UI รับรู้
		_ = ref.Set(ctx, map[string]interface{}{
			"order_id":   order.ID,
			"status":     order.Status, // เช่น "new"
			"updated_at": time.Now().Unix(),
		})
	}

	return err
}

func (r *OrderRepository) GetAllOrders(ctx context.Context) ([]*models.Order, error) {
	// 1. คำนวณหาเวลาเริ่มต้นของวันนี้ (00:00:00) และวันพรุ่งนี้
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfTomorrow := startOfDay.AddDate(0, 0, 1)

	// 2. คิวรี Firestore โดยกรองเวลา (>= เริ่มต้นวันนี้ และ < เริ่มต้นวันพรุ่งนี้)
	snapshots, err := r.Client.Collection("orders").
		Where("CreatedAt", ">=", startOfDay).
		Where("CreatedAt", "<", startOfTomorrow).
		OrderBy("CreatedAt", firestore.Desc).
		Documents(ctx).
		GetAll()

	if err != nil {
		return nil, err
	}

	// แนะนำให้ make array ว่างไว้กันเหนียวตามที่บอกไปครับ
	orders := make([]*models.Order, 0)

	for _, snap := range snapshots {
		var order models.Order
		if err := snap.DataTo(&order); err != nil {
			return nil, err
		}

		// ดึง Document ID ของ Firestore มาใส่ใน Struct ด้วย (ถ้า Field ใน Struct ชื่อ ID)
		order.ID = snap.Ref.ID

		orders = append(orders, &order)
	}

	return orders, nil
}

// 🌟 เปลี่ยน Return type เป็น []models.SuccessOrderSummary
func (r *OrderRepository) GetSuccessOrders(ctx context.Context, targetDate time.Time) ([]models.SuccessOrderSummary, error) {
	// 1. คำนวณหาเวลาเริ่มต้นของวันที่ระบุ (00:00:00) และวันถัดไป
	startOfDay := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location())
	startOfTomorrow := startOfDay.AddDate(0, 0, 1)

	// 2. คิวรี Firestore
	snapshots, err := r.Client.Collection("orders").
		Where("status", "==", "success").
		Where("CreatedAt", ">=", startOfDay).
		Where("CreatedAt", "<", startOfTomorrow).
		OrderBy("CreatedAt", firestore.Desc).
		Documents(ctx).
		GetAll()

	if err != nil {
		return nil, err
	}

	// 🌟 สร้าง Array ว่างของ Struct ตัวใหม่
	ordersSummary := make([]models.SuccessOrderSummary, 0)

	for _, snap := range snapshots {
		var order models.Order
		if err := snap.DataTo(&order); err != nil {
			return nil, err
		}

		order.ID = snap.Ref.ID

		// 🌟 ประกอบร่างข้อมูล เอาเฉพาะส่วนที่อยากส่งออกไป
		summary := models.SuccessOrderSummary{
			OrderID:    order.ID,
			Status:     order.Status,
			Recipient:  order.Shipping.Recipient,
			Address:    order.Shipping.Address,
			GrandTotal: order.Totals.GrandTotal,
			CreatedAt:  order.CreatedAt,
		}

		// นำไปใส่ใน Array
		ordersSummary = append(ordersSummary, summary)
	}

	return ordersSummary, nil
}

func (r *OrderRepository) GetOrdersByUserID(ctx context.Context, userID string) ([]*models.Order, error) {
	// กรองหาออเดอร์ที่มี user_id ตรงกับที่ส่งมา และเรียงตามเวลาที่สร้าง
	snapshots, err := r.Client.Collection("orders").
		Where("user_id", "==", userID).
		OrderBy("CreatedAt", firestore.Desc).
		Documents(ctx).
		GetAll()

	if err != nil {
		return nil, err
	}

	orders := make([]*models.Order, 0)
	riderRefsMap := make(map[string]*firestore.DocumentRef)

	for _, snap := range snapshots {
		var order models.Order
		if err := snap.DataTo(&order); err != nil {
			return nil, err
		}

		order.ID = snap.Ref.ID

		// 🎯 ✨ เพิ่มเงื่อนไขแปลงสถานะตรงนี้:
		// ถ้าสถานะเป็น pending หรือ success ให้เปลี่ยนเป็น delivered ก่อนส่งออกไป
		if order.Status == "pending" || order.Status == "success" {
			order.Status = "delivered"
		}

		orders = append(orders, &order)

		// ถ้ารายการไหนมีการมอบหมาย RiderID ไว้แล้ว ให้เก็บลง Map เพื่อเตรียมไปดึงข้อมูล
		if order.RiderID != "" {
			riderRefsMap[order.RiderID] = r.Client.Collection("users").Doc(order.RiderID)
		}
	}

	// เรียกใช้ฟังก์ชันตัวช่วยเพื่อประกอบร่างชื่อไรเดอร์
	r.attachRiderNames(ctx, orders, riderRefsMap)

	return orders, nil
}

func (r *OrderRepository) GetOrderByID(ctx context.Context, orderID string) (*models.Order, error) {
	// ใช้ .Doc().Get() เพื่อดึงข้อมูลเอกสารแบบเจาะจง ID
	snap, err := r.Client.Collection("orders").Doc(orderID).Get(ctx)
	if err != nil {
		// หากหาไม่เจอ หรือเกิดข้อผิดพลาด จะส่ง error กลับไป
		return nil, err
	}

	var order models.Order
	// นำข้อมูลที่ได้มาแปลงใส่ใน Struct
	if err := snap.DataTo(&order); err != nil {
		return nil, err
	}

	return &order, nil
}

func (r *OrderRepository) UpdateOrderStatus(ctx context.Context, orderID string, status string, userID string) (string, error) {

	// 1. ดึงข้อมูลออเดอร์ปัจจุบันมาเช็กก่อน
	docSnap, err := r.Client.Collection("orders").Doc(orderID).Get(ctx)
	if err != nil {
		return "", err
	}

	var order models.Order
	if err := docSnap.DataTo(&order); err != nil {
		return "", err
	}

	// 2. เช็กเงื่อนไขการเปลี่ยนสถานะอัตโนมัติ
	finalStatus := status

	// เงื่อนไขที่ 1: ถ้าเป็น delivered และจ่ายด้วย promptpay ให้เด้งไป pending อัตโนมัติ
	if finalStatus == "delivered" {
		// ใช้ strings.ToLower เพื่อป้องกันปัญหาตัวพิมพ์เล็ก-ใหญ่ (เช่น "PromptPay")
		if strings.ToLower(order.Payment.Method) == "promptpay" {
			finalStatus = "pending"
		}
	}

	// เงื่อนไขที่ 2: ถ้ากำลังจะกลายเป็น pending แต่ "ไม่ใช้เตา" (needEquipment == false) ให้ข้ามไป success เลย
	if finalStatus == "pending" {
		if !order.Equipment.NeedEquipment {
			finalStatus = "success"
		}
	}

	// 3. สร้างตัวแปรเก็บสิ่งที่จะอัปเดตลงตาราง Orders (ใช้ finalStatus)
	var updates []firestore.Update
	updates = append(updates, firestore.Update{Path: "status", Value: finalStatus})
	updates = append(updates, firestore.Update{Path: "updated_at", Value: time.Now()})

	// แยกเก็บคนละฟิลด์
	if finalStatus == "ready" {
		updates = append(updates, firestore.Update{Path: "rider_id", Value: userID})
	} else {
		updates = append(updates, firestore.Update{Path: "updated_by", Value: userID})
	}

	// 4. อัปเดตข้อมูลตาราง orders ลง Firestore
	_, err = r.Client.Collection("orders").Doc(orderID).Update(ctx, updates)
	if err != nil {
		return "", err
	}

	// 5. เงื่อนไขเพิ่มเติม: เมื่อสถานะเป็น "ready" ให้สร้างงานส่งตาราง jobs
	if finalStatus == "ready" {
		jobRef := r.Client.Collection("jobs").Doc(orderID)

		jobData := map[string]interface{}{
			"id":         orderID,
			"order_id":   orderID,
			"user_id":    userID,
			"status":     finalStatus,
			"created_at": time.Now(),
			"updated_at": time.Now(),
		}

		_, err = jobRef.Set(ctx, jobData)
		if err != nil {
			return "", err
		}

		// ซิงค์ข้อมูลลง Realtime Database (RTDB) ให้แอปไรเดอร์เด้งรับงาน
		refLiveJob := r.RTDBClient.NewRef("live_jobs/" + orderID)
		_ = refLiveJob.Set(ctx, map[string]interface{}{
			"order_id":   orderID,
			"user_id":    userID,
			"status":     finalStatus,
			"updated_at": time.Now().Unix(),
		})
	}

	// 6. อัปเดตสถานะออเดอร์ลง Realtime Database (RTDB) สำหรับลูกค้า
	refLiveOrder := r.RTDBClient.NewRef("live_orders/" + orderID)
	_ = refLiveOrder.Update(ctx, map[string]interface{}{
		"status":     finalStatus,
		"updated_at": time.Now().Unix(),
	})

	// ⚡ คืนค่า finalStatus กลับไปให้ Handler
	return finalStatus, nil
}

func (r *OrderRepository) GetTodayOrderCount(ctx context.Context, startOfDay time.Time, endOfDay time.Time) (int64, error) {
	// 🚨 ตรง Where ให้ใช้ชื่อ Field ให้ตรงกับใน Database (จากโค้ดเดิมเห็นใน Repo คุณใช้ "CreatedAt")
	query := r.Client.Collection("orders").
		Where("CreatedAt", ">=", startOfDay).
		Where("CreatedAt", "<", endOfDay)

	aggQuery := query.NewAggregationQuery().WithCount("all")
	result, err := aggQuery.Get(ctx)
	if err != nil {
		return 0, err
	}

	if val, ok := result["all"].(*firestorepb.Value); ok {
		return val.GetIntegerValue(), nil
	}

	return 0, nil
}

// ✨ ฟังก์ชันสำหรับไปดึงข้อมูลจากตาราง locations มาประกอบในออเดอร์
func (r *OrderRepository) GetLocationByID(ctx context.Context, locationID string) (*models.UserLocation, error) {
	snap, err := r.Client.Collection("locations").Doc(locationID).Get(ctx)
	if err != nil {
		return nil, err
	}

	var loc models.UserLocation
	if err := snap.DataTo(&loc); err != nil {
		return nil, err
	}

	return &loc, nil
}

func (r *OrderRepository) BulkAssignJobs(ctx context.Context, jobs []models.AssignJobPayload) error {
	// ถ้าไม่มีงานส่งมา ให้ข้ามไปเลย
	if len(jobs) == 0 {
		return nil
	}

	batch := r.Client.Batch()
	rtdbUpdates := make(map[string]interface{})
	now := time.Now()
	nowUnix := now.Unix()

	// 🌟 ดึง RiderID ออกมา (เพราะการมอบหมายแบบ Bulk นี้คือการโยนให้ไรเดอร์ 1 คนพร้อมกัน)
	riderID := jobs[0].RiderID

	// 🌟 1. สร้างตัวแปร Array เอาไว้เก็บ Object งานทั้งหมด
	var taskItems []interface{}

	for _, job := range jobs {
		// --- อัปเดตตาราง orders ---
		orderRef := r.Client.Collection("orders").Doc(job.OrderID)
		batch.Update(orderRef, []firestore.Update{
			{Path: "status", Value: "ready"},
			{Path: "rider_id", Value: riderID}, // อัปเดต ID ไรเดอร์ผู้รับผิดชอบงาน
			{Path: "queue_number", Value: job.QueueNumber},
			{Path: "updated_at", Value: now},
		})

		// 🌟 2. ประกอบร่าง Object สำหรับงาน 1 ชิ้น
		taskObj := map[string]interface{}{
			"order_id":     job.OrderID,
			"status":       "ready",
			"queue_number": job.QueueNumber,
			"assigned_at":  now,
		}
		// จับยัดเข้า Array
		taskItems = append(taskItems, taskObj)

		// --- เตรียมข้อมูล RTDB ฝั่งลูกค้า/ร้านค้า (อัปเดตออเดอร์) ---
		rtdbUpdates["live_orders/"+job.OrderID+"/status"] = "ready"
		rtdbUpdates["live_orders/"+job.OrderID+"/updated_at"] = nowUnix

		// --- เตรียมข้อมูล RTDB ฝั่งแอปไรเดอร์ (จัดกลุ่มตาม rider_id) ---
		rtdbUpdates["live_jobs/"+riderID+"/"+job.OrderID] = map[string]interface{}{
			"status":       "ready",
			"queue_number": job.QueueNumber,
			"updated_at":   nowUnix,
		}
	}

	// 🌟 3. บันทึก Array of Objects ลงตาราง "jobs"
	// ✨ แก้ไข: ให้สร้าง Document ใหม่เลยในแต่ละรอบ (แทนที่จะใช้ riderID เป็นชื่อ Doc)
	jobRef := r.Client.Collection("jobs").NewDoc() // ให้ Firestore สุ่ม ID ใหม่ให้ทุกรอบการจ่ายงาน
	batch.Set(jobRef, map[string]interface{}{
		"job_id":      jobRef.ID, // เก็บ ID ของรอบจ่ายงานนี้ไว้ด้วยเผื่อใช้งาน
		"rider_id":    riderID,
		"created_at":  now,
		"updated_at":  now,
		"active_jobs": taskItems, // ใส่ข้อมูล Array เข้าไปตรงๆ ได้เลย ไม่ต้องใช้ ArrayUnion แล้ว
		"status":      "active",  // กำหนด status ของ job batch นี้
	})

	// 🌟 4. เพิ่มการอัปเดตสถานะของไรเดอร์ในตาราง "users" ให้เป็น "pending"
	userRef := r.Client.Collection("users").Doc(riderID)
	batch.Update(userRef, []firestore.Update{
		{Path: "status", Value: "pending"},
		{Path: "updated_at", Value: now},
	})

	// สั่ง Commit ข้อมูลทั้งหมดใน Firestore
	_, err := batch.Commit(ctx)
	if err != nil {
		return err
	}

	// สั่งอัปเดตข้อมูล Realtime Database ทั้งหมด
	err = r.RTDBClient.NewRef("/").Update(ctx, rtdbUpdates)
	if err != nil {
		return err
	}

	return nil
}

func (r *OrderRepository) GetNewOrders(ctx context.Context, userID string, page int, limit int) ([]*models.Order, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfTomorrow := startOfDay.AddDate(0, 0, 1)
	offset := (page - 1) * limit

	query := r.Client.Collection("orders").
		Where("status", "in", []string{"new", "preparing"}).
		Where("CreatedAt", ">=", startOfDay).
		Where("CreatedAt", "<", startOfTomorrow).
		OrderBy("CreatedAt", firestore.Desc).
		Offset(offset).
		Limit(limit)

	snapshots, err := query.Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	orders := make([]*models.Order, 0)
	riderRefsMap := make(map[string]*firestore.DocumentRef) // 🌟 Map เก็บ Ref ของ Rider

	for _, snap := range snapshots {
		var order models.Order
		if err := snap.DataTo(&order); err != nil {
			return nil, err
		}
		order.ID = snap.Ref.ID
		orders = append(orders, &order)

		// 🌟 ถ้ารายการไหนมี RiderID ให้เก็บลง Map เพื่อเตรียมไปดึงข้อมูล
		if order.RiderID != "" {
			riderRefsMap[order.RiderID] = r.Client.Collection("users").Doc(order.RiderID)
		}
	}

	// 🌟 ดึงชื่อ Rider (ถ้ามี)
	r.attachRiderNames(ctx, orders, riderRefsMap)

	return orders, nil
}

func (r *OrderRepository) GetDeliveryOrders(ctx context.Context, userID string, page int, limit int) ([]*models.Order, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfTomorrow := startOfDay.AddDate(0, 0, 1)
	offset := (page - 1) * limit

	query := r.Client.Collection("orders").
		Where("status", "in", []string{"ready", "shipping", "delivered"}).
		Where("CreatedAt", ">=", startOfDay).
		Where("CreatedAt", "<", startOfTomorrow).
		OrderBy("CreatedAt", firestore.Desc).
		Offset(offset).
		Limit(limit)

	snapshots, err := query.Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	orders := make([]*models.Order, 0)
	riderRefsMap := make(map[string]*firestore.DocumentRef) // 🌟 Map เก็บ Ref ของ Rider

	for _, snap := range snapshots {
		var order models.Order
		if err := snap.DataTo(&order); err != nil {
			return nil, err
		}
		order.ID = snap.Ref.ID
		orders = append(orders, &order)

		// 🌟 ถ้ารายการไหนมี RiderID ให้เก็บลง Map เพื่อเตรียมไปดึงข้อมูล
		if order.RiderID != "" {
			riderRefsMap[order.RiderID] = r.Client.Collection("users").Doc(order.RiderID)
		}
	}

	// 🌟 ดึงชื่อ Rider (ถ้ามี)
	r.attachRiderNames(ctx, orders, riderRefsMap)

	return orders, nil
}

// 🌟 ฟังก์ชันดึงข้อมูล Rider Profile ทั้งก้อนมาประกอบร่างเข้ากับ Orders
func (r *OrderRepository) attachRiderNames(ctx context.Context, orders []*models.Order, riderRefsMap map[string]*firestore.DocumentRef) {
	if len(riderRefsMap) == 0 {
		return
	}

	var refs []*firestore.DocumentRef
	for _, ref := range riderRefsMap {
		refs = append(refs, ref)
	}

	// ยิงไปดึงข้อมูลไรเดอร์ทั้งหมดในครั้งเดียว
	userSnaps, err := r.Client.GetAll(ctx, refs)
	if err != nil {
		return // หากดึงไม่สำเร็จ ก็ปล่อยผ่านไป ไม่ต้องทำให้ API พัง
	}

	// 🎯 1. เปลี่ยน Map ให้เก็บ UserProfile ทั้งก้อน แทนที่จะเก็บแค่ string (ชื่อ)
	riderProfiles := make(map[string]models.UserProfile)

	for _, uSnap := range userSnaps {
		if uSnap.Exists() {
			var userProfile models.UserProfile

			// 🎯 2. ใช้ DataTo แปลงข้อมูลจาก Firestore เข้า Struct UserProfile ทั้งก้อน
			if err := uSnap.DataTo(&userProfile); err == nil {
				// ยัด UID ใส่ไปด้วยเผื่อใน Document ไม่มีฟิลด์ UID
				if userProfile.UID == "" {
					userProfile.UID = uSnap.Ref.ID
				}

				// เก็บข้อมูลทั้งก้อนลงใน Map
				riderProfiles[uSnap.Ref.ID] = userProfile
			}
		}
	}

	// 🎯 3. นำ Profile ทั้งก้อนกลับไปหยอดใส่ใน Object ของ Order
	for _, order := range orders {
		if order.RiderID != "" {
			if profile, exists := riderProfiles[order.RiderID]; exists {
				order.RiderName = profile // ⚡ ยัด UserProfile เข้าไปทั้งก้อน
			}
		}
	}
}
