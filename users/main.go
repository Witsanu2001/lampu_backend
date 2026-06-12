package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"users/handlers"
	"users/repository"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

// Test CI/CD trigger

func initFirestore() *firestore.Client {
	ctx := context.Background()
	credentialsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	var app *firebase.App
	var err error

	if credentialsFile != "" {
		opt := option.WithCredentialsFile(credentialsFile)
		app, err = firebase.NewApp(ctx, nil, opt)
	} else {
		app, err = firebase.NewApp(ctx, nil)
	}
	if err != nil {
		log.Fatalf("error initializing app: %v", err)
	}

	client, err := app.Firestore(ctx)
	if err != nil {
		log.Fatalf("error initializing firestore: %v", err)
	}
	log.Println("Firestore connected successfully")
	return client
}

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("No ../.env file found. Using system environment variables.")
	}

	firestoreClient := initFirestore()
	defer firestoreClient.Close()

	// 🌟 1. สร้าง Repository ทั้งสองตัว
	userRepo := repository.NewUserRepository(firestoreClient)
	locationRepo := repository.NewLocationRepository(firestoreClient) // เพิ่มบรรทัดนี้

	// 🌟 2. สร้าง Handler ทั้งสองตัว
	userHandler := handlers.NewUserHandler(userRepo)
	locationHandler := handlers.NewLocationHandler(locationRepo) // เพิ่มบรรทัดนี้

	// 3. กำหนด Route ให้ User
	http.HandleFunc("/api/users/sync", userHandler.SyncUserHandler)
	http.HandleFunc("/api/users/all", userHandler.GetAllUsersHandler)
	http.HandleFunc("/api/users/get_rider", userHandler.GetRiderHandler)

	// 🌟 4. กำหนด Route ให้ Location (เรียกผ่าน locationHandler)
	http.HandleFunc("/api/users/location_add", locationHandler.SaveLocationHandler)
	http.HandleFunc("/api/users/location_update", locationHandler.UpdateLocationHandler)
	http.HandleFunc("/api/users/location_delete", locationHandler.DeleteLocationHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("USER_PORT")
		if port == "" {
			port = "8081"
		}
	}

	log.Printf("User Service is running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
