package handlers

import (
	"context"
	"fmt"
	"io"
	"orders/models"
	"orders/repository"
	"strconv"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type MenuHandler struct {
	Repo          *repository.MenuRepository
	StorageBucket *storage.BucketHandle
}

// ฟังก์ชันนี้แหละที่ main.go มองหาอยู่!
func NewMenuHandler(repo *repository.MenuRepository, bucket *storage.BucketHandle) *MenuHandler {
	return &MenuHandler{Repo: repo, StorageBucket: bucket}
}

func (h *MenuHandler) CreateMenu(c *fiber.Ctx) error {
	name := c.FormValue("name")
	description := c.FormValue("description")
	priceStr := c.FormValue("price")
	category := c.FormValue("category")
	availableStr := c.FormValue("available")
	available := true

	if availableStr == "false" {
		available = false
	}

	if name == "" || priceStr == "" || category == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Name, price, and category are required"})
	}

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid price format"})
	}

	fileHeader, err := c.FormFile("image")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Image is required"})
	}

	file, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to open image"})
	}
	defer file.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	filename := uuid.New().String() + "-" + fileHeader.Filename
	obj := h.StorageBucket.Object("menus/" + filename)
	writer := obj.NewWriter(ctx)

	writer.ObjectAttrs.ACL = []storage.ACLRule{{Entity: storage.AllUsers, Role: storage.RoleReader}}

	if _, err := io.Copy(writer, file); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to upload image"})
	}
	if err := writer.Close(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to finalize image upload"})
	}

	bucketName := "lampu-5a178.firebasestorage.app"
	imageURL := fmt.Sprintf("https://firebasestorage.googleapis.com/v0/b/%s/o/menus%%2F%s?alt=media", bucketName, filename)

	menu := models.Menu{
		Name:        name,
		Description: description,
		Price:       price,
		Category:    category,
		Available:   available,
		ImageURL:    imageURL,
	}

	if err := h.Repo.CreateMenu(c.Context(), &menu); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to save menu data"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Menu created successfully",
		"data":    menu,
	})
}
