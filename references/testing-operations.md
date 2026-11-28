# Testing & Verification Operations

### Context: Port 8080 Teardown & Post-Test Cleanup

#### Problem
Processes spun up during tests or local verification sessions (such as `main.go`, local web servers, or test runners) frequently bind to port `8080`. If not cleanly terminated, lingering background processes hold port 8080 open on Windows, causing subsequent tests and server boots to fail with `bind: address already in use`.

#### Enforced Solution
- **Mandatory Post-Test Teardown**: After running any test command (`go test`, integration scripts, or local server verification runs), always terminate any process listening on port `8080`.
- **Windows PowerShell Cleanup Command**:
  ```powershell
  Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }
  ```
- **Go Test Suite Invariant**: In integration tests spinning up HTTP servers, always register a `t.Cleanup(func() { ... })` or use `httptest.NewServer` with dynamic ports when possible. If port 8080 is used, release it immediately upon test completion.

### Context: Playwright End-to-End (E2E) Testing Operations

#### Architecture & Isolation Standards
- **Dedicated Port**: Playwright webServer runs on `PORT=8085` to avoid colliding with active development sessions on `8080`.
- **Database Isolation**: The test runner sets `DATABASE_URL=series_tkd_e2e.db`. `e2e/global-setup.ts` wipes previous test db files so each run begins with a pristine schema and deterministic `SeedDefaultData()` fixtures.
- **Teardown Invariant**: `e2e/global-teardown.ts` terminates any lingering process on port `8085` and removes SQLite temporary files.

#### Windows PowerShell Command Standards
On Windows PowerShell, invocation scripts must use `npm.cmd` and `npx.cmd` to prevent `.ps1` ExecutionPolicy security errors:
```powershell
# Run headless test suite
npm.cmd run test:e2e

# Interactive UI Mode
npm.cmd run test:e2e:ui

# Headed browser execution
npm.cmd run test:e2e:headed

# View HTML report
npm.cmd run test:e2e:report
```
