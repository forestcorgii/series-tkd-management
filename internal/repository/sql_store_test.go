package repository_test

import (
	"database/sql"
	"os"
	"path/filepath"
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
		CoachID:      func() *uuid.UUID { id := uuid.MustParse("11111111-1111-1111-1111-111111111111"); return &id }(),
		TrainingType: models.TrainingSparring,
	}
	if err := store.CreateSession(targetSession); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	att, err := store.CheckInStudent(targetSession.ID, chloe.ID, &pkg.ID, nil)
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
	_, err = store.CheckInStudent(targetSession.ID, chloe.ID, &pkg.ID, nil)
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
	if err := store.PromoteStudent(stID, models.BeltLowYellow); err != nil {
		t.Fatalf("PromoteStudent failed: %v", err)
	}
	promotedSt, err := store.GetStudentByID(stID)
	if err != nil {
		t.Fatalf("GetStudentByID after promotion failed: %v", err)
	}
	if promotedSt.CurrentBelt != models.BeltLowYellow {
		t.Errorf("expected Low Yellow, got %s", promotedSt.CurrentBelt)
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

func TestSQLStore_UpdateCoachAndUser(t *testing.T) {
	dbFile := "test_coach_user.db"
	_ = os.Remove(dbFile)
	defer os.Remove(dbFile)

	store, _, err := repository.InitDatabase(dbFile)
	if err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}

	// 1. Test UpdateCoach
	coaches, err := store.GetAllCoaches()
	if err != nil || len(coaches) == 0 {
		t.Fatalf("failed to get seeded coaches: %v", err)
	}
	targetCoach := coaches[0]
	targetCoach.Phone = "+1-555-999-8888"
	targetCoach.Specialties = []string{"Kyorugi/Sparring", "Acrobatic Kicking"}
	if err := store.UpdateCoach(targetCoach); err != nil {
		t.Fatalf("UpdateCoach failed: %v", err)
	}
	updatedCoach, err := store.GetCoachByID(targetCoach.ID)
	if err != nil {
		t.Fatalf("GetCoachByID failed: %v", err)
	}
	if updatedCoach.Phone != "+1-555-999-8888" {
		t.Errorf("expected phone '+1-555-999-8888', got '%s'", updatedCoach.Phone)
	}
	if len(updatedCoach.Specialties) != 2 || updatedCoach.Specialties[1] != "Acrobatic Kicking" {
		t.Errorf("expected updated specialties, got %v", updatedCoach.Specialties)
	}

	// 2. Test UpdateUser
	user, err := store.GetUserByEmail("manager@seriestkd.com")
	if err != nil {
		t.Fatalf("GetUserByEmail failed: %v", err)
	}
	_ = user.SetPassword("newsecret123")
	if err := store.UpdateUser(user); err != nil {
		t.Fatalf("UpdateUser failed: %v", err)
	}
	refetchedUser, err := store.GetUserByID(user.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if !refetchedUser.CheckPassword("newsecret123") {
		t.Errorf("expected password to match updated hash")
	}
}

func TestSQLStore_CancelSession(t *testing.T) {
	dbFile := "test_cancel_session.db"
	_ = os.Remove(dbFile)
	defer os.Remove(dbFile)

	store, _, err := repository.InitDatabase(dbFile)
	if err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}

	coaches, _ := store.GetAllCoaches()
	if len(coaches) == 0 {
		t.Fatalf("no seeded coaches")
	}
	coach := coaches[0]

	students, _ := store.GetAllStudents()
	if len(students) == 0 {
		t.Fatalf("no seeded students")
	}
	student := students[0]

	templates, _ := store.GetPackageTemplates()
	if len(templates) == 0 {
		t.Fatalf("no seeded templates")
	}
	tplID := templates[0].ID

	// Create a package for student with 5 remaining sessions
	remSessions := 5
	totSessions := 10
	pkg := &models.StudentPackage{
		ID:                uuid.New(),
		StudentID:         student.ID,
		TemplateID:        tplID,
		TotalSessions:     &totSessions,
		RemainingSessions: &remSessions,
		PurchaseDate:      time.Now(),
		ExpiryDate:        time.Now().AddDate(0, 1, 0),
		PaymentStatus:     "paid",
	}
	if err := store.AssignPackage(pkg); err != nil {
		t.Fatalf("AssignPackage failed: %v", err)
	}

	// 1. Create session 1 (to be cancelled WITH refund)
	sess1 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  time.Now(),
		StartTime:    "17:00",
		EndTime:      "18:30",
		CoachID:      &coach.ID,
		TrainingType: models.TrainingSparring,
		Notes:        "Sparring fundamentals",
	}
	if err := store.CreateSession(sess1); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}

	// Deduct 1 credit for check-in
	*pkg.RemainingSessions = 4
	if err := store.UpdateStudentPackage(pkg); err != nil {
		t.Fatalf("UpdateStudentPackage failed: %v", err)
	}
	if _, err := store.CheckInStudent(sess1.ID, student.ID, &pkg.ID, nil); err != nil {
		t.Fatalf("CheckInStudent failed: %v", err)
	}

	// Verify check-in exists
	atts, err := store.GetSessionAttendances(sess1.ID)
	if err != nil || len(atts) != 1 {
		t.Fatalf("expected 1 attendance before cancel, got %d", len(atts))
	}

	// Cancel session 1 WITH refund
	if err := store.CancelSession(sess1.ID, "Typhoon signal", true); err != nil {
		t.Fatalf("CancelSession failed: %v", err)
	}

	// Verify session status
	updatedSess1, err := store.GetSessionByID(sess1.ID)
	if err != nil {
		t.Fatalf("GetSessionByID failed: %v", err)
	}
	if !updatedSess1.IsCancelled {
		t.Errorf("expected session to be marked cancelled")
	}
	if updatedSess1.CancellationReason != "Typhoon signal" {
		t.Errorf("expected cancellation reason 'Typhoon signal', got '%s'", updatedSess1.CancellationReason)
	}
	if updatedSess1.CancelledAt == nil {
		t.Errorf("expected CancelledAt to be populated")
	}

	// Verify attendances cleared
	attsAfter, err := store.GetSessionAttendances(sess1.ID)
	if err != nil || len(attsAfter) != 0 {
		t.Errorf("expected 0 attendances after refund cancellation, got %d", len(attsAfter))
	}

	// Verify package remaining sessions refunded from 4 back to 5
	pkgs, _ := store.GetStudentPackages(student.ID)
	var foundPkg *models.StudentPackage
	for _, p := range pkgs {
		if p.ID == pkg.ID {
			foundPkg = p
			break
		}
	}
	if foundPkg == nil || foundPkg.RemainingSessions == nil || *foundPkg.RemainingSessions != 5 {
		t.Errorf("expected package sessions to be refunded to 5, got %v", foundPkg)
	}

	// Verify double cancellation returns error
	if err := store.CancelSession(sess1.ID, "Try again", true); err == nil {
		t.Errorf("expected error on cancelling already cancelled session")
	}

	// 2. Create session 2 (cancelled WITHOUT refund)
	sess2 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  time.Now(),
		StartTime:    "19:00",
		EndTime:      "20:30",
		CoachID:      &coach.ID,
		TrainingType: models.TrainingPoomsae,
	}
	if err := store.CreateSession(sess2); err != nil {
		t.Fatalf("CreateSession failed: %v", err)
	}
	if _, err := store.CheckInStudent(sess2.ID, student.ID, nil, nil); err != nil {
		t.Fatalf("CheckInStudent failed: %v", err)
	}

	if err := store.CancelSession(sess2.ID, "Coach sick", false); err != nil {
		t.Fatalf("CancelSession without refund failed: %v", err)
	}
	updatedSess2, _ := store.GetSessionByID(sess2.ID)
	if !updatedSess2.IsCancelled {
		t.Errorf("expected sess2 to be cancelled")
	}
	// Attendances should remain
	attsSess2, _ := store.GetSessionAttendances(sess2.ID)
	if len(attsSess2) != 1 {
		t.Errorf("expected 1 attendance kept when refund=false, got %d", len(attsSess2))
	}
}

