package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/joho/godotenv"
	"google.golang.org/api/option"
)

func initFirebase() *firebase.App {
	credentialsFile := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	var app *firebase.App
	var err error

	if credentialsFile != "" {
		opt := option.WithCredentialsFile(credentialsFile)
		app, err = firebase.NewApp(context.Background(), nil, opt)
		log.Println("Initialized Firebase with local Service Account Key")
	} else {
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
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func authMiddleware(app *firebase.App, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Missing Authorization header", http.StatusUnauthorized)
			return
		}

		idToken := strings.TrimPrefix(authHeader, "Bearer ")
		if idToken == authHeader || idToken == "" {
			http.Error(w, "Invalid Token format", http.StatusUnauthorized)
			return
		}

		client, err := app.Auth(context.Background())
		if err != nil {
			http.Error(w, "Error getting Auth client", http.StatusInternalServerError)
			return
		}

		// 🌟 1. ใช้ VerifyIDTokenAndCheckRevoked เพื่อเช็คว่า Token โดนเตะ (Revoke) หรือยัง
		token, err := client.VerifyIDTokenAndCheckRevoked(context.Background(), idToken)
		if err != nil {
			// 🌟 2. ถ้า Error เพราะโดน Revoke ให้ตอบกลับเป็น 403 Forbidden ทันที
			if auth.IsIDTokenRevoked(err) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusForbidden)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"success": false,
					"message": "บัญชีของคุณถูกระงับการใช้งาน หรือโดนบังคับให้ออกจากระบบ",
				})
				return
			}

			log.Printf("Token verification failed: %v", err)
			http.Error(w, "Invalid Token", http.StatusUnauthorized)
			return
		}

		// 🌟 3. ตรวจสอบ Custom Claims เสริม (กรณี Token เพิ่งสร้างใหม่แต่ดึงค่าบล็อคมาด้วย)
		if isBlocked, ok := token.Claims["is_blocked"].(bool); ok && isBlocked {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"success": false,
				"message": "บัญชีของคุณถูกระงับการใช้งาน",
			})
			return
		}

		log.Printf("Verified user: %s\n", token.UID)

		// (Optional) แนบ UID ไปใน Header ให้ Service ด้านหลังนำไปใช้งานต่อได้ง่ายๆ
		r.Header.Set("X-User-Id", token.UID)

		next.ServeHTTP(w, r)
	}
}

func createProxy(targetURL string) *httputil.ReverseProxy {
	target, err := url.Parse(targetURL)
	if err != nil {
		log.Fatalf("Invalid target URL: %v", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host
	}
	return proxy
}

func lineLoginHandler(app *firebase.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var reqBody struct {
			IDToken string `json:"id_token"`
		}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// 🌟 1. ใส่ Channel ID ของทั้ง 2 แอป (ดอกแรก=Frontend, ดอกสอง=Rider)
		clientIDs := []string{"2010209102", "2010385468"}
		var validSub string

		// 🌟 2. วนลูปเช็คทีละแอป
		for _, clientID := range clientIDs {
			// ส่ง ID Token พร้อมกับ Client ID ไปให้ LINE ตรวจสอบ
			resp, err := http.PostForm("https://api.line.me/oauth2/v2.1/verify", url.Values{
				"id_token":  {reqBody.IDToken},
				"client_id": {clientID}, // 🚨 ต้องส่งไป ห้ามลบเด็ดขาดครับ
			})
			if err != nil {
				continue // ถ้าเน็ตเวิร์กมีปัญหา ให้ข้ามไปลองรหัสถัดไป
			}

			var lineRes struct {
				Sub   string `json:"sub"`
				Error string `json:"error"`
			}
			json.NewDecoder(resp.Body).Decode(&lineRes)
			resp.Body.Close() // ปิดการเชื่อมต่อเพื่อประหยัดทรัพยากร

			// 🌟 ถ้าไม่มี Error และมี UID (Sub) คืนมา แสดงว่ากุญแจดอกนี้ถูกต้อง!
			if lineRes.Error == "" && lineRes.Sub != "" {
				validSub = lineRes.Sub
				break // หยุดลูปทันที
			}
		}

		// 🌟 3. ถ้าลองครบทุกรหัสแล้วยังไม่ผ่าน ค่อยเตะออก (ขึ้น 401)
		if validSub == "" {
			http.Error(w, "Invalid LINE Token", http.StatusUnauthorized)
			return
		}

		client, err := app.Auth(context.Background())
		if err != nil {
			http.Error(w, "Error getting Auth client", http.StatusInternalServerError)
			return
		}

		customToken, err := client.CustomToken(context.Background(), validSub)
		if err != nil {
			log.Printf("🔥 Failed to create custom token: %v", err)
			http.Error(w, "Error creating custom token", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			"firebase_token": customToken,
		})
	}
}

func main() {
	if err := godotenv.Load("../.env"); err != nil {
		log.Println("No ../.env file found. Using system environment variables.")
	}

	app := initFirebase()

	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		log.Fatal("Error: USER_SERVICE_URL is not set!")
	}

	ordersServiceURL := os.Getenv("ORDERS_SERVICE_URL")
	if ordersServiceURL == "" {
		log.Fatal("Error: ORDERS_SERVICE_URL is not set!")
	}

	jobsServiceURL := os.Getenv("JOBS_SERVICE_URL")
	if jobsServiceURL == "" {
		log.Fatal("Error: JOBS_SERVICE_URL is not set!")
	}

	userProxy := createProxy(userServiceURL)
	orderProxy := createProxy(ordersServiceURL)
	jobProxy := createProxy(jobsServiceURL)
	router := http.NewServeMux()

	router.HandleFunc("/api/auth/line", corsMiddleware(lineLoginHandler(app)))

	router.HandleFunc("/api/secure-data", authMiddleware(app, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, API Gateway is working!")
	}))

	router.HandleFunc("/api/users/", authMiddleware(app, func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Forwarding request to User Service: %s", r.URL.Path)
		userProxy.ServeHTTP(w, r)
	}))

	router.HandleFunc("/api/orders/", authMiddleware(app, func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Forwarding request to Order Service: %s", r.URL.Path)
		orderProxy.ServeHTTP(w, r)
	}))

	router.HandleFunc("/api/jobs/", authMiddleware(app, func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Forwarding request to Job Service: %s", r.URL.Path)
		jobProxy.ServeHTTP(w, r)
	}))

	router.HandleFunc("/api/systems/", authMiddleware(app, func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Forwarding request to System Service: %s", r.URL.Path)
		jobProxy.ServeHTTP(w, r)
	}))

	port := os.Getenv("PORT")
	if port == "" {
		port = os.Getenv("GATEWAY_PORT")
		if port == "" {
			port = "8080"
		}
	}

	log.Printf("API Gateway is running on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, corsMiddleware(router.ServeHTTP)))
}
