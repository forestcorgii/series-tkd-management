package handlers_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

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

func TestHandleFourWeekPlan_CreationAndCheckIn(t *testing.T) {
	app, store := setupTestApp(t)

	// 1. Create a 4-week 4-class plan template
	formData := url.Values{
		"title":         {"4-Week Weekly Cadet Pass"},
		"description":   {"1 class per week strictly over 4 weeks"},
		"plan_type":     {"four_week"},
		"session_count": {"4"},
		"validity_days": {"28"},
		"price":         {"95.00"},
	}

	req := httptest.NewRequest(http.MethodPost, "/packages/templates", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()

	app.HandleCreatePackageTemplate(rec, req)
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect 303, got %d", rec.Code)
	}

	// Verify template created
	templates, _ := store.GetPackageTemplates()
	var created *models.PackageTemplate
	for _, tpl := range templates {
		if tpl.Title == "4-Week Weekly Cadet Pass" {
			created = tpl
			break
		}
	}
	if created == nil {
		t.Fatal("4-week package template was not created")
	}
	if !created.IsFourWeek() {
		t.Fatal("expected template to be IsFourWeek")
	}
	if created.ValidityDays != 28 {
		t.Fatalf("expected 28 validity days, got %d", created.ValidityDays)
	}
	if created.WeeklyCadence() != 1 {
		t.Fatalf("expected weekly cadence 1, got %d", created.WeeklyCadence())
	}

	// 2. Assign the 4-week package to a new student
	student := &models.Student{
		ID:                uuid.New(),
		FullName:          "Cadet Jordan",
		DOB:               time.Now().AddDate(-12, 0, 0),
		CurrentBelt:       models.BeltWhite,
		LastPromotionDate: time.Now(),
		EmergencyName:     "Parent Jordan",
		EmergencyPhone:    "555-0011",
		EmergencyRelation: "Parent",
		IsActive:          true,
	}
	_ = store.CreateStudent(student)

	assignData := url.Values{
		"student_id":     {student.ID.String()},
		"template_id":    {created.ID.String()},
		"payment_status": {"paid"},
	}
	assignReq := httptest.NewRequest(http.MethodPost, "/packages/assign", strings.NewReader(assignData.Encode()))
	assignReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	assignRec := httptest.NewRecorder()

	app.HandleAssignPackage(assignRec, assignReq)
	if assignRec.Code != http.StatusSeeOther {
		t.Fatalf("expected redirect 303, got %d", assignRec.Code)
	}

	// 3. Create two training sessions on the same day
	coaches, _ := store.GetAllCoaches()
	cID := coaches[0].ID
	sess1 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  time.Now(),
		StartTime:    "10:00",
		EndTime:      "11:00",
		CoachID:      &cID,
		TrainingType: models.TrainingSparring,
	}
	sess2 := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  time.Now(),
		StartTime:    "14:00",
		EndTime:      "15:00",
		CoachID:      &cID,
		TrainingType: models.TrainingPoomsae,
	}
	_ = store.CreateSession(sess1)
	_ = store.CreateSession(sess2)

	// Check-in 1: Should succeed
	checkin1Req := httptest.NewRequest(http.MethodPost, "/sessions/"+sess1.ID.String()+"/checkin/"+student.ID.String(), nil)
	checkin1Rec := httptest.NewRecorder()
	app.HandleCheckIn(checkin1Rec, checkin1Req)
	if checkin1Rec.Code != http.StatusOK {
		t.Fatalf("expected first check-in to succeed (200), got %d. Body: %s", checkin1Rec.Code, checkin1Rec.Body.String())
	}

	// Check-in 2 (same week, 2nd class for 1x/wk plan): Should be rejected with 402 and weekly limit message
	checkin2Req := httptest.NewRequest(http.MethodPost, "/sessions/"+sess2.ID.String()+"/checkin/"+student.ID.String(), nil)
	checkin2Rec := httptest.NewRecorder()
	app.HandleCheckIn(checkin2Rec, checkin2Req)
	if checkin2Rec.Code != http.StatusPaymentRequired {
		t.Fatalf("expected second check-in to be rejected with 402, got %d. Body: %s", checkin2Rec.Code, checkin2Rec.Body.String())
	}
	if !strings.Contains(checkin2Rec.Body.String(), "weekly limit") {
		t.Fatalf("expected rejection body to mention weekly limit, got: %s", checkin2Rec.Body.String())
	}

	// Check-in 2 with override=true: Should succeed
	overrideReq := httptest.NewRequest(http.MethodPost, "/sessions/"+sess2.ID.String()+"/checkin/"+student.ID.String()+"?override=true", nil)
	overrideRec := httptest.NewRecorder()
	app.HandleCheckIn(overrideRec, overrideReq)
	if overrideRec.Code != http.StatusOK {
		t.Fatalf("expected override check-in to succeed (200), got %d. Body: %s", overrideRec.Code, overrideRec.Body.String())
	}
}

