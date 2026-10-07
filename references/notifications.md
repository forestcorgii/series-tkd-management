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
