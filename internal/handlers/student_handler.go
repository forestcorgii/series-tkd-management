package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/services"
)

type StudentListItem struct {
	Student          *models.Student
	Readiness        services.PromotionReadiness
	ActivePackage    *models.StudentPackage
	LatestEvaluation *models.StudentEvaluation
	SVGRadarPolygon  string
}

type StudentsPageData struct {
	CurrentUser   *models.User
	Students      []StudentListItem
	Templates     []*models.PackageTemplate
	Search        string
	SuccessNotice string
	ErrorNotice   string
}

func (a *AppHandler) HandleStudents(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	students, err := a.store.SearchStudents(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessions, _ := a.store.GetAllSessions()
	allSessionsMap := make(map[string]*models.TrainingSession, len(sessions))
	for _, s := range sessions {
		allSessionsMap[s.ID.String()] = s
	}

	allAttendances, _ := a.store.GetAllAttendances()
	studentAttendancesMap := make(map[uuid.UUID][]*models.Attendance, len(students))
	for _, att := range allAttendances {
		studentAttendancesMap[att.StudentID] = append(studentAttendancesMap[att.StudentID], att)
	}

	latestEvalsMap, _ := a.store.GetLatestEvaluations()
	packagesMap, _ := a.store.GetAllStudentPackagesGrouped()
	now := time.Now()

	items := make([]StudentListItem, 0, len(students))
	for _, st := range students {
		atts := studentAttendancesMap[st.ID]
		latestEval := latestEvalsMap[st.ID]
		readiness := a.promotionSvc.EvaluateReadiness(st, atts, allSessionsMap, latestEval)

		pkgs := packagesMap[st.ID]
		var activePkg *models.StudentPackage
		for _, p := range pkgs {
			if p.IsValidAt(now) {
				activePkg = p
				break
			}
		}

		polygon := ""
		if latestEval != nil {
			polygon = latestEval.ToSVGPolygon(100, 100, 80)
		}

		items = append(items, StudentListItem{
			Student:          st,
			Readiness:        readiness,
			ActivePackage:    activePkg,
			LatestEvaluation: latestEval,
			SVGRadarPolygon:  polygon,
		})
	}

	templates, _ := a.store.GetPackageTemplates()

	user := GetUserFromContext(r.Context())
	data := StudentsPageData{
		CurrentUser:   user,
		Students:      items,
		Templates:     templates,
		Search:        query,
		SuccessNotice: r.URL.Query().Get("success"),
		ErrorNotice:   r.URL.Query().Get("error"),
	}

	if r.Header.Get("HX-Request") == "true" {
		a.RenderPartial(w, "student_table_rows.html", data)
		return
	}

	a.RenderPage(w, "students.html", data)
}

func (a *AppHandler) HandleStudentDetail(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/students/")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid student ID", http.StatusBadRequest)
		return
	}

	user := GetUserFromContext(r.Context())
	if user != nil {
		if user.Role == models.RoleStudent {
			if user.StudentID == nil || *user.StudentID != id {
				http.Redirect(w, r, "/portal/student", http.StatusSeeOther)
				return
			}
		}
	}

	student, err := a.store.GetStudentByID(id)
	if err != nil {
		http.Error(w, "Student not found", http.StatusNotFound)
		return
	}

	sessions, _ := a.store.GetAllSessions()
	allSessionsMap := make(map[string]*models.TrainingSession)
	for _, s := range sessions {
		allSessionsMap[s.ID.String()] = s
	}

	atts, _ := a.store.GetStudentAttendances(student.ID)
	latestEval, _ := a.store.GetLatestEvaluation(student.ID)
	readiness := a.promotionSvc.EvaluateReadiness(student, atts, allSessionsMap, latestEval)
	pkgs, _ := a.store.GetStudentPackages(student.ID)
	evals, _ := a.store.GetStudentEvaluations(student.ID)
	coaches, _ := a.store.GetAllCoaches()

	polygon := ""
	if latestEval != nil {
		polygon = latestEval.ToSVGPolygon(100, 100, 80)
	}

	data := struct {
		CurrentUser      *models.User
		Student          *models.Student
		Readiness        services.PromotionReadiness
		Packages         []*models.StudentPackage
		LatestEvaluation *models.StudentEvaluation
		Evaluations      []*models.StudentEvaluation
		Coaches          []*models.Coach
		SVGRadarPolygon  string
	}{
		CurrentUser:      user,
		Student:          student,
		Readiness:        readiness,
		Packages:         pkgs,
		LatestEvaluation: latestEval,
		Evaluations:      evals,
		Coaches:          coaches,
		SVGRadarPolygon:  polygon,
	}

	a.RenderPage(w, "student_detail.html", data)
}

