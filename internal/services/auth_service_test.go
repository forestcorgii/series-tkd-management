package services_test

import (
	"errors"
	"testing"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
	"series-tkd-management/internal/services"
)

func TestAuthService_LoginAndSessionLifecycle(t *testing.T) {
	store := repository.NewMemoryStore()
	authSvc := services.NewAuthService(store)

	// 1. Success Login for seeded admin
	user, token, err := authSvc.Login("admin@seriestkd.com", "admin123")
	if err != nil {
		t.Fatalf("expected successful login, got err: %v", err)
	}
	if user == nil || user.Role != models.RoleAdmin {
		t.Fatalf("expected admin user, got %v", user)
	}
	if token == "" {
		t.Fatalf("expected non-empty session token")
	}

	// 2. Validate Session
	validUser, err := authSvc.ValidateSession(token)
	if err != nil {
		t.Fatalf("expected valid session, got %v", err)
	}
	if validUser.ID != user.ID {
		t.Errorf("expected session user ID %s, got %s", user.ID, validUser.ID)
	}

	// 3. Failed Login (Wrong password)
	_, _, err = authSvc.Login("admin@seriestkd.com", "wrongpassword")
	if err != services.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}

	// 4. Failed Login (Non-existent email)
	_, _, err = authSvc.Login("nonexistent@seriestkd.com", "any")
	if err != services.ErrInvalidCredentials {
		t.Errorf("expected ErrInvalidCredentials, got %v", err)
	}

	// 5. Logout
	if err := authSvc.Logout(token); err != nil {
		t.Fatalf("logout failed: %v", err)
	}

	// 6. Validate Session after Logout -> should fail
	_, err = authSvc.ValidateSession(token)
	if err == nil {
		t.Errorf("expected error validating expired/deleted session token")
	}
}

