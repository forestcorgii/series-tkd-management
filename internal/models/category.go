package models

import (
	"time"

	"github.com/google/uuid"
)

type TrainingCategory struct {
	ID        uuid.UUID `json:"id"`
	Name      string    `json:"name"`
	Color     string    `json:"color"`
	CreatedAt time.Time `json:"created_at"`
}
