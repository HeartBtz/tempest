package handler

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSettingsUpdateRejectsInvalidNumericValues(t *testing.T) {
	tests := []string{
		`{"upload_speed":-1,"target_ratio":1}`,
		`{"download_speed":1099511627777,"target_ratio":1}`,
		`{"speed_variance":-1,"target_ratio":1}`,
		`{"target_ratio":0}`,
		`{"target_ratio":1000001}`,
		`{"target_ratio":1,"max_upload":-1}`,
		`{"target_ratio":1,"max_download":9007199254740992}`,
	}
	for _, body := range tests {
		rr := httptest.NewRecorder()
		(&SettingsHandler{}).Update(rr, httptest.NewRequest(http.MethodPut, "/api/settings", strings.NewReader(body)))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want 400; response=%s", body, rr.Code, rr.Body.String())
		}
	}
}

func TestCategoryCreateRejectsInvalidNumericValues(t *testing.T) {
	tests := []string{
		`{"name":"test","upload_speed":-1,"target_ratio":2}`,
		`{"name":"test","download_speed":1099511627777,"target_ratio":2}`,
		`{"name":"test","speed_variance":-1,"target_ratio":2}`,
		`{"name":"test","target_ratio":0}`,
		`{"name":"test","target_ratio":1e309}`,
	}
	for _, body := range tests {
		rr := httptest.NewRecorder()
		(&CategoryHandler{}).Create(rr, httptest.NewRequest(http.MethodPost, "/api/categories", strings.NewReader(body)))
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("body %s: status = %d, want 400; response=%s", body, rr.Code, rr.Body.String())
		}
	}
}
