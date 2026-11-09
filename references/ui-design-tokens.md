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
Repeated inline utility classes across pages and partials led to discrepancies in button padding, corner radiuses (`rounded-xl` vs `rounded-lg`), form focus ring colors (emerald vs brand crimson), and modal structures.

**Enforced Solution:**
Components are centralized in [`web/static/css/input.css`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/css/input.css) using `@layer components`:

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
  5. **Explicit Scheduling CTAs ([sessions.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/sessions.html), [session_cards.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/session_cards.html))**: Unconditional `+ Schedule Class` action buttons in the calendar navigation bar ensure front-desk staff can schedule concurrent sessions without needing an empty calendar grid slot.

### Context: Floor Attendance Search Pagination & Result Simplicity

* **Problem:** 
  1. Floor attendance search previously returned an empty state when the query was blank, forcing staff to know exact student names or scan barcodes.
  2. Large dojang rosters need clean pagination capped at top 10 students per page without overloading front-desk tablet DOM trees.
  3. Displaying promotion readiness badges (`READY`, `PRE-TEST`, `DEVELOPING`) in attendance search results created visual clutter and diverted staff attention from rapid floor admittance and membership verification.
* **Enforced Solution:**
  1. **Blank Query Directory Listing ([session_handler.go](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/handlers/session_handler.go))**: When text input is blank, `HandleSearchStudent` returns all active students deterministically ordered by full name, defaulting to top 10 on page 1.
  2. **Top-10 Pagination Controls ([search_results.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/search_results.html), [live_checkin.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/live_checkin.html))**: Renders responsive pagination summary (`Showing 1 to 10 of 42 students (Page 1 of 5)`), Previous/Next buttons, and numbered page buttons with HTMX post handlers and page synchronization on `attendanceUpdated`.
  3. **Readiness Omission in Search**: Promotion readiness status badges are removed from `search_results.html`, keeping the search result card focused purely on student identity, belt rank, membership/rate status, and admittance/removal actions.

### Context: Mobile-First Uncluttered UX & Touchscreen Universal Tooltip System

* **Problem:**
  1. **Vertical Screen Waste:** Bulky top page banners (`<h1>` headers with descriptive paragraph subtitles) occupied 100–160px of prime viewport real estate on mobile devices and floor tablets, forcing repetitive scrolling to access core operational forms, tables, and buttons.
  2. **Mobile Nav Cramping:** Rendering a horizontal scrolling single-row overflow link bar with 11 items caused accidental horizontal swipes, clipped icons, and poor ergonomics on small handheld screens.
  3. **Touch Incompatibility of Tooltips:** Traditional CSS `:hover` tooltips fail on touchscreens and cannot be dismissed without tapping another interactive element.
  4. **Table Crushing:** Responsive multi-column operational tables (5–8 columns) collapsed into illegible vertical text wraps without minimum column width safeguards.

