# Database & Persistence Architecture

### Context: Hybrid PostgreSQL & SQLite Storage Driver Strategy

* **Problem**: The application was initially wired to volatile RAM (`MemoryStore`), which caused student check-ins, package credit deductions, and new registrations to be lost upon server restarts. Local development had no default database setup, while production deployment required PostgreSQL per the specification.
* **Enforced Solution**:
  1. **Unified `RepositoryStore` SQL Engine**: Implemented [SQLStore](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/sql_store.go) using standard Go `database/sql` queries with `$1, $2, ...` positional parameters, which are natively supported by both PostgreSQL and SQLite.
  2. **Automatic Environment Detection ([InitDatabase](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/sql_store.go#L37))**:
     * **Production (`DATABASE_URL=postgres://...`)**: Connects using `github.com/jackc/pgx/v5/stdlib` and applies PostgreSQL DDL.
     * **Local Development (No `DATABASE_URL`)**: Connects to an embedded zero-CGO SQLite database (`series_tkd.db` via `modernc.org/sqlite`), creates tables, and auto-seeds initial dojang data on first boot.
  3. **Guaranteed Transactional Floor Check-in**: [CheckInStudent](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/sql_store.go#L518) enforces unique constraints `(session_id, student_id)` and writes attendance logs and package deductions permanently to disk.

### Context: PostgreSQL Strict Type Matching in COALESCE Expressions

* **Problem**: In PostgreSQL, `COALESCE` requires all arguments to have identical or implicitly castable types. Using `COALESCE(has_safety_flag, 0)` caused runtime SQL error `SQLSTATE 42804: COALESCE types boolean and integer cannot be matched` because `has_safety_flag` is defined as `BOOLEAN` in PostgreSQL. While SQLite permits integer fallbacks like `0`, PostgreSQL strictly rejects them.
* **Enforced Solution**:
  * Always use boolean literals `FALSE` or `TRUE` in `COALESCE` for boolean columns: `COALESCE(has_safety_flag, FALSE)`.
  * Both modern SQLite (`modernc.org/sqlite`) and PostgreSQL strictly support the `FALSE` keyword.

### Context: Customizable Membership Plans Schema Evolution

* **Problem**: Adding plan descriptions and checkout overrides (custom prices, notes, and session overrides) requires schema updates without breaking pre-existing development SQLite databases or PostgreSQL deployments.
* **Enforced Solution**:
  * Added `description TEXT DEFAULT ''` to `package_templates`, and `custom_price REAL / NUMERIC(10, 2)` & `notes TEXT DEFAULT ''` to `student_packages`.
  * In `SQLStore.runMigrations()`, execute runtime column evolution using `ALTER TABLE ... ADD COLUMN ...` with safe SQLite and PostgreSQL variants (`ADD COLUMN IF NOT EXISTS` for PostgreSQL).
  * Use `COALESCE(description, '')` and `COALESCE(notes, '')` in SELECT statements to guarantee non-nil string scans.

### Context: Session Concluded Lifecycle & Attendance Removal with Credit Refund

* **Problem**: Training sessions that passed their scheduled end time remained displayed as live classes, with no status distinction indicating concluded attendance. Furthermore, floor staff had no way to remove students who were mistakenly admitted or needed roster removal, leaving package deductions unrefunded and preventing re-check-in.
* **Enforced Solution**:
  * **Dynamic Session Lifecycle (`IsDone()`)**: Evaluated via [TrainingSession.IsDone()](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/session.go) and [IsPastEndTime()](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/session.go), automatically transitioning sessions to `🔒 ATTENDANCE CLOSED` upon passing their scheduled end time without blocking audit edits.
  * **Continuous Roster Modification**: Concluded sessions permit post-session additions (late admissions) and removals for front-desk auditing.
  * **Transactional Student Removal (`RemoveAttendance`)**: Implemented in both [SQLStore.RemoveAttendance](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/sql_store.go) and [MemoryStore.RemoveAttendance](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/store.go). Deletes the attendance record and automatically restores (`+1`) the deducted package class credit for limited passes.
  * **Reactive HTMX Roster Sync**: Attendance modifications dispatch `HX-Trigger: attendanceUpdated`, keeping live rosters and search admittance buttons instantly in sync across floor tablets.

### Context: School Fixed-Rate Attendance & Package Credit Exemption

* **Problem**: In addition to gym students with memberships/packages, school students pay a fixed session rate per class. Checking in school students must record the session rate on attendance and must NOT deduct or modify the student's package session credits.
* **Enforced Solution**:
  * **Database Evolution**: Added `session_rate NUMERIC(10, 2)` (PostgreSQL) and `session_rate REAL` (SQLite) to `attendance` table with runtime `ALTER TABLE` execution.
  * **Domain & Model**: Added `SessionRate *float64` and `SessionRateVal() float64` to [Attendance](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/session.go).
  * **Credit Exemption**: When `session_rate` is specified in check-in requests, package deduction logic (`ProcessCheckInDeduction`) is completely bypassed. `student_package_id` is left null, and `RemainingSessions` remains untouched.
  * **Safe Removal & Cancellation**: Since `student_package_id` is null on fix-rate attendances, roster removal and class cancellation never grant spurious package credits.
  * **UI Display**: Admitted fix-rate students show a distinct `🏫 Fix Rate: ₱X.XX` badge in floor rosters, and search results provide an inline School/Fix Rate check-in option.

### Context: Optional Lead Coach & Floor Session Architecture

* **Problem**: Training sessions initially enforced a non-nullable `coach_id` constraint across database tables, domain structs, and UI check-in/scheduling modals. However, classes often need to be scheduled or floor attendance opened when a lead coach has not yet been designated or is unavailable.
* **Enforced Solution**:
  * **Domain Models ([TrainingSession](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/session.go))**: `CoachID` transitioned from `uuid.UUID` to `*uuid.UUID` (`omitempty`), matching optional foreign keys like `AdminID` and `LocationID`. Added [CoachIDString()](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/session.go) for safe template rendering without nil dereference panics.
  * **Database DDL & Evolution**: Removed `NOT NULL` on `training_sessions.coach_id` in PostgreSQL DDL and SQLite schema. In `SQLStore.Init()`, PostgreSQL executes `ALTER TABLE training_sessions ALTER COLUMN coach_id DROP NOT NULL`, and SQLite performs table recreation if `PRAGMA table_info` reports `notnull == 1`.
  * **Repository & Query Layer**: `scanSession` utilizes `sql.NullString` for `coach_id`, creating nil pointers when unassigned. `CreateSession` and `UpdateSession` bind `nil` when `CoachID` is nil. `MemoryStore` ensures `CoachName` is cleanly emptied when unassigned.
  * **Floor Attendance & UI Modals**: Modals in [sessions.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/sessions.html), [live_checkin.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/live_checkin.html), and [admin_portal.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/admin_portal.html) label the input as `Lead Coach (Optional)` and provide a `<option value="">None / Unassigned</option>` option. Cards and check-in rosters gracefully display `Unassigned` instead of blank values.

### Context: Class-Level Fixed Rate & Streamlined Floor Check-In

* **Problem**: Entering a fixed session rate individually per student during floor attendance was slow and repetitive when an entire school or group class operates under a uniform fixed rate.
* **Enforced Solution**:
  * **Class Schema & Model**: Added `session_rate NUMERIC(10, 2)` (PostgreSQL) and `session_rate REAL` (SQLite) to `training_sessions`. Updated [TrainingSession](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/session.go) with `SessionRate *float64`, `SessionRateVal() float64`, and `HasFixedRate() bool`.
  * **Inherited Rate on Attendance**: When admitting students to a class that has `SessionRate` set, the attendance record automatically inherits `att.SessionRate = session.SessionRate`.
  * **Credit Exemption Invariant**: Any check-in inheriting the class fixed rate satisfies `sessionRate != nil`, ensuring student package credits are untouched and `RemainingSessions` is not decremented.
  * **1-Click Floor Check-In**: In [search_results.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/partials/search_results.html), classes with a fixed rate render a single-click `Admit Student (₱X.XX)` button with no dropdown or manual rate input required per student.
  * **Scheduling & Edit Modals**: Schedulers and editors in [sessions.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/sessions.html), [live_checkin.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/live_checkin.html), and [admin_portal.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/admin_portal.html) include the optional `School / Fixed Rate (₱)` field. Calendar cards and table rows prominently display the `₱X.XX` badge.

### Context: Training Location Fixed Rate & Automated Rate Pre-Fill

* **Problem**: Off-site satellite dojangs, partner academies, and school venues often have fixed pricing agreements per attendee. Requiring operators to manually remember and re-type the location's fixed fee when creating or scheduling classes was prone to human error and slowed down floor workflow.
* **Enforced Solution**:
  * **Database Evolution**: Added `fixed_rate NUMERIC(10, 2)` (PostgreSQL) and `fixed_rate REAL` (SQLite) to the `locations` table, along with runtime migration `ALTER TABLE locations ADD COLUMN ...`.
  * **Domain Model**: Updated [Location](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/location.go) with `FixedRate *float64`, `FixedRateVal() float64`, and `HasFixedRate() bool`.
  * **Locations Management UI ([locations.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/locations.html))**: Added fixed rate configuration to "Add Location" and "Edit Location" modals, plus prominent `🏫 ₱X.XX` badges on the venue table.
  * **Real-Time Client-Side Pre-Fill**: Location dropdowns in [sessions.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/sessions.html), [live_checkin.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/live_checkin.html), and [admin_portal.html](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/templates/pages/admin_portal.html) embed `data-fixed-rate`. Selecting a location immediately populates the class's `session_rate` input in real-time, while still permitting manual administrative adjustment.
  * **Server-Side Fallback Guarantee**: If a session creation or schedule request specifies `location_id` but leaves `session_rate` blank or omitted, [HandleCreateSession](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/handlers/session_handler.go) and [HandleAPIAdminSchedule](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/handlers/portal_handler.go) automatically resolve and assign `sess.SessionRate = loc.FixedRate`.

### Context: Configurable Training Categories & Disciplines Schema Evolution

* **Problem**: Training categories (e.g. Sparring, Poomsae, Conditioning, Promotion Prep) and their visual indicators were hardcoded across backend handlers and frontend templates. Operators had no self-service method to define new disciplines or configure customized badge colors without modifying codebase constants and CSS rules.
* **Enforced Solution**:
  * **Database Evolution**: Created `training_categories` table in PostgreSQL (`id UUID PRIMARY KEY`, `name VARCHAR(100) NOT NULL UNIQUE`, `color VARCHAR(30) NOT NULL`) and SQLite (`id TEXT PRIMARY KEY`, `name TEXT NOT NULL UNIQUE`, `color TEXT NOT NULL`). In `SQLStore.runMigrations()`, runtime migrations ensure tables exist and auto-seed standard categories if empty.
  * **Domain & Model**: Defined [TrainingCategory](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/category.go) with `Name` and `Color` properties.
  * **Cascading Session Sync & Deletion Restriction**: Renaming a category automatically updates existing session records via `UPDATE training_sessions SET training_type = $1 WHERE training_type = $2`. Deleting a category with active assigned sessions is safely blocked to maintain relational integrity.

### Context: Database Indexing, SQLite WAL & Batch N+1 Query Elimination

* **Problem**: 
  1. Handlers (`HandleDashboard`, `HandleStudents`, `HandleSessions`, `HandleLiveSession`, `HandleCoaches`, role portals) repeatedly executed hundreds of individual SQL queries inside loops for attendances, latest evaluations, and student packages (up to 500+ queries per page load).
  2. Tables lacked B-Tree indexes on foreign keys and frequently queried fields (`attendance.session_id`, `attendance.student_id`, `student_packages.student_id`, `training_sessions.session_date`), leading to full-table scans.
  3. SQLite operated under default synchronous `DELETE` journal mode with no connection pool bounds, leading to latency and locks under concurrent requests.
* **Enforced Solution**:
  * **Database Indexes**: Created 12 composite and targeted indexes across `attendance`, `student_packages`, `training_sessions`, `student_evaluations`, `students`, `users`, `user_sessions`, and `safety_incidents`.
  * **SQLite WAL & Connection Pool Tuning**: Enabled Write-Ahead Logging (`PRAGMA journal_mode=WAL;`), `PRAGMA synchronous=NORMAL;`, `PRAGMA busy_timeout=5000;`, and `PRAGMA cache_size=-20000;` (20MB cache) alongside pool tuning (`db.SetMaxOpenConns(10)`, `db.SetMaxIdleConns(5)`).
  * **High-Performance Batch Repository Queries**:
    1. `GetAllAttendances()`: Single query returning all attendances with student, template, session, and location data joined.
    2. `GetSessionAttendanceCounts()`: Aggregated counts via `SELECT session_id, COUNT(*) FROM attendance GROUP BY session_id`.
    3. `GetLatestEvaluations()`: Windowed single-pass query using ANSI `ROW_NUMBER() OVER (PARTITION BY student_id ORDER BY evaluation_date DESC, created_at DESC)`.
    4. `GetAllStudentPackagesGrouped()`: Fetches all packages grouped by student in a single query.
  * **UI Gzip & Buffer Pooling**: Added [GzipMiddleware](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/handlers/gzip_middleware.go) reducing HTML transfer sizes by ~85% and `sync.Pool` buffer pooling for Go template execution.

### Context: Supervising Admin Foreign Key Decoupling

* **Problem**: In PostgreSQL, `training_sessions` was originally defined with `admin_id UUID REFERENCES coaches(id)`, creating the constraint `training_sessions_admin_id_fkey`. When multi-role user portals were implemented, supervising admins were selected from the `users` table (`RoleAdmin` and `RoleOperationManager`). Inserting or updating a training session with an admin `user.ID` caused `ERROR: insert or update on table "training_sessions" violates foreign key constraint "training_sessions_admin_id_fkey" (SQLSTATE 23503)` because the user UUID did not exist in the `coaches` table.
* **Enforced Solution**:
  * **FK Decoupling**: Removed `REFERENCES coaches(id)` from `admin_id` in PostgreSQL DDL (`001_init.sql`, `sql_store.go`) and SQLite schema. In PostgreSQL runtime migrations (`SQLStore.runMigrations()`), automatically execute `ALTER TABLE training_sessions DROP CONSTRAINT IF EXISTS training_sessions_admin_id_fkey`.
  * **Polymorphic Joining**: Queries in `SQLStore` continue to join across both tables (`LEFT JOIN users u ON ts.admin_id = u.id LEFT JOIN coaches ca ON ts.admin_id = ca.id`), supporting both user accounts and legacy coach references.
  * **Cascading Nullification**: Deleting a user in `SQLStore` or `MemoryStore` safely nullifies `training_sessions.admin_id = NULL` for all associated sessions.

