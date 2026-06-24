package repository

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"orders/models"
	"sort"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/firestore/apiv1/firestorepb"
	"cloud.google.com/go/storage"
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

func (r *OrderRepository) GetAllOrdersByID(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	snapshots, err := r.Client.Collection("orders").
		Where("user_id", "==", userID).
		OrderBy("CreatedAt", firestore.Desc).
		Documents(ctx).
		GetAll()

	if err != nil {
		return nil, err
	}

	var response []map[string]interface{}
	for _, snap := range snapshots {
		var order models.Order
		if err := snap.DataTo(&order); err != nil {
			return nil, err
		}
		order.ID = snap.Ref.ID
		var mappedMainItems []map[string]interface{}
		for _, item := range order.MainItems {
			mappedMainItems = append(mappedMainItems, map[string]interface{}{
				"name":     item.Name,
				"quantity": item.Quantity,
			})
		}

		mappedOrder := map[string]interface{}{
			"id":        order.ID,
			"user_id":   order.UserID,
			"mainItems": mappedMainItems,
			"shipping": map[string]interface{}{
				"recipient": order.Shipping.Recipient,
				"phone":     order.Shipping.Phone,
				"address":   order.Shipping.Address,
			},
			"payment": map[string]interface{}{
				"method": order.Payment.Method,
			},
			"totals": map[string]interface{}{
				"grandTotal": order.Totals.GrandTotal,
			},
			"status":     order.Status,
			"created_at": order.CreatedAt,
			"updated_at": order.UpdatedAt,
		}

		if order.Status == "refuse" {
			mappedOrder["cancel_reason"] = order.CancelReason
		}

		response = append(response, mappedOrder)
	}

	return response, nil
}

func (r *OrderRepository) GetSuccessOrders(ctx context.Context, targetDate time.Time, page, limit int) ([]map[string]interface{}, error) {
	startOfDay := time.Date(targetDate.Year(), targetDate.Month(), targetDate.Day(), 0, 0, 0, 0, targetDate.Location())
	startOfTomorrow := startOfDay.AddDate(0, 0, 1)
	offset := (page - 1) * limit
	snapshots, err := r.Client.Collection("orders").
		Where("status", "in", []string{"success", "pending"}).
		Where("CreatedAt", ">=", startOfDay).
		Where("CreatedAt", "<", startOfTomorrow).
		OrderBy("CreatedAt", firestore.Desc).
		Offset(offset).
		Limit(limit).
		Documents(ctx).
		GetAll()

	if err != nil {
		return nil, err
	}

	var response []map[string]interface{}
	for _, snap := range snapshots {
		var order models.Order
		if err := snap.DataTo(&order); err != nil {
			return nil, err
		}

		order.ID = snap.Ref.ID

		var mappedMainItems []map[string]interface{}
		for _, item := range order.MainItems {
			mappedMainItems = append(mappedMainItems, map[string]interface{}{
				"name":     item.Name,
				"quantity": item.Quantity,
			})
		}

		var mappedAddOnItems []map[string]interface{}
		for _, item := range order.AddOnItems {
			mappedAddOnItems = append(mappedAddOnItems, map[string]interface{}{
				"name":     item.Name,
				"quantity": item.Quantity,
			})
		}

		summary := map[string]interface{}{
			"id":         order.ID,
			"user_id":    order.UserID,
			"mainItems":  mappedMainItems,
			"addOnItems": mappedAddOnItems,
			"shipping": map[string]interface{}{
				"recipient": order.Shipping.Recipient,
				"phone":     order.Shipping.Phone,
				"address":   order.Shipping.Address,
			},
			"payment": map[string]interface{}{
				"method":  order.Payment.Method,
				"slip":    order.Payment.HasSlip,
				"slipURL": order.SlipURL,
			},
			"totals": map[string]interface{}{
				"addOnTotal":  order.Totals.AddOnTotal,
				"shippingFee": order.Totals.ShippingFee,
				"grandTotal":  order.Totals.GrandTotal,
			},
			"rider": map[string]interface{}{
				"riderName": order.RiderName.DisplayName,
			},
			"status":     order.Status,
			"created_at": order.CreatedAt,
			"updated_at": order.UpdatedAt,
		}

		response = append(response, summary)
	}

	return response, nil
}

