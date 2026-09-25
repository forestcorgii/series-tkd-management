package services

import (
	"fmt"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
)

type CoachPayrollSummary struct {
	CoachID              uuid.UUID `json:"coach_id"`
	CoachName            string    `json:"coach_name"`
	RatePerSession       float64   `json:"rate_per_session"`
	FirstAidCertified    bool      `json:"first_aid_certified"`
	FirstAidValid        bool      `json:"first_aid_valid"`
	FirstAidExpiryDays   int       `json:"first_aid_expiry_days"`
	TotalSessionsLed     int       `json:"total_sessions_led"`
	TotalStudentsTaught  int       `json:"total_students_taught"`
	CalculatedPayout     float64   `json:"calculated_payout"`
	FirstAidWarningFlag  bool      `json:"first_aid_warning_flag"`
	WarningMessage       string    `json:"warning_message,omitempty"`
}

type PayrollService struct{}

func NewPayrollService() *PayrollService {
	return &PayrollService{}
}

func (ps *PayrollService) CalculateCoachPayroll(
	coach *models.Coach,
	sessions []*models.TrainingSession,
	attendances []*models.Attendance,
	startDate time.Time,
	endDate time.Time,
) CoachPayrollSummary {
	sessionsLed := 0
	studentsCountMap := make(map[string]bool)

	for _, sess := range sessions {
		if sess.CoachID == coach.ID {
			if !sess.SessionDate.Before(startDate) && !sess.SessionDate.After(endDate) {
				sessionsLed++
			}
		}
	}

	for _, att := range attendances {
		for _, sess := range sessions {
			if sess.ID == att.SessionID && sess.CoachID == coach.ID {
				if !sess.SessionDate.Before(startDate) && !sess.SessionDate.After(endDate) {
					studentsCountMap[att.StudentID.String()] = true
				}
			}
		}
	}

	payout := float64(sessionsLed) * coach.RatePerSession
	firstAidValid := coach.IsFirstAidValid()
	expiryDays := coach.DaysUntilFirstAidExpiry()

	warningFlag := false
	warningMsg := ""

	if !coach.FirstAidCertified {
		warningFlag = true
		warningMsg = "Coach lacks Red Cross/BLS First Aid certification!"
	} else if !firstAidValid {
		warningFlag = true
		warningMsg = "Red Cross/BLS First Aid certification HAS EXPIRED!"
	} else if expiryDays <= 30 {
		warningFlag = true
		warningMsg = fmt.Sprintf("First Aid certification expires in %d days", expiryDays)
	}

	return CoachPayrollSummary{
		CoachID:             coach.ID,
		CoachName:           coach.FullName,
		RatePerSession:      coach.RatePerSession,
		FirstAidCertified:   coach.FirstAidCertified,
		FirstAidValid:       firstAidValid,
		FirstAidExpiryDays:  expiryDays,
		TotalSessionsLed:    sessionsLed,
		TotalStudentsTaught: len(studentsCountMap),
		CalculatedPayout:    payout,
		FirstAidWarningFlag: warningFlag,
		WarningMessage:      warningMsg,
	}
}
