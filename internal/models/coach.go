package models

import (
	"time"

	"github.com/google/uuid"
)

type Coach struct {
	ID                uuid.UUID  `json:"id"`
	FullName          string     `json:"full_name"`
	Email             string     `json:"email"`
	Phone             string     `json:"phone"`
	BeltRank          string     `json:"belt_rank"`
	RatePerSession    float64    `json:"rate_per_session"`
	FirstAidCertified bool       `json:"first_aid_certified"`
	FirstAidExpiry    *time.Time `json:"first_aid_expiry"`
	Specialties       []string   `json:"specialties"`
	IsActive          bool       `json:"is_active"`
	CreatedAt         time.Time  `json:"created_at"`
}

func (c *Coach) IsFirstAidValid() bool {
	if !c.FirstAidCertified {
		return false
	}
	if c.FirstAidExpiry == nil {
		return false
	}
	return c.FirstAidExpiry.After(time.Now())
}

func (c *Coach) DaysUntilFirstAidExpiry() int {
	if c.FirstAidExpiry == nil {
		return -1
	}
	duration := time.Until(*c.FirstAidExpiry)
	return int(duration.Hours() / 24)
}
