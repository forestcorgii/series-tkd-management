package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/handlers"
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

	handler := app.AuthMiddleware(app.RequireRole(models.RoleOperationManager)(app.HandleDeleteStudent))

	// 1. Admin is NOT permitted to delete student
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

	// Admin must be rejected (403 Forbidden or redirected away)
	if recAdmin.Code == http.StatusSeeOther && recAdmin.Header().Get("Location") == "/students" {
		t.Fatalf("admin should NOT be allowed to delete student")
	}
	if found, _ := store.GetStudentByID(st1.ID); found == nil {
		t.Fatalf("expected student1 to still exist after admin delete attempt")
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

func TestStudentHandler_HandleCreateStudent_WithUsername(t *testing.T) {
	app, store := setupTestApp(t)

	form := url.Values{}
	form.Set("full_name", "Daniel LaRusso")
	form.Set("dob", "2010-06-20")
	form.Set("gender", "Male")
	form.Set("phone", "09987654321")
	form.Set("current_belt", string(models.BeltWhite))
	form.Set("emergency_name", "Lucille LaRusso")
	form.Set("emergency_phone", "09987654320")
	form.Set("emergency_relation", "Mother")
	form.Set("email", "daniel@seriestkd.com")
	form.Set("password", "karate123")
	form.Set("username", "daniel.larusso")

	req := httptest.NewRequest(http.MethodPost, "/students", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.HandleCreateStudent(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}

	u, err := store.GetUserByEmail("daniel@seriestkd.com")
	if err != nil || u == nil {
		t.Fatalf("expected user created for student, got: %v", err)
	}
	if u.Username != "daniel.larusso" {
		t.Errorf("expected username 'daniel.larusso', got '%s'", u.Username)
	}

	byUname, err := store.GetUserByUsername("daniel.larusso")
	if err != nil || byUname.ID != u.ID {
		t.Errorf("expected GetUserByUsername to find created student user, got: %v", err)
	}
}

func TestStudentHandler_HandleStudents_BoostedNavigation(t *testing.T) {
	app, _ := setupTestApp(t)

	// Boosted page navigation sends both HX-Request and HX-Boosted
	req := httptest.NewRequest(http.MethodGet, "/students", nil)
	req.Header.Set("HX-Request", "true")
	req.Header.Set("HX-Boosted", "true")
	rec := httptest.NewRecorder()

	app.HandleStudents(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for boosted navigation, got %d", rec.Code)
	}
	body := rec.Body.String()
	// Must contain the full page shell with navigation header
	if !strings.Contains(body, "SERIES TAEKWONDO") {
		t.Errorf("expected full layout with navbar header in boosted response, got partial without it")
	}
	if !strings.Contains(body, "<header") {
		t.Errorf("expected <header> tag in boosted response")
	}

	// Normal HTMX filter/search request without HX-Boosted returns partial
	reqPartial := httptest.NewRequest(http.MethodGet, "/students?query=Carlos", nil)
	reqPartial.Header.Set("HX-Request", "true")
	recPartial := httptest.NewRecorder()

	app.HandleStudents(recPartial, reqPartial)
	if recPartial.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for partial search, got %d", recPartial.Code)
	}
	partialBody := recPartial.Body.String()
	if strings.Contains(partialBody, "<!DOCTYPE html>") || strings.Contains(partialBody, "<header") {
		t.Errorf("expected partial response without layout shell for search request")
	}
}

func TestStudentHandler_ManagerUpdateStudentCredentials(t *testing.T) {
	app, store := setupTestApp(t)

	// Create student
	st := &models.Student{
		ID:                uuid.New(),
		FullName:          "Johnny Lawrence",
		DOB:               time.Now().AddDate(-16, 0, 0),
		Gender:            "Male",
		Phone:             "09112224455",
		CurrentBelt:       models.BeltLowYellow,
		LastPromotionDate: time.Now(),
		EmergencyName:     "Laura Lawrence",
		EmergencyPhone:    "09112224466",
		EmergencyRelation: "Mother",
		IsActive:          true,
	}
	if err := store.CreateStudent(st); err != nil {
		t.Fatalf("failed to create student: %v", err)
	}

	// Create linked user
	u := &models.User{
		ID:        uuid.New(),
		Email:     "johnny.old@seriestkd.com",
		Username:  "johnny.old",
		Role:      models.RoleStudent,
		StudentID: &st.ID,
		IsActive:  true,
	}
	_ = u.SetPassword("student123")
	if err := store.CreateUser(u); err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Another user to test uniqueness collisions
	otherUser := &models.User{
		ID:       uuid.New(),
		Email:    "taken@seriestkd.com",
		Username: "takenhandle",
		Role:     models.RoleStudent,
		IsActive: true,
	}
	_ = store.CreateUser(otherUser)

	mgr := &models.User{
		ID:       uuid.New(),
		Email:    "manager@seriestkd.com",
		Role:     models.RoleOperationManager,
		IsActive: true,
	}

	// 1. Manager successfully updates email and username
	form := url.Values{}
	form.Set("full_name", "Johnny Lawrence")
	form.Set("dob", "2008-01-01")
	form.Set("gender", "Male")
	form.Set("phone", "09112224455")
	form.Set("emergency_name", "Laura Lawrence")
	form.Set("emergency_phone", "09112224466")
	form.Set("emergency_relation", "Mother")
	form.Set("email", "johnny.updated@seriestkd.com")
	form.Set("username", "johnny.updated")

	req := httptest.NewRequest(http.MethodPost, "/students/"+st.ID.String(), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", st.ID.String())
	ctx := context.WithValue(req.Context(), handlers.UserContextKey, mgr)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	app.HandleUpdateStudent(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}

	updatedUser, err := store.GetUserByStudentID(st.ID)
	if err != nil {
		t.Fatalf("expected to find student user, got: %v", err)
	}
	if updatedUser.Email != "johnny.updated@seriestkd.com" {
		t.Errorf("expected email 'johnny.updated@seriestkd.com', got '%s'", updatedUser.Email)
	}
	if updatedUser.Username != "johnny.updated" {
		t.Errorf("expected username 'johnny.updated', got '%s'", updatedUser.Username)
	}

	// 2. Reject duplicate email
	formDupEmail := url.Values{}
	formDupEmail.Set("full_name", "Johnny Lawrence")
	formDupEmail.Set("email", "taken@seriestkd.com")
	reqDup := httptest.NewRequest(http.MethodPost, "/students/"+st.ID.String(), strings.NewReader(formDupEmail.Encode()))
	reqDup.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqDup.SetPathValue("id", st.ID.String())
	reqDup = reqDup.WithContext(ctx)
	recDup := httptest.NewRecorder()

	app.HandleUpdateStudent(recDup, reqDup)
	if !strings.Contains(recDup.Header().Get("Location"), "error=") {
		t.Errorf("expected error redirect on duplicate email, got %s", recDup.Header().Get("Location"))
	}

	// 3. Reject duplicate username
	formDupUname := url.Values{}
	formDupUname.Set("full_name", "Johnny Lawrence")
	formDupUname.Set("username", "takenhandle")
	reqDupU := httptest.NewRequest(http.MethodPost, "/students/"+st.ID.String(), strings.NewReader(formDupUname.Encode()))
	reqDupU.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	reqDupU.SetPathValue("id", st.ID.String())
	reqDupU = reqDupU.WithContext(ctx)
	recDupU := httptest.NewRecorder()

	app.HandleUpdateStudent(recDupU, reqDupU)
	if !strings.Contains(recDupU.Header().Get("Location"), "error=") {
		t.Errorf("expected error redirect on duplicate username, got %s", recDupU.Header().Get("Location"))
	}
}

func TestStudentHandler_AdminCannotUpdateStudentCredentials(t *testing.T) {
	app, store := setupTestApp(t)

	st := &models.Student{
		ID:                uuid.New(),
		FullName:          "Miguel Diaz",
		DOB:               time.Now().AddDate(-15, 0, 0),
		Gender:            "Male",
		Phone:             "09112227788",
		CurrentBelt:       models.BeltWhite,
		LastPromotionDate: time.Now(),
		EmergencyName:     "Carmen Diaz",
		EmergencyPhone:    "09112227799",
		EmergencyRelation: "Mother",
		IsActive:          true,
	}
	_ = store.CreateStudent(st)

	u := &models.User{
		ID:        uuid.New(),
		Email:     "miguel.original@seriestkd.com",
		Username:  "miguel.orig",
		Role:      models.RoleStudent,
		StudentID: &st.ID,
		IsActive:  true,
	}
	_ = store.CreateUser(u)

	// Admin staff attempting to modify email or username
	admin := &models.User{
		ID:       uuid.New(),
		Email:    "admin@seriestkd.com",
		Role:     models.RoleAdmin,
		IsActive: true,
	}

	form := url.Values{}
	form.Set("full_name", "Miguel Diaz")
	form.Set("email", "hacked@seriestkd.com")
	form.Set("username", "hacked.handle")

	req := httptest.NewRequest(http.MethodPost, "/students/"+st.ID.String(), strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", st.ID.String())
	ctx := context.WithValue(req.Context(), handlers.UserContextKey, admin)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	app.HandleUpdateStudent(rec, req)

	// User credentials should remain untouched
	currUser, _ := store.GetUserByStudentID(st.ID)
	if currUser.Email != "miguel.original@seriestkd.com" {
		t.Errorf("admin must not be allowed to change email: expected 'miguel.original@seriestkd.com', got '%s'", currUser.Email)
	}
	if currUser.Username != "miguel.orig" {
		t.Errorf("admin must not be allowed to change username: expected 'miguel.orig', got '%s'", currUser.Username)
	}
}

func TestStudentHandler_ResetStudentPassword_Success(t *testing.T) {
	app, store := setupTestApp(t)

	st := &models.Student{
		ID:                uuid.New(),
		FullName:          "Robby Keene",
		DOB:               time.Now().AddDate(-17, 0, 0),
		Gender:            "Male",
		Phone:             "09112228899",
		CurrentBelt:       models.BeltHighBlue,
		LastPromotionDate: time.Now(),
		EmergencyName:     "Shannon Keene",
		EmergencyPhone:    "09112228800",
		EmergencyRelation: "Mother",
		IsActive:          true,
	}
	_ = store.CreateStudent(st)

	u := &models.User{
		ID:        uuid.New(),
		Email:     "robby@seriestkd.com",
		Username:  "robby.keene",
		Role:      models.RoleStudent,
		StudentID: &st.ID,
		IsActive:  true,
	}
	_ = u.SetPassword("oldpassword123")
	_ = store.CreateUser(u)

	mgr := &models.User{
		ID:       uuid.New(),
		Email:    "manager@seriestkd.com",
		Role:     models.RoleOperationManager,
		IsActive: true,
	}

	form := url.Values{}
	form.Set("new_password", "newsecret2026")
	form.Set("confirm_password", "newsecret2026")

	req := httptest.NewRequest(http.MethodPost, "/students/"+st.ID.String()+"/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", st.ID.String())
	ctx := context.WithValue(req.Context(), handlers.UserContextKey, mgr)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	app.HandleResetStudentPassword(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}

	// Verify new password
	refetched, err := store.GetUserByStudentID(st.ID)
	if err != nil {
		t.Fatalf("failed to retrieve student user: %v", err)
	}
	if !refetched.CheckPassword("newsecret2026") {
		t.Errorf("new password failed authentication check")
	}
	if refetched.CheckPassword("oldpassword123") {
		t.Errorf("old password should no longer be valid")
	}
}

func TestStudentHandler_ResetStudentPassword_JSON(t *testing.T) {
	app, store := setupTestApp(t)

	st := &models.Student{
		ID:                uuid.New(),
		FullName:          "Eli Moskowitz",
		DOB:               time.Now().AddDate(-16, 0, 0),
		Gender:            "Male",
		Phone:             "09112229900",
		CurrentBelt:       models.BeltLowRed,
		LastPromotionDate: time.Now(),
		EmergencyName:     "Mrs. Moskowitz",
		EmergencyPhone:    "09112229911",
		EmergencyRelation: "Mother",
		IsActive:          true,
	}
	_ = store.CreateStudent(st)

	u := &models.User{
		ID:        uuid.New(),
		Email:     "hawk@seriestkd.com",
		Username:  "hawk",
		Role:      models.RoleStudent,
		StudentID: &st.ID,
		IsActive:  true,
	}
	_ = u.SetPassword("hawkinitial123")
	_ = store.CreateUser(u)

	mgr := &models.User{
		ID:       uuid.New(),
		Email:    "manager@seriestkd.com",
		Role:     models.RoleOperationManager,
		IsActive: true,
	}

	payload := `{"new_password":"flipthescript!","confirm_password":"flipthescript!"}`
	req := httptest.NewRequest(http.MethodPost, "/api/students/"+st.ID.String()+"/reset-password", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetPathValue("id", st.ID.String())
	ctx := context.WithValue(req.Context(), handlers.UserContextKey, mgr)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	app.HandleResetStudentPassword(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}
	if resp["status"] != "success" {
		t.Errorf("expected status 'success', got %v", resp["status"])
	}

	refetched, _ := store.GetUserByStudentID(st.ID)
	if !refetched.CheckPassword("flipthescript!") {
		t.Errorf("new password failed verification")
	}
}

func TestStudentHandler_ResetStudentPassword_NonManager_Forbidden(t *testing.T) {
	app, store := setupTestApp(t)

	st := &models.Student{
		ID:       uuid.New(),
		FullName: "Demetri Alexopoulos",
		IsActive: true,
	}
	_ = store.CreateStudent(st)

	admin := &models.User{
		ID:       uuid.New(),
		Email:    "admin@seriestkd.com",
		Role:     models.RoleAdmin,
		IsActive: true,
	}

	form := url.Values{}
	form.Set("new_password", "newsecret123")
	form.Set("confirm_password", "newsecret123")

	req := httptest.NewRequest(http.MethodPost, "/students/"+st.ID.String()+"/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", st.ID.String())
	ctx := context.WithValue(req.Context(), handlers.UserContextKey, admin)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	app.HandleResetStudentPassword(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Errorf("expected 403 Forbidden for non-manager, got %d", rec.Code)
	}
}

func TestStudentHandler_ResetStudentPassword_ProvisionNewAccount(t *testing.T) {
	app, store := setupTestApp(t)

	st := &models.Student{
		ID:       uuid.New(),
		FullName: "Samantha LaRusso",
		IsActive: true,
	}
	_ = store.CreateStudent(st)

	// Ensure no user account exists yet
	if u, _ := store.GetUserByStudentID(st.ID); u != nil {
		t.Fatalf("expected no initial user account")
	}

	mgr := &models.User{
		ID:       uuid.New(),
		Email:    "manager@seriestkd.com",
		Role:     models.RoleOperationManager,
		IsActive: true,
	}

	// Manager resets password and provides email to provision account
	form := url.Values{}
	form.Set("email", "sam.larusso@seriestkd.com")
	form.Set("username", "sam.larusso")
	form.Set("new_password", "miyagido2026")
	form.Set("confirm_password", "miyagido2026")

	req := httptest.NewRequest(http.MethodPost, "/students/"+st.ID.String()+"/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", st.ID.String())
	ctx := context.WithValue(req.Context(), handlers.UserContextKey, mgr)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	app.HandleResetStudentPassword(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect, got %d", rec.Code)
	}

	createdUser, err := store.GetUserByStudentID(st.ID)
	if err != nil || createdUser == nil {
		t.Fatalf("expected student account to be provisioned, got error: %v", err)
	}
	if createdUser.Email != "sam.larusso@seriestkd.com" {
		t.Errorf("expected email 'sam.larusso@seriestkd.com', got '%s'", createdUser.Email)
	}
	if createdUser.Username != "sam.larusso" {
		t.Errorf("expected username 'sam.larusso', got '%s'", createdUser.Username)
	}
	if !createdUser.CheckPassword("miyagido2026") {
		t.Errorf("provisioned account failed password authentication")
	}
}

func TestStudentHandler_ResetStudentPassword_FormError_RedirectsToPage(t *testing.T) {
	app, store := setupTestApp(t)

	st := &models.Student{
		ID:       uuid.New(),
		FullName: "Tory Nichols",
		IsActive: true,
	}
	_ = store.CreateStudent(st)

	mgr := &models.User{
		ID:       uuid.New(),
		Email:    "manager@seriestkd.com",
		Role:     models.RoleOperationManager,
		IsActive: true,
	}

	// 1. Password mismatch should redirect with ?error=, never throw 400 Bad Request
	form := url.Values{}
	form.Set("email", "tory@seriestkd.com")
	form.Set("new_password", "password123")
	form.Set("confirm_password", "different123")

	req := httptest.NewRequest(http.MethodPost, "/students/"+st.ID.String()+"/reset-password", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", st.ID.String())
	ctx := context.WithValue(req.Context(), handlers.UserContextKey, mgr)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	app.HandleResetStudentPassword(rec, req)

	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on form validation failure, got status %d", rec.Code)
	}
	loc := rec.Header().Get("Location")
	if !strings.Contains(loc, "error=") {
		t.Errorf("expected location to include error notice, got %s", loc)
	}

	// 2. GET on reset-password redirects to /students/{id} cleanly
	reqGet := httptest.NewRequest(http.MethodGet, "/students/"+st.ID.String()+"/reset-password", nil)
	reqGet.SetPathValue("id", st.ID.String())
	reqGet = reqGet.WithContext(ctx)
	recGet := httptest.NewRecorder()

	app.HandleResetStudentPassword(recGet, reqGet)
	if recGet.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect on GET reset-password, got status %d", recGet.Code)
	}
	if recGet.Header().Get("Location") != "/students/"+st.ID.String() {
		t.Errorf("expected redirect to /students/%s, got %s", st.ID.String(), recGet.Header().Get("Location"))
	}
}

func TestStudentHandler_StudentDetail_TrailingSlashAndSubpaths(t *testing.T) {
	app, store := setupTestApp(t)

	st := &models.Student{
		ID:          uuid.New(),
		FullName:    "Bert",
		CurrentBelt: models.BeltWhite,
		IsActive:    true,
	}
	_ = store.CreateStudent(st)

	mgr := &models.User{
		ID:       uuid.New(),
		Email:    "manager@seriestkd.com",
		Role:     models.RoleOperationManager,
		IsActive: true,
	}

	// Request with trailing slash must not return 400 Bad Request
	req := httptest.NewRequest(http.MethodGet, "/students/"+st.ID.String()+"/", nil)
	ctx := context.WithValue(req.Context(), handlers.UserContextKey, mgr)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	app.HandleStudentDetail(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200 OK for detail with trailing slash, got %d", rec.Code)
	}
}


