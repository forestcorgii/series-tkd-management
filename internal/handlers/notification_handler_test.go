package handlers_test

import (
	"encoding/json"
	"fmt"
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

func TestNotificationHandler_UpdateSettings_DisablingEmailAndEnablingTelegram(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	token := uuid.New().String()
	_ = store.CreateSessionToken(token, managerUser.ID, time.Now().Add(time.Hour))

	// Post settings update with email UNCHECKED and telegram CHECKED
	form := url.Values{}
	form.Set("telegram_enabled", "on")
	form.Set("telegram_bot_token", "test-token-123")
	form.Set("telegram_public_chat_id", "@seriestkd_test")
	form.Set("telegram_staff_chat_id", "-1001234567890")
	form.Set("notify_telegram_new_class", "on")
	form.Set("notify_telegram_new_student", "on")
	form.Set("notify_telegram_slot_reserved", "on")
	// Note: email_enabled is intentionally NOT set!
	form.Set("notify_admin_student_admitted", "on")
	form.Set("notify_student_promotion_eligible", "on")
	form.Set("smtp_host", "mail.seriestkd.com")
	form.Set("smtp_port", "465")
	form.Set("smtp_from", "notifications@seriestkd.com")
	form.Set("smtp_from_name", "Series Taekwondo Central")

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

	if !saved.TelegramEnabled {
		t.Errorf("expected TelegramEnabled to be true")
	}
	if saved.TelegramBotToken != "test-token-123" {
		t.Errorf("expected telegram_bot_token test-token-123, got %s", saved.TelegramBotToken)
	}
	if saved.TelegramPublicChatID != "@seriestkd_test" {
		t.Errorf("expected telegram_public_chat_id @seriestkd_test, got %s", saved.TelegramPublicChatID)
	}
	if saved.TelegramStaffChatID != "-1001234567890" {
		t.Errorf("expected telegram_staff_chat_id -1001234567890, got %s", saved.TelegramStaffChatID)
	}
	if saved.EmailEnabled {
		t.Errorf("expected EmailEnabled to be disabled (false)")
	}
	if saved.SMTPHost != "mail.seriestkd.com" || saved.SMTPPort != 465 {
		t.Errorf("expected SMTP host/port mail.seriestkd.com:465, got %s:%d", saved.SMTPHost, saved.SMTPPort)
	}
	if saved.SMTPFromName != "Series Taekwondo Central" {
		t.Errorf("expected SMTPFromName 'Series Taekwondo Central', got %q", saved.SMTPFromName)
	}
}

func TestNotificationHandler_NoDuplicateNotificationsDropdown(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	token := uuid.New().String()
	_ = store.CreateSessionToken(token, managerUser.ID, time.Now().Add(time.Hour))

	coaches, _ := store.GetAllCoaches()
	var coachID *uuid.UUID
	if len(coaches) > 0 {
		coachID = &coaches[0].ID
	}
	sess := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  time.Now(),
		StartTime:    "10:00",
		EndTime:      "11:30",
		CoachID:      coachID,
		TrainingType: models.TrainingSparring,
	}
	_ = store.CreateSession(sess)

	students, _ := store.GetAllStudents()
	if len(students) == 0 {
		t.Fatal("expected test students")
	}

	var targetStudent *models.Student
	for _, s := range students {
		if s.FullName == "Alex Vance" {
			targetStudent = s
			break
		}
	}
	if targetStudent == nil {
		targetStudent = students[0]
	}

	// Trigger a single check-in via HTTP
	reqCheckIn := httptest.NewRequest("POST", fmt.Sprintf("/sessions/%s/checkin/%s", sess.ID, targetStudent.ID), nil)
	reqCheckIn.SetPathValue("id", sess.ID.String())
	reqCheckIn.SetPathValue("student_id", targetStudent.ID.String())
	reqCheckIn.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recCheckIn := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleCheckIn)).ServeHTTP(recCheckIn, reqCheckIn)

	if recCheckIn.Code != http.StatusOK {
		t.Fatalf("expected check-in status 200, got %d: %s", recCheckIn.Code, recCheckIn.Body.String())
	}

	// Fetch notifications for the manager
	reqAPI := httptest.NewRequest("GET", "/api/notifications", nil)
	reqAPI.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recAPI := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleGetNotificationsAPI)).ServeHTTP(recAPI, reqAPI)

	var apiResp struct {
		Notifications []*models.Notification `json:"notifications"`
		Total         int                    `json:"total"`
		UnreadCount   int                    `json:"unread_count"`
	}
	_ = json.Unmarshal(recAPI.Body.Bytes(), &apiResp)

	// Verify that the manager receives EXACTLY 2 distinct event notifications for this action
	// (1 for EventStudentAdmitted + 1 for EventPromotionEligible), with ZERO channel duplicates (previously 8 notifications)
	if apiResp.Total != 2 {
		t.Fatalf("expected exactly 2 distinct event notifications for the manager, got %d", apiResp.Total)
	}
	if apiResp.UnreadCount != 2 {
		t.Fatalf("expected unread count to be 2, got %d", apiResp.UnreadCount)
	}

	seenEvents := make(map[models.NotificationEventType]int)
	for _, n := range apiResp.Notifications {
		seenEvents[n.EventType]++
	}
	if seenEvents[models.EventStudentAdmitted] != 1 {
		t.Errorf("expected exactly 1 EventStudentAdmitted, got %d", seenEvents[models.EventStudentAdmitted])
	}
	if seenEvents[models.EventPromotionEligible] != 1 {
		t.Errorf("expected exactly 1 EventPromotionEligible, got %d", seenEvents[models.EventPromotionEligible])
	}

	// Verify the dropdown renders exactly 2 notification items
	reqDropdown := httptest.NewRequest("GET", "/api/notifications/dropdown", nil)
	reqDropdown.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recDropdown := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleNotificationDropdown)).ServeHTTP(recDropdown, reqDropdown)

	dropdownHTML := recDropdown.Body.String()
	countNotifItems := strings.Count(dropdownHTML, "id=\"notif-item-")
	if countNotifItems != 2 {
		t.Fatalf("expected exactly 2 notification items rendered in dropdown, got %d", countNotifItems)
	}
}

