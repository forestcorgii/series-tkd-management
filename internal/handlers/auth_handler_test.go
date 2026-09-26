package handlers_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"series-tkd-management/internal/handlers"
	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"

	"github.com/google/uuid"
)

func TestAuthHandler_WebFlow(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	// 1. GET /login should render 200 OK with login form
	req := httptest.NewRequest("GET", "/login", nil)
	rec := httptest.NewRecorder()
	app.HandleLoginPage(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /login, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "SERIES") || !strings.Contains(body, "PORTAL ACCESS") {
		t.Errorf("login page missing portal access headers")
	}

	// 2. POST /login with invalid credentials
	form := url.Values{}
	form.Set("email", "admin@seriestkd.com")
	form.Set("password", "wrongpass")
	req = httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleLoginSubmit(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on failed login, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected error in redirect query, got %s", loc)
	}

	// 3. POST /login with valid admin credentials
	form.Set("password", "admin123")
	req = httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleLoginSubmit(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on successful login, got %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/portal/admin" {
		t.Errorf("expected redirect to /portal/admin, got %s", rec.Header().Get("Location"))
	}

	// Check cookie
	cookies := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "stms_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatalf("expected stms_session cookie to be set")
	}

	// 4. Access /portal/admin using session cookie wrapped in AuthMiddleware
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /portal/admin", app.RequireRole(models.RoleAdmin)(app.HandleAdminPortal))
	handler := app.AuthMiddleware(adminMux)

	req = httptest.NewRequest("GET", "/portal/admin", nil)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 accessing /portal/admin with session cookie, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Promotion Pipeline &amp; Dan Roster Control") {
		t.Errorf("expected admin portal content on /portal/admin")
	}

	// 5. Logout
	req = httptest.NewRequest("GET", "/logout", nil)
	req.AddCookie(sessionCookie)
	rec = httptest.NewRecorder()
	app.HandleLogout(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on logout, got %d", rec.Code)
	}
}

