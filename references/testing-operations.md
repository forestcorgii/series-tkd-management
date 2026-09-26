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
