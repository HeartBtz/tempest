package handler

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/HeartBtz/tempest/internal/engine"
	"github.com/HeartBtz/tempest/internal/storage"
)

type CategoryHandler struct {
	db      *storage.Database
	manager *engine.Manager
}

func NewCategoryHandler(db *storage.Database, manager *engine.Manager) *CategoryHandler {
	return &CategoryHandler{db: db, manager: manager}
}

func (h *CategoryHandler) List(w http.ResponseWriter, r *http.Request) {
	categories, err := h.db.ListCategories()
	if err != nil {
		log.Printf("Failed to list categories: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list categories")
		return
	}
	if categories == nil {
		categories = []storage.CategoryWithStats{}
	}
	writeJSON(w, http.StatusOK, categories)
}

type CreateCategoryRequest struct {
	Name          string   `json:"name"`
	Color         string   `json:"color"`
	UploadSpeed   int64    `json:"upload_speed"`
	DownloadSpeed int64    `json:"download_speed"`
	SpeedVariance int64    `json:"speed_variance"`
	TargetRatio   *float64 `json:"target_ratio"`
}

func (req *CreateCategoryRequest) validate() error {
	if err := validateSpeed("upload_speed", req.UploadSpeed); err != nil {
		return err
	}
	if err := validateSpeed("download_speed", req.DownloadSpeed); err != nil {
		return err
	}
	if err := validateSpeed("speed_variance", req.SpeedVariance); err != nil {
		return err
	}
	if req.TargetRatio != nil {
		return validateTargetRatio(*req.TargetRatio)
	}
	return nil
}

func (h *CategoryHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "Category name is required")
		return
	}
	if req.Color == "" {
		req.Color = "#6366f1"
	}
	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	targetRatio := 2.0
	if req.TargetRatio != nil {
		targetRatio = *req.TargetRatio
	}
	cat := &storage.Category{
		ID:            generateID(),
		Name:          strings.TrimSpace(req.Name),
		Color:         req.Color,
		UploadSpeed:   req.UploadSpeed,
		DownloadSpeed: req.DownloadSpeed,
		SpeedVariance: req.SpeedVariance,
		TargetRatio:   targetRatio,
		CreatedAt:     time.Now(),
	}

	if err := h.db.CreateCategory(cat); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeError(w, http.StatusConflict, "A category with this name already exists")
			return
		}
		log.Printf("Failed to create category: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create category")
		return
	}

	h.manager.RefreshRunnerAllocations()
	writeJSON(w, http.StatusCreated, cat)
}

func (h *CategoryHandler) HandleOne(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/categories/"), "/")
	id := parts[0]
	if id == "" {
		writeError(w, http.StatusBadRequest, "Missing category ID")
		return
	}

	if len(parts) > 1 {
		if len(parts) != 2 || parts[1] != "assign" {
			writeError(w, http.StatusNotFound, "Route not found")
			return
		}
		if r.Method != http.MethodPut {
			methodNotAllowed(w, http.MethodPut)
			return
		}
		h.Assign(w, r, id)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.Get(w, r, id)
	case http.MethodPut:
		h.Update(w, r, id)
	case http.MethodDelete:
		h.Delete(w, r, id)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPut, http.MethodDelete)
	}
}

func (h *CategoryHandler) Get(w http.ResponseWriter, r *http.Request, id string) {
	cat, err := h.db.GetCategory(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Category not found")
		return
	}
	writeJSON(w, http.StatusOK, cat)
}

func (h *CategoryHandler) Update(w http.ResponseWriter, r *http.Request, id string) {
	cat, err := h.db.GetCategory(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Category not found")
		return
	}

	var req CreateCategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if err := req.validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if strings.TrimSpace(req.Name) != "" {
		cat.Name = strings.TrimSpace(req.Name)
	}
	if req.Color != "" {
		cat.Color = req.Color
	}
	cat.UploadSpeed = req.UploadSpeed
	cat.DownloadSpeed = req.DownloadSpeed
	cat.SpeedVariance = req.SpeedVariance
	if req.TargetRatio != nil {
		cat.TargetRatio = *req.TargetRatio
	}

	if err := h.db.UpdateCategory(cat); err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "Category not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to update category")
		return
	}

	h.manager.RefreshRunnerAllocations()
	writeJSON(w, http.StatusOK, cat)
}

func (h *CategoryHandler) Delete(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.db.DeleteCategory(id); err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "Category not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to delete category")
		return
	}
	h.manager.RefreshRunnerAllocations()
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

type AssignRequest struct {
	TorrentIDs []string `json:"torrent_ids"`
	CategoryID *string  `json:"category_id"` // null to unassign
}

func (h *CategoryHandler) Assign(w http.ResponseWriter, r *http.Request, id string) {
	var req AssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if len(req.TorrentIDs) == 0 {
		writeError(w, http.StatusBadRequest, "torrent_ids must contain at least one ID")
		return
	}
	if _, err := h.db.GetCategory(id); err != nil {
		writeError(w, http.StatusNotFound, "Category not found")
		return
	}
	catID := &id
	if err := h.db.AssignTorrentsToCategory(req.TorrentIDs, catID); err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "Torrent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to assign torrents")
		return
	}
	h.manager.RefreshRunnerAllocations()
	writeJSON(w, http.StatusOK, map[string]string{"status": "assigned"})
}

// UnassignHandler handles PUT /api/categories/unassign
func (h *CategoryHandler) Unassign(w http.ResponseWriter, r *http.Request) {
	var req AssignRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}
	if len(req.TorrentIDs) == 0 {
		writeError(w, http.StatusBadRequest, "torrent_ids must contain at least one ID")
		return
	}
	if err := h.db.AssignTorrentsToCategory(req.TorrentIDs, nil); err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "Torrent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to unassign torrents")
		return
	}
	h.manager.RefreshRunnerAllocations()
	writeJSON(w, http.StatusOK, map[string]string{"status": "unassigned"})
}
