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
	menuHandler := handlers.NewMenuHandler(menuRepo, bucket) // ส่ง bucket เข้าไปด้วย

	menuApi := app.Group("/menus")
	menuApi.Post("/", menuHandler.CreateMenu) // API สำหรับสร้างเมนูและอัปโหลดรูป

	port := os.Getenv("PORT")
	if port == "" {
		port = "8082"
	}
	log.Printf("Service is running on port %s", port)
	app.Listen(":" + port)
}
