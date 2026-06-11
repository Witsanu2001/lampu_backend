package repository

import (
	"context"
	"orders/models"
	"time"

	"cloud.google.com/go/firestore"
)

type OrderRepository struct {
	Client *firestore.Client
}

// ฟังก์ชันนี้แหละที่ main.go มองหาอยู่!
func NewOrderRepository(client *firestore.Client) *OrderRepository {
	return &OrderRepository{Client: client}
}

func (r *OrderRepository) CreateOrder(ctx context.Context, order *models.Order) error {
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	_, err := r.Client.Collection("orders").Doc(order.ID).Set(ctx, order)
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
