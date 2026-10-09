# Performance Standards & Optimization Patterns

### Context: Core Performance Principles in STMS
STMS operates on floor tablets and mobile devices with varying network stability. Performance is treated as an operational requirement.

| Layer | Primary Bottleneck | Optimization Standard |
| :--- | :--- | :--- |
| **Database** | N+1 queries, unindexed filters | Batch queries, indexed lookups, explicit `SELECT` projections |
| **Go Backend** | Template re-parsing, heap allocations | Global pre-compiled templates, efficient slice sizing, pooled buffers |
| **HTMX / Client** | Excessive payload size, chatter | Targeted partial swaps, debounced inputs (`delay:250ms`), lean SVG polygons |

---

### Database Optimization Checklist
1. **Attendance Check-In Hot Path**:
   - Package deduction and attendance insertion must execute in a single atomic transaction.
   - Filter `student_packages` using `WHERE student_id = $1 AND remaining_sessions > 0 AND expiry_date >= CURRENT_DATE ORDER BY purchase_date ASC LIMIT 1 FOR UPDATE`.
2. **Search Autocomplete**:
   - Limit student quick-search results to 10 records.
   - Use trigram or prefix indexes on `students(full_name, phone)`.

---

### Go & Template Optimization Checklist
1. **Pre-Parsed Templates**:
   - Templates stored in a synchronized map or pre-compiled template tree during startup.
2. **Buffer Re-use**:
   - Render templates into a `bytes.Buffer` before writing to `http.ResponseWriter` to handle rendering errors cleanly without partial chunk writes.

---

### Verification Workflow
- Check query counts in test logs for every endpoint.
- Benchmark complex business domain logic (e.g., promotion readiness matrix, radar polygon calculations) using Go's standard benchmark tooling (`go test -bench=.`).

---

### Context: Page Switching Lag Elimination & HTMX Boost Strategy

* **Problem**: Switching between application pages incurred noticeable UI lag, white flashes, and repeated network overhead. Investigation identified four core bottlenecks:
  1. **Full-Page Teardowns**: Internal navigation links lacked `hx-boost="true"`, causing hard page reloads that repeatedly tore down the DOM and forced re-parsing of external web fonts and the 102KB Tailwind stylesheet on every click.
  2. **External Blocking CDN Script**: `<script src="https://unpkg.com/htmx.org@1.9.10"></script>` was requested from an unpkg CDN in `<head>`, blocking page paint on network/TLS latency.
  3. **Backend N+1 Query Loops**:
     - `/students` queried `GetUserByStudentID` in a loop for each student avatar.
     - `/packages` called `GetStudentPackages` in a loop for each student rather than using pre-aggregated packages.
     - `/locations` queried `GetSessionAttendances` in a loop for each training session rather than using batch attendances.
     - `/coaches` queried `GetUserByCoachID` and `GetUserByEmail` multiple times per coach.
  4. **Missing Database Indexes**: The `users` table lacked indexes on foreign keys `student_id` and `coach_id`.

* **Enforced Solution**:
  1. **Locally Vendored HTMX**: Bundled HTMX under `web/static/js/htmx.min.js` and added it to the PWA `PRECACHE_ASSETS` in `sw.js`.
  2. **HTMX Boost on Navigation (`hx-boost="true"`)**: Enabled `hx-boost="true"` on the root `<body>` tag so page navigation executes instant hypermedia swaps via AJAX without re-parsing stylesheets, scripts, or web fonts. Logout links explicitly set `hx-boost="false"` for clean session teardown.
  3. **Theme & Mobile Drawer Synchronization**: Hooked `syncThemeUI()` to `htmx:load` and `htmx:afterSwap`, and auto-closed the mobile drawer upon boosted navigation link clicks.
  4. **Batch User Lookups (`GetAllUsers`)**: Added `GetAllUsers()` to `RepositoryStore` and indexed user mappings by `student_id`, `coach_id`, and `email` before rendering loops in `HandleStudents`, `HandleCoaches`, and `HandleAdminPortal`.
  5. **Pre-Aggregated Package & Attendance Queries**: Swapped loop queries in `HandlePackages` to `GetAllStudentPackagesGrouped()` and in `HandleLocations` to `GetAllAttendances()`.
  6. **Foreign Key Indexes**: Added `idx_users_student_id` and `idx_users_coach_id` to database migrations.
  7. **Boosted Navigation vs. Filter Partials Invariant**: In handlers that support both full-page loads and targeted filter/search partials (`HandleStudents`, `HandleSessions`), never inspect only `HX-Request: true`. Boosted navigations send both `HX-Request: true` AND `HX-Boosted: true`. The handler must check `r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true"` before returning partial snippets to ensure boosted page navigation receives the full layout shell (navbar, header, modals).


