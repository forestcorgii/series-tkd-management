package models

import (
	"time"

	"github.com/google/uuid"
)

type AuditCategory string

const (
	AuditCategoryAuth       AuditCategory = "AUTH"
	AuditCategoryStudents   AuditCategory = "STUDENTS"
	AuditCategoryAttendance AuditCategory = "ATTENDANCE"
	AuditCategorySessions   AuditCategory = "SESSIONS"
	AuditCategoryCoaches    AuditCategory = "COACHES"
	AuditCategoryAdmins     AuditCategory = "ADMINS"
	AuditCategoryPackages   AuditCategory = "PACKAGES"
	AuditCategorySafety     AuditCategory = "SAFETY"
	AuditCategorySettings   AuditCategory = "SETTINGS"
	AuditCategoryLocations  AuditCategory = "LOCATIONS"
)

type AuditLog struct {
	ID          uuid.UUID     `json:"id"`
	UserID      *uuid.UUID    `json:"user_id,omitempty"`     // Actor user ID (nil if unauthenticated/system)
	ActorName   string        `json:"actor_name"`            // Actor display name or identifier
	ActorEmail  string        `json:"actor_email"`           // Actor email
	ActorRole   UserRole      `json:"actor_role"`            // Role at time of action
	Action      string        `json:"action"`                // E.g. "AUTH_LOGIN", "STUDENT_CREATE", "ATTENDANCE_CHECKIN"
	Category    AuditCategory `json:"category"`              // E.g. "AUTH", "STUDENTS", "ATTENDANCE"
	TargetType  string        `json:"target_type,omitempty"` // E.g. "Student", "Session", "Package"
	TargetID    string        `json:"target_id,omitempty"`   // Target ID or UUID string
	TargetName  string        `json:"target_name,omitempty"` // Human-friendly target name
	Description string        `json:"description"`           // Human-readable narrative
	IPAddress   string        `json:"ip_address,omitempty"`
	UserAgent   string        `json:"user_agent,omitempty"`
	Metadata    string        `json:"metadata,omitempty"`    // JSON string for structured extra details
	CreatedAt   time.Time     `json:"created_at"`
}

type AuditLogFilter struct {
	Category  string `json:"category,omitempty"`
	Role      string `json:"role,omitempty"`
	Action    string `json:"action,omitempty"`
	Search    string `json:"search,omitempty"`
	StartDate string `json:"start_date,omitempty"` // "YYYY-MM-DD"
	EndDate   string `json:"end_date,omitempty"`   // "YYYY-MM-DD"
	Limit     int    `json:"limit,omitempty"`
	Offset    int    `json:"offset,omitempty"`
}

type AuditTelemetry struct {
	TotalLogs       int `json:"total_logs"`
	TodayLogs       int `json:"today_logs"`
	ActiveUsers     int `json:"active_users"`
	SecurityActions int `json:"security_actions"`
}
