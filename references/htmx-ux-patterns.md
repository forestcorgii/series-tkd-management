# HTMX Interaction & UX Patterns

## Quick-Lookup Invariant Matrix

| Pattern / Flow | Trigger / Attribute | Target & Swap | Backend Handler Rule |
| :--- | :--- | :--- | :--- |
| **Global Page Navigation** | `<body hx-boost="true">` | Full page swap (preserves CSS/JS/fonts) | Boosted requests send `HX-Request: true` **AND** `HX-Boosted: true`. |
| **Filter / Search Partials** | `hx-get` / `hx-post` with `delay:250ms` | `#student-table-rows`, `#classes-container`, `#search-results` | Only return partial if `HX-Request == "true" && HX-Boosted != "true"`. |
| **Dynamic JS Modal Forms** | `hx-boost="false" hx-disable="true"` | Native form submit (`303 See Other`) | Include `<input type="hidden" name="session_id">` and guard `HandleCreate*` against duplicate creation when ID is present. |
| **Live Attendance Sync** | Response header `HX-Trigger: attendanceUpdated` | `hx-trigger="attendanceUpdated from:body"` | Syncs `#student-header`, `#readiness-hud`, `#wallet-ledger-card`, and live floor rosters simultaneously. |
| **Calendar Drag-and-Drop** | `htmx.ajax('POST', '/sessions/{id}/reschedule', ...)` | `HX-Target: classes-container` (`innerHTML`) | `HandleRescheduleSession` returns `session_cards.html` partial with `HX-Push-Url`. |
| **Logout / Session Teardown** | `hx-boost="false"` | Full browser navigation to `/login` | Ensures clean session cookie clearing and DOM reset. |

---

### Context: Boosted Navigation vs. Targeted Filter Partials (`HX-Boosted` Invariant)

* **Problem**: Enabling `hx-boost="true"` on `<body>` makes all internal link clicks send `HX-Request: true`. Handlers (`HandleStudents`, `HandleSessions`) that checked only `r.Header.Get("HX-Request") == "true"` returned bare table/calendar fragments instead of the full page layout when navigating from the navbar.
* **Enforced Solution**:
  * Always distinguish targeted partial requests from boosted page navigation using:
    ```go
    isPartial := r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true"
    ```
  * Hook theme synchronization (`syncThemeUI()`) and mobile drawer auto-close to `htmx:load` and `htmx:afterSwap`.

### Context: Dynamic Modal Forms & HTMX Boost Closure Hazard

* **Problem**: Modal forms initialized with `action=""` (populated via JS on clicking "Edit") have their empty `action=""` captured by HTMX's `hx-boost` listener at page load, causing edits to `POST` to `/sessions` (creating duplicate classes) instead of `/sessions/{id}/edit`.
* **Enforced Solution**:
  1. Add `hx-boost="false"` and `hx-disable="true"` to all dynamic modal `<form>` elements (`#edit-session-form`, `#delete-session-form`, `#cancel-session-form`, `#reschedule-session-form`, `#edit-location-form`, `#edit-plan-form`, `#edit-cat-form`).
  2. Set both `form.action = url` and `form.setAttribute('action', url)` in JS modal openers, plus an `onsubmit` handler that re-verifies `this.action`.
  3. Include `<input type="hidden" name="session_id" id="edit-session-id">` and guard `HandleCreateSession` to delegate to `HandleUpdateSession` if an ID is present.

### Context: Event-Driven Multi-Widget Sync (`attendanceUpdated`)

* **Problem**: Admitting or removing a student on `/portal/student` or `/sessions/{id}/live` left adjacent widgets (Readiness HUD, Prepaid Wallet balance, search admittance buttons) stale until a manual page refresh.
* **Enforced Solution**:
  * Check-in and attendance removal endpoints emit `w.Header().Set("HX-Trigger", "attendanceUpdated")`.
  * Dependent widgets declare `hx-trigger="attendanceUpdated from:body"` to refresh in-place automatically.

### Context: Searchable Combobox & HTMX Event Isolation

* **Problem**: Typing inside a custom `.searchable-select` filter input bubbled `input` events up to parent `<form hx-trigger="input changed delay:250ms">` containers, firing premature filter queries on every keystroke before an option was chosen.
* **Enforced Solution**:
  * Typing in the combobox search box calls `e.stopPropagation()` on `input` events.
  * Selecting an option updates the underlying `<select>` value and dispatches synthetic `change` and `input` events (`{ bubbles: true }`) so HTMX forms trigger only upon selection.

