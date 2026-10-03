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
		CoachID:      &coach.ID,
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
			CoachID:      &coach.ID,
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
		CoachID:      &coach.ID,
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
		CoachID:      &coach.ID,
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
		CoachID:      &coach.ID,
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
		CoachID:      &coach.ID,
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
		CoachID:      &coach.ID,
		TrainingType: models.TrainingConditioning,
		Notes:        "Afternoon Conditioning Class",
		CreatedAt:    time.Now(),
	}
	_ = store.CreateSession(s3)

	// Attendances:
	// Alice in s1 and s3
	_, _ = store.CheckInStudent(s1.ID, studentA.ID, nil, nil)
	_, _ = store.CheckInStudent(s3.ID, studentA.ID, nil, nil)
	// Bob in s2
	_, _ = store.CheckInStudent(s2.ID, studentB.ID, nil, nil)

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
			CoachID:      &coach.ID,
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

func TestSessionHandler_AdminDropdownEditAndDelete(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	coach1 := &models.Coach{
		ID:       uuid.New(),
		FullName: "Coach Mario",
		BeltRank: "3rd Dan",
		IsActive: true,
	}
	coach2 := &models.Coach{
		ID:       uuid.New(),
		FullName: "Coach Luigi",
		BeltRank: "4th Dan",
		IsActive: true,
	}
	_ = store.CreateCoach(coach1)
	_ = store.CreateCoach(coach2)

	admin := &models.User{
		ID:          uuid.New(),
		Email:       "peach@seriestkd.com",
		Role:        models.RoleAdmin,
		DisplayName: "Admin Peach",
		IsActive:    true,
	}
	_ = store.CreateUser(admin)

	var createdSessionID string

	// 1. Create session with coach and admin
	t.Run("HandleCreateSession with Coach and Admin", func(t *testing.T) {
		form := url.Values{
			"session_date":  {"2026-10-20"},
			"start_time":    {"15:00"},
			"end_time":      {"17:00"},
			"coach_id":      {coach1.ID.String()},
			"admin_id":      {admin.ID.String()},
			"training_type": {"Sparring"},
			"notes":         {"Tactical sparring"},
		}
		req := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		app.HandleCreateSession(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Fatalf("expected 303 SeeOther, got %d: %s", rec.Code, rec.Body.String())
		}
		loc := rec.Header().Get("Location")
		if !strings.HasPrefix(loc, "/sessions/") || !strings.HasSuffix(loc, "/live") {
			t.Fatalf("unexpected redirect location: %s", loc)
		}
		createdSessionID = strings.TrimSuffix(strings.TrimPrefix(loc, "/sessions/"), "/live")

		sessUUID, _ := uuid.Parse(createdSessionID)
		sess, err := store.GetSessionByID(sessUUID)
		if err != nil {
			t.Fatalf("failed to retrieve created session: %v", err)
		}
		if sess.CoachID == nil || *sess.CoachID != coach1.ID {
			t.Errorf("expected coach %s, got %v", coach1.ID, sess.CoachID)
		}
		if sess.AdminID == nil || *sess.AdminID != admin.ID {
			t.Errorf("expected admin %s, got %v", admin.ID, sess.AdminID)
		}
	})

	// 2. Edit session: change coach to coach2, change time to 16:00 - 18:00, update notes
	t.Run("HandleUpdateSession edits coach, admin, and time", func(t *testing.T) {
		form := url.Values{
			"session_date":  {"2026-10-21"},
			"start_time":    {"16:00"},
			"end_time":      {"18:00"},
			"coach_id":      {coach2.ID.String()},
			"admin_id":      {admin.ID.String()},
			"training_type": {"Poomsae"},
			"notes":         {"Updated to Poomsae forms"},
			"redirect_url":  {"/sessions"},
		}
		req := httptest.NewRequest(http.MethodPost, "/sessions/"+createdSessionID+"/edit", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		app.HandleUpdateSession(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Fatalf("expected 303 SeeOther, got %d: %s", rec.Code, rec.Body.String())
		}

		sessUUID, _ := uuid.Parse(createdSessionID)
		sess, err := store.GetSessionByID(sessUUID)
		if err != nil {
			t.Fatalf("failed to retrieve updated session: %v", err)
		}
		if sess.CoachID == nil || *sess.CoachID != coach2.ID {
			t.Errorf("expected coach %s, got %v", coach2.ID, sess.CoachID)
		}
		if sess.CoachName != "Coach Luigi" {
			t.Errorf("expected Coach Luigi, got %s", sess.CoachName)
		}
		if sess.AdminName != "Admin Peach" {
			t.Errorf("expected Admin Peach, got %s", sess.AdminName)
		}
		if sess.StartTime != "16:00" || sess.EndTime != "18:00" {
			t.Errorf("expected time 16:00 - 18:00, got %s - %s", sess.StartTime, sess.EndTime)
		}
		if sess.TrainingType != models.TrainingPoomsae {
			t.Errorf("expected Poomsae, got %s", sess.TrainingType)
		}
	})

	// 2b. Create session without lead coach (optional lead coach)
	t.Run("HandleCreateSession without Coach succeeds", func(t *testing.T) {
		form := url.Values{
			"session_date":  {"2026-10-22"},
			"start_time":    {"17:00"},
			"end_time":      {"19:00"},
			"coach_id":      {""}, // Empty / unassigned
			"admin_id":      {admin.ID.String()},
			"training_type": {"Conditioning"},
			"notes":         {"Coach unassigned class"},
		}
		req := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()

		app.HandleCreateSession(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Fatalf("expected 303 SeeOther, got %d: %s", rec.Code, rec.Body.String())
		}
		loc := rec.Header().Get("Location")
		noCoachSessionID := strings.TrimSuffix(strings.TrimPrefix(loc, "/sessions/"), "/live")
		sessUUID, _ := uuid.Parse(noCoachSessionID)
		sess, err := store.GetSessionByID(sessUUID)
		if err != nil {
			t.Fatalf("failed to retrieve created session: %v", err)
		}
		if sess.CoachID != nil {
			t.Errorf("expected nil CoachID, got %v", sess.CoachID)
		}
		if sess.CoachName != "" {
			t.Errorf("expected empty CoachName, got %s", sess.CoachName)
		}

		// Edit session to remove coach
		editForm := url.Values{
			"session_date":  {"2026-10-22"},
			"start_time":    {"17:00"},
			"end_time":      {"19:00"},
			"coach_id":      {""}, // Keep/make unassigned
			"training_type": {"Conditioning"},
		}
		editReq := httptest.NewRequest(http.MethodPost, "/sessions/"+noCoachSessionID+"/edit", strings.NewReader(editForm.Encode()))
		editReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		editRec := httptest.NewRecorder()
		app.HandleUpdateSession(editRec, editReq)
		if editRec.Code != http.StatusSeeOther {
			t.Fatalf("expected 303 SeeOther on edit to no coach, got %d", editRec.Code)
		}
		updatedSess, _ := store.GetSessionByID(sessUUID)
		if updatedSess.CoachID != nil {
			t.Errorf("expected nil CoachID after edit, got %v", updatedSess.CoachID)
		}
	})

	// 3. Verify Sessions Page renders the Admin dropdown and option
	t.Run("HandleSessions renders Admin dropdown options", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
		rec := httptest.NewRecorder()
		app.HandleSessions(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Admin Peach") {
			t.Errorf("expected 'Admin Peach' in rendered session page / dropdown options")
		}
		if !strings.Contains(body, "Supervising Admin") {
			t.Errorf("expected 'Supervising Admin' label in modal")
		}
		if !strings.Contains(body, "Edit Training Class") {
			t.Errorf("expected 'Edit Training Class' modal")
		}
	})

	// 4. Delete Open Session
	t.Run("HandleDeleteSession deletes open session", func(t *testing.T) {
		// Ensure session is in future / open
		sessUUID, _ := uuid.Parse(createdSessionID)
		sess, _ := store.GetSessionByID(sessUUID)
		sess.SessionDate = time.Now().AddDate(0, 0, 2)
		_ = store.UpdateSession(sess)

		req := httptest.NewRequest(http.MethodPost, "/sessions/"+createdSessionID+"/delete", nil)
		rec := httptest.NewRecorder()

		app.HandleDeleteSession(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Fatalf("expected 303 SeeOther, got %d", rec.Code)
		}

		// Verify session is deleted
		_, err := store.GetSessionByID(sessUUID)
		if err != repository.ErrNotFound {
			t.Errorf("expected ErrNotFound for deleted session, got %v", err)
		}

		// Attempting to delete again returns 404
		rec2 := httptest.NewRecorder()
		app.HandleDeleteSession(rec2, req)
		if rec2.Code != http.StatusNotFound {
			t.Errorf("expected 404 NotFound on second delete, got %d", rec2.Code)
		}
	})

	// 5. Deleting closed/past session should be rejected
	t.Run("HandleDeleteSession rejects closed session", func(t *testing.T) {
		pastSess := &models.TrainingSession{
			ID:           uuid.New(),
			SessionDate:  time.Now().AddDate(0, 0, -2), // 2 days ago
			StartTime:    "09:00",
			EndTime:      "10:00",
			CoachID:      &coach1.ID,
			TrainingType: models.TrainingConditioning,
		}
		_ = store.CreateSession(pastSess)

		req := httptest.NewRequest(http.MethodPost, "/sessions/"+pastSess.ID.String()+"/delete", nil)
		rec := httptest.NewRecorder()

		app.HandleDeleteSession(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Errorf("expected 400 BadRequest when deleting closed session, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestHandleCheckIn_SessionRate_NoPackageCreditDeduction(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("NewAppHandler failed: %v", err)
	}

	coach := &models.Coach{
		ID:        uuid.New(),
		FullName:  "Master Kim",
		Email:     "kim@seriestkd.com",
		Phone:     "09171112222",
		BeltRank:  "5th Dan Black",
		IsActive:  true,
		CreatedAt: time.Now(),
	}
	_ = store.CreateCoach(coach)

	session := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  time.Now(),
		StartTime:    "17:00",
		EndTime:      "18:30",
		CoachID:      &coach.ID,
		TrainingType: models.TrainingSparring,
	}
	_ = store.CreateSession(session)

	// 1. Student with active package (5 remaining credits)
	studentWithPkg := &models.Student{
		ID:                uuid.New(),
		FullName:          "Gym Student With Package",
		DOB:               time.Now().AddDate(-10, 0, 0),
		CurrentBelt:       models.BeltWhite,
		LastPromotionDate: time.Now(),
		EmergencyName:     "Parent",
		EmergencyPhone:    "09170001111",
		EmergencyRelation: "Father",
		IsActive:          true,
		CreatedAt:         time.Now(),
	}
	_ = store.CreateStudent(studentWithPkg)

	remCredits := 5
	pkg := &models.StudentPackage{
		ID:                uuid.New(),
		StudentID:         studentWithPkg.ID,
		TemplateID:        uuid.New(),
		TotalSessions:     &remCredits,
		RemainingSessions: &remCredits,
		PurchaseDate:      time.Now(),
		ExpiryDate:        time.Now().AddDate(0, 1, 0),
		PaymentStatus:     "paid",
		CreatedAt:         time.Now(),
	}
	_ = store.AssignPackage(pkg)

	// POST /sessions/{sessionID}/checkin/{studentID} with session_rate=175.50
	form := url.Values{}
	form.Set("session_rate", "175.50")
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sessions/%s/checkin/%s", session.ID, studentWithPkg.ID), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.HandleCheckIn(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for check-in with session_rate, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Fix Rate: ₱175.50") {
		t.Errorf("expected response to contain Fix Rate badge 'Fix Rate: ₱175.50', got: %s", body)
	}

	// Verify package credit was NOT deducted/changed!
	pkgs, _ := store.GetStudentPackages(studentWithPkg.ID)
	if len(pkgs) == 0 || pkgs[0].RemainingSessions == nil || *pkgs[0].RemainingSessions != 5 {
		t.Fatalf("expected student's package credits to remain 5, got %v", pkgs[0].RemainingSessions)
	}

	// 2. Student with NO package checking in with session_rate
	studentNoPkg := &models.Student{
		ID:                uuid.New(),
		FullName:          "School Fix Rate Student",
		DOB:               time.Now().AddDate(-11, 0, 0),
		CurrentBelt:       models.BeltWhite,
		LastPromotionDate: time.Now(),
		EmergencyName:     "Parent B",
		EmergencyPhone:    "09170002222",
		EmergencyRelation: "Mother",
		IsActive:          true,
		CreatedAt:         time.Now(),
	}
	_ = store.CreateStudent(studentNoPkg)

	formNoPkg := url.Values{}
	formNoPkg.Set("session_rate", "200.00")
	reqNoPkg := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sessions/%s/checkin/%s", session.ID, studentNoPkg.ID), strings.NewReader(formNoPkg.Encode()))
	reqNoPkg.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recNoPkg := httptest.NewRecorder()

	app.HandleCheckIn(recNoPkg, reqNoPkg)

	if recNoPkg.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for student with no package when session_rate specified, got %d: %s", recNoPkg.Code, recNoPkg.Body.String())
	}
	if !strings.Contains(recNoPkg.Body.String(), "Fix Rate: ₱200.00") {
		t.Errorf("expected response to contain 'Fix Rate: ₱200.00', got: %s", recNoPkg.Body.String())
	}

	// 3. Normal check-in WITHOUT session_rate on another student: deducts credit
	studentNormal := &models.Student{
		ID:                uuid.New(),
		FullName:          "Gym Normal Member",
		DOB:               time.Now().AddDate(-12, 0, 0),
		CurrentBelt:       models.BeltWhite,
		LastPromotionDate: time.Now(),
		EmergencyName:     "Parent C",
		EmergencyPhone:    "09170003333",
		EmergencyRelation: "Guardian",
		IsActive:          true,
		CreatedAt:         time.Now(),
	}
	_ = store.CreateStudent(studentNormal)

	remNorm := 10
	pkgNorm := &models.StudentPackage{
		ID:                uuid.New(),
		StudentID:         studentNormal.ID,
		TemplateID:        uuid.New(),
		TotalSessions:     &remNorm,
		RemainingSessions: &remNorm,
		PurchaseDate:      time.Now(),
		ExpiryDate:        time.Now().AddDate(0, 1, 0),
		PaymentStatus:     "paid",
		CreatedAt:         time.Now(),
	}
	_ = store.AssignPackage(pkgNorm)

	reqNormal := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sessions/%s/checkin/%s", session.ID, studentNormal.ID), nil)
	recNormal := httptest.NewRecorder()
	app.HandleCheckIn(recNormal, reqNormal)

	if recNormal.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for normal package check-in, got %d: %s", recNormal.Code, recNormal.Body.String())
	}
	pkgsNorm, _ := store.GetStudentPackages(studentNormal.ID)
	if len(pkgsNorm) == 0 || pkgsNorm[0].RemainingSessions == nil || *pkgsNorm[0].RemainingSessions != 9 {
		t.Errorf("expected normal checkin to deduct 1 credit (remaining 9), got %v", pkgsNorm[0].RemainingSessions)
	}
}

