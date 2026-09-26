---
description: Rules for executing tests, verification runs, and managing local server ports
---

# Testing Operations & Port Cleanup

1. **Always Clean Up Port 8080 After Tests**
   - After executing any test run (e.g., `go test ./...`), verification loop, or local server process, always ensure port 8080 is freed.
   - Run the PowerShell cleanup command if any process was started on or bound to port 8080:
     ```powershell
     Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }
     ```

2. **No Orphaned Test Servers**
   - Do not leave background server processes running after completing a test step.
   - Verify port availability before and after test execution to prevent port collision errors.
