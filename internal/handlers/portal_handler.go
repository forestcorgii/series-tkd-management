package handlers

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/services"
)

// View Data Structs for Portals
type StudentPortalData struct {
	CurrentUser            *models.User
	Student                *models.Student
	Readiness              services.PromotionReadiness
	SessionCompletionPct   int
	TenureCompletionPct    int
	LatestEvaluation       *models.StudentEvaluation
	SVGRadarPolygon        string
	ActivePackage          *models.StudentPackage
	PastPackages           []*models.StudentPackage
	RecentAttendances      []*models.Attendance
	UpcomingSessions       []*models.TrainingSession
	HasActiveSafetyHold    bool
}

type CoachPortalData struct {
	CurrentUser          *models.User
	Coach                *models.Coach
	LiveSession          *models.TrainingSession
	LiveAttendances      []*models.Attendance
	AllSessions          []*models.TrainingSession
	AllStudents          []*models.Student
	PriorityReviewQueue  []StudentReadinessSummary
	OpenSafetyIncidents  []*models.SafetyIncident
}

type AdminPortalData struct {
	CurrentUser          *models.User
	ActiveStudentsCount  int
	TotalSessionsCount   int
	ActiveCoachesCount   int
	OpenIncidentsCount   int
	StudentsWithReadiness []StudentReadinessSummary
	SafetyIncidents      []*models.SafetyIncident
	Coaches              []*models.Coach
	PackageTemplates     []*models.PackageTemplate
	Sessions             []*models.TrainingSession
}

func (a *AppHandler) HandleStudentPortal(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var targetStudentID uuid.UUID
	if user.StudentID != nil && *user.StudentID != uuid.Nil {
		targetStudentID = *user.StudentID
	} else {
		// If Admin/Coach viewing or student_id query param
		if qID := r.URL.Query().Get("student_id"); qID != "" {
			if parsed, err := uuid.Parse(qID); err == nil {
				targetStudentID = parsed
			}
		}
		if targetStudentID == uuid.Nil {
			students, _ := a.store.GetAllStudents()
			if len(students) > 0 {
				targetStudentID = students[0].ID
			}
		}
	}

	student, err := a.store.GetStudentByID(targetStudentID)
	if err != nil {
		http.Error(w, "Student record not found", http.StatusNotFound)
		return
	}

	attendances, _ := a.store.GetStudentAttendances(student.ID)
	sessions, _ := a.store.GetAllSessions()
	allSessionsMap := make(map[string]*models.TrainingSession, len(sessions))
	for _, s := range sessions {
		allSessionsMap[s.ID.String()] = s
	}

	latestEval, _ := a.store.GetLatestEvaluation(student.ID)
	readiness := a.promotionSvc.EvaluateReadiness(student, attendances, allSessionsMap, latestEval)

	sessPct := 0
	if readiness.RequiredSessions > 0 {
		sessPct = int(math.Min(100, float64(readiness.TotalSessions)/float64(readiness.RequiredSessions)*100.0))
	}
	tenurePct := 0
	if readiness.RequiredDaysInRank > 0 {
		tenurePct = int(math.Min(100, float64(readiness.DaysInRank)/float64(readiness.RequiredDaysInRank)*100.0))
	}

	polygon := ""
	if latestEval != nil {
		polygon = latestEval.ToSVGPolygon(100, 100, 75)
	}

	pkgs, _ := a.store.GetStudentPackages(student.ID)
	var activePkg *models.StudentPackage
	var pastPkgs []*models.StudentPackage
	now := time.Now()
	for _, p := range pkgs {
		if p.IsValidAt(now) && activePkg == nil {
			activePkg = p
		} else {
			pastPkgs = append(pastPkgs, p)
		}
	}

	// Upcoming sessions (today or future)
	var upcoming []*models.TrainingSession
	for _, s := range sessions {
		if s.SessionDate.Equal(now) || s.SessionDate.After(now.AddDate(0, 0, -1)) {
			upcoming = append(upcoming, s)
		}
	}

	data := StudentPortalData{
		CurrentUser:          user,
		Student:              student,
		Readiness:            readiness,
		SessionCompletionPct: sessPct,
		TenureCompletionPct:  tenurePct,
		LatestEvaluation:     latestEval,
		SVGRadarPolygon:      polygon,
		ActivePackage:        activePkg,
		PastPackages:         pastPkgs,
		RecentAttendances:    attendances,
		UpcomingSessions:     upcoming,
		HasActiveSafetyHold:  student.HasSafetyFlag,
	}

	a.RenderPage(w, "student_portal.html", data)
}