* **Enforced Solution:**
  1. **Accessible Page Header Removal ([students.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/students.html), [sessions.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/sessions.html), [coaches.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/coaches.html), [admins.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/admins.html), [packages.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/packages.html), [locations.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/locations.html), [settings.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/settings.html))**:
     - Large visible `<h1>` header banners and descriptive subtitles are removed across all operational pages.
     - Preserves semantic accessibility and test assertions using `<h1 class="sr-only">Title</h1>`, which consumes 0px of visual space.
     - Primary action buttons and search controls are consolidated into compact single-row toolbars (`flex items-center justify-between gap-3`).
  2. **Universal Touchscreen Tooltip Engine ([layout.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html))**:
     - Standardized container `.has-tooltip` with button `.tooltip-icon` (`ⓘ`, min 28px tap target) and popover `.tooltip-content` (dark slate `#0F172A` / `#1E293B`, rounded-lg, z-70, shadow-xl).
     - Hover-enabled on desktop (`@media (hover: hover)`).
     - Tap-to-toggle enabled on mobile devices via global delegated click handler toggling `.tooltip-active`, with automatic outside-tap dismissal.
  3. **Mobile Drawer & Role-Aware Bottom Quick Bar ([layout.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html))**:
     - Replaced cramped horizontal link row with a sleek slide-out navigation drawer (`#mobile-nav-drawer`) triggered by header hamburger button (`lg:hidden`).
     - Added a fixed ergonomic bottom bar (`#mobile-bottom-bar`, `lg:hidden`) tailored by user role:
       - **Manager / Admin:** Dashboard, Classes, Students, Passes, More (opens drawer).
       - **Coach:** Classes, Students, Floor Console, More (opens drawer).
       - **Student:** Portal, Profile, Menu (opens drawer).
     - Base container padding updated to `px-3 sm:px-6 py-4 sm:py-6 pb-20 lg:pb-6 min-w-0` to guarantee breathing room and clear the bottom bar.
  4. **Guarded Anti-Cramp Table Scrollers**:
     - All multi-column tables are wrapped in `overflow-x-auto -mx-1 sm:mx-0` with explicit column guardrails:
       - Students Table: `min-w-[700px]`
       - Sessions Calendar: `min-w-[700px]`
       - Coaches Table: `min-w-[750px]`
       - Admins Table: `min-w-[700px]`
       - Packages / Memberships: `min-w-[700px]`
       - Locations Table: `min-w-[700px]`
       - Training Categories: `min-w-[600px]`
       - Live Floor Roster: `min-w-[550px]`
       - Promotion Pipeline Tables: `min-w-[540px]` to `min-w-[700px]`

### Context: Case-Sensitive Input & Targeted Textbox Uppercase Suppression

**Problem:**
1. Global alpha-numeric uppercase engine and CSS (`text-transform: uppercase`) forcibly transformed all textboxes into uppercase on typing, paste, and form submit.
2. Case-sensitive or casing-specific credentials and configuration settings—including login/forgot-password identifiers (email or username), user registration handles, display names, SMTP hosts/usernames, SMS phone numbers, and cryptographic Web Push VAPID public keys—were corrupted or visually distorted when transformed into uppercase.

**Enforced Solution:**
1. **Multi-Layer Selector & Property Opt-Out ([layout.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html))**:
   - CSS `text-transform: uppercase` excludes `.no-uppercase`, `[data-no-uppercase="true"]`, `[autocapitalize="none"]`, `[autocapitalize="off"]`, `[name="username"]`, `[name="identifier"]`, and `[autocomplete="username"]`.
   - Explicit override rule `text-transform: none !important;` applies to `.no-uppercase`, its child inputs/textareas, and known credential field names (`smtp_host`, `smtp_user`, `web_push_public_key`, `web_push_subject`, `sms_from_number`).
2. **Path & Identifier Aware JS Engine ([layout.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html))**:
   - `isAlphaNumericTextInput(el)` inspects `el.closest('.no-uppercase, [data-no-uppercase="true"]')` as well as standard HTML `autocapitalize="none"`.
   - Entire case-sensitive routes (`/login`, `/register`, `/settings`, `/forgot-password`, `/reset-password`) are automatically exempted.
   - Case-sensitive names (`username`, `identifier`, `token`, `key`, `secret`, `smtp_host`, `smtp_user`, `web_push_public_key`, `web_push_subject`, `sms_from_number`, `display_name`) are exempted globally across all routes.
3. **Template Declarations**:
   - Explicitly annotate forms and inputs in `login.html`, `register.html`, `settings.html`, `forgot_password.html`, `reset_password.html`, `admins.html`, and `profile.html` with `.no-uppercase`, `data-no-uppercase="true"`, `autocapitalize="none"`, and `spellcheck="false"` where appropriate.

### Context: High-Density Compact System Redesign & Redundancy Purging

