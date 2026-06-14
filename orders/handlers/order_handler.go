package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
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

	order.Status = "new"

	userID := c.FormValue("user_id")
	if userID != "" {
		order.UserID = userID
	} else {
		return c.Status(fiber.StatusUnauthorized).JSON(utils.APIResponse{
			Success: false,
			Message: "User ID is required",
		})
	}

	if order.Shipping.LocationID != "" {
		locationData, err := h.Repo.GetLocationByID(ctx, order.Shipping.LocationID)
		if err != nil {
			log.Printf("Error fetching location details: %v", err)
			return c.Status(fiber.StatusBadRequest).JSON(utils.APIResponse{
				Success: false,
				Message: "ไม่พบข้อมูลที่อยู่จัดส่งที่ระบุ (Location not found)",
			})
		}

		order.Shipping.Recipient = locationData.Name
		order.Shipping.Phone = locationData.Phone
		order.Shipping.Address = locationData.Details
		order.Shipping.Note = locationData.Note
		order.Shipping.Location.Lat = locationData.Location.Lat
		order.Shipping.Location.Lng = locationData.Location.Lng
	}
	// ----------------------------------------------------

	if order.ID == "" {
		now := time.Now()
		timeStr := now.Format("20060102-150405")

		startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		endOfDay := startOfDay.Add(24 * time.Hour)

		countToday, err := h.Repo.GetTodayOrderCount(ctx, startOfDay, endOfDay)
		if err != nil {
			log.Printf("Error counting today's orders: %v", err)
		}

		nextSeq := countToday + 1
		order.ID = fmt.Sprintf("ORD-%s-%03d", timeStr, nextSeq)
	}

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

	if err := h.Repo.CreateOrder(ctx, &order); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to create order",
		})
	}

	lampuAdminUID := "U9728d3e3d66a3af73ee87768874cee0d"

	lineMsg := fmt.Sprintf("🔔 มีออเดอร์ใหม่เข้าครับ!\nเลขออเดอร์: %s\nยอดรวม: %.2f บาท\nช่องทางชำระเงิน: %s\nพิกัดจัดส่ง: %s",
		order.ID,
		order.Totals.GrandTotal,
		order.Payment.Method,
		order.Shipping.Recipient)

	// ✨ ลบคำว่า 'go' ออก แล้วเขียนรับค่า Error ตรงๆ พร้อมพิมพ์ Log ก่อนและหลังส่ง
	log.Println("⏳ กำลังพยายามส่ง LINE ไปที่ UID:", lampuAdminUID)

	errLine := utils.SendOrderAdminNotification(lampuAdminUID, lineMsg)

	if errLine != nil {
		log.Printf("❌ ส่ง LINE ขัดข้อง Error: %v\n", errLine)
	} else {
		log.Println("✅ ส่ง LINE สำเร็จเรียบร้อย!")
	}

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
		Message: "ดึงข้อมูลออเดอร์ของวันนี้สำเร็จ",
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

		log.Printf("🔥 Firestore Error: %v", err)

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

func (h *OrderHandler) UpdateOrderStatus(c *fiber.Ctx) error {
	ctx := context.Background()

	// ดึง order_id จาก URL Parameter
	orderID := c.Params("id")
	if orderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Order ID is required",
		})
	}

	// ดึงค่า user_id และ status จาก Request Body
	var req models.UpdateStatusRequest
	if err := c.BodyParser(&req); err != nil {
		req.UserID = c.FormValue("user_id")
		req.Status = c.FormValue("status")
	}

	// ตรวจสอบความถูกต้องของข้อมูล
	if req.Status == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Status is required",
		})
	}
	if req.UserID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User ID is required",
		})
	}

	// ส่งให้ Repository จัดการอัปเดต
	err := h.Repo.UpdateOrderStatus(ctx, orderID, req.Status, req.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update order status: " + err.Error(),
		})
	}

	// สร้างข้อความตอบกลับ
	var responseMsg string
	switch req.Status {
	case "preparing":
		responseMsg = "รับออเดอร์เรียบร้อยแล้ว กำลังเตรียมอาหาร 🥘"
	case "refuse":
		responseMsg = "ปฏิเสธออเดอร์นี้เรียบร้อยแล้ว ❌"
	case "ready":
		responseMsg = "มอบหมายงานสำเร็จ อาหารพร้อมส่งแล้ว 🛵"
	case "shipping":
		responseMsg = "กำลังนำส่งอาหารให้ลูกค้า 🚀"
	case "delivered":
		responseMsg = "จัดส่งสำเร็จ ปิดออเดอร์เรียบร้อย 🎉"
	default:
		responseMsg = "อัปเดตสถานะสำเร็จ"
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": responseMsg,
	})
}

func (h *OrderHandler) BulkAssignJobs(c *fiber.Ctx) error {
	ctx := context.Background()
	var req models.BulkAssignRequest

	// แปลง JSON ก้อนใหญ่ที่ส่งมา
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "รูปแบบข้อมูลไม่ถูกต้อง",
		})
	}

	if len(req.Jobs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "ไม่มีออเดอร์ให้มอบหมาย",
		})
	}

	// ส่งไปบันทึก
	err := h.Repo.BulkAssignJobs(ctx, req.Jobs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "บันทึกข้อมูลล้มเหลว: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "มอบหมายงานและจัดคิวเรียบร้อยแล้ว 🛵",
	})
}
