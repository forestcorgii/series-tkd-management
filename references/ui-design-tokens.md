# UI Design Tokens & Contrast Patterns

### Context: Light Mode Text Contrast & Avatar Icon Layout

**Problem:**
1. Using `text-slate-400` or `text-slate-500` for secondary text or labels on white/cream backgrounds in light mode produces low-contrast, washed-out text. In inverted templates (`text-slate-400 dark:text-slate-500`), light mode becomes even fainter than dark mode.
2. Full coach and student names placed inside fixed avatar icons (`h-12 w-12` or `h-16 w-16`) with large font classes (`text-lg`, `text-2xl`) overflow and overlap container borders and adjacent text elements.
3. Fixed-height dashboard lists (e.g. Active Floor Sessions) can expand without bounds and deform the page layout when items accumulate.

**Enforced Solution:**
| UI Element | Light Mode Enforced Class | Dark Mode Enforced Class | Notes |
| :--- | :--- | :--- | :--- |
| Secondary / Helper Text | `text-slate-600` | `dark:text-slate-400` | Never use `text-slate-400` on light mode |
| Table Headers & Labels | `text-slate-700` | `dark:text-slate-400` | Crisp legible slate contrast |
| Empty / Placeholder States | `text-slate-600` / `placeholder-slate-500` | `dark:text-slate-400` / `dark:placeholder-slate-400` | Maintains WCAG readability |
| Small Avatar Icon (`h-12 w-12`) | `text-[10px] leading-tight text-center px-1 overflow-hidden break-words line-clamp-2` | Same | Prevents text overlap on adjacent elements |
| Large Avatar Icon (`h-16 w-16`) | `text-[11px] leading-tight text-center p-1 overflow-hidden break-words line-clamp-2` | Same | Prevents text overlap on adjacent elements |
| Active Floor Sessions Panel | `max-h-[30rem] overflow-y-auto pr-1` | Same | Ensures scrollability when list grows |

### Context: Primary Action Buttons & Brand Crimson Palette

**Problem:**
1. Using emerald/green styling (`bg-emerald-500 hover:bg-emerald-400 text-slate-950`) for primary action buttons conflicts with the Series Taekwondo brand identity where Series Crimson (`#990303`) is the signature brand color.
2. Inconsistent button styling across views causes visual disorientation between auth/portals (red) and legacy CRUD/floor views (green).

