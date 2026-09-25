package handlers

import (
	"net/http"
	"time"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/services"
)

type DashboardViewData struct {
	ActiveStudentsCount int
	ActiveCoachesCount  int
	TotalSessionsCount  int
	FirstAidAlertsCount int
	RecentSessions      []*models.TrainingSession
	ReadinessList       []StudentReadinessSummary
}

type StudentReadinessSummary struct {
	Student   *models.Student
	Readiness services.PromotionReadiness
}

func (a *AppHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	students, _ := a.store.GetAllStudents()
	coaches, _ := a.store.GetAllCoaches()
	sessions, _ := a.store.GetAllSessions()
	var allAttendances []*models.Attendance
	for _, sess := range sessions {
		atts, _ := a.store.GetSessionAttendances(sess.ID)
		allAttendances = append(allAttendances, atts...)
	}

	firstAidAlerts := 0
	now := time.Now()
	start := now.AddDate(0, -1, 0)

	for _, c := range coaches {
		summary := a.payrollSvc.CalculateCoachPayroll(c, sessions, allAttendances, start, now)
		if summary.FirstAidWarningFlag {
			firstAidAlerts++
		}
	}

	readinessSummaries := []StudentReadinessSummary{}
	allSessionsMap := make(map[string]*models.TrainingSession)
	for _, s := range sessions {
		allSessionsMap[s.ID.String()] = s
	}

	for _, st := range students {
		atts, _ := a.store.GetStudentAttendances(st.ID)
		latestEval, _ := a.store.GetLatestEvaluation(st.ID)
		readiness := a.promotionSvc.EvaluateReadiness(st, atts, allSessionsMap, latestEval)

		readinessSummaries = append(readinessSummaries, StudentReadinessSummary{
			Student:   st,
			Readiness: readiness,
		})
	}

	data := DashboardViewData{
		ActiveStudentsCount: len(students),
		ActiveCoachesCount:  len(coaches),
		TotalSessionsCount:  len(sessions),
		FirstAidAlertsCount: firstAidAlerts,
		RecentSessions:      sessions,
		ReadinessList:       readinessSummaries,
	}

	a.RenderPage(w, "dashboard.html", data)
}