func TestSQLStore_FilterSessions(t *testing.T) {
	dbFile := "test_filter_session.db"
	_ = os.Remove(dbFile)
	defer os.Remove(dbFile)

	store, _, err := repository.InitDatabase(dbFile)
	if err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}

	if sqlStore, ok := store.(*repository.SQLStore); ok {
		_, _ = sqlStore.DB().Exec("DELETE FROM attendance")
		_, _ = sqlStore.DB().Exec("DELETE FROM training_sessions")
	}

	coaches, _ := store.GetAllCoaches()
	if len(coaches) < 2 {
		t.Fatalf("expected at least 2 coaches")
	}
	coachA := coaches[0]
	coachB := coaches[1]

	date1, _ := time.Parse("2006-01-02", "2026-10-01")
	date2, _ := time.Parse("2006-01-02", "2026-10-02")

	// Create distinctive sessions
	s1 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  date1,
		StartTime:    "10:00",
		EndTime:      "11:00",
		CoachID:      &coachA.ID,
		TrainingType: models.TrainingSparring,
	}
	s2 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  date1,
		StartTime:    "14:00",
		EndTime:      "15:00",
		CoachID:      &coachB.ID,
		TrainingType: models.TrainingPoomsae,
	}
	s3 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  date2,
		StartTime:    "16:00",
		EndTime:      "17:00",
		CoachID:      &coachA.ID,
		TrainingType: models.TrainingConditioning,
	}

	for _, s := range []*models.TrainingSession{s1, s2, s3} {
		if err := store.CreateSession(s); err != nil {
			t.Fatalf("CreateSession failed: %v", err)
		}
	}

	// 1. Filter by Coach
	byCoachA, err := store.GetSessions(repository.SessionFilter{CoachID: &coachA.ID})
	if err != nil {
		t.Fatalf("Filter by coach failed: %v", err)
	}
	for _, s := range byCoachA {
		if s.CoachID == nil || *s.CoachID != coachA.ID {
			t.Errorf("expected session coach %v, got %v", coachA.ID, s.CoachID)
		}
	}

	// 2. Filter by Category
	bySparring, err := store.GetSessions(repository.SessionFilter{TrainingType: "Sparring"})
	if err != nil {
		t.Fatalf("Filter by category failed: %v", err)
	}
	for _, s := range bySparring {
		if s.TrainingType != models.TrainingSparring {
			t.Errorf("expected Sparring, got %s", s.TrainingType)
		}
	}

	// 3. Filter by Date
	byDate2, err := store.GetSessions(repository.SessionFilter{Date: "2026-10-02"})
	if err != nil {
		t.Fatalf("Filter by date failed: %v", err)
	}
	for _, s := range byDate2 {
		if s.SessionDate.Format("2006-01-02") != "2026-10-02" {
			t.Errorf("expected date 2026-10-02, got %s", s.SessionDate.Format("2006-01-02"))
		}
	}

	// 4. Combined: CoachA + Date1
	combined, err := store.GetSessions(repository.SessionFilter{
		CoachID: &coachA.ID,
		Date:    "2026-10-01",
	})
	if err != nil {
		t.Fatalf("Combined filter failed: %v", err)
	}
	if len(combined) != 1 || combined[0].ID != s1.ID {
		t.Errorf("expected s1, got %d sessions", len(combined))
	}

	// 5. Date Range: StartDate & EndDate
	rangeSessions, err := store.GetSessions(repository.SessionFilter{
		StartDate: "2026-10-01",
		EndDate:   "2026-10-02",
	})
	if err != nil {
		t.Fatalf("Date range filter failed: %v", err)
	}
	if len(rangeSessions) != 3 {
		t.Errorf("expected 3 sessions in range 2026-10-01 to 2026-10-02, got %d", len(rangeSessions))
	}

	rangeDay1, err := store.GetSessions(repository.SessionFilter{
		StartDate: "2026-10-01",
		EndDate:   "2026-10-01",
	})
	if err != nil {
		t.Fatalf("Single day range filter failed: %v", err)
	}
	if len(rangeDay1) != 2 {
		t.Errorf("expected 2 sessions on 2026-10-01, got %d", len(rangeDay1))
	}
}

