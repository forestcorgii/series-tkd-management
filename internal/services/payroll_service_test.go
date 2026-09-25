package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/services"
)

func TestPayrollService_CalculateCoachPayroll(t *testing.T) {
	svc := services.NewPayrollService()
	coachID := uuid.New()

	t.Run("Calculates correct payout and flags expired First Aid", func(t *testing.T) {
		pastExpiry := time.Now().AddDate(0, 0, -5) // expired 5 days ago
		coach := &models.Coach{
			ID:                coachID,
			FullName:          "Master Han",
			RatePerSession:    50.00,
			FirstAidCertified: true,
			FirstAidExpiry:    &pastExpiry,
		}

		sess1 := uuid.New()
		sess2 := uuid.New()
		sessions := []*models.TrainingSession{
			{ID: sess1, CoachID: coachID, SessionDate: time.Now()},
			{ID: sess2, CoachID: coachID, SessionDate: time.Now()},
		}

		student1 := uuid.New()
		student2 := uuid.New()
		attendances := []*models.Attendance{
			{SessionID: sess1, StudentID: student1},
			{SessionID: sess2, StudentID: student2},
		}

		start := time.Now().AddDate(0, 0, -1)
		end := time.Now().AddDate(0, 0, 1)

		summary := svc.CalculateCoachPayroll(coach, sessions, attendances, start, end)

		if summary.TotalSessionsLed != 2 {
			t.Errorf("expected 2 sessions led, got %d", summary.TotalSessionsLed)
		}
		if summary.CalculatedPayout != 100.00 {
			t.Errorf("expected payout 100.00, got %f", summary.CalculatedPayout)
		}
		if summary.TotalStudentsTaught != 2 {
			t.Errorf("expected 2 unique students taught, got %d", summary.TotalStudentsTaught)
		}
		if !summary.FirstAidWarningFlag {
			t.Errorf("expected FirstAidWarningFlag to be true for expired first aid")
		}
	})
}
