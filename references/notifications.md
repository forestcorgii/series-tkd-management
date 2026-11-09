# Multi-Channel Notifications Architecture & Delivery Channels

### Context: Push, SMS, and Email Notifications System
- **Problem**: The dojang needed multi-channel alerts (Push, SMS, Email, In-App) for key operational and instructional lifecycle events, with strict requirements that Email and SMS notifications can be globally or individually disabled without breaking in-app operational logs.
- **Enforced Solution**:
  1. **Domain Model & Master Switches**:
     - `models.NotificationSettings` maintains global channel toggles: `PushEnabled`, `SMSEnabled`, `EmailEnabled`.
     - In HTML forms, omitted checkboxes evaluate to `false`, allowing administrators to disable SMS and Email alerts with immediate effect.
     - `NotificationService` checks `settings.IsChannelEnabled(channel)` and `userPrefs.ShouldReceive(channel)`. When a channel (like SMS or Email) is disabled, outward dispatch is suppressed and flagged as `StatusDisabled`.
     - `models.ChannelInApp` remains unconditionally enabled as the dojang's audit and operational inbox.
  2. **Personal Channel Opt-Out**:
     - `models.UserNotificationPreferences` allows practitioners, coaches, and administrators in `/profile` to opt out of Email or SMS notifications individually.
  3. **Event Triggers Supported**:
     - **Coaches & Administrators**:
       - `EventStudentAdmitted`: Student self-admit or front-desk check-in.
       - `EventNewClassOpened`: New training class opened or scheduled.
       - `EventStaffRegistered`: Staff/coach registration pending manager review.
       - `EventStudentInjured`: Mat injury / safety incident logged.
       - `EventClassCancelled`: Class session cancelled.
     - **Students & Guardians**:
       - `EventPromotionEligible`: Belt promotion readiness criteria achieved.
       - `EventStudentInjured`: Priority alert to student and emergency contact phone/email.
       - `EventNewEvaluation`: Coach logged 6-factor ability evaluation with radar coordinates.
       - `EventMembershipAwarded` / `EventMembershipRevoked`: Membership pass changes.
       - `EventSafetyResolved`: Injury/medical clearance restored.
       - `EventClassCancelled`: Registered class cancellation.
       - `EventPassExpiring`: Pass balance running low or validity expiring soon.
  4. **Interactive In-App Notification Center**:
     - Bell icon in navbar with polling badge (`/api/notifications/badge` via HTMX every 30s).
     - Interactive popover dropdown (`/api/notifications/dropdown`) with one-click dismiss and mark-all-read.
     - JSON API endpoint (`/api/notifications`) for mobile and external integration.

### Context: Standards-Compliant Web Push & VAPID Delivery
- **Problem**: PWA floor tablets and mobile devices required real OS-level background push alerts across Chrome, Safari, Android, and desktop devices without requiring proprietary push bridges or breaking when devices are offline.
- **Enforced Solution**:
  1. **VAPID Cryptographic Engine (RFC 8291 / 8292)**:
     - Uses `github.com/SherClockHolmes/webpush-go` (NIST P-256 curve).
     - `NotificationService.EnsureVAPIDKeys()` automatically generates and saves valid VAPID public/private keypairs upon first boot if keys are missing.
     - Managers can force key generation in `/settings?tab=notifications` (`POST /settings/notifications/generate-vapid`).
  2. **Multi-Device Subscription Storage (`push_subscriptions`)**:
     - Table in SQLite and PostgreSQL: `id`, `user_id`, `role`, `endpoint UNIQUE`, `p256dh`, `auth`, `user_agent`, `created_at`, `updated_at`.
     - `ON CONFLICT(endpoint) DO UPDATE` ensures device re-subscriptions update keys without duplication.
     - Prunes stale/dead subscriptions automatically when browser push services return HTTP 410 Gone or 404 Not Found.
  3. **Service Worker Push & Deep Linking (`web/static/sw.js`)**:
     - `'push'` event displays native system notification banner (`showNotification`) with Series TKD branding and vibration sequence.
     - `'notificationclick'` focuses an existing app window or opens the deep link destination (e.g. `/sessions/{id}/live`, `/students/{id}`).
  4. **Client Engine & Diagnostics**:
     - [`web/static/js/push-client.js`](file:///c:/Users/USER/Documents/Coding%20Projects/antigravity/series-tkd-management/web/static/js/push-client.js) (`window.STMSPush`) handles VAPID subscription conversions and background syncing.
     - Interactive "Test Push" diagnostic triggers available in `/profile` and `/settings` (`POST /api/push/test`) returning delivery feedback.
     - Interactive "Test Email" diagnostic trigger in `/settings?tab=notifications` (`POST /api/email/test` and `POST /settings/notifications/test-email`) testing live SMTP connectivity, TLS/STARTTLS negotiation, auth credentials, and fallback to logged-in administrator email.

### Context: Class Creation & Self-Reservation Staff Routing & Outward Email Delivery
- **Problem**: When a new training class was opened or when a guest/student self-reserved a slot, notification dispatch lacked staff targeting (causing unassigned slots to be missed or individual coaches to receive irrelevant schedule alerts) and outward email dispatch was purely logged without sending actual emails.
- **Enforced Solution**:
  1. **Unified Staff Routing Matrix (`getClassStaffRecipients`)**:
     - **Operation Managers**: Always receive all notifications unconditionally.
     - **Coach**: If specified on the class (`CoachID != nil`), notify only that assigned coach; if unassigned (`CoachID == nil`), notify all active coaches.
     - **Admin**: If specified on the class (`AdminID != nil`), notify only that assigned admin; if unassigned (`AdminID == nil`), notify all active admins.
     - **Unassigned Fallback**: When neither coach nor admin is specified, full broadcast to all active coaches, admins, and managers.
     - **Deduplication**: Runs through `deduplicateRecipients()` so staff members with multiple roles receive exactly 1 notification.
  2. **Lifecycle Events**:
     - `EventNewClassOpened`: Dispatched on `POST /sessions` and `POST /api/v1/sessions` with rich timetable metadata (`session_id`, `training_type`, `session_date`, `time_range`, `coach`, `admin`). Deep-links to `/sessions?date=YYYY-MM-DD`.
     - `EventSlotReserved`: Dispatched on `POST /reserve/{id}` and `POST /landing/reserve` for both guest registrations (with emergency contact) and student self-reservations. Deep-links to `/sessions/{id}/live`.
  3. **Outward Asynchronous SMTP Delivery**:
     - `NotificationService.dispatch()` invokes `go s.sendEmailNotification(...)` so HTTP request handling remains instant (<50ms).
     - Formats multipart MIME emails with Series TKD branding (Crimson `#990303`, details table, actionable CTA button) and plain-text fallback.
     - Supports simulated host (`simulated`, `mock`, `*.test`) for testing/development environments.