func TestTelegramNotification_Handlers(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	token := uuid.New().String()
	_ = store.CreateSessionToken(token, managerUser.ID, time.Now().Add(time.Hour))

	// POST /settings/notifications/test-telegram
	form := url.Values{}
	form.Set("telegram_bot_token", "simulated")
	form.Set("telegram_public_chat_id", "@seriestkd_test")

	reqTest := httptest.NewRequest("POST", "/settings/notifications/test-telegram", strings.NewReader(form.Encode()))
	reqTest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqTest.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recTest := httptest.NewRecorder()

	app.AuthMiddleware(http.HandlerFunc(app.HandleTestTelegram)).ServeHTTP(recTest, reqTest)

	if recTest.Code != http.StatusOK {
		t.Fatalf("expected 200 for telegram test, got %d: %s", recTest.Code, recTest.Body.String())
	}

	if !strings.Contains(recTest.Body.String(), "Connected to bot") {
		t.Errorf("expected success badge in response, got: %s", recTest.Body.String())
	}
}

func TestEmailNotification_Handlers(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	token := uuid.New().String()
	_ = store.CreateSessionToken(token, managerUser.ID, time.Now().Add(time.Hour))

	// 1. Unauthorized request
	reqUnauth := httptest.NewRequest("POST", "/api/email/test", nil)
	recUnauth := httptest.NewRecorder()
	app.HandleEmailTest(recUnauth, reqUnauth)
	if recUnauth.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauthenticated request, got %d", recUnauth.Code)
	}

	// 2. Error when SMTP host not configured
	emptyPayload := `{"smtp_host":"","recipient_email":"admin@seriestkd.com"}`
	reqEmpty := httptest.NewRequest("POST", "/api/email/test", strings.NewReader(emptyPayload))
	reqEmpty.Header.Set("Content-Type", "application/json")
	reqEmpty.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recEmpty := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleEmailTest)).ServeHTTP(recEmpty, reqEmpty)

	if recEmpty.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty SMTP host, got %d: %s", recEmpty.Code, recEmpty.Body.String())
	}
	var errResp map[string]string
	_ = json.Unmarshal(recEmpty.Body.Bytes(), &errResp)
	if !strings.Contains(errResp["error"], "SMTP host is not configured") {
		t.Errorf("expected SMTP host error, got %s", errResp["error"])
	}

	// 3. Success with simulated SMTP host via JSON including display name
	simPayload := `{"smtp_host":"simulated","smtp_port":587,"smtp_from":"test@seriestkd.com","smtp_from_name":"Series TKD HQ","recipient_email":"manager@seriestkd.com"}`
	reqSim := httptest.NewRequest("POST", "/api/email/test", strings.NewReader(simPayload))
	reqSim.Header.Set("Content-Type", "application/json")
	reqSim.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recSim := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleEmailTest)).ServeHTTP(recSim, reqSim)

	if recSim.Code != http.StatusOK {
		t.Fatalf("expected 200 for simulated email test, got %d: %s", recSim.Code, recSim.Body.String())
	}
	var okResp map[string]interface{}
	_ = json.Unmarshal(recSim.Body.Bytes(), &okResp)
	if okResp["status"] != "ok" {
		t.Errorf("expected status ok, got %v", okResp["status"])
	}

	// 4. Success via Form POST with HTML redirect and display name
	form := url.Values{}
	form.Set("smtp_host", "simulated")
	form.Set("smtp_port", "587")
	form.Set("smtp_from", "notifications@seriestkd.com")
	form.Set("smtp_from_name", "Series Taekwondo Main")
	form.Set("recipient_email", "admin@seriestkd.com")

	reqForm := httptest.NewRequest("POST", "/settings/notifications/test-email", strings.NewReader(form.Encode()))
	reqForm.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqForm.Header.Set("Accept", "text/html,application/xhtml+xml")
	reqForm.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recForm := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleEmailTest)).ServeHTTP(recForm, reqForm)

	if recForm.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect for HTML form post, got %d", recForm.Code)
	}
	location := recForm.Header().Get("Location")
	if !strings.Contains(location, "success=") || !strings.Contains(location, "tab=notifications") {
		t.Errorf("expected redirect to notification settings tab with success, got %s", location)
	}
}
