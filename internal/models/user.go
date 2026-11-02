package models

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

type UserRole string

const (
	// The 2 primary user divisions in STMS:
	RoleStudent UserRole = "STUDENT"
	RoleStaff   UserRole = "STAFF"

	// Legacy aliases kept for complete backward compatibility
	RoleCoach            UserRole = "COACH"
	RoleAdmin            UserRole = "ADMIN"
	RoleOperationManager UserRole = "OPERATION_MANAGER"
)

var (
	ErrInvalidCredentials     = errors.New("invalid email or password")
	ErrUserInactive           = errors.New("user account is inactive")
	ErrUnauthorized           = errors.New("unauthorized")
	ErrForbidden              = errors.New("forbidden: insufficient permissions")
	ErrInvalidResetToken      = errors.New("invalid or expired password reset token")
	ErrUsernameAlreadyExists  = errors.New("a user with this username already exists")
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

	// Enriched fields for view & authorization convenience
	DisplayName       string       `json:"display_name,omitempty"`
	ProfilePictureURL string       `json:"profile_picture_url,omitempty"`
	Roles             []*StaffRole `json:"roles,omitempty"`
	Permissions       []string     `json:"permissions,omitempty"`
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

// IsStudent returns true if the user is a student practitioner.
func (u *User) IsStudent() bool {
	if u == nil {
		return false
	}
	return u.Role == RoleStudent
}

// IsStaff returns true if the user is a staff member (coach, admin, manager, or custom role).
func (u *User) IsStaff() bool {
	if u == nil {
		return false
	}
	return u.Role == RoleStaff || u.Role == RoleCoach || u.Role == RoleAdmin || u.Role == RoleOperationManager
}

// HasPermission checks if the user has a specific permission code or the wildcard "*" grant.
func (u *User) HasPermission(perm string) bool {
	if u == nil {
		return false
	}
	// Operation Manager legacy role always has all permissions
	if u.Role == RoleOperationManager {
		return true
	}
	// Students never have staff permissions
	if u.IsStudent() {
		return false
	}
	// Check user aggregated permissions
	for _, p := range u.Permissions {
		if p == PermWildcard || p == perm {
			return true
		}
	}
	// Also inspect individual assigned staff roles
	for _, r := range u.Roles {
		if r.HasPermission(perm) {
			return true
		}
	}
	return false
}

// HasAnyPermission checks if the user possesses at least one of the specified permissions.
func (u *User) HasAnyPermission(perms ...string) bool {
	for _, p := range perms {
		if u.HasPermission(p) {
			return true
		}
	}
	return false
}

// HasAllPermissions checks if the user possesses every single specified permission.
func (u *User) HasAllPermissions(perms ...string) bool {
	for _, p := range perms {
		if !u.HasPermission(p) {
			return false
		}
	}
	return true
}

// HasRoleSlug checks if the user has an assigned staff role matching a specific slug (e.g. "coach", "admin", "manager").
func (u *User) HasRoleSlug(slug string) bool {
	if u == nil {
		return false
	}
	target := strings.ToLower(strings.TrimSpace(slug))
	for _, r := range u.Roles {
		if strings.ToLower(r.Slug) == target {
			return true
		}
	}
	return false
}

// HasRole checks if user matches any given role, supporting both new ("STAFF", "STUDENT") and legacy roles.
func (u *User) HasRole(roles ...UserRole) bool {
	if u == nil {
		return false
	}
	for _, r := range roles {
		if u.Role == r {
			return true
		}
		// If checking for legacy roles, evaluate through slug/permission helpers
		switch r {
		case RoleOperationManager:
			if u.IsOperationManager() {
				return true
			}
		case RoleAdmin:
			if u.IsAdmin() {
				return true
			}
		case RoleCoach:
			if u.IsCoach() {
				return true
			}
		case RoleStaff:
			if u.IsStaff() {
				return true
			}
		case RoleStudent:
			if u.IsStudent() {
				return true
			}
		}
	}
	return false
}

// IsOperationManager checks if user is an Operation Manager (wildcard/roles:manage/manager slug/legacy role).
func (u *User) IsOperationManager() bool {
	if u == nil {
		return false
	}
	return u.Role == RoleOperationManager || u.HasPermission(PermRolesManage) || u.HasPermission(PermWildcard) || u.HasRoleSlug("manager")
}

// IsAdmin checks if user is a Dojang Administrator (admin slug/legacy role or has packages:assign & students:create).
func (u *User) IsAdmin() bool {
	if u == nil || u.IsOperationManager() {
		return false
	}
	if u.Role == RoleAdmin || u.HasRoleSlug("admin") {
		return true
	}
	return u.IsStaff() && u.HasPermission(PermPackagesAssign) && u.HasPermission(PermStudentsCreate)
}

// IsCoach checks if user is a Coach / Instructor (coach slug/legacy role, has coach ID, or has evaluations:create).
func (u *User) IsCoach() bool {
	if u == nil || u.IsOperationManager() {
		return false
	}
	if u.Role == RoleCoach || u.HasRoleSlug("coach") || u.CoachID != nil {
		return true
	}
	return u.IsStaff() && u.HasPermission(PermEvaluationsCreate)
}

// PrimaryRoleName returns a friendly display name for the user's role.
func (u *User) PrimaryRoleName() string {
	if u == nil {
		return "Guest"
	}
	if u.IsStudent() {
		return "Student Practitioner"
	}
	if len(u.Roles) > 0 {
		return u.Roles[0].Name
	}
	if u.IsOperationManager() {
		return "Operation Manager"
	}
	if u.IsAdmin() {
		return "Dojang Administrator"
	}
	if u.IsCoach() {
		return "Coach / Instructor"
	}
	return "Staff Member"
}

func (u *User) IsPendingApproval() bool {
	return !u.IsActive && u.LastLoginAt == nil
}

func (u *User) RoleDashboardURL() string {
	if u == nil {
		return "/login"
	}
	return "/"
}
