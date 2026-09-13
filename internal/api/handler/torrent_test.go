package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/HeartBtz/tempest/internal/config"
	"github.com/HeartBtz/tempest/internal/engine"
	"github.com/HeartBtz/tempest/internal/storage"
)

func TestTorrentUploadRejectsOversizedFile(t *testing.T) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	part, err := w.CreateFormFile("torrent", "large.torrent")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := part.Write(make([]byte, maxTorrentFileSize+1)); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/torrents", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	(&TorrentHandler{}).Create(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d; body=%s", rr.Code, http.StatusRequestEntityTooLarge, rr.Body.String())
	}
	var response UploadResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 || response.Results[0].Status != "error" || response.Results[0].Error == "" {
		t.Fatalf("unexpected upload response: %+v", response)
	}
}

func TestTorrentSingleUploadReturnsStructuredStartedResult(t *testing.T) {
	config.Set(config.Default())
	db, err := storage.NewDatabase(t.TempDir() + "/tempest.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager := engine.NewManager(db)
	defer manager.StopAll()

	req := torrentUploadRequest(t, map[string][]byte{
		"valid.torrent": []byte("d4:infod6:lengthi1e4:name4:testee"),
	})
	rr := httptest.NewRecorder()
	NewTorrentHandler(db, manager).Create(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	var response UploadResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 || response.Results[0].Status != "started" ||
		response.Results[0].Torrent == nil || response.Results[0].Torrent.Name != "test" || response.Results[0].SessionID == "" {
		t.Fatalf("unexpected upload response: %+v", response)
	}
}

func TestTorrentSingleUploadFailureReturnsStructuredError(t *testing.T) {
	req := torrentUploadRequest(t, map[string][]byte{
		"invalid.torrent": []byte("not bencode"),
	})
	rr := httptest.NewRecorder()
	(&TorrentHandler{}).Create(rr, req)
	if rr.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body=%s", rr.Code, rr.Body.String())
	}
	var response UploadResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 || response.Results[0].Status != "error" || response.Results[0].Error == "" {
		t.Fatalf("unexpected error response: %+v", response)
	}
}

func TestTorrentSingleUploadReportsSavedWhenSessionDoesNotStart(t *testing.T) {
	cfg := config.Default()
	cfg.Engine.DefaultAnnouncePort = 0
	config.Set(cfg)
	db, err := storage.NewDatabase(t.TempDir() + "/tempest.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager := engine.NewManager(db)
	req := torrentUploadRequest(t, map[string][]byte{
		"valid.torrent": []byte("d4:infod6:lengthi1e4:name4:testee"),
	})
	rr := httptest.NewRecorder()
	NewTorrentHandler(db, manager).Create(rr, req)
	if rr.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207; body=%s", rr.Code, rr.Body.String())
	}
	var response UploadResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 1 || response.Results[0].Status != "saved" ||
		response.Results[0].Torrent == nil || response.Results[0].Error == "" {
		t.Fatalf("unexpected saved result: %+v", response)
	}
}

func TestTorrentUploadReportsEachPartialOutcome(t *testing.T) {
	config.Set(config.Default())
	db, err := storage.NewDatabase(t.TempDir() + "/tempest.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager := engine.NewManager(db)
	defer manager.StopAll()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for name, data := range map[string][]byte{
		"valid.torrent":   []byte("d4:infod6:lengthi1e4:name4:testee"),
		"invalid.torrent": []byte("not bencode"),
	} {
		part, err := w.CreateFormFile("torrent", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/torrents", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	rr := httptest.NewRecorder()
	NewTorrentHandler(db, manager).Create(rr, req)
	if rr.Code != http.StatusMultiStatus {
		t.Fatalf("status = %d, want 207; body=%s", rr.Code, rr.Body.String())
	}
	var response UploadResponse
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatal(err)
	}
	if len(response.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(response.Results))
	}
	statuses := make(map[string]string, len(response.Results))
	for _, result := range response.Results {
		statuses[result.Filename] = result.Status
	}
	if statuses["valid.torrent"] != "started" || statuses["invalid.torrent"] != "error" {
		t.Fatalf("unexpected outcomes: %+v", statuses)
	}
}

func torrentUploadRequest(t *testing.T, files map[string][]byte) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	for name, data := range files {
		part, err := w.CreateFormFile("torrent", name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := part.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/torrents", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req
}

func TestLogStreamDoesNotSetWildcardCORS(t *testing.T) {
	config.Set(config.Default())
	db, err := storage.NewDatabase(t.TempDir() + "/tempest.db")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	manager := engine.NewManager(db)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/ws/logs", nil).WithContext(ctx)
	rr := httptest.NewRecorder()
	NewStatsHandler(db, manager).StreamLogs(rr, req)
	if got := rr.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
	if got := rr.Body.String(); got != ": connected\n\n" {
		t.Fatalf("initial SSE payload = %q", got)
	}
}
