# UI & Operations Terminology Standards

This document specifies the canonical terminology standard for user-facing UI labels, navigation menus, badges, buttons, and customer-facing reports across the Series Taekwondo Management System (STMS).

---

## 1. Core Terminology Mappings

To maintain operational clarity for students, parents, coaches, and front-desk staff while ensuring 100% backward-compatibility with backend databases, APIs, and Go domain models, the following presentation-layer mapping is enforced:

| Operational Concept | Canonical UI Term | Prohibited / Legacy UI Terms | Underlying Backend / DB Mapping (Intact) |
| :--- | :--- | :--- | :--- |
| **Scheduled Class** | **Class / Training Class** | Session, Training Session, Floor Block | `training_sessions` table, `models.Session`, `/sessions` routes |
| **Attendance Check-In** | **Attendance / Open Attendance** | Floor Check-In, Mat Check-In, Admittance | `attendance` table, `/sessions/{id}/checkin` endpoints |
| **Passes & Passes Roster**| **Memberships / Membership Plans**| Package, Pass, Passes & Billing, Package Pass | `package_templates`, `student_packages` tables, `models.Package` |
| **Session Units** | **Classes / Remaining Classes** | Sessions, Credits, Remaining Sessions | `student_packages.remaining_sessions`, `student_packages.total_sessions` |
| **Live Roster Screen** | **Live Attendance** | Live Floor Check-In, Floor Standby | `/sessions/{id}/live` view handler |
| **Currency Display** | **Philippine Peso (₱ / PHP)** | USD, US Dollar ($) | Float64 rates & prices (`RatePerSession`, `Price`, `CustomPrice`) |

---

## 2. Page & Navigation Conventions

- **Global Navigation (Header):**
  - Desktop: `Dashboard`, `Students`, `Attendance`, `Coaches`, `Memberships`
  - Mobile: `Dashboard`, `Students`, `Attendance`, `Coaches`, `Memberships`
- **Dashboard CTA & Stats:**
  - Hero Button: `⚡ Open Attendance`
  - Counter Metric: `Total Classes`
  - Floor Quick-Access: `Active Classes`
- **Memberships Screen:**
  - Header: `Memberships`
  - Primary Action: `💳 Assign Membership to Student`
  - Template Badge: `Membership Template`
  - Roster Table: `Active Student Memberships Roster`
- **Classes Screen:**
  - Header: `Training Classes & Floor Attendance Log`
  - Scheduling Modal: `Schedule Training Class`
  - Date Field: `Class Date`

---

## 3. Belt Progression Standards

### Context: PTA Belt Progression Hierarchy
- **Problem:** Belt ranking previously contained non-standard "Tag" suffixes and omitted brown belts, which conflicted with the official Philippine Taekwondo Association (PTA) curriculum.
- **Enforced Solution:** Use 12 sequential belt stages from White to 3rd Dan Black Belt.

| Progression Order | Belt Rank Name | Backend Constant | Default Session Req | Default Tenure Days |
| :--- | :--- | :--- | :--- | :--- |
| 1 | **White** | `BeltWhite` | 16 | 45 |
| 2 | **Low Yellow** | `BeltLowYellow` | 20 | 60 |
| 3 | **High Yellow** | `BeltHighYellow` | 24 | 60 |
| 4 | **Low Blue** | `BeltLowBlue` | 28 | 75 |
| 5 | **High Blue** | `BeltHighBlue` | 32 | 90 |
| 6 | **Low Red** | `BeltLowRed` | 36 | 105 |
| 7 | **High Red** | `BeltHighRed` | 40 | 120 |
| 8 | **Low Brown** | `BeltLowBrown` | 44 | 135 |
| 9 | **High Brown** | `BeltHighBrown` | 48 | 150 |
| 10 | **1st Dan Black** | `BeltBlack1stDan` | 60 | 180 |
| 11 | **2nd Dan Black** | `BeltBlack2ndDan` | 72 | 240 |
| 12 | **3rd Dan Black** | `BeltBlack3rdDan` | 84 | 365 |

---

## 4. Membership Plan Types & Cadence Rules

### Context: 4-Week Consumable Membership Plans
- **Problem:** Dojangs offer strictly consumable monthly passes (e.g. 4 classes = 1x/week, 8 classes = 2x/week, 12 classes = 3x/week) that must not roll over beyond 28 days and must enforce weekly cadence quotas during floor attendance check-in.
- **Enforced Solution:**
  - `plan_type` attribute with values: `'standard'`, `'four_week'`, `'unlimited'`.
  - Fixed 28-day validity for all `four_week` templates and student packages.
  - Calculated weekly cadence quota: $\text{Quota} = \max(1, \lfloor \text{Total Sessions} / 4 \rfloor)$.
  - 7-day rolling cycle boundaries calculated relative to `PurchaseDate`: `[cycleStart, cycleEnd)`.
  - Floor check-in enforcement: Rejects check-in attempts when weekly quota is reached, indicating cycle reset date, while allowing one-click coach/admin manual override (`?override=true`).

---

## 5. Dashboard Operations Standards

