package models_test

import (
	"testing"

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

	if !admin.IsAdmin() || admin.IsCoach() || admin.IsStudent() {
		t.Errorf("admin role check failed")
	}
	if !coach.IsCoach() || coach.IsAdmin() || coach.IsStudent() {
		t.Errorf("coach role check failed")
	}
	if !student.IsStudent() || student.IsAdmin() || student.IsCoach() {
		t.Errorf("student role check failed")
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
