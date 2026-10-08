package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"series-tkd-management/internal/handlers"
	"series-tkd-management/internal/repository"
	"series-tkd-management/internal/storage"
)

func TestHandleServeStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "stms_serve_storage_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	localStore, err := storage.NewLocalStorage(tempDir, "/storage")
	if err != nil {
		t.Fatalf("failed to create local storage: %v", err)
	}

	// Seed test image file
	dummyImg := "image-binary-payload-data"
	uploadedURL, err := localStore.Upload(context.Background(), "avatar-user1.png", strings.NewReader(dummyImg), "image/png")
	if err != nil {
		t.Fatalf("failed to upload test file: %v", err)
	}
	if uploadedURL != "/storage/avatar-user1.png" {
		t.Errorf("unexpected uploaded url: %s", uploadedURL)
	}

	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}
	app.SetStorage(localStore)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /storage/{key...}", app.HandleServeStorage)

	// Test 1: Successful retrieval
	req := httptest.NewRequest(http.MethodGet, "/storage/avatar-user1.png", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Header().Get("Content-Type"), "image/png") {
		t.Errorf("expected Content-Type image/png, got %q", rr.Header().Get("Content-Type"))
	}
	if !strings.Contains(rr.Header().Get("Cache-Control"), "public") {
		t.Errorf("expected Cache-Control header, got %q", rr.Header().Get("Cache-Control"))
	}
	if rr.Body.String() != dummyImg {
		t.Errorf("expected body %q, got %q", dummyImg, rr.Body.String())
	}

	// Test 2: Non-existent file returns 404
	req404 := httptest.NewRequest(http.MethodGet, "/storage/nonexistent.png", nil)
	rr404 := httptest.NewRecorder()
	mux.ServeHTTP(rr404, req404)

	if rr404.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rr404.Code)
	}
}
