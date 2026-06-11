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
	snapshots, err := r.Client.Collection("orders").OrderBy("created_at", firestore.Desc).Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}

	var orders []*models.Order
	for _, snap := range snapshots {
		var order models.Order
		if err := snap.DataTo(&order); err != nil {
			return nil, err
		}
		orders = append(orders, &order)
	}

	return orders, nil
}
