package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"series-tkd-management/internal/models"
)

func TestLandingHandler_HandleLandingPage_Guest(t *testing.T) {
	app, store := setupTestApp(t)

	// Ensure a default location exists
	locs, _ := store.GetAllLocations()
	if len(locs) > 0 {
		_ = store.SetDefaultLocation(locs[0].ID)
	}

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	app.AuthMiddleware(http.HandlerFunc(app.HandleLandingPage)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for unauthenticated guest, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()

	// 1. Verify Clean Public Headline
	if !strings.Contains(body, "Class Schedule &amp; Slot Reservations") && !strings.Contains(body, "Class Schedule & Slot Reservations") {
		t.Errorf("expected landing page to contain schedule headline")
	}

	// 2. Verify Calendar Container & Wrapper
	if !strings.Contains(body, "landing-calendar-container") {
		t.Errorf("expected landing page to contain landing-calendar-container")
	}
	if !strings.Contains(body, "landing-calendar-wrapper") {
		t.Errorf("expected landing page to contain landing-calendar-wrapper")
	}

	// 3. Verify Modal Elements
	if !strings.Contains(body, "guest-reserve-modal") {
		t.Errorf("expected landing page to contain guest-reserve-modal")
	}

	// 4. Verify Removed Sections are NOT Present
	removedSnippets := []string{
		"Where Discipline Meets",
		"Integrated Operations Portals",
		"Unified Dojang Management",
		"Begin Your Martial Arts Journey",
		"Ready to Step on the Mat?",
	}
	for _, snip := range removedSnippets {
		if strings.Contains(body, snip) {
			t.Errorf("expected landing page to NOT contain removed snippet %q", snip)
		}
	}
}

func TestLandingHandler_HandleLandingPage_AuthenticatedRedirects(t *testing.T) {
	app, store := setupTestApp(t)

	// 1. Student User -> Redirect to /portal/student
	studentUser, err := store.GetUserByEmail("alex.vance@seriestkd.com")
	if err != nil || studentUser == nil {
		t.Fatalf("expected seeded student user: %v", err)
	}
	studentToken := uuid.New().String()
	_ = store.CreateSessionToken(studentToken, studentUser.ID, time.Now().Add(time.Hour))

	reqStudent := httptest.NewRequest("GET", "/", nil)
	reqStudent.AddCookie(&http.Cookie{Name: "stms_session", Value: studentToken})
	recStudent := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleLandingPage)).ServeHTTP(recStudent, reqStudent)

	if recStudent.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for logged in student, got %d", recStudent.Code)
	}
	if recStudent.Header().Get("Location") != "/portal/student" {
		t.Errorf("expected redirect to /portal/student, got %s", recStudent.Header().Get("Location"))
	}

	// 2. Coach User -> Redirect to /sessions
	coachUser, err := store.GetUserByEmail("jiwoo.park@seriestkd.com")
	if err != nil || coachUser == nil {
		t.Fatalf("expected seeded coach user: %v", err)
	}
	coachToken := uuid.New().String()
	_ = store.CreateSessionToken(coachToken, coachUser.ID, time.Now().Add(time.Hour))

	reqCoach := httptest.NewRequest("GET", "/", nil)
	reqCoach.AddCookie(&http.Cookie{Name: "stms_session", Value: coachToken})
	recCoach := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleLandingPage)).ServeHTTP(recCoach, reqCoach)

	if recCoach.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for logged in coach, got %d", recCoach.Code)
	}
	if recCoach.Header().Get("Location") != "/sessions" {
		t.Errorf("expected redirect to /sessions, got %s", recCoach.Header().Get("Location"))
	}

	// 3. Admin User -> Redirect to /sessions
	adminUser, err := store.GetUserByEmail("admin@seriestkd.com")
	if err != nil || adminUser == nil {
		t.Fatalf("expected seeded admin user: %v", err)
	}
	adminToken := uuid.New().String()
	_ = store.CreateSessionToken(adminToken, adminUser.ID, time.Now().Add(time.Hour))

	reqAdmin := httptest.NewRequest("GET", "/", nil)
	reqAdmin.AddCookie(&http.Cookie{Name: "stms_session", Value: adminToken})
	recAdmin := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleLandingPage)).ServeHTTP(recAdmin, reqAdmin)

	if recAdmin.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for logged in admin, got %d", recAdmin.Code)
	}
	if recAdmin.Header().Get("Location") != "/sessions" {
		t.Errorf("expected redirect to /sessions, got %s", recAdmin.Header().Get("Location"))
	}

	// 4. Operation Manager -> Redirect to /dashboard
	managerUser, err := store.GetUserByEmail("manager@seriestkd.com")
	if err != nil || managerUser == nil {
		t.Fatalf("expected seeded manager user: %v", err)
	}
	managerToken := uuid.New().String()
	_ = store.CreateSessionToken(managerToken, managerUser.ID, time.Now().Add(time.Hour))

	reqMgr := httptest.NewRequest("GET", "/", nil)
	reqMgr.AddCookie(&http.Cookie{Name: "stms_session", Value: managerToken})
	recMgr := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleLandingPage)).ServeHTTP(recMgr, reqMgr)

	if recMgr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for logged in manager, got %d", recMgr.Code)
	}
	if recMgr.Header().Get("Location") != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", recMgr.Header().Get("Location"))
	}

	// 5. Preview override with ?preview=1 allows authenticated user to view landing page
	reqPreview := httptest.NewRequest("GET", "/?preview=1", nil)
	reqPreview.AddCookie(&http.Cookie{Name: "stms_session", Value: managerToken})
	recPreview := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleLandingPage)).ServeHTTP(recPreview, reqPreview)

	if recPreview.Code != http.StatusOK {
		t.Errorf("expected 200 OK for preview parameter, got %d", recPreview.Code)
	}
	if !strings.Contains(recPreview.Body.String(), "previewing the public landing page") {
		t.Errorf("expected preview banner to appear")
	}
}

