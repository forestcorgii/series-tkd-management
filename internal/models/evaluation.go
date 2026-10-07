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
	Sparring       int       `json:"sparring"`    // 1-10 (Apex / Top)
	Flexibility    int       `json:"flexibility"` // 1-10 (Bottom-Right)
	Poomsae        int       `json:"poomsae"`     // 1-10 (Bottom-Left)

	// Legacy compatibility fields
	Technique    int       `json:"technique,omitempty"`   // 1-10 (mapped to Poomsae)
	SparringIQ   int       `json:"sparring_iq,omitempty"` // 1-10 (mapped to Sparring)
	Stamina      int       `json:"stamina,omitempty"`     // 1-10
	Power        int       `json:"power,omitempty"`       // 1-10
	Discipline   int       `json:"discipline,omitempty"`  // 1-10
	CoachRemarks string    `json:"coach_remarks"`
	CreatedAt    time.Time `json:"created_at"`
}

func (e *StudentEvaluation) GetSparring() int {
	if e.Sparring > 0 {
		return e.Sparring
	}
	if e.SparringIQ > 0 {
		return e.SparringIQ
	}
	return 0
}

func (e *StudentEvaluation) GetFlexibility() int {
	return e.Flexibility
}

func (e *StudentEvaluation) GetPoomsae() int {
	if e.Poomsae > 0 {
		return e.Poomsae
	}
	if e.Technique > 0 {
		return e.Technique
	}
	return 0
}

func (e *StudentEvaluation) SyncLegacyFields() {
	sparr := e.GetSparring()
	poom := e.GetPoomsae()
	flex := e.GetFlexibility()

	if e.Sparring == 0 {
		e.Sparring = sparr
	}
	if e.SparringIQ == 0 {
		e.SparringIQ = sparr
	}
	if e.Poomsae == 0 {
		e.Poomsae = poom
	}
	if e.Technique == 0 {
		e.Technique = poom
	}
	if e.Stamina == 0 {
		e.Stamina = flex
	}
	if e.Power == 0 {
		e.Power = sparr
	}
	if e.Discipline == 0 {
		e.Discipline = poom
	}
}

func (e *StudentEvaluation) AverageScore() float64 {
	sum := e.GetSparring() + e.GetFlexibility() + e.GetPoomsae()
	return float64(sum) / 3.0
}

func (e *StudentEvaluation) MinScore() int {
	scores := []int{e.GetSparring(), e.GetFlexibility(), e.GetPoomsae()}
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

// ToSVGPolygon calculates SVG polygon coordinates for 3 axes: Sparring (Top), Flexibility (Bottom-Right), Poomsae (Bottom-Left)
func (e *StudentEvaluation) ToSVGPolygon(cx, cy, maxRadius float64) string {
	scores := []int{
		e.GetSparring(),
		e.GetFlexibility(),
		e.GetPoomsae(),
	}

	points := make([]string, 3)
	for i, score := range scores {
		// 3 axes starting from top (-PI/2)
		// Axis 0: Sparring (angle: -PI/2)
		// Axis 1: Flexibility (angle: PI/6)
		// Axis 2: Poomsae (angle: 5*PI/6)
		angle := (float64(i) * 2.0 * math.Pi / 3.0) - (math.Pi / 2.0)
		r := maxRadius * (float64(score) / 10.0)
		x := cx + r*math.Cos(angle)
		y := cy + r*math.Sin(angle)
		points[i] = fmt.Sprintf("%.1f,%.1f", x, y)
	}
	return strings.Join(points, " ")
}
