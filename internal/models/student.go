package models

import (
	"time"

	"github.com/google/uuid"
)

type BeltRank string

const (
	BeltWhite        BeltRank = "White"
	BeltYellowTag    BeltRank = "Yellow Tag"
	BeltYellow       BeltRank = "Yellow"
	BeltGreenTag     BeltRank = "Green Tag"
	BeltGreen        BeltRank = "Green"
	BeltBlueTag      BeltRank = "Blue Tag"
	BeltBlue         BeltRank = "Blue"
	BeltRedTag       BeltRank = "Red Tag"
	BeltRed          BeltRank = "Red"
	BeltBlackTag     BeltRank = "Black Tag"
	BeltBlack1stDan  BeltRank = "1st Dan Black"
	BeltBlack2ndDan  BeltRank = "2nd Dan Black"
	BeltBlack3rdDan  BeltRank = "3rd Dan Black"
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
