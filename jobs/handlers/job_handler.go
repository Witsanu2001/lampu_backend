package handlers

import (
	"jobs/repository"
	"jobs/utils"

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
