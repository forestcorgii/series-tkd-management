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
	if strings.Contains(body, `class="btn btn-primary btn-sm">`) && strings.Contains(body, "Sign In") {
		t.Errorf("login page should not have top right sign in button in header")
	}
	if strings.Contains(body, `class="btn btn-primary btn-xs">Sign In</a>`) {
		t.Errorf("login page should not have mobile top right sign in button in header")
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

	// 3. POST /login with valid admin credentials (redirects to /sessions)
	form.Set("password", "admin123")
	req = httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleLoginSubmit(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on successful login, got %d", rec.Code)
	}
	// 3. POST /login with valid admin credentials (redirects to customized Admin dashboard at /)
	if rec.Header().Get("Location") != "/" {
		t.Errorf("expected redirect to / for admin, got %s", rec.Header().Get("Location"))
	}

	// 3b. POST /login with valid coach credentials (redirects to customized Coach dashboard at /)
	formCoach := url.Values{}
	formCoach.Set("email", "jiwoo.park@seriestkd.com")
	formCoach.Set("password", "coach123")
	reqCoach := httptest.NewRequest("POST", "/login", strings.NewReader(formCoach.Encode()))
	reqCoach.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recCoach := httptest.NewRecorder()
	app.HandleLoginSubmit(recCoach, reqCoach)
	if recCoach.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on successful coach login, got %d", recCoach.Code)
	}
	if recCoach.Header().Get("Location") != "/" {
		t.Errorf("expected redirect to / for coach, got %s", recCoach.Header().Get("Location"))
	}

	// 4. POST /login with Operation Manager credentials (redirects to /)
	formMgr := url.Values{}
	formMgr.Set("email", "manager@seriestkd.com")
	formMgr.Set("password", "manager123")
	req = httptest.NewRequest("POST", "/login", strings.NewReader(formMgr.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleLoginSubmit(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on successful login, got %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/" {
		t.Errorf("expected redirect to / for manager, got %s", rec.Header().Get("Location"))
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

	// 5. Access /portal/admin using session cookie wrapped in AuthMiddleware
	adminMux := http.NewServeMux()
	adminMux.HandleFunc("GET /portal/admin", app.RequireRole(models.RoleOperationManager)(app.HandleAdminPortal))
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

	// 6. Logout
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
			CoachID:      &coaches[0].ID,
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
		if promotedStudent.CurrentBelt != models.BeltLowYellow {
			t.Errorf("expected Alex Vance to be promoted to Low Yellow, got %s", promotedStudent.CurrentBelt)
		}
	}
}

func TestPortals_RenderPages(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

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

	// 3. Render Admin Portal (Operation Manager)
	mgrUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	tokenMgr := "mgr-token-test"
	_ = store.CreateSessionToken(tokenMgr, mgrUser.ID, time.Now().Add(time.Hour))
	req = httptest.NewRequest("GET", "/portal/admin", nil)
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: tokenMgr})
	rec = httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleAdminPortal)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 rendering /portal/admin, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Safety Incident &amp; First Aid Clearance") {
		t.Errorf("expected admin portal content")
	}
}