func (a *AppHandler) HandleCoachPortal(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var coach *models.Coach
	if user.CoachID != nil {
		coach, _ = a.store.GetCoachByID(*user.CoachID)
	}

	sessions, _ := a.store.GetAllSessions()
	var liveSess *models.TrainingSession
	if len(sessions) > 0 {
		liveSess = sessions[0]
	}

	// If session_id query specified
	if sid := r.URL.Query().Get("session_id"); sid != "" {
		if parsed, err := uuid.Parse(sid); err == nil {
			if s, err := a.store.GetSessionByID(parsed); err == nil {
				liveSess = s
			}
		}
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

	// Build priority review queue: Students who are READY or PRE-TEST ELIGIBLE
	var reviewQueue []StudentReadinessSummary
	for _, st := range students {
		atts, _ := a.store.GetStudentAttendances(st.ID)
		latestEval, _ := a.store.GetLatestEvaluation(st.ID)
		readiness := a.promotionSvc.EvaluateReadiness(st, atts, allSessionsMap, latestEval)
		if readiness.Status == services.StatusReady || readiness.Status == services.StatusPreTestEligible {
			reviewQueue = append(reviewQueue, StudentReadinessSummary{
				Student:   st,
				Readiness: readiness,
			})
		}
	}

	openIncidents, _ := a.store.GetSafetyIncidents(boolPtr(false))

	data := CoachPortalData{
		CurrentUser:         user,
		Coach:               coach,
		LiveSession:         liveSess,
		LiveAttendances:     liveAtts,
		AllSessions:         sessions,
		AllStudents:         students,
		PriorityReviewQueue: reviewQueue,
		OpenSafetyIncidents: openIncidents,
	}

	a.RenderPage(w, "coach_portal.html", data)
}

func (a *AppHandler) HandleAdminPortal(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	students, _ := a.store.GetAllStudents()
	coaches, _ := a.store.GetAllCoaches()
	sessions, _ := a.store.GetAllSessions()
	pkgTemplates, _ := a.store.GetPackageTemplates()
	incidents, _ := a.store.GetSafetyIncidents(nil)

	openCount := 0
	for _, inc := range incidents {
		if !inc.Resolved {
			openCount++
		}
	}

	allSessionsMap := make(map[string]*models.TrainingSession, len(sessions))
	for _, s := range sessions {
		allSessionsMap[s.ID.String()] = s
	}

	var readinessList []StudentReadinessSummary
	for _, st := range students {
		atts, _ := a.store.GetStudentAttendances(st.ID)
		latestEval, _ := a.store.GetLatestEvaluation(st.ID)
		readiness := a.promotionSvc.EvaluateReadiness(st, atts, allSessionsMap, latestEval)
		readinessList = append(readinessList, StudentReadinessSummary{
			Student:   st,
			Readiness: readiness,
		})
	}

	data := AdminPortalData{
		CurrentUser:           user,
		ActiveStudentsCount:   len(students),
		TotalSessionsCount:    len(sessions),
		ActiveCoachesCount:    len(coaches),
		OpenIncidentsCount:    openCount,
		StudentsWithReadiness: readinessList,
		SafetyIncidents:       incidents,
		Coaches:               coaches,
		PackageTemplates:      pkgTemplates,
		Sessions:              sessions,
	}

	a.RenderPage(w, "admin_portal.html", data)
}

// REST & HTMX APIs per Section 4

// 1. GET /api/student/readiness
func (a *AppHandler) HandleAPIStudentReadiness(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var targetStudentID uuid.UUID
	if user.Role == models.RoleStudent {
		if user.StudentID == nil {
			http.Error(w, "No student profile linked", http.StatusBadRequest)
			return
		}
		targetStudentID = *user.StudentID
	} else {
		// Admin/Coach can query by ?student_id=...
		qID := r.URL.Query().Get("student_id")
		if qID == "" {
			http.Error(w, "student_id parameter required", http.StatusBadRequest)
			return
		}
		var err error
		targetStudentID, err = uuid.Parse(qID)
		if err != nil {
			http.Error(w, "Invalid student_id", http.StatusBadRequest)
			return
		}
	}

	student, err := a.store.GetStudentByID(targetStudentID)
	if err != nil {
		http.Error(w, "Student not found", http.StatusNotFound)
		return
	}

	attendances, _ := a.store.GetStudentAttendances(student.ID)
	sessions, _ := a.store.GetAllSessions()
	allSessionsMap := make(map[string]*models.TrainingSession, len(sessions))
	for _, s := range sessions {
		allSessionsMap[s.ID.String()] = s
	}
	latestEval, _ := a.store.GetLatestEvaluation(student.ID)
	readiness := a.promotionSvc.EvaluateReadiness(student, attendances, allSessionsMap, latestEval)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"student":    student,
		"readiness":  readiness,
		"evaluation": latestEval,
	})
}

