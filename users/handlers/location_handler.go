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
