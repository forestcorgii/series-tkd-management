package models_test

import (
	"testing"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
)

func TestDefaultNotificationSettings(t *testing.T) {
	settings := models.DefaultNotificationSettings()
	if !settings.EmailEnabled {
		t.Errorf("expected EmailEnabled default true")
	}
	if settings.TelegramEnabled {
		t.Errorf("expected TelegramEnabled default false")
	}
	if !settings.NotifyAdminStudentAdmitted {
		t.Errorf("expected NotifyAdminStudentAdmitted default true")
	}
	if !settings.NotifyStudentPromotionEligible {
		t.Errorf("expected NotifyStudentPromotionEligible default true")
	}
	if !settings.NotifyTelegramNewClass {
		t.Errorf("expected NotifyTelegramNewClass default true")
	}
	if !settings.NotifyTelegramNewStudent {
		t.Errorf("expected NotifyTelegramNewStudent default true")
	}
	if !settings.NotifyTelegramSlotReserved {
		t.Errorf("expected NotifyTelegramSlotReserved default true")
	}
	if settings.SMTPFromName != "Series Taekwondo" {
		t.Errorf("expected SMTPFromName default 'Series Taekwondo', got %q", settings.SMTPFromName)
	}
}

func TestNotificationSettings_DisableChannels(t *testing.T) {
	settings := models.DefaultNotificationSettings()

	// Verify channels enabled initially
	if !settings.IsChannelEnabled(models.ChannelEmail) {
		t.Errorf("expected Email enabled")
	}
	if settings.IsChannelEnabled(models.ChannelTelegram) {
		t.Errorf("expected Telegram disabled by default")
	}
	if !settings.IsChannelEnabled(models.ChannelInApp) {
		t.Errorf("expected In-App to remain enabled as system operational inbox")
	}

	// Enable Telegram and disable Email
	settings.TelegramEnabled = true
	settings.EmailEnabled = false

	if settings.IsChannelEnabled(models.ChannelEmail) {
		t.Errorf("expected Email to be disabled when EmailEnabled is false")
	}
	if !settings.IsChannelEnabled(models.ChannelTelegram) {
		t.Errorf("expected Telegram to be enabled")
	}
}

func TestNotificationSettings_IsEventEnabled(t *testing.T) {
	settings := models.DefaultNotificationSettings()

	if !settings.IsEventEnabled(models.EventStudentAdmitted) {
		t.Errorf("expected EventStudentAdmitted to be enabled")
	}
	if !settings.IsEventEnabled(models.EventStudentRegistered) {
		t.Errorf("expected EventStudentRegistered to be enabled")
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
	if !prefs.ShouldReceive(models.ChannelInApp) {
		t.Errorf("expected user in-app preference default true")
	}

	// User opts out of Email
	prefs.EmailEnabled = false
	if prefs.ShouldReceive(models.ChannelEmail) {
		t.Errorf("expected user email preference to be false when disabled")
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

func TestNotificationSettings_HasTelegramConfig(t *testing.T) {
	settings := models.DefaultNotificationSettings()
	if settings.HasTelegramConfig() {
		t.Errorf("expected HasTelegramConfig to be false initially")
	}

	settings.TelegramBotToken = "123456:ABC-DEF1234ghIkl-zyx57W2v1u123ew11"
	if settings.HasTelegramConfig() {
		t.Errorf("expected HasTelegramConfig to be false when both chat IDs are empty")
	}

	settings.TelegramPublicChatID = "@seriestkd_news"
	if !settings.HasTelegramConfig() {
		t.Errorf("expected HasTelegramConfig to be true when token and public chat ID are present")
	}

	settings.TelegramPublicChatID = ""
	settings.TelegramStaffChatID = "-1001234567890"
	if !settings.HasTelegramConfig() {
		t.Errorf("expected HasTelegramConfig to be true when token and staff chat ID are present")
	}
}

