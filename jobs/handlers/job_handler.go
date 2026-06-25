package handlers

import (
	"jobs/models"
	"jobs/repository"
	"jobs/utils"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
)

type JobHandler struct {
	repo *repository.JobRepository
}

func NewJobHandler(repo *repository.JobRepository) *JobHandler {
	return &JobHandler{repo: repo}
}

func (h *JobHandler) GetJobUser(c *fiber.Ctx) error {
	// 🌟 ดึง user_id จาก Locals (กรณีใช้ Token Middleware) เพื่อความปลอดภัยสูงสุด
	userID, ok := c.Locals("user_id").(string)

	// 🌟 Fallback: หากยังไม่ได้ต่อ Middleware ให้ดึงจาก Query String แทนได้เหมือนเดิม
	if !ok || userID == "" {
		userID = c.Query("user_id")
	}

	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id is required",
		})
	}

	// เรียกใช้ฟังก์ชันจาก Repository (ซึ่งตอนนี้จะรวมข้อมูลออเดอร์มาให้แล้ว)
	jobs, err := h.repo.GetJobsByUser(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch jobs with details: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(utils.APIResponse{
		Success: true,
		Message: "success",
		Data:    jobs,
	})
}

func (h *JobHandler) GetJobByID(c *fiber.Ctx) error {
	ctx := c.UserContext()
	orderID := c.Params("id")
	if orderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(utils.APIResponse{
			Success: false,
			Message: "Order ID is required",
		})
	}
	order, err := h.repo.GetJobByID(ctx, orderID)
	if err != nil {
		// ดักจับกรณีที่เกิด Error หรือหาออเดอร์ไม่เจอ
		return c.Status(fiber.StatusNotFound).JSON(utils.APIResponse{
			Success: false,
			Message: "Order not found or error occurred",
		})
	}
	return c.Status(fiber.StatusOK).JSON(utils.APIResponse{
		Success: true,
		Message: "ดึงรายละเอียดออเดอร์สำเร็จ",
		Data:    order,
	})
}

func (h *JobHandler) GetHistory(c *fiber.Ctx) error {
	// ดึง ID ผู้ใช้
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		userID = c.Query("user_id")
	}

	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id is required"})
	}

	// 🌟 รับค่า Date, Page, Limit จาก URL Query
	dateStr := c.Query("date") // ถ้าไม่มีจะส่งสตริงว่างมา

	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil {
		page = 1
	}

	limit, err := strconv.Atoi(c.Query("limit", "10"))
	if err != nil {
		limit = 10
	}

	// 🌟 เรียก Repository พร้อมส่งค่า page และ limit เข้าไป
	history, err := h.repo.GetHistory(c.Context(), userID, dateStr, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch history: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "success",
		"data":    history,
	})
}

func (h *JobHandler) GetJobSummary(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		userID = c.Query("user_id")
	}

	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id is required",
		})
	}

	// 🌟 รับค่าวันที่จาก Query String ในรูปแบบ YYYY-MM-DD
	dateStr := c.Query("date")
	if dateStr == "" {
		// ถ้าไม่ได้ส่งมา ให้ใช้วันนี้เป็นค่าเริ่มต้น
		dateStr = time.Now().Format("2006-01-02")
	}

	// 🌟 ส่ง userID (ซึ่งก็คือ rider_id) และ dateStr ไปให้ Repo
	summary, err := h.repo.GetJobSummary(c.Context(), userID, dateStr)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch job summary: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(utils.APIResponse{
		Success: true,
		Message: "success",
		Data:    summary,
	})
}

func (h *JobHandler) GetStove(c *fiber.Ctx) error {
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		userID = c.Query("user_id")
	}

	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id is required",
		})
	}

	// เรียกใช้ฟังก์ชันจาก Repository (ซึ่งตอนนี้จะรวมข้อมูลออเดอร์มาให้แล้ว)
	jobs, err := h.repo.GetStove(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch jobs with details: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(utils.APIResponse{
		Success: true,
		Message: "success",
		Data:    jobs,
	})
}

func (h *JobHandler) GetStoveSuccess(c *fiber.Ctx) error {
	// ดึง ID ผู้ใช้
	userID, ok := c.Locals("user_id").(string)
	if !ok || userID == "" {
		userID = c.Query("user_id")
	}

	if userID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "user_id is required"})
	}

	// 🌟 รับค่า Date, Page, Limit จาก URL Query
	dateStr := c.Query("date") // ถ้าไม่มีจะส่งสตริงว่างมา

	page, err := strconv.Atoi(c.Query("page", "1"))
	if err != nil {
		page = 1
	}

	limit, err := strconv.Atoi(c.Query("limit", "10"))
	if err != nil {
		limit = 10
	}

	// 🌟 เรียก Repository พร้อมส่งค่า page และ limit เข้าไป
	history, err := h.repo.GetStoveSuccess(c.Context(), userID, dateStr, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch history: " + err.Error(),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "success",
		"data":    history,
	})
}

func (h *JobHandler) GetStoveByRiderId(c *fiber.Ctx) error {
	riderID := c.Query("rider_id")
	if riderID == "" {
		if tokenID, ok := c.Locals("user_id").(string); ok {
			riderID = tokenID
		}
	}

	if riderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "rider_id or authorization token is required",
		})
	}

	pageStr := c.Query("page", "1")
	limitStr := c.Query("limit", "10")

	// 🌟 2. แปลงเป็นตัวเลข (int)
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1 // ถ้าส่งมาผิดรูปแบบ หรือติดลบ ให้เป็นหน้า 1 เสมอ
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10 // ถ้าส่งมาผิดรูปแบบ ให้จำกัดที่ 10 รายการ
	}
	if limit > 100 {
		limit = 100 // ป้องกันโดนยิง request ขอทีละล้านรายการ (Max 100)
	}

	// 🌟 3. ส่ง page และ limit เข้าไปใน Repo
	jobs, err := h.repo.GetStoveByRiderId(c.Context(), riderID, page, limit)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch jobs with details: " + err.Error(),
		})
	}

	// 🌟 4. ส่งกลับไปพร้อม Metadata เพื่อให้ Frontend รู้ว่าอยู่หน้าไหน
	return c.Status(fiber.StatusOK).JSON(utils.APIResponse{
		Success: true,
		Message: "success",
		Data: fiber.Map{
			"items": jobs,
			"meta": fiber.Map{
				"page":  page,
				"limit": limit,
				"count": len(jobs),
			},
		},
	})
}

func (h *JobHandler) PostStoveStatusFalse(c *fiber.Ctx) error {
	var req models.UpdateStoveStatusRequest

	// 1. รับค่าจาก Request Body
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "รูปแบบข้อมูลไม่ถูกต้อง",
		})
	}

	// 2. ตรวจสอบข้อมูลเบื้องต้น
	if req.OrderID == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "กรุณาระบุ order_id",
		})
	}

	if !req.IsComplete && req.Reason == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "กรุณาระบุเหตุผลกรณีที่เก็บอุปกรณ์ไม่ครบ",
		})
	}

	// ดึง ID ไรเดอร์จาก Token (เพื่อเอาไปบันทึกว่าใครเป็นคนรายงาน)
	riderID, _ := c.Locals("user_id").(string)

	// 3. เรียกใช้งาน Repository
	err := h.repo.PostStoveStatusFalse(c.Context(), req, riderID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "ไม่สามารถอัปเดตสถานะได้: " + err.Error(),
		})
	}

	// 4. ส่งผลลัพธ์กลับ
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "อัปเดตสถานะการเก็บเตาเรียบร้อยแล้ว",
	})
}
