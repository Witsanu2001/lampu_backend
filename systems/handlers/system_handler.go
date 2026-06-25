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

func (h *SystemHandler) CheckPin(c *fiber.Ctx) error {
	type Request struct {
		Project string `json:"project"`
		PIN     string `json:"PIN"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid body"})
	}

	settings, err := h.repo.GetSettings(c.Context(), req.Project)
	if err != nil || settings.PIN != req.PIN {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "PIN ไม่ถูกต้อง"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true})
}

// อัปเดต PIN
func (h *SystemHandler) UpdatePin(c *fiber.Ctx) error {
	type Request struct {
		Project string `json:"project"`
		OldPIN  string `json:"oldPIN"`
		NewPIN  string `json:"newPIN"`
	}
	var req Request
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"success": false, "message": "Invalid body"})
	}

	// เช็ค PIN เดิมก่อน
	settings, err := h.repo.GetSettings(c.Context(), req.Project)
	if err != nil || settings.PIN != req.OldPIN {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "รหัส PIN เดิมไม่ถูกต้อง"})
	}

	// อัปเดต PIN ใหม่
	err = h.repo.UpdatePIN(c.Context(), req.Project, req.NewPIN)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "อัปเดตไม่สำเร็จ"})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "เปลี่ยน PIN สำเร็จ"})
}
