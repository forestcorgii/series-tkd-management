package handlers_test

import (
	"fmt"
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

func TestSessionHandler_CalendarMultiHourAndHalfHour(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	coach := &models.Coach{
		ID:       uuid.New(),
		FullName: "Coach Dan",
		BeltRank: "5th Dan Black Belt",
	}
	_ = store.CreateCoach(coach)

	// Wednesday of test week: 2026-10-14
	sessDate, _ := time.Parse("2006-01-02", "2026-10-14")
	sess9to11 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  sessDate,
		StartTime:    "09:00",
		EndTime:      "11:00",
		CoachID:      coach.ID,
		TrainingType: models.TrainingSparring,
		Notes:        "Morning Sparring Camp",
	}
	_ = store.CreateSession(sess9to11)

	// Thursday: half-hour class 09:30 to 11:30
	thursDate, _ := time.Parse("2006-01-02", "2026-10-15")
	sess930to1130 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  thursDate,
		StartTime:    "09:30",
		EndTime:      "11:30",
		CoachID:      coach.ID,
		TrainingType: models.TrainingPoomsae,
		Notes:        "Half-hour Poomsae",
	}
	_ = store.CreateSession(sess930to1130)

	req := httptest.NewRequest(http.MethodGet, "/sessions?date=2026-10-14", nil)
	rec := httptest.NewRecorder()
	app.HandleSessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	body := rec.Body.String()

	// 1. Check 09:00 - 11:00 session on Wednesday
	if !strings.Contains(body, "Morning Sparring Camp") {
		t.Errorf("expected session notes for 9:00-11:00 session in body")
	}
	// Verify Google Calendar continuous 2-hour spanning card layout (top: 60px; height: 118px)
	if !strings.Contains(body, "top: 60px; height: 118px;") {
		t.Errorf("expected Google Calendar 2-hour card spanning at top: 60px; height: 118px;")
	}

	// 2. Check 09:30 - 11:30 session on Thursday (top: 90px; height: 118px)
	if !strings.Contains(body, "Half-hour Poomsae") {
		t.Errorf("expected 9:30 half-hour session in body")
	}
	if !strings.Contains(body, "top: 90px; height: 118px;") {
		t.Errorf("expected Google Calendar half-hour card at top: 90px; height: 118px;")
	}

	// 3. Check half-hour scheduling buttons are present
	if !strings.Contains(body, "openNewSessionModal('2026-10-14', '09:30')") && !strings.Contains(body, "openNewSessionModal('2026-10-12', '09:30')") {
		t.Errorf("expected half-hour booking slot modal calls in calendar grid")
	}
}

func TestSessionHandler_FilterByStudentAndRecentSorting(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	coach := &models.Coach{
		ID:       uuid.New(),
		FullName: "Coach Test",
		BeltRank: "3rd Dan",
	}
	_ = store.CreateCoach(coach)

	studentA := &models.Student{
		ID:          uuid.New(),
		FullName:    "Alice Walker",
		CurrentBelt: "Yellow",
	}
	studentB := &models.Student{
		ID:          uuid.New(),
		FullName:    "Bob Smith",
		CurrentBelt: "Green",
	}
	_ = store.CreateStudent(studentA)
	_ = store.CreateStudent(studentB)

	// Session 1: Older date
	d1, _ := time.Parse("2006-01-02", "2026-10-10")
	s1 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  d1,
		StartTime:    "10:00",
		EndTime:      "12:00",
		CoachID:      coach.ID,
		TrainingType: models.TrainingPoomsae,
		Notes:        "Old Poomsae Class",
		CreatedAt:    time.Now().Add(-48 * time.Hour),
	}
	_ = store.CreateSession(s1)

	// Session 2: Newer date, morning
	d2, _ := time.Parse("2006-01-02", "2026-10-12")
	s2 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  d2,
		StartTime:    "09:00",
		EndTime:      "11:00",
		CoachID:      coach.ID,
		TrainingType: models.TrainingSparring,
		Notes:        "Morning Sparring Class",
		CreatedAt:    time.Now().Add(-24 * time.Hour),
	}
	_ = store.CreateSession(s2)

	// Session 3: Newer date, afternoon (most recent)
	s3 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  d2,
		StartTime:    "16:00",
		EndTime:      "18:00",
		CoachID:      coach.ID,
		TrainingType: models.TrainingConditioning,
		Notes:        "Afternoon Conditioning Class",
		CreatedAt:    time.Now(),
	}
	_ = store.CreateSession(s3)

	// Attendances:
	// Alice in s1 and s3
	_, _ = store.CheckInStudent(s1.ID, studentA.ID, nil)
	_, _ = store.CheckInStudent(s3.ID, studentA.ID, nil)
	// Bob in s2
	_, _ = store.CheckInStudent(s2.ID, studentB.ID, nil)

	// 1. Verify sorting in roster (more recent first: s3 > s2 > s1)
	t.Run("Class Rosters are sorted by most recent first", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
		rec := httptest.NewRecorder()
		app.HandleSessions(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		body := rec.Body.String()

		pos3 := strings.Index(body, "Afternoon Conditioning Class")
		pos2 := strings.Index(body, "Morning Sparring Class")
		pos1 := strings.Index(body, "Old Poomsae Class")

		if pos3 == -1 || pos2 == -1 || pos1 == -1 {
			t.Fatalf("expected all 3 classes in roster, got pos3=%d, pos2=%d, pos1=%d", pos3, pos2, pos1)
		}
		if !(pos3 < pos2 && pos2 < pos1) {
			t.Errorf("expected order s3 (newest 16:00) < s2 (newest 09:00) < s1 (oldest), got pos3=%d, pos2=%d, pos1=%d", pos3, pos2, pos1)
		}
	})

	// 2. Filter by Student A
	t.Run("Filter by Student A returns only Alice's classes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sessions?student_id="+studentA.ID.String(), nil)
		rec := httptest.NewRecorder()
		app.HandleSessions(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		body := rec.Body.String()

		if !strings.Contains(body, "Afternoon Conditioning Class") {
			t.Errorf("expected Alice's s3 class in filtered view")
		}
		if !strings.Contains(body, "Old Poomsae Class") {
			t.Errorf("expected Alice's s1 class in filtered view")
		}
		if strings.Contains(body, "Morning Sparring Class") {
			t.Errorf("did not expect Bob's s2 class in Alice's filtered view")
		}
		if !strings.Contains(body, "Filter by Student") {
			t.Errorf("expected Filter by Student dropdown label")
		}
	})

	// 3. Filter by Student B
	t.Run("Filter by Student B returns only Bob's classes", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sessions?student_id="+studentB.ID.String(), nil)
		rec := httptest.NewRecorder()
		app.HandleSessions(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		body := rec.Body.String()

		if !strings.Contains(body, "Morning Sparring Class") {
			t.Errorf("expected Bob's s2 class in filtered view")
		}
		if strings.Contains(body, "Afternoon Conditioning Class") {
			t.Errorf("did not expect Alice's s3 class in Bob's filtered view")
		}
		if strings.Contains(body, "Old Poomsae Class") {
			t.Errorf("did not expect Alice's s1 class in Bob's filtered view")
		}
	})
}

