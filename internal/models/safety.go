package models

import (
	"time"

	"github.com/google/uuid"
)

type SafetyIncident struct {
	ID           uuid.UUID  `json:"id"`
	StudentID    uuid.UUID  `json:"student_id"`
	CoachID      *uuid.UUID `json:"coach_id,omitempty"`
	StudentName  string     `json:"student_name,omitempty"`
	CoachName    string     `json:"coach_name,omitempty"`
	IncidentType string     `json:"incident_type"`
	Notes        string     `json:"notes"`
	Resolved     bool       `json:"resolved"`
	ResolvedBy   string     `json:"resolved_by,omitempty"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
}
