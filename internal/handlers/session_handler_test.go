package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/handlers"
	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

func TestSessionHandler_FiltersAndCancellation(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	coach := &models.Coach{
		ID:       uuid.New(),
		FullName: "Coach Carlos",
		BeltRank: "4th Dan Black Belt",
	}
	_ = store.CreateCoach(coach)

	sessDate, _ := time.Parse("2006-01-02", "2026-10-15")
	sess := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  sessDate,
		StartTime:    "17:00",
		EndTime:      "18:30",
		CoachID:      coach.ID,
		TrainingType: models.TrainingSparring,
		Notes:        "Sparring strategy",
	}
	_ = store.CreateSession(sess)

	t.Run("HandleSessions renders full page for standard GET", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
		rec := httptest.NewRecorder()

		app.HandleSessions(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Training Classes &amp; Floor Attendance Log") {
			t.Errorf("expected page header in full page render")
		}
		if !strings.Contains(body, "Filter by Coach") {
			t.Errorf("expected filter panel in full page render")
		}
		if !strings.Contains(body, "Coach Carlos") {
			t.Errorf("expected coach name in rendered session card")
		}
	})

	t.Run("HandleSessions renders partial for HTMX filter request", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sessions?category=Sparring", nil)
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()

		app.HandleSessions(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		body := rec.Body.String()
		if strings.Contains(body, "<!DOCTYPE html>") || strings.Contains(body, "<html") {
			t.Errorf("expected partial response without layout shell")
		}
		if !strings.Contains(body, "Sparring strategy") {
			t.Errorf("expected filtered sparring session in partial")
		}
	})

	t.Run("HandleSessions filters out non-matching criteria", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sessions?category=NonExistent", nil)
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()

		app.HandleSessions(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "No Training Classes Found") {
			t.Errorf("expected empty state when category doesn't match, got: %s", body)
		}
	})

	t.Run("HandleCancelSession cancels the session", func(t *testing.T) {
		form := url.Values{}
		form.Set("reason", "Inclement weather alert")
		form.Set("refund_credits", "true")

		req := httptest.NewRequest(http.MethodPost, "/sessions/"+sess.ID.String()+"/cancel", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		app.HandleCancelSession(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Fatalf("expected 303 redirect, got %d", rec.Code)
		}

		updatedSess, err := store.GetSessionByID(sess.ID)
		if err != nil {
			t.Fatalf("GetSessionByID failed: %v", err)
		}
		if !updatedSess.IsCancelled {
			t.Errorf("expected session to be marked as cancelled")
		}
		if updatedSess.CancellationReason != "Inclement weather alert" {
			t.Errorf("expected reason 'Inclement weather alert', got '%s'", updatedSess.CancellationReason)
		}
	})

	t.Run("HandleCheckIn rejects check-in for cancelled session", func(t *testing.T) {
		student := &models.Student{
			ID:          uuid.New(),
			FullName:    "Marco Silva",
			CurrentBelt: models.BeltWhite,
		}
		_ = store.CreateStudent(student)

		req := httptest.NewRequest(http.MethodPost, "/sessions/"+sess.ID.String()+"/checkin/"+student.ID.String(), nil)
		rec := httptest.NewRecorder()

		app.HandleCheckIn(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400 Bad Request on cancelled session, got %d", rec.Code)
		}
		if !strings.Contains(rec.Body.String(), "This class has been cancelled") {
			t.Errorf("expected rejection notice in body, got: %s", rec.Body.String())
		}
	})

	t.Run("Done session still allows add and remove students", func(t *testing.T) {
		// Session from yesterday -> IsDone() is true
		yesterday := time.Now().AddDate(0, 0, -1)
		doneSess := &models.TrainingSession{
			ID:           uuid.New(),
			SessionDate:  yesterday,
			StartTime:    "10:00",
			EndTime:      "11:30",
			CoachID:      coach.ID,
			TrainingType: models.TrainingPoomsae,
		}
		_ = store.CreateSession(doneSess)

		if !doneSess.IsDone() {
			t.Fatalf("expected doneSess.IsDone() to be true")
		}

		// Verify live session page shows Attendance Closed
		reqLive := httptest.NewRequest(http.MethodGet, "/sessions/"+doneSess.ID.String()+"/live", nil)
		recLive := httptest.NewRecorder()
		app.HandleLiveSession(recLive, reqLive)
		if recLive.Code != http.StatusOK {
			t.Fatalf("expected 200 on live session page, got %d", recLive.Code)
		}
		liveBody := recLive.Body.String()
		if !strings.Contains(liveBody, "ATTENDANCE CLOSED") {
			t.Errorf("expected 'ATTENDANCE CLOSED' badge on done session live page")
		}
		if !strings.Contains(liveBody, "Action") {
			t.Errorf("expected 'Action' column in roster table header")
		}

		// Admit a student to this concluded session (override checkin)
		student := &models.Student{
			ID:          uuid.New(),
			FullName:    "Lucas Vance",
			CurrentBelt: models.BeltLowYellow,
		}
		_ = store.CreateStudent(student)

		// Create student package with 8 credits
		tpls, _ := store.GetPackageTemplates()
		rem := 8
		pkg := &models.StudentPackage{
			ID:                uuid.New(),
			StudentID:         student.ID,
			TemplateID:        tpls[0].ID,
			TotalSessions:     &rem,
			RemainingSessions: &rem,
			PurchaseDate:      time.Now(),
			ExpiryDate:        time.Now().AddDate(0, 1, 0),
			PaymentStatus:     "paid",
		}
		_ = store.AssignPackage(pkg)

		// Check-in student to done session
		reqCheckIn := httptest.NewRequest(http.MethodPost, "/sessions/"+doneSess.ID.String()+"/checkin/"+student.ID.String(), nil)
		recCheckIn := httptest.NewRecorder()
		app.HandleCheckIn(recCheckIn, reqCheckIn)

		if recCheckIn.Code != http.StatusOK {
			t.Fatalf("expected checkin to succeed for done session, got status %d, body: %s", recCheckIn.Code, recCheckIn.Body.String())
		}
		if !strings.Contains(recCheckIn.Body.String(), "Lucas Vance") {
			t.Errorf("expected student name in returned checkin row")
		}
		if !strings.Contains(recCheckIn.Body.String(), "Remove") {
			t.Errorf("expected Remove button in checkin row")
		}
		if recCheckIn.Header().Get("HX-Trigger") != "attendanceUpdated" {
			t.Errorf("expected HX-Trigger: attendanceUpdated, got %s", recCheckIn.Header().Get("HX-Trigger"))
		}

		// Verify package was decremented
		pkgs, _ := store.GetStudentPackages(student.ID)
		if *pkgs[0].RemainingSessions != 7 {
			t.Errorf("expected 7 sessions remaining after checkin, got %d", *pkgs[0].RemainingSessions)
		}

		// Now remove student from done session attendance
		reqRemove := httptest.NewRequest(http.MethodPost, "/sessions/"+doneSess.ID.String()+"/attendance/"+student.ID.String()+"/remove", nil)
		reqRemove.Header.Set("HX-Request", "true")
		recRemove := httptest.NewRecorder()
		app.HandleRemoveAttendance(recRemove, reqRemove)

		if recRemove.Code != http.StatusOK {
			t.Fatalf("expected 200 OK on remove attendance, got %d", recRemove.Code)
		}
		if recRemove.Header().Get("HX-Trigger") != "attendanceUpdated" {
			t.Errorf("expected HX-Trigger: attendanceUpdated on remove, got %s", recRemove.Header().Get("HX-Trigger"))
		}

		// Verify student is removed from attendance
		atts, err := store.GetSessionAttendances(doneSess.ID)
		if err != nil {
			t.Fatalf("GetSessionAttendances failed: %v", err)
		}
		for _, a := range atts {
			if a.StudentID == student.ID {
				t.Errorf("student still found in session attendance after removal")
			}
		}

		// Verify package was refunded back to 8 credits
		pkgsAfter, _ := store.GetStudentPackages(student.ID)
		if *pkgsAfter[0].RemainingSessions != 8 {
			t.Errorf("expected 8 sessions remaining after refund, got %d", *pkgsAfter[0].RemainingSessions)
		}

		// Removing again should return 404
		recRemoveAgain := httptest.NewRecorder()
		app.HandleRemoveAttendance(recRemoveAgain, reqRemove)
		if recRemoveAgain.Code != http.StatusNotFound {
			t.Errorf("expected 404 when removing non-existent attendance, got %d", recRemoveAgain.Code)
		}
	})
}