func TestRolePermissions_ViewMatrix(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	// Build full routing mux matching main.go
	mux := http.NewServeMux()

	// Portals
	mux.HandleFunc("GET /portal/student", app.RequireRole(models.RoleStudent, models.RoleCoach, models.RoleOperationManager)(app.HandleStudentPortal))
	mux.HandleFunc("GET /portal/coach", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleCoachPortal))
	mux.HandleFunc("GET /portal/admin", app.RequireRole(models.RoleOperationManager)(app.HandleAdminPortal))

	// Core Views
	mux.HandleFunc("GET /", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleDashboard))
	mux.HandleFunc("GET /students", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleStudents))
	mux.HandleFunc("GET /students/{id}", app.RequireRole(models.RoleStudent, models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleStudentDetail))
	mux.HandleFunc("POST /students", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleCreateStudent))
	mux.HandleFunc("POST /students/{id}", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleUpdateStudent))
	mux.HandleFunc("POST /students/{id}/evaluations", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleCreateEvaluation))
	mux.HandleFunc("GET /sessions", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleSessions))
	mux.HandleFunc("GET /coaches", app.RequireRole(models.RoleOperationManager)(app.HandleCoaches))
	mux.HandleFunc("GET /packages", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandlePackages))

	handler := app.AuthMiddleware(mux)

	// Fetch users
	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	adminUser, _ := store.GetUserByEmail("admin@seriestkd.com")
	coachUser, _ := store.GetUserByEmail("jiwoo.park@seriestkd.com")
	studentUser, _ := store.GetUserByEmail("alex.vance@seriestkd.com")

	// Create tokens
	tokManager := "tok-mgr"
	_ = store.CreateSessionToken(tokManager, managerUser.ID, time.Now().Add(time.Hour))
	tokAdmin := "tok-adm"
	_ = store.CreateSessionToken(tokAdmin, adminUser.ID, time.Now().Add(time.Hour))
	tokCoach := "tok-coa"
	_ = store.CreateSessionToken(tokCoach, coachUser.ID, time.Now().Add(time.Hour))
	tokStudent := "tok-stu"
	_ = store.CreateSessionToken(tokStudent, studentUser.ID, time.Now().Add(time.Hour))

	testReq := func(token string, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest("GET", path, nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	// 1. OPERATION MANAGER (Full Control: can view everything)
	t.Run("OperationManager_FullControl", func(t *testing.T) {
		for _, path := range []string{"/", "/students", "/sessions", "/coaches", "/packages", "/portal/admin"} {
			rec := testReq(tokManager, path)
			if rec.Code != http.StatusOK {
				t.Errorf("Operation Manager expected 200 on %s, got %d", path, rec.Code)
			}
		}
	})

	// 2. COACH (Can view Dashboard, Students, Attendance, and Coach Portal)
	t.Run("Coach_AllowedViews", func(t *testing.T) {
		for _, path := range []string{"/", "/students", "/sessions", "/portal/coach"} {
			rec := testReq(tokCoach, path)
			if rec.Code != http.StatusOK {
				t.Errorf("Coach expected 200 on %s, got %d: %s", path, rec.Code, rec.Body.String())
			}
		}
	})

	t.Run("Coach_RestrictedViews", func(t *testing.T) {
		// Coaches cannot view Coaches directory, Packages, or Admin Portal (redirects to default page /)
		for _, path := range []string{"/coaches", "/packages", "/portal/admin"} {
			rec := testReq(tokCoach, path)
			if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
				t.Errorf("Coach expected redirect to / on %s, got %d to %s", path, rec.Code, rec.Header().Get("Location"))
			}
		}
	})

	// 3. ADMIN (Can view Dashboard, membership packages, attendance, and student directory)
	t.Run("Admin_AllowedViews", func(t *testing.T) {
		for _, path := range []string{"/", "/packages", "/sessions", "/students"} {
			rec := testReq(tokAdmin, path)
			if rec.Code != http.StatusOK {
				t.Errorf("Admin expected 200 on %s, got %d", path, rec.Code)
			}
		}
	})

	t.Run("Admin_RestrictedViews", func(t *testing.T) {
		// Admin cannot view Coaches or Admin Portal (redirects to default page /)
		for _, path := range []string{"/coaches", "/portal/admin"} {
			rec := testReq(tokAdmin, path)
			if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
				t.Errorf("Admin expected redirect to / on %s, got %d to %s", path, rec.Code, rec.Header().Get("Location"))
			}
		}
	})

	// 4. STUDENT (Can only view his/her profile)
	t.Run("Student_OwnProfile", func(t *testing.T) {
		rec := testReq(tokStudent, "/portal/student")
		if rec.Code != http.StatusOK {
			t.Errorf("Student expected 200 on /portal/student, got %d", rec.Code)
		}

		if studentUser.StudentID != nil {
			recOwn := testReq(tokStudent, "/students/"+studentUser.StudentID.String())
			if recOwn.Code != http.StatusOK {
				t.Errorf("Student expected 200 on own detail /students/%s, got %d", studentUser.StudentID.String(), recOwn.Code)
			}
		}
	})

	t.Run("Student_RestrictedViews", func(t *testing.T) {
		// Student cannot view other students, students list, sessions, coaches, packages, dashboard
		otherStudentID := "22222222-2222-2222-2222-222222222222"
		for _, path := range []string{"/", "/students", "/sessions", "/coaches", "/packages", "/portal/admin", "/students/" + otherStudentID} {
			rec := testReq(tokStudent, path)
			if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/portal/student" {
				t.Errorf("Student expected redirect to /portal/student on %s, got %d to %s", path, rec.Code, rec.Header().Get("Location"))
			}
		}
	})
}

