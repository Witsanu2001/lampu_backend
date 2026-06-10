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

func initFirebase() *firebase.App {
	ctx := context.Background()
	// เช็คว่ามีตัวแปรชี้ไปหาไฟล์คีย์ไหม (จะมีตอนรันในเครื่องเรา)
	credentialsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	var app *firebase.App
	var err error

	if credentialsFile != "" {
		// รันบน Local: ใช้ไฟล์ JSON
		opt := option.WithCredentialsFile(credentialsFile)
		app, err = firebase.NewApp(ctx, nil, opt)
		log.Println("Initialized Firebase with local Service Account Key")
	} else {
		// รันบน Cloud Run: ใช้สิทธิ์ของระบบอัตโนมัติ ไม่ต้องพึ่งไฟล์
		app, err = firebase.NewApp(ctx, nil)
		log.Println("Initialized Firebase with Cloud Run default credentials")
	}

	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
	}
	return app
}

func main() {
	// 1. เชื่อมต่อ Firebase
	appFirebase := initFirebase()
	ctx := context.Background()

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

	// 5. Start Server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	log.Printf("Orders Service is running on port %s", port)
	app.Listen(":" + port)
}
