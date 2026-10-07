# Progressive Web App (PWA) & Offline Resilience Architecture

### Context: Floor Tablet PWA Operations & Network Resilience

**Problem:**
1. Coaches and front desk staff operate STMS on mobile phones and floor tablets on training mats with spotty Wi-Fi coverage or intermittent signal dropouts.
2. If service workers are served under subpaths (like `/static/sw.js`), browser security defaults restrict their scope to `/static/`, preventing the service worker from intercepting root paths (`/`, `/sessions`, `/students`) unless explicitly bypassed with headers.
3. Plain browser dropouts produce unhandled connection error pages (e.g. dinosaur crash screens), leaving coaches confused about data persistence and check-in statuses.

**Enforced Solution:**
1. **Root Scope Service Worker**:
   - `sw.js` is registered at root scope (`/sw.js`) with HTTP response headers:
     - `Content-Type: application/javascript; charset=utf-8`
     - `Service-Worker-Allowed: /`
     - `Cache-Control: no-cache, no-store, must-revalidate`
   - This ensures full control over all routes (`/sessions`, `/students`, `/portal/*`, `/dashboard`) across Chrome, Safari/iOS, Edge, and Android tablets.

2. **Dual-Tier Cache Strategy & HTMX Isolation**:
   - `sw.js` strictly isolates full page loads by testing `request.mode === 'navigate'` instead of checking `Accept: text/html` (which HTMX AJAX requests also send).
   - Authenticated HTML responses are never cached in dynamic browser caches to avoid serving stale RBAC views or caching redirected login pages.
   | Request Type | Strategy | Offline Fallback |
   | :--- | :--- | :--- |
   | Full Navigation (`request.mode === 'navigate'`) | Network | Pre-cached `/offline` branded fallback template |
   | HTMX Requests (`HX-Request: true`) | Network | Amber alert fragment warning user about offline state |
   | Static Assets (`/static/`, fonts, CSS) | Stale-While-Revalidate | Serves cached asset immediately, updates in background |
   | Mutations (`POST`, `PUT`, `DELETE`) | Bypass | Network only; avoids caching mutation responses |

3. **Offline Fallback Interface**:
   - [`web/templates/pages/offline.html`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/offline.html) provides dojang floor operational guidelines, real-time connectivity detection (`window.addEventListener('online')`), and auto-reconnection reloading.
   - Floating amber offline pill appears globally across all views when `navigator.onLine` drops or HTMX requests fail.

4. **PWA Manifest & Icon Specification**:
   - Manifest at `/manifest.webmanifest` and `/manifest.json` configured with:
     - `name`: Series Taekwondo Management System
     - `short_name`: Series TKD
     - `display`: `standalone`
     - `theme_color`: `#990303` (Series Crimson)
     - `background_color`: `#000000`
   - Icons located in [`web/static/icons/`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/icons):
     - `icon-192.png`, `icon-512.png` (Standard)
     - `icon-maskable-192.png`, `icon-maskable-512.png` (Adaptive with 20% safe zone margin)
     - `apple-touch-icon.png` (iOS Home Screen, 180x180)
     - `icon.svg`, `favicon.svg` (High-DPI vector tabs and desktop shortcuts)
