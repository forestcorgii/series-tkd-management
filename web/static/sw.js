// Series Taekwondo Management System (STMS) - Progressive Web App Service Worker
const CACHE_VERSION = 'stms-v14';
const STATIC_CACHE = `${CACHE_VERSION}-static`;

const PRECACHE_ASSETS = [
    '/offline',
    '/manifest.webmanifest',
    '/static/css/tailwind.css',
    '/static/js/htmx.min.js',
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

    // 2. HTMX Partial Requests -> Network with non-200 offline signal (prevents corrupting tables/spans)
    if (request.headers.get('HX-Request') === 'true') {
        event.respondWith(
            fetch(request).catch(() => {
                return new Response('Connection lost. Operating in offline mode.', {
                    status: 503,
                    statusText: 'Service Unavailable',
                    headers: { 'Content-Type': 'text/plain; charset=utf-8' }
                });
            })
        );
        return;
    }

    // 3. Static Assets (Icons, Fonts, Scripts, Styles; exclude user-uploaded avatars) -> Stale-While-Revalidate
    const isStaticAsset =
        (url.pathname.startsWith('/static/') && !url.pathname.startsWith('/static/uploads/')) ||
        url.hostname === 'fonts.googleapis.com' ||
        url.hostname === 'fonts.gstatic.com' ||
        url.hostname === 'unpkg.com';

    if (isStaticAsset) {
        event.respondWith(
            caches.match(request, { ignoreSearch: true }).then((cachedResponse) => {
                const fetchPromise = fetch(request)
                    .then((networkResponse) => {
                        if (networkResponse && networkResponse.status === 200) {
                            const copy = networkResponse.clone();
                            caches.open(STATIC_CACHE).then((cache) => cache.put(request, copy));
                        }
                        return networkResponse;
                    })
                    .catch(() => cachedResponse || Response.error());

                return cachedResponse || fetchPromise;
            })
        );
        return;
    }

    // Default: Standard fetch with safe offline fallback
    event.respondWith(
        fetch(request).catch(() => caches.match(request, { ignoreSearch: true }).then((r) => r || Response.error()))
    );
});

// Handle update messages
self.addEventListener('message', (event) => {
    if (event.data && event.data.type === 'SKIP_WAITING') {
        self.skipWaiting();
    }
});
