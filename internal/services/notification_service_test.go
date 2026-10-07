package services_test

import (
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
