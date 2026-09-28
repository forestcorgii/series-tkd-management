package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"series-tkd-management/internal/handlers"
	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"

	"github.com/google/uuid"
)

func TestStudentPortal_ClassCheckIn(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to create AppHandler: %v", err)
	}

	// Retrieve seeded student (Alex Vance)
	alexUser, err := store.GetUserByEmail("alex.vance@seriestkd.com")
	if err != nil || alexUser == nil || alexUser.StudentID == nil {
		t.Fatalf("failed to retrieve seeded student Alex Vance: %v", err)
	}
	studentID := *alexUser.StudentID

	coaches, _ := store.GetAllCoaches()
	if len(coaches) == 0 {
		t.Fatalf("expected seeded coaches")
	}

	// Create an open session for today
	todaySess := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  time.Now(),
		StartTime:    "17:00",
		EndTime:      "18:15",
		CoachID:      coaches[0].ID,
		CoachName:    coaches[0].FullName,
		TrainingType: models.TrainingSparring,
		CreatedAt:    time.Now(),
	}
	_ = store.CreateSession(todaySess)

	// 1. Student self check-in via HTMX POST /api/student/check-in
	form := url.Values{}
	form.Set("session_id", todaySess.ID.String())
	form.Set("student_id", studentID.String())

	req := httptest.NewRequest("POST", "/api/student/check-in", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	ctx := context.WithValue(req.Context(), handlers.UserContextKey, alexUser)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	app.HandleAPIStudentCheckIn(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/student/check-in, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Checked In") {
		t.Errorf("expected response to contain 'Checked In' badge, got: %s", body)
	}
	if !strings.Contains(body, "Checked in successfully") {
		t.Errorf("expected success feedback message, got: %s", body)
	}
	if rec.Header().Get("HX-Trigger") != "attendanceUpdated" {
		t.Errorf("expected HX-Trigger: attendanceUpdated, got %s", rec.Header().Get("HX-Trigger"))
	}

	// Verify attendance was recorded in database
	atts, _ := store.GetStudentAttendances(studentID)
	found := false
	for _, a := range atts {
		if a.SessionID == todaySess.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected attendance to be saved for student %s in session %s", studentID, todaySess.ID)
	}

	// 2. Second check-in to same session should indicate already checked in
	rec2 := httptest.NewRecorder()
	app.HandleAPIStudentCheckIn(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on duplicate check-in, got %d: %s", rec2.Code, rec2.Body.String())
	}
	if !strings.Contains(rec2.Body.String(), "already checked in") {
		t.Errorf("expected duplicate check-in to report 'already checked in', got: %s", rec2.Body.String())
	}

	// 3. Delegation via POST /api/coach/check-in with student user role
	sess2 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  time.Now(),
		StartTime:    "18:30",
		EndTime:      "19:45",
		CoachID:      coaches[0].ID,
		CoachName:    coaches[0].FullName,
		TrainingType: models.TrainingPoomsae,
		CreatedAt:    time.Now(),
	}
	_ = store.CreateSession(sess2)

	form2 := url.Values{}
	form2.Set("session_id", sess2.ID.String())
	form2.Set("student_id", studentID.String())

	reqCoach := httptest.NewRequest("POST", "/api/coach/check-in", strings.NewReader(form2.Encode()))
	reqCoach.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqCoach.Header.Set("HX-Request", "true")
	reqCoach = reqCoach.WithContext(ctx)

	recCoach := httptest.NewRecorder()
	app.HandleAPICoachCheckIn(recCoach, reqCoach)

	if recCoach.Code != http.StatusOK {
		t.Fatalf("expected 200 OK when student calls /api/coach/check-in, got %d: %s", recCoach.Code, recCoach.Body.String())
	}
	if !strings.Contains(recCoach.Body.String(), "Checked In") {
		t.Errorf("expected student_session_item rendering on student delegation to coach check-in, got: %s", recCoach.Body.String())
	}

	// 4. Safety hold blocking
	stRecord, _ := store.GetStudentByID(studentID)
	stRecord.HasSafetyFlag = true
	_ = store.UpdateStudent(stRecord)

	sess3 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  time.Now(),
		StartTime:    "20:00",
		EndTime:      "21:00",
		CoachID:      coaches[0].ID,
		CoachName:    coaches[0].FullName,
		TrainingType: models.TrainingConditioning,
		CreatedAt:    time.Now(),
	}
	_ = store.CreateSession(sess3)

	form3 := url.Values{}
	form3.Set("session_id", sess3.ID.String())
	form3.Set("student_id", studentID.String())

	reqHold := httptest.NewRequest("POST", "/api/student/check-in", strings.NewReader(form3.Encode()))
	reqHold.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqHold.Header.Set("HX-Request", "true")
	reqHold = reqHold.WithContext(ctx)

	recHold := httptest.NewRecorder()
	app.HandleAPIStudentCheckIn(recHold, reqHold)

	if !strings.Contains(recHold.Body.String(), "Safety Hold Active") {
		t.Errorf("expected safety hold rejection notice, got: %s", recHold.Body.String())
	}

	// 5. Verification on student portal view
	reqPortal := httptest.NewRequest("GET", "/portal/student", nil)
	reqPortal = reqPortal.WithContext(ctx)
	recPortal := httptest.NewRecorder()
	app.HandleStudentPortal(recPortal, reqPortal)

	if recPortal.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on GET /portal/student, got %d", recPortal.Code)
	}
	portalBody := recPortal.Body.String()
	if !strings.Contains(portalBody, "Checked In") {
		t.Errorf("expected student portal to render Checked In badge for checked-in classes")
	}
}
