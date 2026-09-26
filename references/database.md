# Database & Persistence Architecture

### Context: Hybrid PostgreSQL & SQLite Storage Driver Strategy

* **Problem**: The application was initially wired to volatile RAM (`MemoryStore`), which caused student check-ins, package credit deductions, and new registrations to be lost upon server restarts. Local development had no default database setup, while production deployment required PostgreSQL per the specification.
* **Enforced Solution**:
  1. **Unified `RepositoryStore` SQL Engine**: Implemented [SQLStore](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/sql_store.go) using standard Go `database/sql` queries with `$1, $2, ...` positional parameters, which are natively supported by both PostgreSQL and SQLite.
  2. **Automatic Environment Detection ([InitDatabase](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/sql_store.go#L37))**:
     * **Production (`DATABASE_URL=postgres://...`)**: Connects using `github.com/jackc/pgx/v5/stdlib` and applies PostgreSQL DDL.
     * **Local Development (No `DATABASE_URL`)**: Connects to an embedded zero-CGO SQLite database (`series_tkd.db` via `modernc.org/sqlite`), creates tables, and auto-seeds initial dojang data on first boot.
  3. **Guaranteed Transactional Floor Check-in**: [CheckInStudent](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/sql_store.go#L518) enforces unique constraints `(session_id, student_id)` and writes attendance logs and package deductions permanently to disk.
