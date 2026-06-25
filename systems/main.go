package main

import (
	"context"
	"log"
	"os"
	"strings"
	"systems/handlers"
	"systems/repository"

	firebase "firebase.google.com/go/v4"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

func AuthMiddleware(appFirebase *firebase.App, sysRepo *repository.SystemRepository) fiber.Handler {
	return func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Missing token"})
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		ctx := context.Background()

		authClient, err := appFirebase.Auth(ctx)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Auth client error"})
		}

		tokenInfo, err := authClient.VerifyIDToken(ctx, token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"success": false, "message": "Invalid token"})
		}

		role, err := sysRepo.GetUserRole(ctx, tokenInfo.UID)
		if err != nil || role != "admin" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"success": false, "message": "Requires admin role"})
		}

		c.Locals("user_id", tokenInfo.UID)
		return c.Next()
	}
}

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
	rtdbClient, err := appFirebase.Database(ctx)
	if err != nil {
		log.Fatalf("error initializing realtime database: %v\n", err)
	}

	app := fiber.New()
	app.Use(logger.New())

	SystemsApi := app.Group("/api/systems")

	sysRepo := repository.NewSystemRepository(firestoreClient, rtdbClient)
	sysHandler := handlers.NewSystemHandler(sysRepo)

	SystemsApi.Get("/systems", AuthMiddleware(appFirebase, sysRepo), sysHandler.GetSystem)
	SystemsApi.Post("/systems_add", AuthMiddleware(appFirebase, sysRepo), sysHandler.AddSystem)

	SystemsApi.Post("/check_pin", AuthMiddleware(appFirebase, sysRepo), sysHandler.CheckPin)
	SystemsApi.Post("/update_pin", AuthMiddleware(appFirebase, sysRepo), sysHandler.UpdatePin)

	SystemsApi.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status":  "ok",
			"message": "Systems service is running smoothly 🛵",
		})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8084"
	}
	log.Printf("Service is running on port %s", port)
	if err := app.Listen(":" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
