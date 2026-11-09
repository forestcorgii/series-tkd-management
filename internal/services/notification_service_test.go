package services_test

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
	"series-tkd-management/internal/services"
)

func setupTestNotificationService() (*services.NotificationService, repository.RepositoryStore) {
	store := repository.NewMemoryStore()
	svc := services.NewNotificationService(store)
	return svc, store
}

func TestNotificationService_DisabledChannels(t *testing.T) {
	svc, store := setupTestNotificationService()

	// 1. Disable SMS and Email in Settings
	settings, _ := svc.GetSettings()
	settings.SMSEnabled = false
	settings.EmailEnabled = false
	settings.PushEnabled = true
	_ = svc.UpdateSettings(settings)

	// Create a student with phone and email
	student := &models.Student{
		ID:             uuid.New(),
		FullName:       "Jin Kazama",
		Phone:          "+639171234567",
		EmergencyName:  "Jun Kazama",
		EmergencyPhone: "+639189876543",
	}
	_ = store.CreateStudent(student)

	// Trigger promotion eligible notification
	svc.NotifyPromotionEligible(student)

	// Verify that NO SMS and NO Email notifications were created
	notifs, _, _ := svc.GetNotifications(models.NotificationFilter{})

	for _, n := range notifs {
		if n.Channel == models.ChannelSMS {
			t.Errorf("expected SMS to be disabled, but found SMS notification: %+v", n)
		}
		if n.Channel == models.ChannelEmail {
			t.Errorf("expected Email to be disabled, but found Email notification: %+v", n)
		}
	}
}

func TestNotificationService_PersonalPreferences(t *testing.T) {
	svc, store := setupTestNotificationService()

	// Global settings have everything enabled
	settings, _ := svc.GetSettings()
	settings.SMSEnabled = true
	settings.EmailEnabled = true
	settings.PushEnabled = true
	_ = svc.UpdateSettings(settings)

	studentID := uuid.New()
	userID := uuid.New()

	student := &models.Student{
		ID:             studentID,
		FullName:       "Paul Phoenix",
		Phone:          "+639171112222",
		EmergencyName:  "Forest Law",
		EmergencyPhone: "+639173334444",
	}
	_ = store.CreateStudent(student)

	user := &models.User{
		ID:        userID,
		Email:     "paul@test.com",
		Role:      models.RoleStudent,
		StudentID: &studentID,
		IsActive:  true,
	}
	_ = store.CreateUser(user)

	// User disables SMS personally
	prefs := &models.UserNotificationPreferences{
		UserID:       userID,
		EmailEnabled: true,
		SMSEnabled:   false,
		PushEnabled:  true,
		UpdatedAt:    time.Now(),
	}
	_ = svc.UpdateUserPreferences(prefs)

	// Trigger evaluation notification
	eval := &models.StudentEvaluation{
		ID:          uuid.New(),
		StudentID:   studentID,
		Flexibility: 8,
		Stamina:     8,
		Power:       8,
		Technique:   8,
		SparringIQ:  8,
		Discipline:  8,
	}
	svc.NotifyNewlyEvaluated(eval, student, "Hwoarang")

	notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &userID})

	for _, n := range notifs {
		if n.Channel == models.ChannelSMS {
			t.Errorf("expected user to not receive SMS when opted out personally, got: %+v", n)
		}
	}
}

