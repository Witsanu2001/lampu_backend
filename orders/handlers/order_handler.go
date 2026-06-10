package handlers

import (
	"orders/models"
	"orders/repository"

	"github.com/gofiber/fiber/v2"
)

type OrderHandler struct {
	Repo *repository.OrderRepository
}

func NewOrderHandler(repo *repository.OrderRepository) *OrderHandler {
	return &OrderHandler{Repo: repo}
}

func (h *OrderHandler) CreateOrder(c *fiber.Ctx) error {
	var order models.Order
	if err := c.BodyParser(&order); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// TODO: ดึง UserID จาก Middleware (Token) มาใส่
	// order.UserID = c.Locals("user_id").(string)

	if err := h.Repo.CreateOrder(c.Context(), &order); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to create order"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "Order created successfully",
		"data":    order,
	})
}

func (h *OrderHandler) GetMyOrders(c *fiber.Ctx) error {
	userID := c.Params("userId") // ชั่วคราว: รับจาก URL ไปก่อน (ควรรับจาก Token ในอนาคต)

	orders, err := h.Repo.GetOrdersByUserID(c.Context(), userID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch orders"})
	}

	return c.JSON(fiber.Map{
		"data": orders,
	})
}
