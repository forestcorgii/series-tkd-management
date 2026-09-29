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

func TestStudentHandler_HandleDeleteStudent_Success(t *testing.T) {
	app, store := setupTestApp(t)

	// Create student
	st := &models.Student{
		ID:                uuid.New(),
		FullName:          "Alex Morgan",
		DOB:               time.Now().AddDate(-12, 0, 0),
		Gender:            "Female",
		Phone:             "09112223344",
		CurrentBelt:       models.BeltWhite,
		LastPromotionDate: time.Now(),
		EmergencyName:     "Sarah Morgan",
		EmergencyPhone:    "09112223355",
		EmergencyRelation: "Mother",
		IsActive:          true,
		CreatedAt:         time.Now(),
	}
	if err := store.CreateStudent(st); err != nil {
		t.Fatalf("failed to create student: %v", err)
	}

	// Request deletion
	req := httptest.NewRequest(http.MethodPost, "/students/"+st.ID.String()+"/delete", nil)
	req.SetPathValue("id", st.ID.String())
	rec := httptest.NewRecorder()

	app.HandleDeleteStudent(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/students?success=") || !strings.Contains(loc, "deleted") {
		t.Errorf("expected success redirect notice, got %s", loc)
	}

	// Verify student no longer exists
	if _, err := store.GetStudentByID(st.ID); err == nil {
		t.Errorf("expected student to be deleted from store")
	}
}

func TestStudentHandler_HandleDeleteStudent_NotFound(t *testing.T) {
	app, _ := setupTestApp(t)

	fakeID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/students/"+fakeID.String()+"/delete", nil)
	req.SetPathValue("id", fakeID.String())
	rec := httptest.NewRecorder()

	app.HandleDeleteStudent(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "/students?error=") {
		t.Errorf("expected error redirect notice, got %s", loc)
	}
}

func TestStudentHandler_HandleDeleteStudent_JSON(t *testing.T) {
	app, store := setupTestApp(t)

	st := &models.Student{
		ID:                uuid.New(),
		FullName:          "Lucas Scott",
		DOB:               time.Now().AddDate(-14, 0, 0),
		Gender:            "Male",
		Phone:             "09223334455",
		CurrentBelt:       models.BeltLowYellow,
		LastPromotionDate: time.Now(),
		EmergencyName:     "Karen Roe",
		EmergencyPhone:    "09223334466",
		EmergencyRelation: "Mother",
		IsActive:          true,
		CreatedAt:         time.Now(),
	}
	_ = store.CreateStudent(st)

	req := httptest.NewRequest(http.MethodDelete, "/api/students/"+st.ID.String(), nil)
	req.SetPathValue("id", st.ID.String())
	req.Header.Set("Accept", "application/json")
	rec := httptest.NewRecorder()

	app.HandleDeleteStudent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for JSON deletion, got %d", rec.Code)
	}

	var resp map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if resp["success"] != true {
		t.Errorf("expected success: true, got %v", resp["success"])
	}
}

func TestStudentHandler_HandleDeleteStudent_HTMX(t *testing.T) {
	app, store := setupTestApp(t)

	st := &models.Student{
		ID:                uuid.New(),
		FullName:          "Brooke Davis",
		DOB:               time.Now().AddDate(-16, 0, 0),
		Gender:            "Female",
		Phone:             "09334445566",
		CurrentBelt:       models.BeltHighYellow,
		LastPromotionDate: time.Now(),
		EmergencyName:     "Victoria Davis",
		EmergencyPhone:    "09334445577",
		EmergencyRelation: "Mother",
		IsActive:          true,
		CreatedAt:         time.Now(),
	}
	_ = store.CreateStudent(st)

	req := httptest.NewRequest(http.MethodPost, "/students/"+st.ID.String()+"/delete", nil)
	req.SetPathValue("id", st.ID.String())
	req.Header.Set("HX-Request", "true")
	rec := httptest.NewRecorder()

	app.HandleDeleteStudent(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for HTMX deletion, got %d", rec.Code)
	}
	hxRedirect := rec.Header().Get("HX-Redirect")
	if !strings.Contains(hxRedirect, "/students?success=") {
		t.Errorf("expected HX-Redirect header with success notice, got %s", hxRedirect)
	}
}

