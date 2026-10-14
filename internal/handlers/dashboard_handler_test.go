package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/handlers"
	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

func TestDashboard_ActiveClassesOnlyShowsOpenClasses(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	coaches, err := store.GetAllCoaches()
	if err != nil || len(coaches) == 0 {
		t.Fatalf("expected seeded coaches, got %v", err)
	}
	coachID := coaches[0].ID

	now := time.Now()
	// Clean existing sessions to control the test environment
	allSessions, _ := store.GetAllSessions()
	for _, s := range allSessions {
		// Cancel or adjust
		_ = store.CancelSession(s.ID, "clean test state", false)
	}

	// 1. Add a future open session (e.g. tomorrow)
	openSess1 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  now.AddDate(0, 0, 1),
		StartTime:    "10:00",
		EndTime:      "11:30",
		CoachID:      &coachID,
		TrainingType: models.TrainingSparring,
		Notes:        "Future Open Sparring Session",
	}
	if err := store.CreateSession(openSess1); err != nil {
		t.Fatalf("failed to create open session 1: %v", err)
	}

	// 2. Add an already concluded session (yesterday)
	closedSess := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  now.AddDate(0, 0, -1),
		StartTime:    "10:00",
		EndTime:      "11:30",
		CoachID:      &coachID,
		TrainingType: models.TrainingPoomsae,
		Notes:        "Yesterday Closed Poomsae Session",
	}
	if err := store.CreateSession(closedSess); err != nil {
		t.Fatalf("failed to create closed session: %v", err)
	}

	// 3. Add a cancelled session (tomorrow but cancelled)
	cancelledSess := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  now.AddDate(0, 0, 1),
		StartTime:    "14:00",
		EndTime:      "15:30",
		CoachID:      &coachID,
		TrainingType: models.TrainingConditioning,
		Notes:        "Cancelled Conditioning Session",
	}
	if err := store.CreateSession(cancelledSess); err != nil {
		t.Fatalf("failed to create cancelled session: %v", err)
	}
	if err := store.CancelSession(cancelledSess.ID, "Coach unavailable", false); err != nil {
		t.Fatalf("failed to cancel session: %v", err)
	}

	// Execute HandleDashboard as an Operation Manager
	req := httptest.NewRequest("GET", "/", nil)
	opManager := &models.User{
		ID:       uuid.New(),
		Email:    "opmanager@seriestkd.com",
		Role:     models.RoleOperationManager,
		IsActive: true,
	}
	ctx := context.WithValue(req.Context(), handlers.UserContextKey, opManager)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	app.HandleDashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()

	// Verify Active Classes header shows 1 Open
	if !strings.Contains(body, "1 Open") {
		t.Errorf("expected dashboard to show '1 Open' active classes, body:\n%s", body)
	}

	// Verify the open session is visible
	if !strings.Contains(body, "10:00 - 11:30") {
		t.Errorf("expected open session time '10:00 - 11:30' to be displayed in Active Classes")
	}

	// Verify the cancelled session is NOT displayed in Active Classes
	if strings.Contains(body, "14:00 - 15:30") {
		t.Errorf("cancelled session time '14:00 - 15:30' should NOT be displayed in Active Classes")
	}

	// Now cancel the open session as well to test empty state
	if err := store.CancelSession(openSess1.ID, "Flooded floor", false); err != nil {
		t.Fatalf("failed to cancel open session: %v", err)
	}

	rec2 := httptest.NewRecorder()
	app.HandleDashboard(rec2, req)

	if rec2.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec2.Code)
	}

	body2 := rec2.Body.String()
	if !strings.Contains(body2, "0 Open") {
		t.Errorf("expected dashboard to show '0 Open' when all sessions are closed or cancelled")
	}
	if !strings.Contains(body2, "No open classes right now") {
		t.Errorf("expected empty state 'No open classes right now' when no open classes exist")
	}
}

func TestCoachDashboard_RendersAssignedClassesAndQuickTools(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	coaches, _ := store.GetAllCoaches()
	if len(coaches) == 0 {
		t.Fatalf("expected seeded coaches")
	}
	coach := coaches[0]

	coachUser := &models.User{
		ID:          uuid.New(),
		Email:       coach.Email,
		Role:        models.RoleCoach,
		CoachID:     &coach.ID,
		IsActive:    true,
		DisplayName: coach.FullName,
	}

	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), handlers.UserContextKey, coachUser)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	app.HandleDashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for Coach Dashboard, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Dan Coach") && !strings.Contains(body, "Instructor Console") {
		t.Errorf("expected Coach Dashboard to contain 'Dan Coach' or 'Instructor Console'")
	}
	if !strings.Contains(body, "Admit to Mat") && !strings.Contains(body, "Mat Roster") {
		t.Errorf("expected Coach Dashboard to contain mat admission or roster tool")
	}
	if !strings.Contains(body, "Evaluate") {
		t.Errorf("expected Coach Dashboard to contain 'Evaluate' launcher")
	}
	if !strings.Contains(body, "Safety Hold") {
		t.Errorf("expected Coach Dashboard to contain 'Safety Hold'")
	}
}

