package repository

import (
	"context"
	"fmt"
	"orders/models"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"firebase.google.com/go/v4/db"
	"google.golang.org/api/iterator"
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
	snap, err := r.Client.Collection("orders").Doc(orderID).Get(ctx)
	if err != nil {
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

	if finalStatus == "shipping" || finalStatus == "delivered" {
		loc := time.FixedZone("UTC+7", 7*3600)
		todayStr := time.Now().In(loc).Format("2006-01-02")

		iterJob := r.Client.Collection("jobs").
			Where("rider_id", "==", userID).
			Where("status_job", "==", "active").
			Limit(1).
			Documents(ctx)

		docJob, err := iterJob.Next()
		var remainingCount int = 0
		var foundJob bool = false

		if err == nil {
			var jobData map[string]interface{}
			docJob.DataTo(&jobData)

			// ดึงค่า Array ออกมาตรวจสอบ
			if activeJobsRaw, ok := jobData["active_jobs"].([]interface{}); ok {
				var updatedActiveJobs []interface{}
				hasChanges := false

				// วนลูปเช็กแต่ละออเดอร์ใน Array active_jobs
				for _, itemRaw := range activeJobsRaw {
					item, ok := itemRaw.(map[string]interface{})
					if !ok {
						updatedActiveJobs = append(updatedActiveJobs, itemRaw)
						continue
					}

					// 💡 1. ถ้าเจอ order_id ที่เรากำลังอัปเดต ให้เปลี่ยนสถานะ
					if item["order_id"] == orderID {
						if finalStatus == "shipping" {
							item["status"] = "start"
						} else if finalStatus == "delivered" {
							item["status"] = "success"
						}
						hasChanges = true
					}

					// 💡 2. นับจำนวนงานที่ยัง "ไม่สำเร็จ" (เช่น ready หรือ start)
					if st, ok := item["status"].(string); ok {
						if st == "ready" || st == "start" {
							remainingCount++
						}
					}

					updatedActiveJobs = append(updatedActiveJobs, item)
				}

				// ถ้ามีการเปลี่ยนสถานะใน Array สำเร็จ ให้ Update กลับลงไปที่ Firestore
				if hasChanges {
					foundJob = true
					_, err = docJob.Ref.Update(ctx, []firestore.Update{
						{Path: "active_jobs", Value: updatedActiveJobs},
						{Path: "updated_at", Value: time.Now()},
					})
					if err != nil {
						return "", err
					}
				}
			}
		}

		if foundJob {
			iterEvent := r.Client.Collection("jobs_event").
				Where("rider_id", "==", userID).
				Where("date", "==", todayStr).
				Where("status", "in", []string{"start", "pending"}).
				Documents(ctx)
			defer iterEvent.Stop()

			for {
				docEvent, err := iterEvent.Next()
				if err == iterator.Done {
					break
				}
				if err != nil {
					return "", err
				}

				var eventStatus string
				if finalStatus == "shipping" {
					eventStatus = "start" // เริ่มไปส่ง
				} else if finalStatus == "delivered" {
					if remainingCount > 0 {
						eventStatus = "pending" // ยังมีคิวอื่นรออยู่ใน Array active_jobs
					} else {
						eventStatus = "" // ส่งครบทุกคิวใน Array แล้ว
					}
				}

				// อัปเดตสถานะลง jobs_event
				_, err = docEvent.Ref.Update(ctx, []firestore.Update{
					{Path: "status", Value: eventStatus},
					{Path: "updated_at", Value: time.Now()},
				})
				if err != nil {
					return "", err
				}
			}
		}
	}

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

func (r *OrderRepository) BulkAssignJobs(ctx context.Context, riderID string, jobs []models.AssignJobPayload) error {
	if len(jobs) == 0 {
		return nil
	}

	batch := r.Client.Batch()
	rtdbUpdates := make(map[string]interface{})
	now := time.Now()
	nowUnix := now.Unix()

	var taskItems []interface{}

	var totalOrderSets int
	var totalDeliveryFee float64

	for _, job := range jobs {
		// --- อัปเดตตาราง orders ---
		orderRef := r.Client.Collection("orders").Doc(job.OrderID)
		batch.Update(orderRef, []firestore.Update{
			{Path: "status", Value: "ready"},
			{Path: "rider_id", Value: riderID},
			{Path: "queue_number", Value: job.QueueNumber},
			{Path: "updated_at", Value: now},
		})

		taskObj := map[string]interface{}{
			"order_id":     job.OrderID,
			"status":       "ready",
			"queue_number": job.QueueNumber,
			"assigned_at":  now,
		}
		taskItems = append(taskItems, taskObj)

		rtdbUpdates["live_orders/"+job.OrderID+"/status"] = "ready"
		rtdbUpdates["live_orders/"+job.OrderID+"/updated_at"] = nowUnix
		rtdbUpdates["live_jobs/"+riderID+"/"+job.OrderID] = map[string]interface{}{
			"status":       "ready",
			"queue_number": job.QueueNumber,
			"updated_at":   nowUnix,
		}

		totalOrderSets += job.OrderSetQty
		totalDeliveryFee += job.DeliveryFee
	}

	jobRef := r.Client.Collection("jobs").NewDoc()
	jobID := jobRef.ID
	batch.Set(jobRef, map[string]interface{}{
		"job_id":      jobID,
		"rider_id":    riderID,
		"created_at":  now,
		"updated_at":  now,
		"active_jobs": taskItems,
		"status_job":  "active",
	})

	userRef := r.Client.Collection("users").Doc(riderID)
	batch.Update(userRef, []firestore.Update{
		{Path: "updated_at", Value: now},
	})

	dateStr := now.Format("2006-01-02")
	jobsEventID := fmt.Sprintf("%s_%s", riderID, dateStr)
	jobsEventRef := r.Client.Collection("jobs_event").Doc(jobsEventID)

	batch.Set(jobsEventRef, map[string]interface{}{
		"rider_id":           riderID,
		"date":               dateStr,
		"status":             "pending",
		"total_order_sets":   firestore.Increment(totalOrderSets),
		"total_delivery_fee": firestore.Increment(totalDeliveryFee),
		"updated_at":         now,
		"job_ids":            firestore.ArrayUnion(jobID),
	}, firestore.MergeAll)

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
