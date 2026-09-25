package handlers

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
)

type StudentPackageViewItem struct {
	Package     *models.StudentPackage
	StudentName string
	StudentBelt models.BeltRank
}

type PackagesPageData struct {
	Templates       []*models.PackageTemplate
	Students        []*models.Student
	StudentPackages []StudentPackageViewItem
}

func (a *AppHandler) HandlePackages(w http.ResponseWriter, r *http.Request) {
	templates, _ := a.store.GetPackageTemplates()
	students, _ := a.store.GetAllStudents()

	var studentPackages []StudentPackageViewItem
	for _, st := range students {
		pkgs, _ := a.store.GetStudentPackages(st.ID)
		for _, p := range pkgs {
			studentPackages = append(studentPackages, StudentPackageViewItem{
				Package:     p,
				StudentName: st.FullName,
				StudentBelt: st.CurrentBelt,
			})
		}
	}

	data := PackagesPageData{
		Templates:       templates,
		Students:        students,
		StudentPackages: studentPackages,
	}

	a.RenderPage(w, "packages.html", data)
}

func (a *AppHandler) HandleAssignPackage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	studentID, _ := uuid.Parse(r.FormValue("student_id"))
	templateID, _ := uuid.Parse(r.FormValue("template_id"))

	templates, _ := a.store.GetPackageTemplates()
	var selectedTpl *models.PackageTemplate
	for _, t := range templates {
		if t.ID == templateID {
			selectedTpl = t
			break
		}
	}

	if selectedTpl == nil {
		http.Error(w, "Invalid template", http.StatusBadRequest)
		return
	}

	now := time.Now()
	expiry := now.AddDate(0, 0, selectedTpl.ValidityDays)

	var totalSess, remSess *int
	if selectedTpl.SessionCount != nil {
		c := *selectedTpl.SessionCount
		totalSess = &c
		remSess = &c
	}

	sp := &models.StudentPackage{
		ID:                uuid.New(),
		StudentID:         studentID,
		TemplateID:        templateID,
		TemplateTitle:     selectedTpl.Title,
		TotalSessions:     totalSess,
		RemainingSessions: remSess,
		PurchaseDate:      now,
		ExpiryDate:        expiry,
		PaymentStatus:     "paid",
	}

	if err := a.store.AssignPackage(sp); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/students/"+studentID.String(), http.StatusSeeOther)
}
