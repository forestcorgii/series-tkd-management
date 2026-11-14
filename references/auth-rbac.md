# Authentication & Multi-Role RBAC Architecture

## 1. Two-Tier User Model & RBAC Matrix
STMS uses a **Two-Tier Primary User Model** (`STUDENT` vs `STAFF`) with customizable [`StaffRole`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/user.go) permissions (`roles:manage`, `students:*`, `sessions:*`, `packages:*`, `evaluations:*`, etc.) and 4-role helper compatibility (`IsOperationManager()`, `IsAdmin()`, `IsCoach()`, `IsStudent()`).

| Role / Helper | Primary Tier | `GET /` Behavior | `/dashboard` Template | Authorized Routes & Key Boundaries |
| :--- | :--- | :--- | :--- | :--- |
| **Public / Guest** (`user == nil`) | Unauthenticated | Renders `landing.html` | Redirects to `/login` | `GET /`, `GET /landing/schedule`, `POST /landing/reserve`, `/login`, `/register` (`/registration`), `/forgot-password`, `/reset-password`. |
| **`OPERATION_MANAGER`** (`IsOperationManager()`) | `STAFF` (`manager` / `*`) | Redirects to `/dashboard` (unless `?preview=1`) | `dashboard.html` (`?view=coach` / `?view=admin` previews) | Full access (`*`): `/dashboard`, `/students` (incl. belt edits, credentials & cascade delete), `/sessions` (incl. closed class delete), `/coaches` (approve, deactivate, cascade delete), `/packages` (incl. revoke), `/locations` (CRUD), `/admins`, `/roles`, `/logs`, `/portal/admin`, `/settings`. |
| **`ADMIN`** (`IsAdmin()`) | `STAFF` (`admin`) | Redirects to `/sessions` | `admin_dashboard.html` | `/dashboard`, `/sessions`, `/packages` (view & assign), `/students` (register & edit personal/emergency info **ONLY**—belt rank, credentials & evals forbidden), `/locations` (view-only), `/settings`. Barred from `/coaches`, `/admins`, `/roles`, `/logs`, `/portal/admin`. |
| **`COACH`** (`IsCoach()`) | `STAFF` (`coach`) | Redirects to `/sessions` | `coach_dashboard.html` (scoped to `IsCoachSchedule()`) | `/dashboard`, `/sessions` (auto-prefiltered to own `coach_id` on initial load when `?coach_id` is absent), `/students` (view & `POST /students/{id}/evaluations`), `/portal/coach`, `/locations` (view-only), `/settings` (view). Barred from `/packages`, `/coaches`, `/admins`, `/roles`, `/logs`. |
| **`STUDENT`** (`IsStudent()`) | `STUDENT` | Renders `landing.html` (default schedule view) | Redirects to `/portal/student` | `/` (calendar + 1-click slot reservation), `/portal/student`, `/students/{own_student_id}`, `POST /api/student/check-in` (identity-locked to `user.StudentID`), `/profile`. Redirected to `/` on unauthorized routes. |

---

## 2. Session Auth, Guards & Template Isolation
- **HTTP-Only Cookie**: `stms_session` (7-day expiry, `Secure: IsHTTPS(r)`).
- **Middleware (`RequireAuth`, `RequireRole`, `RequirePermission`)**:
  - Returns `401 Unauthorized` / `403 Forbidden` JSON for `/api/*`, `Accept: application/json`, `Content-Type: application/json`, or `HX-Request: true`.
  - Returns `303 See Other` redirects to `/login?redirect=...` or `user.RoleDashboardURL()` for browser navigation.
- **Template `currentUser` Helper**: Always extract the user in `layout.html` and partials via `{{$user := currentUser .}}` (reflective `FuncMap` helper) rather than `.CurrentUser` directly.
- **Role Enrichment**: `GetUserBySessionToken`, `GetUserByEmail`, `GetUserByID`, and `GetUserByIdentifier` populate `user.Roles` and `user.Permissions` in both `MemoryStore` and `SQLStore`.

---

