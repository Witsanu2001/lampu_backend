package main

import (
	"context"
	"jobs/handlers"
	"jobs/repository"
	"log"
	"os"

	"jobs/middleware"

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
	firestoreClient, err := appFirebase.Firestore(ctx)
	if err != nil {
		log.Fatalf("error initializing firestore: %v\n", err)
	}
	defer firestoreClient.Close()

	// 2. Initialize Realtime Database
	_, err = appFirebase.Database(ctx)
	if err != nil {
		log.Fatalf("error initializing realtime database: %v\n", err)
	}

	app := fiber.New()
	app.Use(logger.New())

	rtdbClient, err := appFirebase.Database(ctx)
	if err != nil {
		log.Fatalf("error initializing realtime database: %v\n", err)
	}

	jobsApi := app.Group("/api/jobs")

	jobRepo := repository.NewJobRepository(firestoreClient, rtdbClient)
	jobHandler := handlers.NewJobHandler(jobRepo)

	jobsApi.Get("/jobs_get", middleware.AuthRequired(), jobHandler.GetJobUser)
	jobsApi.Get("/jobs_get/:id", middleware.AuthRequired(), jobHandler.GetJobByID)
	jobsApi.Get("/jobs_history", middleware.AuthRequired(), jobHandler.GetHistory)
	jobsApi.Get("/stove", middleware.AuthRequired(), jobHandler.GetStove)
	jobsApi.Get("/stove_success", middleware.AuthRequired(), jobHandler.GetStoveSuccess)
	jobsApi.Get("/stove_rider", middleware.AuthRequired(), jobHandler.GetStoveByRiderId)

	jobsApi.Post("/stove_status", middleware.AuthRequired(), jobHandler.PostStoveStatusFalse)

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
