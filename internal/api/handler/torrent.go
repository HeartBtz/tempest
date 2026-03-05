package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/tempest-bt/tempest/internal/protocol"
	"github.com/tempest-bt/tempest/internal/storage"
)

type TorrentHandler struct {
	db *storage.Database
}

func NewTorrentHandler(db *storage.Database) *TorrentHandler {
	return &TorrentHandler{db: db}
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

func (h *TorrentHandler) Create(w http.ResponseWriter, r *http.Request) {
	// Parse multipart form (max 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "Failed to parse form: "+err.Error())
		return
	}

	file, header, err := r.FormFile("torrent")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Missing torrent file: "+err.Error())
		return
	}
	defer file.Close()

	// Read file content
	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to read file")
		return
	}

	// Parse torrent
	tf, err := protocol.ParseTorrent(bytes.NewReader(data))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Failed to parse torrent: "+err.Error())
		return
	}

	// Convert trackers to JSON
	trackersJSON, _ := json.Marshal(tf.Trackers)

	torrent := &storage.Torrent{
		ID:        generateID(),
		Name:      tf.Name,
		InfoHash:  tf.InfoHashHex,
		Size:      tf.Size,
		Trackers:  string(trackersJSON),
		Comment:   tf.Comment,
		FilePath:  header.Filename,
		CreatedAt: time.Now(),
	}

	if err := h.db.CreateTorrent(torrent); err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			writeError(w, http.StatusConflict, "Torrent with this info hash already exists")
			return
		}
		log.Printf("Failed to save torrent: %v", err)
		writeError(w, http.StatusInternalServerError, "Failed to save torrent")
		return
	}

	writeJSON(w, http.StatusCreated, torrent)
}

func (h *TorrentHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := extractID(r.URL.Path, "/api/torrents/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "Missing torrent ID")
		return
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

func extractID(path, prefix string) string {
	id := strings.TrimPrefix(path, prefix)
	id = strings.Split(id, "/")[0]
	return id
}

func generateID() string {
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