func TestAuthService_RegisterUser(t *testing.T) {
	store := repository.NewMemoryStore()
	authSvc := services.NewAuthService(store)

	newUser, err := authSvc.RegisterUser("newcoach@seriestkd.com", "pass12345", models.RoleCoach, nil, nil)
	if err != nil {
		t.Fatalf("RegisterUser failed: %v", err)
	}
	if newUser.Role != models.RoleCoach {
		t.Errorf("expected role COACH, got %s", newUser.Role)
	}

	// Try registering with same email -> must fail
	_, err = authSvc.RegisterUser("newcoach@seriestkd.com", "other", models.RoleStudent, nil, nil)
	if err != services.ErrEmailAlreadyExists {
		t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestAuthService_CoachDeactivationAndDeletionLoginBlocked(t *testing.T) {
	store := repository.NewMemoryStore()
	authSvc := services.NewAuthService(store)

	// Coach Ji-Woo Park is seeded with email jiwoo.park@seriestkd.com, password coach123
	coachUser, token, err := authSvc.Login("jiwoo.park@seriestkd.com", "coach123")
	if err != nil {
		t.Fatalf("expected successful coach login, got: %v", err)
	}
	if coachUser == nil || coachUser.Role != models.RoleCoach {
		t.Fatalf("expected coach role, got %v", coachUser)
	}

	// Session is valid initially
	validatedUser, err := authSvc.ValidateSession(token)
	if err != nil || validatedUser.ID != coachUser.ID {
		t.Fatalf("expected valid session, got err: %v", err)
	}

	// Deactivate Coach
	if err := store.ToggleCoachActive(*coachUser.CoachID, false); err != nil {
		t.Fatalf("failed to deactivate coach: %v", err)
	}

	// 1. Existing session should now be rejected as inactive or purged
	_, err = authSvc.ValidateSession(token)
	if err == nil {
		t.Errorf("expected session to be invalid after coach deactivation on ValidateSession, got nil err")
	}


	// 2. New login attempt must fail with ErrUserInactive
	_, _, err = authSvc.Login("jiwoo.park@seriestkd.com", "coach123")
	if err != services.ErrUserInactive {
		t.Errorf("expected ErrUserInactive after coach deactivation on Login, got: %v", err)
	}

	// 3. Reactivate Coach -> login should succeed again
	if err := store.ToggleCoachActive(*coachUser.CoachID, true); err != nil {
		t.Fatalf("failed to reactivate coach: %v", err)
	}
	_, newToken, err := authSvc.Login("jiwoo.park@seriestkd.com", "coach123")
	if err != nil || newToken == "" {
		t.Fatalf("expected successful login after reactivation, got err: %v", err)
	}
}

func TestAuthService_UsernameLogin(t *testing.T) {
	store := repository.NewMemoryStore()
	authSvc := services.NewAuthService(store)

	// 1. Login with username (exact)
	user, token, err := authSvc.Login("admin", "admin123")
	if err != nil {
		t.Fatalf("expected successful login with username, got: %v", err)
	}
	if user.Role != models.RoleAdmin {
		t.Errorf("expected role ADMIN, got %s", user.Role)
	}
	if token == "" {
		t.Errorf("expected non-empty token")
	}

	// 2. Login with username (case-insensitive)
	user, _, err = authSvc.Login("ADMIN", "admin123")
	if err != nil {
		t.Fatalf("expected case-insensitive username login to succeed, got: %v", err)
	}
	if user.Username != "admin" {
		t.Errorf("expected username admin, got %s", user.Username)
	}

	// 3. Login with student username
	studentUser, _, err := authSvc.Login("alex.vance", "student123")
	if err != nil {
		t.Fatalf("expected student login by username to succeed: %v", err)
	}
	if studentUser.Role != models.RoleStudent {
		t.Errorf("expected STUDENT role, got %s", studentUser.Role)
	}

	// 4. Register new user with username
	newStaff, err := authSvc.RegisterUser("desk@seriestkd.com", "deskpass123", models.RoleAdmin, nil, nil, "deskstaff")
	if err != nil {
		t.Fatalf("failed to register user with username: %v", err)
	}
	if newStaff.Username != "deskstaff" {
		t.Errorf("expected username deskstaff, got %s", newStaff.Username)
	}

	// 5. Login new user by username
	loggedNew, _, err := authSvc.Login("deskstaff", "deskpass123")
	if err != nil || loggedNew.ID != newStaff.ID {
		t.Fatalf("failed to login newly registered user by username: %v", err)
	}

	// 6. Duplicate username registration rejected
	_, err = authSvc.RegisterUser("other@seriestkd.com", "deskpass123", models.RoleAdmin, nil, nil, "deskstaff")
	if err != services.ErrUsernameAlreadyExists {
		t.Errorf("expected ErrUsernameAlreadyExists, got %v", err)
	}
}

func TestAuthService_ForgotPasswordAndReset(t *testing.T) {
	store := repository.NewMemoryStore()
	authSvc := services.NewAuthService(store)

	// 1. Request password reset using email
	tokenRec, err := authSvc.RequestPasswordReset("alex.vance@seriestkd.com")
	if err != nil {
		t.Fatalf("failed to request password reset by email: %v", err)
	}
	if tokenRec == nil || tokenRec.Token == "" {
		t.Fatalf("expected valid token record")
	}

	// 2. Request password reset using username
	tokenRecUser, err := authSvc.RequestPasswordReset("alex.vance")
	if err != nil {
		t.Fatalf("failed to request password reset by username: %v", err)
	}
	if tokenRecUser == nil || tokenRecUser.Token == "" {
		t.Fatalf("expected valid token record")
	}

	// 3. Non-existent account returns nil without error (anti-enumeration)
	unknownRec, err := authSvc.RequestPasswordReset("doesnotexist@nowhere.com")
	if err != nil {
		t.Errorf("expected nil error for unknown identifier (anti-enumeration), got: %v", err)
	}
	if unknownRec != nil {
		t.Errorf("expected nil token for unknown identifier")
	}

	// 4. Validate token
	validatedToken, targetUser, err := authSvc.ValidatePasswordResetToken(tokenRec.Token)
	if err != nil {
		t.Fatalf("expected token to be valid: %v", err)
	}
	if validatedToken.Token != tokenRec.Token {
		t.Errorf("token mismatch")
	}
	if targetUser.Email != "alex.vance@seriestkd.com" {
		t.Errorf("user mismatch, got email: %s", targetUser.Email)
	}

	// 5. Reset password with new password
	newPassword := "brandNewPassword2026"
	updatedUser, err := authSvc.ResetPassword(tokenRec.Token, newPassword)
	if err != nil {
		t.Fatalf("expected password reset to succeed: %v", err)
	}
	if updatedUser.ID != targetUser.ID {
		t.Errorf("user ID mismatch")
	}

	// 6. Login with old password must fail
	_, _, err = authSvc.Login("alex.vance", "student123")
	if err != services.ErrInvalidCredentials {
		t.Errorf("expected old password to fail with ErrInvalidCredentials, got %v", err)
	}

	// 7. Login with new password must succeed
	loginUser, sessionToken, err := authSvc.Login("alex.vance", newPassword)
	if err != nil {
		t.Fatalf("expected login with new password to succeed, got %v", err)
	}
	if loginUser.ID != targetUser.ID || sessionToken == "" {
		t.Errorf("login response invalid")
	}

	// 8. Attempting to reuse the reset token must fail
	_, err = authSvc.ResetPassword(tokenRec.Token, "yetAnotherPassword")
	if err != services.ErrInvalidResetToken {
		t.Errorf("expected ErrInvalidResetToken on token reuse, got %v", err)
	}
}

func TestAuthService_CoachRegistrationAndApproval(t *testing.T) {
	store := repository.NewMemoryStore()
	authSvc := services.NewAuthService(store)

	// 1. Register pending coach
	coach, user, err := authSvc.RegisterPendingCoach(
		"Master Dae-Hyun Kim",
		"daehyun.kim@seriestkd.com",
		"daehyun.kim",
		"blackbelt2026",
		"+63 917 555 1234",
		"5th Dan Master",
		[]string{"Poomsae", "Sparring"},
		true,
	)
	if err != nil {
		t.Fatalf("expected coach registration to succeed, got %v", err)
	}
	if coach.IsActive || user.IsActive {
		t.Fatalf("expected newly registered coach and user to be inactive pending approval")
	}
	if user.Role != models.RoleCoach {
		t.Errorf("expected RoleCoach, got %s", user.Role)
	}

	// 2. Coach login attempt before approval must be blocked with ErrAccountPendingApproval
	_, _, err = authSvc.Login("daehyun.kim", "blackbelt2026")
	if !errors.Is(err, services.ErrAccountPendingApproval) {
		t.Fatalf("expected ErrAccountPendingApproval before approval, got: %v", err)
	}

	// Also check login by email
	_, _, err = authSvc.Login("daehyun.kim@seriestkd.com", "blackbelt2026")
	if !errors.Is(err, services.ErrAccountPendingApproval) {
		t.Fatalf("expected ErrAccountPendingApproval by email before approval, got: %v", err)
	}

	// 3. Manager approves coach via ToggleCoachActive
	if err := store.ToggleCoachActive(coach.ID, true); err != nil {
		t.Fatalf("failed to approve coach: %v", err)
	}

	// Verify status after approval
	approvedCoach, _ := store.GetCoachByID(coach.ID)
	approvedUser, _ := store.GetUserByID(user.ID)
	if !approvedCoach.IsActive || !approvedUser.IsActive {
		t.Fatalf("expected coach and user to both be active after manager approval")
	}

	// 4. Coach logs in successfully after manager approval
	loginUser, token, err := authSvc.Login("daehyun.kim", "blackbelt2026")
	if err != nil {
		t.Fatalf("expected successful login after manager approval, got: %v", err)
	}
	if loginUser.ID != user.ID || token == "" {
		t.Errorf("invalid login response after approval")
	}
}

func TestAuthService_AdminRegistrationAndApproval(t *testing.T) {
	store := repository.NewMemoryStore()
	authSvc := services.NewAuthService(store)

	// 1. Register pending administrator
	user, err := authSvc.RegisterPendingAdmin(
		"Sarah Frontdesk",
		"sarah.desk@seriestkd.com",
		"sarah.desk",
		"deskpass2026",
	)
	if err != nil {
		t.Fatalf("expected admin registration to succeed, got %v", err)
	}
	if user.IsActive {
		t.Fatalf("expected newly registered admin to be inactive pending approval")
	}
	if user.Role != models.RoleAdmin {
		t.Errorf("expected RoleAdmin, got %s", user.Role)
	}

	// 2. Admin login attempt before approval must be blocked with ErrAccountPendingApproval
	_, _, err = authSvc.Login("sarah.desk", "deskpass2026")
	if !errors.Is(err, services.ErrAccountPendingApproval) {
		t.Fatalf("expected ErrAccountPendingApproval before approval, got: %v", err)
	}

	// 3. Duplicate email or username registration must be rejected
	_, err = authSvc.RegisterPendingAdmin("Duplicate", "sarah.desk@seriestkd.com", "otheruser", "password123")
	if !errors.Is(err, services.ErrEmailAlreadyExists) {
		t.Errorf("expected ErrEmailAlreadyExists, got %v", err)
	}
	_, err = authSvc.RegisterPendingAdmin("Duplicate", "other@seriestkd.com", "sarah.desk", "password123")
	if !errors.Is(err, services.ErrUsernameAlreadyExists) {
		t.Errorf("expected ErrUsernameAlreadyExists, got %v", err)
	}

	// 4. Manager approves admin via ToggleUserActive
	if err := store.ToggleUserActive(user.ID, true); err != nil {
		t.Fatalf("failed to approve admin: %v", err)
	}

	// 5. Admin logs in successfully after manager approval
	loginUser, token, err := authSvc.Login("sarah.desk", "deskpass2026")
	if err != nil {
		t.Fatalf("expected successful login after manager approval, got: %v", err)
	}
	if loginUser.ID != user.ID || token == "" {
		t.Errorf("invalid login response after approval")
	}
}



