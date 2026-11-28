# Database & Persistence Architecture

## 1. Hybrid PostgreSQL & SQLite Engine
- **Unified `RepositoryStore` Contract**: Every persistence change must maintain 100% parity across [`store.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/store.go) (`RepositoryStore` + `MemoryStore`) and [`sql_store.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/sql_store.go) (`SQLStore`).
- **Auto-Detection ([`InitDatabase`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/sql_store.go#L37))**:
  - `DATABASE_URL=postgres://...` -> PostgreSQL via `github.com/jackc/pgx/v5/stdlib`.
  - Unset `DATABASE_URL` -> embedded zero-CGO SQLite (`series_tkd.db` via `modernc.org/sqlite`) with auto-seeded initial data.
- **SQL Compatibility Rules**:
  - Use `$1, $2, ...` positional parameters (supported by both `pgx/v5/stdlib` and `modernc.org/sqlite`).
  - **Strict Boolean `COALESCE`**: Always write `COALESCE(bool_col, FALSE)` (never `0`, which fails in PostgreSQL with `SQLSTATE 42804`).
  - **Runtime Migrations**: `SQLStore.runMigrations()` in `internal/repository/sql_store.go` is the authoritative DDL & migration source (the legacy `internal/database/migrations/001_init.sql` file is unused). Add new columns to base DDL and append safe `ALTER TABLE ... ADD COLUMN ...` statements inside `runMigrations()`. Use `COALESCE(text_col, '')` or `sql.Null*` when scanning nullable columns.
- **SQLite WAL & Pool Tuning**: DSN pragmas (`_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)`) ensure every pooled connection (`MaxOpenConns(10)`, `MaxIdleConns(5)`) inherits WAL, foreign key enforcement, and busy timeouts.

---

## 2. Batch Queries & Indexing (No N+1)
- **Indexes**: 12 composite/targeted indexes across `attendance`, `student_packages`, `training_sessions`, `student_evaluations`, `students`, `users`, `user_sessions`, and `safety_incidents`.
- **Required Batch Methods**:
  - `GetAllAttendances()` — single joined query for attendances, students, templates, sessions, and locations.
  - `GetSessionAttendanceCounts()` — `SELECT session_id, COUNT(*) FROM attendance GROUP BY session_id`.
  - `GetLatestEvaluations()` — windowed `ROW_NUMBER() OVER (PARTITION BY student_id ORDER BY evaluation_date DESC, created_at DESC)`.
  - `GetAllStudentPackagesGrouped()` — all packages grouped by student.
  - `GetAllUsers()` — all user accounts for O(1) map lookups.

---

## 3. Core Schema & Domain Invariants

### Unified Schedule (`training_sessions` & [`TrainingSession`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/session.go))
- **`entry_type`**: `'class'` (default), `'open_session'` (free/open mat), `'duty'` (staff shift), `'event'` (exams/tournaments). Includes `title` and `assigned_staff` (`JSON`/`TEXT`).
- **Optional `coach_id` & `admin_id`**:
  - `coach_id` is nullable (`*uuid.UUID`; use `CoachIDString()` in templates).
  - `admin_id` is decoupled from `coaches(id)` (`DROP CONSTRAINT IF EXISTS training_sessions_admin_id_fkey`) and polymorphically left-joins `users` and `coaches`; deleting a user sets `admin_id = NULL`.
- **Session Lifecycle (`IsDone()`)**: `!IsCancelled && IsPastEndTime()` transitions sessions to `🔒 ATTENDANCE CLOSED`, while still permitting post-session audit additions/removals.

### Attendance, Fixed Rates & Package Credits (`attendance`, `locations`, `student_packages`)
- **Transactional Check-In (`CheckInStudent`)**: Enforces unique `(session_id, student_id)` and atomically writes attendance + deducts 1 credit from the oldest active `student_package`.
- **Transactional Removal (`RemoveAttendance`)**: Deletes the attendance record and refunds `+1` credit to limited passes when `student_package_id` is set.
- **Fixed-Rate & Open-Session Exemption**:
  - `locations.fixed_rate` pre-fills `training_sessions.session_rate` (client-side via `data-fixed-rate` and server-side fallback in `HandleCreateSession` / `HandleAPIAdminSchedule`).
  - When `session_rate != nil` or `session.IsOpenSession()` is true, `ProcessCheckInDeduction` is bypassed (`student_package_id = NULL`, package credits untouched).
- **Guest & Staff Attendees (`POST /sessions/{id}/checkin-attendee`)**:
  - `attendance` stores nullable `student_id`, `attendee_type` (`student`/`guest`/`staff`), `attendee_name`, and `attendee_role`.
  - Guest admittance inherits `session_rate` (or `session.SessionRate` / `location.FixedRate`) and `ReserveSessionSlot` assigns `att.SessionRate = sess.SessionRate`; only `IsOpenSession()` renders `🔓 Free` for guests.

### Locations, Categories & Membership Plans
- **Locations (`locations` & [`Location`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/location.go))**: Supports `fixed_rate` (`*float64`) and `max_trainees` (`*int`).
- **Training Categories (`training_categories` & [`TrainingCategory`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/models/category.go))**: Auto-seeded if empty; renaming cascades `UPDATE training_sessions SET training_type = $1 WHERE training_type = $2`; deletion is blocked if sessions reference it.
- **Membership Plans (`package_templates`, `student_packages`)**: `package_templates.description` and `student_packages.custom_price` / `notes` support checkout overrides.
