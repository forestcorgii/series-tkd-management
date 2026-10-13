package models

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
)

var (
	ErrRoleNotFound      = errors.New("role not found")
	ErrRoleNameRequired  = errors.New("role name is required")
	ErrRoleSlugRequired  = errors.New("role slug is required")
	ErrRoleSlugExists    = errors.New("a role with this slug already exists")
	ErrSystemRoleUndeletable = errors.New("system default roles cannot be deleted")
	ErrRoleAssignedToUsers = errors.New("cannot delete role while staff members are assigned to it")
)

// Standard Permission Constants
const (
	// Wildcard
	PermWildcard = "*"

	// Dashboards
	PermDashboardRead = "dashboard:read"

	// Sessions & Timetable
	PermSessionsRead   = "sessions:read"
	PermSessionsCreate = "sessions:create"
	PermSessionsUpdate = "sessions:update"
	PermSessionsDelete = "sessions:delete"

	// Floor Attendance
	PermAttendanceCheckin = "attendance:checkin"
	PermAttendanceRemove  = "attendance:remove"

	// Students
	PermStudentsRead    = "students:read"
	PermStudentsCreate  = "students:create"
	PermStudentsUpdate  = "students:update"
	PermStudentsDelete  = "students:delete"
	PermStudentsPromote = "students:promote"

	// Athletic Ability Evaluations
	PermEvaluationsCreate = "evaluations:create"

	// Safety & Incidents
	PermSafetyFlag    = "safety:flag"
	PermSafetyResolve = "safety:resolve"

	// Packages & Billing
	PermPackagesRead   = "packages:read"
	PermPackagesAssign = "packages:assign"
	PermPackagesManage = "packages:manage"
	PermPackagesRevoke = "packages:revoke"

	// Staff & Coaches Directory
	PermStaffRead   = "staff:read"
	PermStaffManage = "staff:manage"

	// Roles & Permissions Governance
	PermRolesManage = "roles:manage"

	// Locations & Branches
	PermLocationsRead   = "locations:read"
	PermLocationsManage = "locations:manage"

	// Dojang Settings & Categories
	PermSettingsRead   = "settings:read"
	PermSettingsManage = "settings:manage"

	// Security & Audit Logs
	PermAuditRead = "audit:read"

	// Specialized Portals
	PermPortalCoach = "portal:coach"
	PermPortalAdmin = "portal:admin"
)

// PermissionInfo describes a granular capability for the UI permission catalog.
type PermissionInfo struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Category    string `json:"category"`
	Description string `json:"description"`
}

// GetAllPermissions returns the structured catalog of all available permissions in STMS.
func GetAllPermissions() []PermissionInfo {
	return []PermissionInfo{
		// Sessions & Attendance
		{Code: PermSessionsRead, Name: "View Timetable & Classes", Category: "Floor Sessions", Description: "View schedule, calendar week grid, and session cards"},
		{Code: PermSessionsCreate, Name: "Create Classes", Category: "Floor Sessions", Description: "Schedule new classes and training sessions"},
		{Code: PermSessionsUpdate, Name: "Edit / Reschedule / Cancel", Category: "Floor Sessions", Description: "Modify times, instructors, or cancel sessions with credit refund"},
		{Code: PermSessionsDelete, Name: "Delete Classes", Category: "Floor Sessions", Description: "Permanently delete sessions (including closed/past sessions)"},
		{Code: PermAttendanceCheckin, Name: "Admit & Check-In Students", Category: "Floor Sessions", Description: "Live tablet check-in, search students, deduct membership pass credits"},
		{Code: PermAttendanceRemove, Name: "Remove Attendance Records", Category: "Floor Sessions", Description: "Revoke mat check-ins and restore debited pass sessions"},

		// Students & Ability
		{Code: PermStudentsRead, Name: "View Student Roster", Category: "Student Directory", Description: "Browse student list, belt ranks, and practitioner profile cards"},
		{Code: PermStudentsCreate, Name: "Register New Students", Category: "Student Directory", Description: "Enroll new practitioners into the dojang roster"},
		{Code: PermStudentsUpdate, Name: "Edit Student & Emergency Info", Category: "Student Directory", Description: "Update contact numbers, guardian names, and medical notes"},
		{Code: PermStudentsDelete, Name: "Delete Student Profiles", Category: "Student Directory", Description: "Permanently purge student records and cascade attendance/history"},
		{Code: PermStudentsPromote, Name: "Perform Belt Promotions", Category: "Student Directory", Description: "Approve rank advancement and update belt Kup/Dan ranks"},
		{Code: PermEvaluationsCreate, Name: "Submit Athletic Evaluations", Category: "Student Directory", Description: "Score 6-pillar athletic ability radar matrix (Forms, Sparring IQ, etc.)"},

		// Safety & First Aid
		{Code: PermSafetyFlag, Name: "Report Floor Incidents", Category: "Safety & Medical", Description: "Log injuries, First Aid events, and place students on floor safety hold"},
		{Code: PermSafetyResolve, Name: "Clear Medical Holds", Category: "Safety & Medical", Description: "Review incident tickets and lift floor safety holds"},

		// Memberships & Passes
		{Code: PermPackagesRead, Name: "View Membership Passes", Category: "Membership Packages", Description: "View pass templates and student credit ledgers"},
		{Code: PermPackagesAssign, Name: "Assign Passes to Students", Category: "Membership Packages", Description: "Issue prepaid class passes or monthly plans to student accounts"},
		{Code: PermPackagesManage, Name: "Manage Pass Templates", Category: "Membership Packages", Description: "Create, edit pricing, and configure session packages"},
		{Code: PermPackagesRevoke, Name: "Revoke Active Passes", Category: "Membership Packages", Description: "Void or refund student membership packages"},

		// Staff, Coaches & Roles
		{Code: PermStaffRead, Name: "View Staff Directory", Category: "Staff & Governance", Description: "View coaches roster, qualifications, and payroll rates"},
		{Code: PermStaffManage, Name: "Manage Staff Accounts", Category: "Staff & Governance", Description: "Approve, deactivate, delete staff and instructors"},
		{Code: PermRolesManage, Name: "Manage Roles & Permissions", Category: "Staff & Governance", Description: "Create custom roles, configure permission checkboxes, and assign roles to staff"},

		// Locations & Facilities
		{Code: PermLocationsRead, Name: "View Dojang Branches", Category: "Facilities", Description: "Browse branch facilities, mat capacities, and location details"},
		{Code: PermLocationsManage, Name: "Manage Dojang Branches", Category: "Facilities", Description: "Add, update, or remove branch locations"},

		// Settings & System
		{Code: PermDashboardRead, Name: "View Dashboard Overview", Category: "System Operations", Description: "Access operational telemetry and floor dashboard summary"},
		{Code: PermSettingsRead, Name: "View Dojang Settings", Category: "System Operations", Description: "View training categories and notification configurations"},
		{Code: PermSettingsManage, Name: "Modify Dojang Settings", Category: "System Operations", Description: "Update training categories, colors, and notification triggers"},
		{Code: PermAuditRead, Name: "Audit & Activity Logs", Category: "System Operations", Description: "Inspect immutable audit log ledger and user activity telemetry"},

		// Portals
		{Code: PermPortalCoach, Name: "Access Coach Portal", Category: "Specialized Portals", Description: "Use rapid floor launcher and priority promotion review queue"},
		{Code: PermPortalAdmin, Name: "Access Operations Portal", Category: "Specialized Portals", Description: "Access master operations portal overview"},
	}
}