func TestSQLStore_AdminsManagement(t *testing.T) {
	dbFile := "test_admins_mgmt.db"
	_ = os.Remove(dbFile)
	defer os.Remove(dbFile)

	store, _, err := repository.InitDatabase(dbFile)
	if err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}

	// Initial seed has admin@seriestkd.com with role ADMIN
	admins, err := store.GetUsersByRole(models.RoleAdmin)
	if err != nil {
		t.Fatalf("GetUsersByRole failed: %v", err)
	}
	if len(admins) == 0 {
		t.Fatalf("expected at least 1 admin from seed, got 0")
	}

	// Create a new admin with custom display name
	newAdmin := &models.User{
		ID:          uuid.New(),
		Email:       "elena.rostova@seriestkd.com",
		Role:        models.RoleAdmin,
		DisplayName: "Elena Rostova",
		IsActive:    true,
	}
	if err := newAdmin.SetPassword("adminpass123"); err != nil {
		t.Fatalf("SetPassword failed: %v", err)
	}
	if err := store.CreateUser(newAdmin); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Retrieve by ID and check display_name
	retrieved, err := store.GetUserByID(newAdmin.ID)
	if err != nil {
		t.Fatalf("GetUserByID failed: %v", err)
	}
	if retrieved.DisplayName != "Elena Rostova" {
		t.Errorf("expected DisplayName 'Elena Rostova', got '%s'", retrieved.DisplayName)
	}

	// Retrieve by role
	adminsAfter, err := store.GetUsersByRole(models.RoleAdmin)
	if err != nil {
		t.Fatalf("GetUsersByRole after create failed: %v", err)
	}
	if len(adminsAfter) != len(admins)+1 {
		t.Errorf("expected %d admins, got %d", len(admins)+1, len(adminsAfter))
	}

	// Toggle active status
	if err := store.ToggleUserActive(newAdmin.ID, false); err != nil {
		t.Fatalf("ToggleUserActive failed: %v", err)
	}
	deactivated, err := store.GetUserByID(newAdmin.ID)
	if err != nil {
		t.Fatalf("GetUserByID after toggle failed: %v", err)
	}
	if deactivated.IsActive {
		t.Errorf("expected admin to be inactive")
	}

	// Reactivate
	if err := store.ToggleUserActive(newAdmin.ID, true); err != nil {
		t.Fatalf("ToggleUserActive reactivate failed: %v", err)
	}
	reactivated, err := store.GetUserByID(newAdmin.ID)
	if err != nil {
		t.Fatalf("GetUserByID after reactivate failed: %v", err)
	}
	if !reactivated.IsActive {
		t.Errorf("expected admin to be active again")
	}

	// Delete user
	if err := store.DeleteUser(newAdmin.ID); err != nil {
		t.Fatalf("DeleteUser failed: %v", err)
	}
	deletedAdmin, err := store.GetUserByID(newAdmin.ID)
	if err == nil || deletedAdmin != nil {
		t.Errorf("expected user to be deleted, got: %v", deletedAdmin)
	}
}

func TestSQLStore_CoachToggleAndDeletion(t *testing.T) {
	dbFile := "test_coach_toggle.db"
	_ = os.Remove(dbFile)
	defer os.Remove(dbFile)

	store, _, err := repository.InitDatabase(dbFile)
	if err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}

	c2ID := uuid.MustParse("22222222-2222-2222-2222-222222222222") // Coach Ji-Woo Park (has sessions)

	// 1. Toggle active to false
	if err := store.ToggleCoachActive(c2ID, false); err != nil {
		t.Fatalf("ToggleCoachActive(false) failed: %v", err)
	}
	coach, err := store.GetCoachByID(c2ID)
	if err != nil || coach.IsActive {
		t.Errorf("expected coach to be inactive, got active=%v, err=%v", coach.IsActive, err)
	}
	coachUser, err := store.GetUserByEmail("jiwoo.park@seriestkd.com")
	if err != nil || coachUser.IsActive {
		t.Errorf("expected linked user to be inactive, got active=%v, err=%v", coachUser.IsActive, err)
	}

	// 2. Toggle active back to true
	if err := store.ToggleCoachActive(c2ID, true); err != nil {
		t.Fatalf("ToggleCoachActive(true) failed: %v", err)
	}
	coach, err = store.GetCoachByID(c2ID)
	if err != nil || !coach.IsActive {
		t.Errorf("expected coach to be active, got active=%v, err=%v", coach.IsActive, err)
	}
	coachUser, err = store.GetUserByEmail("jiwoo.park@seriestkd.com")
	if err != nil || !coachUser.IsActive {
		t.Errorf("expected linked user to be active, got active=%v, err=%v", coachUser.IsActive, err)
	}

	// 3. Ji-Woo Park has sessions and evaluations: DeleteCoach must cascade and succeed
	if err := store.DeleteCoach(c2ID); err != nil {
		t.Fatalf("expected successful cascade DeleteCoach, got: %v", err)
	}
	if _, err := store.GetCoachByID(c2ID); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for deleted coach, got: %v", err)
	}
	if _, err := store.GetUserByEmail("jiwoo.park@seriestkd.com"); err != repository.ErrNotFound {
		t.Errorf("expected linked user to be cascade deleted, got: %v", err)
	}

	// 4. Create a fresh coach with no sessions or evaluations
	freshCoach := &models.Coach{
		ID:             uuid.New(),
		FullName:       "Temp Coach John",
		Email:          "temp.john@seriestkd.com",
		Phone:          "+1555111222",
		BeltRank:       "2nd Dan",
		RatePerSession: 50.00,
		IsActive:       true,
		CreatedAt:      time.Now(),
	}
	if err := store.CreateCoach(freshCoach); err != nil {
		t.Fatalf("CreateCoach failed: %v", err)
	}
	freshUser := &models.User{
		ID:          uuid.New(),
		Email:       freshCoach.Email,
		Role:        models.RoleCoach,
		CoachID:     &freshCoach.ID,
		DisplayName: freshCoach.FullName,
		IsActive:    true,
	}
	_ = freshUser.SetPassword("coach123")
	if err := store.CreateUser(freshUser); err != nil {
		t.Fatalf("CreateUser failed: %v", err)
	}

	// Deleting fresh coach should succeed completely
	if err := store.DeleteCoach(freshCoach.ID); err != nil {
		t.Fatalf("DeleteCoach failed on fresh coach: %v", err)
	}

	// Verify coach is gone
	if _, err := store.GetCoachByID(freshCoach.ID); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for deleted coach, got: %v", err)
	}
	// Verify user is gone
	if _, err := store.GetUserByEmail(freshCoach.Email); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound for deleted coach user, got: %v", err)
	}
}

