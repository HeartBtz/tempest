package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/tempest-bt/tempest/internal/client"
	"github.com/tempest-bt/tempest/internal/engine"
	"github.com/tempest-bt/tempest/internal/storage"
)

type SessionHandler struct {
	db      *storage.Database
	manager *engine.Manager
}

type CreateSessionRequest struct {
	TorrentID string `json:"torrent_id"`
}

type UpdateSessionRequest struct {
	UploadSpeed      *int64   `json:"upload_speed,omitempty"`
	DownloadSpeed    *int64   `json:"download_speed,omitempty"`
	SpeedVariance    *int64   `json:"speed_variance,omitempty"`
	TargetRatio      *float64 `json:"target_ratio,omitempty"`
	StopAtRatio      *bool    `json:"stop_at_ratio,omitempty"`
	MaxUpload        *int64   `json:"max_upload,omitempty"`
	MaxDownload      *int64   `json:"max_download,omitempty"`
	NetworkInterface *string  `json:"network_interface,omitempty"`
}

func NewSessionHandler(db *storage.Database, manager *engine.Manager) *SessionHandler {
	return &SessionHandler{db: db, manager: manager}
}

func (h *SessionHandler) List(w http.ResponseWriter, r *http.Request) {
	sessions, err := h.db.ListSessionsWithTorrents()
	if err != nil {
		log.Printf("Failed to list sessions: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list sessions")
		return
	}
	if sessions == nil {
		sessions = []storage.SessionWithTorrent{}
	}

	// Update running status from manager
	for i := range sessions {
		if h.manager.IsRunning(sessions[i].ID) {
			sessions[i].Status = "running"
		}
	}

	writeJSON(w, http.StatusOK, sessions)
}

func (h *SessionHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req CreateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.TorrentID == "" {
		writeError(w, http.StatusBadRequest, "torrent_id is required")
		return
	}

	// Verify torrent exists
	torrent, err := h.db.GetTorrent(req.TorrentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "Torrent not found")
		return
	}

	// Get global settings
	settings, err := h.db.GetSettings()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get settings")
		return
	}

	// Get profile
	profileName := settings.ClientProfile
	profile, ok := client.GetProfile(profileName)
	if !ok {
		profile = client.DefaultProfile()
		profileName = "qbittorrent-4.6.2"
	}

	port := 6881

	session := &storage.Session{
		ID:               generateID(),
		TorrentID:        req.TorrentID,
		Status:           "stopped",
		ClientProfile:    profileName,
		PeerID:           profile.GeneratePeerID(),
		Port:             port,
		Key:              profile.GenerateKey(),
		Uploaded:         0,
		Downloaded:       0,
		Left:             torrent.Size,
		UploadSpeed:      settings.UploadSpeed,
		DownloadSpeed:    settings.DownloadSpeed,
		SpeedVariance:    settings.SpeedVariance,
		TargetRatio:      settings.TargetRatio,
		StopAtRatio:      settings.StopAtRatio,
		MaxUpload:        settings.MaxUpload,
		MaxDownload:      settings.MaxDownload,
		NetworkInterface: settings.NetworkInterface,
		AnnounceInterval: 1800,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	if err := h.db.CreateSession(session); err != nil {
		log.Printf("Failed to create session: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to create session")
		return
	}

	writeJSON(w, http.StatusCreated, session)
}

func (h *SessionHandler) HandleSession(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	parts := strings.Split(strings.TrimPrefix(path, "/api/sessions/"), "/")
	sessionID := parts[0]

	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "Missing session ID")
		return
	}

	// Check for action
	if len(parts) > 1 {
		action := parts[1]
		switch action {
		case "start":
			h.Start(w, r, sessionID)
		case "stop":
			h.Stop(w, r, sessionID)
		default:
			writeError(w, http.StatusNotFound, "Unknown action")
		}
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.Get(w, r, sessionID)
	case http.MethodPut:
		h.Update(w, r, sessionID)
	case http.MethodDelete:
		h.Delete(w, r, sessionID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *SessionHandler) Get(w http.ResponseWriter, r *http.Request, id string) {
	session, err := h.db.GetSession(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}
	if h.manager.IsRunning(id) {
		session.Status = "running"
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *SessionHandler) Update(w http.ResponseWriter, r *http.Request, id string) {
	var req UpdateSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	session, err := h.db.GetSession(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Session not found")
		return
	}

	if req.UploadSpeed != nil {
		session.UploadSpeed = *req.UploadSpeed
	}
	if req.DownloadSpeed != nil {
		session.DownloadSpeed = *req.DownloadSpeed
	}
	if req.SpeedVariance != nil {
		session.SpeedVariance = *req.SpeedVariance
	}
	if req.TargetRatio != nil {
		session.TargetRatio = *req.TargetRatio
	}
	if req.StopAtRatio != nil {
		session.StopAtRatio = *req.StopAtRatio
	}
	if req.MaxUpload != nil {
		session.MaxUpload = *req.MaxUpload
	}
	if req.MaxDownload != nil {
		session.MaxDownload = *req.MaxDownload
	}
	if req.NetworkInterface != nil {
		session.NetworkInterface = *req.NetworkInterface
	}

	if err := h.db.UpdateSession(session); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to update session")
		return
	}

	writeJSON(w, http.StatusOK, session)
}

func (h *SessionHandler) Delete(w http.ResponseWriter, r *http.Request, id string) {
	// Stop if running
	h.manager.StopSession(id)

	if err := h.db.DeleteSession(id); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to delete session")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *SessionHandler) Start(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.manager.StartSession(id); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to start session: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (h *SessionHandler) Stop(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.manager.StopSession(id); err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to stop session: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}