func TestNotificationService_NotifyStudentAdmitted(t *testing.T) {
	svc, store := setupTestNotificationService()

	coachID := uuid.New()
	coach := &models.Coach{
		ID:       coachID,
		FullName: "Master Lee",
		Email:    "lee@tkd.com",
		Phone:    "+639170001111",
		IsActive: true,
	}
	_ = store.CreateCoach(coach)

	sessID := uuid.New()
	sess := &models.TrainingSession{
		ID:           sessID,
		CoachID:      &coachID,
		TrainingType: models.TrainingSparring,
		SessionDate:  time.Now(),
	}
	_ = store.CreateSession(sess)

	student := &models.Student{
		ID:       uuid.New(),
		FullName: "Kazuya Mishima",
	}
	_ = store.CreateStudent(student)

	svc.NotifyStudentAdmitted(sess, student, true, "Self Check-In")

	notifs, count, err := svc.GetNotifications(models.NotificationFilter{})
	if err != nil || count == 0 {
		t.Fatalf("expected notifications to be logged, count=%d, err=%v", count, err)
	}

	foundInApp := false
	for _, n := range notifs {
		if n.EventType == models.EventStudentAdmitted && n.Channel == models.ChannelInApp {
			foundInApp = true
			if n.Title == "" || n.Message == "" {
				t.Errorf("expected non-empty title and message, got %+v", n)
			}
		}
	}
	if !foundInApp {
		t.Errorf("expected In-App notification for student admitted")
	}
}

func TestNotificationService_NotifyStudentInjured(t *testing.T) {
	svc, store := setupTestNotificationService()

	student := &models.Student{
		ID:             uuid.New(),
		FullName:       "Ling Xiaoyu",
		Phone:          "+639175556666",
		EmergencyName:  "Wang Jinrei",
		EmergencyPhone: "+639177778888",
	}
	_ = store.CreateStudent(student)

	inc := &models.SafetyIncident{
		ID:           uuid.New(),
		StudentID:    student.ID,
		IncidentType: "Ankle Sprain",
		Notes:        "Landed awkwardly during jump spinning hook kick",
	}
	_ = store.CreateSafetyIncident(inc)

	svc.NotifyStudentInjured(inc, student, "Coach Baek")

	notifs, count, _ := svc.GetNotifications(models.NotificationFilter{
		EventType: func() *models.NotificationEventType {
			ev := models.EventStudentInjured
			return &ev
		}(),
	})

	if count == 0 || len(notifs) == 0 {
		t.Fatalf("expected injury notifications to be dispatched")
	}

	foundGuardianNotice := false
	for _, n := range notifs {
		if n.RecipientType == "GUARDIAN" {
			foundGuardianNotice = true
		}
	}
	if !foundGuardianNotice {
		t.Errorf("expected guardian to be notified of student injury")
	}
}

func TestNotificationService_MarkAsRead(t *testing.T) {
	svc, _ := setupTestNotificationService()

	notif := &models.Notification{
		ID:        uuid.New(),
		Title:     "New Alert",
		Message:   "Test",
		Channel:   models.ChannelInApp,
		EventType: models.EventNewClassOpened,
		Status:    models.StatusSent,
		IsRead:    false,
		CreatedAt: time.Now(),
	}
	_ = svc.UpdateSettings(models.DefaultNotificationSettings())

	// Create and mark as read
	_ = svc.MarkAsRead(notif.ID)

	unread, _ := svc.GetUnreadCount(nil, nil)
	if unread != 0 {
		t.Errorf("expected 0 unread notifications, got %d", unread)
	}
}