**Problem:**
1. Oversized hero cards, 6-card redundant "Quick Access" launcher hubs, and deep card gutters (`p-6` to `p-8`, `gap-6` to `gap-8`) wasted vertical viewport space above the fold across all administrative, coach, and student views.
2. Tables suffered from excessive cell vertical padding (`py-4`), multi-line filter stacks, and heavy SVG polygon radar charts inside table rows (>90px row height).
3. Redundant duplicate action buttons (e.g. "+ Schedule Class" rendered both in filter bars and card loops, duplicate logout buttons in headers and sidebars) added unnecessary visual noise.

**Enforced Solution:**
1. **Design Tokens & Palette Preservation**:
   - Strict 100% brand color palette retention: Crimson (`#990303`/`#7D0202`), Dark Scarlet (`#DC2626`), Black (`#000000`), Cream (`#FFFDF4`/`#F4F1E4`), Slate (`#E5E3D8`), and semantic badges (emerald/amber/rose).
2. **High-Density KPI Ribbons (`.kpi-strip` & `.kpi-tile`)**:
   - Multi-card metric grids replaced with unified horizontal strips (`.kpi-strip grid grid-cols-2 lg:grid-cols-4 p-2 gap-2 rounded-xl bg-slate-50/80 dark:bg-neutral-900/60`).
   - Compact tiles (`.kpi-tile glass-panel p-2.5 sm:p-3 rounded-lg`) reduce vertical footprint from ~140px down to ~52px.
3. **Compact Tables (`.table-compact`)**:
   - `.table-compact th` (`py-2 px-3 text-[11px] font-bold uppercase tracking-wider`).
   - `.table-compact td` (`py-2 px-3 text-xs`).
   - In student table rows, replaced heavy 64x64 SVG radar bottleneck with sleek athletic metric pills (`⚡ 8.4/10`), reducing row height from >90px to ~38px.
4. **Header & Shell Compression**:
   - Slimmed global header from 64px to 48px (`py-2`, `h-8 w-8` badge, `px-2.5 py-1 text-xs` nav links).
   - Compacted main container vertical gutters (`py-3 sm:py-4 pb-16 lg:pb-4 min-w-0`) and slimmed footer (`py-2 text-[11px]`).
5. **Redundancy Elimination**:
   - Purged duplicate launchers, duplicate schedule buttons, and duplicate logout controls. All views deliver instant operational controls directly on the surface.

### Context: Calendar Header Resilient Grid & Horizontal Day Alignment

* **Problem:**
  1. Relying solely on Tailwind arbitrary class `grid-cols-[64px_repeat(7,1fr)]` without explicit inline styles created vulnerability to browser cache mismatches (e.g., PWA Service Worker caching prior CSS builds lacking `64px`), causing `display: grid` with unspecified columns to collapse into a single column where all 7 day headers stacked vertically.
  2. Day column header cells previously stacked the Day Short name, Day Number circle, and class count across 3 separate vertical blocks, exacerbating perceived vertical stretching in calendar headers.
  3. Week navigation bars with `flex-col sm:flex-row` collapsed into vertical stacks on narrower viewports.

* **Enforced Solution:**
  1. **Inline Grid Resilience ([session_cards.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/session_cards.html))**: Added explicit `style="display: grid; grid-template-columns: 64px repeat(7, minmax(0, 1fr));"` directly on both the sticky day headers row and the scrollable timeline body grid. Even if external CSS caching delays arbitrary class evaluation, the 8-column layout (64px time gutter + 7 day columns) never collapses.
  2. **Horizontal Day Alignment ([session_cards.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/session_cards.html))**: Day of the week (`.DayShort`) and date number circle (`.DayNumber`) are enclosed in a horizontal flex layout (`flex items-center justify-center gap-1.5`) with a compact class count pill underneath, maintaining a clean, modern Google Calendar-style horizontal presentation.
  3. **Non-Collapsing Week Navigation ([session_cards.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/session_cards.html))**: Week controls and date range use `flex flex-wrap items-center justify-between` to ensure horizontal cohesion across viewports.

