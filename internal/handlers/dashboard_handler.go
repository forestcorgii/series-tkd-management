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

type ExpiringPackageSummary struct {
	Package           *models.StudentPackage
	StudentID         uuid.UUID
	StudentName       string
	TemplateTitle     string
	RemainingSessions int
	TotalSessions     int
	ExpiryDate        time.Time
	DaysUntilExpiry   int
	IsLowCredit       bool
	IsExpiringSoon    bool
	IsUnlimited       bool
}

type CoachDashboardViewData struct {
	CurrentUser           *models.User
	Coach                 *models.Coach
	TodaySessions         []*models.TrainingSession
	LiveSession           *models.TrainingSession
	LiveAttendances       []*models.Attendance
	AllSessions           []*models.TrainingSession
	AllStudents           []*models.Student
	PriorityReviewQueue   []StudentReadinessSummary
	OpenSafetyIncidents   []*models.SafetyIncident
	ClassesConductedCount int
	Categories            []*models.TrainingCategory
	Locations             []*models.Location
}

type AdminDashboardViewData struct {
	CurrentUser              *models.User
	ActiveStudentsCount      int
	TodaySessionsCount       int
	TotalSessionsCount       int
	ActivePackagesCount      int
	ExpiringPackagesCount    int
	OpenSafetyIncidentsCount int
	TodaySessions            []*models.TrainingSession
	LiveSessions             []*models.TrainingSession
	AllStudents              []*models.Student
	AllCoaches               []*models.Coach
	AllAdmins                []*models.User
	Templates                []*models.PackageTemplate
	ExpiringPackages         []*ExpiringPackageSummary
	OpenSafetyIncidents      []*models.SafetyIncident
	Locations                []*models.Location
	Categories               []*models.TrainingCategory
}

func (a *AppHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		a.renderOperationManagerDashboard(w, r, nil)
		return
	}

	if user.Role == models.RoleOperationManager {
		if r.URL.Query().Get("view") == "coach" {
			a.renderCoachDashboard(w, r, user)
			return
		}
		if r.URL.Query().Get("view") == "admin" {
			a.renderAdminDashboard(w, r, user)
			return
		}
	}

	switch user.Role {
	case models.RoleCoach:
		a.renderCoachDashboard(w, r, user)
		return
	case models.RoleAdmin:
		a.renderAdminDashboard(w, r, user)
		return
	case models.RoleStudent:
		http.Redirect(w, r, "/portal/student", http.StatusSeeOther)
		return
	default:
		a.renderOperationManagerDashboard(w, r, user)
		return
	}
}

func (a *AppHandler) renderCoachDashboard(w http.ResponseWriter, r *http.Request, user *models.User) {
	var coach *models.Coach
	if user.CoachID != nil {
		coach, _ = a.store.GetCoachByID(*user.CoachID)
	} else {
		// Fallback for admin or manager previewing coach dashboard
		coaches, _ := a.store.GetAllCoaches()
		if len(coaches) > 0 {
			coach = coaches[0]
		}
	}

	sessions, _ := a.store.GetAllSessions()
	now := time.Now()
	todayStr := now.Format("2006-01-02")

	var todaySessions []*models.TrainingSession
	var liveSess *models.TrainingSession

	if sid := r.URL.Query().Get("session_id"); sid != "" {
		if parsed, err := uuid.Parse(sid); err == nil {
			if s, err := a.store.GetSessionByID(parsed); err == nil {
				liveSess = s
			}
		}
	}

	for _, s := range sessions {
		if s.SessionDate.Format("2006-01-02") == todayStr {
			todaySessions = append(todaySessions, s)
		}
		if liveSess == nil && s.IsOpen() {
			if coach != nil && s.CoachID != nil && *s.CoachID == coach.ID {
				liveSess = s
			} else if liveSess == nil {
				liveSess = s
			}
		}
	}

	if liveSess == nil && len(todaySessions) > 0 {
		liveSess = todaySessions[0]
	} else if liveSess == nil && len(sessions) > 0 {
		liveSess = sessions[0]
	}

	var liveAtts []*models.Attendance
	if liveSess != nil {
		liveAtts, _ = a.store.GetSessionAttendances(liveSess.ID)
	}

	students, _ := a.store.GetAllStudents()
	allSessionsMap := make(map[string]*models.TrainingSession, len(sessions))
	for _, s := range sessions {
		allSessionsMap[s.ID.String()] = s
	}

	allAtts, _ := a.store.GetAllAttendances()
	studentAttendancesMap := make(map[uuid.UUID][]*models.Attendance, len(allAtts))
	for _, att := range allAtts {
		studentAttendancesMap[att.StudentID] = append(studentAttendancesMap[att.StudentID], att)
	}
	latestEvalsMap, _ := a.store.GetLatestEvaluations()

	var reviewQueue []StudentReadinessSummary
	for _, st := range students {
		atts := studentAttendancesMap[st.ID]
		latestEval := latestEvalsMap[st.ID]
		readiness := a.promotionSvc.EvaluateReadiness(st, atts, allSessionsMap, latestEval)
		if readiness.Status == services.StatusReady || readiness.Status == services.StatusPreTestEligible {
			reviewQueue = append(reviewQueue, StudentReadinessSummary{
				Student:   st,
				Readiness: readiness,
			})
		}
	}

	openIncidents, _ := a.store.GetSafetyIncidents(boolPtr(false))

	conductedCount := 0
	if coach != nil {
		for _, s := range sessions {
			if s.CoachID != nil && *s.CoachID == coach.ID {
				conductedCount++
			}
		}
	}

	categories, _ := a.store.GetAllTrainingCategories()
	locations, _ := a.store.GetAllLocations()

	data := CoachDashboardViewData{
		CurrentUser:           user,
		Coach:                 coach,
		TodaySessions:         todaySessions,
		LiveSession:           liveSess,
		LiveAttendances:       liveAtts,
		AllSessions:           sessions,
		AllStudents:           students,
		PriorityReviewQueue:   reviewQueue,
		OpenSafetyIncidents:   openIncidents,
		ClassesConductedCount: conductedCount,
		Categories:            categories,
		Locations:             locations,
	}

	a.RenderPage(w, "coach_dashboard.html", data)
}