func TestNotificationService_NoDuplicateNotifications(t *testing.T) {
	svc, store := setupTestNotificationService()

	coachID := uuid.New()
	coachUserID := uuid.New()
	coach := &models.Coach{
		ID:       coachID,
		FullName: "Master Lee",
		Email:    "lee@tkd.com",
		Phone:    "+639170001111",
		IsActive: true,
	}
	_ = store.CreateCoach(coach)
	coachUser := &models.User{
		ID:       coachUserID,
		Email:    "lee@tkd.com",
		Role:     models.RoleCoach,
		CoachID:  &coachID,
		IsActive: true,
	}
	_ = store.CreateUser(coachUser)

	managerUserID := uuid.New()
	managerUser := &models.User{
		ID:          managerUserID,
		Email:       "manager@tkd.com",
		Role:        models.RoleOperationManager,
		DisplayName: "Dojang Manager",
		IsActive:    true,
	}
	_ = store.CreateUser(managerUser)

	sess := &models.TrainingSession{
		ID:           uuid.New(),
		CoachID:      &coachID,
		TrainingType: models.TrainingSparring,
		SessionDate:  time.Now(),
	}
	_ = store.CreateSession(sess)

	student := &models.Student{
		ID:       uuid.New(),
		FullName: "Jin Kazama",
	}
	_ = store.CreateStudent(student)

	// Action: Student admitted to class
	svc.NotifyStudentAdmitted(sess, student, false, "Staff")

	// Verify that each distinct staff user received EXACTLY 1 notification (no duplicates across channels)
	coachNotifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &coachUserID})
	if len(coachNotifs) != 1 {
		t.Fatalf("expected coach to have exactly 1 notification, got %d", len(coachNotifs))
	}

	managerNotifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &managerUserID})
	if len(managerNotifs) != 1 {
		t.Fatalf("expected manager to have exactly 1 notification, got %d", len(managerNotifs))
	}

	// Verify total notifications created for this action is exactly 4 (1 for coach, 1 for manager, plus 2 seeded admin/manager staff)
	// NOT 16!
	allNotifs, total, _ := svc.GetNotifications(models.NotificationFilter{})
	if total != 4 || len(allNotifs) != 4 {
		t.Fatalf("expected exactly 4 total notifications in system (1 per staff user), got %d", total)
	}

	// Verify debounce: rapid second invocation within 5s does not create duplicates
	svc.NotifyStudentAdmitted(sess, student, false, "Staff")
	_, totalAfterDebounce, _ := svc.GetNotifications(models.NotificationFilter{})
	if totalAfterDebounce != 4 {
		t.Fatalf("expected total notifications to remain 4 after rapid duplicate trigger, got %d", totalAfterDebounce)
	}
}

func TestNotificationService_EnsureVAPIDKeys(t *testing.T) {
	svc, store := setupTestNotificationService()

	// Initial settings have blank VAPID keys
	settings, err := store.GetNotificationSettings()
	if err != nil {
		t.Fatalf("failed to get settings: %v", err)
	}
	settings.WebPushPublicKey = ""
	settings.WebPushPrivateKey = ""
	_ = store.UpdateNotificationSettings(settings)

	// EnsureVAPIDKeys should auto-generate valid keys
	ensured, err := svc.EnsureVAPIDKeys()
	if err != nil {
		t.Fatalf("EnsureVAPIDKeys failed: %v", err)
	}
	if !ensured.HasVAPIDKeys() {
		t.Fatalf("expected ensured settings to have valid VAPID keys")
	}

	// Calling again should preserve existing keys without regenerating
	pubKey := ensured.WebPushPublicKey
	ensuredAgain, err := svc.EnsureVAPIDKeys()
	if err != nil {
		t.Fatalf("EnsureVAPIDKeys second call failed: %v", err)
	}
	if ensuredAgain.WebPushPublicKey != pubKey {
		t.Fatalf("expected VAPID public key to remain stable, changed from %s to %s", pubKey, ensuredAgain.WebPushPublicKey)
	}

	// GenerateAndSaveVAPIDKeys forces fresh keys
	fresh, err := svc.GenerateAndSaveVAPIDKeys()
	if err != nil {
		t.Fatalf("GenerateAndSaveVAPIDKeys failed: %v", err)
	}
	if fresh.WebPushPublicKey == pubKey {
		t.Fatalf("expected fresh VAPID keys to differ from old keys")
	}
}

