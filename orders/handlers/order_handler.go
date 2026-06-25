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
	"os"
	"strconv"
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

	lineMsg := fmt.Sprintf("🔔 มีออเดอร์ใหม่เข้า!\nเลขออเดอร์: %s\nยอดรวม: %.2f บาท\nช่องทางชำระเงิน: %s\nพิกัดจัดส่ง: %s",
		order.ID,
		order.Totals.GrandTotal,
		order.Payment.Method,
		order.Shipping.Address)

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

func (h *OrderHandler) GetAllOrdersByID(c *fiber.Ctx) error {
	ctx := context.Background()

	// 🌟 1. ดึงค่า id (User ID) จาก URL Parameter
	userID := c.Params("id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.APIResponse{
			Success: false,
			Message: "User ID is required",
		})
	}

	// 🌟 2. ส่ง userID ไปให้ Repository ทำการคิวรี
	orders, err := h.Repo.GetAllOrdersByID(ctx, userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to get orders: " + err.Error(),
		})
	}

	// 3. คืนค่าสำเร็จพร้อมครอบด้วย APIResponse
	return c.Status(fiber.StatusOK).JSON(utils.APIResponse{
		Success: true,
		Message: "ดึงข้อมูลออเดอร์ทั้งหมดของผู้ใช้สำเร็จ",
		Data:    orders,
	})
}

func (h *OrderHandler) GetSuccessOrders(c *fiber.Ctx) error {
	ctx := context.Background()

	// ดึงค่าจาก Query Parameter
	dateStr := c.Query("date")
	page := c.QueryInt("page", 1)    // ถ้าไม่ส่งมา ให้เป็นหน้า 1
	limit := c.QueryInt("limit", 10) // ถ้าไม่ส่งมา ให้เป็น 10

	targetDate := time.Now()
	if dateStr != "" {
		parsedDate, err := time.ParseInLocation("2006-01-02", dateStr, time.Local)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(utils.APIResponse{
				Success: false,
				Message: "รูปแบบวันที่ไม่ถูกต้อง กรุณาใช้ YYYY-MM-DD",
			})
		}
		targetDate = parsedDate
	}

	orders, err := h.Repo.GetSuccessOrders(ctx, targetDate, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to get orders: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(utils.APIResponse{
		Success: true,
		Message: "ดึงข้อมูลออเดอร์สำเร็จ",
		Data:    orders,
		Meta: map[string]interface{}{
			"page":  page,
			"limit": limit,
		},
	})
}

func (h *OrderHandler) GetOrdersPDF(c *fiber.Ctx) error {
	ctx := context.Background()

	// 1. รับค่า Start Date และ End Date
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.APIResponse{
			Success: false,
			Message: "กรุณาระบุ start_date และ end_date (YYYY-MM-DD)",
		})
	}

	// 2. แปลง String เป็น Time
	startDate, errStart := time.ParseInLocation("2006-01-02", startDateStr, time.Local)
	endDate, errEnd := time.ParseInLocation("2006-01-02", endDateStr, time.Local)
	if errStart != nil || errEnd != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.APIResponse{
			Success: false,
			Message: "รูปแบบวันที่ไม่ถูกต้อง กรุณาใช้ YYYY-MM-DD",
		})
	}

	// 3. ดึงข้อมูลจาก Repo ตามช่วงเวลา
	orders, err := h.Repo.GetOrdersPDF(ctx, startDate, endDate)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to get orders: " + err.Error(),
		})
	}

	if len(orders) == 0 {
		return c.Status(fiber.StatusNotFound).JSON(utils.APIResponse{
			Success: false,
			Message: "ไม่พบข้อมูลออเดอร์ในช่วงเวลาที่กำหนด",
		})
	}

	// 4. สร้าง PDF
	pdfPath, err := utils.GenerateOrderPDF(orders)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to generate PDF: " + err.Error(),
		})
	}

	// 🌟 ตั้งค่าให้ลบไฟล์ PDF ในเครื่องเซิร์ฟเวอร์ทิ้งเสมอเมื่อฟังก์ชันทำงานเสร็จ (ประหยัดพื้นที่)
	defer os.Remove(pdfPath)

	// 5. 🌟 อัปโหลดขึ้น Storage แทนการดาวน์โหลด
	// ใช้ Unix Time ต่อท้ายเพื่อป้องกันชื่อไฟล์ซ้ำกัน
	destFileName := fmt.Sprintf("reports/orders_report_%s_to_%s_%d.pdf", startDateStr, endDateStr, time.Now().Unix())

	pdfURL, err := utils.UploadPDFToStorage(ctx, pdfPath, destFileName)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "อัปโหลด PDF ไม่สำเร็จ: " + err.Error(),
		})
	}

	// 6. 🌟 ส่ง URL กลับไปให้หน้าบ้าน (React) ในรูปแบบ JSON
	return c.Status(fiber.StatusOK).JSON(utils.APIResponse{
		Success: true,
		Message: "สร้างและอัปโหลดรายงานสำเร็จ",
		Data:    pdfURL, // หน้าบ้านจะเอา URL นี้ไปแสดงผลใน iframe หรือให้ผู้ใช้กดโหลดได้เลย
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
	userID := c.Params("user_id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.APIResponse{
			Success: false,
			Message: "User ID is required",
		})
	}

	orders, err := h.Repo.GetOrdersByUserID(ctx, userID)
	if err != nil {

		log.Printf("🔥 Firestore Error: %v", err)

		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to get orders for this user",
		})
	}

	return c.Status(fiber.StatusOK).JSON(utils.APIResponse{
		Success: true,
		Message: "ดึงข้อมูลออเดอร์ของผู้ใช้งานสำเร็จ",
		Data:    orders,
	})
}

