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
}
