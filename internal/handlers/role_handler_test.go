package handlers_test

import (
	"bytes"
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

func TestRoleHandler_UpdateRoleNameAndSlug(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, err := store.GetUserByEmail("manager@seriestkd.com")
	if err != nil {
		t.Fatalf("failed to find manager user: %v", err)
	}
	token := uuid.New().String()
	_ = store.CreateSessionToken(token, managerUser.ID, time.Now().Add(time.Hour))

	// Find the system Coach role
	coachRole, err := store.GetRoleBySlug("coach")
	if err != nil {
		t.Fatalf("expected system coach role: %v", err)
	}

	// 1. Update system role name and slug via Form POST
	form := url.Values{}
	form.Set("name", "Senior Head Instructor")
	form.Set("slug", "head-instructor")
	form.Set("description", "Updated coaching responsibilities")
	form.Add("permissions", models.PermSessionsRead)
	form.Add("permissions", models.PermAttendanceCheckin)

	req := httptest.NewRequest("POST", "/roles/"+coachRole.ID.String()+"/edit", strings.NewReader(form.Encode()))
	req.SetPathValue("id", coachRole.ID.String())
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})

	rec := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager)(app.HandleUpdateRole)).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect (303), got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify updated in store
	updatedCoach, err := store.GetRoleByID(coachRole.ID)
	if err != nil {
		t.Fatalf("failed to get updated coach role: %v", err)
	}
	if updatedCoach.Name != "Senior Head Instructor" {
		t.Errorf("expected role name 'Senior Head Instructor', got %q", updatedCoach.Name)
	}
	if updatedCoach.Slug != "head-instructor" {
		t.Errorf("expected role slug 'head-instructor', got %q", updatedCoach.Slug)
	}

	// 2. Update role name and slug via JSON API
	payload := map[string]interface{}{
		"name":        "Master Instructor",
		"slug":        "master-coach",
		"description": "Master floor lead",
		"permissions": []string{models.PermSessionsRead, models.PermEvaluationsCreate},
	}
	jsonBody, _ := json.Marshal(payload)
	reqJSON := httptest.NewRequest("POST", "/roles/"+coachRole.ID.String()+"/edit", bytes.NewReader(jsonBody))
	reqJSON.SetPathValue("id", coachRole.ID.String())
	reqJSON.Header.Set("Content-Type", "application/json")
	reqJSON.AddCookie(&http.Cookie{Name: "stms_session", Value: token})

	recJSON := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager)(app.HandleUpdateRole)).ServeHTTP(recJSON, reqJSON)

	if recJSON.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for JSON role update, got %d: %s", recJSON.Code, recJSON.Body.String())
	}

	updatedAgain, _ := store.GetRoleByID(coachRole.ID)
	if updatedAgain.Name != "Master Instructor" {
		t.Errorf("expected role name 'Master Instructor', got %q", updatedAgain.Name)
	}
	if updatedAgain.Slug != "master-coach" {
		t.Errorf("expected role slug 'master-coach', got %q", updatedAgain.Slug)
	}

	// 3. Prevent duplicate slug
	adminRole, err := store.GetRoleBySlug("admin")
	if err != nil {
		t.Fatalf("expected admin role: %v", err)
	}

	dupPayload := map[string]interface{}{
		"name": "Clashing Admin",
		"slug": "master-coach", // Already taken by updatedCoach
	}
	dupJSON, _ := json.Marshal(dupPayload)
	reqDup := httptest.NewRequest("POST", "/roles/"+adminRole.ID.String()+"/edit", bytes.NewReader(dupJSON))
	reqDup.SetPathValue("id", adminRole.ID.String())
	reqDup.Header.Set("Content-Type", "application/json")
	reqDup.AddCookie(&http.Cookie{Name: "stms_session", Value: token})

	recDup := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager)(app.HandleUpdateRole)).ServeHTTP(recDup, reqDup)

	if recDup.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request on duplicate slug, got %d", recDup.Code)
	}
}

func TestRoleHandler_ListAndCreateRole(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, err := store.GetUserByEmail("manager@seriestkd.com")
	if err != nil {
		t.Fatalf("failed to find manager user: %v", err)
	}
	token := uuid.New().String()
	_ = store.CreateSessionToken(token, managerUser.ID, time.Now().Add(time.Hour))

	// 1. List roles
	req := httptest.NewRequest("GET", "/roles", nil)
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	rec := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager)(app.HandleRoles)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HandleRoles, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Governance") || !strings.Contains(body, "Create Custom Role") {
		t.Errorf("expected page content in response")
	}

	// 2. Create custom role
	form := url.Values{}
	form.Set("name", "Desk Assistant")
	form.Set("slug", "desk-assistant")
	form.Set("description", "Front desk welcoming staff")
	form.Add("permissions", models.PermStudentsRead)

	reqCreate := httptest.NewRequest("POST", "/roles", strings.NewReader(form.Encode()))
	reqCreate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqCreate.AddCookie(&http.Cookie{Name: "stms_session", Value: token})

	recCreate := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager)(app.HandleCreateRole)).ServeHTTP(recCreate, reqCreate)

	if recCreate.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect after create, got %d", recCreate.Code)
	}

	created, err := store.GetRoleBySlug("desk-assistant")
	if err != nil {
		t.Fatalf("expected custom role to be created: %v", err)
	}
	if created.Name != "Desk Assistant" {
		t.Errorf("expected name 'Desk Assistant', got %q", created.Name)
	}
}
