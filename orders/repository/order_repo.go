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
	snapshots, err := r.Client.Collection("orders").OrderBy("CreatedAt", firestore.Desc).Documents(ctx).GetAll()
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

		// ถ้าคุณต้องการดึง Document ID ของ Firestore มาใส่ใน Struct ด้วย ให้เพิ่มบรรทัดนี้ครับ:
		// order.ID = snap.Ref.ID

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

func (r *OrderRepository) UpdateOrderStatus(ctx context.Context, orderID string, status string, userID string) error {

	// 1. อัปเดตสถานะลงในตาราง orders ของ Firestore (ของเดิม)
	_, err := r.Client.Collection("orders").Doc(orderID).Update(ctx, []firestore.Update{
		{
			Path:  "status",
			Value: status,
		},
		{
			Path:  "updated_at",
			Value: time.Now(),
		},
		{
			Path:  "updated_by",
			Value: userID,
		},
	})
	if err != nil {
		return err
	}

	// 2. ✨ เงื่อนไขเพิ่มเติม: เมื่อสถานะเป็น "ready" ให้สร้างเอกสารใหม่ในตาราง jobs
	if status == "ready" {
		// สร้างเอกสารใหม่ใน collection "jobs"
		// (แนะนำให้ใช้ orderID เป็น Document ID ของตาราง jobs ไปเลย จะได้ค้นหาหรืออ้างอิงคู่กันได้ง่ายครับ)
		jobRef := r.Client.Collection("jobs").Doc(orderID)

		jobData := map[string]interface{}{
			"id":        orderID,
			"order_id":  orderID,
			"user_id":   userID, // รับค่า user_id ของไรเดอร์ที่ส่งมาจากหน้าบ้าน
			"status":    status, // สถานะ "ready"
			"CreatedAt": time.Now(),
			"UpdatedAt": time.Now(),
		}

		_, err = jobRef.Set(ctx, jobData)
		if err != nil {
			return err
		}

		// ⚡ ซิงค์ข้อมูลตาราง jobs ลง Realtime Database (RTDB) เพื่อให้แอปฝั่งไรเดอร์เห็นงานเด้งขึ้นมาทันที
		refLiveJob := r.RTDBClient.NewRef("live_jobs/" + orderID)
		_ = refLiveJob.Set(ctx, map[string]interface{}{
			"order_id":  orderID,
			"user_id":   userID,
			"status":    status,
			"UpdatedAt": time.Now().Unix(),
		})
	}

	// 3. อัปเดตสถานะออเดอร์ลง Realtime Database (RTDB) เพื่อให้หน้าบ้านฝั่งลูกค้าและร้านค้าอัปเดต
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
