package handlers

import (
	"context"
	"fmt"
	"io"
	"orders/models"
	"orders/repository"
	"strconv"
	"time"

	"cloud.google.com/go/firestore"
	"cloud.google.com/go/storage"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// 📌 กำหนดโครงสร้าง Response มาตรฐานที่คุณต้องการ
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type MenuHandler struct {
	Repo          *repository.MenuRepository
	StorageBucket *storage.BucketHandle
}

func NewMenuHandler(repo *repository.MenuRepository, bucket *storage.BucketHandle) *MenuHandler {
	return &MenuHandler{Repo: repo, StorageBucket: bucket}
}

func (h *MenuHandler) CreateMenu(c *fiber.Ctx) error {
	name_menu := c.FormValue("name")
	description_menu := c.FormValue("description")
	priceStr := c.FormValue("price")
	type_menu := c.FormValue("type_menu")
	availableStr := c.FormValue("available")
	available := true

	if availableStr == "false" {
		available = false
	}

	if name_menu == "" || priceStr == "" || type_menu == "" {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Message: "Name, price, and type_menu are required",
		})
	}

	price_menu, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Message: "Invalid price format",
		})
	}

	fileHeader, err := c.FormFile("image")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Message: "Image is required",
		})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Message: "Failed to open image",
		})
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	filename := uuid.New().String() + "-" + fileHeader.Filename
	obj := h.StorageBucket.Object("menus/" + filename)
	writer := obj.NewWriter(ctx)

	writer.ObjectAttrs.ACL = []storage.ACLRule{{Entity: storage.AllUsers, Role: storage.RoleReader}}

	if _, err := io.Copy(writer, file); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Message: "Failed to upload image",
		})
	}
	if err := writer.Close(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Message: "Failed to finalize image upload",
		})
	}

	bucketName := "lampu-5a178.firebasestorage.app"
	imageURL_Menu := fmt.Sprintf("https://firebasestorage.googleapis.com/v0/b/%s/o/menus%%2F%s?alt=media", bucketName, filename)

	menu := models.Menu{
		NameMenu:        name_menu,
		DescriptionMenu: description_menu,
		PriceMenu:       price_menu,
		TypeMenu:        type_menu,
		Available:       available,
		ImageURLMenu:    imageURL_Menu,
	}

	if err := h.Repo.CreateMenu(c.Context(), &menu); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Message: "Failed to save menu data",
		})
	}

	// 📌 คืนค่าเมื่อสำเร็จ
	return c.Status(fiber.StatusCreated).JSON(APIResponse{
		Success: true,
		Message: "Menu created successfully",
		Data:    menu,
	})
}

func (h *MenuHandler) GetAllMenus(c *fiber.Ctx) error {
	menus, err := h.Repo.GetAllMenus(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Message: "Failed to fetch menus",
		})
	}

	// 📌 คืนค่าเมื่อสำเร็จ
	return c.Status(fiber.StatusOK).JSON(APIResponse{
		Success: true,
		Message: "Menus fetched successfully",
		Data:    menus,
	})
}

func (h *MenuHandler) GetMenusByType(c *fiber.Ctx) error {
	// ดึงค่า type_menu จาก URL Parameter (เช่น /type/ชุดหลัก)
	typeMenu := c.Params("type_menu")

	if typeMenu == "" {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Message: "type_menu parameter is required",
		})
	}

	menus, err := h.Repo.GetMenusByType(c.Context(), typeMenu)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Message: "Failed to fetch menus by type",
		})
	}

	return c.Status(fiber.StatusOK).JSON(APIResponse{
		Success: true,
		Message: "Menus fetched successfully for type: " + typeMenu,
		Data:    menus,
	})
}

func (h *MenuHandler) UpdateMenu(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Message: "Menu ID is required",
		})
	}

	name_menu := c.FormValue("name_menu")
	description_menu := c.FormValue("description_menu")
	priceStr := c.FormValue("price_menu")
	type_menu := c.FormValue("type_menu")
	availableStr := c.FormValue("available")

	// เตรียมรายการฟิลด์ที่จะส่งไปอัปเดตใน Firestore
	var updates []firestore.Update

	if name_menu != "" {
		updates = append(updates, firestore.Update{Path: "name", Value: name_menu})
	}
	if description_menu != "" {
		updates = append(updates, firestore.Update{Path: "description", Value: description_menu})
	}
	if priceStr != "" {
		price_menu, err := strconv.ParseFloat(priceStr, 64)
		if err == nil {
			updates = append(updates, firestore.Update{Path: "price", Value: price_menu})
		}
	}
	if type_menu != "" {
		updates = append(updates, firestore.Update{Path: "type_menu", Value: type_menu})
	}
	if availableStr != "" {
		available := true
		if availableStr == "false" {
			available = false
		}
		updates = append(updates, firestore.Update{Path: "available", Value: available})
	}

	// 📸 เช็คว่าหน้าบ้านมีการอัปโหลดไฟล์รูปภาพใหม่มาด้วยหรือไม่
	fileHeader, err := c.FormFile("image")
	if err == nil { // ถ้าไม่มี error แปลว่ามีไฟล์รูปใหม่ส่งมา
		file, err := fileHeader.Open()
		if err == nil {
			defer file.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
			defer cancel()

			filename := uuid.New().String() + "-" + fileHeader.Filename
			obj := h.StorageBucket.Object("menus/" + filename)
			writer := obj.NewWriter(ctx)
			writer.ObjectAttrs.ACL = []storage.ACLRule{{Entity: storage.AllUsers, Role: storage.RoleReader}}

			if _, err := io.Copy(writer, file); err == nil {
				writer.Close()
				bucketName := "lampu-5a178.firebasestorage.app"
				imageURL_Menu := fmt.Sprintf("https://firebasestorage.googleapis.com/v0/b/%s/o/menus%%2F%s?alt=media", bucketName, filename)
				updates = append(updates, firestore.Update{Path: "image_url", Value: imageURL_Menu})
			}
		}
	}

	if len(updates) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Message: "No fields to update",
		})
	}

	// ส่งรายการไปอัปเดตที่ Repo
	if err := h.Repo.UpdateMenu(c.Context(), id, updates); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Message: "Failed to update menu",
		})
	}

	return c.Status(fiber.StatusOK).JSON(APIResponse{
		Success: true,
		Message: "Menu updated successfully",
	})
}

func (h *MenuHandler) DeleteMenu(c *fiber.Ctx) error {
	id := c.Params("id")
	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(APIResponse{
			Success: false,
			Message: "Menu ID is required",
		})
	}

	if err := h.Repo.DeleteMenu(c.Context(), id); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(APIResponse{
			Success: false,
			Message: "Failed to delete menu",
		})
	}

	return c.Status(fiber.StatusOK).JSON(APIResponse{
		Success: true,
		Message: "Menu deleted successfully",
	})
}