func TestAdmin_StudentPermissions(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /students", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleStudents))
	mux.HandleFunc("GET /students/{id}", app.RequireRole(models.RoleStudent, models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleStudentDetail))
	mux.HandleFunc("POST /students", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleCreateStudent))
	mux.HandleFunc("POST /students/{id}", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleUpdateStudent))
	mux.HandleFunc("POST /students/{id}/evaluations", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleCreateEvaluation))

	handler := app.AuthMiddleware(mux)

	adminUser, _ := store.GetUserByEmail("admin@seriestkd.com")
	tokAdmin := "tok-admin-student-perm"
	_ = store.CreateSessionToken(tokAdmin, adminUser.ID, time.Now().Add(time.Hour))

	// 1. Admin can GET /students
	req := httptest.NewRequest("GET", "/students", nil)
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: tokAdmin})
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Admin expected 200 on GET /students, got %d", rec.Code)
	}

	// 2. Admin can POST /students (add student)
	form := url.Values{}
	form.Set("full_name", "Sarah Connor")
	form.Set("dob", "2005-04-12")
	form.Set("gender", "Female")
	form.Set("phone", "+1 555-0987")
	form.Set("current_belt", "White")
	form.Set("emergency_name", "John Connor")
	form.Set("emergency_phone", "+1 555-0988")
	form.Set("emergency_relation", "Son")
	form.Set("medical_notes", "No known allergies")

	req = httptest.NewRequest("POST", "/students", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: tokAdmin})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/students" {
		t.Fatalf("Admin expected 303 redirect to /students on create, got %d to %s", rec.Code, rec.Header().Get("Location"))
	}

	// Verify student was created
	students, _ := store.SearchStudents("Sarah Connor")
	if len(students) != 1 {
		t.Fatalf("expected 1 student named Sarah Connor, found %d", len(students))
	}
	sarah := students[0]

	// 3. Admin can GET /students/{id}
	req = httptest.NewRequest("GET", "/students/"+sarah.ID.String(), nil)
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: tokAdmin})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("Admin expected 200 on GET /students/%s, got %d", sarah.ID.String(), rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Sarah Connor") {
		t.Errorf("expected student detail page to contain Sarah Connor")
	}

	// 4. Admin can edit student personal and emergency info, but NOT belt rank
	editForm := url.Values{}
	editForm.Set("full_name", "Sarah J. Connor")
	editForm.Set("dob", "2005-04-12")
	editForm.Set("gender", "Female")
	editForm.Set("phone", "+1 555-1111")
	editForm.Set("emergency_name", "Kyle Reese")
	editForm.Set("emergency_phone", "+1 555-2222")
	editForm.Set("emergency_relation", "Partner")
	editForm.Set("medical_notes", "Asthma inhaler required")
	// Malicious/unauthorized attempt by admin to alter belt rank
	editForm.Set("current_belt", string(models.BeltBlack1stDan))

	req = httptest.NewRequest("POST", "/students/"+sarah.ID.String(), strings.NewReader(editForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: tokAdmin})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/students/"+sarah.ID.String() {
		t.Fatalf("Admin expected 303 redirect to /students/%s, got %d to %s", sarah.ID.String(), rec.Code, rec.Header().Get("Location"))
	}

	// Verify personal and emergency info was updated, but belt rank remained White
	updated, _ := store.GetStudentByID(sarah.ID)
	if updated.FullName != "Sarah J. Connor" {
		t.Errorf("expected FullName 'Sarah J. Connor', got %q", updated.FullName)
	}
	if updated.Phone != "+1 555-1111" {
		t.Errorf("expected Phone '+1 555-1111', got %q", updated.Phone)
	}
	if updated.EmergencyName != "Kyle Reese" {
		t.Errorf("expected EmergencyName 'Kyle Reese', got %q", updated.EmergencyName)
	}
	if updated.EmergencyPhone != "+1 555-2222" {
		t.Errorf("expected EmergencyPhone '+1 555-2222', got %q", updated.EmergencyPhone)
	}
	if updated.EmergencyRelation != "Partner" {
		t.Errorf("expected EmergencyRelation 'Partner', got %q", updated.EmergencyRelation)
	}
	if updated.MedicalNotes != "Asthma inhaler required" {
		t.Errorf("expected MedicalNotes 'Asthma inhaler required', got %q", updated.MedicalNotes)
	}
	// Belt rank MUST NOT be changed by Admin!
	if updated.CurrentBelt != models.BeltWhite {
		t.Errorf("security violation: admin altered belt rank to %q; must remain White", updated.CurrentBelt)
	}

	// 5. Admin CANNOT submit coach evaluations (forbidden / redirected)
	evalForm := url.Values{}
	evalForm.Set("coach_id", uuid.New().String())
	evalForm.Set("flexibility", "10")
	evalForm.Set("stamina", "10")
	evalForm.Set("power", "10")
	evalForm.Set("technique", "10")
	evalForm.Set("sparring_iq", "10")
	evalForm.Set("discipline", "10")
	evalForm.Set("coach_remarks", "Admin attempting evaluation")

	req = httptest.NewRequest("POST", "/students/"+sarah.ID.String()+"/evaluations", strings.NewReader(evalForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: tokAdmin})
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Admin is not authorized on POST /students/{id}/evaluations, must redirect to /
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/" {
		t.Errorf("expected Admin evaluation attempt to be rejected (303 to /), got %d to %s", rec.Code, rec.Header().Get("Location"))
	}
}