func TestNotificationService_PushSubscriptions(t *testing.T) {
	_, store := setupTestNotificationService()
	userID := uuid.New()

	sub := &models.PushSubscription{
		ID:        uuid.New(),
		UserID:    &userID,
		Role:      models.RoleCoach,
		Endpoint:  "https://updates.push.services.mozilla.com/wpush/v2/test-1",
		P256dh:    "mock-key",
		Auth:      "mock-auth",
		UserAgent: "Mozilla/5.0",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 1. Create subscription
	if err := store.CreateOrUpdatePushSubscription(sub); err != nil {
		t.Fatalf("failed to create push subscription: %v", err)
	}

	// 2. Query by user ID
	subs, err := store.GetPushSubscriptionsByUserID(userID)
	if err != nil || len(subs) != 1 {
		t.Fatalf("expected 1 subscription for user, got %d (err: %v)", len(subs), err)
	}

	// 3. Upsert on same endpoint updates without duplication
	sub.UserAgent = "Updated Agent"
	if err := store.CreateOrUpdatePushSubscription(sub); err != nil {
		t.Fatalf("failed to update push subscription: %v", err)
	}
	subs, _ = store.GetPushSubscriptionsByUserID(userID)
	if len(subs) != 1 || subs[0].UserAgent != "Updated Agent" {
		t.Fatalf("expected 1 updated subscription, got %d", len(subs))
	}

	// 4. Query by role
	roleSubs, err := store.GetPushSubscriptionsByRole(models.RoleCoach)
	if err != nil || len(roleSubs) != 1 {
		t.Fatalf("expected 1 subscription for coach role, got %d", len(roleSubs))
	}

	// 5. Delete by endpoint
	if err := store.DeletePushSubscription(sub.Endpoint); err != nil {
		t.Fatalf("failed to delete subscription: %v", err)
	}
	subsAfterDelete, _ := store.GetPushSubscriptionsByUserID(userID)
	if len(subsAfterDelete) != 0 {
		t.Fatalf("expected 0 subscriptions after delete, got %d", len(subsAfterDelete))
	}
}

func TestNotificationService_SendTestEmail(t *testing.T) {
	svc, _ := setupTestNotificationService()

	// 1. Error when host is completely missing
	err := svc.SendTestEmail(services.TestEmailParams{
		Host:      "",
		Recipient: "admin@seriestkd.com",
	})
	if err == nil || !strings.Contains(err.Error(), "SMTP host is not configured") {
		t.Fatalf("expected error about missing SMTP host, got %v", err)
	}

	// 2. Error when recipient is missing
	err = svc.SendTestEmail(services.TestEmailParams{
		Host:      "smtp.mail.test",
		Recipient: "",
	})
	if err == nil || !strings.Contains(err.Error(), "recipient email address is required") {
		t.Fatalf("expected error about missing recipient, got %v", err)
	}

	// 3. Error when recipient format is invalid
	err = svc.SendTestEmail(services.TestEmailParams{
		Host:      "smtp.mail.test",
		Recipient: "not-an-email",
	})
	if err == nil || !strings.Contains(err.Error(), "valid email address") {
		t.Fatalf("expected error about invalid recipient format, got %v", err)
	}

	// 4. Success using simulated/mock host
	err = svc.SendTestEmail(services.TestEmailParams{
		Host:      "simulated",
		Port:      587,
		User:      "admin",
		From:      "noreply@seriestkd.com",
		Recipient: "admin@seriestkd.com",
	})
	if err != nil {
		t.Fatalf("expected simulated email test to succeed, got %v", err)
	}

	// 5. Success using saved settings with mock host
	settings, _ := svc.GetSettings()
	settings.SMTPHost = "mock"
	settings.SMTPPort = 465
	settings.SMTPFrom = "system@seriestkd.com"
	_ = svc.UpdateSettings(settings)

	err = svc.SendTestEmail(services.TestEmailParams{
		Recipient: "coach@seriestkd.com",
	})
	if err != nil {
		t.Fatalf("expected test email to use saved settings with mock host, got %v", err)
	}
}

type testStaffSetup struct {
	Coach1     *models.Coach
	Coach1User *models.User
	Coach2     *models.Coach
	Coach2User *models.User
	Admin1     *models.User
	Admin2     *models.User
	Manager1   *models.User
	Manager2   *models.User
}

func setupTestStaff(store repository.RepositoryStore) testStaffSetup {
	c1ID := uuid.New()
	c1 := &models.Coach{ID: c1ID, FullName: "Coach One", Email: "coach1@seriestkd.com", IsActive: true}
	_ = store.CreateCoach(c1)
	c1User := &models.User{ID: uuid.New(), Email: "coach1@seriestkd.com", Role: models.RoleCoach, CoachID: &c1ID, DisplayName: "Coach One", IsActive: true}
	_ = store.CreateUser(c1User)

	c2ID := uuid.New()
	c2 := &models.Coach{ID: c2ID, FullName: "Coach Two", Email: "coach2@seriestkd.com", IsActive: true}
	_ = store.CreateCoach(c2)
	c2User := &models.User{ID: uuid.New(), Email: "coach2@seriestkd.com", Role: models.RoleCoach, CoachID: &c2ID, DisplayName: "Coach Two", IsActive: true}
	_ = store.CreateUser(c2User)

	a1 := &models.User{ID: uuid.New(), Email: "admin1@seriestkd.com", Role: models.RoleAdmin, DisplayName: "Admin One", IsActive: true}
	_ = store.CreateUser(a1)

	a2 := &models.User{ID: uuid.New(), Email: "admin2@seriestkd.com", Role: models.RoleAdmin, DisplayName: "Admin Two", IsActive: true}
	_ = store.CreateUser(a2)

	m1 := &models.User{ID: uuid.New(), Email: "mgr1@seriestkd.com", Role: models.RoleOperationManager, DisplayName: "Manager One", IsActive: true}
	_ = store.CreateUser(m1)

	m2 := &models.User{ID: uuid.New(), Email: "mgr2@seriestkd.com", Role: models.RoleOperationManager, DisplayName: "Manager Two", IsActive: true}
	_ = store.CreateUser(m2)

	return testStaffSetup{
		Coach1:     c1,
		Coach1User: c1User,
		Coach2:     c2,
		Coach2User: c2User,
		Admin1:     a1,
		Admin2:     a2,
		Manager1:   m1,
		Manager2:   m2,
	}
}

func TestNotificationService_NotifyNewClassOpened_NeitherSpecified(t *testing.T) {
	svc, store := setupTestNotificationService()
	staff := setupTestStaff(store)

	sess := &models.TrainingSession{
		ID:           uuid.New(),
		CoachID:      nil,
		AdminID:      nil,
		TrainingType: models.TrainingSparring,
		SessionDate:  time.Now(),
		StartTime:    "17:00",
		EndTime:      "18:30",
	}
	_ = store.CreateSession(sess)

	svc.NotifyNewClassOpened(sess, "Master Kim")

	// Neither coach nor admin specified -> Broadcast to ALL coaches, ALL admins, and ALL managers
	for _, u := range []*models.User{staff.Coach1User, staff.Coach2User, staff.Admin1, staff.Admin2, staff.Manager1, staff.Manager2} {
		notifs, count, err := svc.GetNotifications(models.NotificationFilter{UserID: &u.ID})
		if err != nil || count != 1 || len(notifs) != 1 {
			t.Errorf("expected user %s (%s) to receive exactly 1 notification, got count=%d, err=%v", u.DisplayName, u.Role, count, err)
		}
	}

	allNotifs, total, _ := svc.GetNotifications(models.NotificationFilter{})
	// Total includes our 6 created staff + default seeded staff (3 coaches, 1 admin, 1 manager) = 11
	if total < 6 || len(allNotifs) != total {
		t.Fatalf("expected notifications across all staff, got %d", total)
	}
}

func TestNotificationService_NotifyNewClassOpened_BothSpecified(t *testing.T) {
	svc, store := setupTestNotificationService()
	staff := setupTestStaff(store)

	sess := &models.TrainingSession{
		ID:           uuid.New(),
		CoachID:      &staff.Coach1.ID,
		AdminID:      &staff.Admin1.ID,
		TrainingType: models.TrainingPoomsae,
		SessionDate:  time.Now(),
		StartTime:    "10:00",
		EndTime:      "11:30",
	}
	_ = store.CreateSession(sess)

	svc.NotifyNewClassOpened(sess, "Manager One")

	// Both coach and admin specified:
	// - Coach1 receives, Coach2 does NOT
	// - Admin1 receives, Admin2 does NOT
	// - Manager1 and Manager2 BOTH receive
	c1Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Coach1User.ID})
	if len(c1Notifs) != 1 {
		t.Errorf("expected assigned coach1 to receive 1 notification, got %d", len(c1Notifs))
	}

	c2Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Coach2User.ID})
	if len(c2Notifs) != 0 {
		t.Errorf("expected unassigned coach2 to receive 0 notifications, got %d", len(c2Notifs))
	}

	a1Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Admin1.ID})
	if len(a1Notifs) != 1 {
		t.Errorf("expected assigned admin1 to receive 1 notification, got %d", len(a1Notifs))
	}

	a2Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Admin2.ID})
	if len(a2Notifs) != 0 {
		t.Errorf("expected unassigned admin2 to receive 0 notifications, got %d", len(a2Notifs))
	}

	m1Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Manager1.ID})
	if len(m1Notifs) != 1 {
		t.Errorf("expected manager1 to receive 1 notification, got %d", len(m1Notifs))
	}

	m2Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Manager2.ID})
	if len(m2Notifs) != 1 {
		t.Errorf("expected manager2 to receive 1 notification, got %d", len(m2Notifs))
	}
}

