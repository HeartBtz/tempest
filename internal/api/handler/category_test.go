package handler

import (
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HeartBtz/tempest/internal/config"
	"github.com/HeartBtz/tempest/internal/engine"
	"github.com/HeartBtz/tempest/internal/storage"
)

func TestCategoryCreateDefaultsOmittedTargetRatio(t *testing.T) {
	handler, _ := newCategoryTestHandler(t)
	rr := httptest.NewRecorder()
	handler.Create(rr, httptest.NewRequest(http.MethodPost, "/api/categories", strings.NewReader(`{"name":"test"}`)))
	if rr.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body=%s", rr.Code, rr.Body.String())
	}
	var category storage.Category
	if err := json.NewDecoder(rr.Body).Decode(&category); err != nil {
		t.Fatal(err)
	}
	if category.TargetRatio != 2 {
		t.Fatalf("target ratio = %v, want 2", category.TargetRatio)
	}
}

func TestCategoryUpdatePreservesOmittedTargetRatio(t *testing.T) {
	handler, db := newCategoryTestHandler(t)
	category := &storage.Category{ID: "category", Name: "old", Color: "#000", TargetRatio: 4, CreatedAt: time.Now()}
	if err := db.CreateCategory(category); err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler.Update(rr, httptest.NewRequest(http.MethodPut, "/api/categories/category", strings.NewReader(`{"name":"new"}`)), category.ID)
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", rr.Code, rr.Body.String())
	}
	updated, err := db.GetCategory(category.ID)
	if err != nil {
		t.Fatal(err)
	}
	if updated.TargetRatio != 4 {
		t.Fatalf("target ratio = %v, want preserved value 4", updated.TargetRatio)
	}
}

func TestCategoryRequestRejectsExplicitNaNTargetRatio(t *testing.T) {
	value := math.NaN()
	if err := (&CreateCategoryRequest{TargetRatio: &value}).validate(); err == nil {
		t.Fatal("NaN target ratio was accepted")
	}
}

func newCategoryTestHandler(t *testing.T) (*CategoryHandler, *storage.Database) {
	t.Helper()
	config.Set(config.Default())
	db, err := storage.NewDatabase(t.TempDir() + "/tempest.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	manager := engine.NewManager(db)
	t.Cleanup(manager.StopAll)
	return NewCategoryHandler(db, manager), db
}
