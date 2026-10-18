package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestLandingHandler_HandleLandingPage_Guest(t *testing.T) {
	app, _ := setupTestApp(t)

	req := httptest.NewRequest("GET", "/", nil)
	rec := httptest.NewRecorder()

	app.AuthMiddleware(http.HandlerFunc(app.HandleLandingPage)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for unauthenticated guest, got %d", rec.Code)
	}

	body := rec.Body.String()

	// 1. Verify Hero Banner & Brand Mission
	if !strings.Contains(body, "Championship Performance") && !strings.Contains(body, "CHAMPIONSHIP PERFORMANCE") {
		t.Errorf("expected landing page to contain hero headline")
	}
	if !strings.Contains(body, "Kukkiwon WT Affiliated") {
		t.Errorf("expected landing page to mention Kukkiwon affiliation")
	}

	// 2. Verify Training Programs Showcase
	expectedPrograms := []string{
		"Sparring (Kyorugi)",
		"Forms (Poomsae)",
		"Cadets",
		"Athletic Conditioning",
	}
	for _, prog := range expectedPrograms {
		if !strings.Contains(body, prog) {
			t.Errorf("expected landing page to showcase program %q", prog)
		}
	}

	// 3. Verify Schedule & Locations Section
	if !strings.Contains(body, "Schedule &amp; Locations") && !strings.Contains(body, "Schedule & Locations") {
		t.Errorf("expected landing page to have Schedule & Locations section")
	}
	if !strings.Contains(body, "landing-schedule-container") {
		t.Errorf("expected landing page to contain schedule container")
	}

	// 4. Verify Portals Hub
	if !strings.Contains(body, "Student &amp; Parent Portal") && !strings.Contains(body, "Student & Parent Portal") {
		t.Errorf("expected landing page to show Student Portal card")
	}
	if !strings.Contains(body, "Coach Floor Tablet") {
		t.Errorf("expected landing page to show Coach Floor Tablet card")
	}
	if !strings.Contains(body, "Dojang Administration") {
		t.Errorf("expected landing page to show Dojang Administration card")
	}
}

func TestLandingHandler_HandleLandingPage_AuthenticatedRedirects(t *testing.T) {
	app, store := setupTestApp(t)

	// 1. Student User -> Redirect to /portal/student
	studentUser, err := store.GetUserByEmail("alex.vance@seriestkd.com")
	if err != nil || studentUser == nil {
		t.Fatalf("expected seeded student user: %v", err)
	}
	studentToken := uuid.New().String()
	_ = store.CreateSessionToken(studentToken, studentUser.ID, time.Now().Add(time.Hour))

	reqStudent := httptest.NewRequest("GET", "/", nil)
	reqStudent.AddCookie(&http.Cookie{Name: "stms_session", Value: studentToken})
	recStudent := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleLandingPage)).ServeHTTP(recStudent, reqStudent)

	if recStudent.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for logged in student, got %d", recStudent.Code)
	}
	if recStudent.Header().Get("Location") != "/portal/student" {
		t.Errorf("expected redirect to /portal/student, got %s", recStudent.Header().Get("Location"))
	}

	// 2. Coach User -> Redirect to /sessions
	coachUser, err := store.GetUserByEmail("jiwoo.park@seriestkd.com")
	if err != nil || coachUser == nil {
		t.Fatalf("expected seeded coach user: %v", err)
	}
	coachToken := uuid.New().String()
	_ = store.CreateSessionToken(coachToken, coachUser.ID, time.Now().Add(time.Hour))

	reqCoach := httptest.NewRequest("GET", "/", nil)
	reqCoach.AddCookie(&http.Cookie{Name: "stms_session", Value: coachToken})
	recCoach := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleLandingPage)).ServeHTTP(recCoach, reqCoach)

	if recCoach.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for logged in coach, got %d", recCoach.Code)
	}
	if recCoach.Header().Get("Location") != "/sessions" {
		t.Errorf("expected redirect to /sessions, got %s", recCoach.Header().Get("Location"))
	}

	// 3. Admin User -> Redirect to /sessions
	adminUser, err := store.GetUserByEmail("admin@seriestkd.com")
	if err != nil || adminUser == nil {
		t.Fatalf("expected seeded admin user: %v", err)
	}
	adminToken := uuid.New().String()
	_ = store.CreateSessionToken(adminToken, adminUser.ID, time.Now().Add(time.Hour))

	reqAdmin := httptest.NewRequest("GET", "/", nil)
	reqAdmin.AddCookie(&http.Cookie{Name: "stms_session", Value: adminToken})
	recAdmin := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleLandingPage)).ServeHTTP(recAdmin, reqAdmin)

	if recAdmin.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for logged in admin, got %d", recAdmin.Code)
	}
	if recAdmin.Header().Get("Location") != "/sessions" {
		t.Errorf("expected redirect to /sessions, got %s", recAdmin.Header().Get("Location"))
	}

	// 4. Operation Manager -> Redirect to /dashboard
	managerUser, err := store.GetUserByEmail("manager@seriestkd.com")
	if err != nil || managerUser == nil {
		t.Fatalf("expected seeded manager user: %v", err)
	}
	managerToken := uuid.New().String()
	_ = store.CreateSessionToken(managerToken, managerUser.ID, time.Now().Add(time.Hour))

	reqMgr := httptest.NewRequest("GET", "/", nil)
	reqMgr.AddCookie(&http.Cookie{Name: "stms_session", Value: managerToken})
	recMgr := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleLandingPage)).ServeHTTP(recMgr, reqMgr)

	if recMgr.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for logged in manager, got %d", recMgr.Code)
	}
	if recMgr.Header().Get("Location") != "/dashboard" {
		t.Errorf("expected redirect to /dashboard, got %s", recMgr.Header().Get("Location"))
	}

	// 5. Preview override with ?preview=1 allows authenticated user to view landing page
	reqPreview := httptest.NewRequest("GET", "/?preview=1", nil)
	reqPreview.AddCookie(&http.Cookie{Name: "stms_session", Value: managerToken})
	recPreview := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleLandingPage)).ServeHTTP(recPreview, reqPreview)

	if recPreview.Code != http.StatusOK {
		t.Errorf("expected 200 OK for preview parameter, got %d", recPreview.Code)
	}
	if !strings.Contains(recPreview.Body.String(), "previewing the public landing page") {
		t.Errorf("expected preview banner to appear")
	}
}

func TestLandingHandler_HandleLandingSchedule_Partial(t *testing.T) {
	app, _ := setupTestApp(t)

	// 1. Initial Schedule partial load
	req := httptest.NewRequest("GET", "/landing/schedule", nil)
	rec := httptest.NewRecorder()
	app.HandleLandingSchedule(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HandleLandingSchedule, got %d", rec.Code)
	}

	// 2. Filter by non-existent location
	reqEmpty := httptest.NewRequest("GET", "/landing/schedule?location=non-existent-id", nil)
	recEmpty := httptest.NewRecorder()
	app.HandleLandingSchedule(recEmpty, reqEmpty)

	if recEmpty.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", recEmpty.Code)
	}
	if !strings.Contains(recEmpty.Body.String(), "No Scheduled Sessions Found") {
		t.Errorf("expected empty schedule message when no sessions match filter")
	}
}
