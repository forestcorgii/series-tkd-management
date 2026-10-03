package handlers_test

import (
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"series-tkd-management/internal/handlers"
)

func TestGzipMiddleware_CompressesWhenAccepted(t *testing.T) {
	samplePayload := strings.Repeat("Series Taekwondo Training Session Payload ", 50)

	handler := handlers.GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Write([]byte(samplePayload))
	}))

	// Request with gzip support
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	res := rec.Result()
	if res.Header.Get("Content-Encoding") != "gzip" {
		t.Fatalf("expected Content-Encoding: gzip, got %s", res.Header.Get("Content-Encoding"))
	}
	if rec.Body.Len() >= len(samplePayload) {
		t.Errorf("expected compressed size (%d) to be smaller than original (%d)", rec.Body.Len(), len(samplePayload))
	}

	// Decompress and verify content match
	gzReader, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("failed to create gzip reader: %v", err)
	}
	decompressed, err := io.ReadAll(gzReader)
	if err != nil {
		t.Fatalf("failed to read decompressed bytes: %v", err)
	}
	if string(decompressed) != samplePayload {
		t.Errorf("decompressed payload mismatch")
	}
}

func TestGzipMiddleware_SkipsWhenNotSupported(t *testing.T) {
	samplePayload := "Plain uncompressed text"

	handler := handlers.GzipMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(samplePayload))
	}))

	req := httptest.NewRequest("GET", "/test", nil)
	// No Accept-Encoding header
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	res := rec.Result()
	if res.Header.Get("Content-Encoding") != "" {
		t.Errorf("expected no Content-Encoding header, got %s", res.Header.Get("Content-Encoding"))
	}
	if rec.Body.String() != samplePayload {
		t.Errorf("expected plain payload, got %s", rec.Body.String())
	}
}
