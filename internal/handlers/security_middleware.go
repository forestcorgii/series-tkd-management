package handlers

import (
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const defaultMaxBodyBytes = 1 << 20 // 1 MB

// BodyLimitMiddleware caps non-multipart request bodies to prevent memory-exhaustion DoS.
// Multipart requests (e.g. /profile avatar uploads) enforce their own tighter limits.
func BodyLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Body != nil && r.Method != http.MethodGet && r.Method != http.MethodHead {
			ct := strings.ToLower(r.Header.Get("Content-Type"))
			if !strings.HasPrefix(ct, "multipart/form-data") {
				r.Body = http.MaxBytesReader(w, r.Body, defaultMaxBodyBytes)
			}
		}
		next.ServeHTTP(w, r)
	})
}

// CSRFMiddleware validates Origin/Referer and Sec-Fetch-Site headers on state-changing
// cookie-authenticated requests to block cross-site request forgery.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			// Only enforce when a session cookie is attached
			if _, err := r.Cookie(sessionCookieKey); err == nil {
				if strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site") {
					http.Error(w, "Cross-site request blocked", http.StatusForbidden)
					return
				}
				if origin := r.Header.Get("Origin"); origin != "" {
					if u, err := url.Parse(origin); err != nil || !strings.EqualFold(u.Host, r.Host) {
						http.Error(w, "Invalid request origin", http.StatusForbidden)
						return
					}
				} else if referer := r.Header.Get("Referer"); referer != "" {
					if u, err := url.Parse(referer); err != nil || (u.Host != "" && !strings.EqualFold(u.Host, r.Host)) {
						http.Error(w, "Invalid request referer", http.StatusForbidden)
						return
					}
				}
			}
		}
		next.ServeHTTP(w, r)
	})
}

type rateBucket struct {
	count   int
	resetAt time.Time
}

// RateLimitMiddleware applies a per-IP request limit on sensitive auth and public reservation endpoints.
func RateLimitMiddleware(next http.Handler) http.Handler {
	var mu sync.Mutex
	buckets := make(map[string]*rateBucket)
	lastCleanup := time.Now()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || !isRateLimitedPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}

		ip := clientIP(r)
		now := time.Now()

		mu.Lock()
		if now.Sub(lastCleanup) > 5*time.Minute {
			for k, b := range buckets {
				if now.After(b.resetAt) {
					delete(buckets, k)
				}
			}
			lastCleanup = now
		}

		b, ok := buckets[ip]
		if !ok || now.After(b.resetAt) {
			b = &rateBucket{count: 0, resetAt: now.Add(time.Minute)}
			buckets[ip] = b
		}
		b.count++
		allowed := b.count <= 30
		mu.Unlock()

		if !allowed {
			w.Header().Set("Retry-After", "60")
			http.Error(w, "Too many requests. Please wait a minute and try again.", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func isRateLimitedPath(path string) bool {
	switch path {
	case "/login", "/api/auth/login",
		"/forgot-password", "/api/auth/forgot-password",
		"/reset-password", "/api/auth/reset-password",
		"/register", "/registration", "/register/student",
		"/landing/reserve":
		return true
	}
	return strings.HasPrefix(path, "/reserve/")
}

func isTrustedProxyEnv() bool {
	if os.Getenv("RAILWAY_ENVIRONMENT") != "" || os.Getenv("RAILWAY_ENVIRONMENT_NAME") != "" {
		return true
	}
	val := strings.TrimSpace(os.Getenv("TRUST_PROXY"))
	return strings.EqualFold(val, "true") || val == "1"
}

func clientIP(r *http.Request) string {
	if isTrustedProxyEnv() {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			for i := len(parts) - 1; i >= 0; i-- {
				candidate := strings.TrimSpace(parts[i])
				if candidate != "" && net.ParseIP(candidate) != nil {
					return candidate
				}
			}
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
