package services

import (
	"series-tkd-management/internal/models"
)

type ReadinessStatus string

const (
	StatusReady           ReadinessStatus = "READY"
	StatusPreTestEligible ReadinessStatus = "PRE-TEST ELIGIBLE"
	StatusDeveloping      ReadinessStatus = "DEVELOPING"
)

type PromotionReadiness struct {
	Status             ReadinessStatus `json:"status"`
	BadgeColor         string          `json:"badge_color"` // green, yellow, red
	BadgeIcon          string          `json:"badge_icon"`  // 🟢, 🟡, 🔴
	TotalSessions      int             `json:"total_sessions"`
	RequiredSessions   int             `json:"required_sessions"`
	DaysInRank         int             `json:"days_in_rank"`
	RequiredDaysInRank int             `json:"required_days_in_rank"`
	SparringRatio      float64         `json:"sparring_ratio"`
	PoomsaeRatio       float64         `json:"poomsae_ratio"`
	LatestMinScore     int             `json:"latest_min_score"`
	LatestAvgScore     float64         `json:"latest_avg_score"`
	HasEvaluation      bool            `json:"has_evaluation"`
	Reasons            []string        `json:"reasons"`
}

type PromotionService struct{}

func NewPromotionService() *PromotionService {
	return &PromotionService{}
}

type BeltRequirement struct {
	RequiredSessions int
	RequiredDays     int
}

func getBeltRequirement(belt models.BeltRank) BeltRequirement {
	switch belt {
	case models.BeltWhite:
		return BeltRequirement{RequiredSessions: 16, RequiredDays: 45}
	case models.BeltLowYellow:
		return BeltRequirement{RequiredSessions: 20, RequiredDays: 60}
	case models.BeltHighYellow:
		return BeltRequirement{RequiredSessions: 24, RequiredDays: 60}
	case models.BeltLowBlue:
		return BeltRequirement{RequiredSessions: 28, RequiredDays: 75}
	case models.BeltHighBlue:
		return BeltRequirement{RequiredSessions: 32, RequiredDays: 90}
	case models.BeltLowRed:
		return BeltRequirement{RequiredSessions: 36, RequiredDays: 105}
	case models.BeltHighRed:
		return BeltRequirement{RequiredSessions: 40, RequiredDays: 120}
	case models.BeltLowBrown:
		return BeltRequirement{RequiredSessions: 44, RequiredDays: 135}
	case models.BeltHighBrown:
		return BeltRequirement{RequiredSessions: 48, RequiredDays: 150}
	case models.BeltBlack1stDan:
		return BeltRequirement{RequiredSessions: 60, RequiredDays: 180}
	case models.BeltBlack2ndDan:
		return BeltRequirement{RequiredSessions: 72, RequiredDays: 240}
	case models.BeltBlack3rdDan:
		return BeltRequirement{RequiredSessions: 84, RequiredDays: 365}
	// Legacy fallback support
	case "Yellow Tag":
		return BeltRequirement{RequiredSessions: 20, RequiredDays: 60}
	case "Yellow":
		return BeltRequirement{RequiredSessions: 24, RequiredDays: 60}
	case "Green Tag", "Green":
		return BeltRequirement{RequiredSessions: 28, RequiredDays: 75}
	case "Blue Tag", "Blue":
		return BeltRequirement{RequiredSessions: 32, RequiredDays: 90}
	case "Red Tag", "Red":
		return BeltRequirement{RequiredSessions: 40, RequiredDays: 120}
	default:
		return BeltRequirement{RequiredSessions: 30, RequiredDays: 90}
	}
}

func (s *PromotionService) EvaluateReadiness(
	student *models.Student,
	attendances []*models.Attendance,
	sessions map[string]*models.TrainingSession,
	latestEval *models.StudentEvaluation,
) PromotionReadiness {
	req := getBeltRequirement(student.CurrentBelt)
	totalSessions := len(attendances)
	daysInRank := student.DaysInCurrentRank()

	sparringCount := 0
	poomsaeCount := 0

	for _, att := range attendances {
		if att == nil || sessions == nil {
			continue
		}
		if sess, ok := sessions[att.SessionID.String()]; ok && sess != nil {
			switch sess.TrainingType {
			case models.TrainingSparring:
				sparringCount++
			case models.TrainingPoomsae:
				poomsaeCount++
			}
		}
	}

	sparringRatio := 0.0
	poomsaeRatio := 0.0
	if totalSessions > 0 {
		sparringRatio = float64(sparringCount) / float64(totalSessions)
		poomsaeRatio = float64(poomsaeCount) / float64(totalSessions)
	}

	reasons := []string{}
	attendanceMet := totalSessions >= req.RequiredSessions
	tenureMet := daysInRank >= req.RequiredDays

	if !attendanceMet {
		reasons = append(reasons, "Attendance count below threshold")
	}
	if !tenureMet {
		reasons = append(reasons, "Time in rank below required calendar interval")
	}

	hasEval := latestEval != nil
	minScore := 0
	avgScore := 0.0
	evalMet := false

	if hasEval {
		minScore = latestEval.MinScore()
		avgScore = latestEval.AverageScore()
		if minScore >= 6 && avgScore >= 6.5 {
			evalMet = true
		} else {
			reasons = append(reasons, "Latest coach evaluation score below readiness benchmark (min 6/10 required)")
		}
	} else {
		reasons = append(reasons, "Pending formal coach evaluation")
	}

	var status ReadinessStatus
	var badgeColor, badgeIcon string

	if attendanceMet && tenureMet && evalMet {
		status = StatusReady
		badgeColor = "emerald"
		badgeIcon = "🟢"
	} else if attendanceMet && tenureMet {
		status = StatusPreTestEligible
		badgeColor = "amber"
		badgeIcon = "🟡"
	} else {
		status = StatusDeveloping
		badgeColor = "rose"
		badgeIcon = "🔴"
	}

	return PromotionReadiness{
		Status:             status,
		BadgeColor:         badgeColor,
		BadgeIcon:          badgeIcon,
		TotalSessions:      totalSessions,
		RequiredSessions:   req.RequiredSessions,
		DaysInRank:         daysInRank,
		RequiredDaysInRank: req.RequiredDays,
		SparringRatio:      sparringRatio,
		PoomsaeRatio:       poomsaeRatio,
		LatestMinScore:     minScore,
		LatestAvgScore:     avgScore,
		HasEvaluation:      hasEval,
		Reasons:            reasons,
	}
}