### Context: Dashboard Active Classes Floor Filter
- **Problem**: The dashboard's "Active Classes" floor quick-links panel previously rendered all sessions indiscriminately, including historical/concluded sessions (`🔒 CLOSED`) and cancelled sessions (`🚫 CANCELLED`), cluttering operational quick-check-in.
- **Enforced Solution**:
  - **Domain Model**: [TrainingSession.IsOpen()](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/session.go) and [TrainingSession.IsOpenAt()](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/session.go) evaluate `!s.IsCancelled && !s.IsPastEndTime()`.
  - **Handler Filter**: [HandleDashboard](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/handlers/dashboard_handler.go) populates `RecentSessions` strictly with sessions satisfying `s.IsOpen()`.
  - **UI Representation**: [web/templates/pages/dashboard.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/dashboard.html) badges each item as `Open`, updates the header telemetry to `N Open`, and provides an empty-state card ("No open classes right now") when the floor has no pending or live classes.

---

## 6. Contact Number Format Standards

### Context: 11-Digit Mobile Format
- **Problem**: Forms previously used inconsistent, legacy international placeholder formats (e.g. `+1 (555) 000-0000`, `+1-555-0100`) and allowed free-form text input that complicated quick front-desk and floor safety communications.
- **Enforced Solution**:
  - **Standard Format**: 11-digit mobile number starting with `09` (e.g., `09626914130`).
  - **Input Attributes**:
    - `type="tel"`
    - `name="phone"` / `name="emergency_phone"`
    - `placeholder="09626914130"`
    - `pattern="09[0-9]{9}"`
    - `maxlength="11"`
    - `inputmode="numeric"`
    - `oninput="this.value = this.value.replace(/\D/g, '').slice(0, 11)"`
    - Helper microcopy: `11 digits starting with 09 (e.g. 09626914130)`
  - **Applied Screens**:
    - Coach Public Registration (`/register`)
    - Coach Directory Modal (`/coaches`)
    - Student Enrollment Modal (`/students`)
---

## 7. Class Scheduling Navigation Flow

### Context: Post-Schedule Calendar View Retention
- **Problem**: When staff or coaches scheduled a class from the 7-day calendar, `HandleCreateSession` automatically redirected to the floor check-in view (`/sessions/{id}/live`), interrupting schedule planning and kicking the user away from the calendar view.
- **Enforced Solution**:
  - **Handler Navigation**: [HandleCreateSession](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/handlers/session_handler.go) and [HandleUpdateSession](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/handlers/session_handler.go) default to `/sessions?date=YYYY-MM-DD` matching the scheduled class date, or honor `redirect_url` when explicitly specified.
  - **HTMX Support**: Evaluates `HX-Request` header and responds with `HX-Redirect` when requested via hypermedia.
  - **UI Button & Modals**: Scheduling modal CTA is labeled `Schedule Class` (never `Schedule & Launch Attendance`), with dynamic `redirect_url` parameters updated on date selection to ensure the user stays focused on that class's week in the calendar view.

---

## 8. Athletic Evaluation Radar Standards

### Context: 3-Axis Athletic Radar Metrics
- **Problem:** The athletic ability evaluation radar previously required 6 crowded metrics (Flexibility, Stamina, Power, Technique, Sparring IQ, Discipline) which overburdened floor coaches during quick evaluations and cluttered student profile visualizations.
- **Enforced Solution:**
  - **Triad Metrics:** Reduced to exactly 3 core martial arts axes:
    1. **Sparring** (Apex / Top, angle: $-90^\circ / -\frac{\pi}{2}$)
    2. **Flexibility** (Bottom-Right, angle: $30^\circ / \frac{\pi}{6}$)
    3. **Poomsae** (Bottom-Left, angle: $150^\circ / \frac{5\pi}{6}$)
  - **Domain Model:** [StudentEvaluation](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/evaluation.go) with `Sparring`, `Flexibility`, `Poomsae`.
  - **Backward Compatibility:** `GetSparring()`, `GetFlexibility()`, `GetPoomsae()`, and `SyncLegacyFields()` ensure existing records, SQL tables, and tests preserve full interoperability.
  - **Polygon Math:** Equilateral triangle concentric grids at scores 10, 8, 6, 4, 2 with [ToSVGPolygon](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/evaluation.go) generating 3-point SVG polygon coordinates.

---

## 9. Interactive Calendar Drag-and-Drop Rescheduling

### Context: Floor Calendar Drag-and-Drop Operations
- **Problem**: Moving sessions, duty shifts, or events required opening edit dialogs and manually typing new dates and times, slowing down front-desk floor operations.
- **Enforced Solution**:
  - **Native Drag & Drop**: Session cards feature `draggable="true"` (except when cancelled), visual grab cursors (`cursor-grab active:cursor-grabbing`), and drag-state opacity highlights.
  - **Drop Zones**: Both 30-minute time slot buttons (`:00` and `:30`) and day column headers serve as active drop targets with interactive dragover styling (`!bg-rose-100 ring-2 ring-[#990303]`).
  - **Duration Preservation**: Dropping on a time slot automatically preserves the original duration (`newEnd = newStart + duration`), while dropping onto a day header keeps the existing time window and shifts only the date.
  - **Confirmation Dialog**: Dropping opens a streamlined `#reschedule-session-modal` displaying original vs new schedules with Enter-to-confirm keyboard accessibility.
  - **Server Endpoint**: [HandleRescheduleSession](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/handlers/session_handler.go) mounted at `POST /sessions/{id}/reschedule` securely updates only date and times while leaving assigned staff, coaches, locations, rates, and student attendances intact. Supports both `HX-Redirect` and standard HTTP 303 redirects.
