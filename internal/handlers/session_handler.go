package handlers

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/services"
)

type SessionListItem struct {
	Session         *models.TrainingSession
	AttendanceCount int
}

type SessionsPageData struct {
	Sessions []*models.TrainingSession
	Coaches  []*models.Coach
}

type LiveCheckInPageData struct {
	Session            *models.TrainingSession
	Attendances        []*models.Attendance
	Students           []*models.Student
	ReadinessMap       map[string]services.PromotionReadiness
	PackageStatusMap   map[string]string
}

type StudentSearchResultItem struct {
	SessionID        uuid.UUID
	Student          *models.Student
	Readiness        services.PromotionReadiness
	ActivePackage    *models.StudentPackage
	IsAlreadyChecked bool
	WarningMessage   string
}

func (a *AppHandler) HandleSessions(w http.ResponseWriter, r *http.Request) {
	sessions, err := a.store.GetAllSessions()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	coaches, _ := a.store.GetAllCoaches()

	data := SessionsPageData{
		Sessions: sessions,
		Coaches:  coaches,
	}

	a.RenderPage(w, "sessions.html", data)
}

func (a *AppHandler) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	coachID, _ := uuid.Parse(r.FormValue("coach_id"))
	dateStr := r.FormValue("session_date")
	sessDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		sessDate = time.Now()
	}

	sess := &models.TrainingSession{
		SessionDate:  sessDate,
		StartTime:    r.FormValue("start_time"),
		EndTime:      r.FormValue("end_time"),
		CoachID:      coachID,
		TrainingType: models.TrainingType(r.FormValue("training_type")),
		Notes:        r.FormValue("notes"),
	}

	if err := a.store.CreateSession(sess); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/sessions/"+sess.ID.String()+"/live", http.StatusSeeOther)
}

func (a *AppHandler) HandleLiveSession(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/sessions/")
	idStr = strings.TrimSuffix(idStr, "/live")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	session, err := a.store.GetSessionByID(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	attendances, _ := a.store.GetSessionAttendances(sessionID)
	allStudents, _ := a.store.GetAllStudents()
	allSessions, _ := a.store.GetAllSessions()

	allSessionsMap := make(map[string]*models.TrainingSession)
	for _, s := range allSessions {
		allSessionsMap[s.ID.String()] = s
	}

	readinessMap := make(map[string]services.PromotionReadiness)
	pkgStatusMap := make(map[string]string)

	for _, st := range allStudents {
		atts, _ := a.store.GetStudentAttendances(st.ID)
		latestEval, _ := a.store.GetLatestEvaluation(st.ID)
		rStatus := a.promotionSvc.EvaluateReadiness(st, atts, allSessionsMap, latestEval)
		readinessMap[st.ID.String()] = rStatus

		pkgs, _ := a.store.GetStudentPackages(st.ID)
		oldestValid, err := a.packageSvc.FindOldestValidPackage(pkgs, time.Now())
		if err != nil {
			pkgStatusMap[st.ID.String()] = "No Active Credits ⚠️"
		} else if oldestValid.RemainingSessions != nil {
			pkgStatusMap[st.ID.String()] = fmt.Sprintf("%d sessions left", *oldestValid.RemainingSessions)
		} else {
			pkgStatusMap[st.ID.String()] = "Unlimited Pass ♾️"
		}
	}

	data := LiveCheckInPageData{
		Session:          session,
		Attendances:      attendances,
		Students:         allStudents,
		ReadinessMap:     readinessMap,
		PackageStatusMap: pkgStatusMap,
	}

	a.RenderPage(w, "live_checkin.html", data)
}

func (a *AppHandler) HandleSearchStudent(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/sessions/")
	idStr = strings.TrimSuffix(idStr, "/search-student")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	query := r.FormValue("query")
	students, _ := a.store.SearchStudents(query)
	existingAttendances, _ := a.store.GetSessionAttendances(sessionID)

	checkedMap := make(map[string]bool)
	for _, att := range existingAttendances {
		checkedMap[att.StudentID.String()] = true
	}

	allSessions, _ := a.store.GetAllSessions()
	allSessionsMap := make(map[string]*models.TrainingSession)
	for _, s := range allSessions {
		allSessionsMap[s.ID.String()] = s
	}

	results := []StudentSearchResultItem{}
	for _, st := range students {
		atts, _ := a.store.GetStudentAttendances(st.ID)
		latestEval, _ := a.store.GetLatestEvaluation(st.ID)
		readiness := a.promotionSvc.EvaluateReadiness(st, atts, allSessionsMap, latestEval)

		pkgs, _ := a.store.GetStudentPackages(st.ID)
		validPkg, pkgErr := a.packageSvc.FindOldestValidPackage(pkgs, time.Now())

		warning := ""
		if pkgErr != nil {
			warning = "EXPIRED / ZERO CREDITS - Override Required"
		}

		results = append(results, StudentSearchResultItem{
			SessionID:        sessionID,
			Student:          st,
			Readiness:        readiness,
			ActivePackage:    validPkg,
			IsAlreadyChecked: checkedMap[st.ID.String()],
			WarningMessage:   warning,
		})
	}

	a.RenderPartial(w, "search_results.html", results)
}

func (a *AppHandler) HandleCheckIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	// /sessions/{sessionID}/checkin/{studentID}
	if len(pathParts) < 5 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	sessionID, err1 := uuid.Parse(pathParts[2])
	studentID, err2 := uuid.Parse(pathParts[4])
	if err1 != nil || err2 != nil {
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		return
	}

	pkgs, _ := a.store.GetStudentPackages(studentID)
	override := r.FormValue("override") == "true"

	usedPkg, err := a.packageSvc.ProcessCheckInDeduction(pkgs, time.Now())
	var pkgID *uuid.UUID
	if err == nil && usedPkg != nil {
		pkgID = &usedPkg.ID
		_ = a.store.UpdateStudentPackage(usedPkg)
	} else if !override {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusPaymentRequired)
		w.Write([]byte(fmt.Sprintf(`<div class="p-3 bg-rose-950 border border-rose-800 text-rose-300 rounded-lg text-sm">
			❌ Check-in rejected: %v. <button hx-post="/sessions/%s/checkin/%s?override=true" hx-target="#attendance-roster" hx-swap="afterbegin" class="underline font-bold ml-2">Manual Override</button>
		</div>`, err, sessionID, studentID)))
		return
	}

	att, err := a.store.CheckInStudent(sessionID, studentID, pkgID)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(fmt.Sprintf(`<div class="p-3 bg-amber-950 border border-amber-800 text-amber-300 rounded-lg text-sm">⚠️ %v</div>`, err)))
		return
	}

	st, _ := a.store.GetStudentByID(studentID)
	allSessions, _ := a.store.GetAllSessions()
	allSessionsMap := make(map[string]*models.TrainingSession)
	for _, s := range allSessions {
		allSessionsMap[s.ID.String()] = s
	}

	atts, _ := a.store.GetStudentAttendances(studentID)
	latestEval, _ := a.store.GetLatestEvaluation(studentID)
	readiness := a.promotionSvc.EvaluateReadiness(st, atts, allSessionsMap, latestEval)

	data := struct {
		Attendance *models.Attendance
		Readiness  services.PromotionReadiness
	}{
		Attendance: att,
		Readiness:  readiness,
	}

	a.RenderPartial(w, "checkin_row.html", data)
}
