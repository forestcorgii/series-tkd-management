package services_test

import (
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

