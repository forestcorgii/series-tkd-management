package handlers

import (
	"net"
	"net/http"
	"os"
	"strings"
)

// isLocalhostAddress checks whether the host is a local/loopback address.
func isLocalhostAddress(host string) bool {
	hostname := host
	if h, _, err := net.SplitHostPort(host); err == nil {
		hostname = h
	}
	hostname = strings.Trim(hostname, "[]")
	if hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1" || strings.HasSuffix(hostname, ".local") {
		return true
	}
	if ip := net.ParseIP(hostname); ip != nil && ip.IsLoopback() {
		return true
	}
	return false
}

// IsHTTPS checks if the request was made via HTTPS directly or via a trusted reverse proxy.
func IsHTTPS(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		first := strings.TrimSpace(strings.Split(proto, ",")[0])
		if strings.EqualFold(first, "https") {
			return true
		}
	}
	if strings.EqualFold(r.Header.Get("X-Forwarded-SSL"), "on") ||
		strings.EqualFold(r.Header.Get("Front-End-Https"), "on") {
		return true
	}
	return false
}

// HTTPSRedirectMiddleware inspects incoming HTTP requests and redirects them to HTTPS
// in production environments or when accessed via non-secure reverse proxies.
// Localhost / loopback requests and explicitly disabled setups are exempted.
func HTTPSRedirectMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Allow disabling via environment variable if needed
		if val := os.Getenv("DISABLE_HTTPS_REDIRECT"); strings.EqualFold(val, "true") || val == "1" {
			next.ServeHTTP(w, r)
			return
		}

		if IsHTTPS(r) {
			// Request is already HTTPS; enforce HSTS and security headers if not localhost
			if !isLocalhostAddress(r.Host) {
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains; preload")
				w.Header().Set("Content-Security-Policy", "upgrade-insecure-requests")
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.Header().Set("X-Frame-Options", "SAMEORIGIN")
				w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			}
			next.ServeHTTP(w, r)
			return
		}

		// If no proxy protocol is declared and the host is localhost, skip redirect for local development
		if r.Header.Get("X-Forwarded-Proto") == "" && isLocalhostAddress(r.Host) {
			next.ServeHTTP(w, r)
			return
		}

		// Determine target host (prefer X-Forwarded-Host if supplied by reverse proxy)
		host := r.Host
		if xfh := r.Header.Get("X-Forwarded-Host"); xfh != "" {
			first := strings.TrimSpace(strings.Split(xfh, ",")[0])
			if first != "" {
				host = first
			}
		}

		// Strip default HTTP/HTTPS ports if present
		if h, port, err := net.SplitHostPort(host); err == nil {
			if port == "80" || port == "443" {
				host = h
			}
		}

		// Compose target URL preserving path and query string
		uri := r.URL.RequestURI()
		if uri == "" || !strings.HasPrefix(uri, "/") {
			uri = r.URL.Path
			if r.URL.RawQuery != "" {
				uri += "?" + r.URL.RawQuery
			}
			if !strings.HasPrefix(uri, "/") {
				uri = "/" + uri
			}
		}

		targetURL := "https://" + host + uri

		// Use 301 Moved Permanently for safe idempotent GET/HEAD requests,
		// and 308 Permanent Redirect for other methods (POST, PUT, DELETE, PATCH)
		// to ensure the HTTP method and request body are preserved per RFC 7538.
		redirectCode := http.StatusMovedPermanently
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			redirectCode = http.StatusPermanentRedirect
		}

		http.Redirect(w, r, targetURL, redirectCode)
	})
}
