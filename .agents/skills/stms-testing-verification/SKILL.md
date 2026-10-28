---
name: stms-testing-verification
description: >-
  Runbooks and verification loops for executing Go domain unit tests, handler/repository tests,
  Playwright E2E tests, and mandatory port 8080/8085 cleanup on Windows PowerShell.
  Use this skill whenever verifying code changes, writing tests, or debugging test failures in STMS.
---

# STMS Testing & Verification Skill

Follow this runbook to execute fast, isolated verification loops and prevent port collisions on Windows.

## 1. Domain-Only TDD Loop (Fastest — Prefer First)
Per `AGENTS.md` Rule II.2, focus unit testing on pure domain logic (`internal/models`, `internal/services`, `internal/storage`):

```powershell
# Run domain models, services, and storage tests
go test -v ./internal/models/... ./internal/services/... ./internal/storage/...
```

## 2. Full Go Test Suite (Handlers & Repository)
When modifying HTTP/HTMX handlers, RBAC guards, or `SQLStore`/`MemoryStore`:

```powershell
go test ./...
```

- **Route Parity Check**: If you added or modified a route, verify it is registered in **both** [`main.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/main.go) and [`cmd/server/main.go`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/cmd/server/main.go).
- **Test Server Invariant**: Always use `httptest.NewServer` (dynamic port) in Go tests rather than binding to `:8080`.

## 3. Playwright End-to-End (E2E) Suite
When verifying browser flows, modals, or HTMX interactions end-to-end:

```powershell
# Always use npm.cmd on Windows PowerShell to avoid ExecutionPolicy errors
npm.cmd run test:e2e
```

- **Isolation**: Playwright spins up the server on `PORT=8085` using `DATABASE_URL=series_tkd_e2e.db` (see [`references/testing-operations.md`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/references/testing-operations.md)).

## 4. Mandatory Post-Test Port Cleanup
Always run this PowerShell command after any verification step that starts a local server or runs tests to ensure ports `8080` and `8085` are freed:

```powershell
Get-NetTCPConnection -LocalPort 8080, 8085 -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }
```