### Context: Scrollable Modals & Sticky Action Footers

* **Problem:**
  1. Multi-field schedule and attendance forms (e.g. Schedule Class, Edit Training Class/Item) exceeded viewport height on typical laptop and tablet screens.
  2. Because `.modal-backdrop` lacked vertical overflow handling and modal dialogs lacked height capping, modal footers containing primary CTAs (e.g. "Save Changes", "Schedule Class") were pushed off-screen and rendered inaccessible.
  3. Centering overflowing dialogs in a pure flex container (`items-center`) clipped top content into negative scroll space when scrolling wasn't managed with `my-auto`.

* **Enforced Solution:**
  1. **Scrollable Modal Dialogs (`.modal-dialog-scrollable`)**:
     - Outer dialog is capped with `@apply max-h-[90vh] sm:max-h-[85vh] flex flex-col p-0 sm:p-0 overflow-hidden space-y-0;`.
     - `.modal-header` and switcher tabs remain pinned at top with `shrink-0`.
     - `.modal-body` contains form fields with `@apply overflow-y-auto flex-1 min-h-0 p-4 sm:p-5 space-y-4;` with custom sleek scrollbars and `overscroll-behavior: contain`.
     - `.modal-footer` is sticky at the bottom with `@apply shrink-0 px-4 sm:px-5 py-3 border-t border-[#E5E3D8] dark:border-neutral-800 bg-white dark:bg-[#111111];`, ensuring "Save Changes" and primary submit actions are immediately visible and clickable at all times.
  2. **Backdrop Overflow Safety**:
     - `.modal-backdrop` includes `@apply ... overflow-y-auto;` and `.modal-dialog` includes `@apply ... my-auto;`, preventing top/bottom clipping on smaller viewports.

### Context: Attendance Page Collapsible Filters & Compact Mobile Scheduling CTAs

* **Problem:**
  1. Static multi-input filter bars on the attendance/sessions calendar occupied 80–120px of vertical viewport real estate, pushing the calendar timeline below the fold on mobile and tablet devices.
  2. The primary "+ Schedule Class" CTA was placed in the top page header far above the calendar navigation controls, causing visual fragmentation and consuming horizontal space on small screens.
* **Enforced Solution:**
  1. **Calendar Toolbar Scheduling CTA ([session_cards.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/session_cards.html))**: Removed the redundant date range text from the navigation bar (as dates are already visible in the sticky timeline day headers). Consolidated the total classes badge immediately before the `+ Schedule Class` action button on the right side of the navigation bar, keeping controls on a single, non-wrapping row across mobile and desktop.
  2. **Mobile Space Optimization with Responsive Tooltip ([session_cards.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/session_cards.html), [input.css](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/css/input.css))**: On mobile screens (`< sm`), the button text collapses to a space-saving `+` icon wrapped in `.has-tooltip` with a right-aligned tooltip (`.tooltip-content.tooltip-right.sm:hidden`) revealing "Schedule Class". On tablet and desktop (`>= sm`), the full `+ Schedule Class` text is rendered.
  3. **Collapsible Filter Bar ([sessions.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/sessions.html))**: The filter form is wrapped in a collapsible container (`#session-filter-container`) toggled by an accessible `#filter-toggle-btn` with active-count badge and animated chevron. Filter preferences persist across page views via `localStorage` and automatically remain expanded whenever any filter criteria are active.

### Context: Calendar Social Media Snapshot & Canvas Export

* **Problem:**
  1. Exporting client-rendered DOM schedules to high-resolution social media graphics requires removing interactive UI elements (filter forms, modals, drag-to-reschedule indicators, "Sign In to Attend →" links, administrative tables).
  2. Rendering directly on the active DOM causes UI flickering, scrollbar clipping, or layout shifts on mobile viewports.
  3. Exported social graphics require consistent dark mode `#111827` branding with high contrast, legible coach names, branch venues, and dates regardless of the user's local theme toggle.

