package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
	"users/models"
	"users/repository"
)

// 🌟 1. สร้าง Struct โครงสร้าง Response มาตรฐาน
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"` // omitempty แปลว่าถ้าไม่มีข้อมูล (nil) ก็ไม่ต้องแสดงฟิลด์นี้
}

type UserHandler struct {
	repo *repository.UserRepository
}

func NewUserHandler(repo *repository.UserRepository) *UserHandler {
	return &UserHandler{repo: repo}
}

// 🌟 2. ฟังก์ชันตัวช่วยสำหรับส่ง JSON ออกไป (ลดการเขียนโค้ดซ้ำซาก)
func sendJSONResponse(w http.ResponseWriter, statusCode int, success bool, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := APIResponse{
		Success: success,
		Message: message,
		Data:    data,
	}
	json.NewEncoder(w).Encode(response)
}

// ---------------------------------------------------------
// Handler Functions
// ---------------------------------------------------------

func (h *UserHandler) SyncUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		sendJSONResponse(w, http.StatusMethodNotAllowed, false, "Method not allowed", nil)
		return
	}

	var user models.UserProfile
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		sendJSONResponse(w, http.StatusBadRequest, false, "Invalid request body", nil)
		return
	}

	user.LastLogin = time.Now()

	if err := h.repo.Save(r.Context(), user); err != nil {
		log.Printf("Error saving to Firestore: %v", err)
		sendJSONResponse(w, http.StatusInternalServerError, false, "Failed to save user", nil)
		return
	}

	// 🌟 ส่งกลับเป็น Object ปกติ (data: { ... })
	sendJSONResponse(w, http.StatusOK, true, "User synced successfully", user)
}

func (h *UserHandler) GetAllUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONResponse(w, http.StatusMethodNotAllowed, false, "Method not allowed", nil)
		return
	}

	users, err := h.repo.GetAll(r.Context())
	if err != nil {
		log.Printf("Error fetching users from Firestore: %v", err)
		sendJSONResponse(w, http.StatusInternalServerError, false, "Failed to fetch users", nil)
		return
	}

	// 🌟 ส่งกลับเป็น Array (data: [ { ... }, { ... } ])
	sendJSONResponse(w, http.StatusOK, true, "Users retrieved successfully", users)
}