func TestHandleCheckIn_ClassFixedRateAutoAppliesAndBypassesCreditDeduction(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("NewAppHandler failed: %v", err)
	}

	fixedClassRate := 250.0
	classSession := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  time.Now(),
		StartTime:    "17:00",
		EndTime:      "18:30",
		TrainingType: models.TrainingSparring,
		Notes:        "School Sparring Clinic",
		SessionRate:  &fixedClassRate,
	}
	_ = store.CreateSession(classSession)

	// Student with active 8-session membership
	student := &models.Student{
		ID:                uuid.New(),
		FullName:          "School Sparring Athlete",
		DOB:               time.Now().AddDate(-13, 0, 0),
		CurrentBelt:       models.BeltWhite,
		LastPromotionDate: time.Now(),
		EmergencyName:     "Parent S",
		EmergencyPhone:    "09171112222",
		EmergencyRelation: "Father",
		IsActive:          true,
		CreatedAt:         time.Now(),
	}
	_ = store.CreateStudent(student)

	remSessions := 8
	pkg := &models.StudentPackage{
		ID:                uuid.New(),
		StudentID:         student.ID,
		TemplateID:        uuid.New(),
		TotalSessions:     &remSessions,
		RemainingSessions: &remSessions,
		PurchaseDate:      time.Now(),
		ExpiryDate:        time.Now().AddDate(0, 1, 0),
		PaymentStatus:     "paid",
		CreatedAt:         time.Now(),
	}
	_ = store.AssignPackage(pkg)

	// Check-in without submitting any per-student rate:
	// The class fixed rate should be automatically applied!
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sessions/%s/checkin/%s", classSession.ID, student.ID), nil)
	rec := httptest.NewRecorder()
	app.HandleCheckIn(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for checkin on class with fixed rate, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Fix Rate: ₱250.00") {
		t.Errorf("expected response to show inherited class fixed rate 'Fix Rate: ₱250.00', got: %s", body)
	}

	// Verify student's package session credit was NOT deducted!
	pkgs, _ := store.GetStudentPackages(student.ID)
	if len(pkgs) == 0 || pkgs[0].RemainingSessions == nil || *pkgs[0].RemainingSessions != 8 {
		t.Fatalf("expected student's package credits to remain 8, got %v", pkgs[0].RemainingSessions)
	}

	// Verify attendance record in store has the inherited rate
	atts, _ := store.GetSessionAttendances(classSession.ID)
	if len(atts) != 1 {
		t.Fatalf("expected 1 attendance record, got %d", len(atts))
	}
	if atts[0].SessionRate == nil || *atts[0].SessionRate != 250.0 {
		t.Errorf("expected attendance session_rate to be 250.00, got %v", atts[0].SessionRate)
	}

	// Verify HandleSearchStudent exposes the class fixed rate
	searchReq := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/sessions/%s/search-student", classSession.ID), strings.NewReader("query=School"))
	searchReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	searchRec := httptest.NewRecorder()
	app.HandleSearchStudent(searchRec, searchReq)

	if searchRec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for search-student, got %d", searchRec.Code)
	}
	searchBody := searchRec.Body.String()
	if !strings.Contains(searchBody, "Class Fixed Rate: ₱250.00") {
		t.Errorf("expected search results to display 'Class Fixed Rate: ₱250.00', got: %s", searchBody)
	}
}