func TestAuthHandler_RoleGuards(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	studentUser, _ := store.GetUserByEmail("alex.vance@seriestkd.com")

	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /portal/admin", app.RequireRole(models.RoleAdmin)(app.HandleAdminPortal))
	handler := app.AuthMiddleware(adminMux)

	// Unauthenticated request should redirect to /login
	req := httptest.NewRequest("GET", "/portal/admin", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "/login") {
		t.Errorf("expected redirect to /login for unauthenticated request, got %d to %s", rec.Code, rec.Header().Get("Location"))
	}

	// Student accessing Admin portal should be rejected (redirected to their student portal)
	token := "student-test-token"
	_ = store.CreateSessionToken(token, studentUser.ID, time.Now().Add(time.Hour))

	req = httptest.NewRequest("GET", "/portal/admin", nil)
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	// 2. Unauthenticated requests to root or protected paths redirect to /login
	appMux := http.NewServeMux()
	appMux.HandleFunc("GET /", app.RequireAuth(app.HandleDashboard))
	appMux.HandleFunc("GET /students", app.RequireAuth(app.HandleStudents))
	appMux.HandleFunc("GET /sessions", app.RequireAuth(app.HandleSessions))
	appMux.HandleFunc("GET /register", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
	appMux.HandleFunc("POST /students", app.RequireRole(models.RoleAdmin)(app.HandleCreateStudent))
	appHandler := app.AuthMiddleware(appMux)

	protectedPaths := []string{"/", "/students", "/sessions"}
	for _, p := range protectedPaths {
		r := httptest.NewRequest("GET", p, nil)
		w := httptest.NewRecorder()
		appHandler.ServeHTTP(w, r)
		if w.Code != http.StatusSeeOther || !strings.Contains(w.Header().Get("Location"), "/login") {
			t.Errorf("expected redirect to /login for unauthenticated request to %s, got %d to %s", p, w.Code, w.Header().Get("Location"))
		}
	}

	// 3. GET /register redirects to /login (public registration disabled)
	rReg := httptest.NewRequest("GET", "/register", nil)
	wReg := httptest.NewRecorder()
	appHandler.ServeHTTP(wReg, rReg)
	if wReg.Code != http.StatusSeeOther || !strings.Contains(wReg.Header().Get("Location"), "/login") {
		t.Errorf("expected redirect to /login on /register, got %d to %s", wReg.Code, wReg.Header().Get("Location"))
	}

	// 4. Non-admin attempting to POST /students should be forbidden/redirected
	studForm := url.Values{}
	studForm.Set("full_name", "Daniel LaRusso")
	studForm.Set("phone", "+1 555-0199")
	studForm.Set("email", "daniel.larusso@miyagido.com")
	studForm.Set("password", "craneKick1984")

	rSubmit := httptest.NewRequest("POST", "/students", strings.NewReader(studForm.Encode()))
	rSubmit.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	wSubmit := httptest.NewRecorder()
	appHandler.ServeHTTP(wSubmit, rSubmit)
	if wSubmit.Code != http.StatusSeeOther || !strings.Contains(wSubmit.Header().Get("Location"), "/login") {
		t.Errorf("expected unauthenticated POST /students to redirect to /login, got %d to %s", wSubmit.Code, wSubmit.Header().Get("Location"))
	}
}

func TestAuthHandler_RESTAPI_Lifecycle(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	// 1. POST /api/auth/login
	loginPayload := map[string]string{
		"email":    "admin@seriestkd.com",
		"password": "admin123",
	}
	bodyBytes, _ := json.Marshal(loginPayload)
	req := httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(bodyBytes))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.HandleAPILogin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/auth/login, got %d: %s", rec.Code, rec.Body.String())
	}
	var loginResp handlers.LoginResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &loginResp); err != nil {
		t.Fatalf("failed to decode login response: %v", err)
	}
	if loginResp.Token == "" || loginResp.User == nil || loginResp.User.Role != models.RoleAdmin {
		t.Fatalf("invalid login response structure: %+v", loginResp)
	}

	// 2. GET /api/auth/me with session
	req = httptest.NewRequest("GET", "/api/auth/me", nil)
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: loginResp.Token})
	rec = httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleAPIMe)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 from /api/auth/me, got %d", rec.Code)
	}

	// 3. POST /api/auth/register (Admin provisioning a new account)
	regPayload := map[string]interface{}{
		"email":    "instructor.choi@seriestkd.com",
		"password": "danInstructor2026",
		"role":     "COACH",
	}
	regBytes, _ := json.Marshal(regPayload)
	req = httptest.NewRequest("POST", "/api/auth/register", bytes.NewReader(regBytes))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	app.HandleAPIRegister(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 from /api/auth/register, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. POST /api/coach/check-in
	alex, _ := store.GetUserByEmail("alex.vance@seriestkd.com")
	coaches, _ := store.GetAllCoaches()
	if len(coaches) > 0 && alex != nil && alex.StudentID != nil {
		freshSess := &models.TrainingSession{
			ID:           uuid.New(),
			SessionDate:  time.Now(),
			StartTime:    "18:00",
			EndTime:      "19:00",
			CoachID:      coaches[0].ID,
			TrainingType: "Poomsae",
			CreatedAt:    time.Now(),
		}
		_ = store.CreateSession(freshSess)

		checkinPayload := map[string]interface{}{
			"session_id": freshSess.ID.String(),
			"student_id": alex.StudentID.String(),
		}
		chBytes, _ := json.Marshal(checkinPayload)
		req = httptest.NewRequest("POST", "/api/coach/check-in", bytes.NewReader(chBytes))
		req.Header.Set("Content-Type", "application/json")
		rec = httptest.NewRecorder()
		app.HandleAPICoachCheckIn(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 from /api/coach/check-in, got %d: %s", rec.Code, rec.Body.String())
		}
	}

	// 5. POST /api/safety/flag and resolve
	if alex != nil && alex.StudentID != nil {
		flagForm := url.Values{}
		flagForm.Set("student_id", alex.StudentID.String())
		flagForm.Set("incident_type", "Wrist Hyperextension")
		flagForm.Set("notes", "Applied cold compress")

		req = httptest.NewRequest("POST", "/api/safety/flag", strings.NewReader(flagForm.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec = httptest.NewRecorder()
		app.HandleAPISafetyFlag(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 from /api/safety/flag, got %d: %s", rec.Code, rec.Body.String())
		}

		st, _ := store.GetStudentByID(*alex.StudentID)
		if !st.HasSafetyFlag {
			t.Errorf("expected student to have safety flag after incident logged")
		}

		// Resolve incident
		incidents, _ := store.GetSafetyIncidents(nil)
		var alexIncID string
		for _, inc := range incidents {
			if inc.StudentID == *alex.StudentID {
				alexIncID = inc.ID.String()
				break
			}
		}
		if alexIncID != "" {
			resolveForm := url.Values{}
			resolveForm.Set("incident_id", alexIncID)
			req = httptest.NewRequest("POST", "/api/safety/resolve", strings.NewReader(resolveForm.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec = httptest.NewRecorder()
			app.HandleAPISafetyResolve(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200 from /api/safety/resolve, got %d: %s", rec.Code, rec.Body.String())
			}

			stAfter, _ := store.GetStudentByID(*alex.StudentID)
			if stAfter.HasSafetyFlag {
				t.Errorf("expected student safety flag to be cleared after resolution")
			}
		}

		// 6. POST /api/admin/promote
		promoteForm := url.Values{}
		promoteForm.Set("student_id", alex.StudentID.String())
		req = httptest.NewRequest("POST", "/api/admin/promote", strings.NewReader(promoteForm.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec = httptest.NewRecorder()
		app.HandleAPIAdminPromote(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("expected 200 from /api/admin/promote, got %d: %s", rec.Code, rec.Body.String())
		}
		promotedStudent, _ := store.GetStudentByID(*alex.StudentID)
		if promotedStudent.CurrentBelt != models.BeltYellowTag {
			t.Errorf("expected Alex Vance to be promoted to Yellow Tag, got %s", promotedStudent.CurrentBelt)
		}
	}
}

func TestPortals_RenderPages(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	adminUser, _ := store.GetUserByEmail("admin@seriestkd.com")
	coachUser, _ := store.GetUserByEmail("jiwoo.park@seriestkd.com")
	studentUser, _ := store.GetUserByEmail("alex.vance@seriestkd.com")

	// 1. Render Student Portal
	req := httptest.NewRequest("GET", "/portal/student", nil)
	ctx := context.WithValue(req.Context(), handlers.GetUserFromContext(req.Context()), studentUser)
	_ = ctx
	// Attach via cookie session
	token := "student-token-test"
	_ = store.CreateSessionToken(token, studentUser.ID, time.Now().Add(time.Hour))
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	rec := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleStudentPortal)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 rendering /portal/student, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Promotion Readiness Engine") {
		t.Errorf("expected student portal content")
	}

	// 2. Render Coach Portal
	tokenCoach := "coach-token-test"
	_ = store.CreateSessionToken(tokenCoach, coachUser.ID, time.Now().Add(time.Hour))
	req = httptest.NewRequest("GET", "/portal/coach", nil)
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: tokenCoach})
	rec = httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleCoachPortal)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 rendering /portal/coach, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Promotion Review Priority Queue") {
		t.Errorf("expected coach portal content")
	}

	// 3. Render Admin Portal
	tokenAdmin := "admin-token-test"
	_ = store.CreateSessionToken(tokenAdmin, adminUser.ID, time.Now().Add(time.Hour))
	req = httptest.NewRequest("GET", "/portal/admin", nil)
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: tokenAdmin})
	rec = httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleAdminPortal)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 rendering /portal/admin, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Safety Incident &amp; First Aid Clearance") {
		t.Errorf("expected admin portal content")
	}
}
