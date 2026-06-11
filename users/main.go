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

	userRepo := repository.NewUserRepository(firestoreClient)
	userHandler := handlers.NewUserHandler(userRepo)

	http.HandleFunc("/api/users/sync", userHandler.SyncUserHandler)
	http.HandleFunc("/api/users/all", userHandler.GetAllUsersHandler)
	http.HandleFunc("/api/users/get_rider", userHandler.GetRiderHandler)

	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("USER_PORT")
		if port == "" {
			port = "8081"
		}
	}

	log.Printf("User Service is running on port %s", port)
	// รันเซิร์ฟเวอร์ด้วยมาตรฐาน Go (จะเรียกใช้ http.HandleFunc ด้านบน)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
