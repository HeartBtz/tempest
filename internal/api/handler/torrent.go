package handler

import (
	"bytes"
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

type uploadResult struct {
	Torrent   *storage.Torrent `json:"torrent"`
	SessionID string           `json:"session_id,omitempty"`
	Error     string           `json:"error,omitempty"`
	Skipped   bool             `json:"skipped,omitempty"`
	TooLarge  bool             `json:"-"`
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

	results := make([]uploadResult, 0, len(files))

	for _, fh := range files {
		result := h.processTorrentFile(fh)
		results = append(results, result)
	}
	// If only one file, keep backward-compatible single-object response
	if len(results) == 1 {
		if results[0].Error != "" {
			status := http.StatusInternalServerError
			if results[0].TooLarge {
				status = http.StatusRequestEntityTooLarge
			} else if results[0].Skipped {
				status = http.StatusConflict
			}
			writeError(w, status, results[0].Error)
			return
		}
		writeJSON(w, http.StatusCreated, results[0].Torrent)
		return
	}

	writeJSON(w, http.StatusCreated, results)
}

func (h *TorrentHandler) processTorrentFile(fh *multipart.FileHeader) uploadResult {
	if fh.Size > maxTorrentFileSize {
		return uploadResult{Error: "Torrent file exceeds the 2 MiB limit", TooLarge: true}
	}

	f, err := fh.Open()
	if err != nil {
		return uploadResult{Error: "Failed to open file: " + err.Error()}
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxTorrentFileSize+1))
	if err != nil {
		return uploadResult{Error: "Failed to read file"}
	}
	if len(data) > maxTorrentFileSize {
		return uploadResult{Error: "Torrent file exceeds the 2 MiB limit", TooLarge: true}
	}

	tf, err := protocol.ParseTorrent(bytes.NewReader(data))
	if err != nil {
		return uploadResult{Error: "Failed to parse torrent: " + err.Error()}
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
			return uploadResult{Error: "Torrent already exists: " + tf.Name, Skipped: true}
		}
		log.Printf("Failed to save torrent: %v", err)
		return uploadResult{Error: "Failed to save torrent"}
	}

	// Auto-create and start session
	sessionID, err := h.createAndStartSession(torrent)
	if err != nil {
		log.Printf("Auto-start failed for %s: %v", torrent.Name, err)
		return uploadResult{Torrent: torrent, Error: "Torrent saved but session failed: " + err.Error()}
	}

	return uploadResult{Torrent: torrent, SessionID: sessionID}
}

func (h *TorrentHandler) createAndStartSession(torrent *storage.Torrent) (string, error) {
	settings, err := h.db.GetSettings()
	if err != nil {
		return "", fmt.Errorf("get settings: %w", err)
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
		return "", fmt.Errorf("start session: %w", err)
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
