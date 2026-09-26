package repository_test

import (
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

func TestSQLStore_SQLitePersistence(t *testing.T) {
	dbFile := "test_stms.db"
	_ = os.Remove(dbFile)
	defer os.Remove(dbFile)

	// 1. Initialize store with SQLite file
	store, driver, err := repository.InitDatabase(dbFile)
	if err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}
	if driver != "sqlite" {
		t.Errorf("expected driver sqlite, got %s", driver)
	}

	// Verify students were seeded
	students, err := store.GetAllStudents()
	if err != nil {
		t.Fatalf("GetAllStudents failed: %v", err)
	}
	if len(students) < 4 {
		t.Fatalf("expected at least 4 seeded students, got %d", len(students))
	}

	// Verify sessions were seeded
	sessions, err := store.GetAllSessions()
	if err != nil {
		t.Fatalf("GetAllSessions failed: %v", err)
	}
	if len(sessions) == 0 {
		t.Fatalf("expected seeded sessions")
	}

	// Find Chloe Ramirez (s2)
	var chloe *models.Student
	for _, s := range students {
		if s.FullName == "Chloe Ramirez" {
			chloe = s
			break
		}
	}
	if chloe == nil {
		t.Fatalf("Chloe Ramirez not found in students")
	}

	// Check Chloe's packages
	pkgs, err := store.GetStudentPackages(chloe.ID)
	if err != nil {
		t.Fatalf("GetStudentPackages failed: %v", err)
	}
	if len(pkgs) == 0 {
		t.Fatalf("expected packages for Chloe")
	}
	pkg := pkgs[0]
	remBefore := *pkg.RemainingSessions

	// Create a dedicated test session to check in Chloe
	targetSession := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  time.Now(),
		StartTime:    "19:00",
		EndTime:      "20:00",
		CoachID:      uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		TrainingType: models.TrainingSparring,
	}
	if err := store.CreateSession(targetSession); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	att, err := store.CheckInStudent(targetSession.ID, chloe.ID, &pkg.ID)
	if err != nil {
		t.Fatalf("CheckInStudent failed: %v", err)
	}
	if att.StudentName != "Chloe Ramirez" {
		t.Errorf("expected StudentName Chloe Ramirez, got %s", att.StudentName)
	}

	// Deduct and update package
	newRem := remBefore - 1
	pkg.RemainingSessions = &newRem
	if err := store.UpdateStudentPackage(pkg); err != nil {
		t.Fatalf("UpdateStudentPackage failed: %v", err)
	}

	// Try checking in Chloe again -> must return ErrAlreadyInRoster
	_, err = store.CheckInStudent(targetSession.ID, chloe.ID, &pkg.ID)
	if err != repository.ErrAlreadyInRoster {
		t.Errorf("expected ErrAlreadyInRoster, got %v", err)
	}

	// Close the store/DB
	if sqlStore, ok := store.(*repository.SQLStore); ok {
		_ = sqlStore.Close()
	}

	// 2. Reopen the exact same SQLite database file to PROVE persistence on disk!
	reopenedDB, err := sql.Open("sqlite", dbFile)
	if err != nil {
		t.Fatalf("failed to reopen db: %v", err)
	}
	reopenedStore := repository.NewSQLStore(reopenedDB, "sqlite")
	defer reopenedStore.Close()

	// Verify Chloe's attendance was persisted to disk!
	atts, err := reopenedStore.GetSessionAttendances(targetSession.ID)
	if err != nil {
		t.Fatalf("GetSessionAttendances after reopen failed: %v", err)
	}
	found := false
	for _, a := range atts {
		if a.StudentID == chloe.ID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Chloe was NOT found in persisted attendances after re-opening the database!")
	}

	// Verify remaining sessions was updated on disk!
	pkgsAfter, err := reopenedStore.GetStudentPackages(chloe.ID)
	if err != nil {
		t.Fatalf("GetStudentPackages after reopen failed: %v", err)
	}
	if len(pkgsAfter) == 0 || pkgsAfter[0].RemainingSessions == nil || *pkgsAfter[0].RemainingSessions != newRem {
		t.Errorf("expected remaining sessions %d, got %v", newRem, pkgsAfter[0].RemainingSessions)
	}

	// Test Creating a new Student in reopened store
	newStudentID := uuid.New()
	err = reopenedStore.CreateStudent(&models.Student{
		ID:                newStudentID,
		FullName:          "Johnny Lawrence",
		DOB:               time.Now().AddDate(-25, 0, 0),
		Gender:            "Male",
		CurrentBelt:       models.BeltBlack1stDan,
		LastPromotionDate: time.Now().AddDate(-1, 0, 0),
		EmergencyName:     "Kreese",
		EmergencyPhone:    "555-9999",
		EmergencyRelation: "Sensei",
		IsActive:          true,
		CreatedAt:         time.Now(),
	})
	if err != nil {
		t.Fatalf("CreateStudent failed: %v", err)
	}

	st, err := reopenedStore.GetStudentByID(newStudentID)
	if err != nil {
		t.Fatalf("GetStudentByID failed: %v", err)
	}
	if st.FullName != "Johnny Lawrence" {
		t.Errorf("expected Johnny Lawrence, got %s", st.FullName)
	}
}

