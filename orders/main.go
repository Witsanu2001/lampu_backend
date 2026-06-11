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
	credentialsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	// สำคัญ: ต้องระบุชื่อ Bucket ของ Firebase Storage
	config := &firebase.Config{
		StorageBucket: "lampu-5a178.firebasestorage.app", // เปลี่ยนเป็นชื่อ Bucket ของคุณ
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

func main() {
	appFirebase := initFirebase()
	ctx := context.Background()

	// Firestore Client
	firestoreClient, err := appFirebase.Firestore(ctx)
	if err != nil {
		log.Fatalf("error initializing firestore: %v\n", err)
	}
	defer firestoreClient.Close()

	// Storage Client (เพิ่มเข้ามาใหม่)
	storageClient, err := appFirebase.Storage(ctx)
	if err != nil {
		log.Fatalf("error initializing storage: %v\n", err)
	}
	bucket, err := storageClient.DefaultBucket()
	if err != nil {
		log.Fatalf("error getting default bucket: %v\n", err)
	}

	app := fiber.New()
	app.Use(logger.New())

	// ---- Menus (ส่วนที่เพิ่มเข้ามาใหม่) ----
	menuRepo := repository.NewMenuRepository(firestoreClient)
	menuHandler := handlers.NewMenuHandler(menuRepo, bucket)

	menuApi := app.Group("/api/orders")
	menuApi.Post("/menus_add", menuHandler.CreateMenu)
	menuApi.Get("/menus_get", menuHandler.GetAllMenus)
	menuApi.Get("/menus_type/:type_menu", menuHandler.GetMenusByType)
	menuApi.Put("/menus_edit/:id", menuHandler.UpdateMenu)
	menuApi.Delete("/menus_delete/:id", menuHandler.DeleteMenu)

	orderRepo := repository.NewOrderRepository(firestoreClient)
	bucketName := os.Getenv("FIREBASE_STORAGE_BUCKET")
	if bucketName == "" {
		bucketName = "lampu-5a178.firebasestorage.app"
	}
	orderHandler := handlers.NewOrderHandler(orderRepo, bucket, bucketName)

	menuApi.Post("/orders_add", orderHandler.CreateOrder)
	menuApi.Get("/orders_get", orderHandler.GetAllOrders)
	menuApi.Get("/orders_get/:id", orderHandler.GetOrderByID)
	menuApi.Get("/orders_get/:user_id/orderByUser", orderHandler.GetByUserID)
	// menuApi.Get("/orders_type/:type_order", orderHandler.GetOrdersByType)
	// menuApi.Put("/orders_edit/:id", orderHandler.UpdateOrder)
	// menuApi.Delete("/orders_delete/:id", orderHandler.DeleteOrder)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	log.Printf("Service is running on port %s", port)
	app.Listen(":" + port)
}
