package models_test

import (
	"testing"
	"time"

	"series-tkd-management/internal/models"
)

func TestUser_PasswordHashing(t *testing.T) {
	u := &models.User{
		Email: "test@seriestkd.com",
		Role:  models.RoleStudent,
	}

	rawPass := "secretPassword123"
	if err := u.SetPassword(rawPass); err != nil {
		t.Fatalf("failed to set password: %v", err)
	}

	if u.PasswordHash == "" || u.PasswordHash == rawPass {
		t.Fatalf("expected hashed password, got %s", u.PasswordHash)
	}

	if !u.CheckPassword(rawPass) {
		t.Errorf("expected CheckPassword to return true for correct password")
	}

	if u.CheckPassword("wrongpassword") {
		t.Errorf("expected CheckPassword to return false for incorrect password")
	}
}

func TestUser_RolePermissions(t *testing.T) {
	admin := &models.User{Role: models.RoleAdmin}
	coach := &models.User{Role: models.RoleCoach}
	student := &models.User{Role: models.RoleStudent}
	opManager := &models.User{Role: models.RoleOperationManager}

	if !opManager.IsOperationManager() || opManager.IsAdmin() || opManager.IsCoach() || opManager.IsStudent() {
		t.Errorf("operation manager role check failed")
	}
	if !admin.IsAdmin() || admin.IsCoach() || admin.IsStudent() || admin.IsOperationManager() {
		t.Errorf("admin role check failed")
	}
	if !coach.IsCoach() || coach.IsAdmin() || coach.IsStudent() || coach.IsOperationManager() {
		t.Errorf("coach role check failed")
	}
	if !student.IsStudent() || student.IsAdmin() || student.IsCoach() || student.IsOperationManager() {
		t.Errorf("student role check failed")
	}

	if !opManager.HasRole(models.RoleOperationManager) {
		t.Errorf("expected opManager to have RoleOperationManager")
	}
	if !admin.HasRole(models.RoleAdmin, models.RoleCoach) {
		t.Errorf("expected admin to have RoleAdmin")
	}
	if !coach.HasRole(models.RoleCoach) {
		t.Errorf("expected coach to have RoleCoach")
	}
	if coach.HasRole(models.RoleAdmin) {
		t.Errorf("coach should not have RoleAdmin")
	}
}

func TestUser_CustomRolesAndPermissions(t *testing.T) {
	defaultRoles := models.DefaultStaffRoles()
	var coachRole, adminRole, managerRole *models.StaffRole
	for _, r := range defaultRoles {
		switch r.Slug {
		case "coach":
			coachRole = r
		case "admin":
			adminRole = r
		case "manager":
			managerRole = r
		}
	}

	if coachRole == nil || adminRole == nil || managerRole == nil {
		t.Fatalf("expected 3 default roles to be pre-created")
	}

	// 1. Staff user assigned pre-created Coach role
	coachUser := &models.User{
		Role:  models.RoleStaff,
		Roles: []*models.StaffRole{coachRole},
	}
	if !coachUser.IsStaff() {
		t.Errorf("expected coachUser to be staff")
	}
	if coachUser.IsStudent() {
		t.Errorf("expected coachUser NOT to be student")
	}
	if !coachUser.HasPermission(models.PermSessionsRead) {
		t.Errorf("expected coachUser to have sessions:read permission")
	}
	if !coachUser.HasPermission(models.PermEvaluationsCreate) {
		t.Errorf("expected coachUser to have evaluations:create permission")
	}
	if coachUser.HasPermission(models.PermStudentsDelete) {
		t.Errorf("coachUser should not have students:delete permission")
	}
	if !coachUser.IsCoach() {
		t.Errorf("expected coachUser.IsCoach() to be true based on assigned coach role")
	}

	// 2. Staff user assigned pre-created Admin role
	adminUser := &models.User{
		Role:  models.RoleStaff,
		Roles: []*models.StaffRole{adminRole},
	}
	if !adminUser.IsStaff() {
		t.Errorf("expected adminUser to be staff")
	}
	if !adminUser.HasPermission(models.PermStudentsCreate) {
		t.Errorf("expected adminUser to have students:create permission")
	}
	if !adminUser.HasPermission(models.PermPackagesAssign) {
		t.Errorf("expected adminUser to have packages:assign permission")
	}
	if adminUser.HasPermission(models.PermStudentsDelete) {
		t.Errorf("adminUser should not have students:delete permission")
	}
	if !adminUser.IsAdmin() {
		t.Errorf("expected adminUser.IsAdmin() to be true")
	}

	// 3. Staff user assigned pre-created Manager role
	managerUser := &models.User{
		Role:  models.RoleStaff,
		Roles: []*models.StaffRole{managerRole},
	}
	if !managerUser.IsStaff() {
		t.Errorf("expected managerUser to be staff")
	}
	if !managerUser.HasPermission(models.PermStudentsDelete) {
		t.Errorf("expected managerUser to have wildcard access covering students:delete")
	}
	if !managerUser.HasPermission(models.PermRolesManage) {
		t.Errorf("expected managerUser to have roles:manage permission")
	}
	if !managerUser.IsOperationManager() {
		t.Errorf("expected managerUser.IsOperationManager() to be true")
	}

	// 4. Custom Role: "Front Desk Assistant" with custom permissions
	customRole := &models.StaffRole{
		Name:        "Front Desk Assistant",
		Slug:        "front-desk",
		Permissions: []string{models.PermSessionsRead, models.PermAttendanceCheckin},
	}
	if err := customRole.Validate(); err != nil {
		t.Fatalf("custom role validation failed: %v", err)
	}

	frontDeskStaff := &models.User{
		Role:  models.RoleStaff,
		Roles: []*models.StaffRole{customRole},
	}
	if !frontDeskStaff.HasPermission(models.PermAttendanceCheckin) {
		t.Errorf("expected frontDeskStaff to have attendance:checkin")
	}
	if frontDeskStaff.HasPermission(models.PermSessionsCreate) {
		t.Errorf("frontDeskStaff should not have sessions:create")
	}

	// 5. Student User
	studentUser := &models.User{
		Role: models.RoleStudent,
	}
	if !studentUser.IsStudent() {
		t.Errorf("expected studentUser to be student")
	}
	if studentUser.IsStaff() {
		t.Errorf("expected studentUser NOT to be staff")
	}
	if studentUser.HasPermission(models.PermSessionsRead) {
		t.Errorf("studentUser should never have staff permissions")
	}
}


