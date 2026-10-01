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


