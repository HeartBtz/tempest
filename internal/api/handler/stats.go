package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/HeartBtz/tempest/internal/engine"
	"github.com/HeartBtz/tempest/internal/storage"
)

type StatsHandler struct {
	db      *storage.Database
	manager *engine.Manager
}

func NewStatsHandler(db *storage.Database, manager *engine.Manager) *StatsHandler {
	return &StatsHandler{db: db, manager: manager}
}

func (h *StatsHandler) GetGlobal(w http.ResponseWriter, r *http.Request) {
	stats, err := h.db.GetGlobalStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get stats")
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *StatsHandler) GetLogs(w http.ResponseWriter, r *http.Request) {
	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n < 1 || n > 1000 {
			writeError(w, http.StatusBadRequest, "limit must be an integer between 1 and 1000")
			return
		}
		limit = n
	}

	logs := h.manager.GetLogs(limit)
	writeJSON(w, http.StatusOK, logs)
}

func (h *StatsHandler) StreamLogs(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "Streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.manager.SubscribeLogs()
	defer h.manager.UnsubscribeLogs(ch)

	_, _ = fmt.Fprint(w, ": connected\n\n")
	flusher.Flush()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprint(w, ": keepalive\n\n")
			flusher.Flush()
		case entry, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(entry)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}
}