func TestAuthHandler_DemoLoginProductionGating(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	t.Run("development default shows demo login", func(t *testing.T) {
		t.Setenv("APP_ENV", "")
		t.Setenv("ENV", "")
		t.Setenv("GO_ENV", "")
		t.Setenv("ENVIRONMENT", "")
		t.Setenv("SHOW_DEMO_LOGIN", "")
		t.Setenv("ENABLE_DEMO_LOGIN", "")
		t.Setenv("RAILWAY_ENVIRONMENT", "")
		t.Setenv("RAILWAY_ENVIRONMENT_NAME", "")
		t.Setenv("DATABASE_URL", "")

		if !app.IsDemoLoginEnabled() {
			t.Errorf("expected demo login enabled in development default")
		}

		req := httptest.NewRequest("GET", "/login", nil)
		rec := httptest.NewRecorder()
		app.HandleLoginPage(rec, req)
		body := rec.Body.String()

		if !strings.Contains(body, "Quick Demo Sign-In") {
			t.Errorf("expected Quick Demo Sign-In in dev login page HTML")
		}
		if !strings.Contains(body, "fillDemo") {
			t.Errorf("expected fillDemo script in dev login page HTML")
		}
	})

	t.Run("production via APP_ENV hides demo login", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("SHOW_DEMO_LOGIN", "")
		t.Setenv("ENABLE_DEMO_LOGIN", "")

		if app.IsDemoLoginEnabled() {
			t.Errorf("expected demo login disabled when APP_ENV=production")
		}

		req := httptest.NewRequest("GET", "/login", nil)
		rec := httptest.NewRecorder()
		app.HandleLoginPage(rec, req)
		body := rec.Body.String()

		if strings.Contains(body, "Quick Demo Sign-In") {
			t.Errorf("demo login must NOT be shown when APP_ENV=production")
		}
		if strings.Contains(body, "fillDemo") {
			t.Errorf("fillDemo credentials script must NOT be rendered when APP_ENV=production")
		}
	})

	t.Run("production via ENV=prod hides demo login", func(t *testing.T) {
		t.Setenv("APP_ENV", "")
		t.Setenv("ENV", "prod")
		t.Setenv("SHOW_DEMO_LOGIN", "")

		if app.IsDemoLoginEnabled() {
			t.Errorf("expected demo login disabled when ENV=prod")
		}

		req := httptest.NewRequest("GET", "/login", nil)
		rec := httptest.NewRecorder()
		app.HandleLoginPage(rec, req)
		body := rec.Body.String()

		if strings.Contains(body, "Quick Demo Sign-In") || strings.Contains(body, "fillDemo") {
			t.Errorf("demo login must NOT be shown when ENV=prod")
		}
	})

	t.Run("production via DATABASE_URL postgres hides demo login", func(t *testing.T) {
		t.Setenv("APP_ENV", "")
		t.Setenv("ENV", "")
		t.Setenv("SHOW_DEMO_LOGIN", "")
		t.Setenv("DATABASE_URL", "postgres://user:pass@host:5432/series_tkd")

		if app.IsDemoLoginEnabled() {
			t.Errorf("expected demo login disabled when DATABASE_URL is postgres")
		}

		req := httptest.NewRequest("GET", "/login", nil)
		rec := httptest.NewRecorder()
		app.HandleLoginPage(rec, req)
		body := rec.Body.String()

		if strings.Contains(body, "Quick Demo Sign-In") || strings.Contains(body, "fillDemo") {
			t.Errorf("demo login must NOT be shown when DATABASE_URL is postgres")
		}
	})

	t.Run("explicit override SHOW_DEMO_LOGIN=false hides in development", func(t *testing.T) {
		t.Setenv("APP_ENV", "development")
		t.Setenv("SHOW_DEMO_LOGIN", "false")

		if app.IsDemoLoginEnabled() {
			t.Errorf("expected demo login disabled when SHOW_DEMO_LOGIN=false")
		}

		req := httptest.NewRequest("GET", "/login", nil)
		rec := httptest.NewRecorder()
		app.HandleLoginPage(rec, req)
		body := rec.Body.String()

		if strings.Contains(body, "Quick Demo Sign-In") || strings.Contains(body, "fillDemo") {
			t.Errorf("demo login must NOT be shown when SHOW_DEMO_LOGIN=false")
		}
	})

	t.Run("explicit override SHOW_DEMO_LOGIN=true enables even in production", func(t *testing.T) {
		t.Setenv("APP_ENV", "production")
		t.Setenv("SHOW_DEMO_LOGIN", "true")

		if !app.IsDemoLoginEnabled() {
			t.Errorf("expected demo login enabled when SHOW_DEMO_LOGIN=true override is set")
		}

		req := httptest.NewRequest("GET", "/login", nil)
		rec := httptest.NewRecorder()
		app.HandleLoginPage(rec, req)
		body := rec.Body.String()

		if !strings.Contains(body, "Quick Demo Sign-In") {
			t.Errorf("expected Quick Demo Sign-In when SHOW_DEMO_LOGIN=true override")
		}
	})
}

