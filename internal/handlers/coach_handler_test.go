package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
)

func TestCoachHandler_HandleCoaches_TelemetryAndDisplay(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")

	req := httptest.NewRequest("GET", "/coaches", nil)
	token := uuid.New().String()
	_ = store.CreateSessionToken(token, managerUser.ID, time.Now().Add(time.Hour))
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})

	rec := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager)(app.HandleCoaches)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HandleCoaches, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Total Coaches") {
		t.Errorf("expected page to contain 'Total Coaches' telemetry")
	}
	if !strings.Contains(body, "Active Instructors") {
		t.Errorf("expected page to contain 'Active Instructors' telemetry")
	}
	if !strings.Contains(body, "Deactivated") {
		t.Errorf("expected page to contain 'Deactivated' telemetry")
	}
	if !strings.Contains(body, "First Aid Certified") {
		t.Errorf("expected page to contain 'First Aid Certified' telemetry")
	}
	if !strings.Contains(body, "Deactivate") {
		t.Errorf("expected page to contain 'Deactivate' action")
	}
}

func TestCoachHandler_HandleToggleCoachStatus_BlocksLogin(t *testing.T) {
	app, store := setupTestApp(t)

	c2ID := uuid.MustParse("22222222-2222-2222-2222-222222222222") // Ji-Woo Park

	// 1. Initial Login should succeed
	loginForm := url.Values{"email": {"jiwoo.park@seriestkd.com"}, "password": {"coach123"}}
	req := httptest.NewRequest("POST", "/login", strings.NewReader(loginForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	app.HandleLoginSubmit(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 for successful login, got %d", rec.Code)
	}
	cookie := rec.Result().Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookie {
		if c.Name == "stms_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("expected session cookie to be set")
	}

	// 2. Deactivate Coach via HandleToggleCoachStatus
	toggleReq := httptest.NewRequest("POST", "/coaches/"+c2ID.String()+"/toggle", nil)
	toggleReq.SetPathValue("id", c2ID.String())
	toggleRec := httptest.NewRecorder()
	app.HandleToggleCoachStatus(toggleRec, toggleReq)

	if toggleRec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect after toggle, got %d", toggleRec.Code)
	}
	loc := toggleRec.Header().Get("Location")
	if !strings.Contains(loc, "success=") || !strings.Contains(loc, "deactivated") {
		t.Errorf("expected redirect location with success message about deactivation, got %s", loc)
	}

	// Verify coach is inactive in store
	coach, _ := store.GetCoachByID(c2ID)
	if coach.IsActive {
		t.Errorf("expected coach to be inactive in store")
	}

	// 3. Attempting to access protected portal using previous session should fail
	portalReq := httptest.NewRequest("GET", "/portal/coach", nil)
	portalReq.AddCookie(sessionCookie)
	portalRec := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleCoach)(app.HandleCoachPortal)).ServeHTTP(portalRec, portalReq)

	// Since session was invalidated, should be redirected to login
	if portalRec.Code != http.StatusSeeOther {
		t.Errorf("expected redirect to login for deactivated session, got %d", portalRec.Code)
	}

	// 4. Attempting to log in as deactivated coach must fail
	loginReq2 := httptest.NewRequest("POST", "/login", strings.NewReader(loginForm.Encode()))
	loginReq2.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRec2 := httptest.NewRecorder()
	app.HandleLoginSubmit(loginRec2, loginReq2)

	loginLoc := loginRec2.Header().Get("Location")
	if !strings.Contains(loginLoc, "Account+is+inactive") && !strings.Contains(loginLoc, "inactive") {
		t.Errorf("expected redirect with inactive account error, got %s", loginLoc)
	}

	// 5. Reactivate Coach via HandleToggleCoachStatus
	reactivateReq := httptest.NewRequest("POST", "/coaches/"+c2ID.String()+"/toggle", nil)
	reactivateReq.SetPathValue("id", c2ID.String())
	reactivateRec := httptest.NewRecorder()
	app.HandleToggleCoachStatus(reactivateRec, reactivateReq)

	if reactivateRec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect after reactivation, got %d", reactivateRec.Code)
	}

	// 6. Login should now succeed again
	loginReq3 := httptest.NewRequest("POST", "/login", strings.NewReader(loginForm.Encode()))
	loginReq3.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRec3 := httptest.NewRecorder()
	app.HandleLoginSubmit(loginRec3, loginReq3)

	if loginRec3.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on login after reactivation, got %d", loginRec3.Code)
	}
	if strings.Contains(loginRec3.Header().Get("Location"), "error=") {
		t.Errorf("expected clean login with no error after reactivation, got %s", loginRec3.Header().Get("Location"))
	}
}