func (h *OrderHandler) GetOrderByUserToday(c *fiber.Ctx) error {
	ctx := context.Background()
	userID := c.Params("user_id")
	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.APIResponse{
			Success: false,
			Message: "User ID is required",
		})
	}

	orders, err := h.Repo.GetOrderByUserToday(ctx, userID)
	if err != nil {

		log.Printf("🔥 Firestore Error: %v", err)

		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to get orders for this user",
		})
	}

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

	orderID := c.Params("id")
	if orderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Order ID is required",
		})
	}

	var req models.UpdateStatusRequest
	if err := c.BodyParser(&req); err != nil {
		req.UserID = c.FormValue("user_id")
		req.Status = c.FormValue("status")
	}

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

	finalStatus, err := h.Repo.UpdateOrderStatus(ctx, orderID, req.Status, req.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update order status: " + err.Error(),
		})
	}

	order, errOrder := h.Repo.GetOrderByID(ctx, orderID)
	var orderDetails string

	if errOrder == nil {
		orderDetails = "\n\n📋 รายการอาหาร:"

		for _, item := range order.MainItems {
			orderDetails += fmt.Sprintf("\n- %s", item.Name)
		}

		if len(order.AddOnItems) > 0 {
			orderDetails += "\n\n➕ เพิ่มเติม:"
			for _, item := range order.AddOnItems {
				orderDetails += fmt.Sprintf("\n- %s", item.Name)
			}
		}

		orderDetails += fmt.Sprintf("\n\n💰 ยอดรวม: %.2f บาท", order.Totals.GrandTotal)
		orderDetails += fmt.Sprintf("\n💳 ชำระเงิน: %s", order.Payment.Method)
	} else {
		log.Printf("⚠️ ไม่สามารถดึงข้อมูลออเดอร์มาแสดงใน LINE ได้: %v\n", errOrder)
	}

	var responseMsg string
	lampuDeliveryUID := req.UserID
	var lineMsg string

	switch finalStatus {
	case "preparing":
		responseMsg = "รับออเดอร์เรียบร้อยแล้ว กำลังเตรียมอาหาร 🥘"
		lineMsg = "🥘 รับออเดอร์เรียบร้อยแล้ว กำลังเตรียมอาหาร"

	case "cancel":
		responseMsg = "ยกเลิกออเดอร์เรียบร้อยแล้ว ❌"
		lineMsg = "❌ ยกเลิกออเดอร์เรียบร้อยแล้ว" + orderDetails

	case "shipping":
		responseMsg = "กำลังนำส่งอาหารให้ลูกค้า 🚀"
		if errOrder == nil {
			if order.Payment.Method == "promptpay" {
				lineMsg = "🚀 กำลังนำส่งหมูกระทะให้คุณ รอกินได้เลย 😋"
			} else if order.Payment.Method == "เก็บเงินปลายทาง" {
				lineMsg = "🚀 กำลังนำส่งหมูกระทะให้คุณ กรุณาเตรียมเงินให้พร้อมด้วยนะครับ 💵"
			} else {
				lineMsg = "🚀 กำลังนำส่งอาหารให้ลูกค้า"
			}
		} else {
			lineMsg = "🚀 กำลังนำส่งอาหารให้ลูกค้า"
		}

	case "delivered":
		responseMsg = "จัดส่งสำเร็จ ปิดออเดอร์เรียบร้อย 🎉"

		if errOrder == nil {
			if order.Equipment.NeedEquipment {
				lineMsg = "🎉 จัดส่งสำเร็จ กรุณาเก็บเตากับกระทะไว้ในที่ปลอดภัยพ้นจากน้ำ และตั้งในจุดที่มองเห็นง่ายด้วยนะครับ 🙏"
			} else {
				lineMsg = "🎉 จัดส่งสำเร็จ ขอขอบคุณที่อุดหนุนมากๆ นะครับ 🥰"
			}
		} else {
			lineMsg = "🎉 จัดส่งสำเร็จ ปิดออเดอร์เรียบร้อย"
		}

	default:
		responseMsg = "อัปเดตสถานะเป็น " + finalStatus + " สำเร็จ"
	}

	if lineMsg != "" {
		log.Println("⏳ กำลังพยายามส่ง LINE ไปที่ UID:", lampuDeliveryUID)
		errLine := utils.SendOrderUserNotification(lampuDeliveryUID, lineMsg)
		if errLine != nil {
			log.Printf("❌ ส่ง LINE ขัดข้อง Error: %v\n", errLine)
		} else {
			log.Println("✅ ส่ง LINE แจ้งเตือนสำเร็จเรียบร้อย!")
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": responseMsg,
		"status":  finalStatus,
	})
}

