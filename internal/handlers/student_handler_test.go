package handlers_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