func TestSQLStore_AuthAndSafetyPersistence(t *testing.T) {
	dbFile := "test_auth.db"
	_ = os.Remove(dbFile)
	defer os.Remove(dbFile)

	store, _, err := repository.InitDatabase(dbFile)
	if err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}
	defer func() {
		if s, ok := store.(*repository.SQLStore); ok {
			_ = s.Close()
		}
	}()

	// 1. Verify seeded users
	admin, err := store.GetUserByEmail("admin@seriestkd.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed for admin: %v", err)
	}
	if admin.Role != models.RoleAdmin || !admin.CheckPassword("admin123") {
		t.Errorf("admin credentials or role mismatch")
	}

	coach, err := store.GetUserByEmail("jiwoo.park@seriestkd.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed for coach: %v", err)
	}
	if coach.Role != models.RoleCoach || coach.CoachID == nil {
		t.Errorf("coach role or coach_id mismatch")
	}

	student, err := store.GetUserByEmail("alex.vance@seriestkd.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed for student: %v", err)
	}
	if student.Role != models.RoleStudent || student.StudentID == nil {
		t.Errorf("student role or student_id mismatch")
	}

	// 2. Test Session Tokens
	token := "test-session-token-xyz-123"
	exp := time.Now().Add(24 * time.Hour)
	if err := store.CreateSessionToken(token, admin.ID, exp); err != nil {
		t.Fatalf("CreateSessionToken failed: %v", err)
	}

	sessionUser, err := store.GetUserBySessionToken(token)
	if err != nil {
		t.Fatalf("GetUserBySessionToken failed: %v", err)
	}
	if sessionUser.Email != "admin@seriestkd.com" {
		t.Errorf("expected admin email, got %s", sessionUser.Email)
	}

	if err := store.DeleteSessionToken(token); err != nil {
		t.Fatalf("DeleteSessionToken failed: %v", err)
	}
	if _, err := store.GetUserBySessionToken(token); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound after session deletion, got %v", err)
	}

	// 3. Test Safety Incidents and Flagging
	incidents, err := store.GetSafetyIncidents(nil)
	if err != nil {
		t.Fatalf("GetSafetyIncidents failed: %v", err)
	}
	if len(incidents) == 0 {
		t.Fatalf("expected seeded safety incident")
	}
	inc := incidents[0]
	if inc.Resolved {
		t.Errorf("seeded incident should initially be unresolved")
	}

	// Resolve the incident
	if err := store.ResolveSafetyIncident(inc.ID, "admin@seriestkd.com"); err != nil {
		t.Fatalf("ResolveSafetyIncident failed: %v", err)
	}

	stAfter, err := store.GetStudentByID(inc.StudentID)
	if err != nil {
		t.Fatalf("GetStudentByID failed: %v", err)
	}
	if stAfter.HasSafetyFlag {
		t.Errorf("student safety flag should be false after resolution")
	}

	// 4. Test Student Promotion
	var stID uuid.UUID
	if student.StudentID != nil {
		stID = *student.StudentID
	}
	if err := store.PromoteStudent(stID, models.BeltYellowTag); err != nil {
		t.Fatalf("PromoteStudent failed: %v", err)
	}
	promotedSt, err := store.GetStudentByID(stID)
	if err != nil {
		t.Fatalf("GetStudentByID after promotion failed: %v", err)
	}
	if promotedSt.CurrentBelt != models.BeltYellowTag {
		t.Errorf("expected Yellow Tag, got %s", promotedSt.CurrentBelt)
	}
}

