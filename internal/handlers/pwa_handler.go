package handlers

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// Embedded fallbacks ensuring zero-downtime if static assets cannot be read from disk
const defaultManifestJSON = `{
  "id": "/",
  "name": "Series Taekwondo Management System",
  "short_name": "Series TKD",
  "description": "Internal club and dojang operations, floor attendance, belt progression, and student management for Series Taekwondo.",
  "start_url": "/",
  "scope": "/",
  "display": "standalone",
  "orientation": "any",
  "background_color": "#000000",
  "theme_color": "#990303",
  "lang": "en-US",
  "dir": "ltr",
  "categories": [
    "sports",
    "productivity",
    "business"
  ],
  "icons": [
    {
      "src": "/static/icons/icon-192.png",
      "sizes": "192x192",
      "type": "image/png",
      "purpose": "any"
    },
    {
      "src": "/static/icons/icon-maskable-192.png",
      "sizes": "192x192",
      "type": "image/png",
      "purpose": "maskable"
    },
    {
      "src": "/static/icons/icon-512.png",
      "sizes": "512x512",
      "type": "image/png",
      "purpose": "any"
    },
    {
      "src": "/static/icons/icon-maskable-512.png",
      "sizes": "512x512",
      "type": "image/png",
      "purpose": "maskable"
    },
    {
      "src": "/static/icons/icon.svg",
      "sizes": "any",
      "type": "image/svg+xml",
      "purpose": "any"
    }
  ]
}`

func findStaticFile(relPath string) ([]byte, error) {
	candidates := []string{
		filepath.Join("web", "static", relPath),
		filepath.Join("..", "web", "static", relPath),
		filepath.Join("..", "..", "web", "static", relPath),
	}
	for _, c := range candidates {
		if data, err := os.ReadFile(c); err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("static file %s not found in candidates", relPath)
}

// HandleManifest serves the web app manifest specification for PWA installation.
func (a *AppHandler) HandleManifest(w http.ResponseWriter, r *http.Request) {
	data, err := findStaticFile("manifest.webmanifest")
	if err != nil {
		data = []byte(defaultManifestJSON)
	}

	w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// HandleServiceWorker serves the Service Worker script from root scope (/).
// Setting Service-Worker-Allowed: / guarantees it can control the entire application.
func (a *AppHandler) HandleServiceWorker(w http.ResponseWriter, r *http.Request) {
	data, err := findStaticFile("sw.js")
	if err != nil {
		http.Error(w, "Service Worker not available", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	w.Header().Set("Service-Worker-Allowed", "/")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// HandleOfflinePage serves the offline fallback interface when connectivity is lost.
func (a *AppHandler) HandleOfflinePage(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	data := map[string]interface{}{
		"CurrentUser": user,
		"Title":       "Offline Mode",
	}
	a.RenderPage(w, "offline.html", data)
}
