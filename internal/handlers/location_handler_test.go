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

func TestLocationHandler_HandleLocations_AccessAndDisplay(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")

	// 1. Manager Access
	req := httptest.NewRequest("GET", "/locations", nil)
	token := uuid.New().String()
	_ = store.CreateSessionToken(token, managerUser.ID, time.Now().Add(time.Hour))
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})

	rec := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager, models.RoleAdmin)(app.HandleLocations)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HandleLocations, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "Training Locations &amp; Branch Dojangs") && !strings.Contains(body, "Training Locations & Branch Dojangs") {
		t.Errorf("expected page to contain Locations header")
	}
	if !strings.Contains(body, "Makati Central Dojang") {
		t.Errorf("expected page to contain seeded Makati Central Dojang")
	}
	if !strings.Contains(body, "BGC High Street Training Hall") {
		t.Errorf("expected page to contain seeded BGC High Street Training Hall")
	}
}

func TestLocationHandler_CreateUpdateDelete(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	token := uuid.New().String()
	_ = store.CreateSessionToken(token, managerUser.ID, time.Now().Add(time.Hour))

	// 1. Create Location
	form := url.Values{}
	form.Set("name", "Ortigas Training Center")
	form.Set("pin", "https://maps.google.com/?q=Ortigas+Center")
	form.Set("fixed_rate", "300.00")

	req := httptest.NewRequest("POST", "/locations", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	rec := httptest.NewRecorder()

	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager, models.RoleAdmin)(app.HandleCreateLocation)).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 SeeOther redirect after create, got %d", rec.Code)
	}
	locHeader := rec.Header().Get("Location")
	if !strings.Contains(locHeader, "success=") {
		t.Errorf("expected success param in redirect URL, got %s", locHeader)
	}

	// Verify in store
	locs, _ := store.GetAllLocations()
	var createdLoc *models.Location
	for _, l := range locs {
		if l.Name == "Ortigas Training Center" {
			createdLoc = l
			break
		}
	}
	if createdLoc == nil {
		t.Fatalf("expected to find created location in store")
	}
	if createdLoc.Pin != "https://maps.google.com/?q=Ortigas+Center" {
		t.Errorf("expected pin to match, got %s", createdLoc.Pin)
	}
	if createdLoc.FixedRate == nil || *createdLoc.FixedRate != 300.00 {
		t.Errorf("expected fixed_rate to be 300.00, got %v", createdLoc.FixedRate)
	}

	// 2. Update Location
	editForm := url.Values{}
	editForm.Set("name", "Ortigas Grand Dojang")
	editForm.Set("pin", "https://maps.google.com/?q=Ortigas+Grand")
	editForm.Set("fixed_rate", "375.50")

	reqEdit := httptest.NewRequest("POST", "/locations/"+createdLoc.ID.String()+"/edit", strings.NewReader(editForm.Encode()))
	reqEdit.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqEdit.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recEdit := httptest.NewRecorder()

	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager, models.RoleAdmin)(app.HandleUpdateLocation)).ServeHTTP(recEdit, reqEdit)

	if recEdit.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 SeeOther redirect after update, got %d", recEdit.Code)
	}

	updated, _ := store.GetLocationByID(createdLoc.ID)
	if updated.Name != "Ortigas Grand Dojang" {
		t.Errorf("expected updated name 'Ortigas Grand Dojang', got %s", updated.Name)
	}
	if updated.FixedRate == nil || *updated.FixedRate != 375.50 {
		t.Errorf("expected updated fixed_rate to be 375.50, got %v", updated.FixedRate)
	}

	// 3. Delete Location
	reqDel := httptest.NewRequest("POST", "/locations/"+createdLoc.ID.String()+"/delete", nil)
	reqDel.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recDel := httptest.NewRecorder()

	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager, models.RoleAdmin)(app.HandleDeleteLocation)).ServeHTTP(recDel, reqDel)

	if recDel.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 SeeOther redirect after delete, got %d", recDel.Code)
	}

	// Verify deletion
	_, err := store.GetLocationByID(createdLoc.ID)
	if err == nil {
		t.Errorf("expected location to be deleted")
	}
}

