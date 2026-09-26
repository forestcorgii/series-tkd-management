package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"series-tkd-management/internal/handlers"
	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

func setupTestApp(t *testing.T) (*handlers.AppHandler, repository.RepositoryStore) {
	t.Helper()
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}
	return app, store
}

func TestHandlePackages_View(t *testing.T) {
	app, _ := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/packages", nil)
	rec := httptest.NewRecorder()

	app.HandlePackages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Memberships") {
		t.Errorf("expected page to contain 'Memberships'")
	}
}

func TestHandleCreatePackageTemplate(t *testing.T) {
	app, store := setupTestApp(t)

	formData := url.Values{
		"title":         {"Advanced Black Belt Prep"},
		"description":   {"Specialized poomsae and board breaking pass"},
		"validity_days": {"120"},
		"price":         {"250.00"},
		"session_count": {"16"},
	}

	req := httptest.NewRequest(http.MethodPost, "/packages/templates", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.HandleCreatePackageTemplate(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect 303, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	templates, _ := store.GetPackageTemplates()
	var created *models.PackageTemplate
	for _, tpl := range templates {
		if tpl.Title == "Advanced Black Belt Prep" {
			created = tpl
			break
		}
	}
	if created == nil {
		t.Fatal("created template not found in store")
	}
	if created.Description != "Specialized poomsae and board breaking pass" {
		t.Errorf("unexpected description: %s", created.Description)
	}
	if created.ValidityDays != 120 || created.Price != 250.00 || created.SessionCount == nil || *created.SessionCount != 16 {
		t.Errorf("unexpected template data: %+v", created)
	}
}

func TestHandleUpdatePackageTemplate(t *testing.T) {
	app, store := setupTestApp(t)

	// Fetch an existing template
	templates, _ := store.GetPackageTemplates()
	if len(templates) == 0 {
		t.Fatal("expected seeded templates")
	}
	tpl := templates[0]

	formData := url.Values{
		"title":         {"Renamed Master Pass"},
		"description":   {"Updated description text"},
		"validity_days": {"150"},
		"price":         {"299.99"},
		"session_count": {"20"},
		"is_active":     {"1"},
	}

	req := httptest.NewRequest(http.MethodPost, "/packages/templates/"+tpl.ID.String(), strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.HandleUpdatePackageTemplate(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect 303, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	updated, err := store.GetPackageTemplateByID(tpl.ID)
	if err != nil {
		t.Fatalf("failed to fetch updated template: %v", err)
	}
	if updated.Title != "Renamed Master Pass" || updated.Price != 299.99 || *updated.SessionCount != 20 {
		t.Errorf("unexpected updated template: %+v", updated)
	}
}

func TestHandleTogglePackageTemplateStatus(t *testing.T) {
	app, store := setupTestApp(t)

	templates, _ := store.GetPackageTemplates()
	if len(templates) == 0 {
		t.Fatal("expected seeded templates")
	}
	tpl := templates[0]
	initialStatus := tpl.IsActive

	req := httptest.NewRequest(http.MethodPost, "/packages/templates/"+tpl.ID.String()+"/toggle", nil)
	rec := httptest.NewRecorder()

	app.HandleTogglePackageTemplateStatus(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect 303, got %d", rec.Code)
	}

	toggled, _ := store.GetPackageTemplateByID(tpl.ID)
	if toggled.IsActive == initialStatus {
		t.Fatalf("expected status to toggle from %v, got %v", initialStatus, toggled.IsActive)
	}
}

func TestHandleAssignPackage_WithCustomOverrides(t *testing.T) {
	app, store := setupTestApp(t)

	students, _ := store.GetAllStudents()
	templates, _ := store.GetPackageTemplates()
	if len(students) == 0 || len(templates) == 0 {
		t.Fatal("expected seeded students and templates")
	}
	student := students[0]
	tpl := templates[0]

	formData := url.Values{
		"student_id":             {student.ID.String()},
		"template_id":            {tpl.ID.String()},
		"override_sessions":      {"18"},
		"override_validity_days": {"100"},
		"custom_price":           {"135.50"},
		"payment_status":         {"paid"},
		"notes":                  {"Special promo 3 bonus classes"},
	}

	req := httptest.NewRequest(http.MethodPost, "/packages/assign", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.HandleAssignPackage(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect 303, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	pkgs, _ := store.GetStudentPackages(student.ID)
	var latest *models.StudentPackage
	for _, p := range pkgs {
		if p.TemplateID == tpl.ID && p.Notes == "Special promo 3 bonus classes" {
			latest = p
			break
		}
	}

	if latest == nil {
		t.Fatal("custom assigned package not found for student")
	}
	if latest.TotalSessions == nil || *latest.TotalSessions != 18 {
		t.Errorf("expected 18 total sessions, got %v", latest.TotalSessions)
	}
	if latest.CustomPrice == nil || *latest.CustomPrice != 135.50 {
		t.Errorf("expected 135.50 custom price, got %v", latest.CustomPrice)
	}
	if latest.Notes != "Special promo 3 bonus classes" {
		t.Errorf("expected custom notes, got %s", latest.Notes)
	}
	expectedExpiryMin := time.Now().AddDate(0, 0, 99)
	if latest.ExpiryDate.Before(expectedExpiryMin) {
		t.Errorf("expected expiry at least 99 days in future, got %v", latest.ExpiryDate)
	}
	_ = strconv.Itoa(0)
}
