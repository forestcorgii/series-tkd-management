package handlers

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubFileStorage struct{}

func (s *stubFileStorage) Upload(_ context.Context, key string, _ io.Reader, _ string) (string, error) {
	return key, nil
}

func (s *stubFileStorage) Get(_ context.Context, _ string) (io.ReadCloser, string, error) {
	return io.NopCloser(strings.NewReader("ok")), "image/png", nil
}

func (s *stubFileStorage) Delete(_ context.Context, _ string) error {
	return nil
}

func TestClientIP_AntiSpoofing(t *testing.T) {
	t.Run("Untrusted proxy ignores X-Forwarded-For", func(t *testing.T) {
		t.Setenv("RAILWAY_ENVIRONMENT", "")
		t.Setenv("RAILWAY_ENVIRONMENT_NAME", "")
		t.Setenv("TRUST_PROXY", "")

		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "198.51.100.10:54321"
		req.Header.Set("X-Forwarded-For", "1.2.3.4")

		if got := clientIP(req); got != "198.51.100.10" {
			t.Fatalf("expected 198.51.100.10, got %q", got)
		}
	})

	t.Run("Trusted proxy uses rightmost valid X-Forwarded-For IP", func(t *testing.T) {
		t.Setenv("TRUST_PROXY", "true")

		req := httptest.NewRequest(http.MethodPost, "/login", nil)
		req.RemoteAddr = "10.0.0.1:12345"
		req.Header.Set("X-Forwarded-For", "9.9.9.9, 203.0.113.42")

		if got := clientIP(req); got != "203.0.113.42" {
			t.Fatalf("expected rightmost proxy IP 203.0.113.42, got %q", got)
		}
	})
}

func TestBodyLimitMiddleware(t *testing.T) {
	handler := BodyLimitMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	t.Run("Allows normal JSON payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"email":"a@b.com"}`))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
	})

	t.Run("Rejects oversized JSON payload over 1MB", func(t *testing.T) {
		oversized := bytes.Repeat([]byte("a"), defaultMaxBodyBytes+1024)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(oversized))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Fatalf("expected 413 Request Entity Too Large, got %d", rec.Code)
		}
	})
}

func TestHandleServeStorage_RejectsUnsafeKeys(t *testing.T) {
	app := &AppHandler{storage: &stubFileStorage{}}

	unsafeKeys := []string{
		"../secret.txt",
		"avatars/../../secret.txt",
		`avatars\secret.txt`,
		"avatars/test\x00.png",
		"other/file.png",
	}

	for _, key := range unsafeKeys {
		req := httptest.NewRequest(http.MethodGet, "/storage/"+strings.ReplaceAll(key, "\x00", "%00"), nil)
		req.SetPathValue("key", key)
		rec := httptest.NewRecorder()
		app.HandleServeStorage(rec, req)
		if rec.Code != http.StatusNotFound {
			t.Errorf("expected 404 for unsafe key %q, got %d", key, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/storage/avatars/test.png", nil)
	req.SetPathValue("key", "avatars/test.png")
	rec := httptest.NewRecorder()
	app.HandleServeStorage(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for valid avatar key, got %d", rec.Code)
	}
}
