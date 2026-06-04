package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	firebase "firebase.google.com/go/v4"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

func initFirebase() *firebase.App {
	// ตรวจสอบว่ามีตัวแปร GOOGLE_APPLICATION_CREDENTIALS ในระบบหรือไม่
	// (เราจะตั้งค่าตัวแปรนี้เวลาเทสบนเครื่องตัวเอง)
	credentialsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")

	var app *firebase.App
	var err error

	if credentialsFile != "" {
		// รันบน Local: ใช้ไฟล์ JSON
		opt := option.WithCredentialsFile(credentialsFile)
		app, err = firebase.NewApp(context.Background(), nil, opt)
		log.Println("Initialized Firebase with local Service Account Key")
	} else {
		// รันบน Cloud Run: ใช้สิทธิ์ของระบบ (Default Service Account) อัตโนมัติ
		app, err = firebase.NewApp(context.Background(), nil)
		log.Println("Initialized Firebase with Cloud Run default credentials")
	}

	if err != nil {
		log.Fatalf("error initializing app: %v\n", err)
	}
	return app
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // หรือใส่ URL หน้าบ้านของคุณแทน "*"
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		// ถ้าเป็น OPTIONS request (การเช็คสิทธิ์ล่วงหน้าของเบราว์เซอร์) ให้ตอบตกลงไปเลย
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	}
}

// authMiddleware เป็นฟังก์ชันสกัดกั้นเพื่อตรวจสอบ Token ก่อนเข้าถึง API
// authMiddleware เป็นฟังก์ชันสกัดกั้นเพื่อตรวจสอบ Token ก่อนเข้าถึง API
func authMiddleware(app *firebase.App, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		// 🌟 1. ดึงเฉพาะ Token ออกมา โดยตัดคำว่า "Bearer " ทิ้งไป
		idToken := strings.TrimPrefix(authHeader, "Bearer ")

		// ถ้าตัดแล้วค่าที่ได้ยังเหมือนเดิม แสดงว่าไม่ได้ส่งคำว่า Bearer มาด้วย
		if idToken == authHeader || idToken == "" {
			http.Error(w, "Invalid Token format", http.StatusUnauthorized)
			return
		}

		// 🌟 2. เรียกใช้งาน Auth Client ของ Firebase
		client, err := app.Auth(context.Background())
		if err != nil {
			http.Error(w, "Error getting Auth client", http.StatusInternalServerError)
			return
		}

		// 🌟 3. ส่ง Token ไปให้ Firebase ตรวจสอบ
		token, err := client.VerifyIDToken(context.Background(), idToken)
		if err != nil {
			// พิมพ์ Error ออกทาง Log ของ Cloud Run เพื่อให้เราตามไปดูได้ว่าผิดที่อะไร
			log.Printf("Token verification failed: %v", err)
			http.Error(w, "Invalid Token", http.StatusUnauthorized)
			return
		}

		// สำเร็จ! สามารถดึง UID ของ User ไปใช้งานต่อได้
		log.Printf("Verified user: %s\n", token.UID)

		// ส่ง Request ไปยังฟังก์ชันถัดไป (เช่น ส่งต่อให้ Reverse Proxy)
		next.ServeHTTP(w, r)
	}
}

func createProxy(targetURL string) *httputil.ReverseProxy {
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Fatalf("Invalid target URL: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// 🌟 [เพิ่มใหม่] จุดสำคัญที่สุด: เปลี่ยนป้ายชื่อ Host ให้ตรงกับเป้าหมาย (Cloud Run บังคับใช้)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host // <--- บรรทัดนี้คือพระเอกที่จะแก้บั๊ก 503 ครับ!
	}

	return proxy
}

func main() {
	// 🌟 ถอยออกไป 1 โฟลเดอร์เพื่อหาไฟล์ .env (../.env)
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("No ../.env file found. Using system environment variables.")
	}

	app := initFirebase()

	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		log.Fatal("Error: USER_SERVICE_URL is not set!")
	}

	userProxy := createProxy(userServiceURL)
	router := http.NewServeMux()

	router.HandleFunc("/api/secure-data", authMiddleware(app, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, API Gateway is working!")
	}))

	router.HandleFunc("/api/users/", authMiddleware(app, func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Forwarding request to User Service: %s", r.URL.Path)
		userProxy.ServeHTTP(w, r)
	}))

	// 🌟 ระบบจัดการพอร์ต: ถ้ามี PORT จาก Cloud Run ให้ใช้ก่อน ถ้าไม่มีให้ใช้ GATEWAY_PORT จาก .env
	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("GATEWAY_PORT")
		if port == "" {
			port = "8080" // ค่าเริ่มต้นเผื่อเหนียว
		}
	}

	log.Printf("API Gateway is running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, corsMiddleware(router.ServeHTTP)))
}
