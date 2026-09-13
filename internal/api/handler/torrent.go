package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/HeartBtz/tempest/internal/client"
	"github.com/HeartBtz/tempest/internal/config"
	"github.com/HeartBtz/tempest/internal/engine"
	"github.com/HeartBtz/tempest/internal/protocol"
	"github.com/HeartBtz/tempest/internal/storage"
)

const (
	maxTorrentFileSize = 2 << 20
	maxUploadBodySize  = 10 << 20
)

type TorrentHandler struct {
	db      *storage.Database
	manager *engine.Manager
}

func NewTorrentHandler(db *storage.Database, manager *engine.Manager) *TorrentHandler {
	return &TorrentHandler{db: db, manager: manager}
}

func (h *TorrentHandler) List(w http.ResponseWriter, r *http.Request) {
	torrents, err := h.db.ListTorrentsWithStats()
	if err != nil {
		log.Printf("Failed to list torrents: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to list torrents")
		return
	}
	if torrents == nil {
		torrents = []storage.TorrentWithStats{}
	}
	writeJSON(w, http.StatusOK, torrents)
}

func (h *TorrentHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/torrents/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Missing torrent ID")
		return
	}

	torrent, err := h.db.GetTorrent(id)
	if err != nil {
		writeError(w, http.StatusNotFound, "Torrent not found")
		return
	}
	writeJSON(w, http.StatusOK, torrent)
}

type UploadResult struct {
	Filename   string           `json:"filename"`
	Status     string           `json:"status"`
	Torrent    *storage.Torrent `json:"torrent,omitempty"`
	SessionID  string           `json:"session_id,omitempty"`
	Error      string           `json:"error,omitempty"`
	httpStatus int
}

type UploadResponse struct {
	Results []UploadResult `json:"results"`
}

// Create handles POST /api/torrents.
// Accepts one or more files under the "torrent" field.
// Each uploaded torrent automatically gets a session created and started.
func (h *TorrentHandler) Create(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBodySize)
	if err := r.ParseMultipartForm(maxUploadBodySize); err != nil {
		status := http.StatusBadRequest
		if strings.Contains(err.Error(), "request body too large") {
			status = http.StatusRequestEntityTooLarge
		}
		writeError(w, status, "Upload must be valid multipart data and no larger than 10 MiB")
		return
	}

	files := r.MultipartForm.File["torrent"]
	if len(files) == 0 {
		writeError(w, http.StatusBadRequest, "No torrent files provided")
		return
	}

	results := make([]UploadResult, 0, len(files))

	for _, fh := range files {
		result := h.processTorrentFile(fh)
		results = append(results, result)
	}
	writeJSON(w, uploadResponseStatus(results), UploadResponse{Results: results})
}

func (h *TorrentHandler) processTorrentFile(fh *multipart.FileHeader) UploadResult {
	result := UploadResult{Filename: fh.Filename, Status: "error", httpStatus: http.StatusBadRequest}
	if fh.Size > maxTorrentFileSize {
		result.Error = "Torrent file exceeds the 2 MiB limit"
		result.httpStatus = http.StatusRequestEntityTooLarge
		return result
	}

	f, err := fh.Open()
	if err != nil {
		result.Error = "Failed to open file"
		result.httpStatus = http.StatusInternalServerError
		return result
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxTorrentFileSize+1))
	if err != nil {
		result.Error = "Failed to read file"
		result.httpStatus = http.StatusInternalServerError
		return result
	}
	if len(data) > maxTorrentFileSize {
		result.Error = "Torrent file exceeds the 2 MiB limit"
		result.httpStatus = http.StatusRequestEntityTooLarge
		return result
	}

	tf, err := protocol.ParseTorrent(bytes.NewReader(data))
	if err != nil {
		result.Error = "Failed to parse torrent: " + err.Error()
		return result
	}

	trackersJSON, _ := json.Marshal(tf.Trackers)

	torrent := &storage.Torrent{
		ID:        generateID(),
		Name:      tf.Name,
		InfoHash:  tf.InfoHashHex,
		Size:      tf.Size,
		Trackers:  string(trackersJSON),
		Comment:   tf.Comment,
		CreatedAt: time.Now(),
	}

	if err := h.db.CreateTorrent(torrent); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			result.Status = "skipped"
			result.Error = "Torrent already exists: " + tf.Name
			result.httpStatus = http.StatusConflict
			return result
		}
		log.Printf("Failed to save torrent: %v", err)
		result.Error = "Failed to save torrent"
		result.httpStatus = http.StatusInternalServerError
		return result
	}
	result.Torrent = torrent

	// Auto-create and start session
	sessionID, err := h.createAndStartSession(torrent)
	result.SessionID = sessionID
	if err != nil {
		log.Printf("Auto-start failed for %s: %v", torrent.Name, err)
		result.Status = "saved"
		result.Error = "Torrent saved but session failed: " + err.Error()
		result.httpStatus = http.StatusMultiStatus
		return result
	}

	result.Status = "started"
	result.SessionID = sessionID
	result.httpStatus = http.StatusCreated
	return result
}

func uploadResponseStatus(results []UploadResult) int {
	if len(results) == 1 {
		return results[0].httpStatus
	}
	for _, result := range results {
		if result.Status != "started" {
			return http.StatusMultiStatus
		}
	}
	return http.StatusCreated
}

func (h *TorrentHandler) createAndStartSession(torrent *storage.Torrent) (string, error) {
	settings, err := h.db.GetSettings()
	if err != nil {
		return "", fmt.Errorf("get settings: %w", err)
	}
	if err := validateSettings(settings); err != nil {
		return "", fmt.Errorf("invalid settings: %w", err)
	}
	if err := validatePort(config.Get().Engine.DefaultAnnouncePort); err != nil {
		return "", err
	}

	profileName := settings.ClientProfile
	profile, ok := client.GetProfile(profileName)
	if !ok {
		profile = client.DefaultProfile()
		profileName = "qbittorrent-4.6.2"
	}

	session := &storage.Session{
		ID:               generateID(),
		TorrentID:        torrent.ID,
		Status:           "stopped",
		ClientProfile:    profileName,
		PeerID:           profile.GeneratePeerID(),
		Port:             config.Get().Engine.DefaultAnnouncePort,
		Key:              profile.GenerateKey(),
		Left:             torrent.Size,
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
		return "", fmt.Errorf("create session: %w", err)
	}

	if err := h.manager.StartSession(session.ID); err != nil {
		return session.ID, fmt.Errorf("start session: %w", err)
	}

	return session.ID, nil
}

func (h *TorrentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/torrents/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Missing torrent ID")
		return
	}

	sessions, err := h.db.ListSessions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to inspect torrent sessions")
		return
	}
	for _, session := range sessions {
		if session.TorrentID == id {
			if err := h.manager.StopSession(session.ID); err != nil {
				writeError(w, http.StatusInternalServerError, "Failed to stop an active torrent session")
				return
			}
		}
	}

	if err := h.db.DeleteTorrent(id); err != nil {
		if err == sql.ErrNoRows {
			writeError(w, http.StatusNotFound, "Torrent not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to delete torrent")
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Helper functions

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func methodNotAllowed(w http.ResponseWriter, methods ...string) {
	w.Header().Set("Allow", strings.Join(methods, ", "))
	writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
}

func extractID(path, prefix string) string {
	id := strings.TrimPrefix(path, prefix)
	id = strings.Split(id, "/")[0]
	return id
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