func TestRemoveAttendance_SQLAndMemory(t *testing.T) {
	// Test both MemoryStore and SQLStore
	dbFile := filepath.Join(t.TempDir(), "test_remove_att.db")
	sqlStore, _, err := repository.InitDatabase(dbFile)
	if err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}
	defer func() {
		if s, ok := sqlStore.(*repository.SQLStore); ok {
			_ = s.Close()
		}
	}()

	memStore := repository.NewMemoryStore()

	stores := map[string]repository.RepositoryStore{
		"SQLStore":    sqlStore,
		"MemoryStore": memStore,
	}

	for storeName, store := range stores {
		t.Run(storeName, func(t *testing.T) {
			students, _ := store.GetAllStudents()
			if len(students) == 0 {
				t.Fatalf("no students found")
			}
			student := students[0]

			coaches, _ := store.GetAllCoaches()
			if len(coaches) == 0 {
				t.Fatalf("no coaches found")
			}

			session := &models.TrainingSession{
				ID:           uuid.New(),
				SessionDate:  time.Now(),
				StartTime:    "17:00",
				EndTime:      "18:30",
				CoachID:      &coaches[0].ID,
				TrainingType: models.TrainingSparring,
			}
			if err := store.CreateSession(session); err != nil {
				t.Fatalf("CreateSession failed: %v", err)
			}

			// Create a package with 5 sessions
			templates, _ := store.GetPackageTemplates()
			tplID := templates[0].ID
			rem := 5
			pkg := &models.StudentPackage{
				ID:                uuid.New(),
				StudentID:         student.ID,
				TemplateID:        tplID,
				TotalSessions:     &rem,
				RemainingSessions: &rem,
				PurchaseDate:      time.Now(),
				ExpiryDate:        time.Now().AddDate(0, 1, 0),
				PaymentStatus:     "paid",
			}
			if err := store.AssignPackage(pkg); err != nil {
				t.Fatalf("AssignPackage failed: %v", err)
			}

			// Deduct 1 session for check-in
			remAfterDeduct := 4
			pkg.RemainingSessions = &remAfterDeduct
			_ = store.UpdateStudentPackage(pkg)

			// Check in student
			att, err := store.CheckInStudent(session.ID, student.ID, &pkg.ID, nil)
			if err != nil {
				t.Fatalf("CheckInStudent failed: %v", err)
			}
			if att.StudentID != student.ID {
				t.Errorf("expected att student ID %s, got %s", student.ID, att.StudentID)
			}

			// Verify attendance exists
			atts, err := store.GetSessionAttendances(session.ID)
			if err != nil {
				t.Fatalf("GetSessionAttendances failed: %v", err)
			}
			found := false
			for _, a := range atts {
				if a.StudentID == student.ID {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected student to be in session attendances")
			}

			// Remove attendance
			if err := store.RemoveAttendance(session.ID, student.ID); err != nil {
				t.Fatalf("RemoveAttendance failed: %v", err)
			}

			// Verify attendance is gone
			attsAfter, err := store.GetSessionAttendances(session.ID)
			if err != nil {
				t.Fatalf("GetSessionAttendances after removal failed: %v", err)
			}
			for _, a := range attsAfter {
				if a.StudentID == student.ID {
					t.Errorf("student still found in session attendances after removal")
				}
			}

			// Verify package session was refunded (+1 to remaining)
			pkgs, _ := store.GetStudentPackages(student.ID)
			var updatedPkg *models.StudentPackage
			for _, p := range pkgs {
				if p.ID == pkg.ID {
					updatedPkg = p
					break
				}
			}
			if updatedPkg == nil || updatedPkg.RemainingSessions == nil || *updatedPkg.RemainingSessions != 5 {
				var got int
				if updatedPkg != nil && updatedPkg.RemainingSessions != nil {
					got = *updatedPkg.RemainingSessions
				}
				t.Errorf("expected package sessions refunded to 5, got %d", got)
			}

			// Calling RemoveAttendance again should return ErrNotFound
			errAgain := store.RemoveAttendance(session.ID, student.ID)
			if errAgain != repository.ErrNotFound {
				t.Errorf("expected ErrNotFound on second removal, got: %v", errAgain)
			}

			// Check in again should now succeed because student was removed
			att2, err := store.CheckInStudent(session.ID, student.ID, &pkg.ID, nil)
			if err != nil {
				t.Errorf("expected CheckInStudent to succeed after removal, got: %v", err)
			}
			if att2 == nil {
				t.Errorf("expected non-nil attendance after re-check-in")
			}
		})
	}
}