func TestLandingHandler_HandleLandingSchedule_Partial(t *testing.T) {
	app, _ := setupTestApp(t)

	// 1. Initial Schedule partial load
	req := httptest.NewRequest("GET", "/landing/schedule", nil)
	rec := httptest.NewRecorder()
	app.HandleLandingSchedule(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HandleLandingSchedule, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "landing-calendar-wrapper") {
		t.Errorf("expected partial to contain landing-calendar-wrapper")
	}
	if !strings.Contains(body, "Prev Week") && !strings.Contains(body, "Prev") {
		t.Errorf("expected partial to have week navigation")
	}
}

func TestLandingHandler_HandleGuestReserveForm(t *testing.T) {
	app, store := setupTestApp(t)

	// Seed or fetch a session
	sessions, err := store.GetAllSessions()
	if err != nil || len(sessions) == 0 {
		t.Fatalf("expected seeded sessions: %v", err)
	}
	sess := sessions[0]

	// 1. Valid session_id
	req := httptest.NewRequest("GET", "/landing/reserve-form?session_id="+sess.ID.String(), nil)
	rec := httptest.NewRecorder()
	app.HandleGuestReserveForm(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HandleGuestReserveForm, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, sess.ID.String()) {
		t.Errorf("expected form to contain session ID")
	}
	if !strings.Contains(body, "guest_name") || !strings.Contains(body, "contact_number") {
		t.Errorf("expected form to contain guest_name and contact_number inputs")
	}

	// 2. Invalid session_id
	reqBad := httptest.NewRequest("GET", "/landing/reserve-form?session_id=not-a-uuid", nil)
	recBad := httptest.NewRecorder()
	app.HandleGuestReserveForm(recBad, reqBad)
	if recBad.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid session ID, got %d", recBad.Code)
	}
}

