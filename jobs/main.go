package main

import (
	"context"
	"log"
	"os"

	firebase "firebase.google.com/go/v4"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

func initFirebase() *firebase.App {
	ctx := context.Background()
	credentialsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	config := &firebase.Config{
		ProjectID:     "lampu-5a178",
		StorageBucket: "lampu-5a178.firebasestorage.app",
		DatabaseURL:   "https://lampu-5a178-default-rtdb.asia-southeast1.firebasedatabase.app",
	}

	var app *firebase.App
	var err error

	if credentialsFile != "" {
		opt := option.WithCredentialsFile(credentialsFile)
		app, err = firebase.NewApp(ctx, config, opt)
		log.Println("Initialized Firebase with local Service Account Key")
	} else {
		app, err = firebase.NewApp(ctx, config)
		log.Println("Initialized Firebase with Cloud Run default credentials")
	}

	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
	}
	return app
}

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("No ../.env file found. Using system environment variables.")
	}

	appFirebase := initFirebase()
	ctx := context.Background()

	// 1. Initialize Firestore
	_, err := appFirebase.Firestore(ctx)
	if err != nil {
		log.Fatalf("error initializing firestore: %v\n", err)
	}
	// defer firestoreClient.Close() // เตรียมไว้เปิดใช้ตอนสร้าง Repository

	// 2. Initialize Realtime Database
	_, err = appFirebase.Database(ctx)
	if err != nil {
		log.Fatalf("error initializing realtime database: %v\n", err)
	}

	app := fiber.New()
	app.Use(logger.New())

	// ✨ ตั้งค่า Route Group สำหรับ jobs
	jobsApi := app.Group("/api/jobs")

	// Route ทดสอบว่า Service รันขึ้นไหม
	jobsApi.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "Jobs service is running smoothly 🛵",
		})
	})

	// ตั้งค่า Port เป็น 8083 สำหรับ Jobs Service
	port := os.Getenv("PORT")
	if port == "" {
		port = "8083"
	}

	log.Printf("Jobs Service is running on port %s", port)
	app.Listen(":" + port)
}
