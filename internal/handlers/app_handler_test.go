package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
			expectedContent: "Active Classes",
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
			expectedContent: "Memberships",
			handlerFunc:     app.HandlePackages,
		},
		{
			path:            "/sessions",
			expectedContent: "Training Classes &amp; Floor Attendance Log",
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

func TestFormatPHTime(t *testing.T) {
	// 2026-09-27 10:05:00 UTC is 18:05:00 (06:05 PM) in Philippine Time (UTC+8)
	utcTime := time.Date(2026, time.September, 27, 10, 5, 0, 0, time.UTC)
	expected := "September 27, 2026 06:05 PM"
	actual := handlers.FormatPHTime(utcTime)
	if actual != expected {
		t.Errorf("expected %q, got %q", expected, actual)
	}

	// Morning time test: 2026-01-05 01:30:00 UTC is 09:30:00 AM PHT
	utcMorning := time.Date(2026, time.January, 5, 1, 30, 0, 0, time.UTC)
	expectedMorning := "January 05, 2026 09:30 AM"
	actualMorning := handlers.FormatPHTime(utcMorning)
	if actualMorning != expectedMorning {
		t.Errorf("expected %q, got %q", expectedMorning, actualMorning)
	}

	// Zero time test
	if zeroStr := handlers.FormatPHTime(time.Time{}); zeroStr != "" {
		t.Errorf("expected empty string for zero time, got %q", zeroStr)
	}
}
