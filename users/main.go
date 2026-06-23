package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"users/handlers"
	"users/repository"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

// 🌟 1. แปลง Middleware ให้รองรับ net/http (ไม่ต้องใช้ Fiber แล้ว)
func AuthMiddleware(appFirebase *firebase.App, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Missing token"})
			return
		}

		token := strings.TrimPrefix(authHeader, "Bearer ")
		ctx := r.Context()

		authClient, err := appFirebase.Auth(ctx)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "Auth client error"})
			return
		}

		// 🌟 เปลี่ยนมาใช้ VerifyIDTokenAndCheckRevoked
		tokenInfo, err := authClient.VerifyIDTokenAndCheckRevoked(ctx, token)
		if err != nil {
			// 🌟 เช็คว่า Error เกิดจากการที่ Token โดนสั่งยกเลิก (Revoke) ใช่หรือไม่
			if auth.IsIDTokenRevoked(err) {
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "บัญชีของคุณถูกระงับการใช้งาน หรือโดนบังคับให้ออกจากระบบ"})
				return
			}

			// ถ้าเป็น Error อื่นๆ (เช่น หมดอายุ 1 ชม. หรือ Token ผิดปกติ)
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "เซสชั่นหมดอายุ กรุณาล็อกอินใหม่"})
			return
		}

		// ตรวจสอบ Custom Claims เสริม (กรณี Token เพิ่งสร้างใหม่แต่ดึงค่าบล็อคมาด้วย)
		if isBlocked, ok := tokenInfo.Claims["is_blocked"].(bool); ok && isBlocked {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{"success": false, "message": "บัญชีของคุณถูกระงับการใช้งาน"})
			return
		}

		ctx = context.WithValue(ctx, "user_id", tokenInfo.UID)
		next(w, r.WithContext(ctx))
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

	// 🌟 3. ประกาศ ctx และเรียกใช้แอป Firebase ทีเดียว
	ctx := context.Background()
	appFirebase := initFirebase()

	// แตกออกมาเป็น Firestore Client
	firestoreClient, err := appFirebase.Firestore(ctx)
	if err != nil {
		log.Fatalf("error initializing firestore: %v", err)
	}
	defer firestoreClient.Close()
	log.Println("Firestore connected successfully")

	// แตกออกมาเป็น Auth Client
	authClient, err := appFirebase.Auth(ctx)
	if err != nil {
		log.Fatalf("error getting auth client: %v\n", err)
	}

	rtdbClient, err := appFirebase.Database(ctx) // 🌟 ต้องมีส่วนนี้ใน main.go
	if err != nil {
		log.Fatalf("error initializing rtdb: %v", err)
	}

	// สร้าง Repository
	userRepo := repository.NewUserRepository(firestoreClient, rtdbClient)
	locationRepo := repository.NewLocationRepository(firestoreClient)

	// สร้าง Handler
	userHandler := handlers.NewUserHandler(userRepo, authClient)
	locationHandler := handlers.NewLocationHandler(locationRepo)

	http.HandleFunc("/api/users/sync_to_live", AuthMiddleware(appFirebase, userHandler.SyncUserToRTDBHandler))

	http.HandleFunc("/api/users/sync", AuthMiddleware(appFirebase, userHandler.SyncUserHandler))
	http.HandleFunc("/api/users/all", AuthMiddleware(appFirebase, userHandler.GetAllUsersHandler))
	http.HandleFunc("/api/users/get_rider", AuthMiddleware(appFirebase, userHandler.GetRiderHandler))

	http.HandleFunc("/api/users/all/edit", AuthMiddleware(appFirebase, userHandler.EditUsersHandler))
	http.HandleFunc("/api/users/all/delete", AuthMiddleware(appFirebase, userHandler.DeleteUsersHandler))
	http.HandleFunc("/api/users/all/block", AuthMiddleware(appFirebase, userHandler.BlockUsersHandler))
	http.HandleFunc("/api/users/all/unblock", AuthMiddleware(appFirebase, userHandler.UnblockUsersHandler))

	// ส่วนของ Location
	http.HandleFunc("/api/users/location_add", AuthMiddleware(appFirebase, locationHandler.SaveLocationHandler))
	http.HandleFunc("/api/users/location_get", AuthMiddleware(appFirebase, locationHandler.GetLocationsHandler))
	http.HandleFunc("/api/users/location_get/default", AuthMiddleware(appFirebase, locationHandler.GetLocationDefaultHandler))
	http.HandleFunc("/api/users/location_update", AuthMiddleware(appFirebase, locationHandler.UpdateLocationHandler))
	http.HandleFunc("/api/users/location_delete", AuthMiddleware(appFirebase, locationHandler.DeleteLocationHandler))

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
