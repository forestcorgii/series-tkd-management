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
     - Logo click routes directly to the role's home view (`/portal/student` for students, `/sessions` for coaches and admins, `/` for managers).
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
  2. **Transactional Cascading Coach Deletion**:
     - `DeleteCoach` performs a transactional cascade:
       - Purges linked `users` accounts and active `user_sessions`.
       - Removes `safety_incidents` associated with the coach.
       - Removes `student_evaluations` authored by the coach.
       - Nullifies supervising `admin_id` in training sessions.
       - Cascades and deletes `attendance` records for sessions led by the coach, then removes the sessions.
       - Finally removes the `coaches` record itself.
  3. **Operational UI & Interactive Listview (`coaches.html`)**:
     - Modern responsive **listview table** replacing the legacy card grid for high-density front-desk and manager usability.
     - Live search filter by instructor name, belt rank, contact number, email, or specialty.
     - Telemetry stats banner: Total Coaches, Active Instructors, Pending Approval, Deactivated, and First Aid Certified.
     - **Click-to-View Coach Info Modal**: Clicking any coach row (or the "Info" action button) opens an interactive modal detailing their qualifications, contact information, First Aid status, coaching specialties, session counts, and monthly floor payouts.
     - Actions: One-click status toggling (`Approve Coach`, `Deactivate`, `Reactivate`) and `🗑️ Delete Coach (Cascade)` with explicit confirmation.

### Context: Username Authentication & Anti-Enumeration Self-Service Password Recovery

* **Problem**:
  1. Practitioners, coaches, and administrators could previously only authenticate using email addresses, which created friction for floor check-in tablets and staff who preferred shorter handle identifiers.
  2. Users who forgot their password had no self-service recovery mechanism, requiring manual administrative intervention in the database or management console.
  3. Reset mechanisms can inadvertently expose account existence (user enumeration) if error messages reveal whether an identifier exists.
* **Enforced Solution**:
  1. **Dual-Identifier Authentication (`GetUserByIdentifier`)**:
     - `models.User` struct persists an optional unique `username` (case-insensitive, alphanumeric + `._-`).
     - `AuthService.Login` and `HandleLoginSubmit` accept either `username` or `email` interchangeably using case-insensitive matching (`LOWER(email) = LOWER($1) OR LOWER(username) = LOWER($1)`).
     - Form input is standard `name="identifier"`, with fallbacks to `email` and `username` for backward compatibility.
  2. **Anti-Enumeration Password Recovery**:
     - `POST /forgot-password` and `POST /api/auth/forgot-password` invoke `AuthService.RequestPasswordReset(identifier)`.
     - If the account does not exist or is inactive, the handler still returns the identical generic confirmation notice (*"If an account with that email or username exists, instructions have been sent to reset your password."*), completely preventing user enumeration.
  3. **Single-Use Cryptographic Reset Tokens (`password_reset_tokens`)**:
     - 32-byte hex cryptographically random token (`GenerateSecureToken()`) with a 1-hour expiration.
     - Tokens are validated upon access via `GET /reset-password?token=...` and invalidated immediately upon successful password change via `MarkPasswordResetTokenUsed(token)`.
     - In development/local environments without an external SMTP gateway, generated reset URLs are output to the server console log for verification and auditing.


### Context: Administrator Management HTMX & JSON API Endpoints (/admins)

* **Problem**:
  1. The /admins endpoints only accepted standard HTML form posts and performed full-page redirects with query parameters (?success=..., ?error=...), breaking compatibility with headless API consumers and HTMX dynamic modal / partial updates.
  2. Deactivating front-desk administrators did not invalidate active user session tokens, allowing deactivated administrators to continue accessing authorized resources until session expiry.
* **Enforced Solution**:
  1. **Dual-Format Request & Response Dispatching**:
     - Endpoints inspect Accept: application/json, Content-Type: application/json, ?format=json, and HX-Request: true.
     - **GET /admins & GET /api/admins**: Returns JSON telemetry summary (	otal, ctive, inactive, dmins array) for API clients; renders dmins.html for browser navigation.
     - **POST /admins & POST /api/admins**: Accepts either JSON body or URL-encoded form data. Returns 201 Created JSON with created admin entity for API calls, HTML alert banner for HTMX, or 303 redirect with query notice for web forms. Handles duplicate emails (409 Conflict) and validation errors (400 Bad Request) cleanly across formats.
     - **POST/PATCH /admins/{id}/toggle & POST/PATCH /api/admins/{id}/toggle**: Toggles active status and immediately invalidates all active session tokens in user_sessions upon deactivation. Returns 200 OK JSON or HTMX status banner.
     - **POST/PUT /admins/{id}/reset-password & POST/PUT /api/admins/{id}/reset-password**: Accepts JSON or form data, hashes new password, and responds with 200 OK JSON, HTMX banner, or 303 redirect.
  2. **Auth Guard Compatibility**:
     - RequireAuth and RequireRole check for Accept: application/json or Content-Type: application/json, returning HTTP 401 Unauthorized or 403 Forbidden JSON payloads instead of redirect loops.

