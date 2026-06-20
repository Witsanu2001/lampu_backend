package handlers

import (
	"systems/repository"
	"systems/utils"

	"github.com/gofiber/fiber/v2"
)

type SystemHandler struct {
	repo *repository.SystemRepository
}

func NewSystemHandler(repo *repository.SystemRepository) *SystemHandler {
	return &SystemHandler{repo: repo}
}

func (h *SystemHandler) GetSystem(c *fiber.Ctx) error {
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

	jobs, err := h.repo.GetSystem(c.Context())
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
