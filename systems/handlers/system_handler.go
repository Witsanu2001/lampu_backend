package handlers

import (
	"context"
	"systems/models"
	"systems/repository"

	"github.com/gofiber/fiber/v2"
)

type SystemHandler struct {
	repo *repository.SystemRepository
}

func NewSystemHandler(repo *repository.SystemRepository) *SystemHandler {
	return &SystemHandler{repo: repo}
}

func (h *SystemHandler) AddSystem(c *fiber.Ctx) error {
	var payload models.SystemSettingsPayload
	if err := c.BodyParser(&payload); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "รูปแบบข้อมูลไม่ถูกต้อง: " + err.Error(),
		})
	}

	if payload.Project == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "กรุณาระบุชื่อโปรเจกต์",
		})
	}

	if err := h.repo.SaveSystemSettings(context.Background(), payload); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "บันทึกข้อมูลล้มเหลว",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "บันทึกการตั้งค่าระบบเรียบร้อย",
	})
}

func (h *SystemHandler) GetSystem(c *fiber.Ctx) error {
	project := c.Query("project")

	if project == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "กรุณาระบุชื่อโปรเจกต์ใน query",
		})
	}

	ctx := context.Background()

	// 1. ดึงข้อมูลการตั้งค่าระบบ (จาก Firestore)
	settings, err := h.repo.GetSystemSettings(ctx, project)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "ไม่พบข้อมูลการตั้งค่าระบบของโปรเจกต์นี้",
		})
	}

	// 3. ส่งข้อมูลกลับ
	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    settings,
	})
}