func TestStudent_NextBeltProgression(t *testing.T) {
	s := &models.Student{CurrentBelt: models.BeltWhite}
	if next := s.NextBelt(); next != models.BeltLowYellow {
		t.Errorf("expected Low Yellow after White, got %s", next)
	}

	chain := []struct {
		current  models.BeltRank
		expected models.BeltRank
	}{
		{models.BeltWhite, models.BeltLowYellow},
		{models.BeltLowYellow, models.BeltHighYellow},
		{models.BeltHighYellow, models.BeltLowBlue},
		{models.BeltLowBlue, models.BeltHighBlue},
		{models.BeltHighBlue, models.BeltLowRed},
		{models.BeltLowRed, models.BeltHighRed},
		{models.BeltHighRed, models.BeltLowBrown},
		{models.BeltLowBrown, models.BeltHighBrown},
		{models.BeltHighBrown, models.BeltBlack1stDan},
		{models.BeltBlack1stDan, models.BeltBlack2ndDan},
		{models.BeltBlack2ndDan, models.BeltBlack3rdDan},
		{models.BeltBlack3rdDan, models.BeltBlack3rdDan},
	}

	for _, step := range chain {
		s.CurrentBelt = step.current
		if next := s.NextBelt(); next != step.expected {
			t.Errorf("expected %s after %s, got %s", step.expected, step.current, next)
		}
	}
}

func TestPasswordResetToken_IsValid(t *testing.T) {
	now := time.Now()

	// Valid token
	validToken := &models.PasswordResetToken{
		ExpiresAt: now.Add(1 * time.Hour),
		UsedAt:    nil,
	}
	if !validToken.IsValid() {
		t.Errorf("expected valid token to return true")
	}

	// Expired token
	expiredToken := &models.PasswordResetToken{
		ExpiresAt: now.Add(-1 * time.Minute),
		UsedAt:    nil,
	}
	if expiredToken.IsValid() {
		t.Errorf("expected expired token to return false")
	}

	// Already used token
	usedTime := now.Add(-5 * time.Minute)
	usedToken := &models.PasswordResetToken{
		ExpiresAt: now.Add(1 * time.Hour),
		UsedAt:    &usedTime,
	}
	if usedToken.IsValid() {
		t.Errorf("expected used token to return false")
	}

	// Nil token
	var nilToken *models.PasswordResetToken
	if nilToken.IsValid() {
		t.Errorf("expected nil token to return false")
	}
}

