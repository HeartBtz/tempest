package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/tempest-bt/tempest/internal/engine"
	"github.com/tempest-bt/tempest/internal/storage"
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
		if n, err := strconv.Atoi(l); err == nil {
			limit = n
		}
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
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := h.manager.SubscribeLogs()
	defer h.manager.UnsubscribeLogs(ch)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
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
