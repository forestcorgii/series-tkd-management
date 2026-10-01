package models_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
)

func TestLocation(t *testing.T) {
	locID := uuid.New()
	loc := &models.Location{
		ID:        locID,
		Name:      "Makati Central Branch",
		Pin:       "https://maps.app.goo.gl/sample123",
		CreatedAt: time.Now(),
	}

	if loc.ID != locID {
		t.Errorf("expected ID %v, got %v", locID, loc.ID)
	}
	if loc.Name != "Makati Central Branch" {
		t.Errorf("expected Name 'Makati Central Branch', got %s", loc.Name)
	}
	if !loc.HasValidPin() {
		t.Errorf("expected valid pin for https URL")
	}

	invalidLoc := &models.Location{
		Pin: "not-a-url",
	}
	if invalidLoc.HasValidPin() {
		t.Errorf("expected invalid pin for non-http string")
	}

	emptyLoc := &models.Location{
		Pin: "   ",
	}
	if emptyLoc.HasValidPin() {
		t.Errorf("expected invalid pin for whitespace string")
	}
}