func (a *AppHandler) HandleCreateStudent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dobStr := r.FormValue("dob")
	dob, _ := time.Parse("2006-01-02", dobStr)
	if dob.IsZero() {
		dob = time.Now().AddDate(-10, 0, 0)
	}

	s := &models.Student{
		FullName:          r.FormValue("full_name"),
		DOB:               dob,
		Gender:            r.FormValue("gender"),
		Phone:             r.FormValue("phone"),
		CurrentBelt:       models.BeltRank(r.FormValue("current_belt")),
		LastPromotionDate: time.Now(),
		EmergencyName:     r.FormValue("emergency_name"),
		EmergencyPhone:    r.FormValue("emergency_phone"),
		EmergencyRelation: r.FormValue("emergency_relation"),
		MedicalNotes:      r.FormValue("medical_notes"),
		IsActive:          true,
	}

	if err := a.store.CreateStudent(s); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.LogAction(r, "STUDENT_CREATE", models.AuditCategoryStudents, "Student", s.ID.String(), s.FullName, "Registered new student: "+s.FullName)

	email := strings.TrimSpace(r.FormValue("email"))
	password := strings.TrimSpace(r.FormValue("password"))
	if email != "" {
		if password == "" {
			password = "student123"
		}
		_, _ = a.authSvc.RegisterUser(email, password, models.RoleStudent, &s.ID, nil)
	}

	if templateIDStr := strings.TrimSpace(r.FormValue("template_id")); templateIDStr != "" {
		_, _ = a.AssignPackageFromForm(r, s.ID)
	}

	http.Redirect(w, r, "/students", http.StatusSeeOther)
}

func (a *AppHandler) HandleCreateEvaluation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/students/")
	idStr = strings.TrimSuffix(idStr, "/evaluations")
	studentID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid student ID", http.StatusBadRequest)
		return
	}

	coachID, _ := uuid.Parse(r.FormValue("coach_id"))
	flex, _ := strconv.Atoi(r.FormValue("flexibility"))
	stam, _ := strconv.Atoi(r.FormValue("stamina"))
	pow, _ := strconv.Atoi(r.FormValue("power"))
	tech, _ := strconv.Atoi(r.FormValue("technique"))
	sparr, _ := strconv.Atoi(r.FormValue("sparring_iq"))
	disc, _ := strconv.Atoi(r.FormValue("discipline"))

	eval := &models.StudentEvaluation{
		ID:             uuid.New(),
		StudentID:      studentID,
		CoachID:        coachID,
		EvaluationDate: time.Now(),
		Flexibility:    flex,
		Stamina:        stam,
		Power:          pow,
		Technique:      tech,
		SparringIQ:     sparr,
		Discipline:     disc,
		CoachRemarks:   r.FormValue("coach_remarks"),
	}

	if err := a.store.CreateEvaluation(eval); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.LogAction(r, "EVALUATION_SUBMIT", models.AuditCategoryStudents, "Student", studentID.String(), "", "Submitted athletic radar evaluation for student")

	http.Redirect(w, r, "/students/"+studentID.String(), http.StatusSeeOther)
}