func TestStore_DeleteStudent(t *testing.T) {
	dbFile := filepath.Join(t.TempDir(), "test_delete_student.db")
	sqlStore, _, err := repository.InitDatabase(dbFile)
	if err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}
	defer func() {
		if s, ok := sqlStore.(*repository.SQLStore); ok {
			_ = s.Close()
		}
	}()

	memStore := repository.NewMemoryStore()

	stores := map[string]repository.RepositoryStore{
		"SQLStore":    sqlStore,
		"MemoryStore": memStore,
	}

	for storeName, store := range stores {
		t.Run(storeName, func(t *testing.T) {
			// 1. Delete non-existent student -> ErrNotFound
			fakeID := uuid.New()
			if err := store.DeleteStudent(fakeID); err != repository.ErrNotFound {
				t.Fatalf("expected ErrNotFound for non-existent student, got: %v", err)
			}

			// 2. Create a student
			st := &models.Student{
				ID:                uuid.New(),
				FullName:          "Kim Possible",
				DOB:               time.Now().AddDate(-15, 0, 0),
				Gender:            "Female",
				Phone:             "09123456789",
				CurrentBelt:       models.BeltHighYellow,
				LastPromotionDate: time.Now().AddDate(0, -2, 0),
				EmergencyName:     "James Possible",
				EmergencyPhone:    "09987654321",
				EmergencyRelation: "Father",
				IsActive:          true,
				CreatedAt:         time.Now(),
			}
			if err := store.CreateStudent(st); err != nil {
				t.Fatalf("CreateStudent failed: %v", err)
			}

			// 3. Create linked user account and session
			user := &models.User{
				ID:          uuid.New(),
				Email:       "kim.possible@seriestkd.com",
				Role:        models.RoleStudent,
				StudentID:   &st.ID,
				DisplayName: st.FullName,
				IsActive:    true,
			}
			_ = user.SetPassword("password123")
			if err := store.CreateUser(user); err != nil {
				t.Fatalf("CreateUser failed: %v", err)
			}
			token := "session_token_" + st.ID.String()
			if err := store.CreateSessionToken(token, user.ID, time.Now().Add(24*time.Hour)); err != nil {
				t.Fatalf("CreateSessionToken failed: %v", err)
			}

			// 4. Create package for student
			tpl := &models.PackageTemplate{
				ID:           uuid.New(),
				Title:        "10-Class Card",
				ValidityDays: 30,
				Price:        100,
				IsActive:     true,
			}
			_ = store.CreatePackageTemplate(tpl)
			rem := 10
			pkg := &models.StudentPackage{
				ID:                uuid.New(),
				StudentID:         st.ID,
				TemplateID:        tpl.ID,
				RemainingSessions: &rem,
				ExpiryDate:        time.Now().AddDate(0, 1, 0),
				PaymentStatus:     "paid",
			}
			if err := store.AssignPackage(pkg); err != nil {
				t.Fatalf("AssignPackage failed: %v", err)
			}

			// 5. Create coach and session, and check in student
			coach := &models.Coach{
				ID:        uuid.New(),
				FullName:  "Master Dan",
				Email:     "master.dan." + storeName + "@seriestkd.com",
				Phone:     "09112223333",
				BeltRank:  "4th Dan",
				IsActive:  true,
				CreatedAt: time.Now(),
			}
			_ = store.CreateCoach(coach)
			sess := &models.TrainingSession{
				ID:           uuid.New(),
				SessionDate:  time.Now(),
				StartTime:    "18:00",
				EndTime:      "19:00",
				CoachID:      &coach.ID,
				TrainingType: models.TrainingPoomsae,
			}
			_ = store.CreateSession(sess)
			_, err = store.CheckInStudent(sess.ID, st.ID, &pkg.ID, nil)
			if err != nil {
				t.Fatalf("CheckInStudent failed: %v", err)
			}

			// 6. Create student evaluation
			eval := &models.StudentEvaluation{
				ID:             uuid.New(),
				StudentID:      st.ID,
				CoachID:        coach.ID,
				EvaluationDate: time.Now(),
				Flexibility:    8,
				Stamina:        8,
				Power:          7,
				Technique:      9,
				SparringIQ:     8,
				Discipline:     10,
				CoachRemarks:   "Great kicks",
			}
			if err := store.CreateEvaluation(eval); err != nil {
				t.Fatalf("CreateEvaluation failed: %v", err)
			}

			// 7. Create safety incident
			inc := &models.SafetyIncident{
				ID:           uuid.New(),
				StudentID:    st.ID,
				CoachID:      &coach.ID,
				IncidentType: "Ankle sprain",
				Notes:        "Iced and rested",
				CreatedAt:    time.Now(),
			}
			if err := store.CreateSafetyIncident(inc); err != nil {
				t.Fatalf("CreateSafetyIncident failed: %v", err)
			}

			// 8. Delete student -> should cascade delete everything associated
			if err := store.DeleteStudent(st.ID); err != nil {
				t.Fatalf("DeleteStudent failed: %v", err)
			}

			// Verify student is gone
			if _, err := store.GetStudentByID(st.ID); err != repository.ErrNotFound {
				t.Errorf("expected ErrNotFound for deleted student, got: %v", err)
			}

			// Verify linked user and session token are purged
			if _, err := store.GetUserByEmail("kim.possible@seriestkd.com"); err != repository.ErrNotFound {
				t.Errorf("expected linked user to be deleted, got: %v", err)
			}
			if _, err := store.GetUserBySessionToken(token); err != repository.ErrNotFound {
				t.Errorf("expected user session to be purged, got: %v", err)
			}

			// Verify student packages are deleted
			pkgs, _ := store.GetStudentPackages(st.ID)
			if len(pkgs) != 0 {
				t.Errorf("expected 0 packages, got %d", len(pkgs))
			}

			// Verify attendance is deleted
			atts, _ := store.GetStudentAttendances(st.ID)
			if len(atts) != 0 {
				t.Errorf("expected 0 attendances for student, got %d", len(atts))
			}

			// Verify evaluations are deleted
			evals, _ := store.GetStudentEvaluations(st.ID)
			if len(evals) != 0 {
				t.Errorf("expected 0 evaluations for student, got %d", len(evals))
			}

			// Verify calling DeleteStudent again returns ErrNotFound
			if err := store.DeleteStudent(st.ID); err != repository.ErrNotFound {
				t.Errorf("expected ErrNotFound on second DeleteStudent, got: %v", err)
			}
		})
	}
}