* **Enforced Solution:**
  1. **Non-Destructive Clone Sanitization (`onclone`)**: Use `html2canvas`'s `onclone(clonedDoc, clonedElement)` hook in [`schedule-export.js`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/js/schedule-export.js) to strip interactive elements (`#filter-toggle-btn`, `#session-filter-container`, `.calendar-slot-dropzone span`, `.calendar-session-card a`, `.table-compact`), un-clamp scroll heights, expand all 7 day columns, and inject branded header/footer banners without mutating the live DOM.
  2. **Retina 2x Scaling & Fixed 1080px Width**: Explicitly set `windowWidth: 1200`, `scale: 2`, and `backgroundColor: '#111827'` so responsive breakpoints render the full 7-day schedule at 2160px canvas resolution, ready for Instagram and Facebook feed posts.
  3. **Visual Feedback & Automatic Download**: The trigger button immediately enters a disabled `aria-busy="true"` state displaying `⏳ Generating...`, automatically initiates a PNG download named `SeriesTKD-Schedule-YYYY-MM-DD.png`, and cleanly restores button state in a `finally` block.
  4. **12-Hour AM/PM Time Formatting & Social Card Cleansing**:
     - Converted all 24-hour card time displays (`10:00 - 12:00`, `17:00 - 18:15`) to 12-hour AM/PM (`10:00 AM - 12:00 PM`, `5:00 PM - 6:15 PM`) using `to12Hour()` and `formatTimeRange12H()`.
     - Completely stripped internal `Coach: ...`, `Admin: ...`, and `Staff: ...` entries to keep public social schedules clean and focused purely on class offerings.
     - Prevented `html2canvas` vertical text slicing by removing `truncate` and `overflow: hidden`, switching card text to standard system font stack (`-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif`), setting `line-height: 1.35`, and enforcing `minHeight: 68px` on cards to ensure attendance (`👥 X Attending`) and times never clip.
     - Purged redundant week navigation bar (`#calendar-week-nav-bar`) and live current time indicator (`#calendar-now-pill`, `#calendar-now-line`).
     - Added cache busting query parameters (`?v=2.7.0`) in [layout.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html) and [sessions.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/sessions.html), and bumped PWA Service Worker to `stms-v13` in [sw.js](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/sw.js) to invalidate stale browser script caches.
  5. **Dynamic System Theme Matching & Clean Social Export**:
     - Dynamically detects the system's current theme (`isDark` from `document.documentElement.classList.contains('dark')` / `localStorage.getItem('stms-theme')`):
       - When system is Dark: exports `#111827` dark background, `#DC2626` accents, and `#F9FAFB` high contrast text.
       - When system is Light: exports `#FFFFFF` clean background, `#990303` accents, and `#0F172A` deep slate text.
     - Pixel-perfect badge row alignment: enforces fixed matching dimensions (`height: 18px`, `display: inline-flex`, `align-items: center`, `justify-content: center`, `line-height: 1`) on both `CLASS` and `CLOSED` badges, aligning them along a shared baseline.
     - Formats split overlapping cards (narrow width < 80px) with stacked 2-line times (`11:30 AM` / `1:30 PM`) so digits never get cut or wrapped across letter boundaries.
     - Stripped internal coaching/curriculum notes (`"Taegeuk 1 through 8..."`) and all attendant counts/icons (`👥`).
     - Fixed header and footer HTML template formatting to eliminate raw string leaks.


### Context: Mobile Calendar Day View & Resilient Multi-Day Grid

