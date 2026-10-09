package models

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

type TrainingType string

const (
	TrainingPoomsae       TrainingType = "Poomsae"
	TrainingSparring      TrainingType = "Sparring"
	TrainingConditioning  TrainingType = "Conditioning"
	TrainingPromotionPrep TrainingType = "Promotion Prep"
)

type EntryType string

const (
	EntryTypeClass       EntryType = "class"
	EntryTypeEvent       EntryType = "event"
	EntryTypeDuty        EntryType = "duty"
	EntryTypeOpenSession EntryType = "open_session"
)

type SessionStaff struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"` // "Coach", "Admin", etc.
}

type TrainingSession struct {
	ID                 uuid.UUID       `json:"id"`
	SessionDate        time.Time       `json:"session_date"`
	StartTime          string          `json:"start_time"`
	EndTime            string          `json:"end_time"`
	CoachID            *uuid.UUID      `json:"coach_id,omitempty"`
	CoachName          string          `json:"coach_name,omitempty"`
	AdminID            *uuid.UUID      `json:"admin_id,omitempty"`
	AdminName          string          `json:"admin_name,omitempty"`
	TrainingType       TrainingType    `json:"training_type"`
	LocationID         *uuid.UUID      `json:"location_id,omitempty"`
	LocationName       string          `json:"location_name,omitempty"`
	LocationPin        string          `json:"location_pin,omitempty"`
	SessionRate        *float64        `json:"session_rate,omitempty"`
	Notes              string          `json:"notes"`
	IsCancelled        bool            `json:"is_cancelled"`
	CancelledAt        *time.Time      `json:"cancelled_at,omitempty"`
	CancellationReason string          `json:"cancellation_reason,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	EntryType          EntryType       `json:"entry_type,omitempty"`
	Title              string          `json:"title,omitempty"`
	AssignedStaff      []*SessionStaff `json:"assigned_staff,omitempty"`
}

// EntryTypeVal returns the normalized EntryType string (defaults to "class").
func (s *TrainingSession) EntryTypeVal() string {
	if s.EntryType == "" {
		return string(EntryTypeClass)
	}
	return string(s.EntryType)
}

// IsEvent returns true if this session represents a non-class schedule or event.
func (s *TrainingSession) IsEvent() bool {
	return s.EntryType == EntryTypeEvent
}

// IsDuty returns true if this session represents on-duty hours for coaches/admins.
func (s *TrainingSession) IsDuty() bool {
	return s.EntryType == EntryTypeDuty
}

// IsOpenSession returns true if this session represents an open / free drop-in floor session.
func (s *TrainingSession) IsOpenSession() bool {
	return s.EntryType == EntryTypeOpenSession
}

// IsClass returns true if this session is a standard training class.
func (s *TrainingSession) IsClass() bool {
	return s.EntryType == "" || s.EntryType == EntryTypeClass
}

// DisplayTitle returns the custom title or falls back to the type/category name.
func (s *TrainingSession) DisplayTitle() string {
	if strings.TrimSpace(s.Title) != "" {
		return s.Title
	}
	if s.IsDuty() {
		return "Staff On-Duty"
	}
	if s.IsOpenSession() {
		return "Free / Open Session"
	}
	if s.IsEvent() {
		return "Special Event"
	}
	if s.TrainingType != "" {
		return string(s.TrainingType)
	}
	return "Training Class"
}

// AssignedStaffNames returns a comma-delimited string of assigned duty staff names.
func (s *TrainingSession) AssignedStaffNames() string {
	if len(s.AssignedStaff) == 0 {
		return ""
	}
	names := make([]string, len(s.AssignedStaff))
	for i, staff := range s.AssignedStaff {
		names[i] = staff.Name
	}
	return strings.Join(names, ", ")
}

// AssignedStaffIDsComma returns a comma-delimited string of assigned staff IDs.
func (s *TrainingSession) AssignedStaffIDsComma() string {
	if len(s.AssignedStaff) == 0 {
		return ""
	}
	ids := make([]string, len(s.AssignedStaff))
	for i, staff := range s.AssignedStaff {
		ids[i] = staff.ID
	}
	return strings.Join(ids, ",")
}

// HasAssignedStaffID checks if a given staff ID is included in AssignedStaff.
func (s *TrainingSession) HasAssignedStaffID(id string) bool {
	for _, staff := range s.AssignedStaff {
		if staff.ID == id {
			return true
		}
	}
	return false
}

// StaffSummary returns a summary string of assigned coaches and admins.
func (s *TrainingSession) StaffSummary() string {
	if s.IsDuty() && len(s.AssignedStaff) > 0 {
		return s.AssignedStaffNames()
	}
	if s.CoachName != "" {
		return s.CoachName
	}
	if s.AdminName != "" {
		return s.AdminName
	}
	return "Unassigned"
}

// CoachIDString returns the coach ID string, or empty string if unassigned.
func (s *TrainingSession) CoachIDString() string {
	if s.CoachID == nil {
		return ""
	}
	return s.CoachID.String()
}

// SessionRateVal returns the dereferenced session rate or 0.
func (s *TrainingSession) SessionRateVal() float64 {
	if s.SessionRate != nil {
		return *s.SessionRate
	}
	return 0
}

// HasFixedRate returns true if the session has a fixed rate configured.
func (s *TrainingSession) HasFixedRate() bool {
	return s.SessionRate != nil && *s.SessionRate >= 0
}

// IsDone returns true if the session is not cancelled and has completed its scheduled end time.
func (s *TrainingSession) IsDone() bool {
	if s.IsCancelled {
		return false
	}
	return s.IsPastEndTimeAt(time.Now())
}

// IsOpen returns true if the session is not cancelled and has not completed its scheduled end time.
func (s *TrainingSession) IsOpen() bool {
	return !s.IsCancelled && !s.IsPastEndTime()
}

// IsOpenAt evaluates whether the session is open (not cancelled and not past its scheduled end time) relative to ref.
func (s *TrainingSession) IsOpenAt(ref time.Time) bool {
	return !s.IsCancelled && !s.IsPastEndTimeAt(ref)
}

// IsPastEndTime returns true if current local time has reached or passed the session's end time.
func (s *TrainingSession) IsPastEndTime() bool {
	return s.IsPastEndTimeAt(time.Now())
}

// IsPastEndTimeAt evaluates whether a given reference time is at or after the session's end time.
func (s *TrainingSession) IsPastEndTimeAt(ref time.Time) bool {
	sessDateStr := s.SessionDate.Format("2006-01-02")
	refDateStr := ref.Format("2006-01-02")
	if sessDateStr < refDateStr {
		return true
	}
	if sessDateStr > refDateStr {
		return false
	}

	endTimeStr := strings.TrimSpace(s.EndTime)
	if endTimeStr == "" {
		return false
	}

	var endHour, endMin int
	parsed := false
	for _, layout := range []string{"15:04", "15:04:05", "3:04 PM", "3:04PM", "03:04 PM", "03:04PM"} {
		if t, err := time.Parse(layout, endTimeStr); err == nil {
			endHour, endMin = t.Hour(), t.Minute()
			parsed = true
			break
		}
	}
	if !parsed {
		parts := strings.Split(endTimeStr, ":")
		if len(parts) >= 2 {
			if h, err := strconv.Atoi(parts[0]); err == nil {
				cleanMin := parts[1]
				if len(cleanMin) > 2 {
					cleanMin = cleanMin[:2]
				}
				if m, err := strconv.Atoi(cleanMin); err == nil {
					endHour, endMin = h, m
					parsed = true
				}
			}
		}
	}

	if !parsed {
		return false
	}

	refMinutes := ref.Hour()*60 + ref.Minute()
	endMinutes := endHour*60 + endMin
	return refMinutes >= endMinutes
}

type Attendance struct {
	ID               uuid.UUID  `json:"id"`
	SessionID        uuid.UUID  `json:"session_id"`
	StudentID        uuid.UUID  `json:"student_id"`
	StudentName      string     `json:"student_name,omitempty"`
	StudentBelt      BeltRank   `json:"student_belt,omitempty"`
	StudentPackageID *uuid.UUID `json:"student_package_id,omitempty"`
	PackageTitle     string     `json:"package_title,omitempty"`
	SessionRate      *float64   `json:"session_rate,omitempty"`
	LocationID       *uuid.UUID `json:"location_id,omitempty"`
	LocationName     string     `json:"location_name,omitempty"`
	LocationPin      string     `json:"location_pin,omitempty"`
	CheckedInAt      time.Time  `json:"checked_in_at"`
	AttendeeType     string     `json:"attendee_type,omitempty"` // "student", "coach", "admin", "guest"
	AttendeeName     string     `json:"attendee_name,omitempty"`
	AttendeeRole     string     `json:"attendee_role,omitempty"`
}

func (a *Attendance) SessionRateVal() float64 {
	if a.SessionRate != nil {
		return *a.SessionRate
	}
	return 0
}

// AttendeeTypeVal returns the normalized attendee type string ("student" by default).
func (a *Attendance) AttendeeTypeVal() string {
	if a.AttendeeType == "" {
		return "student"
	}
	return a.AttendeeType
}

// DisplayName returns AttendeeName if set, otherwise StudentName.
func (a *Attendance) DisplayName() string {
	if strings.TrimSpace(a.AttendeeName) != "" {
		return a.AttendeeName
	}
	if strings.TrimSpace(a.StudentName) != "" {
		return a.StudentName
	}
	return "Attendee"
}

// DisplayRole returns AttendeeRole, BeltRank, or the capitalized AttendeeType.
func (a *Attendance) DisplayRole() string {
	if strings.TrimSpace(a.AttendeeRole) != "" {
		return a.AttendeeRole
	}
	if a.AttendeeType == "coach" {
		return "Coach"
	}
	if a.AttendeeType == "admin" {
		return "Admin"
	}
	if a.AttendeeType == "guest" {
		return "Guest"
	}
	if a.StudentBelt != "" {
		return string(a.StudentBelt)
	}
	return "Student"
}

// IsGuest returns true if this check-in is for a guest attendee.
func (a *Attendance) IsGuest() bool {
	return a.AttendeeType == "guest"
}

// IsStaff returns true if this check-in is for a coach or admin staff.
func (a *Attendance) IsStaff() bool {
	return a.AttendeeType == "coach" || a.AttendeeType == "admin"
}