func TestCoachHandler_HandleDeleteCoach(t *testing.T) {
	app, store := setupTestApp(t)

	c2ID := uuid.MustParse("22222222-2222-2222-2222-222222222222") // Ji-Woo Park (has sessions)

	// 1. Delete coach with existing sessions -> should cascade and succeed
	delReq := httptest.NewRequest("POST", "/coaches/"+c2ID.String()+"/delete", nil)
	delReq.SetPathValue("id", c2ID.String())
	delRec := httptest.NewRecorder()
	app.HandleDeleteCoach(delRec, delReq)

	if delRec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on successful cascade delete, got %d", delRec.Code)
	}
	loc := delRec.Header().Get("Location")
	if !strings.Contains(loc, "success=") || !strings.Contains(loc, "deleted") {
		t.Errorf("expected success notification on cascade delete, got %s", loc)
	}

	// Coach should be deleted
	if _, err := store.GetCoachByID(c2ID); err == nil {
		t.Fatalf("expected coach to be cascade deleted from store")
	}

	// 2. Create fresh coach with no sessions or evaluations
	freshCoach := &models.Coach{
		ID:             uuid.New(),
		FullName:       "Disposable Coach",
		Email:          "disposable@seriestkd.com",
		BeltRank:       "1st Dan",
		RatePerSession: 40,
		IsActive:       true,
		CreatedAt:      time.Now(),
	}
	_ = store.CreateCoach(freshCoach)
	freshUser := &models.User{
		ID:          uuid.New(),
		Email:       freshCoach.Email,
		Role:        models.RoleCoach,
		CoachID:     &freshCoach.ID,
		DisplayName: freshCoach.FullName,
		IsActive:    true,
	}
	_ = freshUser.SetPassword("coach123")
	_ = store.CreateUser(freshUser)

	// Delete fresh coach -> should succeed
	delFreshReq := httptest.NewRequest("POST", "/coaches/"+freshCoach.ID.String()+"/delete", nil)
	delFreshReq.SetPathValue("id", freshCoach.ID.String())
	delFreshRec := httptest.NewRecorder()
	app.HandleDeleteCoach(delFreshRec, delFreshReq)

	if delFreshRec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on successful delete, got %d", delFreshRec.Code)
	}
	locFresh := delFreshRec.Header().Get("Location")
	if !strings.Contains(locFresh, "success=") || !strings.Contains(locFresh, "deleted") {
		t.Errorf("expected success notification on delete, got %s", locFresh)
	}

	// Verify coach and user no longer exist
	if _, err := store.GetCoachByID(freshCoach.ID); err == nil {
		t.Errorf("expected coach to be deleted from store")
	}
	if _, err := store.GetUserByEmail(freshCoach.Email); err == nil {
		t.Errorf("expected coach user account to be deleted from store")
	}

	// Login attempt should fail with invalid credentials (not found)
	loginForm := url.Values{"email": {freshCoach.Email}, "password": {"coach123"}}
	loginReq := httptest.NewRequest("POST", "/login", strings.NewReader(loginForm.Encode()))
	loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	loginRec := httptest.NewRecorder()
	app.HandleLoginSubmit(loginRec, loginReq)

	loginLoc := loginRec.Header().Get("Location")
	if !strings.Contains(loginLoc, "Invalid+email+or+password") {
		t.Errorf("expected Invalid email or password for deleted coach, got %s", loginLoc)
	}
}
