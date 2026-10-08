# Profile Picture & Railway Bucket Storage Architecture

### Context: User Profile Picture & Railway Bucket Storage Architecture

* **Problem**:
  1. Users across all roles (Students, Coaches, Administrators, Operation Managers) lacked the ability to upload and display profile pictures, with avatars defaulting to text initials or generic placeholders.
  2. Cloud object storage was needed for production on Railway (Railway S3 Buckets) without breaking local offline development, unit tests, or requiring external cloud credentials during CI.
  3. UI components across top navigation bar, drawer menus, user profile cards, and operational rosters (Admins, Coaches, Students) needed a consistent avatar display pattern with a standardized person icon fallback.

* **Enforced Solution**:
  1. **Railway Bucket S3 Storage & Local Disk Fallback (`internal/storage`)**:
     - `storage.FileStorage` abstraction provides `Upload` and `Delete` contracts.
     - Pure Go standard library AWS Signature Version 4 (`storage.S3Storage`) integrates with Railway Object Storage / S3-compatible buckets using `BUCKET_NAME`, `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_ENDPOINT_URL_S3`, `AWS_REGION`, and optional `BUCKET_PUBLIC_URL` without heavy third-party dependencies.
     - When bucket environment variables are not configured (local dev, tests), `storage.LocalStorage` automatically activates and saves uploads under `web/static/uploads/avatars/`, served by the built-in HTTP static file handler.
  2. **Domain Model & Persistence**:
     - `models.User` struct persists `ProfilePictureURL string`.
     - Dual database schemas (SQLite & PostgreSQL) include `profile_picture_url` column with non-destructive migrations (`ALTER TABLE users ADD COLUMN profile_picture_url`).
     - In-memory mock store and SQL repository provide `GetUserByStudentID` and `GetUserByCoachID` lookup helpers to link student and coach rosters to their user avatar.
  3. **Profile Upload & Removal Management (`/profile`)**:
     - Profile form supports `enctype="multipart/form-data"` with a 5MB payload limit.
     - Accepts `.jpg`, `.jpeg`, `.png`, `.webp`, `.gif` with validation against invalid file extensions.
     - Client-side instantaneous image preview via `FileReader` and file size validation.
     - Supports 1-click photo removal (`remove_avatar=1`) which clears the URL and cleans up the stored file.
     - Actions audited via `PROFILE_AVATAR_UPLOAD` and `PROFILE_AVATAR_REMOVE`.
  4. **Universal UI Avatar Display & Person Icon Fallback**:
     - Standardized heroicons person SVG icon (`<svg viewBox="0 0 24 24"><path d="M7.5 6a4.5 4.5..."/>`) rendered whenever `ProfilePictureURL` is empty.
     - Active across:
       - **Top Navbar Profile Pill (`layout.html`)**: Circular thumbnail inside `#profile-menu-button`.
       - **Dropdown Header (`layout.html`)**: Rounded avatar beside user identity.
       - **Mobile Drawer Header (`layout.html`)**: Compact avatar beside username and role badge.
       - **Profile Hero Card (`profile.html`)**: Large 20x20 rounded container.
       - **Admins Roster (`admins.html`)**: Admin list row avatars.
       - **Coaches Roster (`coaches.html`)**: Coach list row and detail modal avatars.
       - **Students Roster (`student_table_rows.html` & `student_detail.html`)**: Student list rows and profile hero card.

### Context: Railway Bucket Zero-Config Storage with Private Bucket Proxying

* **Problem**:
  On Railway Object Storage, only **bucket name** (`BUCKET_NAME` / `BUCKET`), **access key** (`ACCESS_KEY_ID` / `AWS_ACCESS_KEY_ID`), and **secret key** (`SECRET_ACCESS_KEY` / `AWS_SECRET_ACCESS_KEY`) are available. No public URL or custom CDN domain is provided. Railway S3 buckets (`storage.railway.app`) are strictly private by default; direct browser requests to `https://storage.railway.app/{bucket}/{key}` fail with HTTP `403 Forbidden`.

* **Enforced Solution**:
  1. **Zero-Configuration Defaults**:
     - `storage.NewStorageFromEnv()` defaults `endpoint` to `https://storage.railway.app` and `region` to `auto` when only bucket name, access key, and secret key are set.
     - Supports all common variable names (`BUCKET`, `BUCKET_NAME`, `ACCESS_KEY_ID`, `AWS_ACCESS_KEY_ID`, `SECRET_ACCESS_KEY`, `AWS_SECRET_ACCESS_KEY`).
  2. **Internal Proxy Route (`GET /storage/{key...}`)**:
     - When `PublicURL` is empty, `S3Storage.Upload(...)` returns the local path `/storage/{key}` (e.g. `/storage/avatars/user-xxx.png`).
     - `HandleServeStorage` on `GET /storage/{key...}` fetches the object from Railway S3 using an AWS SigV4 signed `GET` request via `a.storage.Get(r.Context(), key)` and streams the bytes directly to the browser.
     - Adds `Cache-Control: public, max-age=86400` so client browsers cache avatar images for 24 hours, minimizing redundant S3 requests.
     - Transparently works with `LocalStorage` during local development as well.

