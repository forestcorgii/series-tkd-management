package handlers_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/handlers"
	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

func TestProfileHandler_StudentFlow(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	// 1. Create student and linked user
	studentID := uuid.New()
	student := &models.Student{
		ID:                studentID,
		FullName:          "Alex Vance",
		DOB:               time.Now().AddDate(-16, 0, 0),
		Gender:            "Male",
		Phone:             "+1-555-0101",
		CurrentBelt:       models.BeltHighYellow,
		LastPromotionDate: time.Now().AddDate(0, -2, 0),
		EmergencyName:     "Maria Vance",
		EmergencyPhone:    "+1-555-0102",
		EmergencyRelation: "Mother",
		MedicalNotes:      "None",
		IsActive:          true,
	}
	if err := store.CreateStudent(student); err != nil {
		t.Fatalf("failed to create student: %v", err)
	}

	userID := uuid.New()
	studentUser := &models.User{
		ID:          userID,
		Email:       "alex@seriestkd.com",
		Role:        models.RoleStudent,
		StudentID:   &studentID,
		IsActive:    true,
		DisplayName: "Alex Vance",
	}
	_ = studentUser.SetPassword("student123")
	if err := store.CreateUser(studentUser); err != nil {
		t.Fatalf("failed to create student user: %v", err)
	}

	// Helper to attach user session via cookie
	withSession := func(req *http.Request, u *models.User) *http.Request {
		token := uuid.New().String()
		_ = store.CreateSessionToken(token, u.ID, time.Now().Add(time.Hour))
		req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
		return req
	}

	executeHandler := func(rec *httptest.ResponseRecorder, req *http.Request) {
		app.AuthMiddleware(http.HandlerFunc(app.HandleProfile)).ServeHTTP(rec, req)
	}

	// 2. Unauthenticated GET /profile redirects to /login
	req := httptest.NewRequest("GET", "/profile", nil)
	rec := httptest.NewRecorder()
	executeHandler(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Errorf("expected 303 redirect for unauthenticated user, got %d", rec.Code)
	}

	// 3. Authenticated GET /profile as student
	req = httptest.NewRequest("GET", "/profile", nil)
	req = withSession(req, studentUser)
	rec = httptest.NewRecorder()
	executeHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for student profile, got %d", rec.Code)
	}
	body := rec.Body.String()

	// Verify navbar branding
	if !strings.Contains(body, "SERIES TAEKWONDO") {
		t.Errorf("expected 'SERIES TAEKWONDO' brand title in navbar")
	}
	if !strings.Contains(body, "Students") {
		t.Errorf("expected 'Students' role badge in navbar")
	}
	// Verify student navbar menu item is removed (should not contain center nav link for student)
	if strings.Contains(body, ">My Profile</a>") {
		t.Errorf("expected student menu 'My Profile' to be removed from navbar")
	}
	// Verify profile button points to /profile
	if !strings.Contains(body, `href="/profile"`) {
		t.Errorf("expected user profile pill to point to /profile")
	}
	// Verify student details
	if !strings.Contains(body, "Alex Vance") || !strings.Contains(body, "Maria Vance") {
		t.Errorf("profile page missing student personal or emergency details")
	}

	// 4. POST /profile update emergency contact and phone
	form := url.Values{}
	form.Set("phone", "+1-555-8888")
	form.Set("gender", "Male")
	form.Set("emergency_name", "Robert Vance")
	form.Set("emergency_phone", "+1-555-9999")
	form.Set("emergency_relation", "Father")
	form.Set("medical_notes", "Asthma inhaler in gym bag")

	req = httptest.NewRequest("POST", "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withSession(req, studentUser)
	rec = httptest.NewRecorder()
	executeHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK on profile update, got %d", rec.Code)
	}
	postBody := rec.Body.String()
	if !strings.Contains(postBody, "Profile updated successfully!") {
		t.Errorf("expected success notification banner on profile update")
	}

	// Verify persistence in repository
	updatedStudent, err := store.GetStudentByID(studentID)
	if err != nil {
		t.Fatalf("failed to retrieve updated student: %v", err)
	}
	if updatedStudent.Phone != "+1-555-8888" {
		t.Errorf("expected updated phone '+1-555-8888', got '%s'", updatedStudent.Phone)
	}
	if updatedStudent.EmergencyName != "Robert Vance" {
		t.Errorf("expected updated emergency name 'Robert Vance', got '%s'", updatedStudent.EmergencyName)
	}
	if updatedStudent.EmergencyPhone != "+1-555-9999" {
		t.Errorf("expected updated emergency phone '+1-555-9999', got '%s'", updatedStudent.EmergencyPhone)
	}
	if updatedStudent.EmergencyRelation != "Father" {
		t.Errorf("expected updated emergency relation 'Father', got '%s'", updatedStudent.EmergencyRelation)
	}
	if updatedStudent.MedicalNotes != "Asthma inhaler in gym bag" {
		t.Errorf("expected updated medical notes, got '%s'", updatedStudent.MedicalNotes)
	}
}

