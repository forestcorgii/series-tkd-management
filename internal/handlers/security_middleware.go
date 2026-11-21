package handlers

import (
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

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

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		first := strings.TrimSpace(strings.Split(xff, ",")[0])
		if first != "" {
			return first
		}
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil && host != "" {
		return host
	}
	return r.RemoteAddr
}