func TestAuthHandler_UsernameLoginFlow(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	// 1. Login with username via form POST /login
	form := url.Values{}
	form.Set("identifier", "admin")
	form.Set("password", "admin123")
	req := httptest.NewRequest("POST", "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	app.HandleLoginSubmit(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on successful login, got %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/" {
		t.Errorf("expected redirect to / for admin, got %s", rec.Header().Get("Location"))
	}

	// 2. Login with student username
	formStudent := url.Values{}
	formStudent.Set("identifier", "alex.vance")
	formStudent.Set("password", "student123")
	req = httptest.NewRequest("POST", "/login", strings.NewReader(formStudent.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleLoginSubmit(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on student login, got %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/portal/student" {
		t.Errorf("expected redirect to /portal/student, got %s", rec.Header().Get("Location"))
	}

	// 3. API Login with username
	apiBody := map[string]string{
		"identifier": "manager",
		"password":   "manager123",
	}
	bodyJSON, _ := json.Marshal(apiBody)
	req = httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(bodyJSON))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	app.HandleAPILogin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on API login with username, got %d: %s", rec.Code, rec.Body.String())
	}
	var res handlers.LoginResponse
	_ = json.Unmarshal(rec.Body.Bytes(), &res)
	if res.User == nil || res.User.Username != "manager" {
		t.Errorf("expected user with username 'manager'")
	}
}

func TestAuthHandler_ForgotPasswordAndResetFlow(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	// 1. GET /forgot-password
	req := httptest.NewRequest("GET", "/forgot-password", nil)
	rec := httptest.NewRecorder()
	app.HandleForgotPasswordPage(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /forgot-password, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "PASSWORD") || !strings.Contains(rec.Body.String(), "RECOVERY") {
		t.Errorf("forgot password page missing title keywords")
	}

	// 2. POST /forgot-password with valid username
	form := url.Values{}
	form.Set("identifier", "alex.vance")
	req = httptest.NewRequest("POST", "/forgot-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleForgotPasswordSubmit(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on submit, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "success=") {
		t.Errorf("expected success query parameter in redirect, got %s", rec.Header().Get("Location"))
	}

	// 3. GET /reset-password without token -> error redirect
	req = httptest.NewRequest("GET", "/reset-password", nil)
	rec = httptest.NewRecorder()
	app.HandleResetPasswordPage(rec, req)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "error=") {
		t.Errorf("expected error redirect when token is missing, got %d to %s", rec.Code, rec.Header().Get("Location"))
	}

	// 4. Generate token via AuthService and test valid GET /reset-password
	tok, err := app.AuthService().RequestPasswordReset("alex.vance")
	if err != nil || tok == nil {
		t.Fatalf("failed to create reset token: %v", err)
	}

	req = httptest.NewRequest("GET", "/reset-password?token="+tok.Token, nil)
	rec = httptest.NewRecorder()
	app.HandleResetPasswordPage(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on valid /reset-password?token=..., got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "SET NEW") || !strings.Contains(rec.Body.String(), "PASSWORD") {
		t.Errorf("reset password page missing title")
	}

	// 5. POST /reset-password with mismatched passwords -> error
	resetForm := url.Values{}
	resetForm.Set("token", tok.Token)
	resetForm.Set("new_password", "newsecret123")
	resetForm.Set("confirm_password", "mismatchpass")
	req = httptest.NewRequest("POST", "/reset-password", strings.NewReader(resetForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleResetPasswordSubmit(rec, req)
	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "error=") {
		t.Errorf("expected error on password mismatch")
	}

	// 6. POST /reset-password with matching valid password
	resetForm.Set("confirm_password", "newsecret123")
	req = httptest.NewRequest("POST", "/reset-password", strings.NewReader(resetForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleResetPasswordSubmit(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on successful reset, got %d", rec.Code)
	}
	if !strings.Contains(rec.Header().Get("Location"), "/login?success=") {
		t.Errorf("expected redirect to /login with success banner, got %s", rec.Header().Get("Location"))
	}

	// 7. Verify login with new password
	loginForm := url.Values{}
	loginForm.Set("identifier", "alex.vance")
	loginForm.Set("password", "newsecret123")
	req = httptest.NewRequest("POST", "/login", strings.NewReader(loginForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleLoginSubmit(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/portal/student" {
		t.Fatalf("expected login with new password to succeed and route to /portal/student, got %d to %s", rec.Code, rec.Header().Get("Location"))
	}
}

func TestAuthHandler_APIForgotAndResetPassword(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	// 1. POST /api/auth/forgot-password
	reqBody, _ := json.Marshal(map[string]string{"identifier": "manager"})
	req := httptest.NewRequest("POST", "/api/auth/forgot-password", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.HandleAPIForgotPassword(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/auth/forgot-password, got %d", rec.Code)
	}

	// 2. Generate token for reset
	tok, _ := app.AuthService().RequestPasswordReset("manager")

	// 3. POST /api/auth/reset-password
	resetBody, _ := json.Marshal(map[string]string{
		"token":    tok.Token,
		"password": "managerNewPass2026",
	})
	req = httptest.NewRequest("POST", "/api/auth/reset-password", bytes.NewReader(resetBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	app.HandleAPIResetPassword(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on /api/auth/reset-password, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. Verify login with new password via API
	loginBody, _ := json.Marshal(map[string]string{
		"identifier": "manager",
		"password":   "managerNewPass2026",
	})
	req = httptest.NewRequest("POST", "/api/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	app.HandleAPILogin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on API login with new password, got %d", rec.Code)
	}
}

func TestAuthHandler_CoachAndAdminRegistrationFlow(t *testing.T) {
	app, store := setupTestApp(t)

	// 1. GET /register returns 200 OK with registration view
	req := httptest.NewRequest("GET", "/register", nil)
	rec := httptest.NewRecorder()
	app.HandleRegisterPage(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET /register, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "REGISTRATION") || !strings.Contains(body, "Coach / Instructor") {
		t.Errorf("registration page missing expected branding or role buttons")
	}
	if strings.Contains(body, `class="btn btn-primary btn-sm">`) && strings.Contains(body, "Sign In") {
		t.Errorf("register page should not have top right sign in button in header")
	}
	if strings.Contains(body, `class="btn btn-primary btn-xs">Sign In</a>`) {
		t.Errorf("register page should not have mobile top right sign in button in header")
	}

	// 2. POST /register for Coach
	coachForm := url.Values{
		"role":             {"COACH"},
		"full_name":        {"Coach Min-Soo Kang"},
		"email":            {"minsoo.kang@seriestkd.com"},
		"username":         {"minsoo.kang"},
		"password":         {"coachPassword123"},
		"confirm_password": {"coachPassword123"},
		"phone":            {"+63 917 111 2222"},
		"belt_rank":        {"3rd Dan Black Belt"},
		"specialties":      {"Sparring, Conditioning"},
	}
	req = httptest.NewRequest("POST", "/register", strings.NewReader(coachForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleRegisterSubmit(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on coach registration, got %d: %s", rec.Code, rec.Body.String())
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/login") || !strings.Contains(loc, "pending+manager+approval") {
		t.Errorf("expected redirect to /login with pending approval notice, got: %s", loc)
	}

	// Verify coach and user stored with IsActive = false
	coachUser, err := store.GetUserByEmail("minsoo.kang@seriestkd.com")
	if err != nil || coachUser == nil {
		t.Fatalf("expected coach user record in store, got: %v", err)
	}
	if coachUser.IsActive {
		t.Errorf("expected coach user to be inactive pending approval")
	}
	if coachUser.CoachID == nil {
		t.Fatalf("expected coachUser to have linked CoachID")
	}
	coachRec, err := store.GetCoachByID(*coachUser.CoachID)
	if err != nil || coachRec == nil {
		t.Fatalf("expected coach record in store, got: %v", err)
	}
	if coachRec.IsActive {
		t.Errorf("expected coach record to be inactive pending approval")
	}

	// 3. Unapproved coach tries to login -> must be blocked with pending approval notice
	loginForm := url.Values{
		"identifier": {"minsoo.kang"},
		"password":   {"coachPassword123"},
	}
	req = httptest.NewRequest("POST", "/login", strings.NewReader(loginForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleLoginSubmit(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on login attempt, got %d", rec.Code)
	}
	loc = rec.Header().Get("Location")
	if !strings.Contains(loc, "pending+manager+approval") {
		t.Errorf("expected redirect with pending manager approval error, got: %s", loc)
	}

	// 4. Operations Manager approves coach via ToggleCoachActive
	if err := store.ToggleCoachActive(coachRec.ID, true); err != nil {
		t.Fatalf("failed to approve coach: %v", err)
	}

	// 5. Approved coach logs in successfully
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/login", strings.NewReader(loginForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.HandleLoginSubmit(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 after approved login, got %d", rec.Code)
	}
	loc = rec.Header().Get("Location")
	if loc != "/" {
		t.Errorf("expected redirect to / for coach, got: %s", loc)
	}
	cookie := rec.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "stms_session=") {
		t.Errorf("expected stms_session cookie on approved coach login")
	}

	// 6. POST /register for Administrator
	adminForm := url.Values{
		"role":             {"ADMIN"},
		"full_name":        {"Admin Clara Oswald"},
		"email":            {"clara@seriestkd.com"},
		"username":         {"clara.oswald"},
		"password":         {"adminPassword123"},
		"confirm_password": {"adminPassword123"},
	}
	req = httptest.NewRequest("POST", "/register", strings.NewReader(adminForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleRegisterSubmit(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on admin registration, got %d", rec.Code)
	}
	loc = rec.Header().Get("Location")
	if !strings.Contains(loc, "/login") || !strings.Contains(loc, "pending+manager+approval") {
		t.Errorf("expected redirect to /login with pending approval notice, got: %s", loc)
	}

	adminUser, err := store.GetUserByEmail("clara@seriestkd.com")
	if err != nil || adminUser == nil {
		t.Fatalf("expected admin user in store, got: %v", err)
	}
	if adminUser.IsActive {
		t.Errorf("expected admin user to be inactive pending approval")
	}

	// 7. Unapproved admin tries to login -> must be blocked
	adminLoginForm := url.Values{
		"identifier": {"clara.oswald"},
		"password":   {"adminPassword123"},
	}
	req = httptest.NewRequest("POST", "/login", strings.NewReader(adminLoginForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleLoginSubmit(rec, req)
	loc = rec.Header().Get("Location")
	if !strings.Contains(loc, "pending+manager+approval") {
		t.Errorf("expected redirect with pending manager approval error, got: %s", loc)
	}

	// 8. Operations Manager approves admin via ToggleUserActive
	if err := store.ToggleUserActive(adminUser.ID, true); err != nil {
		t.Fatalf("failed to approve admin: %v", err)
	}

	// 9. Approved admin logs in successfully
	rec = httptest.NewRecorder()
	req = httptest.NewRequest("POST", "/login", strings.NewReader(adminLoginForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	app.HandleLoginSubmit(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 after approved login, got %d", rec.Code)
	}
	loc = rec.Header().Get("Location")
	if loc != "/" {
		t.Errorf("expected redirect to / for admin, got: %s", loc)
	}
	cookie = rec.Header().Get("Set-Cookie")
	if !strings.Contains(cookie, "stms_session=") {
		t.Errorf("expected stms_session cookie on approved admin login")
	}

	// 10. Manager can reject/delete an unapproved registration
	rejectForm := url.Values{
		"role":             {"ADMIN"},
		"full_name":        {"Rejected Admin"},
		"email":            {"rejected@seriestkd.com"},
		"username":         {"rejected.admin"},
		"password":         {"pass123456"},
		"confirm_password": {"pass123456"},
	}
	req = httptest.NewRequest("POST", "/register", strings.NewReader(rejectForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleRegisterSubmit(rec, req)
	rejUser, _ := store.GetUserByEmail("rejected@seriestkd.com")
	if rejUser == nil {
		t.Fatalf("expected rejected user to be initially registered")
	}

	// Call HandleDeleteAdmin with mux path value
	delReq := httptest.NewRequest("POST", "/admins/"+rejUser.ID.String()+"/delete", nil)
	delReq.SetPathValue("id", rejUser.ID.String())
	delRec := httptest.NewRecorder()
	app.HandleDeleteAdmin(delRec, delReq)
	if delRec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect after deleting admin, got %d", delRec.Code)
	}
	deletedCheck, _ := store.GetUserByID(rejUser.ID)
	if deletedCheck != nil {
		t.Errorf("expected user to be completely deleted after rejection")
	}
}

func TestAuthHandler_RegistrationRoutesAndAliases(t *testing.T) {
	app, store := setupTestApp(t)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /register", app.HandleRegisterPage)
	mux.HandleFunc("POST /register", app.HandleRegisterSubmit)
	mux.HandleFunc("GET /register/", func(w http.ResponseWriter, r *http.Request) {
		target := "/register"
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
	})
	mux.HandleFunc("GET /registration", func(w http.ResponseWriter, r *http.Request) {
		target := "/register"
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
	})
	mux.HandleFunc("GET /registration/", func(w http.ResponseWriter, r *http.Request) {
		target := "/register"
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
	})
	mux.HandleFunc("POST /registration", app.HandleRegisterSubmit)

	handler := app.AuthMiddleware(mux)

	// 1. GET /register returns 200 OK
	req := httptest.NewRequest("GET", "/register", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on GET /register, got %d", rec.Code)
	}

	// 2. GET /registration redirects to /register with query params preserved
	req = httptest.NewRequest("GET", "/registration?role=admin", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on GET /registration, got %d", rec.Code)
	}
	if rec.Header().Get("Location") != "/register?role=admin" {
		t.Fatalf("expected redirect to /register?role=admin, got %s", rec.Header().Get("Location"))
	}

	// 3. GET /registration/ redirects to /register
	req = httptest.NewRequest("GET", "/registration/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/register" {
		t.Fatalf("expected redirect to /register, got %d to %s", rec.Code, rec.Header().Get("Location"))
	}

	// 4. POST /registration works seamlessly
	form := url.Values{
		"role":             {"COACH"},
		"full_name":        {"Coach Alias Test"},
		"email":            {"aliastest@seriestkd.com"},
		"username":         {"aliastest"},
		"password":         {"password123"},
		"confirm_password": {"password123"},
		"phone":            {"+63 917 555 8888"},
	}
	req = httptest.NewRequest("POST", "/registration", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on POST /registration, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Location"), "/login") {
		t.Fatalf("expected redirect to /login after registration, got: %s", rec.Header().Get("Location"))
	}

	user, err := store.GetUserByEmail("aliastest@seriestkd.com")
	if err != nil || user == nil {
		t.Fatalf("expected user to be created from POST /registration")
	}
}