func TestStore_UpdateAndDeleteSession(t *testing.T) {
	dbFile := filepath.Join(t.TempDir(), "test_update_delete_session.db")
	sqlStore, _, err := repository.InitDatabase(dbFile)
	if err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}
	defer func() {
		if s, ok := sqlStore.(*repository.SQLStore); ok {
			_ = s.Close()
		}
	}()

	memStore := repository.NewMemoryStore()

	stores := map[string]repository.RepositoryStore{
		"SQLStore":    sqlStore,
		"MemoryStore": memStore,
	}

	intPtr := func(i int) *int { return &i }

	for storeName, store := range stores {
		t.Run(storeName, func(t *testing.T) {
			// 1. Create coach 1 and coach 2
			coach1 := &models.Coach{
				ID:        uuid.New(),
				FullName:  "Coach Alpha",
				Email:     "alpha." + storeName + "@seriestkd.com",
				Phone:     "09110001111",
				BeltRank:  "3rd Dan",
				IsActive:  true,
				CreatedAt: time.Now(),
			}
			coach2 := &models.Coach{
				ID:        uuid.New(),
				FullName:  "Coach Beta",
				Email:     "beta." + storeName + "@seriestkd.com",
				Phone:     "09110002222",
				BeltRank:  "4th Dan",
				IsActive:  true,
				CreatedAt: time.Now(),
			}
			_ = store.CreateCoach(coach1)
			_ = store.CreateCoach(coach2)

			// 2. Create admin user
			adminUser := &models.User{
				ID:          uuid.New(),
				Email:       "admin." + storeName + "@seriestkd.com",
				Role:        models.RoleAdmin,
				DisplayName: "Admin Sarah",
				IsActive:    true,
				CreatedAt:   time.Now(),
			}
			_ = store.CreateUser(adminUser)

			// 3. Create a session
			initialRate := 350.0
			sess := &models.TrainingSession{
				ID:           uuid.New(),
				SessionDate:  time.Now(),
				StartTime:    "14:00",
				EndTime:      "15:30",
				CoachID:      &coach1.ID,
				TrainingType: models.TrainingSparring,
				Notes:        "Sparring drills",
				SessionRate:  &initialRate,
				CreatedAt:    time.Now(),
			}
			if err := store.CreateSession(sess); err != nil {
				t.Fatalf("CreateSession failed: %v", err)
			}

			// Verify created session
			loaded, err := store.GetSessionByID(sess.ID)
			if err != nil {
				t.Fatalf("GetSessionByID failed: %v", err)
			}
			if loaded.CoachName != "Coach Alpha" {
				t.Errorf("expected Coach Alpha, got %s", loaded.CoachName)
			}
			if loaded.AdminName != "" {
				t.Errorf("expected empty AdminName initially, got %s", loaded.AdminName)
			}
			if loaded.SessionRate == nil || *loaded.SessionRate != 350.0 {
				t.Errorf("expected SessionRate 350.0, got %v", loaded.SessionRate)
			}

			// 4. Update session: switch to coach 2, assign admin, edit time to 16:00 - 18:00, update session rate
			updatedRate := 400.0
			loaded.CoachID = &coach2.ID
			loaded.AdminID = &adminUser.ID
			loaded.StartTime = "16:00"
			loaded.EndTime = "18:00"
			loaded.TrainingType = models.TrainingPoomsae
			loaded.Notes = "Updated poomsae class"
			loaded.SessionRate = &updatedRate
			if err := store.UpdateSession(loaded); err != nil {
				t.Fatalf("UpdateSession failed: %v", err)
			}

			// Verify updated session
			updated, err := store.GetSessionByID(sess.ID)
			if err != nil {
				t.Fatalf("GetSessionByID failed after update: %v", err)
			}
			if updated.CoachName != "Coach Beta" {
				t.Errorf("expected Coach Beta, got %s", updated.CoachName)
			}
			if updated.AdminName != "Admin Sarah" {
				t.Errorf("expected Admin Sarah, got %s", updated.AdminName)
			}
			if updated.StartTime != "16:00" || updated.EndTime != "18:00" {
				t.Errorf("expected 16:00 - 18:00, got %s - %s", updated.StartTime, updated.EndTime)
			}
			if updated.TrainingType != models.TrainingPoomsae {
				t.Errorf("expected Poomsae, got %s", updated.TrainingType)
			}
			if updated.SessionRate == nil || *updated.SessionRate != 400.0 {
				t.Errorf("expected SessionRate 400.0, got %v", updated.SessionRate)
			}

			// 5. Test deletion with package refund
			// Create student with package
			st := &models.Student{
				ID:          uuid.New(),
				FullName:    "Timmy Test",
				DOB:         time.Now().AddDate(-12, 0, 0),
				CurrentBelt: "Yellow",
				IsActive:    true,
				CreatedAt:   time.Now(),
			}
			_ = store.CreateStudent(st)
			tpl := &models.PackageTemplate{
				ID:           uuid.New(),
				Title:        "10 Pass",
				SessionCount: intPtr(10),
				ValidityDays: 60,
				Price:        1000,
				IsActive:     true,
			}
			_ = store.CreatePackageTemplate(tpl)
			sp := &models.StudentPackage{
				ID:                uuid.New(),
				StudentID:         st.ID,
				TemplateID:        tpl.ID,
				TotalSessions:     intPtr(10),
				RemainingSessions: intPtr(9), // 1 credit deducted
				ExpiryDate:        time.Now().AddDate(0, 1, 0),
				PaymentStatus:     "paid",
			}
			_ = store.AssignPackage(sp)

			// Record attendance for this session
			att, err := store.CheckInStudent(sess.ID, st.ID, &sp.ID, nil)
			if err != nil {
				t.Fatalf("CheckInStudent failed: %v", err)
			}
			if att == nil {
				t.Fatalf("expected attendance record")
			}

			// 6. Delete session -> should refund the credit and purge attendance
			if err := store.DeleteSession(sess.ID); err != nil {
				t.Fatalf("DeleteSession failed: %v", err)
			}

			// Verify session is deleted
			if _, err := store.GetSessionByID(sess.ID); err != repository.ErrNotFound {
				t.Errorf("expected ErrNotFound for deleted session, got: %v", err)
			}

			// Verify credit is refunded from 9 back to 10
			pkgs, _ := store.GetStudentPackages(st.ID)
			if len(pkgs) == 0 || *pkgs[0].RemainingSessions != 10 {
				t.Errorf("expected remaining sessions refunded to 10, got: %v", pkgs)
			}

			// Verify attendance records are gone
			sAtts, _ := store.GetSessionAttendances(sess.ID)
			if len(sAtts) != 0 {
				t.Errorf("expected 0 attendances for deleted session, got: %d", len(sAtts))
			}

			// Verify deleting again returns ErrNotFound
			if err := store.DeleteSession(sess.ID); err != repository.ErrNotFound {
				t.Errorf("expected ErrNotFound on second DeleteSession, got: %v", err)
			}
		})
	}
}

