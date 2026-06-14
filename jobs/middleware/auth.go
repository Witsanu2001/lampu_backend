package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
)

// AuthRequired เป็น Middleware สำหรับตรวจสอบและแกะ UID ออกจาก Token
func AuthRequired() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// 1. ดึง Token จาก Header "Authorization: Bearer <token>"
		authHeader := c.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Missing or invalid Authorization header",
			})
		}

		// ตัดคำว่า "Bearer " ออกให้เหลือแค่ก้อน Token
		tokenString := strings.TrimPrefix(authHeader, "Bearer ")

		// 2. แกะ Token เพื่ออ่านค่า Payload ด้านใน
		// (ใช้ ParseUnverified เพื่อให้อ่านค่าได้ทั้ง LINE และ Firebase อย่างรวดเร็ว)
		token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Invalid token format",
			})
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "Invalid token claims",
			})
		}

		// 3. ดึงค่า User ID ออกมา (LINE มักจะเก็บไว้ใน "sub", Firebase เก็บใน "user_id" หรือ "sub")
		var userID string
		if sub, ok := claims["sub"].(string); ok && sub != "" {
			userID = sub
		} else if uid, ok := claims["user_id"].(string); ok && uid != "" {
			userID = uid
		}

		if userID == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"success": false,
				"message": "User ID not found in token",
			})
		}

		// 🌟 4. สำคัญที่สุด: เอา User ID ไปฝากไว้ใน c.Locals เพื่อให้ Handler เอาไปใช้ต่อได้
		c.Locals("user_id", userID)

		// อนุญาตให้ผ่านไปทำงานที่ Handler ของ API ถัดไปได้
		return c.Next()
	}
}