* **Problem:**
  1. Cramming 3 full day columns into a mobile viewport (`< 768px`) with `min-w-0` reduced each column to ~90-100px. Inner cards had only 80px width (or ~45px when overlapping), causing category badges, status chips, times, coach names, and action buttons (`⚡ Live`, `Reserve Slot`) to wrap into 3+ lines or clip severely.
  2. The mobile grid arbitrarily hid days 3 to 6 (Thursday–Sunday) with no interactive weekday overview to jump between days of the active week.
  3. Relying purely on Tailwind arbitrary classes like `grid-cols-[48px_repeat(3,1fr)]` without explicit CSS grid fallbacks risked layout collapse if CSS builds were deferred.

* **Enforced Solution:**
  1. **7-Day Interactive Mobile Week Strip ([session_cards.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/session_cards.html), [landing_calendar_grid.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/landing_calendar_grid.html))**: Added a horizontal 7-day pill strip (`#mobile-week-strip` / `#landing-mobile-week-strip`) directly above the calendar on mobile. Displays all 7 days with day abbreviation, date number circle, and session count dots. Tapping any day pill immediately selects that day with instant client-side column switching.
  2. **Mobile Day View Default (`.calendar-grid-mobile-day`)**: By default on mobile (`< md`), the calendar displays in spacious **Day View** (`44px 1fr`), giving the active day ~320px width. Cards render with generous breathing room, legible typography, unclipped times, and spacious action buttons (`⚡ Live` and `Reserve Slot`).
  3. **3-Day View Toggle with Minimum Width Guard (`.calendar-grid-mobile-3day`)**: Provided a mobile view switcher (`Day` / `3-Day`). In 3-day mode, enforces `minmax(130px, 1fr)` per day column (total container `min-w-[434px]`) with smooth touch scrolling and auto-scroll to the selected day.
  4. **CSS Grid Resilience ([input.css](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/css/input.css))**: Centralized `.calendar-grid-desktop` (`64px repeat(7, minmax(0, 1fr))`), `.calendar-grid-mobile-day` (`44px 1fr`), and `.calendar-grid-mobile-3day` (`44px repeat(3, minmax(130px, 1fr))`) in `@layer components` to ensure consistent rendering across all viewports.
  5. **Universal Hidden Scrollbars (`.scrollbar-none` / `.no-scrollbar`)**: Enforce `-ms-overflow-style: none; scrollbar-width: none;` and `::-webkit-scrollbar { display: none; }` on horizontal chip/pill selectors (such as the branch location filter bar and 7-day strip) to eliminate unsightly OS scrollbars while preserving touch, trackpad, and mousewheel horizontal scrolling.

### Context: Mobile Bottom Quick-Access Bar & Active Route Highlighting

* **Problem:**
  1. Student users on mobile viewports lacked a direct bottom navigation button to access the class schedule (`/`), forcing them to open the mobile drawer or use the top logo navigation.
  2. The student bottom bar had hardcoded active styling on the "Portal" button, causing it to appear highlighted even when viewing the Calendar (`/`) or Profile (`/profile`).

* **Enforced Solution:**
  1. **Calendar Quick-Access ([layout.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html))**: Added `📅 Calendar` (linking to `/`) directly in `#mobile-bottom-bar` alongside `🥋 Portal`, `👤 Profile`, and `☰ Menu`, giving student users 1-tap switching between classes and portal.
  2. **Client-Side Route Sync Engine (`syncMobileBottomBarUI`)**: Added an event-driven sync engine bound to `DOMContentLoaded`, `htmx:load`, `htmx:afterSwap`, and `htmx:historyRestore`. Dynamically matches `window.location.pathname` to apply active brand styling (`text-[#990303] dark:text-red-400 font-bold`) and reset inactive items to `text-slate-600 dark:text-slate-400`.

### Context: Calendar Grid Responsive Isolation & Mobile Mode Retention on HTMX Swap

