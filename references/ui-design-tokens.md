# UI Design Tokens & Contrast Patterns

## 1. Color Palette & Contrast Rules
| Element | Light Mode | Dark Mode | Notes |
| :--- | :--- | :--- | :--- |
| **Brand Canvas / Cards** | `#FFFDF4` / `#F4F1E4` (border `#E5E3D8`) | `#000000` / `#111111` (border `neutral-800`) | Warm Cream light, Series Black dark. |
| **Primary CTA** | `bg-[#990303] hover:bg-[#7D0202] text-white` | `dark:bg-[#DC2626] dark:hover:bg-[#B91C1C]` | `font-display uppercase tracking-wider`. |
| **Secondary / Cancel** | `bg-slate-200 hover:bg-slate-300 text-slate-700` | `dark:bg-slate-800 dark:hover:bg-slate-700 dark:text-slate-300` | Neutral slate hierarchy. |
| **Secondary / Helper Text** | `text-slate-600` | `dark:text-slate-400` | **Never** use `text-slate-400` on light mode. |
| **Table Headers & Labels** | `text-slate-700` | `dark:text-slate-400` | Crisp contrast. |
| **Placeholders** | `placeholder-slate-500` | `dark:placeholder-slate-400` | WCAG legible. |

---

## 2. Centralized Component Classes ([`web/static/css/input.css`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/css/input.css))
- **Buttons**: `.btn` (base) + `.btn-primary`, `.btn-secondary`, `.btn-outline`, `.btn-ghost`, `.btn-danger`, `.btn-success`, `.btn-warning` + sizes `.btn-xs` to `.btn-lg`.
- **Forms**: `.form-input`, `.form-select`, `.form-textarea`, `.form-label`.
- **Badges**: `.badge-belt`, `.badge-ready` (🟢), `.badge-pretest` (🟡), `.badge-developing` (🔴).
- **KPI Ribbons**: `.kpi-strip` (`grid-cols-2 lg:grid-cols-4 p-2 gap-2`) + `.kpi-tile` (`glass-panel p-2.5 sm:p-3 rounded-lg`).
- **Compact Tables**: `.table-compact` (`th`: `py-2 px-3 text-[11px]`, `td`: `py-2 px-3 text-xs`). Wrap multi-column tables in `overflow-x-auto -mx-1 sm:mx-0` with `min-w-[540px]`–`min-w-[750px]` guardrails.
- **Scrollable Modals**: `.modal-backdrop` (`overflow-y-auto`), `.modal-dialog.modal-dialog-scrollable` (`max-h-[90vh] sm:max-h-[85vh] flex flex-col overflow-hidden my-auto`), `.modal-header` (`shrink-0`), `.modal-body` (`overflow-y-auto flex-1 min-h-0`), sticky `.modal-footer` (`shrink-0`).
- **Avatars & Panels**: `h-12 w-12` (`text-[10px] leading-tight line-clamp-2`), `h-16 w-16` (`text-[11px] leading-tight line-clamp-2`). Active floor list: `max-h-[30rem] overflow-y-auto pr-1`.
- **Hidden Scrollbars**: `.scrollbar-none` / `.no-scrollbar` on horizontal chip/pill strips.

---

## 3. Shell, Navigation & Accessibility
- **Compact Headers**: Operational pages use `<h1 class="sr-only">Title</h1>` (0px visual height) and single-row toolbars. Global header is `48px` (`py-2`), container padding `py-3 sm:py-4 pb-16 lg:pb-4 min-w-0`.
- **Profile Dropdown ([`layout.html`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html))**: Exposes `/profile`, `/settings`, `/logs` (Manager only), and `/logout`.
- **Mobile Navigation**: Slide-out `#mobile-nav-drawer` (`lg:hidden`) + fixed `#mobile-bottom-bar` synced via `syncMobileBottomBarUI()` on `DOMContentLoaded`, `htmx:load`, `htmx:afterSwap`, and `htmx:historyRestore`.
- **Touchscreen Tooltips**: `.has-tooltip` -> `.tooltip-icon` (`ⓘ`, min 28px tap target) + `.tooltip-content` (`z-70`). Hover on `@media (hover: hover)`; tap-to-toggle `.tooltip-active` on touch devices with outside-tap dismissal.
- **Floor Search Pagination (`HandleSearchStudent`)**: Blank query returns active students ordered by name, paginated at 10 per page in [`search_results.html`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/search_results.html) without readiness badges.

---

## 4. 7-Day Calendar Grid, Mobile Views & Public Privacy
- **Responsive Grid Isolation ([`input.css`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/css/input.css))**:
  - `@media (min-width: 768px)`: `.calendar-grid-desktop` (`64px repeat(7, minmax(0, 1fr)) !important`), reinforced with inline grid styles on [`session_cards.html`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/session_cards.html). Never hardcode `.calendar-grid-mobile-*` in server-rendered HTML.
  - `@media (max-width: 767px)`: `.calendar-grid-mobile-day` (`44px 1fr !important`, default) and `.calendar-grid-mobile-3day` (`44px repeat(3, minmax(130px, 1fr)) !important`), toggled via `#mobile-week-strip` / `setMobileCalendarViewMode` and preserved across `htmx:afterSwap`.