func (h *OrderHandler) UpdateOrderCancel(c *fiber.Ctx) error {
	ctx := context.Background()

	// 1. รับ Order ID จาก URL Params
	orderID := c.Params("id")
	if orderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Order ID is required",
		})
	}

	var req struct {
		UserID string `json:"user_id" form:"user_id"`
		Reason string `json:"reason" form:"reason"`
	}

	if err := c.BodyParser(&req); err != nil {
		req.UserID = c.FormValue("user_id")
		req.Reason = c.FormValue("reason")
	}

	if req.UserID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "User ID is required",
		})
	}
	if req.Reason == "" {
		req.Reason = "ไม่ได้ระบุเหตุผล"
	}

	// 🌟 เพิ่มเงื่อนไขต่อท้ายเหตุผลตรงนี้

	// switch req.Reason {
	// case "การชำระเงินไม่ถูกต้อง":
	// 	req.Reason += " กรุณาแก้ไขชำระเงินให้ถูกต้อง"
	// case "อยู่นอกพื้นที่ให้บริการ":
	// 	req.Reason += " กรุณาเปลี่ยนตำแหน่งและนัดรับออเดอร์แทน"
	// case "วัตถุดิบหมด", "สินค้าหมด / วัตถุดิบไม่พอ":
	// 	req.Reason += " ไว้โอกาสหน้านะคะ ต้องขออภัยด้วยคะ"
	// }

	switch req.Reason {
	case "การชำระเงินไม่ถูกต้อง":
		req.Reason += " กรุณาแก้ไขชำระเงินให้ถูกต้อง"
	case "วัตถุดิบหมด", "สินค้าหมด / วัตถุดิบไม่พอ":
		req.Reason += " ไว้โอกาสหน้านะคะ ต้องขออภัยด้วยคะ"
	}

	err := h.Repo.CancelOrder(ctx, orderID, req.Reason, req.UserID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to cancel order: " + err.Error(),
		})
	}

	responseMsg := "ปฏิเสธออเดอร์นี้เรียบร้อยแล้ว ❌"
	lineMsg := fmt.Sprintf("❌ ร้านปฏิเสธ/ยกเลิกออเดอร์\nเลขออเดอร์: %s\nเหตุผล: %s", orderID, req.Reason)
	lampuDeliveryUID := req.UserID

	if lineMsg != "" {
		log.Println("⏳ กำลังพยายามส่ง LINE ไปที่ UID:", lampuDeliveryUID)
		errLine := utils.SendOrderUserNotification(lampuDeliveryUID, lineMsg)
		if errLine != nil {
			log.Printf("❌ ส่ง LINE ขัดข้อง Error: %v\n", errLine)
		} else {
			log.Println("✅ ส่ง LINE แจ้งเตือนสำเร็จเรียบร้อย!")
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": responseMsg,
		"status":  "refuse",
		"reason":  req.Reason,
	})
}