func (r *OrderRepository) GetOrdersPDF(ctx context.Context, startDate, endDate time.Time) ([]models.Order, error) {
	start := time.Date(startDate.Year(), startDate.Month(), startDate.Day(), 0, 0, 0, 0, startDate.Location())
	end := time.Date(endDate.Year(), endDate.Month(), endDate.Day(), 23, 59, 59, 999999999, endDate.Location())
	snapshots, err := r.Client.Collection("orders").
		Where("status", "==", "success").
		Where("CreatedAt", ">=", start).
		Where("CreatedAt", "<=", end).
		OrderBy("CreatedAt", firestore.Desc).
		Documents(ctx).
		GetAll()

	if err != nil {
		return nil, err
	}

	// 🌟 สร้าง Array ของ models.Order เปล่าๆ
	var orders []models.Order

	// วนลูปยัดข้อมูลลง Array ตรงๆ ได้เลย ไม่ต้องประกอบร่างใหม่แล้ว!
	for _, snap := range snapshots {
		var order models.Order
		if err := snap.DataTo(&order); err != nil {
			return nil, err
		}

		// ดึง ID ของ Document มาใส่ในฟิลด์ ID
		order.ID = snap.Ref.ID

		// เอาใส่ Array
		orders = append(orders, order)
	}

	return orders, nil
}

func (r *OrderRepository) GetOrdersByUserID(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	snapshots, err := r.Client.Collection("orders").
		Where("user_id", "==", userID).
		OrderBy("CreatedAt", firestore.Desc).
		Documents(ctx).
		GetAll()

	if err != nil {
		return nil, err
	}

	var response []map[string]interface{}
	for _, snap := range snapshots {
		var order models.Order
		if err := snap.DataTo(&order); err != nil {
			return nil, err
		}
		order.ID = snap.Ref.ID
		var mappedMainItems []map[string]interface{}
		for _, item := range order.MainItems {
			mappedMainItems = append(mappedMainItems, map[string]interface{}{
				"name":     item.Name,
				"quantity": item.Quantity,
			})
		}

		mappedOrder := map[string]interface{}{
			"id":        order.ID,
			"user_id":   order.UserID,
			"mainItems": mappedMainItems,
			"shipping": map[string]interface{}{
				"recipient": order.Shipping.Recipient,
				"phone":     order.Shipping.Phone,
				"address":   order.Shipping.Address,
			},
			"payment": map[string]interface{}{
				"method": order.Payment.Method,
			},
			"totals": map[string]interface{}{
				"grandTotal": order.Totals.GrandTotal,
			},
			"equipment": map[string]interface{}{
				"needEquipment": order.Equipment.NeedEquipment,
				"stoveCount":    order.Equipment.StoveCount,
				"panCount":      order.Equipment.PanCount,
			},
			"status":     order.Status,
			"created_at": order.CreatedAt,
			"updated_at": order.UpdatedAt,
		}

		if order.Status == "refuse" {
			mappedOrder["cancel_reason"] = order.CancelReason
		}

		response = append(response, mappedOrder)
	}

	return response, nil
}

