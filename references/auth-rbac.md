# Authentication & Multi-Role RBAC Architecture

### Context: Multi-Role Session Auth & Template Context Isolation

* **Problem**: 
  1. In Go's `html/template`, accessing `.CurrentUser` directly in `layout.html` when pages pass disparate view data models (e.g., `DashboardViewData`, `StudentsPageData`) causes template execution errors (`can't evaluate field CurrentUser in type ...`).
  2. Multi-role authorization requires rigid route guarding across browser page views and REST/HTMX endpoints (returning 401/403 JSON for API calls instead of redirect loops).
* **Enforced Solution**:
  1. **Reflective `currentUser` Template Function**: Registered `"currentUser"` in `template.FuncMap` that inspects view data using reflection to extract `.CurrentUser` or map key `"CurrentUser"` safely, returning `nil` if absent without throwing template execution errors.
  2. **Multi-Role RBAC Route Guards**:
     - `RequireAuth`: Validates 7-day HTTP-only `stms_session` cookie; redirects web visitors to `/login?redirect=...` while serving 401 JSON for `/api/*` and HTMX requests.
     - `RequireRole(roles ...UserRole)`: Enforces role boundaries (`STUDENT`, `COACH`, `ADMIN`, `OPERATION_MANAGER`); redirects mismatched users to their authorized view.
  3. **Role-Linked Portals**:
     - **Student Portal (`/portal/student`)**: Readiness HUD, 6-pillar SVG spider matrix, prepaid credit balance, and scheduled mat check-in.
     - **Coach Portal (`/portal/coach`)**: Live session roster, rapid mat check-in with auto-credit decrement, dynamic athletic score quick-entry, priority review queue (`READY` 🟢 / `PRE-TEST ELIGIBLE` 🟡), and First Aid incident logging (`has_safety_flag = true`).
     - **Admin Portal (`/portal/admin`)**: Operations telemetry, promotion pipeline oversight with 1-click belt advancements, First Aid ticket clearance queue, and timetable schedule generator.

### Context: 4-Tier Role Visibility & Access Matrix (Operation Manager / Admin / Coach / Student)

* **Problem**: 
  1. Default access permissions exposed management dashboards and rates to unauthorized roles, and did not reflect the floor operational structure where coaches only require floor rosters and students must only see their own records.
  2. Administrative staff require package viewing and assignment capabilities without having full operational overrides (promotions, timetable creation, safety ticket clearances).
* **Enforced Solution**:
  1. **Strict 4-Role RBAC Model**:
     - **`OPERATION_MANAGER` (Full Control)**: Full oversight and write permissions over Dashboard (`/`), Students (`/students`), Attendance/Sessions (`/sessions`), Coaches & Payroll (`/coaches`), Packages & Templates (`/packages`), Master Operations Portal (`/portal/admin`), User Registration, Belt Promotions, and Incident Resolution.
     - **`ADMIN` (Membership Packages Only)**: Restricted strictly to viewing membership packages and assigning packages to students (`/packages`, `POST /packages/assign`). All other endpoints redirect to `/packages`.
     - **`COACH` (Students & Attendance Only)**: Permitted to view student directory (`/students`), student details (`/students/{id}`), submit coach evaluations (`POST /students/{id}/evaluations`), attendance and floor live check-ins (`/sessions`, `/sessions/{id}/live`), and coach portal (`/portal/coach`). Excluded from Dashboard (`/`), Coaches (`/coaches`), and Packages (`/packages`).
     - **`STUDENT` (Own Profile Only)**: Permitted strictly to view their personal profile (`/portal/student` or `/students/{own_student_id}`). Attempting to view other students' profiles or administrative routes automatically redirects to `/portal/student`.
  2. **Navigation Bar Conditional Filtering**:
     - [`layout.html`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html) conditionally renders only authorized navigation links per role (e.g. Memberships for Admin; Students & Attendance for Coach; My Profile for Student; All links for Operation Manager).