func TestProfileHandler_CoachAndAdminFlow(t *testing.T) {
	store := repository.NewMemoryStore()
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}

	// 1. Setup Coach
	coachID := uuid.New()
	coach := &models.Coach{
		ID:                coachID,
		FullName:          "Master Park",
		Email:             "coach@seriestkd.com",
		Phone:             "+1-555-0200",
		BeltRank:          "4th Dan Black",
		RatePerSession:    65.0,
		FirstAidCertified: true,
		Specialties:       []string{"Poomsae", "Conditioning"},
		IsActive:          true,
	}
	if err := store.CreateCoach(coach); err != nil {
		t.Fatalf("failed to create coach: %v", err)
	}

	coachUserID := uuid.New()
	coachUser := &models.User{
		ID:          coachUserID,
		Email:       "coach@seriestkd.com",
		Role:        models.RoleCoach,
		CoachID:     &coachID,
		IsActive:    true,
		DisplayName: "Master Park",
	}
	_ = coachUser.SetPassword("coach123")
	if err := store.CreateUser(coachUser); err != nil {
		t.Fatalf("failed to create coach user: %v", err)
	}

	// Helper to attach user session via cookie
	withSession := func(req *http.Request, u *models.User) *http.Request {
		token := uuid.New().String()
		_ = store.CreateSessionToken(token, u.ID, time.Now().Add(time.Hour))
		req.AddCookie(&http.Cookie{Name: "stms_session", Value: token})
		return req
	}

	executeHandler := func(rec *httptest.ResponseRecorder, req *http.Request) {
		app.AuthMiddleware(http.HandlerFunc(app.HandleProfile)).ServeHTTP(rec, req)
	}

	// 2. GET /profile as Coach
	req := httptest.NewRequest("GET", "/profile", nil)
	req = withSession(req, coachUser)
	rec := httptest.NewRecorder()
	executeHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for coach profile, got %d", rec.Code)
	}
	coachBody := rec.Body.String()
	if !strings.Contains(coachBody, "Coach") {
		t.Errorf("expected 'Coach' role badge in navbar")
	}
	if !strings.Contains(coachBody, "Students") || !strings.Contains(coachBody, "Attendance") {
		t.Errorf("expected coach operational links (Students, Attendance) in navbar")
	}

	// 3. POST /profile as Coach (update phone, specialties)
	form := url.Values{}
	form.Set("phone", "+1-555-7777")
	form.Set("specialties", "Kyorugi/Sparring, Demo Team, Cadets")
	req = httptest.NewRequest("POST", "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withSession(req, coachUser)
	rec = httptest.NewRecorder()
	executeHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for coach profile update, got %d", rec.Code)
	}

	updatedCoach, _ := store.GetCoachByID(coachID)
	if updatedCoach.Phone != "+1-555-7777" {
		t.Errorf("expected updated coach phone, got %s", updatedCoach.Phone)
	}
	if len(updatedCoach.Specialties) != 3 || updatedCoach.Specialties[0] != "Kyorugi/Sparring" {
		t.Errorf("expected updated coach specialties, got %v", updatedCoach.Specialties)
	}

	// 4. Setup Operation Manager
	adminUserID := uuid.New()
	adminUser := &models.User{
		ID:          adminUserID,
		Email:       "manager@seriestkd.com",
		Role:        models.RoleOperationManager,
		IsActive:    true,
		DisplayName: "Operations Director",
	}
	_ = adminUser.SetPassword("manager123")
	if err := store.CreateUser(adminUser); err != nil {
		t.Fatalf("failed to create admin user: %v", err)
	}

	// 5. GET /profile as Manager
	req = httptest.NewRequest("GET", "/profile", nil)
	req = withSession(req, adminUser)
	rec = httptest.NewRecorder()
	executeHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for manager profile, got %d", rec.Code)
	}
	managerBody := rec.Body.String()
	if !strings.Contains(managerBody, "Manager") {
		t.Errorf("expected 'Manager' role badge in navbar")
	}

	// 6. POST /profile as Manager (update display name & password)
	form = url.Values{}
	form.Set("display_name", "Head Director")
	form.Set("new_password", "newsecurepass")
	form.Set("confirm_password", "newsecurepass")

	req = httptest.NewRequest("POST", "/profile", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req = withSession(req, adminUser)
	rec = httptest.NewRecorder()
	executeHandler(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 OK for manager profile update, got %d", rec.Code)
	}

	updatedAdmin, _ := store.GetUserByID(adminUserID)
	if updatedAdmin.DisplayName != "Head Director" {
		t.Errorf("expected updated display name 'Head Director', got '%s'", updatedAdmin.DisplayName)
	}
	if !updatedAdmin.CheckPassword("newsecurepass") {
		t.Errorf("expected updated password to verify successfully")
	}
}