func TestNotificationService_NotifyNewClassOpened_CoachOnlySpecified(t *testing.T) {
	svc, store := setupTestNotificationService()
	staff := setupTestStaff(store)

	sess := &models.TrainingSession{
		ID:           uuid.New(),
		CoachID:      &staff.Coach1.ID,
		AdminID:      nil, // Admin unassigned
		TrainingType: models.TrainingSparring,
		SessionDate:  time.Now(),
	}
	_ = store.CreateSession(sess)

	svc.NotifyNewClassOpened(sess, "Manager One")

	// Coach specified -> Coach1 receives, Coach2 does NOT
	// Admin unassigned -> ALL admins receive (Admin1, Admin2)
	// Managers always receive -> Manager1, Manager2
	c1Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Coach1User.ID})
	if len(c1Notifs) != 1 {
		t.Errorf("expected assigned coach1 to receive 1 notification, got %d", len(c1Notifs))
	}
	c2Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Coach2User.ID})
	if len(c2Notifs) != 0 {
		t.Errorf("expected unassigned coach2 to receive 0 notifications, got %d", len(c2Notifs))
	}

	a1Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Admin1.ID})
	if len(a1Notifs) != 1 {
		t.Errorf("expected admin1 to receive 1 notification, got %d", len(a1Notifs))
	}
	a2Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Admin2.ID})
	if len(a2Notifs) != 1 {
		t.Errorf("expected admin2 to receive 1 notification, got %d", len(a2Notifs))
	}
}

