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

func NewOrderRepository(client *firestore.Client) *OrderRepository {
	return &OrderRepository{Client: client}
}

// สร้างออเดอร์ใหม่
func (r *OrderRepository) CreateOrder(ctx context.Context, order *models.Order) error {
	order.CreatedAt = time.Now()
	order.Status = "pending"

	ref := r.Client.Collection("orders").NewDoc()
	order.ID = ref.ID

	_, err := ref.Set(ctx, order)
	return err
}

// ดึงออเดอร์ทั้งหมดของ User
func (r *OrderRepository) GetOrdersByUserID(ctx context.Context, userID string) ([]models.Order, error) {
	var orders []models.Order
	iter := r.Client.Collection("orders").Where("user_id", "==", userID).Documents(ctx)

	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}
		var order models.Order
		doc.DataTo(&order)
		order.ID = doc.Ref.ID
		orders = append(orders, order)
	}
	return orders, nil
}