func (a *AppHandler) renderAdminDashboard(w http.ResponseWriter, r *http.Request, user *models.User) {
	students, _ := a.store.GetAllStudents()
	coaches, _ := a.store.GetAllCoaches()
	sessions, _ := a.store.GetAllSessions()
	pkgTemplates, _ := a.store.GetPackageTemplates()
	openIncidents, _ := a.store.GetSafetyIncidents(boolPtr(false))
	locations, _ := a.store.GetAllLocations()
	categories, _ := a.store.GetAllTrainingCategories()
	admins, _ := a.store.GetUsersByRole(models.RoleAdmin)

	now := time.Now()
	todayStr := now.Format("2006-01-02")

	var todaySessions []*models.TrainingSession
	var liveSessions []*models.TrainingSession
	for _, s := range sessions {
		if s.SessionDate.Format("2006-01-02") == todayStr {
			todaySessions = append(todaySessions, s)
		}
		if s.IsOpen() {
			liveSessions = append(liveSessions, s)
		}
	}

	studentsMap := make(map[uuid.UUID]*models.Student, len(students))
	activeStudentsCount := 0
	for _, st := range students {
		studentsMap[st.ID] = st
		if st.IsActive {
			activeStudentsCount++
		}
	}

	templateMap := make(map[uuid.UUID]*models.PackageTemplate, len(pkgTemplates))
	for _, tpl := range pkgTemplates {
		templateMap[tpl.ID] = tpl
	}

	allPkgsMap, _ := a.store.GetAllStudentPackagesGrouped()
	activePkgsCount := 0
	var expiringSummaries []*ExpiringPackageSummary

	for _, pkgs := range allPkgsMap {
		for _, pkg := range pkgs {
			if pkg.IsValidAt(now) {
				activePkgsCount++
				daysLeft := int(pkg.ExpiryDate.Sub(now).Hours() / 24)
				if daysLeft < 0 {
					daysLeft = 0
				}

				remSessions := 0
				totalSessions := 0
				isUnlimited := pkg.TotalSessions == nil
				if pkg.RemainingSessions != nil {
					remSessions = *pkg.RemainingSessions
				}
				if pkg.TotalSessions != nil {
					totalSessions = *pkg.TotalSessions
				}

				isLowCredit := !isUnlimited && remSessions <= 2
				isExpiringSoon := daysLeft <= 7

				if isLowCredit || isExpiringSoon {
					stName := "Unknown Student"
					if st, ok := studentsMap[pkg.StudentID]; ok {
						stName = st.FullName
					}
					tplTitle := "Custom Membership Pass"
					if tpl, ok := templateMap[pkg.TemplateID]; ok {
						tplTitle = tpl.Title
					}

					expiringSummaries = append(expiringSummaries, &ExpiringPackageSummary{
						Package:           pkg,
						StudentID:         pkg.StudentID,
						StudentName:       stName,
						TemplateTitle:     tplTitle,
						RemainingSessions: remSessions,
						TotalSessions:     totalSessions,
						ExpiryDate:        pkg.ExpiryDate,
						DaysUntilExpiry:   daysLeft,
						IsLowCredit:       isLowCredit,
						IsExpiringSoon:    isExpiringSoon,
						IsUnlimited:       isUnlimited,
					})
				}
			}
		}
	}

	data := AdminDashboardViewData{
		CurrentUser:              user,
		ActiveStudentsCount:      activeStudentsCount,
		TodaySessionsCount:       len(todaySessions),
		TotalSessionsCount:       len(sessions),
		ActivePackagesCount:      activePkgsCount,
		ExpiringPackagesCount:    len(expiringSummaries),
		OpenSafetyIncidentsCount: len(openIncidents),
		TodaySessions:            todaySessions,
		LiveSessions:             liveSessions,
		AllStudents:              students,
		AllCoaches:               coaches,
		AllAdmins:                admins,
		Templates:                pkgTemplates,
		ExpiringPackages:         expiringSummaries,
		OpenSafetyIncidents:      openIncidents,
		Locations:                locations,
		Categories:               categories,
	}

	a.RenderPage(w, "admin_dashboard.html", data)
}

func (a *AppHandler) renderOperationManagerDashboard(w http.ResponseWriter, r *http.Request, user *models.User) {
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