func TestHandleCreateSession_LocationFixedRatePreFill(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	locRate := 350.0
	loc := &models.Location{
		ID:        uuid.New(),
		Name:      "San Agustin Academy",
		Pin:       "https://maps.google.com/?q=San+Agustin",
		FixedRate: &locRate,
		CreatedAt: time.Now(),
	}
	_ = store.CreateLocation(loc)

	// 1. Create session via HandleCreateSession with location_id and NO session_rate:
	// Should auto-fill session_rate from location's FixedRate
	form1 := url.Values{
		"session_date":  {"2026-10-25"},
		"start_time":    {"16:00"},
		"end_time":      {"17:30"},
		"training_type": {"Sparring"},
		"location_id":   {loc.ID.String()},
	}
	req1 := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(form1.Encode()))
	req1.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec1 := httptest.NewRecorder()
	app.HandleCreateSession(rec1, req1)

	if rec1.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 SeeOther, got %d: %s", rec1.Code, rec1.Body.String())
	}
	locURL1 := rec1.Header().Get("Location")
	sessID1 := strings.TrimSuffix(strings.TrimPrefix(locURL1, "/sessions/"), "/live")
	sessUUID1, _ := uuid.Parse(sessID1)
	sess1, err := store.GetSessionByID(sessUUID1)
	if err != nil {
		t.Fatalf("failed to retrieve session 1: %v", err)
	}
	if sess1.SessionRate == nil || *sess1.SessionRate != 350.0 {
		t.Errorf("expected session rate 350.0 inherited from location, got %v", sess1.SessionRate)
	}

	// 2. Create session via HandleCreateSession with location_id AND explicit session_rate override
	form2 := url.Values{
		"session_date":  {"2026-10-26"},
		"start_time":    {"16:00"},
		"end_time":      {"17:30"},
		"training_type": {"Poomsae"},
		"location_id":   {loc.ID.String()},
		"session_rate":  {"450.00"},
	}
	req2 := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(form2.Encode()))
	req2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec2 := httptest.NewRecorder()
	app.HandleCreateSession(rec2, req2)

	if rec2.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 SeeOther, got %d: %s", rec2.Code, rec2.Body.String())
	}
	locURL2 := rec2.Header().Get("Location")
	sessID2 := strings.TrimSuffix(strings.TrimPrefix(locURL2, "/sessions/"), "/live")
	sessUUID2, _ := uuid.Parse(sessID2)
	sess2, err := store.GetSessionByID(sessUUID2)
	if err != nil {
		t.Fatalf("failed to retrieve session 2: %v", err)
	}
	if sess2.SessionRate == nil || *sess2.SessionRate != 450.0 {
		t.Errorf("expected overridden session rate 450.0, got %v", sess2.SessionRate)
	}

	// 3. Create session via HandleAPIAdminSchedule with location_id and NO session_rate
	form3 := url.Values{
		"session_date": {"2026-10-27"},
		"start_time":   {"17:00"},
		"end_time":     {"18:30"},
		"discipline":   {"Sparring"},
		"location_id":  {loc.ID.String()},
	}
	req3 := httptest.NewRequest(http.MethodPost, "/api/admin/schedule", strings.NewReader(form3.Encode()))
	req3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec3 := httptest.NewRecorder()
	app.HandleAPIAdminSchedule(rec3, req3)

	if rec3.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created from HandleAPIAdminSchedule, got %d: %s", rec3.Code, rec3.Body.String())
	}
	sessions, _ := store.GetAllSessions()
	var adminSess *models.TrainingSession
	for _, s := range sessions {
		if s.SessionDate.Format("2006-01-02") == "2026-10-27" {
			adminSess = s
			break
		}
	}
	if adminSess == nil {
		t.Fatalf("expected to find session generated by HandleAPIAdminSchedule")
	}
	if adminSess.SessionRate == nil || *adminSess.SessionRate != 350.0 {
		t.Errorf("expected admin schedule session rate 350.0 from location, got %v", adminSess.SessionRate)
	}
}