func (a *AppHandler) HandleUpdateStudent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		idStr = strings.TrimPrefix(r.URL.Path, "/students/")
		idStr = strings.TrimSuffix(idStr, "/edit")
	}
	studentID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid student ID", http.StatusBadRequest)
		return
	}

	student, err := a.store.GetStudentByID(studentID)
	if err != nil {
		http.Error(w, "Student not found", http.StatusNotFound)
		return
	}

	fullName := strings.TrimSpace(r.FormValue("full_name"))
	if fullName == "" {
		http.Error(w, "Full name is required", http.StatusBadRequest)
		return
	}
	student.FullName = fullName

	dobStr := strings.TrimSpace(r.FormValue("dob"))
	if dobStr != "" {
		if dob, err := time.Parse("2006-01-02", dobStr); err == nil && !dob.IsZero() {
			student.DOB = dob
		}
	}

	if gender := strings.TrimSpace(r.FormValue("gender")); gender != "" {
		student.Gender = gender
	}
	if phone := strings.TrimSpace(r.FormValue("phone")); phone != "" {
		student.Phone = phone
	}

	emName := strings.TrimSpace(r.FormValue("emergency_name"))
	emPhone := strings.TrimSpace(r.FormValue("emergency_phone"))
	if emName != "" {
		student.EmergencyName = emName
	}
	if emPhone != "" {
		student.EmergencyPhone = emPhone
	}
	if emRel := strings.TrimSpace(r.FormValue("emergency_relation")); emRel != "" {
		student.EmergencyRelation = emRel
	}
	student.MedicalNotes = strings.TrimSpace(r.FormValue("medical_notes"))

	// Non-admin roles like Operation Manager could update current_belt if provided,
	// but Admin is strictly barred from modifying belt rank or promotion dates.
	user := GetUserFromContext(r.Context())
	if user != nil && user.IsOperationManager() {
		if belt := strings.TrimSpace(r.FormValue("current_belt")); belt != "" {
			student.CurrentBelt = models.BeltRank(belt)
		}
	}

	if err := a.store.UpdateStudent(student); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	a.LogAction(r, "STUDENT_UPDATE", models.AuditCategoryStudents, "Student", student.ID.String(), student.FullName, "Updated student profile details for: "+student.FullName)

	http.Redirect(w, r, "/students/"+studentID.String(), http.StatusSeeOther)
}

// HandleDeleteStudent permanently removes a student profile, attendance, evaluations, packages, and linked user account
func (a *AppHandler) HandleDeleteStudent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := GetUserFromContext(r.Context())
	if user != nil && user.Role != models.RoleOperationManager {
		if strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("format") == "json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Forbidden: only Operation Managers can delete students."})
			return
		}
		http.Error(w, "Forbidden: only Operation Managers can delete students", http.StatusForbidden)
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		idStr = strings.TrimPrefix(r.URL.Path, "/students/")
		idStr = strings.TrimSuffix(idStr, "/delete")
		idStr = strings.TrimPrefix(idStr, "/api/students/")
	}
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		if strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("format") == "json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid student ID."})
			return
		}
		http.Redirect(w, r, "/students?error="+url.QueryEscape("Invalid student ID."), http.StatusSeeOther)
		return
	}

	student, err := a.store.GetStudentByID(targetID)
	if err != nil || student == nil {
		if strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("format") == "json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Student record not found."})
			return
		}
		http.Redirect(w, r, "/students?error="+url.QueryEscape("Student record not found."), http.StatusSeeOther)
		return
	}

	if err := a.store.DeleteStudent(targetID); err != nil {
		if strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("format") == "json" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Failed to delete student: " + err.Error()})
			return
		}
		http.Redirect(w, r, "/students?error="+url.QueryEscape("Failed to delete student: "+err.Error()), http.StatusSeeOther)
		return
	}

	a.LogAction(r, "STUDENT_DELETE", models.AuditCategoryStudents, "Student", targetID.String(), student.FullName, "Permanently deleted student profile: "+student.FullName)

	msg := fmt.Sprintf("Student '%s' and all associated records have been permanently deleted.", student.FullName)

	if strings.Contains(r.Header.Get("Accept"), "application/json") || r.URL.Query().Get("format") == "json" {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "message": msg})
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Redirect", "/students?success="+url.QueryEscape(msg))
		w.WriteHeader(http.StatusOK)
		return
	}

	http.Redirect(w, r, "/students?success="+url.QueryEscape(msg), http.StatusSeeOther)
}