// StaffRole represents a customizable role assigned to staff members.
type StaffRole struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Slug        string    `json:"slug"`
	Description string    `json:"description"`
	IsSystem    bool      `json:"is_system"`
	Permissions []string  `json:"permissions"`
	MemberCount int       `json:"member_count,omitempty"` // Enriched for view convenience
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Validate checks role domain invariants.
func (r *StaffRole) Validate() error {
	if strings.TrimSpace(r.Name) == "" {
		return ErrRoleNameRequired
	}
	if strings.TrimSpace(r.Slug) == "" {
		return ErrRoleSlugRequired
	}
	r.Slug = strings.ToLower(strings.TrimSpace(r.Slug))
	r.Name = strings.TrimSpace(r.Name)
	return nil
}

// HasPermission checks if the role explicitly contains a permission or has wildcard "*".
func (r *StaffRole) HasPermission(perm string) bool {
	if r == nil {
		return false
	}
	for _, p := range r.Permissions {
		if p == PermWildcard || p == perm {
			return true
		}
	}
	return false
}

// DefaultStaffRoles returns the 3 pre-created roles: Coach, Admin, Manager.
func DefaultStaffRoles() []*StaffRole {
	now := time.Now()

	coachRole := &StaffRole{
		ID:          uuid.MustParse("d1111111-1111-1111-1111-111111111111"),
		Name:        "Coach / Instructor",
		Slug:        "coach",
		Description: "Floor instruction, live session rosters, attendance check-in, safety incident logging, and athletic evaluations.",
		IsSystem:    true,
		Permissions: []string{
			PermDashboardRead,
			PermSessionsRead,
			PermSessionsCreate,
			PermSessionsUpdate,
			PermAttendanceCheckin,
			PermAttendanceRemove,
			PermStudentsRead,
			PermEvaluationsCreate,
			PermSafetyFlag,
			PermLocationsRead,
			PermSettingsRead,
			PermPortalCoach,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	adminRole := &StaffRole{
		ID:          uuid.MustParse("d2222222-2222-2222-2222-222222222222"),
		Name:        "Dojang Administrator",
		Slug:        "admin",
		Description: "Front-desk operations, student registration & contact updates, membership pass assignment, and floor check-in.",
		IsSystem:    true,
		Permissions: []string{
			PermDashboardRead,
			PermSessionsRead,
			PermSessionsCreate,
			PermSessionsUpdate,
			PermAttendanceCheckin,
			PermAttendanceRemove,
			PermStudentsRead,
			PermStudentsCreate,
			PermStudentsUpdate,
			PermSafetyFlag,
			PermPackagesRead,
			PermPackagesAssign,
			PermLocationsRead,
			PermSettingsRead,
			PermSettingsManage,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	managerRole := &StaffRole{
		ID:          uuid.MustParse("d3333333-3333-3333-3333-333333333333"),
		Name:        "Operation Manager",
		Slug:        "manager",
		Description: "Full operational authority: belt promotions, billing revocation, class deletion, staff & role governance, and audit trails.",
		IsSystem:    true,
		Permissions: []string{
			PermWildcard, // Wildcard grants all permissions
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	return []*StaffRole{coachRole, adminRole, managerRole}
}
