// Series Taekwondo Management System (STMS) - Progressive Web App Service Worker
const CACHE_VERSION = 'stms-v2';
const STATIC_CACHE = `${CACHE_VERSION}-static`;

const PRECACHE_ASSETS = [
    '/offline',
    '/manifest.webmanifest',
    '/static/icons/icon-192.png',
    '/static/icons/icon-512.png',
    '/static/icons/icon-maskable-192.png',
    '/static/icons/icon-maskable-512.png',
    '/static/icons/icon.svg',
    '/static/icons/favicon.svg',
    '/static/icons/apple-touch-icon.png'
];

// Install: Cache offline fallback and critical static assets
self.addEventListener('install', (event) => {
    event.waitUntil(
        caches.open(STATIC_CACHE).then((cache) => {
            return cache.addAll(PRECACHE_ASSETS).catch((err) => {
                console.warn('STMS PWA: Failed to precache some assets:', err);
            });
        }).then(() => self.skipWaiting())
    );
});

// Activate: Clean up outdated caches and claim clients immediately
self.addEventListener('activate', (event) => {
    event.waitUntil(
        caches.keys().then((keys) => {
            return Promise.all(
                keys.map((key) => {
                    if (key !== STATIC_CACHE) {
                        return caches.delete(key);
                    }
                })
            );
        }).then(() => self.clients.claim())
    );
});

// Fetch Handler
self.addEventListener('fetch', (event) => {
    const { request } = event;
    const url = new URL(request.url);

    // Bypass caching for non-GET requests (mutations)
    if (request.method !== 'GET') {
        return;
    }

    // 1. Navigation requests (Full Page Load) -> Network First with Offline Fallback
    // Strictly check request.mode === 'navigate' so HTMX AJAX partials are not confused with full pages
    if (request.mode === 'navigate') {
        event.respondWith(
            fetch(request).catch(async () => {
                const offlinePage = await caches.match('/offline');
                if (offlinePage) {
                    return offlinePage;
                }
                return new Response(
                    `<!DOCTYPE html><html><body style="font-family:sans-serif;text-align:center;padding:50px;">
                    <h1>Series TKD Offline</h1><p>Network connection unavailable. Please check your signal.</p>
                    </body></html>`,
                    { headers: { 'Content-Type': 'text/html; charset=utf-8' } }
                );
            })
        );
        return;
    }

    // 2. HTMX Partial Requests -> Network with Offline Alert Fallback
    if (request.headers.get('HX-Request') === 'true') {
        event.respondWith(
            fetch(request).catch(async () => {
                return new Response(
                    `<div class="p-4 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-700 dark:text-amber-400 text-xs sm:text-sm font-semibold flex items-center gap-2">
                        <svg class="w-5 h-5 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                        </svg>
                        <span>Connection lost. Operating in offline mode. Please reconnect floor device.</span>
                    </div>`,
                    { headers: { 'Content-Type': 'text/html; charset=utf-8' } }
                );
            })
        );
        return;
    }

    // 3. Static Assets (Icons, Fonts, Scripts, Styles) -> Stale-While-Revalidate
    const isStaticAsset =
        url.pathname.startsWith('/static/') ||
        url.hostname.includes('fonts.googleapis.com') ||
        url.hostname.includes('fonts.gstatic.com') ||
        url.hostname.includes('cdn.tailwindcss.com') ||
        url.hostname.includes('unpkg.com');

    if (isStaticAsset) {
        event.respondWith(
            caches.match(request).then((cachedResponse) => {
                const fetchPromise = fetch(request)
                    .then((networkResponse) => {
                        if (networkResponse && networkResponse.status === 200) {
                            const copy = networkResponse.clone();
                            caches.open(STATIC_CACHE).then((cache) => cache.put(request, copy));
                        }
                        return networkResponse;
                    })
                    .catch(() => cachedResponse);

                return cachedResponse || fetchPromise;
            })
        );
        return;
    }

    // Default: Standard fetch
    event.respondWith(
        fetch(request).catch(() => caches.match(request))
    );
});

// Handle update messages
self.addEventListener('message', (event) => {
    if (event.data && event.data.type === 'SKIP_WAITING') {
        self.skipWaiting();
    }
});