func (r *OrderRepository) GetOrderByUserToday(ctx context.Context, userID string) ([]map[string]interface{}, error) {
	// 🌟 1. คำนวณหาเวลาเริ่มต้นของวันนี้ (00:00:00) และวันพรุ่งนี้ (00:00:00)
	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfTomorrow := startOfToday.Add(24 * time.Hour)

	// 🌟 2. เพิ่ม Where กรองเวลาเข้าไปใน Query
	snapshots, err := r.Client.Collection("orders").
		Where("user_id", "==", userID).
		Where("CreatedAt", ">=", startOfToday).   // ตั้งแต่เริ่มวัน
		Where("CreatedAt", "<", startOfTomorrow). // จนถึงก่อนเริ่มวันพรุ่งนี้
		OrderBy("CreatedAt", firestore.Desc).
		Documents(ctx).
		GetAll()

	if err != nil {
		return nil, err
	}

	var response []map[string]interface{}

	// ถ้าไม่มีข้อมูลเลย ให้ return array ว่างกลับไปเพื่อไม่ให้เป็น null
	if len(snapshots) == 0 {
		return make([]map[string]interface{}, 0), nil
	}

	for _, snap := range snapshots {
		var order models.Order
		if err := snap.DataTo(&order); err != nil {
			return nil, err
		}
		order.ID = snap.Ref.ID
		var mappedMainItems []map[string]interface{}
		for _, item := range order.MainItems {
			mappedMainItems = append(mappedMainItems, map[string]interface{}{
				"name":     item.Name,
				"quantity": item.Quantity,
			})
		}

		mappedOrder := map[string]interface{}{
			"id":        order.ID,
			"user_id":   order.UserID,
			"mainItems": mappedMainItems,
			"shipping": map[string]interface{}{
				"recipient": order.Shipping.Recipient,
				"phone":     order.Shipping.Phone,
				"address":   order.Shipping.Address,
			},
			"payment": map[string]interface{}{
				"method": order.Payment.Method,
			},
			"totals": map[string]interface{}{
				"grandTotal": order.Totals.GrandTotal,
			},
			"equipment": map[string]interface{}{
				"needEquipment": order.Equipment.NeedEquipment,
				"stoveCount":    order.Equipment.StoveCount,
				"panCount":      order.Equipment.PanCount,
			},
			"status":     order.Status,
			"created_at": order.CreatedAt,
			"updated_at": order.UpdatedAt,
		}

		if order.Status == "refuse" {
			mappedOrder["cancel_reason"] = order.CancelReason
		}

		response = append(response, mappedOrder)
	}

	return response, nil
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

	docSnap, err := r.Client.Collection("orders").Doc(orderID).Get(ctx)
	if err != nil {
		return "", err
	}

	var order models.Order
	if err := docSnap.DataTo(&order); err != nil {
		return "", err
	}

	// 🌟 1. ดักจับสถานะที่ส่งเข้ามาจริงๆ และแยกตัวแปรไม่ให้ปนกับตอนถูกแปลงอัตโนมัติ
	incomingStatus := strings.ToLower(strings.TrimSpace(status))
	finalStatus := incomingStatus

	isCanceled := false
	if incomingStatus == "cancel" {
		finalStatus = "preparing"
		isCanceled = true
	}

	// 🌟 เงื่อนไขเปลี่ยนสถานะอัตโนมัติ
	if finalStatus == "delivered" {
		if strings.ToLower(order.Payment.Method) == "promptpay" {
			finalStatus = "pending"
		}
	}

	if finalStatus == "pending" {
		if !order.Equipment.NeedEquipment {
			finalStatus = "success"

			// 🌟 1. อัปเดตสถานะ "" ลงใน jobs_event ทันที
			loc := time.FixedZone("UTC+7", 7*3600)
			todayStr := time.Now().In(loc).Format("2006-01-02")

			// หา Rider ID
			var riderIDForEvent string
			if raw, ok := docSnap.Data()["rider_id"].(string); ok && raw != "" {
				riderIDForEvent = raw
			} else {
				riderIDForEvent = userID
			}

			// สั่งอัปเดต Firestore
			jobsEventID := fmt.Sprintf("%s_%s", riderIDForEvent, todayStr)
			_, errEvent := r.Client.Collection("jobs_event").Doc(jobsEventID).Update(ctx, []firestore.Update{
				{Path: "status", Value: ""},
				{Path: "updated_at", Value: time.Now()},
			})
			if errEvent != nil {
				log.Printf("❌ อัปเดตเคลียร์ status ใน jobs_event ขัดข้อง: %v\n", errEvent)
			}

			// ✂️ การลบ live_orders ถูกย้ายไปดักที่บรรทัดล่างสุดแล้วครับ
		}
	}

	// 🌟 เตรียมคำสั่งอัปเดตออเดอร์
	var updates []firestore.Update
	updates = append(updates, firestore.Update{Path: "status", Value: finalStatus})
	updates = append(updates, firestore.Update{Path: "updated_at", Value: time.Now()})

	if finalStatus == "ready" {
		updates = append(updates, firestore.Update{Path: "rider_id", Value: userID})
	} else if isCanceled {
		// 🌟 ถ้าไรเดอร์ยกเลิก ให้เอา rider_id ออกจากออเดอร์
		updates = append(updates, firestore.Update{Path: "rider_id", Value: firestore.Delete})
		updates = append(updates, firestore.Update{Path: "updated_by", Value: userID})
	} else {
		updates = append(updates, firestore.Update{Path: "updated_by", Value: userID})
	}

	if incomingStatus == "delivered" {
		log.Println("🎯 [UpdateOrderStatus] ได้รับคำสั่ง delivered แล้ว! เตรียมสั่งลบรูป...")

		// สั่งลบฟิลด์ใน Firestore
		updates = append(updates, firestore.Update{Path: "home_image_url", Value: firestore.Delete})

		// งัด URL ออกมา
		imageUrl := order.HomeImageURL
		if imageUrl == "" {
			rawMap := docSnap.Data()
			if val, ok := rawMap["home_image_url"].(string); ok {
				imageUrl = val
			}
		}

		if imageUrl != "" {
			log.Println("📸 [UpdateOrderStatus] กำลังส่ง URL ไปลบที่ Storage:", imageUrl)
			// ยิงคำสั่งให้ฟังก์ชัน deleteFileFromStorage ทำงานเบื้องหลัง
			go r.deleteFileFromStorage(context.Background(), imageUrl)
		} else {
			log.Println("⚠️ [UpdateOrderStatus] ไม่พบลิงก์รูปภาพ (ค่าว่าง)")
		}
	}

	// อัปเดตลง Firestore
	_, err = r.Client.Collection("orders").Doc(orderID).Update(ctx, updates)
	if err != nil {
		return "", err
	}

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

		refLiveJob := r.RTDBClient.NewRef("live_jobs/" + orderID)
		_ = refLiveJob.Set(ctx, map[string]interface{}{
			"order_id":   orderID,
			"user_id":    userID,
			"status":     finalStatus,
			"updated_at": time.Now().Unix(),
		})
	}

	// 🌟 ป้องกันไม่ให้ข้อมูลฟื้นคืนชีพใน RTDB ถ้ามันกำลังจะถูกลบ
	if finalStatus != "pending" && finalStatus != "success" {
		refLiveOrder := r.RTDBClient.NewRef("live_orders/" + orderID)
		_ = refLiveOrder.Update(ctx, map[string]interface{}{
			"status":     finalStatus,
			"updated_at": time.Now().Unix(),
		})
	}

	// 🌟 จัดการดึงงานออกจาก jobs เมื่อ shipping, delivered หรือ cancel
	if incomingStatus == "shipping" || incomingStatus == "delivered" || isCanceled {
		loc := time.FixedZone("UTC+7", 7*3600)
		todayStr := time.Now().In(loc).Format("2006-01-02")

		var riderID string
		if raw, ok := docSnap.Data()["rider_id"].(string); ok && raw != "" {
			riderID = raw
		} else {
			riderID = userID
		}

		// ค้นหาคิวงานทั้งหมดของไรเดอร์คนนี้
		iterJob := r.Client.Collection("jobs").
			Where("rider_id", "==", riderID).
			Where("status_job", "==", "active").
			Documents(ctx)

		var totalRemainingCount int = 0
		var foundJob bool = false
		var targetJobDocID string

		for {
			docJob, err := iterJob.Next()
			if err == iterator.Done {
				break
			}
			if err != nil {
				log.Println("❌ Error iterating jobs:", err)
				break
			}

			var jobData map[string]interface{}
			docJob.DataTo(&jobData)

			if activeJobsRaw, ok := jobData["active_jobs"].([]interface{}); ok {
				var updatedActiveJobs []interface{}
				hasChanges := false
				localRemainingCount := 0

				for _, itemRaw := range activeJobsRaw {
					item, ok := itemRaw.(map[string]interface{})
					if !ok {
						updatedActiveJobs = append(updatedActiveJobs, itemRaw)
						continue
					}

					if item["order_id"] == orderID {
						hasChanges = true
						foundJob = true
						targetJobDocID = docJob.Ref.ID

						if incomingStatus == "shipping" {
							item["status"] = "start"
							updatedActiveJobs = append(updatedActiveJobs, item)
							localRemainingCount++
						} else if incomingStatus == "delivered" || isCanceled {
							continue // ลบทิ้งจากอาร์เรย์
						}
					} else {
						updatedActiveJobs = append(updatedActiveJobs, item)
						if st, ok := item["status"].(string); ok {
							if st == "ready" || st == "start" {
								localRemainingCount++
							}
						}
					}
				}

				if hasChanges {
					if localRemainingCount == 0 || len(updatedActiveJobs) == 0 {
						_, _ = docJob.Ref.Delete(ctx)
					} else {
						_, _ = docJob.Ref.Update(ctx, []firestore.Update{
							{Path: "active_jobs", Value: updatedActiveJobs},
							{Path: "updated_at", Value: time.Now()},
						})
					}
				}

				totalRemainingCount += localRemainingCount
			}
		}

		if foundJob {
			jobsEventID := fmt.Sprintf("%s_%s", riderID, todayStr)
			jobsEventRef := r.Client.Collection("jobs_event").Doc(jobsEventID)

			if incomingStatus == "shipping" {
				var totalOrderSets int
				var totalDeliveryFee float64

				for _, item := range order.MainItems {
					totalOrderSets += item.Quantity
				}

				rawMap := docSnap.Data()
				if shippingRaw, ok := rawMap["shipping"].(map[string]interface{}); ok {
					if tf, ok := shippingRaw["totalFee"].(float64); ok {
						totalDeliveryFee = tf
					} else if tf, ok := shippingRaw["totalFee"].(int64); ok {
						totalDeliveryFee = float64(tf)
					}
				} else if totalsRaw, ok := rawMap["totals"].(map[string]interface{}); ok {
					if tf, ok := totalsRaw["shippingFee"].(float64); ok {
						totalDeliveryFee = tf
					} else if tf, ok := totalsRaw["shippingFee"].(int64); ok {
						totalDeliveryFee = float64(tf)
					}
				}

				_, err = jobsEventRef.Set(ctx, map[string]interface{}{
					"rider_id":           riderID,
					"date":               todayStr,
					"status":             "start",
					"total_order_sets":   firestore.Increment(totalOrderSets),
					"total_delivery_fee": firestore.Increment(totalDeliveryFee),
					"updated_at":         time.Now(),
					"job_ids":            firestore.ArrayUnion(targetJobDocID),
				}, firestore.MergeAll)

				if err != nil {
					log.Printf("❌ อัปเดต jobs_event ขัดข้อง: %v\n", err)
				}

			} else if incomingStatus == "delivered" || isCanceled {
				var eventStatus string

				// ถ้ายังมีงานเหลือให้คงสถานะ "start" ไว้
				if totalRemainingCount > 0 {
					eventStatus = "start"
				} else {
					// ถ้าส่งครบหมดแล้ว ค่อยเคลียร์เป็นว่าง ""
					eventStatus = ""
				}

				_, err = jobsEventRef.Update(ctx, []firestore.Update{
					{Path: "status", Value: eventStatus},
					{Path: "updated_at", Value: time.Now()},
				})
				if err != nil {
					log.Printf("❌ อัปเดต jobs_event ตอน delivered/cancel ขัดข้อง: %v\n", err)
				}
			}
		}
	}

	if incomingStatus == "delivered" || isCanceled {
		var riderID string
		if raw, ok := docSnap.Data()["rider_id"].(string); ok && raw != "" {
			riderID = raw
		} else {
			riderID = userID
		}

		if riderID != "" {
			refLiveJobToDelete := r.RTDBClient.NewRef(fmt.Sprintf("live_jobs/%s/%s", riderID, orderID))
			err := refLiveJobToDelete.Delete(ctx)
			if err != nil {
				log.Printf("❌ ลบ RTDB ขัดข้อง: %v\n", err)
			} else {
				log.Printf("✅ ลบ RTDB สำเร็จ! ล้างงานของ Rider: %s ออกแล้ว\n", riderID)
			}
		} else {
			log.Println("⚠️ ไม่สามารถลบ live_jobs ได้เพราะไม่รู้ว่า Rider ID คืออะไร")
		}
	}

	// 🌟 รวบยอดลบ live_orders ทีเดียวตรงนี้เลย ทั้งเคสที่กลายเป็น pending และ success
	if finalStatus == "pending" || finalStatus == "success" {
		refLiveOrderToDelete := r.RTDBClient.NewRef(fmt.Sprintf("live_orders/%s", orderID))
		err := refLiveOrderToDelete.Delete(ctx)
		if err != nil {
			log.Printf("❌ ลบ live_orders ขัดข้อง: %v\n", err)
		} else {
			log.Printf("✅ ลบ live_orders สำเร็จ! ล้างออเดอร์ %s ออกจากระบบ RTDB แล้ว\n", orderID)
		}
	}

	return incomingStatus, nil
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

