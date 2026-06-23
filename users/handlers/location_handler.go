package handlers

import (
	"encoding/json"
	"net/http"
	"time"
	"users/models"
	"users/repository"
	"users/utils" // นำเข้า SendJSONResponse ของคุณ (อย่าลืมแก้เป็นตัวพิมพ์ใหญ่)
)

type LocationHandler struct {
	repo *repository.LocationRepository
}

func NewLocationHandler(repo *repository.LocationRepository) *LocationHandler {
	return &LocationHandler{repo: repo}
}

func (h *LocationHandler) SaveLocationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.SendJSONResponse(w, http.StatusMethodNotAllowed, false, "Method not allowed", nil)
		return
	}

	var location models.Location
	if err := json.NewDecoder(r.Body).Decode(&location); err != nil {
		utils.SendJSONResponse(w, http.StatusBadRequest, false, "Invalid request body", nil)
		return
	}

	// บังคับว่าต้องมี UserID
	if location.UserID == "" {
		utils.SendJSONResponse(w, http.StatusBadRequest, false, "user_id is required", nil)
		return
	}

	location.CreatedAt = time.Now()

	err := h.repo.SaveLocation(r.Context(), location)
	if err != nil {
		utils.SendJSONResponse(w, http.StatusInternalServerError, false, "Failed to save location", nil)
		return
	}

	utils.SendJSONResponse(w, http.StatusOK, true, "Location saved successfully", location)
}

// ฟังก์ชัน Handler สำหรับดึงรายการที่อยู่
func (h *LocationHandler) GetLocationsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.SendJSONResponse(w, http.StatusMethodNotAllowed, false, "Method not allowed", nil)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		utils.SendJSONResponse(w, http.StatusBadRequest, false, "Missing 'user_id' parameter", nil)
		return
	}

	locations, err := h.repo.GetLocationsByUserID(r.Context(), userID)
	if err != nil {
		utils.SendJSONResponse(w, http.StatusInternalServerError, false, "Failed to fetch locations", nil)
		return
	}

	// ถ้าไม่มีข้อมูล ให้ส่ง array ว่างกลับไปแทน null
	if locations == nil {
		locations = make([]*models.Location, 0)
	}

	utils.SendJSONResponse(w, http.StatusOK, true, "Locations fetched successfully", locations)
}

func (h *LocationHandler) GetLocationDefaultHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.SendJSONResponse(w, http.StatusMethodNotAllowed, false, "Method not allowed", nil)
		return
	}

	userID := r.URL.Query().Get("user_id")
	if userID == "" {
		utils.SendJSONResponse(w, http.StatusBadRequest, false, "Missing 'user_id' parameter", nil)
		return
	}

	location, err := h.repo.GetLocationDefault(r.Context(), userID)
	if err != nil {
		utils.SendJSONResponse(w, http.StatusInternalServerError, false, "Failed to fetch default location", nil)
		return
	}

	if location == nil {
		utils.SendJSONResponse(w, http.StatusNotFound, false, "Default location not found", nil)
		return
	}

	utils.SendJSONResponse(w, http.StatusOK, true, "Default location fetched successfully", location)
}

// ฟังก์ชัน Handler สำหรับการแก้ไขที่อยู่
func (h *LocationHandler) UpdateLocationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut && r.Method != http.MethodPost {
		utils.SendJSONResponse(w, http.StatusMethodNotAllowed, false, "Method not allowed", nil)
		return
	}

	var location models.Location
	if err := json.NewDecoder(r.Body).Decode(&location); err != nil {
		utils.SendJSONResponse(w, http.StatusBadRequest, false, "Invalid request body", nil)
		return
	}

	if location.ID == "" {
		utils.SendJSONResponse(w, http.StatusBadRequest, false, "Location ID is required for update", nil)
		return
	}

	location.CreatedAt = time.Now() // หรือจะดึงค่าเดิมมาใส่ก็ได้

	err := h.repo.SaveLocation(r.Context(), location)
	if err != nil {
		utils.SendJSONResponse(w, http.StatusInternalServerError, false, "Failed to update location", nil)
		return
	}

	utils.SendJSONResponse(w, http.StatusOK, true, "Location updated successfully", location)
}

// ฟังก์ชัน Handler สำหรับการลบที่อยู่
func (h *LocationHandler) DeleteLocationHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost {
		utils.SendJSONResponse(w, http.StatusMethodNotAllowed, false, "Method not allowed", nil)
		return
	}

	// ดึง ID จาก Query Parameter เช่น /api/users/location_delete?id=xxxx
	id := r.URL.Query().Get("id")
	if id == "" {
		utils.SendJSONResponse(w, http.StatusBadRequest, false, "Missing 'id' parameter", nil)
		return
	}

	err := h.repo.DeleteLocation(r.Context(), id)
	if err != nil {
		utils.SendJSONResponse(w, http.StatusInternalServerError, false, "Failed to delete location", nil)
		return
	}

	utils.SendJSONResponse(w, http.StatusOK, true, "Location deleted successfully", map[string]string{"id": id})
}