- **Class Cards & Overlaps ([`session_handler.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/handlers/session_handler.go))**:
  - Category colors applied dynamically via `border-left-color: {{.CategoryColor}}` and `{{.CategoryColor}}18` badges.
  - Overlapping slots (`TotalColumns > 1`) stagger as a deck (`width = 50 + 30/n` capped at 75%, `hover:z-30`).
  - Full-card `onclick="openClassInfoFromCard(this)"` with `data-*` attributes; `calculateEndTime()` auto-sets `+2h` on `start_time` input.
- **Collapsible Filters ([`sessions.html`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/sessions.html))**: `#session-filter-container` toggled by `#filter-toggle-btn` with `localStorage` persistence; mobile `+` CTA uses `.tooltip-content.tooltip-right.sm:hidden`.
- **Public Calendar Privacy ([`landing_calendar_grid.html`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/landing_calendar_grid.html))**: Uses `PublicDisplayName` (`First L.`) and `PublicInitials` (`guest_reservation_form.html` modal shows confirmed attendees).

---

## 5. Interactive Controls & Social Canvas Export
- **Searchable Dropdowns (`.searchable-select` / `data-searchable="true"`)**: Keeps native `<select>` in DOM (`tabindex="-1"`), calls `e.stopPropagation()` while typing, dispatches synthetic `change` + `input` on selection, intercepts `select.value` setter, and supports `ArrowDown`/`ArrowUp`/`Enter`/`Escape`.
- **Global Uppercase Engine ([`layout.html`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html))**:
  - Uppercases general text inputs/textareas on `input` (preserving cursor), `submit`, and `htmx:configRequest`; resets `::placeholder` to `text-transform: none`.
  - **Opt-Out**: Add `.no-uppercase`, `data-no-uppercase="true"`, and `autocapitalize="none"` to case-sensitive fields. Automatically exempted: `/login`, `/register`, `/settings`, `/forgot-password`, `/reset-password`, non-text input types, and `username`, `identifier`, `token`, `key`, `secret`, `smtp_host`, `smtp_user`, `display_name`.
- **Schedule PNG Export ([`schedule-export.js`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/js/schedule-export.js))**:
  - Uses `html2canvas` `onclone` (`windowWidth: 1200`, `scale: 2`) to strip interactive controls, expand all 7 columns, format 12h AM/PM times, omit internal `Coach:`/`Admin:`/notes/attendee counts, and match the active system theme (`#111827` dark / `#FFFFFF` light) to download `SeriesTKD-Schedule-YYYY-MM-DD.png`.

### Context: Active Navigation Highlighting, Global HTMX Progress, & Auto-Dismiss Notices

* **Problem:**
  1. Top navigation and mobile bottom quick bar links lacked visual active state feedback, making path location ambiguous.
  2. HTMX requests (searches, pagination, filters) gave no immediate feedback while requests were in flight.
  3. Success and error notice banners persisted indefinitely unless manually closed.
  4. Modals required manual click on close buttons and lacked backdrop click or Escape key dismissal.
* **Enforced Solution:**
  1. **Active Link Highlighting Engine ([layout.html](file:///C:/Users/seani/Documents/Repo/series-tkd-management/web/templates/layout.html))**: `syncActiveNavLinks()` dynamically updates desktop navbar and mobile bottom bar links matching `window.location.pathname` with crimson text/background accents on `DOMContentLoaded`, `popstate`, and `htmx:afterSwap`.
  2. **Global Progress Indicator ([layout.html](file:///C:/Users/seani/Documents/Repo/series-tkd-management/web/templates/layout.html), [input.css](file:///C:/Users/seani/Documents/Repo/series-tkd-management/web/static/css/input.css))**: `#htmx-indicator` renders a fixed top gradient bar during HTMX requests via `hx-indicator="#htmx-indicator"`.
  3. **Auto-Dismiss Notice Banners ([layout.html](file:///C:/Users/seani/Documents/Repo/series-tkd-management/web/templates/layout.html))**: `autoDismissNotices()` automatically fades out and removes `.animate-fade-in` notice banners after 5 seconds with vertical translation.
  4. **Global Modal Keyboard & Backdrop Dismissal ([layout.html](file:///C:/Users/seani/Documents/Repo/series-tkd-management/web/templates/layout.html))**: Listens to global `Escape` key and backdrop clicks on `.modal-backdrop` to hide open dialogs.
  5. **Empty Search Directory States ([student_table_rows.html](file:///C:/Users/seani/Documents/Repo/series-tkd-management/web/templates/partials/student_table_rows.html))**: Added an `{{else}}` empty state row with search icon and actionable message when search queries yield zero matching rows.
