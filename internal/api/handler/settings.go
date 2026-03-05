package handler

import (
	"encoding/json"
	"net/http"

	"github.com/tempest-bt/tempest/internal/storage"
)

type SettingsHandler struct {
	db *storage.Database
}

func NewSettingsHandler(db *storage.Database) *SettingsHandler {
	return &SettingsHandler{db: db}
}

func (h *SettingsHandler) Get(w http.ResponseWriter, r *http.Request) {
	settings, err := h.db.GetSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get settings")
		return
	}
	writeJSON(w, http.StatusOK, settings)
}

func (h *SettingsHandler) Update(w http.ResponseWriter, r *http.Request) {
	var settings storage.Settings
	if err := json.NewDecoder(r.Body).Decode(&settings); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if settings.TargetRatio <= 0 {
		settings.TargetRatio = 1.0
	}

	if err := h.db.SaveSettings(&settings); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to save settings")
		return
	}

	writeJSON(w, http.StatusOK, settings)
}