func (r *OrderRepository) deleteFileFromStorage(ctx context.Context, fileURL string) error {
	if !strings.Contains(fileURL, "firebasestorage.googleapis.com") {
		return nil
	}

	u, err := url.Parse(fileURL)
	if err != nil {
		log.Printf("❌ Parse URL ขัดข้อง: %v\n", err)
		return err
	}

	// u.Path ของ URL นี้จะได้: /v0/b/lampu-5a178.firebasestorage.app/o/home_images%2F08a61e...jpg
	segments := strings.Split(u.Path, "/o/")
	if len(segments) < 2 {
		return nil
	}

	// 🌟 ใช้ PathUnescape เพื่อแปลง %2F ให้กลับมาเป็นเครื่องหมาย /
	// ผลลัพธ์จะได้: home_images/08a61e18-2607-441d-a68f-1f54dd43ef19_20260622115721.jpg
	objectPath, err := url.PathUnescape(segments[1])
	if err != nil {
		return err
	}

	// แยกหาชื่อ Bucket
	bucketSegments := strings.Split(segments[0], "/b/")
	if len(bucketSegments) < 2 {
		return nil
	}
	bucketName := bucketSegments[1] // จะได้ lampu-5a178.firebasestorage.app

	// พิมพ์ Log ยืนยันว่า Path ถูกต้อง (มีโฟลเดอร์ home_images/ นำหน้า)
	log.Printf("🎯 กำลังสั่งลบรูปภาพใน Bucket: [%s] File: [%s]\n", bucketName, objectPath)

	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Printf("❌ สร้าง Storage Client ไม่สำเร็จ (อาจเป็นปัญหาเรื่องสิทธิ์): %v\n", err)
		return err
	}
	defer client.Close()

	// สั่งลบไฟล์
	err = client.Bucket(bucketName).Object(objectPath).Delete(ctx)
	if err != nil {
		log.Printf("❌ ลบรูปภาพไม่สำเร็จ! กรุณาเช็คสิทธิ์ (IAM) ของ Service Account ว่ามีสิทธิ์ Storage Object Admin หรือไม่ Error: %v\n", err)
		return err
	}

	log.Println("✅ ลบรูปภาพออกจาก Firebase Storage สำเร็จ!:", objectPath)
	return nil
}

