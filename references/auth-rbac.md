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
     - **`ADMIN` (Attendance, Membership Packages & Student Contact Info)**: Permitted to manage floor attendance check-ins (`/sessions`, `/sessions/{id}/live`, `POST /sessions/{id}/checkin/{student_id}`, `POST /api/coach/check-in`), view and assign membership packages (`/packages`, `POST /packages/assign`), and access the Student Directory (`/students`, `/students/{id}`). Within the Student Directory, Admins are permitted to register new students (`POST /students`) and edit their personal and emergency medical info (`POST /students/{id}`), but are strictly forbidden from modifying belt ranks, promotion dates, or submitting athletic coach evaluations. Excluded from Dashboard (`/`), Coaches (`/coaches`), and Operations Portal (`/portal/admin`).
     - **`COACH` (Students & Attendance Only)**: Permitted to view student directory (`/students`), student details (`/students/{id}`), submit coach evaluations (`POST /students/{id}/evaluations`), attendance and floor live check-ins (`/sessions`, `/sessions/{id}/live`), and coach portal (`/portal/coach`). Excluded from Dashboard (`/`), Coaches (`/coaches`), and Packages (`/packages`).
     - **`STUDENT` (Own Profile Only)**: Permitted strictly to view their personal profile (`/portal/student` or `/students/{own_student_id}`). Attempting to view other students' profiles or administrative routes automatically redirects to `/portal/student`.
  2. **Navigation Bar Conditional Filtering**:
     - [`layout.html`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html) conditionally renders authorized navigation links per role (e.g. Students, Attendance, Memberships for Admin; Students & Attendance for Coach; All links for Operation Manager).
     - **Students View Simplification**: The student role has its center navbar menu links cleared completely. Students navigate to their Practitioner Portal via the brand logo and access profile settings via the user indicator pill.

### Context: Self-Service Multi-Role User Profile & Navbar Simplification (`/profile`)

* **Problem**: 
  1. Practitioners, instructors, and administrators lacked a direct self-service interface to update personal contact info and safety-critical emergency medical notes without contacting a database administrator.
  2. The navigation header had redundant links ("My Profile") for student users whose sole home interface is the Practitioner Portal.
* **Enforced Solution**:
  1. **Branding & Dynamic Role Indicator**:
     - Brand title standard is **"SERIES TAEKWONDO"**.
     - Dynamic role badge reflects authenticated role: `Students`, `Coach`, `Admin`, or `Manager`.
     - Logo click routes directly to the role's home view (`/portal/student` for students, `/portal/coach` for coaches, `/packages` for admins, `/` for managers).
  2. **Dedicated Profile Route (`/profile`)**:
     - Protected by `RequireAuth`.
     - Accessible via the user profile pill directly adjacent to the Sign Out button.
     - **Student Profile**: Permits editing phone, gender, and floor safety-critical emergency details (`emergency_name`, `emergency_phone`, `emergency_relation`, `medical_notes`).
     - **Coach Profile**: Permits editing instructor phone number and coaching specialties list.
     - **Admin & Manager Profile**: Permits updating account display name and password credentials.

### Context: Admin Student Management Boundaries (Personal & Emergency Info Only)

* **Problem**:
  1. Administrative staff needed access to manage student rosters and maintain accurate emergency contact and personal details for floor safety without granting them authority to alter athletic belt ranks or bypass coach-led evaluations.
* **Enforced Solution**:
  1. **Route Guarding**:
     - `GET /students`, `GET /students/{id}`, `POST /students`, `POST /students/{id}` permit `models.RoleAdmin`.
     - `POST /students/{id}/evaluations` strictly restricted to `models.RoleCoach` and `models.RoleOperationManager`.
  2. **Server-Side Field Isolation**:
     - `HandleUpdateStudent` allows Admins to update `full_name`, `dob`, `gender`, `phone`, `emergency_name`, `emergency_phone`, `emergency_relation`, and `medical_notes`.
     - Requests attempting to alter `current_belt` or promotion timestamps are strictly ignored unless executed by an `OperationManager`.
  3. **UI Adaptation**:
     - In `students.html`: "➕ Register New Student" button rendered for both Admin and Operation Manager.
     - In `student_detail.html`: "✏️ Edit Student Info" modal rendered for Admin and Operation Manager; the `current_belt` input is rendered as a read-only badge for Admin. Coach evaluation forms remain hidden from Admin.

