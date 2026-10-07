package models_test

import (
	"testing"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
)

func TestDefaultNotificationSettings(t *testing.T) {
	settings := models.DefaultNotificationSettings()
	if !settings.PushEnabled {
		t.Errorf("expected PushEnabled default true")
	}
	if !settings.SMSEnabled {
		t.Errorf("expected SMSEnabled default true")
	}
	if !settings.EmailEnabled {
		t.Errorf("expected EmailEnabled default true")
	}
	if !settings.NotifyAdminStudentAdmitted {
		t.Errorf("expected NotifyAdminStudentAdmitted default true")
	}
	if !settings.NotifyStudentPromotionEligible {
		t.Errorf("expected NotifyStudentPromotionEligible default true")
	}
}

func TestNotificationSettings_DisableChannels(t *testing.T) {
	settings := models.DefaultNotificationSettings()

	// Verify channels enabled initially
	if !settings.IsChannelEnabled(models.ChannelSMS) {
		t.Errorf("expected SMS enabled")
	}
	if !settings.IsChannelEnabled(models.ChannelEmail) {
		t.Errorf("expected Email enabled")
	}
	if !settings.IsChannelEnabled(models.ChannelPush) {
		t.Errorf("expected Push enabled")
	}

	// Disable SMS and Email
	settings.SMSEnabled = false
	settings.EmailEnabled = false

	if settings.IsChannelEnabled(models.ChannelSMS) {
		t.Errorf("expected SMS to be disabled when SMSEnabled is false")
	}
	if settings.IsChannelEnabled(models.ChannelEmail) {
		t.Errorf("expected Email to be disabled when EmailEnabled is false")
	}
	if !settings.IsChannelEnabled(models.ChannelPush) {
		t.Errorf("expected Push to remain enabled")
	}
	if !settings.IsChannelEnabled(models.ChannelInApp) {
		t.Errorf("expected In-App to remain enabled as system operational inbox")
	}
}

func TestNotificationSettings_IsEventEnabled(t *testing.T) {
	settings := models.DefaultNotificationSettings()

	if !settings.IsEventEnabled(models.EventStudentAdmitted) {
		t.Errorf("expected EventStudentAdmitted to be enabled")
	}

	settings.NotifyAdminStudentAdmitted = false
	if settings.IsEventEnabled(models.EventStudentAdmitted) {
		t.Errorf("expected EventStudentAdmitted to be disabled")
	}

	settings.NotifyStudentPromotionEligible = false
	if settings.IsEventEnabled(models.EventPromotionEligible) {
		t.Errorf("expected EventPromotionEligible to be disabled")
	}
}

func TestUserNotificationPreferences(t *testing.T) {
	userID := uuid.New()
	prefs := models.DefaultUserPreferences(userID)

	if !prefs.ShouldReceive(models.ChannelEmail) {
		t.Errorf("expected user email preference default true")
	}
	if !prefs.ShouldReceive(models.ChannelSMS) {
		t.Errorf("expected user sms preference default true")
	}

	// User opts out of SMS
	prefs.SMSEnabled = false
	if prefs.ShouldReceive(models.ChannelSMS) {
		t.Errorf("expected user sms preference to be false when disabled")
	}
	if !prefs.ShouldReceive(models.ChannelEmail) {
		t.Errorf("expected user email preference to still be true")
	}
}

func TestNotification_MarkAsRead(t *testing.T) {
	n := &models.Notification{
		ID:        uuid.New(),
		Title:     "Test",
		Message:   "Test message",
		IsRead:    false,
		EventType: models.EventStudentAdmitted,
	}

	if n.IsRead {
		t.Errorf("expected initially unread")
	}

	n.MarkAsRead()
	if !n.IsRead {
		t.Errorf("expected notification to be marked as read")
	}

	if n.Icon() != "🥋" {
		t.Errorf("expected martial arts icon for student admitted, got %s", n.Icon())
	}
}
