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

func TestSettingsHandler_HandleSettings(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	token := uuid.New().String()
	_ = store.CreateSessionToken(token, managerUser.ID, time.Now().Add(time.Hour))

	req := httptest.NewRequest("GET", "/settings", nil)
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})

	rec := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager, models.RoleAdmin, models.RoleCoach)(app.HandleSettings)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HandleSettings, got %d", rec.Code)
	}

	body := rec.Body.String()
	if !strings.Contains(body, "System Settings") {
		t.Errorf("expected page to contain System Settings header")
	}
	if !strings.Contains(body, "Sparring") || !strings.Contains(body, "Poomsae") {
		t.Errorf("expected page to contain default categories")
	}
}

func TestSettingsHandler_CreateUpdateDeleteCategory(t *testing.T) {
	app, store := setupTestApp(t)

	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	token := uuid.New().String()
	_ = store.CreateSessionToken(token, managerUser.ID, time.Now().Add(time.Hour))

	// 1. Create Category
	form := url.Values{}
	form.Set("name", "Cadet Sparring")
	form.Set("color", "#059669")

	req := httptest.NewRequest("POST", "/settings/categories", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})

	rec := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager, models.RoleAdmin)(app.HandleCreateCategory)).ServeHTTP(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on create, got %d", rec.Code)
	}

	cat, err := store.GetTrainingCategoryByName("Cadet Sparring")
	if err != nil {
		t.Fatalf("expected created category in store: %v", err)
	}
	if cat.Color != "#059669" {
		t.Errorf("expected color #059669, got %s", cat.Color)
	}

	// 2. Duplicate Create should fail and redirect with error
	reqDup := httptest.NewRequest("POST", "/settings/categories", strings.NewReader(form.Encode()))
	reqDup.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqDup.AddCookie(&http.Cookie{Name: "stms_session", Value: token})

	recDup := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager, models.RoleAdmin)(app.HandleCreateCategory)).ServeHTTP(recDup, reqDup)

	loc := recDup.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected redirect with error for duplicate category, got %s", loc)
	}

	// 3. Update Category
	updateForm := url.Values{}
	updateForm.Set("name", "High Performance Cadet Sparring")
	updateForm.Set("color", "#047857")

	reqUpdate := httptest.NewRequest("POST", "/settings/categories/"+cat.ID.String()+"/edit", strings.NewReader(updateForm.Encode()))
	reqUpdate.SetPathValue("id", cat.ID.String())
	reqUpdate.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqUpdate.AddCookie(&http.Cookie{Name: "stms_session", Value: token})

	recUpdate := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager, models.RoleAdmin)(app.HandleUpdateCategory)).ServeHTTP(recUpdate, reqUpdate)

	if recUpdate.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on update, got %d", recUpdate.Code)
	}

	updated, err := store.GetTrainingCategoryByID(cat.ID)
	if err != nil || updated.Name != "High Performance Cadet Sparring" || updated.Color != "#047857" {
		t.Errorf("unexpected updated category: %+v, err: %v", updated, err)
	}

	// 4. Delete Category (when in use by session, must fail)
	sess := &models.TrainingSession{
		SessionDate:  time.Now(),
		StartTime:    "09:00",
		EndTime:      "10:00",
		TrainingType: models.TrainingType(updated.Name),
	}
	_ = store.CreateSession(sess)

	reqDelBlocked := httptest.NewRequest("POST", "/settings/categories/"+cat.ID.String()+"/delete", nil)
	reqDelBlocked.SetPathValue("id", cat.ID.String())
	reqDelBlocked.AddCookie(&http.Cookie{Name: "stms_session", Value: token})

	recDelBlocked := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager, models.RoleAdmin)(app.HandleDeleteCategory)).ServeHTTP(recDelBlocked, reqDelBlocked)

	delBlockedLoc := recDelBlocked.Header().Get("Location")
	if !strings.Contains(delBlockedLoc, "error=") {
		t.Errorf("expected error deleting category in use, got %s", delBlockedLoc)
	}

	// Remove session and delete should succeed
	_ = store.DeleteSession(sess.ID)

	reqDel := httptest.NewRequest("POST", "/settings/categories/"+cat.ID.String()+"/delete", nil)
	reqDel.SetPathValue("id", cat.ID.String())
	reqDel.AddCookie(&http.Cookie{Name: "stms_session", Value: token})

	recDel := httptest.NewRecorder()
	app.AuthMiddleware(app.RequireRole(models.RoleOperationManager, models.RoleAdmin)(app.HandleDeleteCategory)).ServeHTTP(recDel, reqDel)

	if recDel.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on delete, got %d", recDel.Code)
	}

	if _, err := store.GetTrainingCategoryByID(cat.ID); err == nil {
		t.Errorf("expected category to be deleted from store")
	}
}
