package models

import (
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
	ID           uuid.UUID    `json:"id"`
	SessionDate  time.Time    `json:"session_date"`
	StartTime    string       `json:"start_time"`
	EndTime      string       `json:"end_time"`
	CoachID      uuid.UUID    `json:"coach_id"`
	CoachName    string       `json:"coach_name,omitempty"`
	AdminID      *uuid.UUID   `json:"admin_id,omitempty"`
	AdminName    string       `json:"admin_name,omitempty"`
	TrainingType TrainingType `json:"training_type"`
	Notes        string       `json:"notes"`
	CreatedAt    time.Time    `json:"created_at"`
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
