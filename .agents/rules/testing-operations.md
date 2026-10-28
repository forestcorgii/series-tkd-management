---
trigger: model_decision
description: When executing unit tests, integration tests, Playwright E2E tests, or starting local verification servers.
---

# Testing Operations & Port Cleanup

For detailed test commands and E2E setup, consult [`references/testing-operations.md`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/references/testing-operations.md).

1. **Always Clean Up Port 8080 After Tests**:
   - After executing any test run (`go test ./...`), verification loop, or local server process, always ensure port `8080` is freed on Windows PowerShell:
     ```powershell
     Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }
     ```
2. **Playwright E2E Isolation (Port 8085)**:
   - E2E tests run on dedicated port `8085` with isolated SQLite DB `series_tkd_e2e.db`.
   - Always invoke npm/npx on Windows PowerShell using `.cmd` (`npm.cmd run test:e2e`) to avoid `.ps1` ExecutionPolicy errors.
3. **No Orphaned Test Servers**:
   - Prefer `httptest.NewServer` with dynamic ports in Go tests, and never leave background server processes running after completing verification.
