package handlers_test

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"series-tkd-management/internal/handlers"
)

func TestHTTPSRedirectMiddleware_RedirectsHTTPToHTTPS(t *testing.T) {
	innerCalled := false
	handler := handlers.HTTPSRedirectMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/students?page=2&filter=active", nil)
	req.Host = "app.seriestkd.com"
	req.Header.Set("X-Forwarded-Proto", "http")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if innerCalled {
		t.Errorf("expected inner handler not to be called on redirect")
	}

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("expected status 301 Moved Permanently, got %d", rec.Code)
	}

	expectedLocation := "https://app.seriestkd.com/students?page=2&filter=active"
	location := rec.Header().Get("Location")
	if location != expectedLocation {
		t.Errorf("expected Location %q, got %q", expectedLocation, location)
	}
}

func TestHTTPSRedirectMiddleware_PreservesMethodOnPOST(t *testing.T) {
	innerCalled := false
	handler := handlers.HTTPSRedirectMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("POST", "/api/student/check-in", nil)
	req.Host = "app.seriestkd.com"
	req.Header.Set("X-Forwarded-Proto", "http")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if innerCalled {
		t.Errorf("expected inner handler not to be called on redirect")
	}

	// For POST, must use 308 to preserve HTTP method and payload per RFC 7538
	if rec.Code != http.StatusPermanentRedirect {
		t.Fatalf("expected status 308 Permanent Redirect, got %d", rec.Code)
	}

	expectedLocation := "https://app.seriestkd.com/api/student/check-in"
	if loc := rec.Header().Get("Location"); loc != expectedLocation {
		t.Errorf("expected Location %q, got %q", expectedLocation, loc)
	}
}

func TestHTTPSRedirectMiddleware_RespectsForwardedHostAndStripsPort80(t *testing.T) {
	handler := handlers.HTTPSRedirectMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/login", nil)
	req.Host = "internal-backend:80"
	req.Header.Set("X-Forwarded-Host", "custom.seriestkd.com:80")
	req.Header.Set("X-Forwarded-Proto", "http")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMovedPermanently {
		t.Fatalf("expected status 301, got %d", rec.Code)
	}

	expectedLocation := "https://custom.seriestkd.com/login"
	if loc := rec.Header().Get("Location"); loc != expectedLocation {
		t.Errorf("expected Location %q, got %q", expectedLocation, loc)
	}
}

func TestHTTPSRedirectMiddleware_PassesThroughWhenAlreadyHTTPS(t *testing.T) {
	innerCalled := false
	handler := handlers.HTTPSRedirectMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("secure content"))
	}))

	// Case A: via X-Forwarded-Proto: https
	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.Host = "app.seriestkd.com"
	req.Header.Set("X-Forwarded-Proto", "https")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !innerCalled {
		t.Errorf("expected inner handler to be called when request is HTTPS")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", rec.Code)
	}
	if hsts := rec.Header().Get("Strict-Transport-Security"); hsts == "" {
		t.Errorf("expected Strict-Transport-Security header to be set on HTTPS responses")
	}
	if csp := rec.Header().Get("Content-Security-Policy"); csp != "upgrade-insecure-requests" {
		t.Errorf("expected Content-Security-Policy: upgrade-insecure-requests, got %q", csp)
	}
	if xcto := rec.Header().Get("X-Content-Type-Options"); xcto != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff, got %q", xcto)
	}
	if xfo := rec.Header().Get("X-Frame-Options"); xfo != "SAMEORIGIN" {
		t.Errorf("expected X-Frame-Options: SAMEORIGIN, got %q", xfo)
	}
	if ref := rec.Header().Get("Referrer-Policy"); ref != "strict-origin-when-cross-origin" {
		t.Errorf("expected Referrer-Policy: strict-origin-when-cross-origin, got %q", ref)
	}

	// Case B: direct TLS connection
	innerCalled = false
	reqTLS := httptest.NewRequest("GET", "/dashboard", nil)
	reqTLS.Host = "app.seriestkd.com"
	reqTLS.TLS = &tls.ConnectionState{}

	recTLS := httptest.NewRecorder()
	handler.ServeHTTP(recTLS, reqTLS)

	if !innerCalled {
		t.Errorf("expected inner handler to be called with direct TLS")
	}
	if recTLS.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", recTLS.Code)
	}
	if hsts := recTLS.Header().Get("Strict-Transport-Security"); hsts == "" {
		t.Errorf("expected Strict-Transport-Security header on TLS response")
	}
	if csp := recTLS.Header().Get("Content-Security-Policy"); csp != "upgrade-insecure-requests" {
		t.Errorf("expected Content-Security-Policy: upgrade-insecure-requests on TLS response, got %q", csp)
	}
}

func TestHTTPSRedirectMiddleware_AllowsLocalhostWithoutRedirect(t *testing.T) {
	innerCalled := false
	handler := handlers.HTTPSRedirectMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	hosts := []string{"localhost:8080", "127.0.0.1:8080", "[::1]:8080", "localhost"}
	for _, host := range hosts {
		innerCalled = false
		req := httptest.NewRequest("GET", "/dashboard", nil)
		req.Host = host

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if !innerCalled {
			t.Errorf("expected localhost request on %q to pass through without redirect", host)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("expected status 200 for %q, got %d", host, rec.Code)
		}
	}
}

func TestHTTPSRedirectMiddleware_HonorsDisableEnvVar(t *testing.T) {
	t.Setenv("DISABLE_HTTPS_REDIRECT", "true")

	innerCalled := false
	handler := handlers.HTTPSRedirectMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		innerCalled = true
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.Host = "app.seriestkd.com"
	req.Header.Set("X-Forwarded-Proto", "http")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !innerCalled {
		t.Errorf("expected inner handler to be called when DISABLE_HTTPS_REDIRECT=true")
	}
	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 OK, got %d", rec.Code)
	}
}
