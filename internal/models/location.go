package models

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Location represents a training facility, branch dojang, or event venue.
type Location struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Pin       string    `json:"pin"` // Google Maps link / share URL
	CreatedAt time.Time `json:"created_at"`
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
