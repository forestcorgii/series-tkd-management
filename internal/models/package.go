package models

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var (
	ErrPackageExpired      = errors.New("student package has expired")
	ErrNoSessionsLeft      = errors.New("no remaining sessions left in package")
	ErrPackageInactive     = errors.New("package is not active")
	ErrInvalidTitle        = errors.New("package template title is required")
	ErrInvalidValidity     = errors.New("validity days must be greater than zero")
	ErrInvalidPrice        = errors.New("price cannot be negative")
	ErrInvalidSessionCount = errors.New("session count must be greater than zero when not unlimited")
	ErrWeeklyLimitExceeded = errors.New("weekly class limit reached for this 4-week plan")
)

type PlanType string

const (
	PlanTypeStandard  PlanType = "standard"
	PlanTypeUnlimited PlanType = "unlimited"
	PlanTypeFourWeek  PlanType = "four_week"
)

type PackageTemplate struct {
	ID              uuid.UUID `json:"id"`
	Title           string    `json:"title"`
	Description     string    `json:"description,omitempty"`
	PlanType        PlanType  `json:"plan_type"`
	SessionCount    *int      `json:"session_count"` // nil means unlimited
	SessionsPerWeek *int      `json:"sessions_per_week,omitempty"`
	ValidityDays    int       `json:"validity_days"`
	Price           float64   `json:"price"`
	IsActive        bool      `json:"is_active"`
}

func (pt *PackageTemplate) IsUnlimited() bool {
	return pt.PlanType == PlanTypeUnlimited || pt.SessionCount == nil
}

func (pt *PackageTemplate) IsFourWeek() bool {
	return pt.PlanType == PlanTypeFourWeek
}

func (pt *PackageTemplate) WeeklyCadence() int {
	if pt.SessionsPerWeek != nil && *pt.SessionsPerWeek > 0 {
		return *pt.SessionsPerWeek
	}
	if pt.SessionCount != nil && *pt.SessionCount > 0 {
		w := *pt.SessionCount / 4
		if w < 1 {
			w = 1
		}
		return w
	}
	return 1
}

func (pt *PackageTemplate) Validate() error {
	if pt.Title == "" {
		return ErrInvalidTitle
	}
	if pt.PlanType == PlanTypeFourWeek && pt.ValidityDays <= 0 {
		pt.ValidityDays = 28
	}
	if pt.ValidityDays <= 0 {
		return ErrInvalidValidity
	}
	if pt.Price < 0 {
		return ErrInvalidPrice
	}
	if pt.PlanType == PlanTypeUnlimited {
		pt.SessionCount = nil
	} else if pt.PlanType == PlanTypeFourWeek {
		if pt.SessionCount == nil || *pt.SessionCount <= 0 {
			return ErrInvalidSessionCount
		}
	} else {
		if pt.SessionCount != nil && *pt.SessionCount <= 0 {
			return ErrInvalidSessionCount
		}
	}
	return nil
}

type StudentPackage struct {
	ID                uuid.UUID `json:"id"`
	StudentID         uuid.UUID `json:"student_id"`
	TemplateID        uuid.UUID `json:"template_id"`
	TemplateTitle     string    `json:"template_title,omitempty"`
	PlanType          PlanType  `json:"plan_type"`
	TotalSessions     *int      `json:"total_sessions"`     // nil = unlimited
	RemainingSessions *int      `json:"remaining_sessions"` // nil = unlimited
	SessionsPerWeek   *int      `json:"sessions_per_week,omitempty"`
	CustomPrice       *float64  `json:"custom_price,omitempty"`
	Notes             string    `json:"notes,omitempty"`
	PurchaseDate      time.Time `json:"purchase_date"`
	ExpiryDate        time.Time `json:"expiry_date"`
	PaymentStatus     string    `json:"payment_status"` // paid, unpaid, refunded
	CreatedAt         time.Time `json:"created_at"`
}

func (sp *StudentPackage) IsFourWeek() bool {
	return sp.PlanType == PlanTypeFourWeek
}

func (sp *StudentPackage) WeeklyCadence() int {
	if sp.SessionsPerWeek != nil && *sp.SessionsPerWeek > 0 {
		return *sp.SessionsPerWeek
	}
	if sp.TotalSessions != nil && *sp.TotalSessions > 0 {
		w := *sp.TotalSessions / 4
		if w < 1 {
			w = 1
		}
		return w
	}
	return 1
}

func (sp *StudentPackage) CurrentCycleWindow(at time.Time) (int, time.Time, time.Time) {
	days := int(at.Sub(sp.PurchaseDate).Hours() / 24)
	if days < 0 {
		days = 0
	}
	weekIndex := days / 7
	cycleStart := sp.PurchaseDate.AddDate(0, 0, weekIndex*7)
	cycleEnd := cycleStart.AddDate(0, 0, 7)
	return weekIndex + 1, cycleStart, cycleEnd
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