func TestStudentHandler_HandleDeleteStudent_RoleGuarded(t *testing.T) {
	app, store := setupTestApp(t)

	adminUser, _ := store.GetUserByEmail("admin@seriestkd.com")
	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	coachUser, _ := store.GetUserByEmail("jiwoo.park@seriestkd.com")

	handler := app.AuthMiddleware(app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleDeleteStudent))

	// 1. Admin is permitted to delete student
	st1 := &models.Student{
		ID:          uuid.New(),
		FullName:    "Student One",
		DOB:         time.Now().AddDate(-10, 0, 0),
		CurrentBelt: models.BeltWhite,
		IsActive:    true,
	}
	_ = store.CreateStudent(st1)

	adminToken := "tok_admin_" + uuid.New().String()
	_ = store.CreateSessionToken(adminToken, adminUser.ID, time.Now().Add(time.Hour))

	reqAdmin := httptest.NewRequest(http.MethodPost, "/students/"+st1.ID.String()+"/delete", nil)
	reqAdmin.SetPathValue("id", st1.ID.String())
	reqAdmin.AddCookie(&http.Cookie{Name: "stms_session", Value: adminToken})
	recAdmin := httptest.NewRecorder()
	handler.ServeHTTP(recAdmin, reqAdmin)

	if recAdmin.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 for admin delete, got %d", recAdmin.Code)
	}

	// 2. Manager is permitted to delete student
	st2 := &models.Student{
		ID:          uuid.New(),
		FullName:    "Student Two",
		DOB:         time.Now().AddDate(-10, 0, 0),
		CurrentBelt: models.BeltWhite,
		IsActive:    true,
	}
	_ = store.CreateStudent(st2)

	mgrToken := "tok_mgr_" + uuid.New().String()
	_ = store.CreateSessionToken(mgrToken, managerUser.ID, time.Now().Add(time.Hour))

	reqMgr := httptest.NewRequest(http.MethodPost, "/students/"+st2.ID.String()+"/delete", nil)
	reqMgr.SetPathValue("id", st2.ID.String())
	reqMgr.AddCookie(&http.Cookie{Name: "stms_session", Value: mgrToken})
	recMgr := httptest.NewRecorder()
	handler.ServeHTTP(recMgr, reqMgr)

	if recMgr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 for manager delete, got %d", recMgr.Code)
	}

	// 3. Coach is FORBIDDEN from deleting student
	st3 := &models.Student{
		ID:          uuid.New(),
		FullName:    "Student Three",
		DOB:         time.Now().AddDate(-10, 0, 0),
		CurrentBelt: models.BeltWhite,
		IsActive:    true,
	}
	_ = store.CreateStudent(st3)

	coachToken := "tok_coach_" + uuid.New().String()
	_ = store.CreateSessionToken(coachToken, coachUser.ID, time.Now().Add(time.Hour))

	reqCoach := httptest.NewRequest(http.MethodPost, "/students/"+st3.ID.String()+"/delete", nil)
	reqCoach.SetPathValue("id", st3.ID.String())
	reqCoach.AddCookie(&http.Cookie{Name: "stms_session", Value: coachToken})
	recCoach := httptest.NewRecorder()
	handler.ServeHTTP(recCoach, reqCoach)

	// Unauthorized role redirects to their authorized portal or gives 403
	if recCoach.Code == http.StatusSeeOther && recCoach.Header().Get("Location") == "/students?success=..." {
		t.Fatalf("coach should NOT be allowed to delete student")
	}
	// Verify student 3 still exists
	if _, err := store.GetStudentByID(st3.ID); err != nil {
		t.Errorf("coach should not have deleted student 3")
	}
}

