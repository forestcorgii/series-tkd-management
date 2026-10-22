import fs from 'fs';
import path from 'path';

export default async function globalSetup() {
  const root = process.cwd();
  const dbFiles = ['series_tkd_e2e.db', 'series_tkd_e2e.db-shm', 'series_tkd_e2e.db-wal'];
  for (const file of dbFiles) {
    const fullPath = path.join(root, file);
    if (fs.existsSync(fullPath)) {
      try {
        fs.unlinkSync(fullPath);
      } catch {
        // file might be locked or already removed
      }
    }
  }
}
