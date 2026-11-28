import { execSync } from 'child_process';
import fs from 'fs';
import path from 'path';

export default async function globalTeardown() {
  // Gracefully or forcibly free port 8085 if any lingering process remains
  if (process.platform === 'win32') {
    try {
      execSync(
        'powershell -NoProfile -Command "Get-NetTCPConnection -LocalPort 8085 -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }"',
        { stdio: 'ignore' }
      );
    } catch {
      // ignore
    }
  }

  // Clean up SQLite e2e database files
  const root = process.cwd();
  const dbFiles = ['series_tkd_e2e.db', 'series_tkd_e2e.db-shm', 'series_tkd_e2e.db-wal'];
  for (const file of dbFiles) {
    const fullPath = path.join(root, file);
    if (fs.existsSync(fullPath)) {
      try {
        fs.unlinkSync(fullPath);
      } catch {
        // ignore
      }
    }
  }
}
