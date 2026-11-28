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

func TestSessionReservePage_GuestView(t *testing.T) {
	app, store := setupTestApp(t)

	locs, _ := store.GetAllLocations()
	coaches, _ := store.GetAllCoaches()
	var coachID *uuid.UUID
	expectedCoach := ""
	if len(coaches) > 0 {
		coachID = &coaches[0].ID
		expectedCoach = coaches[0].FullName
	}

	sess := &models.TrainingSession{
		ID:           uuid.New(),
		Title:        "Advanced Sparring Drill",
		TrainingType: models.TrainingSparring,
		LocationID:   &locs[0].ID,
		LocationName: locs[0].Name,
		CoachID:      coachID,
		SessionDate:  time.Now().AddDate(0, 0, 2),
		StartTime:    "16:00",
		EndTime:      "17:30",
	}
	if err := store.CreateSession(sess); err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	req := httptest.NewRequest("GET", "/reserve/"+sess.ID.String(), nil)
	req.SetPathValue("id", sess.ID.String())
	rec := httptest.NewRecorder()

	app.AuthMiddleware(http.HandlerFunc(app.HandleSessionReservePage)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Advanced Sparring Drill") {
		t.Errorf("expected page to contain session title")
	}
	if expectedCoach != "" && !strings.Contains(body, expectedCoach) {
		t.Errorf("expected page to contain coach name %q", expectedCoach)
	}
	if !strings.Contains(body, "reserve_guest_name") {
		t.Errorf("expected guest reservation name field for unauthenticated visitor")
	}
	if !strings.Contains(body, "reserve_contact_number") {
		t.Errorf("expected guest contact number field")
	}
	if !strings.Contains(body, "/login?redirect=/reserve/"+sess.ID.String()) {
		t.Errorf("expected student login shortcut with return redirect")
	}
	if !strings.Contains(body, "copySessionReservationLink") {
		t.Errorf("expected copy link action on reservation page")
	}
}

func TestSessionReservePage_StudentView(t *testing.T) {
	app, store := setupTestApp(t)

	studentUser, err := store.GetUserByEmail("alex.vance@seriestkd.com")
	if err != nil || studentUser == nil {
		t.Fatalf("seeded student user missing: %v", err)
	}
	studentToken := uuid.New().String()
	_ = store.CreateSessionToken(studentToken, studentUser.ID, time.Now().Add(time.Hour))

	locs, _ := store.GetAllLocations()
	sess := &models.TrainingSession{
		ID:           uuid.New(),
		Title:        "Poomsae Excellence",
		TrainingType: models.TrainingPoomsae,
		LocationID:   &locs[0].ID,
		LocationName: locs[0].Name,
		SessionDate:  time.Now().AddDate(0, 0, 2),
		StartTime:    "10:00",
		EndTime:      "11:30",
	}
	_ = store.CreateSession(sess)

	req := httptest.NewRequest("GET", "/reserve/"+sess.ID.String(), nil)
	req.SetPathValue("id", sess.ID.String())
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: studentToken})
	rec := httptest.NewRecorder()

	app.AuthMiddleware(http.HandlerFunc(app.HandleSessionReservePage)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for student, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Alex Vance") {
		t.Errorf("expected practitioner name to be displayed")
	}
	if !strings.Contains(body, "Confirm Slot Reservation") {
		t.Errorf("expected 1-click confirmation button for student")
	}
	if strings.Contains(body, "reserve_guest_name") {
		t.Errorf("guest name input should NOT be shown for logged-in student")
	}
}

func TestSessionReservePage_AlreadyReserved(t *testing.T) {
	app, store := setupTestApp(t)

	studentUser, _ := store.GetUserByEmail("alex.vance@seriestkd.com")
	studentToken := uuid.New().String()
	_ = store.CreateSessionToken(studentToken, studentUser.ID, time.Now().Add(time.Hour))

	locs, _ := store.GetAllLocations()
	sess := &models.TrainingSession{
		ID:           uuid.New(),
		Title:        "Saturday Mat Sparring",
		TrainingType: models.TrainingSparring,
		LocationID:   &locs[0].ID,
		SessionDate:  time.Now().AddDate(0, 0, 3),
		StartTime:    "14:00",
		EndTime:      "15:30",
	}
	_ = store.CreateSession(sess)

	// Pre-reserve student
	_, _ = store.CheckInStudent(sess.ID, *studentUser.StudentID, nil, nil)

	req := httptest.NewRequest("GET", "/reserve/"+sess.ID.String(), nil)
	req.SetPathValue("id", sess.ID.String())
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: studentToken})
	rec := httptest.NewRecorder()

	app.AuthMiddleware(http.HandlerFunc(app.HandleSessionReservePage)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "already reserved a slot") {
		t.Errorf("expected 'already reserved' banner for student who already reserved")
	}
}

