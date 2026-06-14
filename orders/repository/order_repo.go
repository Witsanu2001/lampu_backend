package repository

import (
	"context"
	"orders/models"
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

func (r *OrderRepository) GetOrdersByUserID(ctx context.Context, userID string) ([]*models.Order, error) {
	// กรองหาออเดอร์ที่มี user_id ตรงกับที่ส่งมา และเรียงตามเวลาที่สร้าง
	snapshots, err := r.Client.Collection("orders").
		Where("user_id", "==", userID).
		OrderBy("CreatedAt", firestore.Desc). // 👈 ✨ แก้กลับเป็นตัวพิมพ์ใหญ่ให้ตรงกับ Index ที่เราสร้างไว้ครับ
		Documents(ctx).
		GetAll()

	if err != nil {
		return nil, err
	}

	orders := make([]*models.Order, 0)
	for _, snap := range snapshots {
		var order models.Order
		if err := snap.DataTo(&order); err != nil {
			return nil, err
		}
		orders = append(orders, &order)
	}

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

// 🌟 3. อัปเดตฟังก์ชัน Repository
func (r *OrderRepository) UpdateOrderStatus(ctx context.Context, orderID string, status string, userID string) error {

	// สร้างตัวแปรเก็บสิ่งที่จะอัปเดตลงตาราง Orders
	var updates []firestore.Update

	updates = append(updates, firestore.Update{Path: "status", Value: status})
	updates = append(updates, firestore.Update{Path: "updated_at", Value: time.Now()})

	// 🌟 แยกเก็บคนละฟิลด์: ถ้าสถานะ ready ให้เซ็ตเป็น rider_id ของไรเดอร์
	if status == "ready" {
		updates = append(updates, firestore.Update{Path: "rider_id", Value: userID})
	} else {
		// ถ้าสถานะอื่น ให้เซ็ตเป็น updated_by ของแอดมิน
		updates = append(updates, firestore.Update{Path: "updated_by", Value: userID})
	}

	// 1. อัปเดตข้อมูลตาราง orders ลง Firestore
	_, err := r.Client.Collection("orders").Doc(orderID).Update(ctx, updates)
	if err != nil {
		return err
	}

	// 2. เงื่อนไขเพิ่มเติม: เมื่อสถานะเป็น "ready" ให้สร้างงานส่งตาราง jobs
	if status == "ready" {
		jobRef := r.Client.Collection("jobs").Doc(orderID)

		jobData := map[string]interface{}{
			"id":         orderID,
			"order_id":   orderID,
			"user_id":    userID, // ฟิลด์ user_id ในตารางนี้จะเก็บ ID ไรเดอร์
			"status":     status,
			"created_at": time.Now(),
			"updated_at": time.Now(),
		}

		_, err = jobRef.Set(ctx, jobData)
		if err != nil {
			return err
		}

		// ⚡ ซิงค์ข้อมูลลง Realtime Database (RTDB) ให้แอปไรเดอร์เด้งรับงานแบบ Real-time
		refLiveJob := r.RTDBClient.NewRef("live_jobs/" + orderID)
		_ = refLiveJob.Set(ctx, map[string]interface{}{
			"order_id":   orderID,
			"user_id":    userID,
			"status":     status,
			"updated_at": time.Now().Unix(),
		})
	}

	// 3. อัปเดตสถานะออเดอร์ลง Realtime Database (RTDB) เพื่อให้หน้าบ้านลูกค้ารู้ว่าเปลี่ยนสถานะแล้ว
	refLiveOrder := r.RTDBClient.NewRef("live_orders/" + orderID)
	_ = refLiveOrder.Update(ctx, map[string]interface{}{
		"status":     status,
		"updated_at": time.Now().Unix(),
	})

	return nil
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
			{Path: "rider_id", Value: riderID},
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
		// โครงสร้างจะเป็น live_jobs/{rider_id}/{order_id}
		rtdbUpdates["live_jobs/"+riderID+"/"+job.OrderID] = map[string]interface{}{
			"status":       "ready",
			"queue_number": job.QueueNumber,
			"updated_at":   nowUnix,
		}
	}

	// 🌟 3. บันทึก Array of Objects ลงตาราง "jobs"
	// ใช้ RiderID เป็นชื่อ Document
	jobRef := r.Client.Collection("jobs").Doc(riderID)

	// ใช้ firestore.ArrayUnion เพื่อเอา Array งานใหม่ ไป "ต่อท้าย" งานเก่า (ถ้ามี)
	// แต่ถ้าเป็นการสร้างครั้งแรก ระบบจะสร้าง Array ให้เอง
	batch.Set(jobRef, map[string]interface{}{
		"rider_id":    riderID,
		"updated_at":  now,
		"active_jobs": firestore.ArrayUnion(taskItems...), // ยัด Array ลงไปในฟิลด์นี้
	}, firestore.MergeAll) // ใช้ MergeAll เผื่อว่ามีฟิลด์อื่นอยู่แล้วจะได้ไม่โดนลบทิ้ง

	// สั่ง Commit ข้อมูล Firestore
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
