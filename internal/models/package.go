package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPackageExpired  = errors.New("student package has expired")
	ErrNoSessionsLeft  = errors.New("no remaining sessions left in package")
	ErrPackageInactive = errors.New("package is not active")
)

type PackageTemplate struct {
	ID           uuid.UUID `json:"id"`
	Title        string    `json:"title"`
	SessionCount *int      `json:"session_count"` // nil means unlimited
	ValidityDays int       `json:"validity_days"`
	Price        float64   `json:"price"`
	IsActive     bool      `json:"is_active"`
}

func (pt *PackageTemplate) IsUnlimited() bool {
	return pt.SessionCount == nil
}

type StudentPackage struct {
	ID                uuid.UUID `json:"id"`
	StudentID         uuid.UUID `json:"student_id"`
	TemplateID        uuid.UUID `json:"template_id"`
	TemplateTitle     string    `json:"template_title,omitempty"`
	TotalSessions     *int      `json:"total_sessions"`     // nil = unlimited
	RemainingSessions *int      `json:"remaining_sessions"` // nil = unlimited
	PurchaseDate      time.Time `json:"purchase_date"`
	ExpiryDate        time.Time `json:"expiry_date"`
	PaymentStatus     string    `json:"payment_status"` // paid, unpaid, refunded
	CreatedAt         time.Time `json:"created_at"`
}

func (sp *StudentPackage) IsUnlimited() bool {
	return sp.RemainingSessions == nil
}

func (sp *StudentPackage) IsValidAt(t time.Time) bool {
	if sp.PaymentStatus != "paid" {
		return false
	}
	if t.After(sp.ExpiryDate) {
		return false
	}
	if sp.RemainingSessions != nil && *sp.RemainingSessions <= 0 {
		return false
	}
	return true
}

func (sp *StudentPackage) DeductSession(t time.Time) error {
	if !sp.IsValidAt(t) {
		if t.After(sp.ExpiryDate) {
			return ErrPackageExpired
		}
		if sp.RemainingSessions != nil && *sp.RemainingSessions <= 0 {
			return ErrNoSessionsLeft
		}
		return ErrPackageInactive
	}

	if sp.RemainingSessions != nil {
		newCount := *sp.RemainingSessions - 1
		sp.RemainingSessions = &newCount
	}
	return nil
}
