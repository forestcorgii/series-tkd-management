# Deployment & Build Configuration

### Context: Railway & Nixpacks mise Go Version Resolution

- **Problem**: Railway uses Nixpacks/mise to provision the build environment. mise deprecated relying solely on the minimum `go` directive in `go.mod` (`mise WARN deprecated [idiomatic.go.mod.go-directive]`), requiring explicit toolchain configuration before version 2026.11.0. Adding `toolchain` to `go.mod` can get stripped by `go mod tidy` if the local compiler is newer.
- **Enforced Solution**:
  - Keep a [`.go-version`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/.go-version) and [`mise.toml`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/mise.toml) (`[tools] go = "1.22.0"`) at the project root.
  - This informs `mise` / `nixpacks` directly without triggering deprecation warnings or conflicting with standard `go mod tidy` operations.

### Context: Railway Railpack / Nixpacks "no Go files in /app" Build Failure

- **Problem**: Railway now uses **Railpack** (BuildKit-based successor to Nixpacks). Railpack executes `go build -ldflags=-w -s -o out` directly inside `/app` (the root directory) and ignores `nixpacks.toml`. If Go source files only reside in subpackages like `cmd/server/main.go`, BuildKit fails with `no Go files in /app`.
- **Enforced Solution**:
  - Keep [`main.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/main.go) at the repository root as the primary entry point.
  - This allows Railpack, Nixpacks, Dockerfiles, and standard Go build tools to build cleanly from root (`/app`) with zero configuration (`go build -ldflags="-w -s" -o out`).
  - Maintain [`nixpacks.toml`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/nixpacks.toml) and [`mise.toml`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/mise.toml) for backward compatibility across Nixpacks and mise environments.

### Context: Tailwind CSS Production Compilation & CLI Workflow

- **Problem**:
  - The Tailwind Play CDN (`<script src="https://cdn.tailwindcss.com"></script>`) is development-only, triggers browser console warnings in production (`cdn.tailwindcss.com should not be used in production`), prevents offline CSS caching in the Service Worker, and incurs client-side runtime compilation overhead on every page load.
- **Enforced Solution**:
  - **Toolchain**: Use Tailwind CLI via `package.json` scripts:
    - Build: `npm run build:css` (`tailwindcss -i ./web/static/css/input.css -o ./web/static/css/tailwind.css --minify`)
    - Watch: `npm run watch:css` (`tailwindcss -i ./web/static/css/input.css -o ./web/static/css/tailwind.css --watch`)
  - **Configuration**: Root [`tailwind.config.js`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/tailwind.config.js) scanning `./web/templates/**/*.html`, `./web/static/**/*.js`, and `./internal/**/*.go`.
  - **Styles**: Base styles, `@layer components`, and STMS design tokens reside in [`web/static/css/input.css`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/css/input.css).
  - **Distribution**: Precompiled minified output is committed at [`web/static/css/tailwind.css`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/css/tailwind.css) and served via HTTP with `Cache-Control: public, max-age=86400`.
  - **Layout & PWA**: [`web/templates/layout.html`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html) links directly to `/static/css/tailwind.css`, and [`web/static/sw.js`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/sw.js) precaches the stylesheet under `PRECACHE_ASSETS`.