func TestLocationHandler_ValidationAndRBAC(t *testing.T) {
	app, store := setupTestApp(t)

	// Unauthenticated request should redirect to /login
	req := httptest.NewRequest("GET", "/locations", nil)
	rec := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager)(app.HandleLocations)).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther || !strings.Contains(rec.Header().Get("Location"), "/login") {
		t.Errorf("expected unauthenticated request to redirect to /login, got code %d to %s", rec.Code, rec.Header().Get("Location"))
	}

	// Empty name creation should fail
	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	token := uuid.New().String()
	_ = store.CreateSessionToken(token, managerUser.ID, time.Now().Add(time.Hour))

	emptyForm := url.Values{}
	emptyForm.Set("name", "")
	reqEmpty := httptest.NewRequest("POST", "/locations", strings.NewReader(emptyForm.Encode()))
	reqEmpty.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqEmpty.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recEmpty := httptest.NewRecorder()

	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager)(app.HandleCreateLocation)).ServeHTTP(recEmpty, reqEmpty)

	if recEmpty.Code != http.StatusSeeOther || !strings.Contains(recEmpty.Header().Get("Location"), "error=") {
		t.Errorf("expected error redirect for empty name, got code %d to %s", recEmpty.Code, recEmpty.Header().Get("Location"))
	}

	// Admin Access (Allowed)
	adminUser, _ := store.GetUserByEmail("admin@seriestkd.com")
	adminToken := uuid.New().String()
	_ = store.CreateSessionToken(adminToken, adminUser.ID, time.Now().Add(time.Hour))
	reqAdmin := httptest.NewRequest("GET", "/locations", nil)
	reqAdmin.AddCookie(&http.Cookie{Name: "stms_session", Value: adminToken})
	recAdmin := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleLocations)).ServeHTTP(recAdmin, reqAdmin)
	if recAdmin.Code != http.StatusOK {
		t.Errorf("expected 200 OK for Admin role on /locations, got %d", recAdmin.Code)
	}

	// Coach Access (Browser request redirects to Coach default portal /sessions)
	coachUser, _ := store.GetUserByEmail("jiwoo.park@seriestkd.com")
	coachToken := uuid.New().String()
	_ = store.CreateSessionToken(coachToken, coachUser.ID, time.Now().Add(time.Hour))
	reqCoach := httptest.NewRequest("GET", "/locations", nil)
	reqCoach.AddCookie(&http.Cookie{Name: "stms_session", Value: coachToken})
	recCoach := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleLocations)).ServeHTTP(recCoach, reqCoach)
	if recCoach.Code != http.StatusSeeOther || !strings.Contains(recCoach.Header().Get("Location"), "/sessions") {
		t.Errorf("expected 303 redirect to /sessions for Coach role on /locations, got code %d to %s", recCoach.Code, recCoach.Header().Get("Location"))
	}

	// Coach Access via HTMX/API (Returns 403 Forbidden)
	reqCoachAPI := httptest.NewRequest("GET", "/locations", nil)
	reqCoachAPI.Header.Set("HX-Request", "true")
	reqCoachAPI.AddCookie(&http.Cookie{Name: "stms_session", Value: coachToken})
	recCoachAPI := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleLocations)).ServeHTTP(recCoachAPI, reqCoachAPI)
	if recCoachAPI.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for Coach role on HTMX /locations, got %d", recCoachAPI.Code)
	}

	// Student Access (Browser request redirects to Student portal /portal/student)
	studentUser, _ := store.GetUserByEmail("alex.vance@seriestkd.com")
	studentToken := uuid.New().String()
	_ = store.CreateSessionToken(studentToken, studentUser.ID, time.Now().Add(time.Hour))
	reqStudent := httptest.NewRequest("GET", "/locations", nil)
	reqStudent.AddCookie(&http.Cookie{Name: "stms_session", Value: studentToken})
	recStudent := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleLocations)).ServeHTTP(recStudent, reqStudent)
	if recStudent.Code != http.StatusSeeOther || !strings.Contains(recStudent.Header().Get("Location"), "/portal/student") {
		t.Errorf("expected 303 redirect to /portal/student for Student role on /locations, got code %d to %s", recStudent.Code, recStudent.Header().Get("Location"))
	}

	// Student Access via HTMX/API (Returns 403 Forbidden)
	reqStudentAPI := httptest.NewRequest("GET", "/locations", nil)
	reqStudentAPI.Header.Set("HX-Request", "true")
	reqStudentAPI.AddCookie(&http.Cookie{Name: "stms_session", Value: studentToken})
	recStudentAPI := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleLocations)).ServeHTTP(recStudentAPI, reqStudentAPI)
	if recStudentAPI.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for Student role on HTMX /locations, got %d", recStudentAPI.Code)
	}
}

func TestLocationHandler_RESTMethodVariations(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	token := uuid.New().String()
	_ = store.CreateSessionToken(token, managerUser.ID, time.Now().Add(time.Hour))

	// Create location first
	loc := &models.Location{
		ID:        uuid.New(),
		Name:      "REST Test Branch",
		Pin:       "https://maps.google.com/?q=Test",
		CreatedAt: time.Now(),
	}
	_ = store.CreateLocation(loc)

	// 1. Update via POST /locations/{id} (without /edit)
	editForm := url.Values{}
	editForm.Set("name", "REST Updated Branch")
	editForm.Set("pin", "https://maps.google.com/?q=Updated")

	reqPost := httptest.NewRequest("POST", "/locations/"+loc.ID.String(), strings.NewReader(editForm.Encode()))
	reqPost.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqPost.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recPost := httptest.NewRecorder()

	app.AuthMiddleware(app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleUpdateLocation)).ServeHTTP(recPost, reqPost)
	if recPost.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 SeeOther redirect after POST update, got %d", recPost.Code)
	}

	updated, _ := store.GetLocationByID(loc.ID)
	if updated.Name != "REST Updated Branch" {
		t.Errorf("expected name 'REST Updated Branch', got %s", updated.Name)
	}

	// 2. Update via PUT /locations/{id}
	putForm := url.Values{}
	putForm.Set("name", "PUT Updated Branch")
	putForm.Set("pin", "https://maps.google.com/?q=PUT")

	reqPut := httptest.NewRequest("PUT", "/locations/"+loc.ID.String(), strings.NewReader(putForm.Encode()))
	reqPut.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqPut.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recPut := httptest.NewRecorder()

	app.AuthMiddleware(app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleUpdateLocation)).ServeHTTP(recPut, reqPut)
	if recPut.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 SeeOther redirect after PUT update, got %d", recPut.Code)
	}

	putUpdated, _ := store.GetLocationByID(loc.ID)
	if putUpdated.Name != "PUT Updated Branch" {
		t.Errorf("expected name 'PUT Updated Branch', got %s", putUpdated.Name)
	}

	// 3. Delete via DELETE /locations/{id}
	reqDel := httptest.NewRequest("DELETE", "/locations/"+loc.ID.String(), nil)
	reqDel.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
	recDel := httptest.NewRecorder()

	app.AuthMiddleware(app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleDeleteLocation)).ServeHTTP(recDel, reqDel)
	if recDel.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 SeeOther redirect after DELETE, got %d", recDel.Code)
	}

	_, err := store.GetLocationByID(loc.ID)
	if err == nil {
		t.Errorf("expected location to be deleted")
	}
}