func TestHandlePackages_CustomPriceRendering(t *testing.T) {
	app, store := setupTestApp(t)

	student := &models.Student{
		ID:          uuid.New(),
		FullName:    "Jin Kazama",
		DOB:         time.Now().AddDate(-16, 0, 0),
		CurrentBelt: models.BeltBlack1stDan,
		IsActive:    true,
	}
	_ = store.CreateStudent(student)

	ten := 10
	tpl := &models.PackageTemplate{
		ID:           uuid.New(),
		Title:        "Custom Rate Plan",
		SessionCount: &ten,
		ValidityDays: 60,
		Price:        1000.00,
		IsActive:     true,
	}
	_ = store.CreatePackageTemplate(tpl)

	customPrice := 1499.50
	sp := &models.StudentPackage{
		ID:                uuid.New(),
		StudentID:         student.ID,
		TemplateID:        tpl.ID,
		TemplateTitle:     tpl.Title,
		TotalSessions:     &ten,
		RemainingSessions: &ten,
		CustomPrice:       &customPrice,
		Notes:             "VIP corporate sibling discount",
		PurchaseDate:      time.Now(),
		ExpiryDate:        time.Now().AddDate(0, 0, 60),
		PaymentStatus:     "paid",
	}
	_ = store.AssignPackage(sp)

	req := httptest.NewRequest(http.MethodGet, "/packages", nil)
	rec := httptest.NewRecorder()
	app.HandlePackages(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	body := rec.Body.String()
	if strings.Contains(body, "%!f") {
		t.Errorf("body contains broken float pointer formatting artifact: %s", body)
	}
	if !strings.Contains(body, "₱1499.50") {
		t.Errorf("expected formatted currency ₱1499.50 in body")
	}
	if !strings.Contains(body, "VIP corporate sibling discount") {
		t.Errorf("expected notes in body")
	}

	// Also verify student detail view renders CustomPrice without %!f artifact
	sReq := httptest.NewRequest(http.MethodGet, "/students/"+student.ID.String(), nil)
	sRec := httptest.NewRecorder()
	app.HandleStudentDetail(sRec, sReq)
	if sRec.Code != http.StatusOK {
		t.Fatalf("expected student detail status 200, got %d", sRec.Code)
	}
	sBody := sRec.Body.String()
	if strings.Contains(sBody, "%!f") {
		t.Errorf("student detail contains broken float pointer formatting artifact: %s", sBody)
	}
	if !strings.Contains(sBody, "₱1499.50") {
		t.Errorf("expected formatted currency ₱1499.50 in student detail body")
	}
}

func TestHandleRevokeStudentPackage(t *testing.T) {
	app, store := setupTestApp(t)

	// 1. Create student and assign package
	student := &models.Student{
		ID:          uuid.New(),
		FullName:    "Revoke Test Student",
		DOB:         time.Now().AddDate(-12, 0, 0),
		CurrentBelt: models.BeltLowYellow,
		IsActive:    true,
	}
	_ = store.CreateStudent(student)

	tpl := &models.PackageTemplate{
		ID:           uuid.New(),
		Title:        "10-Class Test Pass",
		ValidityDays: 30,
		Price:        100.0,
		IsActive:     true,
	}
	_ = store.CreatePackageTemplate(tpl)

	tot := 10
	sp := &models.StudentPackage{
		ID:                uuid.New(),
		StudentID:         student.ID,
		TemplateID:        tpl.ID,
		TemplateTitle:     tpl.Title,
		PlanType:          models.PlanTypeStandard,
		TotalSessions:     &tot,
		RemainingSessions: &tot,
		PurchaseDate:      time.Now(),
		ExpiryDate:        time.Now().AddDate(0, 1, 0),
		PaymentStatus:     "paid",
	}
	_ = store.AssignPackage(sp)

	// 2. Non-manager is forbidden
	coachUser, _ := store.GetUserByEmail("jiwoo.park@seriestkd.com")
	ctxCoach := context.WithValue(context.Background(), handlers.UserContextKey, coachUser)
	reqCoach := httptest.NewRequest(http.MethodPost, "/packages/student-packages/"+sp.ID.String()+"/revoke", nil).WithContext(ctxCoach)
	reqCoach.SetPathValue("id", sp.ID.String())
	recCoach := httptest.NewRecorder()
	app.HandleRevokeStudentPackage(recCoach, reqCoach)

	if recCoach.Code != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for non-manager revoke, got %d", recCoach.Code)
	}

	// 3. Manager successfully revokes package
	managerUser, _ := store.GetUserByEmail("manager@seriestkd.com")
	ctxMgr := context.WithValue(context.Background(), handlers.UserContextKey, managerUser)
	reqMgr := httptest.NewRequest(http.MethodPost, "/packages/student-packages/"+sp.ID.String()+"/revoke", nil).WithContext(ctxMgr)
	reqMgr.SetPathValue("id", sp.ID.String())
	recMgr := httptest.NewRecorder()
	app.HandleRevokeStudentPackage(recMgr, reqMgr)

	if recMgr.Code != http.StatusSeeOther {
		t.Fatalf("expected 303 redirect for manager revoke, got %d", recMgr.Code)
	}

	// 4. Verify package is now revoked and invalid
	pkgs, _ := store.GetStudentPackages(student.ID)
	var revokedPkg *models.StudentPackage
	for _, p := range pkgs {
		if p.ID == sp.ID {
			revokedPkg = p
			break
		}
	}
	if revokedPkg == nil {
		t.Fatal("revoked package not found in student packages")
	}
	if revokedPkg.PaymentStatus != "revoked" {
		t.Errorf("expected PaymentStatus 'revoked', got %s", revokedPkg.PaymentStatus)
	}
	if revokedPkg.IsValidAt(time.Now()) {
		t.Errorf("expected revoked package to NOT be valid for check-in")
	}
}