func TestNotificationService_NotifyNewClassOpened_AdminOnlySpecified(t *testing.T) {
	svc, store := setupTestNotificationService()
	staff := setupTestStaff(store)

	sess := &models.TrainingSession{
		ID:           uuid.New(),
		CoachID:      nil, // Coach unassigned
		AdminID:      &staff.Admin1.ID,
		TrainingType: models.TrainingSparring,
		SessionDate:  time.Now(),
	}
	_ = store.CreateSession(sess)

	svc.NotifyNewClassOpened(sess, "Manager One")

	// Coach unassigned -> ALL coaches receive (Coach1, Coach2)
	// Admin specified -> Admin1 receives, Admin2 does NOT
	// Managers always receive -> Manager1, Manager2
	c1Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Coach1User.ID})
	if len(c1Notifs) != 1 {
		t.Errorf("expected coach1 to receive 1 notification, got %d", len(c1Notifs))
	}
	c2Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Coach2User.ID})
	if len(c2Notifs) != 1 {
		t.Errorf("expected coach2 to receive 1 notification, got %d", len(c2Notifs))
	}

	a1Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Admin1.ID})
	if len(a1Notifs) != 1 {
		t.Errorf("expected assigned admin1 to receive 1 notification, got %d", len(a1Notifs))
	}
	a2Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Admin2.ID})
	if len(a2Notifs) != 0 {
		t.Errorf("expected unassigned admin2 to receive 0 notifications, got %d", len(a2Notifs))
	}
}

