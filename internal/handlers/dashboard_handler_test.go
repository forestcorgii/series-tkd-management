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
	if !strings.Contains(body, "Instructor Quick Access Hub") {
		t.Errorf("expected Coach Dashboard to contain 'Instructor Quick Access Hub'")
	}
	if !strings.Contains(body, "Mat Check-In") {
		t.Errorf("expected Coach Dashboard to contain 'Mat Check-In' launcher")
	}
	if !strings.Contains(body, "Evaluate Athlete") {
		t.Errorf("expected Coach Dashboard to contain 'Evaluate Athlete' launcher")
	}
	if !strings.Contains(body, "Floor Safety Holds") {
		t.Errorf("expected Coach Dashboard to contain 'Floor Safety Holds'")
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
	if !strings.Contains(body, "Front-Desk Quick Access Hub") {
		t.Errorf("expected Admin Dashboard to contain 'Front-Desk Quick Access Hub'")
	}
	if !strings.Contains(body, "Register Student") {
		t.Errorf("expected Admin Dashboard to contain 'Register Student'")
	}
	if !strings.Contains(body, "Assign Package") {
		t.Errorf("expected Admin Dashboard to contain 'Assign Package'")
	}
	if !strings.Contains(body, "Schedule Class") {
		t.Errorf("expected Admin Dashboard to contain 'Schedule Class'")
	}
	if !strings.Contains(body, "Floor Schedule Today") {
		t.Errorf("expected Admin Dashboard to contain 'Floor Schedule Today'")
	}
	if !strings.Contains(body, "Pass Renewal & Low-Credit Alerts") && !strings.Contains(body, "Pass Renewal") {
		t.Errorf("expected Admin Dashboard to contain pass renewal alerts")
	}
}

