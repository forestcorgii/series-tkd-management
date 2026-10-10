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

---

### Context: Dynamic Modal Forms & HTMX Boost Invariant

* **Problem**: When `hx-boost="true"` is active on `<body>` for fast SPA-style navigation, HTMX automatically boosts all `<form>` elements during DOM initialization. For modal forms initialized with `action=""` (where the destination URL like `/sessions/{id}/edit` is populated dynamically via JavaScript upon clicking "Edit"), HTMX captures `action=""` into its event listener closure at page load. Upon submit, HTMX falls back to the current page URL (e.g. `POST /sessions`), invoking the creation endpoint (`HandleCreateSession`) instead of the update endpoint (`HandleUpdateSession`), causing duplicate entities to be created when saving edits.
* **Enforced Solution**:
  1. **Exempt Dynamic Modal Forms (`hx-boost="false"`)**: Always add `hx-boost="false"` to any `<form>` whose `action` attribute is set dynamically via JavaScript upon opening (e.g., `#edit-session-form`, `#delete-session-form`, `#cancel-session-form`, `#reschedule-session-form`, `#edit-location-form`, `#edit-plan-form`, `#edit-cat-form`).
  2. **Synchronize Form Action Attributes**: In modal opening functions, update both `form.action = url` and `form.setAttribute('action', url)`.
  3. **Explicit Form Tag Closure**: Ensure every modal `<form>` has an explicit closing `</form>` tag before closing modal dialog wrapper elements to prevent form nesting hazards in browser DOM parsers.
  4. **Hidden Entity ID & Onsubmit Action Re-evaluation**: Always include `<input type="hidden" name="session_id" id="edit-session-id">` in dynamic modal forms, attach `hx-disable="true"` so HTMX completely leaves the form to native browser submission, and attach an explicit `onsubmit` handler that re-evaluates `this.action` with the entity ID at the moment of submission.
  5. **Backend Creation Endpoint Guard**: In creation endpoints (e.g., `HandleCreateSession`), always check if an entity ID (`session_id` or `id`) is present in `r.FormValue`. If present, immediately delegate to `HandleUpdateSession` instead of inserting a duplicate row. In update handlers (`HandleUpdateSession`), fall back to extracting the entity ID from form parameters if the URL path does not contain it.
