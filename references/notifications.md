# Notifications Architecture & Delivery Channels

### Context: Telegram Bot & Email Notifications Architecture (SMS & Push Deprecated)
- **Problem**: The dojang required real-time public announcements (new classes, cancellations) and private staff alerts (new students enrolled, self-reservations, pending staff registrations) without relying on expensive SMS gateways or fragile browser Web Push subscriptions. Additionally, multiple branch locations needed curated Telegram destinations while falling back to global settings.
- **Enforced Solution**:
  1. **Clean Purge of Web Push & SMS**:
     - Removed `ChannelPush` and `ChannelSMS`. Purged all VAPID/push subscription database tables, migrations, models, client scripts (`push-client.js`), and `webpush-go` dependency.
     - Global master toggles in `models.NotificationSettings` are now: `TelegramEnabled` and `EmailEnabled`.
     - In-App notifications (`models.ChannelInApp`) remain unconditionally active as the immutable audit log and floor inbox.
  2. **Zero-Dependency Telegram Client (`internal/services/telegram_client.go`)**:
     - Standard Go library `net/http` client calling `https://api.telegram.org/bot<TOKEN>/sendMessage`.
     - Formats messages in HTML mode (`parse_mode: "HTML"`).
     - Supports inline keyboard buttons (`inline_keyboard`) linking directly to STMS web actions (e.g. Schedule, Live Floor, Student Profile).
     - Diagnostic testing via `TestTelegramConnection` and `POST /settings/notifications/test-telegram`.
  3. **Multi-Location Curated Routing**:
     - Each `models.Location` can define its own `TelegramChannelID` (Public Announcements) and `TelegramStaffChatID` (Private Staff Group).
     - When dispatching class- or location-bound notifications, `NotificationService` checks the session's location:
       - Uses `loc.TelegramChannelID` if set, otherwise falls back to `settings.TelegramPublicChatID`.
       - Uses `loc.TelegramStaffChatID` if set, otherwise falls back to `settings.TelegramStaffChatID`.
  4. **Supported Telegram Announcement Triggers**:
     - `EventNewClassOpened` -> Broadcast to Location Public Channel (or Global Public Channel).
     - `EventStudentRegistered` -> Dispatched to Staff Group with student details and direct profile link.
     - `EventSlotReserved` -> Dispatched privately to Staff Group when a student or guest self-reserves a slot.
     - `EventStaffRegistered` -> Dispatched privately to Staff Group when a coach or admin registers.
     - `EventClassCancelled` -> Broadcast to both Public Channel and Staff Group.
  5. **Asynchronous Non-Blocking Dispatch**:
     - Dispatched in lightweight goroutines (`go s.sendTelegramNotification(...)`) to keep request response times under 50ms.

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

### Context: Outward Email Sender Display Name Configuration
- **Problem**: Previously, outward emails only included the raw `SMTPFrom` email address (e.g., `notifications@seriestkd.com`) in the RFC 5322 `From:` header. Mail clients rendered the sender without an official display name unless the client deduced one, and administrators could not configure a custom sender branding.
- **Enforced Solution**:
  1. **Configurable Display Name (`models.NotificationSettings.SMTPFromName`)**:
     - Added `SMTPFromName` to `models.NotificationSettings` (`smtp_from_name` in PostgreSQL & SQLite DDL and runtime migrations) defaulting to `"Series Taekwondo"`.
     - In `settings.html`, added a Display Name input field (`name="smtp_from_name"`) in a responsive 4-column desktop layout alongside Host, Port, and From Address.
     - Handlers support both `smtp_from_name` and `smtp_display_name` in form and JSON requests for diagnostic test email (`/api/email/test`) and settings update (`/settings/notifications`).
  2. **RFC 5322 Header & SMTP Command Separation**:
     - `net/mail.Address{Name: fromName, Address: from}.String()` formats the MIME `From:` header (e.g., `"Series Taekwondo" <notifications@seriestkd.com>`).
     - SMTP command `client.Mail(from)` strictly continues to pass the raw email address without display name, preventing SMTP 501 syntax error responses.
