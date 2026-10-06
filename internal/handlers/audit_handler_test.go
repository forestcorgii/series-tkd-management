package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"series-tkd-management/internal/handlers"
	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

func setupTestAppHandler(t *testing.T) (*handlers.AppHandler, repository.RepositoryStore) {
	t.Helper()
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to create AppHandler: %v", err)
	}
	return app, store
}

func withUserContext(r *http.Request, user *models.User) *http.Request {
	ctx := context.WithValue(r.Context(), handlers.UserContextKey, user)
	return r.WithContext(ctx)
}

func TestAuditHandler_RBAC(t *testing.T) {
	app, _ := setupTestAppHandler(t)

	manager := &models.User{
		ID:          uuid.New(),
		Email:       "manager@seriestkd.com",
		Role:        models.RoleOperationManager,
		DisplayName: "Operation Manager",
		IsActive:    true,
	}

	admin := &models.User{
		ID:          uuid.New(),
		Email:       "admin@seriestkd.com",
		Role:        models.RoleAdmin,
		DisplayName: "Front Desk Admin",
		IsActive:    true,
	}

	coach := &models.User{
		ID:          uuid.New(),
		Email:       "coach@seriestkd.com",
		Role:        models.RoleCoach,
		DisplayName: "Head Coach",
		IsActive:    true,
	}

	student := &models.User{
		ID:          uuid.New(),
		Email:       "student@seriestkd.com",
		Role:        models.RoleStudent,
		DisplayName: "Practitioner",
		IsActive:    true,
	}

	t.Run("unauthenticated request redirects to login", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs", nil)
		rec := httptest.NewRecorder()

		app.HandleAuditLogs(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("expected 303 See Other, got %d", rec.Code)
		}
		loc := rec.Header().Get("Location")
		if loc != "/login?redirect=/logs" {
			t.Errorf("expected redirect to login, got %s", loc)
		}
	})

	t.Run("manager access returns 200 OK HTML", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs", nil)
		req = withUserContext(req, manager)
		rec := httptest.NewRecorder()

		app.HandleAuditLogs(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 OK, got %d", rec.Code)
		}
	})

	t.Run("admin access redirected to role dashboard", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs", nil)
		req = withUserContext(req, admin)
		rec := httptest.NewRecorder()

		app.HandleAuditLogs(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("expected 303 See Other, got %d", rec.Code)
		}
		loc := rec.Header().Get("Location")
		if loc != "/sessions" {
			t.Errorf("expected redirect to /sessions, got %s", loc)
		}
	})

	t.Run("coach access redirected to role dashboard", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs", nil)
		req = withUserContext(req, coach)
		rec := httptest.NewRecorder()

		app.HandleAuditLogs(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("expected 303 See Other, got %d", rec.Code)
		}
	})

	t.Run("student access redirected to practitioner portal", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/logs", nil)
		req = withUserContext(req, student)
		rec := httptest.NewRecorder()

		app.HandleAuditLogs(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("expected 303 See Other, got %d", rec.Code)
		}
		loc := rec.Header().Get("Location")
		if loc != "/portal/student" {
			t.Errorf("expected redirect to /portal/student, got %s", loc)
		}
	})

	t.Run("non-manager API access returns 403 Forbidden", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
		req = withUserContext(req, admin)
		rec := httptest.NewRecorder()

		app.HandleAuditLogs(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Errorf("expected 403 Forbidden for API request by non-manager, got %d", rec.Code)
		}
	})
}

func TestAuditHandler_HTMXPartialSwap(t *testing.T) {
	app, _ := setupTestAppHandler(t)

	manager := &models.User{
		ID:          uuid.New(),
		Email:       "manager@seriestkd.com",
		Role:        models.RoleOperationManager,
		DisplayName: "Operation Manager",
		IsActive:    true,
	}

	// Seed some audit entries
	app.LogAction(nil, "STUDENT_CREATE", models.AuditCategoryStudents, "Student", uuid.New().String(), "Carlos Yulo", "Created student")

	req := httptest.NewRequest(http.MethodGet, "/logs?q=carlos", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Target", "audit-log-rows")
	req = withUserContext(req, manager)
	rec := httptest.NewRecorder()

	app.HandleAuditLogs(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !containsStr(body, "audit-log-rows") {
		t.Errorf("expected partial response containing audit-log-rows div, got %s", body)
	}
	if !containsStr(body, "Carlos Yulo") {
		t.Errorf("expected matching Carlos Yulo in rows, got %s", body)
	}
}

func TestAuditHandler_JSONAPIAndTelemetry(t *testing.T) {
	app, _ := setupTestAppHandler(t)

	manager := &models.User{
		ID:          uuid.New(),
		Email:       "manager@seriestkd.com",
		Role:        models.RoleOperationManager,
		DisplayName: "Operation Manager",
		IsActive:    true,
	}

	// Log some actions
	app.LogAction(nil, "AUTH_LOGIN", models.AuditCategoryAuth, "User", manager.ID.String(), manager.DisplayName, "Manager signed in")
	app.LogAction(nil, "SESSION_CANCEL", models.AuditCategorySessions, "Session", uuid.New().String(), "Sparring", "Class cancelled")

	t.Run("GET /api/logs", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs", nil)
		req = withUserContext(req, manager)
		rec := httptest.NewRecorder()

		app.HandleAuditLogs(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var resp struct {
			Logs      []*models.AuditLog     `json:"logs"`
			Total     int                    `json:"total"`
			Telemetry *models.AuditTelemetry `json:"telemetry"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("failed to decode JSON response: %v", err)
		}

		if resp.Total != 2 {
			t.Errorf("expected Total 2, got %d", resp.Total)
		}
		if len(resp.Logs) != 2 {
			t.Errorf("expected 2 logs returned, got %d", len(resp.Logs))
		}
		if resp.Telemetry == nil || resp.Telemetry.TotalLogs != 2 {
			t.Errorf("expected Telemetry TotalLogs 2, got %+v", resp.Telemetry)
		}
	})

	t.Run("GET /api/logs/telemetry", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/logs/telemetry", nil)
		req = withUserContext(req, manager)
		rec := httptest.NewRecorder()

		app.HandleAuditTelemetry(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 OK, got %d", rec.Code)
		}

		var telem models.AuditTelemetry
		if err := json.Unmarshal(rec.Body.Bytes(), &telem); err != nil {
			t.Fatalf("failed to decode JSON telemetry: %v", err)
		}

		if telem.TotalLogs != 2 {
			t.Errorf("expected 2 total logs, got %d", telem.TotalLogs)
		}
	})
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(substr) > 0 && indexOf(s, substr) >= 0))
}

func indexOf(s, substr string) int {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
