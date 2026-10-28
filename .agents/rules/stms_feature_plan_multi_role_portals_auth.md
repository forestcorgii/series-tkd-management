---
trigger: model_decision
description: When working on authentication, authorization, role-based access control (RBAC), user portals, or session security.
---

# STMS Multi-Role Portals & RBAC Quick Rules

For full endpoint matrices and historical context, read [`references/auth-rbac.md`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/references/auth-rbac.md).

## 1. Role Hierarchy & Access Model
STMS uses a **Two-Tier Primary User Model** (`STUDENT` and `STAFF`) with customizable `StaffRole` permissions and full 4-role backward compatibility:

| Role / Helper | Default Landing (`GET /` redirect) | Dashboard Route | Core Authorized Scope |
| :--- | :--- | :--- | :--- |
| **`OPERATION_MANAGER`** (`IsOperationManager()`) | `/dashboard` | `/dashboard` (`dashboard.html`) | Full control (`*`): all routes, `/coaches`, `/admins`, `/roles`, `/logs`, `/portal/admin`, belt promotions, safety clearances, cascade deletions. |
| **`ADMIN`** (`IsAdmin()`) | `/sessions` | `/dashboard` (`admin_dashboard.html`) | `/sessions` (attendance & scheduling), `/packages` (view & assign), `/students` (register & edit contact/emergency info ONLY—no belt rank/eval edits), `/locations` (view-only), `/settings`. |
| **`COACH`** (`IsCoach()`) | `/sessions` | `/dashboard` (`coach_dashboard.html`) | `/sessions` (pre-filtered to own `coach_id` on initial load), `/students` (view & submit evaluations), `/portal/coach`, `/locations` (view-only), `/settings` (view). |
| **`STUDENT`** (`IsStudent()`) | `/portal/student` | `/portal/student` (`student_portal.html`) | Own profile only (`/portal/student`, `/students/{own_id}`, `POST /api/student/check-in`). Redirected to `/portal/student` on unauthorized routes. |

## 2. Critical Auth & Routing Invariants
1. **Public Landing vs Authenticated Workspace (`GET /`)**:
   - `GET /` serves `HandleLandingPage` publicly. If an authenticated user visits `/` without `?preview=1`, they redirect per the table above (`Student -> /portal/student`, `Coach/Admin -> /sessions`, `Manager -> /dashboard`).
2. **Route Registration Sync**:
   - Always register new HTTP routes in **both** [`main.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/main.go) and [`cmd/server/main.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/cmd/server/main.go).
3. **Dual-Format Auth Guards (`RequireAuth` / `RequireRole` / `RequirePermission`)**:
   - Return `401 Unauthorized` / `403 Forbidden` JSON for `/api/*`, `Accept: application/json`, or `HX-Request: true`, and `303 See Other` redirects for standard browser navigation.
4. **Template `.CurrentUser` Isolation**:
   - In `layout.html` and shared partials, always extract the current user via `{{$user := currentUser .}}` (reflective `template.FuncMap` helper) rather than `.CurrentUser` directly.
5. **Deactivation & Session Revocation**:
   - Deactivating or deleting a coach, admin, or student must immediately purge their active sessions in `user_sessions`.