func (r *OrderRepository) CancelOrder(ctx context.Context, orderID string, reason string, userID string) error {

	// 1. เตรียมข้อมูลที่จะอัปเดตลงตาราง orders (Firestore)
	var updates []firestore.Update
	updates = append(updates, firestore.Update{Path: "status", Value: "refuse"})
	updates = append(updates, firestore.Update{Path: "cancel_reason", Value: reason}) // เพิ่มฟิลด์เหตุผล
	updates = append(updates, firestore.Update{Path: "updated_at", Value: time.Now()})
	updates = append(updates, firestore.Update{Path: "updated_by", Value: userID})

	// 2. อัปเดตข้อมูลตาราง orders ลง Firestore
	_, err := r.Client.Collection("orders").Doc(orderID).Update(ctx, updates)
	if err != nil {
		return err // ถ้าไม่เจอ order หรืออัปเดตพลาด จะ return error กลับไป
	}

	// 3. อัปเดตสถานะออเดอร์ลง Realtime Database (RTDB) สำหรับแอปฝั่งลูกค้า
	// เพื่อให้หน้าแอปของลูกค้าขึ้นว่า "ร้านปฏิเสธ" ได้ทันทีโดยไม่ต้องรีเฟรชหน้าจอ
	refLiveOrder := r.RTDBClient.NewRef("live_orders/" + orderID)
	_ = refLiveOrder.Update(ctx, map[string]interface{}{
		"status":        "refuse",
		"cancel_reason": reason,
		"updated_at":    time.Now().Unix(),
	})

	return nil
}

