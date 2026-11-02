package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Location represents a training facility, branch dojang, or event venue.
type Location struct {
	ID          uuid.UUID `json:"id"`
	Name        string    `json:"name"`
	Pin         string    `json:"pin"`                    // Google Maps link / share URL
	FixedRate   *float64  `json:"fixed_rate,omitempty"`   // Default session rate pre-filled for classes at this location
	MaxTrainees *int      `json:"max_trainees,omitempty"` // Max trainees permitted per session at this location
	IsDefault   bool      `json:"is_default"`             // Pre-selected default location for public schedules and check-ins
	CreatedAt   time.Time `json:"created_at"`
}

// HasValidPin returns true if the Pin field contains a non-empty web URL.
func (l *Location) HasValidPin() bool {
	pin := strings.TrimSpace(l.Pin)
	if pin == "" {
		return false
	}
	return strings.HasPrefix(pin, "http://") || strings.HasPrefix(pin, "https://")
}

// DisplayPin returns the Google Maps URL or an empty string if invalid.
func (l *Location) DisplayPin() string {
	if l == nil {
		return ""
	}
	return strings.TrimSpace(l.Pin)
}

// FixedRateVal returns the fixed rate value if present, or 0.0 if not.
func (l *Location) FixedRateVal() float64 {
	if l != nil && l.FixedRate != nil {
		return *l.FixedRate
	}
	return 0.0
}

// HasFixedRate returns true if a fixed rate is specified and greater than zero.
func (l *Location) HasFixedRate() bool {
	return l != nil && l.FixedRate != nil && *l.FixedRate > 0
}

// MaxTraineesVal returns the max trainees value if present, or 0 if not.
func (l *Location) MaxTraineesVal() int {
	if l != nil && l.MaxTrainees != nil {
		return *l.MaxTrainees
	}
	return 0
}

// HasMaxTrainees returns true if a max trainees per session limit is configured.
func (l *Location) HasMaxTrainees() bool {
	return l != nil && l.MaxTrainees != nil && *l.MaxTrainees > 0
}