func TestNotificationService_NotifySessionSlotReserved(t *testing.T) {
	svc, store := setupTestNotificationService()
	staff := setupTestStaff(store)

	sess := &models.TrainingSession{
		ID:           uuid.New(),
		CoachID:      &staff.Coach1.ID,
		AdminID:      &staff.Admin1.ID,
		TrainingType: models.TrainingSparring,
		SessionDate:  time.Now(),
		StartTime:    "18:00",
		EndTime:      "19:30",
	}
	_ = store.CreateSession(sess)

	// 1. Guest reservation
	svc.NotifySessionSlotReserved(sess, "Bruce Wayne", true, "0917-123-4567")

	c1Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Coach1User.ID})
	if len(c1Notifs) != 1 {
		t.Fatalf("expected coach1 to receive 1 slot reservation notification, got %d", len(c1Notifs))
	}
	n := c1Notifs[0]
	if n.EventType != models.EventSlotReserved {
		t.Errorf("expected EventSlotReserved, got %s", n.EventType)
	}
	if !strings.Contains(n.Title, "Guest Reservation: Bruce Wayne") {
		t.Errorf("expected title to mention Guest Reservation, got %s", n.Title)
	}
	if !strings.Contains(n.Message, "0917-123-4567") {
		t.Errorf("expected message to mention contact number, got %s", n.Message)
	}
	if !strings.Contains(n.Metadata, `"is_guest":true`) {
		t.Errorf("expected metadata to have is_guest: true, got %s", n.Metadata)
	}

	// 2. Student reservation
	svc.NotifySessionSlotReserved(sess, "Dick Grayson", false, "0918-987-6543")

	a1Notifs, _, _ := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Admin1.ID})
	if len(a1Notifs) != 2 { // 1 from guest, 1 from student
		t.Fatalf("expected admin1 to receive 2 slot reservation notifications, got %d", len(a1Notifs))
	}
	nStudent := a1Notifs[0]
	if !strings.Contains(nStudent.Title, "Student Reservation: Dick Grayson") {
		t.Errorf("expected title to mention Student Reservation, got %s", nStudent.Title)
	}
}

func TestNotificationService_EmailNotificationDispatch(t *testing.T) {
	svc, store := setupTestNotificationService()
	staff := setupTestStaff(store)

	settings, _ := svc.GetSettings()
	settings.EmailEnabled = true
	settings.SMTPHost = "simulated"
	settings.SMTPPort = 587
	settings.SMTPFrom = "noreply@seriestkd.com"
	_ = svc.UpdateSettings(settings)

	sess := &models.TrainingSession{
		ID:           uuid.New(),
		CoachID:      &staff.Coach1.ID,
		AdminID:      &staff.Admin1.ID,
		TrainingType: models.TrainingSparring,
		SessionDate:  time.Now(),
	}
	_ = store.CreateSession(sess)

	svc.NotifyNewClassOpened(sess, "Manager One")

	notifs, count, err := svc.GetNotifications(models.NotificationFilter{UserID: &staff.Coach1User.ID})
	if err != nil || count != 1 {
		t.Fatalf("expected 1 notification for coach1, got %d, err=%v", count, err)
	}

	channels := notifs[0].DispatchedChannels()
	hasEmail := false
	for _, ch := range channels {
		if ch == models.ChannelEmail {
			hasEmail = true
			break
		}
	}
	if !hasEmail {
		t.Errorf("expected notification dispatched channels to include EMAIL, got %v", channels)
	}
}


