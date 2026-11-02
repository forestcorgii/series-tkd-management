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

	// 1. Student User -> Defaults to landing page with calendar view (200 OK)
	studentUser, err := store.GetUserByEmail("alex.vance@seriestkd.com")
	if err != nil || studentUser == nil {
		t.Fatalf("expected seeded student user: %v", err)
	}
	studentToken := uuid.New().String()
	_ = store.CreateSessionToken(studentToken, studentUser.ID, time.Now().Add(time.Hour))

	// Seed a future session at the default location that Alex is NOT registered for
	locs, _ := store.GetAllLocations()
	if len(locs) > 0 {
		_ = store.SetDefaultLocation(locs[0].ID)
		futureSess := &models.TrainingSession{
			ID:           uuid.New(),
			Title:        "Open Sparring Prep",
			TrainingType: models.TrainingSparring,
			LocationID:   &locs[0].ID,
			LocationName: locs[0].Name,
			SessionDate:  time.Now().In(time.FixedZone("Asia/Manila", 8*3600)).AddDate(0, 0, 1),
			StartTime:    "15:00",
			EndTime:      "16:30",
		}
		_ = store.CreateSession(futureSess)
	}

	reqStudent := httptest.NewRequest("GET", "/?location=all", nil)
	reqStudent.AddCookie(&http.Cookie{Name: "stms_session", Value: studentToken})
	recStudent := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleLandingPage)).ServeHTTP(recStudent, reqStudent)

	if recStudent.Code != http.StatusOK {
		t.Errorf("expected 200 OK for logged in student on default landing calendar, got %d", recStudent.Code)
	}
	if !strings.Contains(recStudent.Body.String(), "landing-calendar-container") {
		t.Errorf("expected landing page calendar container for student")
	}
	if strings.Contains(recStudent.Body.String(), "previewing the public landing page") {
		t.Errorf("student should not see preview notice on their default page")
	}
	if !strings.Contains(recStudent.Body.String(), `id="guest-reserve-modal"`) {
		t.Errorf("logged in student should have reserve modal available on landing calendar")
	}
	if !strings.Contains(recStudent.Body.String(), "Reserved") {
		t.Errorf("logged in student should see Reserved status for their enrolled class")
	}
	if !strings.Contains(recStudent.Body.String(), "Reserve Slot") {
		t.Errorf("logged in student should see Reserve Slot button for open classes")
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

	// 3. Logged in student accessing reservation form:
	// Renders simplified practitioner confirmation view WITHOUT text input form fields (no guest_name or contact_number inputs)
	studentUser, _ := store.GetUserByEmail("alex.vance@seriestkd.com")
	studentToken := uuid.New().String()
	_ = store.CreateSessionToken(studentToken, studentUser.ID, time.Now().Add(time.Hour))

	targetSess := &models.TrainingSession{
		ID:           uuid.New(),
		Title:        "Saturday Form Practice",
		TrainingType: models.TrainingPoomsae,
		SessionDate:  time.Now().AddDate(0, 0, 2),
		StartTime:    "10:00",
		EndTime:      "11:30",
	}
	_ = store.CreateSession(targetSess)

	reqStudent := httptest.NewRequest("GET", "/landing/reserve-form?session_id="+targetSess.ID.String(), nil)
	reqStudent.AddCookie(&http.Cookie{Name: "stms_session", Value: studentToken})
	recStudent := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleGuestReserveForm)).ServeHTTP(recStudent, reqStudent)
	if recStudent.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for student on reserve form, got %d", recStudent.Code)
	}
	studentBody := recStudent.Body.String()
	if !strings.Contains(studentBody, "Alex Vance") {
		t.Errorf("expected student name in confirmation modal, got: %s", studentBody)
	}
	if strings.Contains(studentBody, `id="guest_name"`) || strings.Contains(studentBody, `id="contact_number"`) {
		t.Errorf("logged in student should NOT see redundant text input form fields for name or phone")
	}
	if !strings.Contains(studentBody, "Confirm Slot Reservation") {
		t.Errorf("expected student confirmation button in modal")
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

	// 5. Logged in student can reserve a slot directly without filling out name or contact number
	studentUser, _ := store.GetUserByEmail("alex.vance@seriestkd.com")
	studentToken := uuid.New().String()
	_ = store.CreateSessionToken(studentToken, studentUser.ID, time.Now().Add(time.Hour))

	studentFormData := url.Values{
		"session_id": {sess.ID.String()},
	}
	recStudentPost := httptest.NewRecorder()
	reqStudentPost := httptest.NewRequest("POST", "/landing/reserve", strings.NewReader(studentFormData.Encode()))
	reqStudentPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqStudentPost.AddCookie(&http.Cookie{Name: "stms_session", Value: studentToken})
	app.AuthMiddleware(http.HandlerFunc(app.HandleGuestReservation)).ServeHTTP(recStudentPost, reqStudentPost)

	if recStudentPost.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for student reserving slot, got %d (body: %s)", recStudentPost.Code, recStudentPost.Body.String())
	}
	studentRespBody := recStudentPost.Body.String()
	if !strings.Contains(studentRespBody, "Slot Reservation Confirmed") {
		t.Errorf("expected success banner for student, got: %s", studentRespBody)
	}
	if !strings.Contains(studentRespBody, "Alex Vance") {
		t.Errorf("expected student full name in confirmation, got: %s", studentRespBody)
	}

	// Verify student attendance record in store
	attendancesStudent, err := store.GetSessionAttendances(sess.ID)
	if err != nil || len(attendancesStudent) != 2 {
		t.Fatalf("expected 2 attendances in store (1 guest + 1 student), got %d (err: %v)", len(attendancesStudent), err)
	}
	var studentAtt *models.Attendance
	for _, a := range attendancesStudent {
		if a.AttendeeType == "student" {
			studentAtt = a
			break
		}
	}
	if studentAtt == nil {
		t.Fatalf("expected student attendance record to be created")
	}
	if studentAtt.StudentName != "Alex Vance" && studentAtt.AttendeeName != "Alex Vance" {
		t.Errorf("expected student name Alex Vance, got %s / %s", studentAtt.StudentName, studentAtt.AttendeeName)
	}

	// 6. Duplicate student reservation is rejected
	recStudentDup := httptest.NewRecorder()
	reqStudentDup := httptest.NewRequest("POST", "/landing/reserve", strings.NewReader(studentFormData.Encode()))
	reqStudentDup.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqStudentDup.AddCookie(&http.Cookie{Name: "stms_session", Value: studentToken})
	app.AuthMiddleware(http.HandlerFunc(app.HandleGuestReservation)).ServeHTTP(recStudentDup, reqStudentDup)

	if !strings.Contains(recStudentDup.Body.String(), "already reserved a slot") {
		t.Errorf("expected duplicate reservation message for student, got: %s", recStudentDup.Body.String())
	}
}