* **Problem:**
  1. **Vertical Column Collapse on Desktop**: Defining `.calendar-grid-mobile-day` (`grid-template-columns: 44px 1fr !important`) without `@media` boundary caused it to override Tailwind's responsive `md:grid-cols-[64px_repeat(7,1fr)]` on desktop (`>= 768px`). The 7 day columns collapsed into a 2-column grid and wrapped into 4 vertical rows.
  2. **Mobile Reversion on Navigation**: When clicking Prev/Next week navigation via HTMX, the partial templates ([session_cards.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/session_cards.html), [landing_calendar_grid.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/landing_calendar_grid.html)) re-rendered with static Day mode markup and classes. The `htmx:afterSwap` lifecycle failed to invoke the active view switcher (`setMobileCalendarViewMode` / `setLandingCalendarViewMode`), leaving the 3-day mode unhidden days inside a 2-column grid (`44px 1fr`), causing the header to stack vertically and the toggle button to falsely display "Day".

* **Enforced Solution:**
  1. **Media Query Isolation ([input.css](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/css/input.css))**: Strictly scoped `.calendar-grid-desktop` and desktop calendar header/body IDs to `@media (min-width: 768px)` (`64px repeat(7, minmax(0, 1fr)) !important`), and scoped `.calendar-grid-mobile-day` (`44px 1fr !important`) and `.calendar-grid-mobile-3day` (`44px repeat(3, minmax(130px, 1fr)) !important`) inside `@media (max-width: 767px)`. Mobile classes can never distort desktop viewports.
  2. **Purge Mobile Classes from Server-Rendered HTML ([session_cards.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/session_cards.html), [landing_calendar_grid.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/landing_calendar_grid.html))**: Removed hardcoded `.calendar-grid-mobile-day` from static HTML templates. Desktop relies purely on `md:grid-cols-[64px_repeat(7,1fr)]`, with mobile classes dynamically added by client JS only when `window.innerWidth < 768`.
  3. **Desktop View Guard in JS ([sessions.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/sessions.html), [landing.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/landing.html))**: Added immediate early returns to `setMobileCalendarViewMode` and `setLandingCalendarViewMode` when `window.innerWidth >= 768`, actively purging any stray mobile grid classes so desktop 7-column layout cannot be altered.
  4. **Production Stylesheet Cache-Busting ([layout.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html))**: Added version query tag `/static/css/tailwind.css?v=3.0.0` to bypass the 24-hour browser HTTP asset cache (`Cache-Control: public, max-age=86400`) and ensure immediate rollout of updated media queries.
  5. **State Retention Across Swaps ([landing.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/landing.html), [sessions.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/sessions.html))**: Updated `initLandingMobileCalendarView` and `initMobileCalendarView` to retain `currentLandingMobileViewMode` and `currentMobileViewMode` across HTMX swaps by calling `setLandingCalendarViewMode` / `setMobileCalendarViewMode`. Re-applies active button styling, grid classes (`calendar-grid-mobile-3day`), and horizontal scrolling to current day.

### Context: Public Calendar Attendee Privacy & Roster Presentation

* **Problem:**
  Displaying full attendee rosters on public landing calendars can violate student privacy (exposing full legal names to unauthenticated visitors) while overloading compact calendar cards with too much text in tight time-grid cells.

* **Enforced Solution:**
  1. **Privacy-Preserving Formatter (`FormatPublicName`, `PublicDisplayName`)**: Format attendee names to First Name + Last Initial (e.g., "Alex Vance" becomes `Alex V.`, "Maria Santos" becomes `Maria S.`) across public-facing views.
  2. **Calendar Grid Card Footer Layout ([landing_calendar_grid.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/landing_calendar_grid.html))**: The attendees indicator (`👥 X / Y Attendees`) and role-colored mini avatar stack are positioned in the card's bottom action bar directly beside the "Reserve Slot" CTA button (or status badges), freeing vertical space in the card body for legible class details.
  3. **Full Modal Roster ([guest_reservation_form.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/guest_reservation_form.html))**: Clicking any calendar card opens the reservation modal featuring a dedicated "Confirmed Attendees" roster section with initials badges (`PublicInitials`), privacy names (`PublicDisplayName`), and rank/role chips.