func TestSessionHandler_RosterPagination(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	coach := &models.Coach{
		ID:       uuid.New(),
		FullName: "Coach Paginate",
		BeltRank: "2nd Dan",
	}
	_ = store.CreateCoach(coach)

	// Create 15 sessions on different days / hours
	baseDate, _ := time.Parse("2006-01-02", "2026-10-01")
	for i := 1; i <= 15; i++ {
		sDate := baseDate.AddDate(0, 0, i)
		sess := &models.TrainingSession{
			ID:           uuid.New(),
			SessionDate:  sDate,
			StartTime:    "10:00",
			EndTime:      "12:00",
			CoachID:      coach.ID,
			TrainingType: models.TrainingSparring,
			Notes:        fmt.Sprintf("Sess-Note-%02d", i),
		}
		_ = store.CreateSession(sess)
	}

	coachFilter := "&coach_id=" + coach.ID.String()

	// 1. Page 1 (default or ?page=1)
	t.Run("Page 1 renders 10 items and pagination info", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sessions?page=1"+coachFilter, nil)
		rec := httptest.NewRecorder()
		app.HandleSessions(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		body := rec.Body.String()

		// Counter check in pagination bar
		if !strings.Contains(body, "(Page 1 of 2)") {
			t.Errorf("expected pagination counter '(Page 1 of 2)', got body")
		}

		// Roster header counter
		if !strings.Contains(body, "Showing 1–10 of 15 matching") {
			t.Errorf("expected roster header 'Showing 1–10 of 15 matching'")
		}

		// The newest session is Sess-Note-15 (day 15)
		if !strings.Contains(body, "Sess-Note-15") {
			t.Errorf("expected newest session (Sess-Note-15) on Page 1")
		}
		// The 11th session (day 5, Sess-Note-05) should NOT be on Page 1
		if strings.Contains(body, "Sess-Note-05") {
			t.Errorf("did not expect session from page 2 (Sess-Note-05) on Page 1")
		}
	})

	// 2. Page 2
	t.Run("Page 2 renders remaining 5 items", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sessions?page=2"+coachFilter, nil)
		rec := httptest.NewRecorder()
		app.HandleSessions(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		body := rec.Body.String()

		// Counter check in pagination bar
		if !strings.Contains(body, "(Page 2 of 2)") {
			t.Errorf("expected pagination counter '(Page 2 of 2)'")
		}

		// Roster header counter
		if !strings.Contains(body, "Showing 11–15 of 15 matching") {
			t.Errorf("expected roster header 'Showing 11–15 of 15 matching'")
		}

		// Oldest session (Sess-Note-01) should be on Page 2
		if !strings.Contains(body, "Sess-Note-01") {
			t.Errorf("expected oldest session (Sess-Note-01) on Page 2")
		}
		// Newest session should NOT be on Page 2
		if strings.Contains(body, "Sess-Note-15") {
			t.Errorf("did not expect newest session (Sess-Note-15) on Page 2")
		}
	})

	// 3. Page out of bounds clamps to max page
	t.Run("Page 999 clamps to page 2", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sessions?page=999"+coachFilter, nil)
		rec := httptest.NewRecorder()
		app.HandleSessions(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		body := rec.Body.String()

		if !strings.Contains(body, "Page 2 of 2") {
			t.Errorf("expected page 999 to clamp to Page 2 of 2")
		}
	})
}



