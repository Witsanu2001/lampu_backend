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
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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

		token, err := client.VerifyIDToken(context.Background(), idToken)
		if err != nil {
			log.Printf("Token verification failed: %v", err)
			http.Error(w, "Invalid Token", http.StatusUnauthorized)
			return
		}

		log.Printf("Verified user: %s\n", token.UID)
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

		resp, err := http.PostForm("https://api.line.me/oauth2/v2.1/verify", url.Values{
			"id_token":  {reqBody.IDToken},
			"client_id": {"2010209102"},
		})
		if err != nil {
			http.Error(w, "Failed to verify with LINE", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		var lineRes struct {
			Sub   string `json:"sub"`
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&lineRes)

		if lineRes.Error != "" || lineRes.Sub == "" {
			http.Error(w, "Invalid LINE Token", http.StatusUnauthorized)
			return
		}

		client, err := app.Auth(context.Background())
		if err != nil {
			http.Error(w, "Error getting Auth client", http.StatusInternalServerError)
			return
		}

		customToken, err := client.CustomToken(context.Background(), lineRes.Sub)
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

	// 🎯 1. เช็ค URL ของ Users
	userServiceURL := os.Getenv("USER_SERVICE_URL")
	if userServiceURL == "" {
		log.Fatal("Error: USER_SERVICE_URL is not set!")
	}

	// 🎯 2. เช็ค URL ของ Orders (ตัวแปรมี S ถูกต้องแล้ว)
	ordersServiceURL := os.Getenv("ORDERS_SERVICE_URL")
	if ordersServiceURL == "" {
		log.Fatal("Error: ORDERS_SERVICE_URL is not set!")
	}

	userProxy := createProxy(userServiceURL)
	orderProxy := createProxy(ordersServiceURL)
	router := http.NewServeMux()

	router.HandleFunc("/api/auth/line", corsMiddleware(lineLoginHandler(app)))

	router.HandleFunc("/api/secure-data", authMiddleware(app, func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Hello, API Gateway is working!")
	}))

	// 🎯 3. เพิ่มเส้นทาง /api/users/ ให้วิ่งไปหา User Service
	router.HandleFunc("/api/users/", authMiddleware(app, func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Forwarding request to User Service: %s", r.URL.Path)
		userProxy.ServeHTTP(w, r)
	}))

	router.HandleFunc("/api/orders/menus", authMiddleware(app, func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Rewriting and Forwarding to Order Service: %s -> /menus", r.URL.Path)
		r.URL.Path = "/menus" // สับเปลี่ยน Path ก่อนโยนให้ Proxy
		orderProxy.ServeHTTP(w, r)
	}))

	// 🎯 4. เพิ่มเส้นทาง /api/orders/ ให้วิ่งไปหา Order Service
	router.HandleFunc("/api/orders/", authMiddleware(app, func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Forwarding request to Order Service: %s", r.URL.Path)
		orderProxy.ServeHTTP(w, r)
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
