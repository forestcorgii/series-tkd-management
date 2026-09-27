package handlers_test

import (
	"encoding/json"
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

func TestAdminHandler_JSON_HTMX_Flows(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	withSession := func(req *http.Request, u *models.User) *http.Request {
		token := uuid.New().String()
		_ = store.CreateSessionToken(token, u.ID, time.Now().Add(time.Hour))
		req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
		return req
	}

	// 1. GET /admins with Accept: application/json returns 200 OK JSON
	req := httptest.NewRequest("GET", "/admins", nil)
	req.Header.Set("Accept", "application/json")
	req = withSession(req, managerUser)
	rec := httptest.NewRecorder()
	app.AuthMiddleware(http.HandlerFunc(app.HandleAdmins)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for JSON GET /admins, got %d", rec.Code)
	}
	var getResp struct {
		Status string         `json:"status"`
		Total  int            `json:"total"`
		Admins []*models.User `json:"admins"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&getResp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if getResp.Total == 0 || len(getResp.Admins) == 0 {
		t.Errorf("expected admins in JSON response, got total=%d", getResp.Total)
	}

	// 2. POST /admins with JSON body (Valid)
	createBody := map[string]string{
		"full_name": "API Admin",
		"email":     "api.admin@seriestkd.com",
		"password":  "securepass123",
	}
	bodyBytes, _ := json.Marshal(createBody)
	req = httptest.NewRequest("POST", "/admins", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")
	req = withSession(req, managerUser)
	rec = httptest.NewRecorder()
	app.HandleCreateAdmin(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created for JSON POST /admins, got %d. Body: %s", rec.Code, rec.Body.String())
	}
	var createResp struct {
		Status string       `json:"status"`
		Admin  *models.User `json:"admin"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&createResp); err != nil {
		t.Fatalf("failed to decode create JSON: %v", err)
	}
	if createResp.Admin == nil || createResp.Admin.Email != "api.admin@seriestkd.com" {
		t.Errorf("expected created admin email api.admin@seriestkd.com, got %v", createResp.Admin)
	}

	// 3. POST /admins with JSON body (Duplicate Email Conflict)
	req = httptest.NewRequest("POST", "/admins", strings.NewReader(string(bodyBytes)))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	app.HandleCreateAdmin(rec, req)
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 Conflict for duplicate admin, got %d", rec.Code)
	}

	// 4. POST /admins with HTMX (Validation Error & Success)
	form := url.Values{}
	form.Set("full_name", "")
	form.Set("email", "")
	req = httptest.NewRequest("POST", "/admins", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec = httptest.NewRecorder()
	app.HandleCreateAdmin(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 Bad Request for HTMX missing fields, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "required") {
		t.Errorf("expected error banner in HTMX body, got: %s", rec.Body.String())
	}

	// Valid HTMX creation
	form.Set("full_name", "HTMX Admin")
	form.Set("email", "htmx.admin@seriestkd.com")
	form.Set("password", "htmxpass123")
	req = httptest.NewRequest("POST", "/admins", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	rec = httptest.NewRecorder()
	app.HandleCreateAdmin(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for valid HTMX creation, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "provisioned successfully") {
		t.Errorf("expected success banner in HTMX body, got: %s", rec.Body.String())
	}

	// 5. POST /admins/{id}/toggle with JSON
	createdAdmin, _ := store.GetUserByEmail("api.admin@seriestkd.com")
	req = httptest.NewRequest("POST", "/admins/"+createdAdmin.ID.String()+"/toggle", nil)
	req.Header.Set("Accept", "application/json")
	req.SetPathValue("id", createdAdmin.ID.String())
	rec = httptest.NewRecorder()
	app.HandleToggleAdminStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for JSON toggle, got %d. Body: %s", rec.Code, rec.Body.String())
	}
	var toggleResp struct {
		Status   string `json:"status"`
		IsActive bool   `json:"is_active"`
	}
	_ = json.NewDecoder(rec.Body).Decode(&toggleResp)
	if toggleResp.IsActive {
		t.Errorf("expected admin to be toggled to inactive")
	}

	// 6. POST /admins/{id}/toggle with HTMX
	req = httptest.NewRequest("POST", "/admins/"+createdAdmin.ID.String()+"/toggle", nil)
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", createdAdmin.ID.String())
	rec = httptest.NewRecorder()
	app.HandleToggleAdminStatus(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HTMX toggle, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "activated") {
		t.Errorf("expected activated banner in HTMX body, got: %s", rec.Body.String())
	}

	// 7. POST /admins/{id}/reset-password with JSON
	resetBody := map[string]string{
		"new_password":     "apipassword456",
		"confirm_password": "apipassword456",
	}
	resetBytes, _ := json.Marshal(resetBody)
	req = httptest.NewRequest("POST", "/admins/"+createdAdmin.ID.String()+"/reset-password", strings.NewReader(string(resetBytes)))
	req.Header.Set("Content-Type", "application/json")
	req.SetPathValue("id", createdAdmin.ID.String())
	rec = httptest.NewRecorder()
	app.HandleResetAdminPassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for JSON password reset, got %d. Body: %s", rec.Code, rec.Body.String())
	}
	refreshed, _ := store.GetUserByID(createdAdmin.ID)
	if !refreshed.CheckPassword("apipassword456") {
		t.Errorf("new JSON password verification failed")
	}

	// 8. POST /admins/{id}/reset-password with HTMX
	form = url.Values{}
	form.Set("new_password", "htmxnewpass123")
	form.Set("confirm_password", "htmxnewpass123")
	req = httptest.NewRequest("POST", "/admins/"+createdAdmin.ID.String()+"/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("HX-Request", "true")
	req.SetPathValue("id", createdAdmin.ID.String())
	rec = httptest.NewRecorder()
	app.HandleResetAdminPassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HTMX password reset, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "reset successfully") {
		t.Errorf("expected success banner in HTMX body, got: %s", rec.Body.String())
	}
}

func TestAdminHandler_RouteAliases(t *testing.T) {
	app, store := setupTestApp(t)
	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")

	withSession := func(req *http.Request, u *models.User) *http.Request {
		token := uuid.New().String()
		_ = store.CreateSessionToken(token, u.ID, time.Now().Add(time.Hour))
		req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
		return req
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admins", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /admin/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admins", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /admins", app.RequireRole(models.RoleOperationManager)(app.HandleAdmins))
	handler := app.AuthMiddleware(mux)

	// GET /admin redirects to /admins
	req := httptest.NewRequest("GET", "/admin", nil)
	req = withSession(req, managerUser)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || rec.Header().Get("Location") != "/admins" {
		t.Fatalf("expected 303 redirect to /admins, got code=%d loc=%s", rec.Code, rec.Header().Get("Location"))
	}
}