func (r *OrderRepository) UpdateSlip(ctx context.Context, orderID string, newSlipURL string) error {
	docSnap, err := r.Client.Collection("orders").Doc(orderID).Get(ctx)
	if err != nil {
		return err
	}

	var order models.Order
	if err := docSnap.DataTo(&order); err != nil {
		return err
	}

	// 🌟 2. เตรียมข้อมูลที่จะอัปเดต
	var updates []firestore.Update
	updates = append(updates, firestore.Update{Path: "slip_url", Value: newSlipURL})
	updates = append(updates, firestore.Update{Path: "old_slip_url", Value: order.SlipURL}) // ✨ ย้ายสลิปเดิมมาเก็บฟิลด์นี้
	updates = append(updates, firestore.Update{Path: "status", Value: "edit"})
	updates = append(updates, firestore.Update{Path: "is_edited_slip", Value: true})
	updates = append(updates, firestore.Update{Path: "updated_at", Value: time.Now()})

	// 3. อัปเดตข้อมูลลง Firestore
	_, err = r.Client.Collection("orders").Doc(orderID).Update(ctx, updates)
	if err != nil {
		return err
	}

	// 4. อัปเดตสถานะลง Realtime Database (RTDB) สำหรับลูกค้า
	refLiveOrder := r.RTDBClient.NewRef("live_orders/" + orderID)
	_ = refLiveOrder.Update(ctx, map[string]interface{}{
		"status":     "edit",
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

// 🌟 เปลี่ยน Return type เป็น []map[string]interface{} เพื่อคุมฟิลด์ที่จะส่งกลับเอง
func (r *OrderRepository) GetNewOrders(ctx context.Context, userID string, page int, limit int) ([]map[string]interface{}, error) {
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	startOfTomorrow := startOfDay.AddDate(0, 0, 1)
	offset := (page - 1) * limit

	query := r.Client.Collection("orders").
		Where("status", "in", []string{"new", "preparing", "refuse", "edit"}).
		Where("CreatedAt", ">=", startOfDay).
		Where("CreatedAt", "<", startOfTomorrow).
		OrderBy("CreatedAt", firestore.Asc).
		Offset(offset).
		Limit(limit)

	snapshots, err := query.Documents(ctx).GetAll()
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
		orders = append(orders, &order)

		if order.RiderID != "" {
			riderRefsMap[order.RiderID] = r.Client.Collection("users").Doc(order.RiderID)
		}
	}

	sort.SliceStable(orders, func(i, j int) bool {
		statusI := orders[i].Status
		statusJ := orders[j].Status
		if statusI == "new" && statusJ != "new" {
			return true
		}
		if statusI != "new" && statusJ == "new" {
			return false
		}
		return false
	})

	// นำ orders ไปประมวลผลหารายชื่อ Rider ตามปกติ
	r.attachRiderNames(ctx, orders, riderRefsMap)

	// 🌟 กำหนดฟิลด์ที่จะส่งไปให้หน้าบ้านตรงนี้เลย!
	var response []map[string]interface{}

	for _, order := range orders {

		var mappedMainItems []map[string]interface{}
		for _, item := range order.MainItems {
			mappedMainItems = append(mappedMainItems, map[string]interface{}{
				"name":     item.Name,
				"quantity": item.Quantity,
			})
		}

		mappedOrder := map[string]interface{}{
			"id":        order.ID,
			"user_id":   order.UserID,
			"mainItems": mappedMainItems,
			"shipping": map[string]interface{}{
				"recipient": order.Shipping.Recipient,
				"phone":     order.Shipping.Phone,
				"address":   order.Shipping.Address,
			},
			"equipment": map[string]interface{}{
				"needEquipment": order.Equipment.NeedEquipment,
				"stoveCount":    order.Equipment.StoveCount,
				"panCount":      order.Equipment.PanCount,
				"charcoalCount": order.Equipment.CharcoalCount,
			},
			"payment": map[string]interface{}{
				"method": order.Payment.Method,
			},
			"totals": map[string]interface{}{
				"shippingFee": order.Totals.ShippingFee,
				"grandTotal":  order.Totals.GrandTotal,
			},
			"status":     order.Status,
			"created_at": order.CreatedAt,
			"updated_at": order.UpdatedAt,
		}

		//ตั้งเงื่อนไข ถ้า status เป็น refuse ค่อยเพิ่ม key "cancel_reason" เข้าไปใน Map
		if order.Status == "refuse" {
			mappedOrder["cancel_reason"] = order.CancelReason
		}

		response = append(response, mappedOrder)
	}

	return response, nil
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
		OrderBy("CreatedAt", firestore.Asc).
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
