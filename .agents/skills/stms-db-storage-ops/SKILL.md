---
name: stms-db-storage-ops
description: >-
  Procedures for evolving database schemas across PostgreSQL and SQLite (SQLStore & MemoryStore),
  writing high-performance batch queries, and managing Railway S3 Bucket file storage.
  Use this skill when adding columns/tables, modifying repository queries, or touching file uploads.
---

# STMS Database & Object Storage Operations

For detailed architecture context, consult:
- [`references/database.md`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/references/database.md)
- [`references/profile-picture-storage.md`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/references/profile-picture-storage.md)

## 1. Repository Store Parity (`RepositoryStore`)
Every persistence change must maintain 100% parity across three files:
1. Interface contract in [`internal/repository/store.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/store.go) (`RepositoryStore`)
2. In-memory implementation in [`internal/repository/store.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/store.go) (`MemoryStore`)
3. SQL implementation in [`internal/repository/sql_store.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/internal/repository/sql_store.go) (`SQLStore`)

## 2. Dual PostgreSQL & SQLite SQL Rules
1. **Positional Parameters**: Always use `$1, $2, ...` placeholders (supported by both `pgx/v5/stdlib` and `modernc.org/sqlite`).
2. **Strict Boolean `COALESCE`**: Never write `COALESCE(bool_col, 0)` (fails in PostgreSQL with `SQLSTATE 42804`). Always write `COALESCE(bool_col, FALSE)`.
3. **Non-Destructive Runtime Migrations**:
   - Add new columns to base DDL in `sql_store.go` AND append safe `ALTER TABLE ... ADD COLUMN ...` statements inside `SQLStore.runMigrations()`.
   - Use `COALESCE(text_col, '')` or `sql.NullString` / `sql.NullFloat64` when scanning nullable columns.
4. **No N+1 Queries**:
   - Never call `store.Get*` inside a loop in HTTP handlers. Use or add batch repository methods (`GetAllAttendances()`, `GetAllStudentPackagesGrouped()`, `GetLatestEvaluations()`, `GetAllUsers()`).

## 3. Railway S3 Bucket & Local Storage (`internal/storage`)
- `storage.NewStorageFromEnv()` auto-selects `S3Storage` when bucket credentials are set, or falls back to `LocalStorage` (`web/static/uploads/avatars/`).
- **Railway Variable Names**: Railway Buckets expose `BUCKET`, `ACCESS_KEY_ID`, `SECRET_ACCESS_KEY`, `ENDPOINT`, and `REGION` (supported alongside `BUCKET_NAME` and `AWS_*` aliases).
- **Addressing**: `S3Storage` uses **Virtual-Hosted Style** (`https://{bucket}.{endpoint_host}/{key}`) by default because Railway Buckets reject path-style requests with `NoSuchBucket`.
- **Serving**: When `BUCKET_PUBLIC_URL` is omitted, uploaded keys return `/storage/{key}` and stream via `GET /storage/{key...}` with SigV4 signing and `Cache-Control: public, max-age=86400`.

