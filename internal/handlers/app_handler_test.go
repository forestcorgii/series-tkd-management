package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
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

	// Verify all pages render with 200 OK and unique content without template errors
	routes := []struct {
		path            string
		expectedContent string
		handlerFunc     func(w http.ResponseWriter, r *http.Request)
	}{
		{
			path:            "/",
			expectedContent: "Dojang Floor &amp; Operations Dashboard",
			handlerFunc:     app.HandleDashboard,
		},
		{
			path:            "/students",
			expectedContent: "Student Directory &amp; Ability Radar",
			handlerFunc:     app.HandleStudents,
		},
		{
			path:            "/coaches",
			expectedContent: "Coach Directory, Safety &amp; Payroll",
			handlerFunc:     app.HandleCoaches,
		},
		{
			path:            "/packages",
			expectedContent: "Membership Packages &amp; Passes",
			handlerFunc:     app.HandlePackages,
		},
		{
			path:            "/sessions",
			expectedContent: "Training Sessions &amp; Floor Roster Log",
			handlerFunc:     app.HandleSessions,
		},
	}

	for _, tc := range routes {
		req := httptest.NewRequest("GET", tc.path, nil)
		rec := httptest.NewRecorder()

		tc.handlerFunc(rec, req)

		if rec.Code != http.StatusOK {
			t.Errorf("expected 200 for %s, got %d: %s", tc.path, rec.Code, rec.Body.String())
		}

		body := rec.Body.String()
		if strings.Contains(body, "Template error") {
			t.Errorf("found 'Template error' on route %s: %s", tc.path, body)
		}

		if !strings.Contains(body, tc.expectedContent) {
			t.Errorf("expected %s to contain %q, but got body length %d", tc.path, tc.expectedContent, len(body))
		}

		// "+ Register New Student" should only be present on the Students & Ability tab
		if tc.path != "/students" && strings.Contains(body, "Register New Student") {
			t.Errorf("unexpected 'Register New Student' found on tab %s", tc.path)
		}
		if tc.path == "/students" && !strings.Contains(body, "Register New Student") {
			t.Errorf("expected 'Register New Student' on /students tab, but was not found")
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
		if strings.Contains(rec.Body.String(), "Template error") {
			t.Errorf("found 'Template error' in student detail: %s", rec.Body.String())
		}
	}
}
