package main

import (
	"context"
	"log"
	"orders/handlers"
	"orders/repository"
	"os"

	firebase "firebase.google.com/go/v4"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"google.golang.org/api/option"
)

func main() {
	// 1. เชื่อมต่อ Firebase
	ctx := context.Background()
	opt := option.WithCredentialsFile("../firebase-key.json") // ชี้ไปที่ไฟล์ key ของคุณ
	appFirebase, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
	}

	client, err := appFirebase.Firestore(ctx)
	if err != nil {
		log.Fatalf("error initializing firestore: %v\n", err)
	}
	defer client.Close()

	// 2. Setup Fiber
	app := fiber.New()
	app.Use(logger.New())

	// 3. Inject Dependencies
	orderRepo := repository.NewOrderRepository(client)
	orderHandler := handlers.NewOrderHandler(orderRepo)

	// 4. Routes
	api := app.Group("/orders")
	api.Post("/", orderHandler.CreateOrder)
	api.Get("/user/:userId", orderHandler.GetMyOrders)

	// 5. Start Server (พอร์ต 8082 เพื่อไม่ให้ชนกับ Users ที่อาจจะใช้ 8081)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	log.Printf("Orders Service is running on port %s", port)
	app.Listen(":" + port)
}
