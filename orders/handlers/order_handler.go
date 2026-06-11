package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"orders/models"
	"orders/repository"
	"strings"
	"time"

	"cloud.google.com/go/storage"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type OrderHandler struct {
	Repo          *repository.OrderRepository
	StorageBucket *storage.BucketHandle
	BucketName    string
}

func NewOrderHandler(repo *repository.OrderRepository, bucket *storage.BucketHandle, bucketName string) *OrderHandler {
	return &OrderHandler{Repo: repo, StorageBucket: bucket, BucketName: bucketName}
}

func (h *OrderHandler) CreateOrder(c *fiber.Ctx) error {
	ctx := context.Background()

	// Get order JSON string from form field
	orderJSON := c.FormValue("order")
	if orderJSON == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Order data is required",
		})
	}

	// Parse order JSON
	var order models.Order
	if err := json.Unmarshal([]byte(orderJSON), &order); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Failed to parse order data",
		})
	}

	// ----------------------------------------------------
	// ✨ เพิ่มการรับค่า user_id จาก Form Data ตรงนี้
	// ----------------------------------------------------
	userID := c.FormValue("user_id")
	if userID != "" {
		order.UserID = userID
	} else {
		// หากต้องการบังคับว่าต้องล็อกอินถึงจะสั่งได้ ให้ดัก Error ตรงนี้
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}
	// ----------------------------------------------------

	// Generate ID if not provided
	if order.ID == "" {
		order.ID = uuid.New().String()
	}

	// Handle slip upload if payment method is promptpay and has slip
	if order.Payment.Method == "promptpay" && order.Payment.HasSlip {
		slipFile, err := c.FormFile("slip")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Slip file is required for promptpay payment",
			})
		}

		slipURL, err := h.uploadFile(ctx, slipFile, "slips")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to upload slip",
			})
		}
		order.SlipURL = slipURL
	}

	// Handle home image upload
	homeImageFile, err := c.FormFile("home_image")
	if err == nil {
		homeImageURL, err := h.uploadFile(ctx, homeImageFile, "home_images")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to upload home image",
			})
		}
		order.HomeImageURL = homeImageURL
	}

	// Create order in database
	if err := h.Repo.CreateOrder(ctx, &order); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create order",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(order)
}

func (h *OrderHandler) GetAllOrders(c *fiber.Ctx) error {
	ctx := context.Background()

	orders, err := h.Repo.GetAllOrders(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get orders",
		})
	}

	return c.JSON(orders)
}

func (h *OrderHandler) uploadFile(ctx context.Context, file *multipart.FileHeader, folder string) (string, error) {
	// Open the uploaded file
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	// Generate unique filename
	ext := strings.Split(file.Filename, ".")
	extension := ext[len(ext)-1]
	filename := fmt.Sprintf("%s/%s_%s.%s", folder, uuid.New().String(), time.Now().Format("20060102150405"), extension)

	// Create a writer to the bucket
	obj := h.StorageBucket.Object(filename)
	writer := obj.NewWriter(ctx)

	// Copy the file content
	if _, err := io.Copy(writer, src); err != nil {
		return "", err
	}

	// Close the writer to finalize the upload
	if err := writer.Close(); err != nil {
		return "", err
	}

	// Make the object publicly accessible (optional)
	if err := obj.ACL().Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
		return "", err
	}

	// Return the public URL using Firebase Storage format
	encodedFilename := strings.ReplaceAll(filename, "/", "%2F")
	return fmt.Sprintf("https://firebasestorage.googleapis.com/v0/b/%s/o/%s?alt=media", h.BucketName, encodedFilename), nil
}

func (h *OrderHandler) GetByUserID(c *fiber.Ctx) error {
	ctx := context.Background()

	// ดึงค่า user_id จาก URL Parameter (/:user_id/orderByUser)
	userID := c.Params("user_id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User ID is required",
		})
	}

	// เรียกใช้ฟังก์ชันจาก Repo (ที่คุณกำลังจะสร้างในขั้นตอนต่อไป)
	orders, err := h.Repo.GetOrdersByUserID(ctx, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to get orders for this user",
		})
	}

	return c.JSON(orders)
}

func (h *OrderHandler) GetOrderByID(c *fiber.Ctx) error {
	ctx := context.Background()

	// 1. ดึงค่า id (Document ID) จาก URL Parameter
	orderID := c.Params("id")
	if orderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Order ID is required",
		})
	}

	// 2. เรียกใช้ Repository เพื่อหาข้อมูลออเดอร์นั้น
	order, err := h.Repo.GetOrderByID(ctx, orderID)
	if err != nil {
		// ดักจับกรณีที่หาออเดอร์ไม่เจอ
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Order not found",
		})
	}

	// 3. ส่งข้อมูลกลับไปให้หน้าบ้าน (Frontend)
	return c.JSON(order)
}
