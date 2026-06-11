package repository

import (
	"context"
	"orders/models"
	"time"

	"cloud.google.com/go/firestore"
)

type MenuRepository struct {
	Client *firestore.Client
}

// ฟังก์ชันนี้แหละที่ main.go มองหาอยู่!
func NewMenuRepository(client *firestore.Client) *MenuRepository {
	return &MenuRepository{Client: client}
}

func (r *MenuRepository) CreateMenu(ctx context.Context, menu *models.Menu) error {
	menu.CreatedAt = time.Now()

	ref := r.Client.Collection("menus").NewDoc()
	menu.ID = ref.ID

	_, err := ref.Set(ctx, menu)
	return err
}