**Enforced Solution:**
| UI Element | Light Mode Enforced Class | Dark Mode Enforced Class | Notes |
| :--- | :--- | :--- | :--- |
| Primary CTA Button | `bg-[#990303] hover:bg-[#7D0202] text-white font-display uppercase tracking-wider` | `dark:bg-[#DC2626] dark:hover:bg-[#B91C1C]` | Deep Crimson (#990303) in light; Vibrant Scarlet (#DC2626) in dark for high contrast against black |
| Primary Action Link/Card | `bg-[#990303] hover:bg-[#7D0202] text-white font-display uppercase tracking-wider` | `dark:bg-[#DC2626] dark:hover:bg-[#B91C1C]` | Same contrast pairing for active floor cards and links |
| Secondary / Cancel Button | `bg-slate-200 hover:bg-slate-300 text-slate-700` | `dark:bg-slate-800 dark:hover:bg-slate-700 dark:text-slate-300` | Neutral slate tone preserves visual hierarchy |

### Context: Centralized Component Utilities Architecture

**Problem:**
Repeated inline utility classes across 11 pages and 3 partials led to discrepancies in button padding, corner radiuses (`rounded-xl` vs `rounded-lg`), form focus ring colors (emerald vs brand crimson), and modal structures.

**Enforced Solution:**
Components are centralized in `web/templates/layout.html` within `<style type="text/tailwindcss">` using `@layer components`:

| Component Class | Description | Standard Usage |
| :--- | :--- | :--- |
| `.btn` | Base flex, uppercase font-display, active scale, transition | Never use alone; combine with variant |
| `.btn-primary` | Brand crimson (`#990303` light / `#DC2626` dark) with shadow | Primary CTAs (Save, Register, Schedule, Sign In) |
| `.btn-secondary` | Neutral slate (`bg-slate-200` / `dark:bg-slate-800`) | Cancel, dismiss, navigate back |
| `.btn-outline` | Crimson border with hover fill | Grade, secondary quick actions |
| `.btn-ghost` | Slate hover with transparent background | Icon buttons, theme toggle |
| `.btn-danger` | Rose red (`bg-rose-600` / `dark:bg-rose-700`) | Safety hold flags, delete actions |
| `.btn-success` | Emerald green (`bg-emerald-600`) | Mat admission, belt promotion, safety clearance |
| `.btn-warning` | Amber tone (`bg-amber-500` / `dark:bg-amber-500`) | Manual overrides, warning bypasses |
| `.btn-xs`, `.btn-sm`, `.btn-md`, `.btn-lg` | Explicit sizing scales from 10px to 14px | Table inline actions to hero CTAs |
| `.form-input` | Consistent border, bg, text, and crimson focus ring | Text, date, email, password, search inputs |
| `.form-select` | Consistent select dropdown with crimson focus ring | Form selects |
| `.form-textarea` | Consistent textarea with crimson focus ring | Form remarks, notes |
| `.form-label` | Uppercase, tracking-wider, legible slate | Form field labels |
| `.badge-belt` | Slate pill with border | Kup and Dan belt ranks |
| `.badge-ready` | Emerald chip with shadow | 🟢 READY status |
| `.badge-pretest` | Amber chip with shadow | 🟡 PRE-TEST status |
| `.badge-developing`| Rose chip with shadow | 🔴 DEVELOPING status |
| `.modal-backdrop` | Fixed inset-0 with backdrop blur and centered flex | Modal overlays |
| `.modal-dialog` | Glass panel max-w-lg or max-w-md with 2xl shadow | Modal container |
| `.modal-header` | Flex space-between with border-b | Modal title bar |
| `.modal-close` | Subtle close button with hover state | `&times;` dismissal |
| `.modal-footer` | Flex justify-end with top border | Action buttons bar |

### Context: 7-Day Time-Grid Calendar & Dynamic Class Plotting

* **Problem:** Presenting floor classes in static 2-column card lists makes it difficult for coaches and front-desk staff to visualize daily mat occupancy, find open time slots, or schedule without time overlap. Manual entry of class end times frequently leads to mismatched class durations.
* **Enforced Solution:**
  1. **7-Day Grid Matrix:** Standardize on an 8-column layout (1 time label column + 7 day columns Monday through Sunday) with hourly time rows spanning operational hours (08:00 AM to 09:00 PM).
  2. **Plotted Class Cards:** Plot classes in their starting hour cells with category-coded left borders (Sparring: Crimson `#990303`, Poomsae: Indigo, Conditioning: Amber, Promotion Prep: Purple), attendance counters, live indicators, and quick action links.
  3. **Interactive Slot Scheduling:** Every empty hourly cell on each day serves as an interactive booking target that pre-fills the clicked date and start time into `#new-session-modal`.
  4. **Automated 2-Hour Duration:** Attach an `oninput` handler `calculateEndTime()` to `start_time` that automatically calculates and populates `end_time` to 2 hours later (`(hours + 2) % 24`), preserving minutes and handling 24-hour wrap.

### Context: Universal Searchable Dropdowns for Large Rosters (Coaches & Students)

* **Problem:** Standard HTML `<select>` elements become unusable when dojangs scale to hundreds of students and coaches, causing slow, cumbersome scrolling on front-desk workstations and floor tablets. Using external JavaScript libraries introduces brittle styling overrides and breaking conflicts with HTMX forms and browser validation.
* **Enforced Solution:**
  1. **Progressive Enhancement Engine:** Enhance any `<select>` with class `searchable-select` or `data-searchable="true"`. The original `<select>` is kept in the DOM (visually hidden with `tabindex="-1"`) ensuring native form submissions, serialized values, and HTMX requests remain 100% intact.
  2. **Tailored Design System Aesthetic:** The combobox wrapper and dropdown menu match STMS design tokens (`#FFFDF4`, `#000000`, crimson focus ring `#990303` / `#DC2626`, `Outfit`/`Montserrat` typography, dark/light theme, custom scrollbar, search icon, clear button `×`, and active checkmark `✓`).
  3. **Event Isolation & HTMX Interoperability:** Typing in the search input stops event propagation (`e.stopPropagation()`) to prevent premature trigger of parent forms listening to `hx-trigger="input"`. Selecting an option programmatically updates the `<select>` and fires both `change` and `input` events so HTMX filters trigger correctly.
  4. **Bidirectional State Synchronization:** The engine intercepts `select.value` assignment via prototype property descriptor and listens to form `reset` events so external script assignments (e.g. grading candidate buttons or filter clearing) instantly update the visible combobox text.
  5. **Native Keyboard Accessibility & Validation:** Supports `ArrowDown`/`ArrowUp` navigation with auto-scroll, `Enter` selection, `Escape`/`Tab` dismissal, and synchronizes `setCustomValidity` when `required` is present on the native select.

### Context: Profile Submenu & Configurable Category Color Badges

* **Problem:**
  1. The user indicator in the header was a single direct link to `/profile`, lacking an extensible submenu for configuration and settings.
  2. Training cards, timetable borders, and category filters relied on static hardcoded Tailwind classes, preventing custom categories from rendering custom brand or discipline colors.
* **Enforced Solution:**
  * **Profile Menu Dropdown ([layout.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html))**: Transformed the header profile indicator into an interactive dropdown with chevron indicator, exposing "Go to Profile" (`/profile`), "Settings" (`/settings`), and "Sign Out" (`/logout`), with seamless outside-click dismissal.
  * **Dynamic Category Theming ([session_cards.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/session_cards.html))**: Timetable calendar cards apply dynamic inline style `border-left-color: {{.CategoryColor}}`, and category badges utilize dynamic color with alpha backgrounds (`color: {{.CategoryColor}}; background-color: {{.CategoryColor}}18; border: 1px solid {{.CategoryColor}}40;`).
  * **Dynamic Form Selects ([sessions.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/sessions.html), [live_checkin.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/live_checkin.html), [admin_portal.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/admin_portal.html))**: Dropdowns dynamically populate from the active configured categories repository.

### Context: Universal Alpha-Numeric Uppercase Transformation Engine

* **Problem:** Manual or inconsistent casing across student names, emergency contacts, notes, specialties, remarks, and venue names results in fragmented data entry and visual inconsistency across floor operations and roster reports. Pure CSS `text-transform: uppercase` renders visually uppercase text but leaves underlying DOM values in mixed/lowercase, transmitting lowercase values on form submit.
* **Enforced Solution:**
  1. **Visual Presentation Layer ([layout.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html))**:
     - Global CSS rule applies `text-transform: uppercase` to all general `input[type="text"]`, `input:not([type])`, and `textarea` elements.
     - Selectively excludes `.searchable-input`, `.no-uppercase`, and non-text types.
     - Explicitly resets `::placeholder` with `text-transform: none` to preserve original casing and legibility of hint text.
  2. **Real-Time Input Event Listener with Selection Preservation ([layout.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html))**:
     - Intercepts input events globally (`document.addEventListener('input', ..., true)`).
     - Converts `el.value` to uppercase on typing and pasting while preserving cursor position via `selectionStart` and `selectionEnd`.
  3. **Multi-Submission & HTMX Hooking ([layout.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html))**:
     - Synchronizes values on native `submit` events and HTMX `htmx:configRequest` payloads so transmitted and stored backend records are uppercase.
     - Synchronizes pre-existing values upon initial load (`DOMContentLoaded`) and post-swap DOM mutations (`htmx:afterSwap`, `htmx:afterProcess`).
  4. **Strict Scope Safeguards**:
     - Automatically excludes sensitive or formatted fields: `email`, `password`, `tel`, `number`, `date`, `time`, `search`, `url`, `color`, `checkbox`, `radio`, `hidden`.

### Context: Multi-Location Overlapping Schedules & Interactive Class Info Modal

* **Problem:** As dojang operations expand across multiple venues/locations, simultaneous classes frequently occur during the same time slot. Crushing overlapping sessions into tiny side-by-side vertical columns caused severe text truncation, clipped action buttons, and blocked users from scheduling parallel classes on occupied time slots. Furthermore, cards lacked an intuitive click target to view full session details.
* **Enforced Solution:**
  1. **Cascading Overlap Geometry ([session_handler.go](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/handlers/session_handler.go))**: When `TotalColumns > 1`, cards allocate a generous width (`width = 50 + 30/n`, capped at 75%) and stagger offsets (`left = (ColumnIndex * (100 - width)) / (n - 1)`) so cards overlap gracefully as a deck without clipping within the day column.
  2. **Hover Elevation & Location Clarity ([session_cards.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/session_cards.html))**: Overlapping cards feature `hover:z-30 hover:shadow-xl hover:border-slate-400` to smoothly surface on mouseover, and prominently render the location tag (`📍 LocationName`) in bold contrast.
  3. **Full-Card Clickability & Data Contract ([session_cards.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/session_cards.html))**: The entire card and roster table rows expose `data-*` attributes (`data-id`, `data-type`, `data-date`, `data-time`, `data-location`, `data-coach`, `data-rate`, etc.) and bind `onclick="openClassInfoFromCard(this)"`, while preserving `e.stopPropagation()` on direct live attendance links.
  4. **Class Info Modal ([sessions.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/sessions.html))**: Displays category, status, date, time, venue with map pin, pricing, instructor, admin, attendance tally, remarks, and complete action CTAs ("+ Add Overlapping Class", "Edit", "Delete", "Cancel", and "⚡ Open Live Attendance").
  5. **Explicit Scheduling CTAs ([sessions.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/sessions.html), [session_cards.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/session_cards.html))**: Unconditional `+ Schedule Class` action buttons in both the page header and calendar navigation bar ensure front-desk staff can schedule concurrent sessions without needing an empty calendar grid slot.




### Context: Floor Attendance Search Pagination & Result Simplicity

* **Problem:** 
  1. Floor attendance search previously returned an empty state when the query was blank, forcing staff to know exact student names or scan barcodes.
  2. Large dojang rosters need clean pagination capped at top 10 students per page without overloading front-desk tablet DOM trees.
  3. Displaying promotion readiness badges (READY, PRE-TEST, DEVELOPING) in attendance search results created visual clutter and diverted staff attention from rapid floor admittance and membership verification.
* **Enforced Solution:**
  1. **Blank Query Directory Listing ([session_handler.go](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/handlers/session_handler.go))**: When text input is blank, HandleSearchStudent returns all active students deterministically ordered by full name, defaulting to top 10 on page 1.
  2. **Top-10 Pagination Controls ([search_results.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/search_results.html), [live_checkin.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/live_checkin.html))**: Renders responsive pagination summary (Showing 1 to 10 of 42 students (Page 1 of 5)), Previous/Next buttons, and numbered page buttons with HTMX post handlers and page synchronization on ttendanceUpdated.
  3. **Readiness Omission in Search**: Promotion readiness status badges are removed from search_results.html, keeping the search result card focused purely on student identity, belt rank, membership/rate status, and admittance/removal actions.