func TestLandingHandler_HandleGuestReservation(t *testing.T) {
	app, store := setupTestApp(t)

	// Seed a test session
	now := time.Now()
	sess := &models.TrainingSession{
		ID:           uuid.New(),
		Title:        "Guest Sparring Clinic",
		TrainingType: models.TrainingSparring,
		SessionDate:  now.AddDate(0, 0, 1),
		StartTime:    "17:00",
		EndTime:      "18:30",
		CreatedAt:    now,
	}
	if err := store.CreateSession(sess); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// 1. Validation failure: missing name
	formData := url.Values{
		"session_id":     {sess.ID.String()},
		"guest_name":     {""},
		"contact_number": {"0917-123-4567"},
	}
	reqNoName := httptest.NewRequest("POST", "/landing/reserve", strings.NewReader(formData.Encode()))
	reqNoName.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recNoName := httptest.NewRecorder()
	app.HandleGuestReservation(recNoName, reqNoName)

	if !strings.Contains(recNoName.Body.String(), "full name") {
		t.Errorf("expected validation error for missing name, got: %s", recNoName.Body.String())
	}

	// 2. Validation failure: missing contact number
	formDataNoContact := url.Values{
		"session_id":     {sess.ID.String()},
		"guest_name":     {"Jane Doe"},
		"contact_number": {""},
	}
	reqNoContact := httptest.NewRequest("POST", "/landing/reserve", strings.NewReader(formDataNoContact.Encode()))
	reqNoContact.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recNoContact := httptest.NewRecorder()
	app.HandleGuestReservation(recNoContact, reqNoContact)

	if !strings.Contains(recNoContact.Body.String(), "contact number") {
		t.Errorf("expected validation error for missing contact number, got: %s", recNoContact.Body.String())
	}

	// 3. Successful guest reservation
	formDataValid := url.Values{
		"session_id":     {sess.ID.String()},
		"guest_name":     {"Jane Doe"},
		"contact_number": {"0917-555-0199"},
	}
	reqValid := httptest.NewRequest("POST", "/landing/reserve", strings.NewReader(formDataValid.Encode()))
	reqValid.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recValid := httptest.NewRecorder()
	app.HandleGuestReservation(recValid, reqValid)

	if recValid.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid reservation, got %d", recValid.Code)
	}
	bodyValid := recValid.Body.String()
	if !strings.Contains(bodyValid, "Slot Reservation Confirmed") {
		t.Errorf("expected success banner, got: %s", bodyValid)
	}
	if !strings.Contains(bodyValid, "Jane Doe") || !strings.Contains(bodyValid, "0917-555-0199") {
		t.Errorf("expected success partial to contain guest name and contact number")
	}

	// Verify attendance record in store
	attendances, err := store.GetSessionAttendances(sess.ID)
	if err != nil || len(attendances) != 1 {
		t.Fatalf("expected 1 attendance record in store, got %d (err: %v)", len(attendances), err)
	}
	att := attendances[0]
	if att.AttendeeName != "Jane Doe" {
		t.Errorf("expected AttendeeName Jane Doe, got %s", att.AttendeeName)
	}
	if att.AttendeeType != "guest" {
		t.Errorf("expected AttendeeType guest, got %s", att.AttendeeType)
	}
	if att.ContactNumber != "0917-555-0199" {
		t.Errorf("expected ContactNumber 0917-555-0199, got %s", att.ContactNumber)
	}

	// 4. Duplicate reservation rejection
	recDup := httptest.NewRecorder()
	reqDup := httptest.NewRequest("POST", "/landing/reserve", strings.NewReader(formDataValid.Encode()))
	reqDup.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.HandleGuestReservation(recDup, reqDup)

	bodyDup := recDup.Body.String()
	if !strings.Contains(bodyDup, "already reserved a slot") {
		t.Errorf("expected duplicate error message, got: %s", bodyDup)
	}
}
