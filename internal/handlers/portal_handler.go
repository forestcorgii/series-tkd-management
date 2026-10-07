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
	CheckedInSessions      map[string]bool
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
	PendingCoachesCount  int
	PendingAdminsCount   int
	StudentsWithReadiness []StudentReadinessSummary
	SafetyIncidents      []*models.SafetyIncident
	Coaches              []*models.Coach
	PackageTemplates     []*models.PackageTemplate
	Sessions             []*models.TrainingSession
	Locations            []*models.Location
	TrainingCategories   []*models.TrainingCategory
}

func (a *AppHandler) HandleStudentPortal(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var targetStudentID uuid.UUID
	if user.Role == models.RoleStudent {
		if user.StudentID == nil || *user.StudentID == uuid.Nil {
			http.Error(w, "No student profile linked to your account", http.StatusBadRequest)
			return
		}
		targetStudentID = *user.StudentID
	} else {
		// If Coach or Operation Manager viewing
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

	checkedInMap := make(map[string]bool, len(attendances))
	for _, att := range attendances {
		checkedInMap[att.SessionID.String()] = true
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
		CheckedInSessions:    checkedInMap,
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

	allAtts, _ := a.store.GetAllAttendances()
	studentAttendancesMap := make(map[uuid.UUID][]*models.Attendance, len(allAtts))
	for _, a := range allAtts {
		studentAttendancesMap[a.StudentID] = append(studentAttendancesMap[a.StudentID], a)
	}
	latestEvalsMap, _ := a.store.GetLatestEvaluations()

	// Build priority review queue: Students who are READY or PRE-TEST ELIGIBLE
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

	allAtts, _ := a.store.GetAllAttendances()
	studentAttendancesMap := make(map[uuid.UUID][]*models.Attendance, len(allAtts))
	for _, a := range allAtts {
		studentAttendancesMap[a.StudentID] = append(studentAttendancesMap[a.StudentID], a)
	}
	latestEvalsMap, _ := a.store.GetLatestEvaluations()

	var readinessList []StudentReadinessSummary
	for _, st := range students {
		atts := studentAttendancesMap[st.ID]
		latestEval := latestEvalsMap[st.ID]
		readiness := a.promotionSvc.EvaluateReadiness(st, atts, allSessionsMap, latestEval)
		readinessList = append(readinessList, StudentReadinessSummary{
			Student:   st,
			Readiness: readiness,
		})
	}

	pendingCoaches := 0
	for _, c := range coaches {
		if !c.IsActive {
			u, _ := a.store.GetUserByEmail(c.Email)
			if u != nil && u.LastLoginAt == nil {
				pendingCoaches++
			}
		}
	}

	pendingAdmins := 0
	admins, _ := a.store.GetUsersByRole(models.RoleAdmin)
	for _, adm := range admins {
		if !adm.IsActive && adm.LastLoginAt == nil {
			pendingAdmins++
		}
	}

	locations, _ := a.store.GetAllLocations()
	categories, _ := a.store.GetAllTrainingCategories()

	data := AdminPortalData{
		CurrentUser:           user,
		ActiveStudentsCount:   len(students),
		TotalSessionsCount:    len(sessions),
		ActiveCoachesCount:    len(coaches),
		OpenIncidentsCount:    openCount,
		PendingCoachesCount:   pendingCoaches,
		PendingAdminsCount:    pendingAdmins,
		StudentsWithReadiness: readinessList,
		SafetyIncidents:       incidents,
		Coaches:               coaches,
		PackageTemplates:      pkgTemplates,
		Sessions:              sessions,
		Locations:             locations,
		TrainingCategories:    categories,
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

// 2b. POST /api/student/check-in
type StudentCheckInItemData struct {
	Session         *models.TrainingSession
	StudentID       uuid.UUID
	IsCheckedIn     bool
	HasSafetyHold   bool
	FeedbackMessage string
	IsSuccess       bool
}

func (a *AppHandler) HandleAPIStudentCheckIn(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		if isJSONRequest(r) && !isHTMXRequest(r) {
			writeJSONResponse(w, http.StatusUnauthorized, map[string]interface{}{
				"error":   "unauthorized",
				"message": "Authentication required",
			})
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	var sessionID uuid.UUID
	var studentID uuid.UUID
	var sessionRate *float64

	if isJSONRequest(r) && !isHTMXRequest(r) {
		var req struct {
			SessionID   uuid.UUID `json:"session_id"`
			StudentID   uuid.UUID `json:"student_id"`
			SessionRate *float64  `json:"session_rate,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONResponse(w, http.StatusBadRequest, map[string]interface{}{
				"error": "Invalid JSON body",
			})
			return
		}
		sessionID = req.SessionID
		studentID = req.StudentID
		sessionRate = req.SessionRate
	} else {
		_ = r.ParseForm()
		sessionID, _ = uuid.Parse(r.FormValue("session_id"))
		studentID, _ = uuid.Parse(r.FormValue("student_id"))
		if rateStr := strings.TrimSpace(r.FormValue("session_rate")); rateStr != "" {
			if val, err := strconv.ParseFloat(rateStr, 64); err == nil && val >= 0 {
				sessionRate = &val
			}
		}
	}

	// For student role, strictly enforce self check-in
	if user.Role == models.RoleStudent {
		if user.StudentID == nil || *user.StudentID == uuid.Nil {
			if isHTMXRequest(r) {
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusOK)
				fmt.Fprint(w, `<div class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-700 dark:text-rose-300 text-xs font-semibold">⚠️ No student profile is linked to your account.</div>`)
				return
			}
			writeJSONResponse(w, http.StatusBadRequest, map[string]interface{}{
				"error": "No student profile linked to your account",
			})
			return
		}
		studentID = *user.StudentID
	} else if studentID == uuid.Nil && user.StudentID != nil {
		studentID = *user.StudentID
	}

	if sessionID == uuid.Nil || studentID == uuid.Nil {
		if isHTMXRequest(r) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `<div class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-700 dark:text-rose-300 text-xs font-semibold">⚠️ session_id and student_id are required.</div>`)
			return
		}
		writeJSONResponse(w, http.StatusBadRequest, map[string]interface{}{
			"error": "session_id and student_id required",
		})
		return
	}

	session, err := a.store.GetSessionByID(sessionID)
	if err != nil || session == nil {
		if isHTMXRequest(r) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `<div class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-700 dark:text-rose-300 text-xs font-semibold">⚠️ Scheduled class not found.</div>`)
			return
		}
		writeJSONResponse(w, http.StatusNotFound, map[string]interface{}{
			"error": "Session not found",
		})
		return
	}

	if sessionRate == nil && session != nil && session.SessionRate != nil {
		sessionRate = session.SessionRate
	}

	student, err := a.store.GetStudentByID(studentID)
	if err != nil || student == nil {
		if isHTMXRequest(r) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `<div class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-700 dark:text-rose-300 text-xs font-semibold">⚠️ Student profile not found.</div>`)
			return
		}
		writeJSONResponse(w, http.StatusNotFound, map[string]interface{}{
			"error": "Student not found",
		})
		return
	}

	itemData := StudentCheckInItemData{
		Session:       session,
		StudentID:     studentID,
		IsCheckedIn:   false,
		HasSafetyHold: student.HasSafetyFlag,
	}

	if session.IsCancelled {
		itemData.FeedbackMessage = "Check-in rejected: This class has been cancelled."
		itemData.IsSuccess = false
		if isHTMXRequest(r) {
			a.RenderPartial(w, "student_session_item.html", itemData)
			return
		}
		writeJSONResponse(w, http.StatusBadRequest, map[string]interface{}{
			"error": "Session is cancelled",
		})
		return
	}

	if student.HasSafetyFlag {
		itemData.FeedbackMessage = "Mat Safety Hold Active: First aid clearance required by coach before stepping on the mat."
		itemData.IsSuccess = false
		if isHTMXRequest(r) {
			a.RenderPartial(w, "student_session_item.html", itemData)
			return
		}
		writeJSONResponse(w, http.StatusForbidden, map[string]interface{}{
			"error": "Safety hold active",
		})
		return
	}

	// Check if already checked in
	atts, _ := a.store.GetStudentAttendances(studentID)
	for _, att := range atts {
		if att.SessionID == sessionID {
			itemData.IsCheckedIn = true
			itemData.FeedbackMessage = "You are already checked in to this class."
			itemData.IsSuccess = true
			if isHTMXRequest(r) {
				w.Header().Set("HX-Trigger", "attendanceUpdated")
				a.RenderPartial(w, "student_session_item.html", itemData)
				return
			}
			writeJSONResponse(w, http.StatusOK, map[string]interface{}{
				"status":     "already_checked_in",
				"attendance": att,
			})
			return
		}
	}

	var pkgID *uuid.UUID
	var usedPkg *models.StudentPackage

	if sessionRate != nil {
		// School / Fix-Rate check-in: do not deduct or change package session credit
	} else {
		// Process package deduction
		pkgs, _ := a.store.GetStudentPackages(studentID)
		var err error
		usedPkg, err = a.packageSvc.ProcessCheckInDeduction(pkgs, atts, time.Now())
		if err != nil {
			itemData.FeedbackMessage = fmt.Sprintf("Check-in rejected: %v. Please see front desk for membership renewal.", err)
			itemData.IsSuccess = false
			if isHTMXRequest(r) {
				a.RenderPartial(w, "student_session_item.html", itemData)
				return
			}
			writeJSONResponse(w, http.StatusPaymentRequired, map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		if usedPkg != nil {
			pkgID = &usedPkg.ID
			_ = a.store.UpdateStudentPackage(usedPkg)
		}
	}

	att, err := a.store.CheckInStudent(sessionID, studentID, pkgID, sessionRate)
	if err != nil {
		itemData.FeedbackMessage = fmt.Sprintf("Check-in failed: %v", err)
		itemData.IsSuccess = false
		if isHTMXRequest(r) {
			a.RenderPartial(w, "student_session_item.html", itemData)
			return
		}
		writeJSONResponse(w, http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	w.Header().Set("HX-Trigger", "attendanceUpdated")

	if isHTMXRequest(r) {
		itemData.IsCheckedIn = true
		itemData.IsSuccess = true
		if sessionRate != nil {
			itemData.FeedbackMessage = fmt.Sprintf("Checked in successfully to %s (%s - %s). School / Fix Rate: ₱%.2f.", session.TrainingType, session.StartTime, session.EndTime, *sessionRate)
		} else {
			itemData.FeedbackMessage = fmt.Sprintf("Checked in successfully to %s (%s - %s). 1 class credit deducted.", session.TrainingType, session.StartTime, session.EndTime)
		}
		a.RenderPartial(w, "student_session_item.html", itemData)
		return
	}

	writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"status":     "checked_in",
		"attendance": att,
		"package":    usedPkg,
	})
}

// 3. POST /api/coach/check-in
type CoachCheckInRequest struct {
	SessionID   uuid.UUID `json:"session_id"`
	StudentID   uuid.UUID `json:"student_id"`
	SessionRate *float64  `json:"session_rate,omitempty"`
}

func (a *AppHandler) HandleAPICoachCheckIn(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user != nil && user.Role == models.RoleStudent {
		a.HandleAPIStudentCheckIn(w, r)
		return
	}

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
		if rateStr := strings.TrimSpace(r.FormValue("session_rate")); rateStr != "" {
			if val, err := strconv.ParseFloat(rateStr, 64); err == nil && val >= 0 {
				req.SessionRate = &val
			}
		}
	}

	if req.SessionID == uuid.Nil || req.StudentID == uuid.Nil {
		http.Error(w, "session_id and student_id required", http.StatusBadRequest)
		return
	}

	if req.SessionRate == nil {
		if sess, _ := a.store.GetSessionByID(req.SessionID); sess != nil && sess.SessionRate != nil {
			req.SessionRate = sess.SessionRate
		}
	}

	atts, _ := a.store.GetStudentAttendances(req.StudentID)
	var pkgID *uuid.UUID
	var usedPkg *models.StudentPackage

	if req.SessionRate != nil {
		// School / Fix-Rate check-in: do not deduct or change package session credit
	} else {
		// Find oldest valid package to decrement respecting weekly cadence
		pkgs, _ := a.store.GetStudentPackages(req.StudentID)
		var err error
		usedPkg, err = a.packageSvc.ProcessCheckInDeduction(pkgs, atts, time.Now())
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if usedPkg != nil {
			pkgID = &usedPkg.ID
			_ = a.store.UpdateStudentPackage(usedPkg)
		}
	}

	att, err := a.store.CheckInStudent(req.SessionID, req.StudentID, pkgID, req.SessionRate)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		st, _ := a.store.GetStudentByID(req.StudentID)
		allSessions, _ := a.store.GetAllSessions()
		allSessionsMap := make(map[string]*models.TrainingSession, len(allSessions))
		for _, s := range allSessions {
			allSessionsMap[s.ID.String()] = s
		}
		latestEval, _ := a.store.GetLatestEvaluation(req.StudentID)
		readiness := a.promotionSvc.EvaluateReadiness(st, atts, allSessionsMap, latestEval)
		data := struct {
			Attendance *models.Attendance
			Readiness  services.PromotionReadiness
		}{
			Attendance: att,
			Readiness:  readiness,
		}
		w.Header().Set("HX-Trigger", "attendanceUpdated")
		a.RenderPartial(w, "checkin_row.html", data)
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

	sparr := parseInt(r.FormValue("sparring"))
	if sparr == 0 {
		sparr = parseInt(r.FormValue("sparring_iq"))
	}
	poom := parseInt(r.FormValue("poomsae"))
	if poom == 0 {
		poom = parseInt(r.FormValue("technique"))
	}

	eval := &models.StudentEvaluation{
		ID:             uuid.New(),
		StudentID:      studentID,
		CoachID:        coachID,
		EvaluationDate: time.Now(),
		Sparring:       sparr,
		Flexibility:    parseInt(r.FormValue("flexibility")),
		Poomsae:        poom,
		CoachRemarks:   strings.TrimSpace(r.FormValue("coach_remarks")),
		CreatedAt:      time.Now(),
	}
	eval.SyncLegacyFields()

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

	a.LogAction(r, "SAFETY_INCIDENT_REPORT", models.AuditCategorySafety, "Student", studentID.String(), "", "Reported safety incident ("+incidentType+") with mat hold placed")

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

	a.LogAction(r, "SAFETY_INCIDENT_RESOLVE", models.AuditCategorySafety, "SafetyIncident", incidentID.String(), "", "Resolved safety incident and cleared practitioner for mat floor")

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

	a.LogAction(r, "STUDENT_PROMOTE", models.AuditCategoryStudents, "Student", studentID.String(), student.FullName, fmt.Sprintf("Promoted %s to belt rank %s", student.FullName, string(newBelt)))

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

	var coachIDPtr *uuid.UUID
	if cidStr := strings.TrimSpace(r.FormValue("coach_id")); cidStr != "" {
		if cID, err := uuid.Parse(cidStr); err == nil && cID != uuid.Nil {
			coachIDPtr = &cID
		}
	}

	dateStr := r.FormValue("session_date")
	sessionDate := time.Now()
	if parsed, err := time.Parse("2006-01-02", dateStr); err == nil {
		sessionDate = parsed
	}

	var locationIDPtr *uuid.UUID
	if locIDStr := strings.TrimSpace(r.FormValue("location_id")); locIDStr != "" {
		if lID, err := uuid.Parse(locIDStr); err == nil && lID != uuid.Nil {
			locationIDPtr = &lID
		}
	}

	var sessionRate *float64
	if rateStr := strings.TrimSpace(r.FormValue("session_rate")); rateStr != "" {
		if val, err := strconv.ParseFloat(rateStr, 64); err == nil && val >= 0 {
			sessionRate = &val
		}
	} else if locationIDPtr != nil {
		if loc, err := a.store.GetLocationByID(*locationIDPtr); err == nil && loc != nil && loc.FixedRate != nil {
			sessionRate = loc.FixedRate
		}
	}

	sess := &models.TrainingSession{
		ID:           uuid.New(),
		SessionDate:  sessionDate,
		StartTime:    r.FormValue("start_time"),
		EndTime:      r.FormValue("end_time"),
		CoachID:      coachIDPtr,
		LocationID:   locationIDPtr,
		TrainingType: models.TrainingType(r.FormValue("discipline")),
		Notes:        strings.TrimSpace(r.FormValue("notes")),
		SessionRate:  sessionRate,
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