## 3. Authentication, Registration & Recovery Flows
- **Dual-Identifier Login (`GetUserByIdentifier`)**: Accepts `email` or `username` (`LOWER(email) = LOWER($1) OR LOWER(username) = LOWER($1)`) via `name="identifier"`.
- **Production Demo Gating (`IsDemoLoginEnabled`)**: `login.html` gates the quick-demo panel and `fillDemo(...)` behind `{{if .ShowDemoLogin}}`, automatically disabled in production (`APP_ENV`/`ENV`/`GO_ENV`/`ENVIRONMENT=production`, `RAILWAY_ENVIRONMENT`, or Postgres `DATABASE_URL`) unless `SHOW_DEMO_LOGIN=true`.
- **Anti-Enumeration Password Recovery**:
  - `POST /forgot-password` and `POST /api/auth/forgot-password` always return the identical generic notice regardless of account existence.
  - `password_reset_tokens`: 32-byte hex token, 1-hour expiry, single-use (`MarkPasswordResetTokenUsed`).
- **Staff Self-Registration (`/register` & `/registration` aliases)**:
  - Public signup for `COACH` or `ADMIN` creates inactive accounts (`is_active = false`, `LastLoginAt == nil`).
  - `AuthService.Login` returns `ErrAccountPendingApproval` until approved by an Operation Manager on `/admins` or `/coaches` (surfaced on `/portal/admin`).

---

## 4. Governance, Cascades & Endpoint Boundaries
- **Self-Service Profile (`/profile`, `RequireAuth`)**: Students edit phone, gender, and emergency/medical notes; Coaches edit phone and specialties; Admins/Managers edit display name and password.
- **Student Governance (`/students`)**:
  - **Admin Boundary**: `HandleUpdateStudent` allows Admins to edit personal/contact/emergency fields only; `current_belt`, promotion dates, and `email`/`username` credential updates are ignored unless `IsOperationManager()`.
  - **Manager Credential Reset**: `POST /students/{id}/reset-password` and `POST /api/students/{id}/reset-password` (`RequireRole(RoleOperationManager)`) update password hash and log `STUDENT_RESET_PASSWORD`. `UpdateUser` syncs normalized lowercase `email` in both stores.
  - **Cascade Delete (`DeleteStudent`, `RequireRole(RoleOperationManager)`)**: Atomically purges `users`, `user_sessions`, `password_reset_tokens`, `safety_incidents`, `student_evaluations`, `attendance`, `student_packages`, and `students`.
- **Coach Governance (`/coaches`, `RequireRole(RoleOperationManager)`)**:
  - **Deactivation**: `AuthService.Login` and `ValidateSession` check both `user.IsActive` and `coach.IsActive`; `ToggleCoachActive(false)` revokes all active `user_sessions`.
  - **Cascade Delete (`DeleteCoach`)**: Purges `users`, `user_sessions`, `safety_incidents`, `student_evaluations`, nullifies `training_sessions.admin_id`, deletes coach `attendance` + `training_sessions`, and deletes `coaches`.
- **Admin & Role Governance (`/admins`, `/roles`)**:
  - `GET/POST /admins`, `POST/PATCH /admins/{id}/toggle`, `POST/PUT /admins/{id}/reset-password` (`RequireRole(RoleOperationManager)`) support JSON, HTMX, and `303` redirects; deactivation immediately revokes `user_sessions`.
  - `/roles` (`RequirePermission(PermRolesManage)`) manages custom `StaffRole`s; system slugs (`coach`, `admin`, `manager`) cannot be deleted or renamed.
- **Sessions, Packages, Locations & Audit Logs**:
  - **Closed Class Delete**: `HandleDeleteSession` allows `!sess.IsOpen()` deletion only for `RoleOperationManager`.
  - **Package Revoke**: `POST /packages/student-packages/{id}/revoke` (`RequireRole(RoleOperationManager)`) sets `payment_status = 'revoked'`, blocking check-in.
  - **Locations**: `GET /locations` is readable by `Coach`, `Admin`, `OperationManager`; all mutations require `RoleOperationManager`.
  - **Audit Logs (`/logs`, `/api/logs`, `RequireRole(RoleOperationManager)`)**: `AppHandler.LogAction(...)` records actor, IP (`X-Forwarded-For`), User-Agent, and JSON metadata; exposed in profile submenu/drawer for managers only.
- **Student Self Check-In & Public Landing**:
  - `POST /api/student/check-in` (`RequireRole(RoleStudent, RoleCoach, RoleOperationManager)`) locks `student_id` to `user.StudentID` for students, enforces safety hold & credit checks, is idempotent, and emits `HX-Trigger: attendanceUpdated` to sync `#student-header`, `#readiness-hud`, and `#wallet-ledger-card`.
  - `POST /landing/reserve` allows authenticated students to reserve open slots with 1 click (omitting guest name/phone inputs) while enforcing `MaxTrainees`.