func TestSQLStore_Locations(t *testing.T) {
	dbFile := "test_locations.db"
	_ = os.Remove(dbFile)
	defer os.Remove(dbFile)

	store, _, err := repository.InitDatabase(dbFile)
	if err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}

	// 1. Create Location
	initialRate := 275.50
	loc := &models.Location{
		ID:        uuid.New(),
		Name:      "BGC Branch",
		Pin:       "https://maps.google.com/?q=BGC+Branch",
		FixedRate: &initialRate,
		CreatedAt: time.Now(),
	}
	if err := store.CreateLocation(loc); err != nil {
		t.Fatalf("CreateLocation failed: %v", err)
	}

	// 2. Get Location By ID
	fetched, err := store.GetLocationByID(loc.ID)
	if err != nil {
		t.Fatalf("GetLocationByID failed: %v", err)
	}
	if fetched.Name != "BGC Branch" || fetched.Pin != loc.Pin {
		t.Errorf("expected fetched location to match, got name: %s, pin: %s", fetched.Name, fetched.Pin)
	}
	if fetched.FixedRate == nil || *fetched.FixedRate != 275.50 {
		t.Errorf("expected fetched location fixed_rate 275.50, got %v", fetched.FixedRate)
	}

	// 3. GetAllLocations
	locs, err := store.GetAllLocations()
	if err != nil {
		t.Fatalf("GetAllLocations failed: %v", err)
	}
	found := false
	for _, l := range locs {
		if l.ID == loc.ID {
			found = true
			if l.FixedRate == nil || *l.FixedRate != 275.50 {
				t.Errorf("expected GetAllLocations item fixed_rate 275.50, got %v", l.FixedRate)
			}
			break
		}
	}
	if !found {
		t.Errorf("expected to find created location in GetAllLocations")
	}

	// 4. Update Location
	updatedRate := 320.00
	loc.Name = "BGC High Street Dojang"
	loc.Pin = "https://maps.google.com/?q=BGC+High+Street"
	loc.FixedRate = &updatedRate
	if err := store.UpdateLocation(loc); err != nil {
		t.Fatalf("UpdateLocation failed: %v", err)
	}
	updated, _ := store.GetLocationByID(loc.ID)
	if updated.Name != "BGC High Street Dojang" {
		t.Errorf("expected updated name 'BGC High Street Dojang', got: %s", updated.Name)
	}
	if updated.FixedRate == nil || *updated.FixedRate != 320.00 {
		t.Errorf("expected updated location fixed_rate 320.00, got %v", updated.FixedRate)
	}

	// 5. Link Session to Location
	coaches, _ := store.GetAllCoaches()
	if len(coaches) == 0 {
		t.Fatalf("expected at least 1 coach")
	}
	sess := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  time.Now(),
		StartTime:    "18:00",
		EndTime:      "19:30",
		CoachID:      &coaches[0].ID,
		LocationID:   &loc.ID,
		TrainingType: models.TrainingSparring,
		Notes:        "Sparring at BGC",
	}
	if err := store.CreateSession(sess); err != nil {
		t.Fatalf("CreateSession with location failed: %v", err)
	}

	// Verify session retrieves location name and pin
	sessFetched, err := store.GetSessionByID(sess.ID)
	if err != nil {
		t.Fatalf("GetSessionByID failed: %v", err)
	}
	if sessFetched.LocationID == nil || *sessFetched.LocationID != loc.ID {
		t.Errorf("expected session location ID %v, got %v", loc.ID, sessFetched.LocationID)
	}
	if sessFetched.LocationName != "BGC High Street Dojang" {
		t.Errorf("expected session location name 'BGC High Street Dojang', got: %s", sessFetched.LocationName)
	}

	// 6. Filter Sessions by Location
	filtered, err := store.GetSessions(repository.SessionFilter{LocationID: &loc.ID})
	if err != nil {
		t.Fatalf("GetSessions by LocationID failed: %v", err)
	}
	if len(filtered) != 1 || filtered[0].ID != sess.ID {
		t.Errorf("expected 1 session for location %v, got %d", loc.ID, len(filtered))
	}

	// 7. Check-in Student and verify attendance is linked to Location
	students, _ := store.GetAllStudents()
	if len(students) == 0 {
		t.Fatalf("expected at least 1 student")
	}
	att, err := store.CheckInStudent(sess.ID, students[0].ID, nil, nil)
	if err != nil {
		t.Fatalf("CheckInStudent failed: %v", err)
	}
	if att.LocationID == nil || *att.LocationID != loc.ID {
		t.Errorf("expected attendance location ID %v, got %v", loc.ID, att.LocationID)
	}
	if att.LocationName != "BGC High Street Dojang" {
		t.Errorf("expected attendance location name 'BGC High Street Dojang', got: %s", att.LocationName)
	}

	// Verify GetSessionAttendances returns location
	sessionAtts, err := store.GetSessionAttendances(sess.ID)
	if err != nil || len(sessionAtts) == 0 {
		t.Fatalf("GetSessionAttendances failed: %v", err)
	}
	if sessionAtts[0].LocationName != "BGC High Street Dojang" {
		t.Errorf("expected session attendance location name 'BGC High Street Dojang', got: %s", sessionAtts[0].LocationName)
	}

	// 8. Delete Location
	otherLoc := &models.Location{
		ID:   uuid.New(),
		Name: "Temporary Venue",
		Pin:  "https://maps.google.com",
	}
	_ = store.CreateLocation(otherLoc)
	if err := store.DeleteLocation(otherLoc.ID); err != nil {
		t.Fatalf("DeleteLocation failed: %v", err)
	}
	if _, err := store.GetLocationByID(otherLoc.ID); err != repository.ErrNotFound {
		t.Errorf("expected ErrNotFound after DeleteLocation, got: %v", err)
	}
}

