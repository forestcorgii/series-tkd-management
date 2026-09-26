package models

import (
	"time"

	"github.com/google/uuid"
)

type BeltRank string

const (
	BeltWhite       BeltRank = "White"
	BeltLowYellow   BeltRank = "Low Yellow"
	BeltHighYellow  BeltRank = "High Yellow"
	BeltLowBlue     BeltRank = "Low Blue"
	BeltHighBlue    BeltRank = "High Blue"
	BeltLowRed      BeltRank = "Low Red"
	BeltHighRed     BeltRank = "High Red"
	BeltLowBrown    BeltRank = "Low Brown"
	BeltHighBrown   BeltRank = "High Brown"
	BeltBlack1stDan BeltRank = "1st Dan Black"
	BeltBlack2ndDan BeltRank = "2nd Dan Black"
	BeltBlack3rdDan BeltRank = "3rd Dan Black"
)

type Student struct {
	ID                uuid.UUID `json:"id"`
	FullName          string    `json:"full_name"`
	DOB               time.Time `json:"dob"`
	Gender            string    `json:"gender"`
	Phone             string    `json:"phone"`
	CurrentBelt       BeltRank  `json:"current_belt"`
	LastPromotionDate time.Time `json:"last_promotion_date"`
	EmergencyName     string    `json:"emergency_name"`
	EmergencyPhone    string    `json:"emergency_phone"`
	EmergencyRelation string    `json:"emergency_relation"`
	MedicalNotes      string    `json:"medical_notes"`
	HasSafetyFlag     bool      `json:"has_safety_flag"`
	IsActive          bool      `json:"is_active"`
	CreatedAt         time.Time `json:"created_at"`
}

func (s *Student) Age() int {
	now := time.Now()
	years := now.Year() - s.DOB.Year()
	if now.YearDay() < s.DOB.YearDay() {
		years--
	}
	return years
}

func (s *Student) DaysInCurrentRank() int {
	return int(time.Since(s.LastPromotionDate).Hours() / 24)
}

func (s *Student) NextBelt() BeltRank {
	switch s.CurrentBelt {
	case BeltWhite:
		return BeltLowYellow
	case BeltLowYellow:
		return BeltHighYellow
	case BeltHighYellow:
		return BeltLowBlue
	case BeltLowBlue:
		return BeltHighBlue
	case BeltHighBlue:
		return BeltLowRed
	case BeltLowRed:
		return BeltHighRed
	case BeltHighRed:
		return BeltLowBrown
	case BeltLowBrown:
		return BeltHighBrown
	case BeltHighBrown:
		return BeltBlack1stDan
	case BeltBlack1stDan:
		return BeltBlack2ndDan
	case BeltBlack2ndDan:
		return BeltBlack3rdDan
	// Legacy fallback cases
	case "Yellow Tag":
		return BeltHighYellow
	case "Yellow":
		return BeltLowBlue
	case "Green Tag", "Green":
		return BeltLowBlue
	case "Blue Tag":
		return BeltHighBlue
	case "Blue":
		return BeltLowRed
	case "Red Tag":
		return BeltHighRed
	case "Red":
		return BeltLowBrown
	case "Black Tag":
		return BeltBlack1stDan
	default:
		return s.CurrentBelt
	}
}
