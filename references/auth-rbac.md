# Authentication & Multi-Role RBAC Architecture

### Context: Multi-Role Session Auth & Template Context Isolation

* **Problem**: 
  1. In Go's `html/template`, accessing `.CurrentUser` directly in `layout.html` when pages pass disparate view data models (e.g., `DashboardViewData`, `StudentsPageData`) causes template execution errors (`can't evaluate field CurrentUser in type ...`).
  2. Multi-role authorization requires rigid route guarding across browser page views and REST/HTMX endpoints (returning 401/403 JSON for API calls instead of redirect loops).
* **Enforced Solution**:
  1. **Reflective `currentUser` Template Function**: Registered `"currentUser"` in `template.FuncMap` that inspects view data using reflection to extract `.CurrentUser` or map key `"CurrentUser"` safely, returning `nil` if absent without throwing template execution errors.
  2. **Multi-Role RBAC Route Guards**:
     - `RequireAuth`: Validates 7-day HTTP-only `stms_session` cookie; redirects web visitors to `/login?redirect=...` while serving 401 JSON for `/api/*` and HTMX requests.
     - `RequireRole(roles ...UserRole)`: Enforces role boundaries (`STUDENT`, `COACH`, `ADMIN`); redirects mismatched users to their designated portal (`/portal/student`, `/portal/coach`, `/portal/admin`).
  3. **Role-Linked Portals**:
     - **Student Portal (`/portal/student`)**: Readiness HUD, 6-pillar SVG spider matrix, prepaid credit balance, and scheduled mat check-in.
     - **Coach Portal (`/portal/coach`)**: Live session roster, rapid mat check-in with auto-credit decrement, dynamic athletic score quick-entry, priority review queue (`READY` 🟢 / `PRE-TEST ELIGIBLE` 🟡), and First Aid incident logging (`has_safety_flag = true`).
     - **Admin Portal (`/portal/admin`)**: Operations telemetry, promotion pipeline oversight with 1-click belt advancements, First Aid ticket clearance queue, and timetable schedule generator.
