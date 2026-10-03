package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"

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
	CurrentUser         *models.User
}

type StudentReadinessSummary struct {
	Student   *models.Student
	Readiness services.PromotionReadiness
}

func (a *AppHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user != nil {
		switch user.Role {
		case models.RoleAdmin:
			http.Redirect(w, r, "/packages", http.StatusSeeOther)
			return
		case models.RoleCoach:
			http.Redirect(w, r, "/students", http.StatusSeeOther)
			return
		case models.RoleStudent:
			http.Redirect(w, r, "/portal/student", http.StatusSeeOther)
			return
		}
	}

	students, _ := a.store.GetAllStudents()
	coaches, _ := a.store.GetAllCoaches()
	sessions, _ := a.store.GetAllSessions()
	allAttendances, _ := a.store.GetAllAttendances()

	firstAidAlerts := 0
	now := time.Now()
	start := now.AddDate(0, -1, 0)

	for _, c := range coaches {
		summary := a.payrollSvc.CalculateCoachPayroll(c, sessions, allAttendances, start, now)
		if summary.FirstAidWarningFlag {
			firstAidAlerts++
		}
	}

	readinessSummaries := make([]StudentReadinessSummary, 0, len(students))
	allSessionsMap := make(map[string]*models.TrainingSession, len(sessions))
	for _, s := range sessions {
		allSessionsMap[s.ID.String()] = s
	}

	studentAttendancesMap := make(map[uuid.UUID][]*models.Attendance, len(students))
	for _, att := range allAttendances {
		studentAttendancesMap[att.StudentID] = append(studentAttendancesMap[att.StudentID], att)
	}

	latestEvalsMap, _ := a.store.GetLatestEvaluations()

	for _, st := range students {
		atts := studentAttendancesMap[st.ID]
		latestEval := latestEvalsMap[st.ID]
		readiness := a.promotionSvc.EvaluateReadiness(st, atts, allSessionsMap, latestEval)

		readinessSummaries = append(readinessSummaries, StudentReadinessSummary{
			Student:   st,
			Readiness: readiness,
		})
	}

	var openSessions []*models.TrainingSession
	for _, s := range sessions {
		if s.IsOpen() {
			openSessions = append(openSessions, s)
		}
	}

	data := DashboardViewData{
		ActiveStudentsCount: len(students),
		ActiveCoachesCount:  len(coaches),
		TotalSessionsCount:  len(sessions),
		FirstAidAlertsCount: firstAidAlerts,
		RecentSessions:      openSessions,
		ReadinessList:       readinessSummaries,
		CurrentUser:         user,
	}

	a.RenderPage(w, "dashboard.html", data)
}