func TestHandlePackages_PaymentStatusFilter(t *testing.T) {
	app, store := setupTestApp(t)

	// Clear seeded student packages? Or create distinctly named students so we can assert precisely
	tpl := &models.PackageTemplate{
		ID:           uuid.New(),
		Title:        "Filter Test Plan",
		ValidityDays: 30,
		Price:        100.0,
		IsActive:     true,
	}
	_ = store.CreatePackageTemplate(tpl)

	createPkgForStudent := func(name, paymentStatus string) {
		st := &models.Student{
			ID:          uuid.New(),
			FullName:    name,
			DOB:         time.Now().AddDate(-15, 0, 0),
			CurrentBelt: models.BeltWhite,
			IsActive:    true,
		}
		_ = store.CreateStudent(st)
		tot := 10
		sp := &models.StudentPackage{
			ID:                uuid.New(),
			StudentID:         st.ID,
			TemplateID:        tpl.ID,
			TemplateTitle:     tpl.Title,
			PlanType:          models.PlanTypeStandard,
			TotalSessions:     &tot,
			RemainingSessions: &tot,
			PurchaseDate:      time.Now(),
			ExpiryDate:        time.Now().AddDate(0, 1, 0),
			PaymentStatus:     paymentStatus,
		}
		_ = store.AssignPackage(sp)
	}

	createPkgForStudent("Paid Roster Student", "paid")
	createPkgForStudent("Pending Roster Student", "pending")
	createPkgForStudent("Unpaid Roster Student", "unpaid")
	createPkgForStudent("Revoked Roster Student", "revoked")

	extractRoster := func(html string) string {
		start := strings.Index(html, `id="package-roster-section"`)
		if start == -1 {
			t.Fatalf("missing #package-roster-section in response")
		}
		end := strings.Index(html[start:], `</table>`)
		if end == -1 {
			t.Fatalf("missing closing </table> in #package-roster-section")
		}
		return html[start : start+end]
	}

	// 1. Default filter (no query param) -> paid only
	reqDefault := httptest.NewRequest(http.MethodGet, "/packages", nil)
	recDefault := httptest.NewRecorder()
	app.HandlePackages(recDefault, reqDefault)
	if recDefault.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recDefault.Code)
	}
	rosterDefault := extractRoster(recDefault.Body.String())
	if !strings.Contains(rosterDefault, "Paid Roster Student") {
		t.Errorf("default roster should include Paid Roster Student")
	}
	if strings.Contains(rosterDefault, "Pending Roster Student") ||
		strings.Contains(rosterDefault, "Unpaid Roster Student") ||
		strings.Contains(rosterDefault, "Revoked Roster Student") {
		t.Errorf("default roster should exclude non-paid students")
	}
	if !strings.Contains(rosterDefault, `<option value="paid" selected>Paid Only</option>`) {
		t.Errorf("expected paid option to be selected by default")
	}

	// 2. Filter: pending (HTMX partial request)
	reqPending := httptest.NewRequest(http.MethodGet, "/packages?payment_status=pending", nil)
	reqPending.Header.Set("HX-Request", "true")
	recPending := httptest.NewRecorder()
	app.HandlePackages(recPending, reqPending)
	if recPending.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", recPending.Code)
	}
	bodyPending := recPending.Body.String()
	if strings.Contains(bodyPending, "<!DOCTYPE html>") {
		t.Errorf("expected HTMX partial response without full layout")
	}
	if !strings.Contains(bodyPending, "Pending Roster Student") {
		t.Errorf("pending filter should include Pending Roster Student")
	}
	if strings.Contains(bodyPending, "Paid Roster Student") ||
		strings.Contains(bodyPending, "Unpaid Roster Student") ||
		strings.Contains(bodyPending, "Revoked Roster Student") {
		t.Errorf("pending filter should exclude non-pending students")
	}

	// 3. Filter: unpaid
	reqUnpaid := httptest.NewRequest(http.MethodGet, "/packages?payment_status=unpaid", nil)
	recUnpaid := httptest.NewRecorder()
	app.HandlePackages(recUnpaid, reqUnpaid)
	rosterUnpaid := extractRoster(recUnpaid.Body.String())
	if !strings.Contains(rosterUnpaid, "Unpaid Roster Student") {
		t.Errorf("unpaid filter should include Unpaid Roster Student")
	}
	if strings.Contains(rosterUnpaid, "Paid Roster Student") || strings.Contains(rosterUnpaid, "Pending Roster Student") {
		t.Errorf("unpaid filter should exclude paid/pending students")
	}

	// 4. Filter: revoked
	reqRevoked := httptest.NewRequest(http.MethodGet, "/packages?payment_status=revoked", nil)
	recRevoked := httptest.NewRecorder()
	app.HandlePackages(recRevoked, reqRevoked)
	rosterRevoked := extractRoster(recRevoked.Body.String())
	if !strings.Contains(rosterRevoked, "Revoked Roster Student") {
		t.Errorf("revoked filter should include Revoked Roster Student")
	}
	if strings.Contains(rosterRevoked, "Paid Roster Student") {
		t.Errorf("revoked filter should exclude paid students")
	}

	// 5. Filter: all
	reqAll := httptest.NewRequest(http.MethodGet, "/packages?payment_status=all", nil)
	recAll := httptest.NewRecorder()
	app.HandlePackages(recAll, reqAll)
	rosterAll := extractRoster(recAll.Body.String())
	for _, name := range []string{"Paid Roster Student", "Pending Roster Student", "Unpaid Roster Student", "Revoked Roster Student"} {
		if !strings.Contains(rosterAll, name) {
			t.Errorf("all filter should include %s", name)
		}
	}

	// 6. Boosted HTMX navigation returns full page
	reqBoosted := httptest.NewRequest(http.MethodGet, "/packages", nil)
	reqBoosted.Header.Set("HX-Request", "true")
	reqBoosted.Header.Set("HX-Boosted", "true")
	recBoosted := httptest.NewRecorder()
	app.HandlePackages(recBoosted, reqBoosted)
	if !strings.Contains(recBoosted.Body.String(), "<!DOCTYPE html>") {
		t.Errorf("expected full page for HX-Boosted navigation")
	}

	// 7. Unknown payment_status falls back safely to paid
	reqUnknown := httptest.NewRequest(http.MethodGet, "/packages?payment_status=bogus", nil)
	recUnknown := httptest.NewRecorder()
	app.HandlePackages(recUnknown, reqUnknown)
	rosterUnknown := extractRoster(recUnknown.Body.String())
	if !strings.Contains(rosterUnknown, "Paid Roster Student") || strings.Contains(rosterUnknown, "Unpaid Roster Student") {
		t.Errorf("unknown payment_status should fall back to paid-only")
	}
}


