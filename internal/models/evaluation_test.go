package models_test

import (
	"math"
	"strings"
	"testing"

	"series-tkd-management/internal/models"
)

func TestStudentEvaluation_AverageScore(t *testing.T) {
	eval := &models.StudentEvaluation{
		Sparring:    9,
		Flexibility: 8,
		Poomsae:     7,
	}

	expected := (9.0 + 8.0 + 7.0) / 3.0
	if math.Abs(eval.AverageScore()-expected) > 0.0001 {
		t.Errorf("expected AverageScore %.2f, got %.2f", expected, eval.AverageScore())
	}
}

func TestStudentEvaluation_MinScore(t *testing.T) {
	eval := &models.StudentEvaluation{
		Sparring:    9,
		Flexibility: 6,
		Poomsae:     8,
	}

	if eval.MinScore() != 6 {
		t.Errorf("expected MinScore 6, got %d", eval.MinScore())
	}
}

func TestStudentEvaluation_ToSVGPolygon(t *testing.T) {
	eval := &models.StudentEvaluation{
		Sparring:    10,
		Flexibility: 10,
		Poomsae:     10,
	}

	poly := eval.ToSVGPolygon(100, 100, 80)
	points := strings.Split(poly, " ")
	if len(points) != 3 {
		t.Fatalf("expected 3 vertices for 3-axis radar, got %d: %s", len(points), poly)
	}

	expectedPoints := []string{"100.0,20.0", "169.3,140.0", "30.7,140.0"}
	for i, p := range points {
		if p != expectedPoints[i] {
			t.Errorf("vertex %d expected %s, got %s", i, expectedPoints[i], p)
		}
	}
}

func TestStudentEvaluation_LegacyFieldSync(t *testing.T) {
	legacy := &models.StudentEvaluation{
		SparringIQ:  8,
		Technique:   9,
		Flexibility: 7,
	}

	if legacy.GetSparring() != 8 {
		t.Errorf("expected GetSparring to return 8 from SparringIQ, got %d", legacy.GetSparring())
	}
	if legacy.GetPoomsae() != 9 {
		t.Errorf("expected GetPoomsae to return 9 from Technique, got %d", legacy.GetPoomsae())
	}

	legacy.SyncLegacyFields()

	if legacy.Sparring != 8 {
		t.Errorf("expected Sparring to be synced to 8, got %d", legacy.Sparring)
	}
	if legacy.Poomsae != 9 {
		t.Errorf("expected Poomsae to be synced to 9, got %d", legacy.Poomsae)
	}
}