// 2. GET /api/coach/sessions/live
func (a *AppHandler) HandleAPICoachLiveSession(w http.ResponseWriter, r *http.Request) {
	sessions, _ := a.store.GetAllSessions()
	if len(sessions) == 0 {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"session": nil, "roster": []interface{}{}})
		return
	}
	liveSess := sessions[0]
	atts, _ := a.store.GetSessionAttendances(liveSess.ID)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"session": liveSess,
		"roster":  atts,
	})
}

// 3. POST /api/coach/check-in
type CoachCheckInRequest struct {
	SessionID uuid.UUID `json:"session_id"`
	StudentID uuid.UUID `json:"student_id"`
}

func (a *AppHandler) HandleAPICoachCheckIn(w http.ResponseWriter, r *http.Request) {
	var req CoachCheckInRequest
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}
	} else {
		_ = r.ParseForm()
		req.SessionID, _ = uuid.Parse(r.FormValue("session_id"))
		req.StudentID, _ = uuid.Parse(r.FormValue("student_id"))
	}

	if req.SessionID == uuid.Nil || req.StudentID == uuid.Nil {
		http.Error(w, "session_id and student_id required", http.StatusBadRequest)
		return
	}

	// Find oldest valid package to decrement
	pkgs, _ := a.store.GetStudentPackages(req.StudentID)
	var chosenPkg *models.StudentPackage
	for _, p := range pkgs {
		if p.IsValidAt(time.Now()) {
			chosenPkg = p
			break
		}
	}

	var pkgID *uuid.UUID
	if chosenPkg != nil {
		pkgID = &chosenPkg.ID
	}

	att, err := a.store.CheckInStudent(req.SessionID, req.StudentID, pkgID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Auto-decrement 1 session from package if capped
	if chosenPkg != nil && chosenPkg.RemainingSessions != nil && *chosenPkg.RemainingSessions > 0 {
		newRemaining := *chosenPkg.RemainingSessions - 1
		chosenPkg.RemainingSessions = &newRemaining
		_ = a.store.UpdateStudentPackage(chosenPkg)
	}

	if r.Header.Get("HX-Request") == "true" {
		a.RenderPartial(w, "checkin_row.html", att)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "checked_in",
		"attendance": att,
	})
}

// 4. POST /api/coach/evaluate
func (a *AppHandler) HandleAPICoachEvaluate(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	_ = r.ParseForm()

	studentID, err := uuid.Parse(r.FormValue("student_id"))
	if err != nil {
		http.Error(w, "Invalid student_id", http.StatusBadRequest)
		return
	}

	var coachID uuid.UUID
	if user != nil && user.CoachID != nil {
		coachID = *user.CoachID
	} else if cID := r.FormValue("coach_id"); cID != "" {
		coachID, _ = uuid.Parse(cID)
	}
	if coachID == uuid.Nil {
		coaches, _ := a.store.GetAllCoaches()
		if len(coaches) > 0 {
			coachID = coaches[0].ID
		}
	}

	parseInt := func(val string) int {
		n, _ := strconv.Atoi(val)
		if n < 1 {
			n = 1
		}
		if n > 10 {
			n = 10
		}
		return n
	}

	eval := &models.StudentEvaluation{
		ID:             uuid.New(),
		StudentID:      studentID,
		CoachID:        coachID,
		EvaluationDate: time.Now(),
		Flexibility:    parseInt(r.FormValue("flexibility")),
		Stamina:        parseInt(r.FormValue("stamina")),
		Power:          parseInt(r.FormValue("power")),
		Technique:      parseInt(r.FormValue("technique")),
		SparringIQ:     parseInt(r.FormValue("sparring_iq")),
		Discipline:     parseInt(r.FormValue("discipline")),
		CoachRemarks:   strings.TrimSpace(r.FormValue("coach_remarks")),
		CreatedAt:      time.Now(),
	}

	if err := a.store.CreateEvaluation(eval); err != nil {
		http.Error(w, fmt.Sprintf("Failed to save evaluation: %v", err), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<div class="p-3 bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 rounded-lg text-xs font-semibold">Evaluation score saved successfully!</div>`)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "ok",
		"evaluation": eval,
	})
}

// 5. POST /api/safety/flag
func (a *AppHandler) HandleAPISafetyFlag(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	_ = r.ParseForm()

	studentID, err := uuid.Parse(r.FormValue("student_id"))
	if err != nil {
		http.Error(w, "Invalid student_id", http.StatusBadRequest)
		return
	}

	incidentType := strings.TrimSpace(r.FormValue("incident_type"))
	if incidentType == "" {
		incidentType = "Floor Medical / First Aid"
	}
	notes := strings.TrimSpace(r.FormValue("notes"))

	var coachID *uuid.UUID
	if user != nil && user.CoachID != nil {
		coachID = user.CoachID
	}

	incident := &models.SafetyIncident{
		ID:           uuid.New(),
		StudentID:    studentID,
		CoachID:      coachID,
		IncidentType: incidentType,
		Notes:        notes,
		Resolved:     false,
		CreatedAt:    time.Now(),
	}

	if err := a.store.CreateSafetyIncident(incident); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<div class="p-3 bg-rose-500/15 border border-rose-500/30 text-rose-400 rounded-lg text-xs font-semibold">🚨 Safety incident logged. Mat hold activated for practitioner.</div>`)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "flagged",
		"incident": incident,
	})
}

