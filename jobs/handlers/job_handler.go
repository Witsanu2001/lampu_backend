package handlers

import (
	"jobs/models"
	"jobs/repository"
	"jobs/utils"
	"strconv"

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
	// 🌟 1. ดึงจาก URL Parameters ก่อน (?rider_id=xxxx)
	riderID := c.Query("rider_id")

	// 🌟 2. ถ้าไม่ได้ส่งทาง URL มา ให้ใช้ ID จากคนที่ล็อกอิน (Token)
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

	// เรียกใช้ฟังก์ชันจาก Repository
	jobs, err := h.repo.GetStoveByRiderId(c.Context(), riderID)
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
