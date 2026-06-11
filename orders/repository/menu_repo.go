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

func (r *MenuRepository) GetAllMenus(ctx context.Context) ([]models.Menu, error) {
	var menus []models.Menu
	iter := r.Client.Collection("menus").Documents(ctx)

	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}
		var menu models.Menu
		doc.DataTo(&menu)
		menu.ID = doc.Ref.ID
		menus = append(menus, menu)
	}
	return menus, nil
}

func (r *MenuRepository) GetMenusByType(ctx context.Context, typeMenu string) ([]models.Menu, error) {
	var menus []models.Menu

	// สั่งค้นหาเฉพาะ Document ที่ฟิลด์ type_menu ตรงกับที่ส่งมา
	iter := r.Client.Collection("menus").Where("type_menu", "==", typeMenu).Documents(ctx)

	for {
		doc, err := iter.Next()
		if err != nil {
			break
		}
		var menu models.Menu
		doc.DataTo(&menu)
		menu.ID = doc.Ref.ID
		menus = append(menus, menu)
	}
	return menus, nil
}

func (r *MenuRepository) UpdateMenu(ctx context.Context, id string, updates []firestore.Update) error {
	_, err := r.Client.Collection("menus").Doc(id).Update(ctx, updates)
	return err
}

func (r *MenuRepository) DeleteMenu(ctx context.Context, id string) error {
	_, err := r.Client.Collection("menus").Doc(id).Delete(ctx)
	return err
}
