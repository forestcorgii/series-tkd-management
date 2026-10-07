package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
)

func TestNotificationHandler_BadgeAndDropdown(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	token := uuid.New().String()
	_ = store.CreateSessionToken(token, managerUser.ID, time.Now().Add(time.Hour))

	// Initially empty badge
	req := httptest.NewRequest("GET", "/api/notifications/badge", nil)
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	rec := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleNotificationBadge)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "hidden") {
		t.Errorf("expected empty badge to be hidden, got %s", rec.Body.String())
	}

	// Create a notification for the manager
	notif := &models.Notification{
		ID:            uuid.New(),
		UserID:        &managerUser.ID,
		RecipientRole: managerUser.Role,
		EventType:     models.EventStudentAdmitted,
		Channel:       models.ChannelInApp,
		Title:         "Student Checked In",
		Message:       "Alex has been admitted to class",
		CreatedAt:     time.Now(),
	}
	_ = store.CreateNotification(notif)

	// Now badge should show count 1
	rec = httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleNotificationBadge)).ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), ">1<") {
		t.Errorf("expected badge to contain >1<, got %s", rec.Body.String())
	}

	// Dropdown should render notification title and message
	reqDropdown := httptest.NewRequest("GET", "/api/notifications/dropdown", nil)
	reqDropdown.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recDropdown := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleNotificationDropdown)).ServeHTTP(recDropdown, reqDropdown)

	if recDropdown.Code != http.StatusOK {
		t.Fatalf("expected 200 for dropdown, got %d", recDropdown.Code)
	}
	dropdownHTML := recDropdown.Body.String()
	if !strings.Contains(dropdownHTML, "Student Checked In") {
		t.Errorf("expected dropdown to contain 'Student Checked In', got %s", dropdownHTML)
	}
	if !strings.Contains(dropdownHTML, "Alex has been admitted to class") {
		t.Errorf("expected dropdown to contain 'Alex has been admitted to class', got %s", dropdownHTML)
	}

	// API endpoint returns JSON
	reqAPI := httptest.NewRequest("GET", "/api/notifications", nil)
	reqAPI.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recAPI := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleGetNotificationsAPI)).ServeHTTP(recAPI, reqAPI)

	if recAPI.Code != http.StatusOK {
		t.Fatalf("expected 200 for API, got %d", recAPI.Code)
	}
	var apiResp struct {
		Notifications []*models.Notification `json:"notifications"`
		Total         int                    `json:"total"`
		UnreadCount   int                    `json:"unread_count"`
	}
	if err := json.Unmarshal(recAPI.Body.Bytes(), &apiResp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if apiResp.UnreadCount != 1 || len(apiResp.Notifications) != 1 {
		t.Errorf("expected 1 unread notification, got %d unread, %d items", apiResp.UnreadCount, len(apiResp.Notifications))
	}

	// Mark single notification read
	reqRead := httptest.NewRequest("POST", "/api/notifications/"+notif.ID.String()+"/read", nil)
	reqRead.SetPathValue("id", notif.ID.String())
	reqRead.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recRead := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleMarkNotificationRead)).ServeHTTP(recRead, reqRead)

	if recRead.Code != http.StatusOK {
		t.Fatalf("expected 200 on mark read, got %d", recRead.Code)
	}
	if recRead.Header().Get("HX-Trigger") != "notificationUpdated" {
		t.Errorf("expected HX-Trigger header, got %s", recRead.Header().Get("HX-Trigger"))
	}

	// Count should now be 0
	unread, _ := store.GetUnreadNotificationCount(&managerUser.ID, &managerUser.Role)
	if unread != 0 {
		t.Errorf("expected 0 unread after mark read, got %d", unread)
	}
}

func TestNotificationHandler_UpdateSettings_DisablingEmailAndSMS(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	token := uuid.New().String()
	_ = store.CreateSessionToken(token, managerUser.ID, time.Now().Add(time.Hour))

	// Post settings update with email and SMS UNCHECKED (omitted from form data)
	form := url.Values{}
	form.Set("push_enabled", "on")
	// Note: sms_enabled and email_enabled are intentionally NOT set!
	form.Set("notify_admin_student_admitted", "on")
	form.Set("notify_student_promotion_eligible", "on")
	form.Set("smtp_host", "mail.seriestkd.com")
	form.Set("smtp_port", "465")
	form.Set("sms_provider", "twilio")

	req := httptest.NewRequest("POST", "/settings/notifications", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	rec := httptest.NewRecorder()

	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager, models.RoleAdmin)(app.HandleUpdateNotificationSettings)).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}

	saved, err := store.GetNotificationSettings()
	if err != nil {
		t.Fatalf("failed to retrieve saved settings: %v", err)
	}

	if !saved.PushEnabled {
		t.Errorf("expected PushEnabled to be true")
	}
	if saved.SMSEnabled {
		t.Errorf("expected SMSEnabled to be disabled (false)")
	}
	if saved.EmailEnabled {
		t.Errorf("expected EmailEnabled to be disabled (false)")
	}
	if saved.SMTPHost != "mail.seriestkd.com" || saved.SMTPPort != 465 {
		t.Errorf("expected SMTP host/port mail.seriestkd.com:465, got %s:%d", saved.SMTPHost, saved.SMTPPort)
	}
	if saved.SMSProvider != "twilio" {
		t.Errorf("expected SMS provider twilio, got %s", saved.SMSProvider)
	}
}