// 6. POST/PATCH /api/safety/resolve
func (a *AppHandler) HandleAPISafetyResolve(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	adminEmail := "admin@seriestkd.com"
	if user != nil {
		adminEmail = user.Email
	}

	incidentIDStr := r.URL.Query().Get("id")
	if incidentIDStr == "" {
		_ = r.ParseForm()
		incidentIDStr = r.FormValue("incident_id")
	}

	incidentID, err := uuid.Parse(incidentIDStr)
	if err != nil {
		http.Error(w, "Invalid incident_id", http.StatusBadRequest)
		return
	}

	if err := a.store.ResolveSafetyIncident(incidentID, adminEmail); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<div class="p-2.5 bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 rounded-lg text-xs font-semibold">Incident cleared &amp; practitioner cleared for mat activity.</div>`)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status":  "resolved",
		"message": "Safety incident resolved successfully",
	})
}

// 7. POST /api/admin/promote
func (a *AppHandler) HandleAPIAdminPromote(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	studentID, err := uuid.Parse(r.FormValue("student_id"))
	if err != nil {
		http.Error(w, "Invalid student_id", http.StatusBadRequest)
		return
	}

	student, err := a.store.GetStudentByID(studentID)
	if err != nil {
		http.Error(w, "Student not found", http.StatusNotFound)
		return
	}

	newBelt := student.NextBelt()
	if customBelt := r.FormValue("new_belt"); customBelt != "" {
		newBelt = models.BeltRank(customBelt)
	}

	if err := a.store.PromoteStudent(studentID, newBelt); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<span class="inline-flex items-center px-2 py-0.5 rounded text-xs font-bold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">Promoted to %s</span>`, string(newBelt))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "promoted",
		"new_belt": newBelt,
	})
}

// 8. POST/PUT /api/admin/schedule
func (a *AppHandler) HandleAPIAdminSchedule(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()

	coachID, err := uuid.Parse(r.FormValue("coach_id"))
	if err != nil {
		http.Error(w, "Invalid coach_id", http.StatusBadRequest)
		return
	}

	dateStr := r.FormValue("session_date")
	sessionDate := time.Now()
	if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
		sessionDate = parsed
	}

	sess := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  sessionDate,
		StartTime:    r.FormValue("start_time"),
		EndTime:      r.FormValue("end_time"),
		CoachID:      coachID,
		TrainingType: models.TrainingType(r.FormValue("discipline")),
		Notes:        strings.TrimSpace(r.FormValue("notes")),
		CreatedAt:    time.Now(),
	}

	if err := a.store.CreateSession(sess); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<div class="p-3 bg-emerald-500/10 border border-emerald-500/30 text-emerald-400 rounded-lg text-xs font-semibold">New floor session added to timetable!</div>`)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "created",
		"session": sess,
	})
}

func boolPtr(b bool) *bool {
	return &b
}
