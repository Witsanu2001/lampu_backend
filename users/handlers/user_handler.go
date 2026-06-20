package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
	"users/models"
	"users/repository"

	"firebase.google.com/go/v4/auth"
)

type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type UserHandler struct {
	repo       *repository.UserRepository
	authClient *auth.Client
}

func NewUserHandler(repo *repository.UserRepository, authClient *auth.Client) *UserHandler {
	return &UserHandler{
		repo:       repo,
		authClient: authClient,
	}
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

	// 🎯 1. ตรวจสอบผู้ใช้เดิมใน Firestore ก่อน
	existingUser, err := h.repo.GetByID(r.Context(), user.UID)
	if err == nil && existingUser != nil {

		// 🚫 ตรวจสอบว่าโดนบล็อกอยู่หรือไม่
		if existingUser.IsBlocked {
			// ถ้าโดนบล็อก ให้เตะออกและส่ง Error กลับไปทันที
			sendJSONResponse(w, http.StatusForbidden, false, "บัญชีนี้ถูกระงับการใช้งาน", nil)
			return
		}

		// ถ้าไม่โดนบล็อก ให้ใช้ role เดิมจากฐานข้อมูล
		if existingUser.Role != "" {
			user.Role = existingUser.Role
		} else {
			user.Role = "user"
		}

		// 🌟 สำคัญ: ต้องดึงสถานะการบล็อกมาใส่ตัวแปรใหม่ด้วย เพื่อป้องกันการถูกเขียนทับตอน Save
		user.IsBlocked = existingUser.IsBlocked

	} else {
		// ถ้าเป็นผู้ใช้ใหม่เอี่ยมที่ยังไม่มีในระบบ ให้ตั้งค่าเริ่มต้น
		user.Role = "user"
		user.IsBlocked = false // ผู้ใช้ใหม่ยังไงก็ไม่โดนบล็อก
	}

	user.LastLogin = time.Now()

	if err := h.repo.Save(r.Context(), user); err != nil {
		log.Printf("Error saving to Firestore: %v", err)
		sendJSONResponse(w, http.StatusInternalServerError, false, "Failed to save user", nil)
		return
	}

	// 🌟 ส่งข้อมูล user ที่มีค่า role ถูกต้องกลับไปให้ Frontend
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

func (h *UserHandler) EditUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut { // ควรใช้ PUT หรือ PATCH สำหรับการแก้ไข
		sendJSONResponse(w, http.StatusMethodNotAllowed, false, "Method not allowed", nil)
		return
	}

	var req models.UserActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UID == "" || req.Role == "" {
		sendJSONResponse(w, http.StatusBadRequest, false, "กรุณาส่ง uid และ role ให้ครบถ้วน", nil)
		return
	}

	err := h.repo.UpdateUser(r.Context(), req.UID, req.Role)
	if err != nil {
		log.Printf("Error updating role: %v", err)
		sendJSONResponse(w, http.StatusInternalServerError, false, "แก้ไขบทบาทไม่สำเร็จ", nil)
		return
	}

	sendJSONResponse(w, http.StatusOK, true, "อัปเดตบทบาทสำเร็จ", nil)
}

func (h *UserHandler) DeleteUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete { // ใช้ DELETE
		sendJSONResponse(w, http.StatusMethodNotAllowed, false, "Method not allowed", nil)
		return
	}

	var req models.UserActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UID == "" {
		sendJSONResponse(w, http.StatusBadRequest, false, "กรุณาส่ง uid ที่ต้องการลบ", nil)
		return
	}

	err := h.repo.DeleteUser(r.Context(), req.UID)
	if err != nil {
		log.Printf("Error deleting user: %v", err)
		sendJSONResponse(w, http.StatusInternalServerError, false, "ลบผู้ใช้งานไม่สำเร็จ", nil)
		return
	}

	sendJSONResponse(w, http.StatusOK, true, "ลบผู้ใช้งานสำเร็จ", nil)
}

// 🌟 3. บล็อกผู้ใช้งาน
func (h *UserHandler) BlockUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut { // ใช้ PUT หรือ PATCH
		sendJSONResponse(w, http.StatusMethodNotAllowed, false, "Method not allowed", nil)
		return
	}

	var req models.UserActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UID == "" {
		sendJSONResponse(w, http.StatusBadRequest, false, "กรุณาส่ง uid ที่ต้องการบล็อก", nil)
		return
	}

	// 🌟 เพิ่ม h.authClient เข้าไปเป็นพารามิเตอร์เพื่อให้ Repo สั่งระงับ Token ได้
	err := h.repo.BlockUser(r.Context(), h.authClient, req.UID)
	if err != nil {
		log.Printf("Error blocking user: %v", err)
		sendJSONResponse(w, http.StatusInternalServerError, false, "บล็อกผู้ใช้งานไม่สำเร็จ", nil)
		return
	}

	sendJSONResponse(w, http.StatusOK, true, "บล็อกผู้ใช้งานเรียบร้อยแล้ว", nil)
}

func (h *UserHandler) UnblockUsersHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		sendJSONResponse(w, http.StatusMethodNotAllowed, false, "Method not allowed", nil)
		return
	}

	var req models.UserActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UID == "" {
		sendJSONResponse(w, http.StatusBadRequest, false, "กรุณาส่ง uid ที่ต้องการปลดบล็อค", nil)
		return
	}

	// 🌟 เพิ่ม h.authClient เข้าไปเป็นพารามิเตอร์เช่นเดียวกัน
	err := h.repo.UnblockUser(r.Context(), h.authClient, req.UID)
	if err != nil {
		log.Printf("Error unblocking user: %v", err)
		sendJSONResponse(w, http.StatusInternalServerError, false, "ปลดบล็อคผู้ใช้งานไม่สำเร็จ", nil)
		return
	}

	sendJSONResponse(w, http.StatusOK, true, "ปลดบล็อคผู้ใช้งานเรียบร้อยแล้ว", nil)
}

func (h *UserHandler) GetRiderHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		sendJSONResponse(w, http.StatusMethodNotAllowed, false, "Method not allowed", nil)
		return
	}

	riders, err := h.repo.GetRiders(r.Context())
	if err != nil {
		log.Printf("Error fetching riders from Firestore: %v", err)
		sendJSONResponse(w, http.StatusInternalServerError, false, "Failed to fetch riders", nil)
		return
	}

	// 🌟 เปลี่ยนให้ตรงกับชื่อ Struct ใหม่ หากไม่มีข้อมูลให้ส่ง Array เปล่า
	if riders == nil {
		riders = []models.RiderWithJobsResponse{}
	}

	// ส่งข้อมูลกลับไปแบบสวยๆ
	sendJSONResponse(w, http.StatusOK, true, "Riders retrieved successfully", riders)
}