### Context: Production Demo Login Gating (/login)

* **Problem**:
  1. The login view featured a 1-click "Quick Demo Sign-In" role switcher exposing pre-filled demo credentials and passwords in client-side HTML/JavaScript.
  2. While convenient for local development and QA, displaying demo logins and credentials in production environments compromises security hygiene and professional presentation.
* **Enforced Solution**:
  1. **Environment-Aware Demo Detection (`IsDemoLoginEnabled`)**:
     - Automatically disables the demo panel when running in production: `APP_ENV=production`, `ENV=production`, `GO_ENV=production`, `ENVIRONMENT=production`, or cloud platforms (`RAILWAY_ENVIRONMENT`).
     - Detects PostgreSQL production connection strings (`DATABASE_URL=postgres://...`) unless explicitly tagged with development flags.
     - Supports explicit opt-in/opt-out via `SHOW_DEMO_LOGIN` or `ENABLE_DEMO_LOGIN` (`"true"` / `"false"`).
  2. **Template & Script Isolation (`login.html`)**:
     - Both the quick demo button group and the client-side `fillDemo(...)` script containing hardcoded credentials are gated behind `{{if .ShowDemoLogin}}`. In production, no demo HTML elements or credentials scripts are rendered to the client browser.

### Context: Student Self Check-In & Reactive Scheduled Classes Floor Sync (`/api/student/check-in`)

* **Problem**:
  1. The "Class Check-In" action in the Practitioner Portal (`student_portal.html`) triggered `POST /api/coach/check-in`, which was guarded strictly by `RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)`. Authenticated `STUDENT` users received HTTP 403 Forbidden, preventing floor check-in.
  2. The schedule card had no state awareness for classes the practitioner was already admitted to, repeatedly showing an active check-in button even after admission.
  3. Successful admittance did not sync other attendance-dependent widgets (Readiness HUD progress bar, Remaining Classes in Prepaid Wallet, or Recent Attendance Log) without a full manual page refresh.
* **Enforced Solution**:
  1. **Dedicated Self Check-In Endpoint (`POST /api/student/check-in`)**:
     - Protected by `RequireRole(models.RoleStudent, models.RoleCoach, models.RoleOperationManager)`.
     - **Identity Lock**: For `STUDENT` users, the admitted `student_id` is strictly derived from the authenticated session context (`user.StudentID`), preventing practitioners from checking in other accounts.
     - **Safety & Membership Verification**: Blocks check-in if an active safety hold is present (`HasSafetyFlag == true`), if the class is cancelled, or if the student has no active valid membership credits.
     - **Idempotency**: Detects prior admittance to the same session and confirms check-in status cleanly without double deduction.
  2. **Partial Swapping & UI State (`student_session_item.html`)**:
     - Renders an emerald `✓ Checked In` pill when `IsCheckedIn == true`, `Cancelled` pill when `IsCancelled == true`, `Safety Hold` pill when flagged, or primary button `Class Check-In`.
     - Targets `#session-item-{{.Session.ID}}` with `hx-swap="outerHTML"`, replacing the card and displaying a dismissible feedback banner on check-in.
  3. **Event-Driven Multi-Widget Sync (`attendanceUpdated`)**:
     - Check-in emits header `HX-Trigger: attendanceUpdated`.
     - `#student-header`, `#readiness-hud`, and `#wallet-ledger-card` in `student_portal.html` declare `hx-trigger="attendanceUpdated from:body" hx-get="/portal/student" hx-select="..." hx-target="..." hx-swap="outerHTML"`, automatically updating attended class counts, readiness percentage bars, and wallet balances in real time.



### Context: Coach & Admin Self-Registration & Manager Approval Governance (/register)

* **Problem**:
  1. Instructor and front-desk administrator onboarding previously required an Operations Manager to manually provision all accounts in the database or administrative modals.
  2. Public sign-up was disabled on /register, redirecting to /login.
  3. When staff accounts are created, they must not have immediate access to sensitive student directories, payroll, or live attendance check-ins without explicit operational vetting and approval by the Operations Manager.
