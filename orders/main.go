package main

import (
	"context"
	"fmt"
	"log"
	"orders/handlers"
	"orders/repository"
	"os"

	firebase "firebase.google.com/go/v4"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	"github.com/line/line-bot-sdk-go/v7/linebot"
	"google.golang.org/api/option"
)

func initFirebase() *firebase.App {
	ctx := context.Background()
	credentialsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	// สำคัญ: ต้องระบุชื่อ Bucket ของ Firebase Storage
	config := &firebase.Config{
		ProjectID:     "lampu-5a178",
		StorageBucket: "lampu-5a178.firebasestorage.app",
		DatabaseURL:   "https://lampu-5a178-default-rtdb.asia-southeast1.firebasedatabase.app",
	}

	var app *firebase.App
	var err error

	if credentialsFile != "" {
		opt := option.WithCredentialsFile(credentialsFile)
		app, err = firebase.NewApp(ctx, config, opt) // ใส่ config เข้าไป
		log.Println("Initialized Firebase with local Service Account Key")
	} else {
		app, err = firebase.NewApp(ctx, config) // ใส่ config เข้าไป
		log.Println("Initialized Firebase with Cloud Run default credentials")
	}

	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
	}
	return app
}

func SendOrderAdminNotification(adminUID string, orderDetails string) error {
	// ใส่ Token ของคุณ
	bot, err := linebot.New("b265bf9a0b64d6ff8844f1f4c8c9ce0d", "utFIjp6YeTghCpmCi5+fH65Ib8iFusrnIV3PTbhoQhMyvyU/gkYIbn6uNgg8npyN72QLI12ogPw2vFL/w6cp5Fnpzi43q6JgjIz2HW2dIM2PywIbt2ZsXLlyYKUOQBOwcW3A03L84j8WJRtcWcnIxgdB04t89/1O/w1cDnyilFU=")
	if err != nil {
		return fmt.Errorf("failed to create line bot: %v", err)
	}

	message := linebot.NewTextMessage(orderDetails)

	_, err = bot.PushMessage(adminUID, message).Do()
	if err != nil {
		return fmt.Errorf("failed to push message: %v", err)
	}

	log.Println("ส่งแจ้งเตือนเข้า LINE ร้านเรียบร้อย!")
	return nil
}

func main() {

	fmt.Println("ระบบเชื่อมต่อกับ LINE SDK เรียบร้อยแล้ว!")

	if err := godotenv.Load("../.env"); err != nil {
		log.Println("No ../.env file found. Using system environment variables.")
	}

	appFirebase := initFirebase()
	ctx := context.Background()

	// Firestore Client
	firestoreClient, err := appFirebase.Firestore(ctx)
	if err != nil {
		log.Fatalf("error initializing firestore: %v\n", err)
	}
	defer firestoreClient.Close()

	storageClient, err := appFirebase.Storage(ctx)
	if err != nil {
		log.Fatalf("error initializing storage: %v\n", err)
	}
	bucket, err := storageClient.DefaultBucket()
	if err != nil {
		log.Fatalf("error getting default bucket: %v\n", err)
	}

	// ✨ 1. เพิ่มโค้ดสร้าง Realtime Database Client ตรงนี้
	rtdbClient, err := appFirebase.Database(ctx)
	if err != nil {
		log.Fatalf("error initializing realtime database: %v\n", err)
	}

	app := fiber.New()
	app.Use(logger.New())

	ordersApi := app.Group("/api/orders")

	menuRepo := repository.NewMenuRepository(firestoreClient)
	menuHandler := handlers.NewMenuHandler(menuRepo, bucket)
	// ... (โค้ด route menu ของเดิม) ...

	// ✨ 2. ส่ง rtdbClient เข้าไปใน NewOrderRepository
	orderRepo := repository.NewOrderRepository(firestoreClient, rtdbClient)

	ordersApi.Post("/menus_add", menuHandler.CreateMenu)
	ordersApi.Get("/menus_get", menuHandler.GetAllMenus)
	ordersApi.Get("/menus_type/:type_menu", menuHandler.GetMenusByType)
	ordersApi.Put("/menus_edit/:id", menuHandler.UpdateMenu)
	ordersApi.Delete("/menus_delete/:id", menuHandler.DeleteMenu)

	bucketName := os.Getenv("FIREBASE_STORAGE_BUCKET")
	if bucketName == "" {
		bucketName = "lampu-5a178.firebasestorage.app"
	}
	orderHandler := handlers.NewOrderHandler(orderRepo, bucket, bucketName)

	ordersApi.Post("/orders_add", orderHandler.CreateOrder)
	ordersApi.Get("/orders_get", orderHandler.GetAllOrders)
	ordersApi.Get("/orders_get/:id", orderHandler.GetOrderByID)
	ordersApi.Get("/orders_get/:user_id/orderByUser", orderHandler.GetByUserID)

	ordersApi.Put("/orders_put/:id/status", orderHandler.UpdateOrderStatus)

	app.Post("/api/test-line", func(c *fiber.Ctx) error {
		type TestPayload struct {
			LineUserID string `json:"line_user_id"`
			Message    string `json:"message"`
		}

		var payload TestPayload
		if err := c.BodyParser(&payload); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
		}

		// เรียกใช้ฟังก์ชันส่ง LINE
		err := SendOrderAdminNotification(payload.LineUserID, payload.Message)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}

		return c.JSON(fiber.Map{
			"success": true,
			"message": "ส่งข้อความเข้า LINE สำเร็จ!",
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	log.Printf("Service is running on port %s", port)
	app.Listen(":" + port)
}