// UpdateEditSlips รับไฟล์สลิปใหม่และอัปเดตข้อมูล
func (h *OrderHandler) UpdateEditSlips(c *fiber.Ctx) error {
	ctx := context.Background()

	// 1. รับ Order ID จาก Params
	orderID := c.Params("order_id")
	if orderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.APIResponse{
			Success: false,
			Message: "Order ID is required",
		})
	}

	// 2. รับไฟล์รูปภาพที่ส่งมาจากหน้าบ้าน
	slipFile, err := c.FormFile("slip")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(utils.APIResponse{
			Success: false,
			Message: "กรุณาอัปโหลดไฟล์สลิป (slip is required)",
		})
	}

	// 3. นำไฟล์ไปอัปโหลดขึ้น Firebase Storage (ใช้ฟังก์ชัน uploadFile เดิมที่คุณมีอยู่แล้ว)
	newSlipURL, err := h.uploadFile(ctx, slipFile, "slips")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to upload new slip: " + err.Error(),
		})
	}

	// 4. เรียก Repo เพื่ออัปเดตข้อมูลลงฐานข้อมูล
	err = h.Repo.UpdateSlip(ctx, orderID, newSlipURL)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to update order with new slip: " + err.Error(),
		})
	}

	adminUID := "U9728d3e3d66a3af73ee87768874cee0d"
	lineMsg := fmt.Sprintf("⚠️ แจ้งเตือน: มีลูกค้าอัปโหลดสลิปมาใหม่!\nเลขออเดอร์: %s\nกรุณาตรวจสอบและยืนยันออเดอร์ในระบบ", orderID)
	errLine := utils.SendOrderAdminNotification(adminUID, lineMsg)
	if errLine != nil {
		log.Printf("❌ ส่ง LINE แจ้งเตือนแอดมินขัดข้อง Error: %v\n", errLine)
	} else {
		log.Println("✅ ส่ง LINE แจ้งเตือนแอดมินสำเร็จเรียบร้อย!")
	}

	// 5. ส่ง Response กลับ
	return c.Status(fiber.StatusOK).JSON(utils.APIResponse{
		Success: true,
		Message: "อัปโหลดสลิปใหม่สำเร็จ",
	})
}

func (h *OrderHandler) BulkAssignJobs(c *fiber.Ctx) error {
	ctx := context.Background()
	var req models.BulkAssignRequest

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

	riderID := req.Jobs[0].RiderID
	err := h.Repo.BulkAssignJobs(ctx, riderID, req.Jobs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "บันทึกข้อมูลล้มเหลว: " + err.Error(),
		})
	}

	lampuDeliveryUID := riderID

	for _, job := range req.Jobs {

		orderData, err := h.Repo.GetOrderByID(ctx, job.OrderID)
		if err != nil {
			continue
		}

		errLine := utils.SendOrderRiderNotification(
			lampuDeliveryUID,
			orderData.MainItems,
			orderData.Totals.GrandTotal,
			orderData.Payment.Method,
			orderData.Shipping.Location.Lat,
			orderData.Shipping.Location.Lng,
		)

		if errLine != nil {
			log.Printf("❌ ส่ง LINE ขัดข้องสำหรับออเดอร์ %s Error: %v\n", job.OrderID, errLine)
		} else {
			log.Printf("✅ ส่ง LINE แจ้งไรเดอร์สำเร็จเรียบร้อย (ออเดอร์: %s)!\n", job.OrderID)
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "มอบหมายงานและจัดคิวเรียบร้อยแล้ว 🛵",
	})
}

func (h *OrderHandler) GetNewOrders(c *fiber.Ctx) error {
	ctx := c.Context()

	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(utils.APIResponse{
			Success: false,
			Message: "Unauthorized",
		})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	orders, err := h.Repo.GetNewOrders(ctx, userID, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to get orders: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(utils.APIResponse{
		Success: true,
		Message: "ดึงข้อมูลออเดอร์สำเร็จ",
		Data:    orders,
	})
}

func (h *OrderHandler) GetDeliveryOrders(c *fiber.Ctx) error {
	ctx := c.Context()

	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(utils.APIResponse{
			Success: false,
			Message: "Unauthorized",
		})
	}

	// 🌟 รับค่า page และ limit
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit, _ := strconv.Atoi(c.Query("limit", "10"))

	orders, err := h.Repo.GetDeliveryOrders(ctx, userID, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(utils.APIResponse{
			Success: false,
			Message: "Failed to get orders: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(utils.APIResponse{
		Success: true,
		Message: "ดึงข้อมูลการจัดส่งสำเร็จ",
		Data:    orders,
	})
}
