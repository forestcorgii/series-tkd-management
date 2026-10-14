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