* **Enforced Solution**:
  1. **Public Staff Registration Flow (GET /register & POST /register)**:
     - Dynamic role switcher between **Coach / Instructor** (COACH) and **Dojang Administrator** (ADMIN).
     - Gathers account credentials (Full Name, Email, Username, Password with 6-char minimum and confirmation match) alongside role-specific qualifications (Phone, Dan Belt Rank, Teaching Specialties, and First Aid Certification for Coaches).
     - Persists new records with is_active = false (and for coaches, both coaches.is_active = false and users.is_active = false).
     - Redirects user to /login with an informational notice: *Registration submitted successfully! Your account is pending manager approval. You can log in once approved.*
  2. **Pending Approval Login Gating (ErrAccountPendingApproval)**:
     - AuthService.Login checks credentials and detects if !user.IsActive (or coach != nil && !coach.IsActive).
     - If user.LastLoginAt == nil, the account is recognized as an unapproved applicant and returns ErrAccountPendingApproval.
     - HandleLoginSubmit presents a dedicated notice: *Your account is pending manager approval. Please wait for an Operations Manager to review and approve your registration.*
  3. **Manager Approval & Rejection Controls**:
     - In **/admins** (Administrator Governance): Inactive accounts with LastLoginAt == nil are tagged with an amber ? Pending Approval badge, offering 1-click ? Approve (POST /admins/{id}/toggle) or ? Reject (POST /admins/{id}/delete).
     - In **/coaches** (Coach Directory): Unapproved coaches display an amber ? Pending Approval pill, card notice, and 1-click ? Approve Coach button.
     - In **/portal/admin** (Master Operations Portal): Displays a dynamic banner when pending staff registrations exist, linking directly to /coaches or /admins for rapid review.

  4. **Production Routing & URL Aliasing (/registration & /register)**:
     - Railway Railpack builds directly from repository root (`main.go`).
     - Both `main.go` and `cmd/server/main.go` register `GET /register`, `POST /register`, and seamless redirects/forwarding for `GET /registration`, `GET /registration/`, `GET /register/` (preserving query parameters like `?role=admin`), as well as `POST /registration` direct processing.

### Context: Student Deletion & Cascading Record Purge Governance (/students/{id}/delete)

* **Problem**:
  1. Front-desk staff and operations managers needed clarity on who is authorized to permanently purge student profiles. Student deletion must not be permitted for coaches or front-desk administrators, but reserved exclusively for Operations Managers.
  2. Deleting a student naively leaves orphaned attendance logs, active student packages, athletic evaluations, open safety incident tickets, and portal login credentials with session cookies.
* **Enforced Solution**:
  1. **Transactional Cascading Storage (`DeleteStudent`)**:
     - Both `MemoryStore.DeleteStudent` and `SQLStore.DeleteStudent` execute an atomic cascade:
       - Purges linked `users` account (`student_id = $1`), active `user_sessions`, and `password_reset_tokens`.
       - Purges `safety_incidents` (`student_id = $1`).
       - Purges `student_evaluations` (`student_id = $1`).
       - Purges `attendance` floor logs (`student_id = $1`).
       - Purges `student_packages` credit passes (`student_id = $1`).
       - Purges the `students` profile record.
  2. **Multi-Role RBAC Route Guarding**:
     - `POST /students/{id}/delete`, `DELETE /students/{id}`, and `DELETE /api/students/{id}` are strictly guarded by `RequireRole(models.RoleOperationManager)`.
     - Unauthorized roles (`ADMIN`, `COACH`, `STUDENT`) are forbidden (HTTP 403).
  3. **Interactive UI & Safe Confirmation**:
     - **Student Profile View (`student_detail.html`)**: Action button `🗑️ Delete Student` rendered in top action bar and within "Edit Student Info" modal exclusively when `.CurrentUser.IsOperationManager` is true.
     - **Student Directory Table (`students.html` / `student_table_rows.html`)**: Quick-action delete button (`🗑️`) rendered exclusively for Operation Managers.
     - **Dynamic Feedback**: Flash alert banners rendered at the top of `/students` (`SuccessNotice` / `ErrorNotice`) and `HX-Redirect` support for HTMX consumers.

### Context: Attendance Default Landing Page for Admins & Coaches (/sessions)

* **Problem**:
  1. Default login and home redirects previously sent front-desk Admins to Memberships (/packages) and Coaches to Student Directory (/students).
  2. For daily dojang operations, both Coaches and front-desk Admins require immediate access to the floor schedule and live Attendance check-in screen (/sessions).
