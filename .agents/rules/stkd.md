---
trigger: model_decision
description: When working on core Series Taekwondo Management System (STMS) architecture, domain models, project structure, or database entities.
---

# Series Taekwondo Management System (STMS) — Architecture Overview

## 1. System Overview & Tech Stack
Series Taekwondo Management System (STMS) is an internal club and dojang operations platform built for fast, reactive, low-overhead floor management across front-desk tablets, coach mobile devices, and desktop workstations.

- **Language / Runtime**: Go 1.22+ (primary entry point at root [`main.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/main.go), secondary entry at [`cmd/server/main.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/cmd/server/main.go))
- **Routing & HTTP**: `net/http` standard library enhanced router (`http.NewServeMux()`)
- **Frontend Interactivity**: HTMX (locally vendored at `web/static/js/htmx.min.js` with `hx-boost="true"` navigation)
- **Styling**: Tailwind CSS CLI (`web/static/css/input.css` -> `web/static/css/tailwind.css`)
- **Visualizations**: Inline SVG 3-axis polygon radar (`Sparring`, `Flexibility`, `Poomsae`)
- **Database**: Hybrid `database/sql` persistence via [`internal/repository/sql_store.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/sql_store.go) supporting PostgreSQL (`DATABASE_URL` via `pgx/v5/stdlib`) in production and embedded zero-CGO SQLite (`series_tkd.db` via `modernc.org/sqlite`) in local development, plus [`internal/repository/store.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/store.go) (`MemoryStore`) for fast unit testing.
- **Object Storage**: [`internal/storage`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/storage/storage.go) supporting Railway S3 Buckets (`S3Storage` with virtual-hosted SigV4 addressing) and local disk fallback (`LocalStorage` under `web/static/uploads/avatars/`).
- **Authentication**: HTTP-only `stms_session` cookie (7-day expiry) with two-tier user model (`STUDENT` vs `STAFF` with customizable `StaffRole`s and 4-role compatibility: `OPERATION_MANAGER`, `ADMIN`, `COACH`, `STUDENT`).

---

## 2. Project Layout
```text
series-tkd-management/
├── main.go                         # Primary server bootstrapper (required at root for Railway Railpack)
├── cmd/server/main.go              # Secondary server entry point (keep route registrations synced with main.go)
├── internal/
│   ├── handlers/                   # HTTP / HTMX / JSON API handlers & middlewares
│   ├── models/                     # Pure domain structs & business invariants (DDD layer)
│   ├── repository/                 # RepositoryStore interface, SQLStore (Postgres/SQLite), MemoryStore
│   ├── services/                   # Domain services (Auth, Promotion, Payroll, Audit, Notifications)
│   └── storage/                    # FileStorage interface, Railway S3Storage, LocalStorage
├── web/
│   ├── static/                     # Precompiled tailwind.css, htmx.min.js, push-client.js, sw.js, icons
│   └── templates/
│       ├── layout.html             # Base shell (Navbar, Mobile Drawer, Bottom Bar, Modals, Tooltips)
│       ├── pages/                  # Isolated page templates cloned onto baseTmpl
│       └── partials/               # HTMX fragments (must wrap content in {{define "<filename>.html"}})
└── references/                     # Tier 3 architectural memory & historical fixes (see references/00-index.md)
```

---

## 3. Core Domain Invariants
1. **Belt Progression (PTA 12-Rank Hierarchy)**:
   - Sequential stages: `White` -> `Low Yellow` -> `High Yellow` -> `Low Blue` -> `High Blue` -> `Low Red` -> `High Red` -> `Low Brown` -> `High Brown` -> `1st Dan Black` -> `2nd Dan Black` -> `3rd Dan Black`.
   - Readiness badges: 🟢 `READY`, 🟡 `PRE-TEST ELIGIBLE`, 🔴 `DEVELOPING`.
2. **3-Axis Athletic Evaluation Radar**:
   - Triad metrics (1–10 scale): **Sparring** (Top, $-90^\circ$), **Flexibility** (Bottom-Right, $30^\circ$), **Poomsae** (Bottom-Left, $150^\circ$).
3. **Schedule & Attendance Deductions**:
   - `TrainingSession` supports `EntryType` (`class`, `open_session`, `duty`, `event`), optional `CoachID`, optional `LocationID`, and optional `SessionRate` (inherited from location `FixedRate` if set).
   - Check-in deducts 1 credit from the oldest active `student_package` *unless* the session is `open_session` or uses a fixed `session_rate`.
   - `four_week` membership plans enforce a 28-day validity and weekly cadence quota ($\max(1, \lfloor \text{Total} / 4 \rfloor)$ per 7-day rolling cycle).
4. **Tier 3 Reference Routing**:
   - Always consult [`references/00-index.md`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/references/00-index.md) before modifying routes, templates, database schemas, or UI components.
