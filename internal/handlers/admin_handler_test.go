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

func TestAdminHandler_HandleAdmins_AccessControl(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	adminUser, _ := store.GetUserByEmail("admin@seriestkd.com")
	coachUser, _ := store.GetUserByEmail("jiwoo.park@seriestkd.com")
	studentUser, _ := store.GetUserByEmail("alex.vance@seriestkd.com")

	withSession := func(req *http.Request, u *models.User) *http.Request {
		token := uuid.New().String()
		_ = store.CreateSessionToken(token, u.ID, time.Now().Add(time.Hour))
		req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
		return req
	}

	executeHandler := func(rec *httptest.ResponseRecorder, req *http.Request) {
		app.AuthMiddleware(app.RequireRole(models.RoleOperationManager)(app.HandleAdmins)).ServeHTTP(rec, req)
	}

	// 1. Unauthenticated request to /admins via RequireRole guard
	req := httptest.NewRequest("GET", "/admins", nil)
	rec := httptest.NewRecorder()
	executeHandler(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for unauthenticated user, got %d", rec.Code)
	}

	// 2. Unauthorized roles (Admin, Coach, Student) should be redirected
	for roleName, user := range map[string]*models.User{
		"Admin":   adminUser,
		"Coach":   coachUser,
		"Student": studentUser,
	} {
		req = httptest.NewRequest("GET", "/admins", nil)
		req = withSession(req, user)
		rec = httptest.NewRecorder()
		executeHandler(rec, req)

		if rec.Code != http.StatusSeeOther {
			t.Errorf("expected redirect for role %s, got %d", roleName, rec.Code)
		}
	}

	// 3. Authorized Operation Manager should get 200 OK
	req = httptest.NewRequest("GET", "/admins", nil)
	req = withSession(req, managerUser)
	rec = httptest.NewRecorder()
	executeHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for Operation Manager, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Administrator Directory") {
		t.Errorf("expected page header in body")
	}
	if !strings.Contains(body, "admin@seriestkd.com") {
		t.Errorf("expected seed admin to be displayed")
	}
}

func TestAdminHandler_HandleCreateAdmin(t *testing.T) {
	app, store := setupTestApp(t)

	// 1. Missing fields should redirect with error
	form := url.Values{}
	form.Set("full_name", "")
	form.Set("email", "")
	req := httptest.NewRequest("POST", "/admins", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	app.HandleCreateAdmin(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 on missing fields, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected error query param, got %s", loc)
	}

	// 2. Short password (< 6 chars) should redirect with error
	form = url.Values{}
	form.Set("full_name", "Test Admin")
	form.Set("email", "test.admin@seriestkd.com")
	form.Set("password", "123")
	req = httptest.NewRequest("POST", "/admins", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleCreateAdmin(rec, req)

	loc = rec.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected error for short password, got %s", loc)
	}

	// 3. Valid creation with custom password
	form = url.Values{}
	form.Set("full_name", "Sarah Connor")
	form.Set("email", "sarah.connor@seriestkd.com")
	form.Set("password", "secureadmin123")
	req = httptest.NewRequest("POST", "/admins", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleCreateAdmin(rec, req)

	loc = rec.Header().Get("Location")
	if !strings.Contains(loc, "success=") {
		t.Errorf("expected success redirect, got %s", loc)
	}

	// Verify user created in store
	created, err := store.GetUserByEmail("sarah.connor@seriestkd.com")
	if err != nil || created == nil {
		t.Fatalf("created user not found in store")
	}
	if created.Role != models.RoleAdmin {
		t.Errorf("expected RoleAdmin, got %s", created.Role)
	}
	if created.DisplayName != "Sarah Connor" {
		t.Errorf("expected DisplayName 'Sarah Connor', got '%s'", created.DisplayName)
	}
	if !created.CheckPassword("secureadmin123") {
		t.Errorf("password verification failed")
	}

	// 4. Duplicate email creation should fail
	req = httptest.NewRequest("POST", "/admins", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	app.HandleCreateAdmin(rec, req)

	loc = rec.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected error for duplicate email, got %s", loc)
	}
}

func TestAdminHandler_HandleToggleAdminStatus(t *testing.T) {
	app, store := setupTestApp(t)

	// Create a test admin
	testAdmin := &models.User{
		ID:          uuid.New(),
		Email:       "toggle.admin@seriestkd.com",
		Role:        models.RoleAdmin,
		DisplayName: "Toggle Admin",
		IsActive:    true,
	}
	_ = testAdmin.SetPassword("admin123")
	_ = store.CreateUser(testAdmin)

	// Deactivate
	req := httptest.NewRequest("POST", "/admins/"+testAdmin.ID.String()+"/toggle", nil)
	req.SetPathValue("id", testAdmin.ID.String())
	rec := httptest.NewRecorder()
	app.HandleToggleAdminStatus(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on toggle, got %d", rec.Code)
	}
	updated, _ := store.GetUserByID(testAdmin.ID)
	if updated.IsActive {
		t.Errorf("expected user to be deactivated")
	}

	// Reactivate
	req = httptest.NewRequest("POST", "/admins/"+testAdmin.ID.String()+"/toggle", nil)
	req.SetPathValue("id", testAdmin.ID.String())
	rec = httptest.NewRecorder()
	app.HandleToggleAdminStatus(rec, req)

	updated, _ = store.GetUserByID(testAdmin.ID)
	if !updated.IsActive {
		t.Errorf("expected user to be reactivated")
	}

	// Attempting to toggle non-admin should fail
	coachUser, _ := store.GetUserByEmail("jiwoo.park@seriestkd.com")
	req = httptest.NewRequest("POST", "/admins/"+coachUser.ID.String()+"/toggle", nil)
	req.SetPathValue("id", coachUser.ID.String())
	rec = httptest.NewRecorder()
	app.HandleToggleAdminStatus(rec, req)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected error when trying to toggle non-admin, got %s", loc)
	}
}

func TestAdminHandler_HandleResetAdminPassword(t *testing.T) {
	app, store := setupTestApp(t)

	testAdmin := &models.User{
		ID:          uuid.New(),
		Email:       "reset.admin@seriestkd.com",
		Role:        models.RoleAdmin,
		DisplayName: "Reset Admin",
		IsActive:    true,
	}
	_ = testAdmin.SetPassword("oldpassword123")
	_ = store.CreateUser(testAdmin)

	// 1. Password confirmation mismatch
	form := url.Values{}
	form.Set("new_password", "newpassword123")
	form.Set("confirm_password", "differentpassword")
	req := httptest.NewRequest("POST", "/admins/"+testAdmin.ID.String()+"/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", testAdmin.ID.String())
	rec := httptest.NewRecorder()
	app.HandleResetAdminPassword(rec, req)

	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected error on password mismatch, got %s", loc)
	}

	// 2. Successful password reset
	form = url.Values{}
	form.Set("new_password", "brandnewpass123")
	form.Set("confirm_password", "brandnewpass123")
	req = httptest.NewRequest("POST", "/admins/"+testAdmin.ID.String()+"/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", testAdmin.ID.String())
	rec = httptest.NewRecorder()
	app.HandleResetAdminPassword(rec, req)

	loc = rec.Header().Get("Location")
	if !strings.Contains(loc, "success=") {
		t.Errorf("expected success on reset, got %s", loc)
	}

	updated, _ := store.GetUserByID(testAdmin.ID)
	if !updated.CheckPassword("brandnewpass123") {
		t.Errorf("new password verification failed")
	}
	if updated.CheckPassword("oldpassword123") {
		t.Errorf("old password should no longer be valid")
	}
}