* **Enforced Solution**:
  1. **Authentication & Session Landing**:
     - HandleLoginSubmit, HandleLoginPage, and RequireRole redirect authenticated ADMIN and COACH users directly to /sessions instead of /packages or /students.
     - HandleDashboard redirects ADMIN and COACH requests from / directly to /sessions.
  2. **Navigation & Home Actions**:
     - Dojang brand logo in [layout.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/layout.html) links directly to /sessions for Admin and Coach.
     - Desktop and mobile navigation menus position **Attendance** as the leading navigation tab for both roles.
     - Profile navigation and cancel buttons in [profile.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/profile.html) return Admins and Coaches to Attendance (/sessions).

### Context: Coach Pre-Filter on Attendance & Classes Floor Page (/sessions)

* **Problem**:
  1. When coaches arrive on the `/sessions` page, the view previously showed classes from all coaches by default, requiring coaches to manually locate and select their own name in the coach filter dropdown every time.
* **Enforced Solution**:
  1. **Automatic Pre-Filter on Initial Load**:
     - In [`HandleSessions`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/handlers/session_handler.go#L246), inspect the authenticated user context.
     - When `user.Role == models.RoleCoach` and `user.CoachID != nil`: if `!r.URL.Query().Has("coach_id")`, automatically default `coachIDStr = user.CoachID.String()`.
     - Sets `FilterCoachID` in template view data and binds query filters so calendar week view, list rosters, and dropdown selection show the coach's own classes.
  2. **Preserving Explicit Selection**:
     - Checking `!r.URL.Query().Has("coach_id")` ensures that when a coach deliberately selects "All Coaches" (submitting `coach_id=""`), the user's explicit filter choice is respected without being overwritten.
     - Admins and Operations Managers remain defaulted to viewing all coaches unless an explicit filter is chosen.

### Context: Manager Deletion of Closed Classes (/sessions/{id}/delete)

* **Problem**:
  1. Previously, classes that were already closed or in the past (!session.IsOpen()) could not be deleted by any user, including Operations Managers, causing errors if erroneous or test classes were concluded.
* **Enforced Solution**:
  1. **Manager Override**:
     - In HandleDeleteSession, if !sess.IsOpen(), allow deletion if user != nil && user.Role == models.RoleOperationManager. Non-managers attempting to delete closed classes are returned HTTP 400 Bad Request ("Only Operation Managers can delete past or closed classes").
  2. **UI Controls**:
     - In session_cards.html, sessions.html, and live_checkin.html, delete buttons and modals are conditionally enabled/visible if or .IsOpen (and  .IsOperationManager).

### Context: Membership Revocation Governance (/packages/student-packages/{id}/revoke)

* **Problem**:
  1. When a student membership package needed to be cancelled, refunded, or voided for policy reasons, the system lacked a direct revocation mechanism.
* **Enforced Solution**:
  1. **Status Transition**:
     - Added RevokeStudentPackage(id uuid.UUID) error to RepositoryStore (MemoryStore and SQLStore), setting payment_status = 'revoked'.
     - StudentPackage.IsValidAt() requires PaymentStatus == "paid" (or non-revoked), immediately preventing revoked passes from being used for floor check-ins.
  2. **Route Guarding & UI**:
     - POST /packages/student-packages/{id}/revoke and POST /api/packages/student-packages/{id}/revoke are protected with RequireRole(models.RoleOperationManager).
     - Action buttons ?? Revoke and REVOKED status badges rendered in packages.html and student_detail.html exclusively for Operation Managers.

### Context: View-Only Locations Directory for Admins & Coaches (/locations)

* **Problem**:
  1. Front-desk administrators and coaches need access to review branch/dojang facility details, mat capacities, and location contacts, but must not be permitted to create, edit, or delete facility records.
* **Enforced Solution**:
  1. **Route Guarding**:
     - GET /locations is accessible to models.RoleCoach, models.RoleAdmin, and models.RoleOperationManager.
     - All mutating routes (POST /locations, POST /locations/{id}, POST /locations/{id}/edit, PUT /locations/{id}, POST /locations/{id}/delete, DELETE /locations/{id}, DELETE /api/locations/{id}) are strictly restricted to models.RoleOperationManager.
  2. **UI Adaptation**:
     - In locations.html, "Add New Location", "Edit", and "Delete" buttons and modals are rendered only when .CurrentUser.IsOperationManager is true. Non-managers see a clean, informative view-only directory of dojang branches.
     - Coach desktop and mobile navigation in layout.html includes the "Locations" link.

### Context: Role-Tailored Customized Dashboards for Coaches & Admins (/ & /dashboard)

* **Problem**:
  1. Default access permissions previously barred Coaches and front-desk Admins from visiting `/` (redirecting them to `/sessions`), leaving them without a centralized operational dashboard.
  2. Coaches lacked a tailored floor overview combining their assigned classes, 1-click mat check-in, athlete promotion review queue (athletes ready for belt testing), and rapid 6-pillar athletic ability scoring launcher.
  3. Front-desk Administrators lacked a tailored console combining floor class schedules, rapid student search, membership pass renewal/low-credit alerts, 1-click package assignment, and student registration shortcuts.
* **Enforced Solution**:
  1. **Role-Aware Polymorphic Dashboard Dispatcher (`HandleDashboard`)**:
     - `GET /` and `GET /dashboard` are authorized for `RoleCoach`, `RoleAdmin`, and `RoleOperationManager`.
     - Request context role inspection dispatches:
       - `RoleCoach` -> `renderCoachDashboard` rendering `coach_dashboard.html`.
       - `RoleAdmin` -> `renderAdminDashboard` rendering `admin_dashboard.html`.
       - `RoleOperationManager` -> `renderOperationManagerDashboard` rendering executive `dashboard.html`.
       - Managers can preview staff dashboards using `?view=coach` and `?view=admin`.
       - `RoleStudent` automatically redirects to Practitioner Portal (`/portal/student`).
  2. **Navigation & Default Landing Integration**:
     - Desktop and mobile navigation headers position **Dashboard** as the leading navigation tab for both Coach and Admin.
     - Brand logo click routes directly to `/` (rendering their role-tailored dashboard).
     - Default login redirect in `HandleLoginSubmit` and `HandleLoginPage` lands Coaches and Admins on `/`.
  3. **Operational Dashboards UX & Micro-Tools**:
     - **Coach Dashboard (`coach_dashboard.html`)**: Features Instructor Telemetry (Today's Sessions, On-Mat Athlete Count, Promotion Review Candidates, Safety Holds), Quick Access Hub (Mat Check-In, Rapid 6-Factor Athlete Evaluator modal, Report Safety Incident modal), Today's Scheduled Classes list with "Assigned to You" indicator, Live Mat Roster with rapid admittance form, Priority Promotion Queue, and Active Medical Holds alert banner.
     - **Admin Dashboard (`admin_dashboard.html`)**: Features Front-Desk Telemetry (Active Students, Today's Classes, Active Passes, Passes Due for Renewal), Quick Access Hub (Register Student modal, Assign Pass modal, Schedule Class modal), Today's Floor Schedule list with 1-click check-in links, Fast Student Finder combobox, Pass Renewal & Low-Credit Alerts table (highlighting <= 7 days or <= 2 sessions left), and Floor Safety Holds queue.

### Context: Activity & Audit Logging Architecture (/logs & Profile Submenu)

* **Problem**:
  1. The platform lacked an immutable, centralized audit trail tracking user interactions, floor check-ins, security actions, administrative overrides, and credential modifications.
  2. Audit inspection needed to be strictly partitioned so only Operations Managers have access, while integrating directly into the profile submenu and mobile navigation without cluttering core floor operational menus.
* **Enforced Solution**:
  1. **Domain & Storage Isolation**:
     - models.AuditLog, models.AuditLogFilter, and models.AuditTelemetry defined under pure domain models.
     - Dual PostgreSQL and SQLite migration with dedicated B-Tree indexes on created_at DESC, category, user_id, and ction.
     - AuditService encapsulates validation, defaults (UUID, PHT timestamps, string trimming), pagination clamping, and telemetry calculations.
  2. **Non-Blocking Action Instrumentation**:
     - AppHandler.LogAction(r, action, category, targetType, targetID, targetName, description, metadata...) extracts actor identity, client IP (respecting X-Forwarded-For), and User-Agent across authentication, student records, sessions, check-ins, memberships, coaches, and administrators.
  3. **Strict Manager-Exclusive RBAC Guard**:
     - Routes /logs, /logs/, /api/logs, and /api/logs/telemetry are guarded with RequireRole(models.RoleOperationManager). Mismatched browser requests are redirected to role portals (RoleDashboardURL()), while API and HTMX requests return HTTP 403 Forbidden.
  4. **Profile Submenu & Reactive UI**:
     - Desktop profile dropdown and mobile slide-out drawer include an **Activity Logs** entry exclusively rendered for OperationManager.
     - udit_logs.html provides live telemetry metrics, search, category/role/date filters, HTMX partial swapping via udit_log_rows.html, and a full detail inspection modal with formatted JSON payload display.