func TestStore_Attendance_SessionRate(t *testing.T) {
	stores := map[string]repository.RepositoryStore{}
	memStore := repository.NewMemoryStore()
	stores["MemoryStore"] = memStore

	dbFile := filepath.Join(t.TempDir(), "test_attendance_rate.db")
	sqlStore, _, err := repository.InitDatabase(dbFile)
	if err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}
	defer func() {
		if s, ok := sqlStore.(*repository.SQLStore); ok {
			_ = s.Close()
		}
	}()
	stores["SQLStore"] = sqlStore

	for name, store := range stores {
		t.Run(name, func(t *testing.T) {
			student := &models.Student{
				ID:                uuid.New(),
				FullName:          "School Student",
				DOB:               time.Now().AddDate(-12, 0, 0),
				CurrentBelt:       models.BeltWhite,
				LastPromotionDate: time.Now(),
				EmergencyName:     "Parent",
				EmergencyPhone:    "09171234567",
				EmergencyRelation: "Mother",
				IsActive:          true,
				CreatedAt:         time.Now(),
			}
			_ = store.CreateStudent(student)

			coach := &models.Coach{
				ID:        uuid.New(),
				FullName:  "Coach Rate",
				Email:     "coach_" + name + "@example.com",
				Phone:     "09170001111",
				BeltRank:  "4th Dan Black",
				IsActive:  true,
				CreatedAt: time.Now(),
			}
			_ = store.CreateCoach(coach)

			sess := &models.TrainingSession{
				ID:           uuid.New(),
				SessionDate:  time.Now(),
				StartTime:    "16:00",
				EndTime:      "17:30",
				CoachID:      &coach.ID,
				TrainingType: models.TrainingPoomsae,
			}
			_ = store.CreateSession(sess)

			rate := 200.00
			att, err := store.CheckInStudent(sess.ID, student.ID, nil, &rate)
			if err != nil {
				t.Fatalf("CheckInStudent with sessionRate failed: %v", err)
			}
			if att.SessionRate == nil || *att.SessionRate != 200.00 {
				t.Fatalf("expected SessionRate 200.00, got %v", att.SessionRate)
			}
			if att.SessionRateVal() != 200.00 {
				t.Errorf("expected SessionRateVal() 200.00, got %f", att.SessionRateVal())
			}

			// Verify GetSessionAttendances returns SessionRate
			sessAtts, err := store.GetSessionAttendances(sess.ID)
			if err != nil || len(sessAtts) == 0 {
				t.Fatalf("GetSessionAttendances failed: %v", err)
			}
			if sessAtts[0].SessionRate == nil || *sessAtts[0].SessionRate != 200.00 {
				t.Errorf("expected GetSessionAttendances to retain SessionRate 200.00, got %v", sessAtts[0].SessionRate)
			}

			// Verify GetStudentAttendances returns SessionRate
			stAtts, err := store.GetStudentAttendances(student.ID)
			if err != nil || len(stAtts) == 0 {
				t.Fatalf("GetStudentAttendances failed: %v", err)
			}
			if stAtts[0].SessionRate == nil || *stAtts[0].SessionRate != 200.00 {
				t.Errorf("expected GetStudentAttendances to retain SessionRate 200.00, got %v", stAtts[0].SessionRate)
			}
		})
	}
}

func TestSQLStore_OptionalCoachID(t *testing.T) {
	stores := map[string]repository.RepositoryStore{}
	stores["MemoryStore"] = repository.NewMemoryStore()

	dbFile := filepath.Join(t.TempDir(), "test_optional_coach.db")
	sqlStore, _, err := repository.InitDatabase(dbFile)
	if err != nil {
		t.Fatalf("InitDatabase failed: %v", err)
	}
	defer func() {
		if s, ok := sqlStore.(*repository.SQLStore); ok {
			_ = s.Close()
		}
	}()
	stores["SQLStore"] = sqlStore

	for name, store := range stores {
		t.Run(name, func(t *testing.T) {
			coach := &models.Coach{
				ID:        uuid.New(),
				FullName:  "Coach Optional " + name,
				Email:     "coach.optional." + name + "@seriestkd.com",
				Phone:     "09179998888",
				BeltRank:  "3rd Dan Black",
				IsActive:  true,
				CreatedAt: time.Now(),
			}
			_ = store.CreateCoach(coach)

			// 1. Create session without lead coach (CoachID: nil)
			sess := &models.TrainingSession{
				ID:           uuid.New(),
				SessionDate:  time.Now(),
				StartTime:    "10:00",
				EndTime:      "11:30",
				CoachID:      nil,
				TrainingType: models.TrainingSparring,
				Notes:        "Session without an assigned coach",
			}
			if err := store.CreateSession(sess); err != nil {
				t.Fatalf("CreateSession with nil CoachID failed: %v", err)
			}

			// 2. Load session: CoachID should be nil, CoachName should be empty
			loaded, err := store.GetSessionByID(sess.ID)
			if err != nil {
				t.Fatalf("GetSessionByID failed: %v", err)
			}
			if loaded.CoachID != nil {
				t.Errorf("expected nil CoachID, got %v", loaded.CoachID)
			}
			if loaded.CoachName != "" {
				t.Errorf("expected empty CoachName, got %s", loaded.CoachName)
			}

			// 3. Filter with coach must not include it
			filteredByCoach, err := store.GetSessions(repository.SessionFilter{CoachID: &coach.ID})
			if err != nil {
				t.Fatalf("GetSessions with CoachID filter failed: %v", err)
			}
			for _, s := range filteredByCoach {
				if s.ID == sess.ID {
					t.Errorf("unassigned session should not appear when filtered by coach")
				}
			}

			// 4. Update session to assign a coach
			loaded.CoachID = &coach.ID
			if err := store.UpdateSession(loaded); err != nil {
				t.Fatalf("UpdateSession assigning coach failed: %v", err)
			}
			updatedWithCoach, err := store.GetSessionByID(sess.ID)
			if err != nil {
				t.Fatalf("GetSessionByID after assigning coach failed: %v", err)
			}
			if updatedWithCoach.CoachID == nil || *updatedWithCoach.CoachID != coach.ID {
				t.Errorf("expected CoachID %v, got %v", coach.ID, updatedWithCoach.CoachID)
			}
			if updatedWithCoach.CoachName != coach.FullName {
				t.Errorf("expected CoachName %s, got %s", coach.FullName, updatedWithCoach.CoachName)
			}

			// 5. Update session back to unassigned coach (nil)
			updatedWithCoach.CoachID = nil
			if err := store.UpdateSession(updatedWithCoach); err != nil {
				t.Fatalf("UpdateSession unassigning coach failed: %v", err)
			}
			clearedSess, err := store.GetSessionByID(sess.ID)
			if err != nil {
				t.Fatalf("GetSessionByID after unassigning coach failed: %v", err)
			}
			if clearedSess.CoachID != nil {
				t.Errorf("expected nil CoachID after unassigning, got %v", clearedSess.CoachID)
			}
			if clearedSess.CoachName != "" {
				t.Errorf("expected empty CoachName after unassigning, got %s", clearedSess.CoachName)
			}
		})
	}
}





