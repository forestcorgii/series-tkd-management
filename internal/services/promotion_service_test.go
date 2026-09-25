package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/services"
)

func TestPromotionService_EvaluateReadiness(t *testing.T) {
	svc := services.NewPromotionService()
	studentID := uuid.New()

	t.Run("Status READY when all criteria met", func(t *testing.T) {
		student := &models.Student{
			ID:                studentID,
			CurrentBelt:       models.BeltWhite,
			LastPromotionDate: time.Now().AddDate(0, 0, -60), // 60 days ago (req: 45)
		}

		// 20 attendance records (req: 16)
		attendances := make([]*models.Attendance, 20)
		sessions := make(map[string]*models.TrainingSession)

		for i := 0; i < 20; i++ {
			sessID := uuid.New()
			tType := models.TrainingPoomsae
			if i%2 == 0 {
				tType = models.TrainingSparring
			}
			sessions[sessID.String()] = &models.TrainingSession{
				ID:           sessID,
				TrainingType: tType,
			}
			attendances[i] = &models.Attendance{
				ID:        uuid.New(),
				SessionID: sessID,
				StudentID: studentID,
			}
		}

		latestEval := &models.StudentEvaluation{
			StudentID:   studentID,
			Flexibility: 7,
			Stamina:     8,
			Power:       7,
			Technique:   8,
			SparringIQ:  7,
			Discipline:  9,
		}

		readiness := svc.EvaluateReadiness(student, attendances, sessions, latestEval)

		if readiness.Status != services.StatusReady {
			t.Errorf("expected status READY, got %s", readiness.Status)
		}
		if readiness.BadgeIcon != "🟢" {
			t.Errorf("expected green badge icon, got %s", readiness.BadgeIcon)
		}
	})

	t.Run("Status PRE-TEST ELIGIBLE when attendance met but missing/low eval", func(t *testing.T) {
		student := &models.Student{
			ID:                studentID,
			CurrentBelt:       models.BeltWhite,
			LastPromotionDate: time.Now().AddDate(0, 0, -60),
		}

		attendances := make([]*models.Attendance, 20)
		sessions := make(map[string]*models.TrainingSession)

		for i := 0; i < 20; i++ {
			sessID := uuid.New()
			sessions[sessID.String()] = &models.TrainingSession{
				ID:           sessID,
				TrainingType: models.TrainingPoomsae,
			}
			attendances[i] = &models.Attendance{
				ID:        uuid.New(),
				SessionID: sessID,
				StudentID: studentID,
			}
		}

		readiness := svc.EvaluateReadiness(student, attendances, sessions, nil) // no evaluation

		if readiness.Status != services.StatusPreTestEligible {
			t.Errorf("expected status PRE-TEST ELIGIBLE, got %s", readiness.Status)
		}
		if readiness.BadgeIcon != "🟡" {
			t.Errorf("expected yellow badge icon, got %s", readiness.BadgeIcon)
		}
	})

	t.Run("Status DEVELOPING when insufficient attendance or tenure", func(t *testing.T) {
		student := &models.Student{
			ID:                studentID,
			CurrentBelt:       models.BeltWhite,
			LastPromotionDate: time.Now().AddDate(0, 0, -10), // only 10 days in rank
		}

		attendances := make([]*models.Attendance, 5) // only 5 sessions
		sessions := make(map[string]*models.TrainingSession)

		for i := 0; i < 5; i++ {
			sessID := uuid.New()
			sessions[sessID.String()] = &models.TrainingSession{
				ID:           sessID,
				TrainingType: models.TrainingPoomsae,
			}
			attendances[i] = &models.Attendance{
				ID:        uuid.New(),
				SessionID: sessID,
				StudentID: studentID,
			}
		}

		readiness := svc.EvaluateReadiness(student, attendances, sessions, nil)

		if readiness.Status != services.StatusDeveloping {
			t.Errorf("expected status DEVELOPING, got %s", readiness.Status)
		}
		if readiness.BadgeIcon != "🔴" {
			t.Errorf("expected red badge icon, got %s", readiness.BadgeIcon)
		}
	})
}
