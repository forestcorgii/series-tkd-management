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

type TrainingSession struct {
	ID                 uuid.UUID    `json:"id"`
	SessionDate        time.Time    `json:"session_date"`
	StartTime          string       `json:"start_time"`
	EndTime            string       `json:"end_time"`
	CoachID            uuid.UUID    `json:"coach_id"`
	CoachName          string       `json:"coach_name,omitempty"`
	AdminID            *uuid.UUID   `json:"admin_id,omitempty"`
	AdminName          string       `json:"admin_name,omitempty"`
	TrainingType       TrainingType `json:"training_type"`
	Notes              string       `json:"notes"`
	IsCancelled        bool         `json:"is_cancelled"`
	CancelledAt        *time.Time   `json:"cancelled_at,omitempty"`
	CancellationReason string       `json:"cancellation_reason,omitempty"`
	CreatedAt          time.Time    `json:"created_at"`
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
	CheckedInAt      time.Time  `json:"checked_in_at"`
}

