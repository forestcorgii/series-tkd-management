package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleManifest(t *testing.T) {
	app, _ := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/manifest.webmanifest", nil)
	rec := httptest.NewRecorder()

	app.HandleManifest(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/manifest+json") {
		t.Errorf("expected Content-Type application/manifest+json, got %s", ct)
	}

	var manifest map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &manifest); err != nil {
		t.Fatalf("manifest is not valid JSON: %v", err)
	}

	if manifest["name"] != "Series Taekwondo Management System" {
		t.Errorf("expected name 'Series Taekwondo Management System', got %v", manifest["name"])
	}

	if manifest["short_name"] != "Series TKD" {
		t.Errorf("expected short_name 'Series TKD', got %v", manifest["short_name"])
	}

	if manifest["display"] != "standalone" {
		t.Errorf("expected display 'standalone', got %v", manifest["display"])
	}

	if manifest["theme_color"] != "#990303" {
		t.Errorf("expected theme_color '#990303', got %v", manifest["theme_color"])
	}

	icons, ok := manifest["icons"].([]interface{})
	if !ok || len(icons) == 0 {
		t.Errorf("expected manifest to contain icons list, got %v", manifest["icons"])
	}
}

func TestHandleServiceWorker(t *testing.T) {
	app, _ := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/sw.js", nil)
	rec := httptest.NewRecorder()

	app.HandleServiceWorker(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/javascript") {
		t.Errorf("expected Content-Type application/javascript, got %s", ct)
	}

	allowed := rec.Header().Get("Service-Worker-Allowed")
	if allowed != "/" {
		t.Errorf("expected Service-Worker-Allowed '/', got %s", allowed)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "addEventListener") {
		t.Errorf("expected service worker body to contain addEventListener, got:\n%s", body)
	}
}

func TestHandleOfflinePage(t *testing.T) {
	app, _ := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/offline", nil)
	rec := httptest.NewRecorder()

	app.HandleOfflinePage(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	ct := rec.Header().Get("Content-Type")
	if !strings.Contains(ct, "text/html") {
		t.Errorf("expected Content-Type text/html, got %s", ct)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "DOJANG CONNECTION") || !strings.Contains(body, "OFFLINE") {
		t.Errorf("expected offline page to contain 'DOJANG CONNECTION' and 'OFFLINE', got:\n%s", body)
	}
}

func TestStaticIconsExist(t *testing.T) {
	candidates := []string{
		filepath.Join("web", "static", "icons"),
		filepath.Join("..", "web", "static", "icons"),
		filepath.Join("..", "..", "web", "static", "icons"),
	}
	var iconsDir string
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			iconsDir = c
			break
		}
	}
	if iconsDir == "" {
		t.Fatal("web/static/icons directory not found in candidates")
	}

	expectedFiles := []string{
		"icon-192.png",
		"icon-512.png",
		"icon-maskable-192.png",
		"icon-maskable-512.png",
		"apple-touch-icon.png",
		"icon.svg",
		"favicon.svg",
	}

	for _, ef := range expectedFiles {
		fp := filepath.Join(iconsDir, ef)
		info, err := os.Stat(fp)
		if err != nil {
			t.Errorf("missing expected icon file %s: %v", ef, err)
		} else if info.Size() == 0 {
			t.Errorf("icon file %s is empty", ef)
		}
	}
}
