package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"series-tkd-management/internal/handlers"
	"series-tkd-management/internal/repository"
)

func TestAppHandler_ParseTemplates(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	// Verify all pages render with 200 OK
	routes := []string{
		"/",
		"/students",
		"/coaches",
		"/packages",
		"/sessions",
	}

	for _, route := range routes {
		req := httptest.NewRequest("GET", route, nil)
		rec := httptest.NewRecorder()

		switch route {
		case "/":
			app.HandleDashboard(rec, req)
		case "/students":
			app.HandleStudents(rec, req)
		case "/coaches":
			app.HandleCoaches(rec, req)
		case "/packages":
			app.HandlePackages(rec, req)
		case "/sessions":
			app.HandleSessions(rec, req)
		}

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for %s, got %d: %s", route, rec.Code, rec.Body.String())
		}
	}

	// Verify student detail render
	students, _ := store.GetAllStudents()
	if len(students) > 0 {
		req := httptest.NewRequest("GET", "/students/"+students[0].ID.String(), nil)
		rec := httptest.NewRecorder()
		app.HandleStudentDetail(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for student detail, got %d: %s", rec.Code, rec.Body.String())
		}
	}
}
