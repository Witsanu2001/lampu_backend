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

	"orders/utils"

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

// อย่าลืม import "fmt" และ "time" ไว้ด้านบนของไฟล์ด้วยนะครับ

func (h *OrderHandler) CreateOrder(c *fiber.Ctx) error {
	ctx := context.Background()

	// Get order JSON string from form field
	orderJSON := c.FormValue("order")
	if orderJSON == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.APIResponse{
			Success: false,
			Message: "Order data is required",
		})
	}

	// Parse order JSON
	var order models.Order
	if err := json.Unmarshal([]byte(orderJSON), &order); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to parse order data",
		})
	}

	// ✨ กำหนดสถานะเริ่มต้นของออเดอร์เป็น "new" (ออเดอร์ใหม่)
	order.Status = "new"

	// ----------------------------------------------------
	// ✨ เพิ่มการรับค่า user_id จาก Form Data ตรงนี้
	// ----------------------------------------------------
	userID := c.FormValue("user_id")
	if userID != "" {
		order.UserID = userID
	} else {
		return c.Status(fiber.StatusUnauthorized).JSON(utils.APIResponse{
			Success: false,
			Message: "User ID is required",
		})
	}
	// ----------------------------------------------------

	// ✨ สร้าง Order ID อัตโนมัติ (วันเดือนปี-เวลา-มิลลิวินาที 3 หลัก)
	if order.ID == "" {
		now := time.Now()

		// รูปแบบ: YYYYMMDD-HHMMSS (ใน Go ต้องใช้ตัวเลขมหัศจรรย์ 20060102-150405 เสมอ)
		timeStr := now.Format("20060102-150405")

		// ดึงเศษมิลลิวินาที 3 หลักมาใช้แทนลำดับ (ได้ค่าตั้งแต่ 000 ถึง 999)
		milliSec := now.UnixMilli() % 1000

		// นำมาต่อกัน จะได้ผลลัพธ์เช่น "ORD-20260611-221840-045"
		order.ID = fmt.Sprintf("ORD-%s-%03d", timeStr, milliSec)
	}

	// Handle slip upload if payment method is promptpay and has slip
	if order.Payment.Method == "promptpay" && order.Payment.HasSlip {
		slipFile, err := c.FormFile("slip")
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(utils.APIResponse{
				Success: false,
				Message: "Slip file is required for promptpay payment",
			})
		}

		slipURL, err := h.uploadFile(ctx, slipFile, "slips")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
				Success: false,
				Message: "Failed to upload slip",
			})
		}
		order.SlipURL = slipURL
	}

	// Handle home image upload
	homeImageFile, err := c.FormFile("home_image")
	if err == nil {
		homeImageURL, err := h.uploadFile(ctx, homeImageFile, "home_images")
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
				Success: false,
				Message: "Failed to upload home image",
			})
		}
		order.HomeImageURL = homeImageURL
	}

	// Create order in database
	if err := h.Repo.CreateOrder(ctx, &order); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to create order",
		})
	}

	// ✨ คืนค่าสำเร็จแบบสวยๆ พร้อมแนบข้อมูล Order กลับไปด้วย
	return c.Status(fiber.StatusCreated).JSON(utils.APIResponse{
		Success: true,
		Message: "สร้างออเดอร์สำเร็จ",
		Data:    order,
	})
}

func (h *OrderHandler) GetAllOrders(c *fiber.Ctx) error {
	ctx := context.Background()

	orders, err := h.Repo.GetAllOrders(ctx)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to get orders",
		})
	}

	// คืนค่าสำเร็จพร้อมครอบด้วย APIResponse
	return c.Status(fiber.StatusOK).JSON(utils.APIResponse{
		Success: true,
		Message: "ดึงข้อมูลออเดอร์ทั้งหมดสำเร็จ",
		Data:    orders,
	})
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
		return c.Status(fiber.StatusBadRequest).JSON(utils.APIResponse{
			Success: false,
			Message: "User ID is required",
		})
	}

	// เรียกใช้ฟังก์ชันจาก Repo
	orders, err := h.Repo.GetOrdersByUserID(ctx, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to get orders for this user",
		})
	}

	// คืนค่าสำเร็จพร้อมครอบด้วย APIResponse
	return c.Status(fiber.StatusOK).JSON(utils.APIResponse{
		Success: true,
		Message: "ดึงข้อมูลออเดอร์ของผู้ใช้งานสำเร็จ",
		Data:    orders,
	})
}

func (h *OrderHandler) GetOrderByID(c *fiber.Ctx) error {
	ctx := context.Background()

	// 1. ดึงค่า id (Document ID) จาก URL Parameter
	orderID := c.Params("id")
	if orderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.APIResponse{
			Success: false,
			Message: "Order ID is required",
		})
	}

	// 2. เรียกใช้ Repository เพื่อหาข้อมูลออเดอร์นั้น
	order, err := h.Repo.GetOrderByID(ctx, orderID)
	if err != nil {
		// ดักจับกรณีที่หาออเดอร์ไม่เจอ
		return c.Status(fiber.StatusNotFound).JSON(utils.APIResponse{
			Success: false,
			Message: "Order not found",
		})
	}

	// 3. ส่งข้อมูลกลับไปให้หน้าบ้าน (Frontend) พร้อมครอบด้วย APIResponse
	return c.Status(fiber.StatusOK).JSON(utils.APIResponse{
		Success: true,
		Message: "ดึงรายละเอียดออเดอร์สำเร็จ",
		Data:    order,
	})
}