func TestSessionHandler_OverlappingSchedulesAndCardClickability(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	loc1 := &models.Location{
		ID:        uuid.New(),
		Name:      "Main Dojang",
		Pin:       "https://maps.google.com/?q=Main+Dojang",
		CreatedAt: time.Now(),
	}
	loc2 := &models.Location{
		ID:        uuid.New(),
		Name:      "West Branch",
		Pin:       "https://maps.google.com/?q=West+Branch",
		CreatedAt: time.Now(),
	}
	_ = store.CreateLocation(loc1)
	_ = store.CreateLocation(loc2)

	sessDate, _ := time.Parse("2006-01-02", "2026-10-14")
	sess1 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  sessDate,
		StartTime:    "16:00",
		EndTime:      "18:00",
		LocationID:   &loc1.ID,
		TrainingType: models.TrainingSparring,
		Notes:        "Sparring class at Main Dojang",
		CreatedAt:    time.Now(),
	}
	sess2 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  sessDate,
		StartTime:    "16:00",
		EndTime:      "18:00",
		LocationID:   &loc2.ID,
		TrainingType: models.TrainingPoomsae,
		Notes:        "Poomsae class at West Branch",
		CreatedAt:    time.Now(),
	}
	_ = store.CreateSession(sess1)
	_ = store.CreateSession(sess2)

	req := httptest.NewRequest(http.MethodGet, "/sessions?date=2026-10-14", nil)
	rec := httptest.NewRecorder()
	app.HandleSessions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}
	body := rec.Body.String()

	// 1. Verify overlapping cascading cards are rendered with generous width (65% instead of squeezed 50%)
	if !strings.Contains(body, "left: calc(0% + 2px); width: calc(65% - 4px);") {
		t.Errorf("expected card 1 to be placed at left 0%% and width 65%%, body snippet not found")
	}
	if !strings.Contains(body, "left: calc(35% + 2px); width: calc(65% - 4px);") {
		t.Errorf("expected card 2 to be placed at left 35%% and width 65%%, body snippet not found")
	}

	// 2. Verify whole card clickability handler is present
	if !strings.Contains(body, "onclick=\"openClassInfoFromCard(this)\"") {
		t.Errorf("expected card to have openClassInfoFromCard(this) click handler")
	}

	// 3. Verify Class Info modal exists in the page
	if !strings.Contains(body, "id=\"class-info-modal\"") {
		t.Errorf("expected class-info-modal to be present in page")
	}

	// 4. Verify location names are present in card headers
	if !strings.Contains(body, "Main Dojang") || !strings.Contains(body, "West Branch") {
		t.Errorf("expected location names in rendered calendar cards")
	}

	// 5. Verify Schedule Class action button is present in header
	if !strings.Contains(body, "Schedule Class") {
		t.Errorf("expected Schedule Class button in page")
	}
}