### Context: Administrator Directory & Access Governance (`/admins`)

* **Problem**:
  1. The Operations Manager lacked a dedicated interface to oversee front-desk administrator accounts, track their active/inactive status, audit last logins, provision new administrator staff credentials, and reset administrator passwords.
  2. Unauthorized roles (Admin, Coach, Student) must be strictly forbidden from accessing administrator governance tools.
* **Enforced Solution**:
  1. **Dedicated Route Guarding**:
     - `GET /admins`, `POST /admins`, `POST /admins/{id}/toggle`, `POST /admins/{id}/reset-password` are protected strictly with `RequireRole(models.RoleOperationManager)`.
     - Non-manager roles navigating to `/admins` are automatically redirected to their respective home views, or returned HTTP 403 Forbidden for API/HTMX requests.
  2. **Persistence & Operations**:
     - `users` table schema guarantees `display_name` column persistence across SQLite and PostgreSQL.
     - `GetUsersByRole(models.RoleAdmin)` and `ToggleUserActive(id, isActive)` methods added to `RepositoryStore`.
  3. **Interactive UI (`admins.html`)**:
     - Displays telemetry stats: Total Administrators, Active Staff (with live pulse indicator), Inactive Accounts, and Front-Desk Scope reminder.
     - Renders administrators roster with name, avatar, email, status badge, last login time in Philippine Time (`January 02, 2006 03:04 PM`), and created date.
     - Modals for **Provision Administrator** and **Reset Password**.
     - Navigation links added to `layout.html` for `IsOperationManager` on desktop and mobile navbars.

### Context: Coach Deactivation, Deletion & Session Invalidation Governance (`/coaches`)

* **Problem**:
  1. The dojang lacked controls to deactivate or delete coach accounts. When coach contracts expire or relationships terminate, coaches must be immediately barred from accessing the coach portal or logging in.
  2. Simply deleting coaches who have conducted classes or submitted student evaluations causes foreign key constraint violations and corrupts historical attendance/payroll audit trails.
* **Enforced Solution**:
  1. **Dual-Layer Login & Session Guarding**:
     - `AuthService.Login` and `AuthService.ValidateSession` verify both `user.IsActive` and the linked coach's `coach.IsActive`. If either is false, returns `ErrUserInactive` ("Account is inactive. Please contact your dojang administrator").
     - On deactivation (`ToggleCoachActive`), all active user sessions in `user_sessions` for the coach are immediately revoked/deleted, forcing instant logout on their next request.
  2. **Preservation-First Coach Deletion**:
     - `DeleteCoach` first queries `training_sessions`, `student_evaluations`, and `safety_incidents` for historical activity.
     - If records exist, hard deletion is blocked with `ErrCoachHasRecords`, prompting the Operations Manager to deactivate the coach instead to safeguard data integrity.
     - If zero historical records exist, the coach profile, associated `users` account, and session tokens are cleanly purged.
  3. **Operational UI & Telemetry (`coaches.html`)**:
     - Telemetry stats banner: Total Coaches, Active Instructors (live pulse indicator), Deactivated accounts, and First Aid Certified count.
     - Each coach card features an Active / Inactive status pill, deactivation warning notice, and quick actions:
       - **Deactivate / Reactivate**: Toggles access with confirmation dialog.
       - **Delete**: Permanently removes unused coach profiles with confirmation.
     - Timetable generator, student evaluation, and new session modals filter out deactivated coaches (`{{if .IsActive}}`).