func TestAdminDashboard_RendersFloorScheduleAndRenewalAlerts(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	adminUser := &models.User{
		ID:          uuid.New(),
		Email:       "admin@seriestkd.com",
		Role:        models.RoleAdmin,
		IsActive:    true,
		DisplayName: "Front-Desk Admin",
	}

	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), handlers.UserContextKey, adminUser)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	app.HandleDashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for Admin Dashboard, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Front-Desk Admin Console") {
		t.Errorf("expected Admin Dashboard to contain 'Front-Desk Admin Console'")
	}
	if !strings.Contains(body, "Student") && !strings.Contains(body, "openNewStudentModal") {
		t.Errorf("expected Admin Dashboard to contain Student intake action")
	}
	if !strings.Contains(body, "Pass") && !strings.Contains(body, "openAssignPassModal") {
		t.Errorf("expected Admin Dashboard to contain Pass assign action")
	}
	if !strings.Contains(body, "Class") && !strings.Contains(body, "openNewSessionModal") {
		t.Errorf("expected Admin Dashboard to contain Class schedule action")
	}
	if !strings.Contains(body, "Floor Schedule Today") {
		t.Errorf("expected Admin Dashboard to contain 'Floor Schedule Today'")
	}
	if !strings.Contains(body, "Pass Renewal") && !strings.Contains(body, "Due for Renewal") {
		t.Errorf("expected Admin Dashboard to contain pass renewal alerts")
	}
}

func TestCoachDashboard_FiltersScheduleTypesToClassesAndOpenSessions(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	coaches, _ := store.GetAllCoaches()
	if len(coaches) == 0 {
		t.Fatalf("expected seeded coaches")
	}
	coach := coaches[0]

	coachUser := &models.User{
		ID:          uuid.New(),
		Email:       coach.Email,
		Role:        models.RoleCoach,
		CoachID:     &coach.ID,
		IsActive:    true,
		DisplayName: coach.FullName,
	}

	now := time.Now()
	// Seed 4 types of sessions for today:
	// 1. Class (must show)
	classSess := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  now,
		StartTime:    "10:00",
		EndTime:      "11:00",
		TrainingType: models.TrainingPoomsae,
		Title:        "Morning Poomsae Class",
		CoachID:      &coach.ID,
		CoachName:    coach.FullName,
		EntryType:    models.EntryTypeClass,
	}
	_ = store.CreateSession(classSess)

	// 2. Open session (must show)
	openSess := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  now,
		StartTime:    "14:00",
		EndTime:      "15:30",
		TrainingType: models.TrainingSparring,
		Title:        "Open Floor Sparring Lab",
		CoachID:      &coach.ID,
		CoachName:    coach.FullName,
		EntryType:    models.EntryTypeOpenSession,
	}
	_ = store.CreateSession(openSess)

	// 3. Duty session (must NOT show on coach dashboard)
	dutySess := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  now,
		StartTime:    "08:00",
		EndTime:      "12:00",
		TrainingType: models.TrainingConditioning,
		Title:        "Front Desk Duty Shift",
		EntryType:    models.EntryTypeDuty,
	}
	_ = store.CreateSession(dutySess)

	// 4. Special Event (must NOT show on coach dashboard)
	eventSess := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  now,
		StartTime:    "16:00",
		EndTime:      "20:00",
		TrainingType: models.TrainingPromotionPrep,
		Title:        "Invitational Regional Championship",
		EntryType:    models.EntryTypeEvent,
	}
	_ = store.CreateSession(eventSess)

	// 1. Default GET /: should show class and open session, but hide duty and event
	req := httptest.NewRequest("GET", "/", nil)
	ctx := context.WithValue(req.Context(), handlers.UserContextKey, coachUser)
	req = req.WithContext(ctx)

	rec := httptest.NewRecorder()
	app.HandleDashboard(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Morning Poomsae Class") {
		t.Errorf("expected Coach Dashboard to show Class session 'Morning Poomsae Class'")
	}
	if !strings.Contains(body, "Open Floor Sparring Lab") {
		t.Errorf("expected Coach Dashboard to show Free/Open session 'Open Floor Sparring Lab'")
	}
	if strings.Contains(body, "Front Desk Duty Shift") {
		t.Errorf("expected Coach Dashboard NOT to show duty schedule 'Front Desk Duty Shift'")
	}
	if strings.Contains(body, "Invitational Regional Championship") {
		t.Errorf("expected Coach Dashboard NOT to show event schedule 'Invitational Regional Championship'")
	}

	// 2. Filter with ?entry_type=class: should only include classes in today's list
	reqClass := httptest.NewRequest("GET", "/?entry_type=class", nil)
	reqClass = reqClass.WithContext(ctx)
	recClass := httptest.NewRecorder()
	app.HandleDashboard(recClass, reqClass)

	bodyClass := recClass.Body.String()
	if !strings.Contains(bodyClass, "Morning Poomsae Class") {
		t.Errorf("expected ?entry_type=class to show class session")
	}
	if strings.Contains(bodyClass, "Open Floor Sparring Lab") {
		t.Errorf("expected ?entry_type=class to exclude open session from today's list")
	}

	// 3. Filter with ?entry_type=open_session: should only include open sessions
	reqOpen := httptest.NewRequest("GET", "/?entry_type=open_session", nil)
	reqOpen = reqOpen.WithContext(ctx)
	recOpen := httptest.NewRecorder()
	app.HandleDashboard(recOpen, reqOpen)

	bodyOpen := recOpen.Body.String()
	if !strings.Contains(bodyOpen, "Open Floor Sparring Lab") {
		t.Errorf("expected ?entry_type=open_session to show open session")
	}
	if strings.Contains(bodyOpen, "Morning Poomsae Class") {
		t.Errorf("expected ?entry_type=open_session to exclude class session")
	}
}