func TestSQLStore_CustomizablePackageTemplates(t *testing.T) {
	dbFile := "test_pkg_templates.db"
	_ = os.Remove(dbFile)
	defer os.Remove(dbFile)

	store, _, err := repository.InitDatabase(dbFile)
	if err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}

	sessions := 10
	tpl := &models.PackageTemplate{
		ID:           uuid.New(),
		Title:        "Custom 10-Class Sparring Pass",
		Description:  "Intensive technical sparring clinic",
		SessionCount: &sessions,
		ValidityDays: 60,
		Price:        149.99,
		IsActive:     true,
	}

	// 1. Create Template
	if err := store.CreatePackageTemplate(tpl); err != nil {
		t.Fatalf("CreatePackageTemplate failed: %v", err)
	}

	// 2. Get by ID
	fetched, err := store.GetPackageTemplateByID(tpl.ID)
	if err != nil {
		t.Fatalf("GetPackageTemplateByID failed: %v", err)
	}
	if fetched.Title != tpl.Title || fetched.Description != tpl.Description || fetched.Price != tpl.Price {
		t.Fatalf("fetched template mismatch: got %+v, want %+v", fetched, tpl)
	}

	// 3. Update Template
	newSessions := 12
	fetched.Title = "Updated 12-Class Sparring Pass"
	fetched.Description = "Updated sparring clinic description"
	fetched.SessionCount = &newSessions
	fetched.Price = 169.99
	fetched.ValidityDays = 75
	if err := store.UpdatePackageTemplate(fetched); err != nil {
		t.Fatalf("UpdatePackageTemplate failed: %v", err)
	}

	updated, err := store.GetPackageTemplateByID(tpl.ID)
	if err != nil {
		t.Fatalf("GetPackageTemplateByID after update failed: %v", err)
	}
	if updated.Title != "Updated 12-Class Sparring Pass" || *updated.SessionCount != 12 || updated.Price != 169.99 {
		t.Fatalf("update verification failed: %+v", updated)
	}

	// 4. Toggle Status
	if err := store.TogglePackageTemplateStatus(tpl.ID, false); err != nil {
		t.Fatalf("TogglePackageTemplateStatus to false failed: %v", err)
	}
	toggled, _ := store.GetPackageTemplateByID(tpl.ID)
	if toggled.IsActive {
		t.Fatal("expected template to be inactive")
	}

	// 5. Assign with Custom Overrides to a student
	students, _ := store.GetAllStudents()
	if len(students) == 0 {
		t.Fatal("expected seeded students")
	}
	st := students[0]

	customPrice := 120.00
	customSessions := 14
	sp := &models.StudentPackage{
		ID:                uuid.New(),
		StudentID:         st.ID,
		TemplateID:        tpl.ID,
		TotalSessions:     &customSessions,
		RemainingSessions: &customSessions,
		CustomPrice:       &customPrice,
		Notes:             "Promotional 2 bonus classes added by Master Kim",
		PurchaseDate:      time.Now(),
		ExpiryDate:        time.Now().AddDate(0, 0, 90),
		PaymentStatus:     "paid",
	}

	if err := store.AssignPackage(sp); err != nil {
		t.Fatalf("AssignPackage with custom overrides failed: %v", err)
	}

	stPkgs, err := store.GetStudentPackages(st.ID)
	if err != nil {
		t.Fatalf("GetStudentPackages failed: %v", err)
	}

	var found *models.StudentPackage
	for _, p := range stPkgs {
		if p.ID == sp.ID {
			found = p
			break
		}
	}
	if found == nil {
		t.Fatal("newly assigned package not found in student packages")
	}
	if found.CustomPrice == nil || *found.CustomPrice != 120.00 {
		t.Fatalf("expected custom price 120.00, got %v", found.CustomPrice)
	}
	if found.Notes != "Promotional 2 bonus classes added by Master Kim" {
		t.Fatalf("expected custom notes, got %s", found.Notes)
	}
	if *found.TotalSessions != 14 || *found.RemainingSessions != 14 {
		t.Fatalf("expected 14 custom sessions, got total %d rem %d", *found.TotalSessions, *found.RemainingSessions)
	}
}
