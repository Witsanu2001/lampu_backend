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
	// แก้จาก "created_at" เป็น "CreatedAt"
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
		OrderBy("CreatedAt", firestore.Desc).
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

	// ใช้ .Doc().Update() เพื่อเจาะจงอัปเดตเฉพาะฟิลด์ที่ต้องการ
	_, err := r.Client.Collection("orders").Doc(orderID).Update(ctx, []firestore.Update{
		{
			Path:  "status",
			Value: status,
		},
		{
			Path:  "updated_at", // 🚨 ตรวจสอบว่าใน Database ของคุณใช้ key นี้ตัวเล็กหรือตัวใหญ่ (เช่น UpdatedAt)
			Value: time.Now(),
		},
		// (ทางเลือก) ถ้าต้องการเก็บประวัติว่าใครเป็นคนกดยืนยันออเดอร์นี้
		{
			Path:  "updated_by",
			Value: userID,
		},
	})

	return err
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
