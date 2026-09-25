package models

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"
)

type StudentEvaluation struct {
	ID             uuid.UUID `json:"id"`
	StudentID      uuid.UUID `json:"student_id"`
	CoachID        uuid.UUID `json:"coach_id"`
	CoachName      string    `json:"coach_name,omitempty"`
	EvaluationDate time.Time `json:"evaluation_date"`
	Flexibility    int       `json:"flexibility"` // 1-10
	Stamina        int       `json:"stamina"`     // 1-10
	Power          int       `json:"power"`       // 1-10
	Technique      int       `json:"technique"`   // 1-10
	SparringIQ     int       `json:"sparring_iq"` // 1-10
	Discipline     int       `json:"discipline"`  // 1-10
	CoachRemarks   string    `json:"coach_remarks"`
	CreatedAt      time.Time `json:"created_at"`
}

func (e *StudentEvaluation) AverageScore() float64 {
	sum := e.Flexibility + e.Stamina + e.Power + e.Technique + e.SparringIQ + e.Discipline
	return float64(sum) / 6.0
}

func (e *StudentEvaluation) MinScore() int {
	scores := []int{e.Flexibility, e.Stamina, e.Power, e.Technique, e.SparringIQ, e.Discipline}
	min := scores[0]
	for _, s := range scores {
		if s < min {
			min = s
		}
	}
	return min
}

type Point struct {
	X float64
	Y float64
}

// ToSVGPolygon calculates SVG polygon coordinates for 6 axes (Flexibility, Stamina, Power, Technique, SparringIQ, Discipline)
func (e *StudentEvaluation) ToSVGPolygon(cx, cy, maxRadius float64) string {
	scores := []int{
		e.Flexibility,
		e.Stamina,
		e.Power,
		e.Technique,
		e.SparringIQ,
		e.Discipline,
	}

	points := make([]string, 6)
	for i, score := range scores {
		// 6 axes starting from top (-PI/2)
		angle := (float64(i) * 2.0 * math.Pi / 6.0) - (math.Pi / 2.0)
		r := maxRadius * (float64(score) / 10.0)
		x := cx + r*math.Cos(angle)
		y := cy + r*math.Sin(angle)
		points[i] = fmt.Sprintf("%.1f,%.1f", x, y)
	}
	return strings.Join(points, " ")
}
