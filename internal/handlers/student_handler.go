package handlers

import (
	"net/http"
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
	Students []StudentListItem
	Search   string
}

func (a *AppHandler) HandleStudents(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("query")
	students, err := a.store.SearchStudents(query)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessions, _ := a.store.GetAllSessions()
	allSessionsMap := make(map[string]*models.TrainingSession)
	for _, s := range sessions {
		allSessionsMap[s.ID.String()] = s
	}

	items := make([]StudentListItem, 0, len(students))
	for _, st := range students {
		atts, _ := a.store.GetStudentAttendances(st.ID)
		latestEval, _ := a.store.GetLatestEvaluation(st.ID)
		readiness := a.promotionSvc.EvaluateReadiness(st, atts, allSessionsMap, latestEval)

		pkgs, _ := a.store.GetStudentPackages(st.ID)
		var activePkg *models.StudentPackage
		for _, p := range pkgs {
			if p.IsValidAt(time.Now()) {
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

	data := StudentsPageData{
		Students: items,
		Search:   query,
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
		Student          *models.Student
		Readiness        services.PromotionReadiness
		Packages         []*models.StudentPackage
		LatestEvaluation *models.StudentEvaluation
		Evaluations      []*models.StudentEvaluation
		Coaches          []*models.Coach
		SVGRadarPolygon  string
	}{
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

	http.Redirect(w, r, "/students/"+studentID.String(), http.StatusSeeOther)
}