func TestStudentHandler_HandleCreateStudent_WithMembershipPackage(t *testing.T) {
	app, store := setupTestApp(t)

	// Create a package template
	sessCount := 12
	tpl := &models.PackageTemplate{
		ID:           uuid.New(),
		Title:        "12-Class Sparring Pass",
		SessionCount: &sessCount,
		ValidityDays: 60,
		Price:        2500.00,
		PlanType:     models.PlanTypeStandard,
		IsActive:     true,
	}
	if err := store.CreatePackageTemplate(tpl); err != nil {
		t.Fatalf("failed to create template: %v", err)
	}

	form := url.Values{}
	form.Set("full_name", "Daniel LaRusso")
	form.Set("dob", "2010-05-15")
	form.Set("gender", "Male")
	form.Set("phone", "09123456789")
	form.Set("current_belt", string(models.BeltWhite))
	form.Set("emergency_name", "Lucille LaRusso")
	form.Set("emergency_phone", "09123456780")
	form.Set("emergency_relation", "Mother")
	form.Set("template_id", tpl.ID.String())
	form.Set("payment_status", "paid")
	form.Set("custom_price", "2300.00")
	form.Set("package_notes", "Summer promotion discount")

	req := httptest.NewRequest(http.MethodPost, "/students", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.HandleCreateStudent(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if loc != "/students" {
		t.Errorf("expected redirect to /students, got %s", loc)
	}

	// Verify student was created
	students, err := store.GetAllStudents()
	if err != nil || len(students) == 0 {
		t.Fatalf("expected student to be created in store")
	}
	var createdStudent *models.Student
	for _, s := range students {
		if s.FullName == "Daniel LaRusso" {
			createdStudent = s
			break
		}
	}
	if createdStudent == nil {
		t.Fatalf("expected to find student Daniel LaRusso")
	}

	// Verify membership package was assigned to createdStudent
	pkgs, err := store.GetStudentPackages(createdStudent.ID)
	if err != nil {
		t.Fatalf("failed to retrieve student packages: %v", err)
	}
	if len(pkgs) != 1 {
		t.Fatalf("expected 1 assigned package, got %d", len(pkgs))
	}
	assigned := pkgs[0]
	if assigned.TemplateID != tpl.ID {
		t.Errorf("expected template ID %v, got %v", tpl.ID, assigned.TemplateID)
	}
	if assigned.RemainingSessions == nil || *assigned.RemainingSessions != 12 {
		t.Errorf("expected 12 remaining sessions, got %v", assigned.RemainingSessions)
	}
	if assigned.CustomPrice == nil || *assigned.CustomPrice != 2300.00 {
		t.Errorf("expected custom price 2300.00, got %v", assigned.CustomPrice)
	}
	if assigned.Notes != "Summer promotion discount" {
		t.Errorf("expected notes 'Summer promotion discount', got %q", assigned.Notes)
	}
	if assigned.PaymentStatus != "paid" {
		t.Errorf("expected payment status 'paid', got %q", assigned.PaymentStatus)
	}
}

func TestStudentHandler_HandleCreateStudent_WithoutPackage(t *testing.T) {
	app, store := setupTestApp(t)

	form := url.Values{}
	form.Set("full_name", "Johnny Lawrence")
	form.Set("dob", "2010-06-20")
	form.Set("gender", "Male")
	form.Set("phone", "09987654321")
	form.Set("current_belt", string(models.BeltWhite))
	form.Set("emergency_name", "Laura Lawrence")
	form.Set("emergency_phone", "09987654320")
	form.Set("emergency_relation", "Mother")
	form.Set("template_id", "")

	req := httptest.NewRequest(http.MethodPost, "/students", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.HandleCreateStudent(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}

	students, _ := store.GetAllStudents()
	var createdStudent *models.Student
	for _, s := range students {
		if s.FullName == "Johnny Lawrence" {
			createdStudent = s
			break
		}
	}
	if createdStudent == nil {
		t.Fatalf("expected to find student Johnny Lawrence")
	}

	pkgs, _ := store.GetStudentPackages(createdStudent.ID)
	if len(pkgs) != 0 {
		t.Errorf("expected 0 packages assigned, got %d", len(pkgs))
	}
}

