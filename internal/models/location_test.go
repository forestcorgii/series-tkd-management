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

	max25 := 25
	locWithCap := &models.Location{
		Name:        "Ortigas Dojang",
		MaxTrainees: &max25,
	}
	if !locWithCap.HasMaxTrainees() {
		t.Errorf("expected HasMaxTrainees to be true")
	}
	if locWithCap.MaxTraineesVal() != 25 {
		t.Errorf("expected MaxTraineesVal to be 25, got %d", locWithCap.MaxTraineesVal())
	}

	zeroCap := 0
	locWithZero := &models.Location{
		Name:        "Pasig Dojang",
		MaxTrainees: &zeroCap,
	}
	if locWithZero.HasMaxTrainees() {
		t.Errorf("expected HasMaxTrainees to be false for 0")
	}
	if locWithZero.MaxTraineesVal() != 0 {
		t.Errorf("expected MaxTraineesVal to be 0, got %d", locWithZero.MaxTraineesVal())
	}

	var nilLoc *models.Location
	if nilLoc.HasMaxTrainees() {
		t.Errorf("expected nil Location HasMaxTrainees to be false")
	}
	if nilLoc.MaxTraineesVal() != 0 {
		t.Errorf("expected nil Location MaxTraineesVal to be 0")
	}
}