func TestSessionReservePage_CancelledAndConcluded(t *testing.T) {
	app, store := setupTestApp(t)

	locs, _ := store.GetAllLocations()
	cancelledSess := &models.TrainingSession{
		ID:           uuid.New(),
		Title:        "Cancelled Drill",
		TrainingType: models.TrainingSparring,
		LocationID:   &locs[0].ID,
		SessionDate:  time.Now().AddDate(0, 0, 2),
		StartTime:    "10:00",
		EndTime:      "11:00",
		IsCancelled:  true,
	}
	_ = store.CreateSession(cancelledSess)

	req := httptest.NewRequest("GET", "/reserve/"+cancelledSess.ID.String(), nil)
	req.SetPathValue("id", cancelledSess.ID.String())
	rec := httptest.NewRecorder()

	app.AuthMiddleware(http.HandlerFunc(app.HandleSessionReservePage)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "This session has been cancelled") {
		t.Errorf("expected cancelled session alert banner")
	}

	// Concluded session
	pastSess := &models.TrainingSession{
		ID:           uuid.New(),
		Title:        "Yesterday Class",
		TrainingType: models.TrainingSparring,
		LocationID:   &locs[0].ID,
		SessionDate:  time.Now().AddDate(0, 0, -3),
		StartTime:    "10:00",
		EndTime:      "11:00",
	}
	_ = store.CreateSession(pastSess)

	reqPast := httptest.NewRequest("GET", "/reserve/"+pastSess.ID.String(), nil)
	reqPast.SetPathValue("id", pastSess.ID.String())
	recPast := httptest.NewRecorder()

	app.AuthMiddleware(http.HandlerFunc(app.HandleSessionReservePage)).ServeHTTP(recPast, reqPast)

	if recPast.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", recPast.Code)
	}
	if !strings.Contains(recPast.Body.String(), "already concluded") {
		t.Errorf("expected concluded session alert banner")
	}
}

func TestSessionReservePage_NotFoundAndInvalid(t *testing.T) {
	app, _ := setupTestApp(t)

	// Invalid UUID
	reqInv := httptest.NewRequest("GET", "/reserve/not-a-uuid", nil)
	reqInv.SetPathValue("id", "not-a-uuid")
	recInv := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleSessionReservePage)).ServeHTTP(recInv, reqInv)
	if recInv.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for invalid UUID, got %d", recInv.Code)
	}

	// Non-existent UUID
	randomID := uuid.New().String()
	reqNF := httptest.NewRequest("GET", "/reserve/"+randomID, nil)
	reqNF.SetPathValue("id", randomID)
	recNF := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleSessionReservePage)).ServeHTTP(recNF, reqNF)
	if recNF.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for non-existent session, got %d", recNF.Code)
	}
}

func TestSessionReserveSubmit_GuestSuccess(t *testing.T) {
	app, store := setupTestApp(t)

	locs, _ := store.GetAllLocations()
	sess := &models.TrainingSession{
		ID:           uuid.New(),
		Title:        "Introductory Taekwondo",
		TrainingType: models.TrainingConditioning,
		LocationID:   &locs[0].ID,
		LocationName: locs[0].Name,
		SessionDate:  time.Now().AddDate(0, 0, 3),
		StartTime:    "17:00",
		EndTime:      "18:00",
	}
	_ = store.CreateSession(sess)

	formData := url.Values{
		"session_id":     {sess.ID.String()},
		"guest_name":     {"Maria Santos"},
		"contact_number": {"09181234567"},
	}
	req := httptest.NewRequest("POST", "/reserve/"+sess.ID.String(), strings.NewReader(formData.Encode()))
	req.SetPathValue("id", sess.ID.String())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.AuthMiddleware(http.HandlerFunc(app.HandleSessionReserveSubmit)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d (body: %s)", rec.Code, rec.Body.String())
	}

	if !strings.Contains(rec.Body.String(), "Slot Reserved Successfully!") {
		t.Errorf("expected success banner in response")
	}

	// Verify attendance was created in store
	attendances, err := store.GetSessionAttendances(sess.ID)
	if err != nil || len(attendances) != 1 {
		t.Fatalf("expected 1 attendance record in store, got %d (err: %v)", len(attendances), err)
	}
	if attendances[0].AttendeeName != "Maria Santos" {
		t.Errorf("expected attendee name 'Maria Santos', got %q", attendances[0].AttendeeName)
	}
	if attendances[0].ContactNumber != "09181234567" {
		t.Errorf("expected contact number '09181234567', got %q", attendances[0].ContactNumber)
	}
}

