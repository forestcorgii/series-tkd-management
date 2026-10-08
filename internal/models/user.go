package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserRole string

const (
	RoleStudent          UserRole = "STUDENT"
	RoleCoach            UserRole = "COACH"
	RoleAdmin            UserRole = "ADMIN"
	RoleOperationManager UserRole = "OPERATION_MANAGER"
)

var (
	ErrInvalidCredentials    = errors.New("invalid email or password")
	ErrUserInactive          = errors.New("user account is inactive")
	ErrUnauthorized          = errors.New("unauthorized")
	ErrForbidden             = errors.New("forbidden: insufficient permissions")
	ErrInvalidResetToken     = errors.New("invalid or expired password reset token")
	ErrUsernameAlreadyExists = errors.New("a user with this username already exists")
	ErrAccountPendingApproval = errors.New("account is pending manager approval")
)

type User struct {
	ID           uuid.UUID  `json:"id"`
	Email        string     `json:"email"`
	Username     string     `json:"username,omitempty"`
	PasswordHash string     `json:"-"`
	Role         UserRole   `json:"role"`
	StudentID    *uuid.UUID `json:"student_id,omitempty"`
	CoachID      *uuid.UUID `json:"coach_id,omitempty"`
	IsActive     bool       `json:"is_active"`
	LastLoginAt  *time.Time `json:"last_login_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`

	// Enriched fields for view convenience
	DisplayName       string `json:"display_name,omitempty"`
	ProfilePictureURL string `json:"profile_picture_url,omitempty"`
}

type PasswordResetToken struct {
	ID        uuid.UUID  `json:"id"`
	UserID    uuid.UUID  `json:"user_id"`
	Token     string     `json:"token"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

func (t *PasswordResetToken) IsValid() bool {
	return t != nil && t.UsedAt == nil && time.Now().Before(t.ExpiresAt)
}

func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}

func (u *User) HasRole(roles ...UserRole) bool {
	for _, r := range roles {
		if u.Role == r {
			return true
		}
	}
	return false
}

func (u *User) IsOperationManager() bool {
	return u.Role == RoleOperationManager
}

func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

func (u *User) IsCoach() bool {
	return u.Role == RoleCoach
}

func (u *User) IsStudent() bool {
	return u.Role == RoleStudent
}

func (u *User) IsPendingApproval() bool {
	return !u.IsActive && u.LastLoginAt == nil
}

func (u *User) RoleDashboardURL() string {
	if u == nil {
		return "/login"
	}
	switch u.Role {
	case RoleOperationManager:
		return "/"
	case RoleAdmin, RoleCoach:
		return "/sessions"
	case RoleStudent:
		return "/portal/student"
	default:
		return "/"
	}
}