func TestSessionReserveSubmit_GuestValidation(t *testing.T) {
	app, store := setupTestApp(t)

	locs, _ := store.GetAllLocations()
	sess := &models.TrainingSession{
		ID:           uuid.New(),
		Title:        "Introductory Taekwondo",
		TrainingType: models.TrainingConditioning,
		LocationID:   &locs[0].ID,
		SessionDate:  time.Now().AddDate(0, 0, 3),
		StartTime:    "17:00",
		EndTime:      "18:00",
	}
	_ = store.CreateSession(sess)

	// Missing Name
	formDataNoName := url.Values{
		"session_id":     {sess.ID.String()},
		"contact_number": {"09181234567"},
	}
	req := httptest.NewRequest("POST", "/reserve/"+sess.ID.String(), strings.NewReader(formDataNoName.Encode()))
	req.SetPathValue("id", sess.ID.String())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.AuthMiddleware(http.HandlerFunc(app.HandleSessionReserveSubmit)).ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "Please enter your full name") {
		t.Errorf("expected error message for missing guest name")
	}

	// Missing Phone
	formDataNoPhone := url.Values{
		"session_id": {sess.ID.String()},
		"guest_name": {"Maria Santos"},
	}
	req2 := httptest.NewRequest("POST", "/reserve/"+sess.ID.String(), strings.NewReader(formDataNoPhone.Encode()))
	req2.SetPathValue("id", sess.ID.String())
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec2 := httptest.NewRecorder()

	app.AuthMiddleware(http.HandlerFunc(app.HandleSessionReserveSubmit)).ServeHTTP(rec2, req2)
	if !strings.Contains(rec2.Body.String(), "Please enter your contact number") {
		t.Errorf("expected error message for missing contact number")
	}
}

func TestSessionReserveSubmit_StudentSuccessAndDuplicate(t *testing.T) {
	app, store := setupTestApp(t)

	studentUser, _ := store.GetUserByEmail("alex.vance@seriestkd.com")
	studentToken := uuid.New().String()
	_ = store.CreateSessionToken(studentToken, studentUser.ID, time.Now().Add(time.Hour))

	locs, _ := store.GetAllLocations()
	sess := &models.TrainingSession{
		ID:           uuid.New(),
		Title:        "Advanced Sparring Drill",
		TrainingType: models.TrainingSparring,
		LocationID:   &locs[0].ID,
		SessionDate:  time.Now().AddDate(0, 0, 4),
		StartTime:    "18:00",
		EndTime:      "19:30",
	}
	_ = store.CreateSession(sess)

	formData := url.Values{
		"session_id": {sess.ID.String()},
	}

	// First submission -> Success
	req := httptest.NewRequest("POST", "/reserve/"+sess.ID.String(), strings.NewReader(formData.Encode()))
	req.SetPathValue("id", sess.ID.String())
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: studentToken})
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.AuthMiddleware(http.HandlerFunc(app.HandleSessionReserveSubmit)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for student reservation, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Slot Reserved Successfully!") {
		t.Errorf("expected success banner in response for student")
	}

	// Second submission -> Already reserved error
	reqDup := httptest.NewRequest("POST", "/reserve/"+sess.ID.String(), strings.NewReader(formData.Encode()))
	reqDup.SetPathValue("id", sess.ID.String())
	reqDup.AddCookie(&http.Cookie{Name: "stms_session", Value: studentToken})
	reqDup.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recDup := httptest.NewRecorder()

	app.AuthMiddleware(http.HandlerFunc(app.HandleSessionReserveSubmit)).ServeHTTP(recDup, reqDup)
	if !strings.Contains(recDup.Body.String(), "already reserved a slot") {
		t.Errorf("expected already reserved error message on duplicate submission")
	}
}

func TestSessionReserveSubmit_CapacityLimit(t *testing.T) {
	app, store := setupTestApp(t)

	// Create location with 1 max trainee
	maxOne := 1
	loc := &models.Location{
		ID:          uuid.New(),
		Name:        "Micro Studio",
		MaxTrainees: &maxOne,
	}
	_ = store.CreateLocation(loc)

	sess := &models.TrainingSession{
		ID:           uuid.New(),
		Title:        "Private Sparring Session",
		TrainingType: models.TrainingSparring,
		LocationID:   &loc.ID,
		SessionDate:  time.Now().AddDate(0, 0, 5),
		StartTime:    "11:00",
		EndTime:      "12:00",
	}
	_ = store.CreateSession(sess)

	// Reserve slot 1 (reaches capacity)
	_, _ = store.ReserveSessionSlot(sess.ID, "First Guest", "09111111111")

	// Attempt to reserve slot 2
	formData := url.Values{
		"session_id":     {sess.ID.String()},
		"guest_name":     {"Second Guest"},
		"contact_number": {"09222222222"},
	}
	req := httptest.NewRequest("POST", "/reserve/"+sess.ID.String(), strings.NewReader(formData.Encode()))
	req.SetPathValue("id", sess.ID.String())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.AuthMiddleware(http.HandlerFunc(app.HandleSessionReserveSubmit)).ServeHTTP(rec, req)

	if !strings.Contains(rec.Body.String(), "maximum capacity") {
		t.Errorf("expected maximum capacity error banner, got %s", rec.Body.String())
	}
}
