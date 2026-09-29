package handlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
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
	Templates         []*models.PackageTemplate
	ActiveTemplates   []*models.PackageTemplate
	ArchivedTemplates []*models.PackageTemplate
	Students          []*models.Student
	StudentPackages   []StudentPackageViewItem
	CurrentUser       *models.User
}

func (a *AppHandler) HandlePackages(w http.ResponseWriter, r *http.Request) {
	templates, _ := a.store.GetPackageTemplates()
	students, _ := a.store.GetAllStudents()
	user := GetUserFromContext(r.Context())

	var activeTpls, archivedTpls []*models.PackageTemplate
	for _, t := range templates {
		if t.IsActive {
			activeTpls = append(activeTpls, t)
		} else {
			archivedTpls = append(archivedTpls, t)
		}
	}

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
		Templates:         templates,
		ActiveTemplates:   activeTpls,
		ArchivedTemplates: archivedTpls,
		Students:          students,
		StudentPackages:   studentPackages,
		CurrentUser:       user,
	}

	a.RenderPage(w, "packages.html", data)
}

func (a *AppHandler) HandleCreatePackageTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	validityDaysStr := r.FormValue("validity_days")
	priceStr := r.FormValue("price")
	planTypeStr := r.FormValue("plan_type")
	isUnlimited := r.FormValue("is_unlimited") == "1" || r.FormValue("is_unlimited") == "true" || r.FormValue("is_unlimited") == "on" || planTypeStr == "unlimited"
	isFourWeek := planTypeStr == "four_week"

	var planType models.PlanType
	if isUnlimited {
		planType = models.PlanTypeUnlimited
	} else if isFourWeek {
		planType = models.PlanTypeFourWeek
	} else {
		planType = models.PlanTypeStandard
	}

	validityDays, err := strconv.Atoi(validityDaysStr)
	if err != nil || validityDays <= 0 {
		if isFourWeek {
			validityDays = 28
		} else {
			validityDays = 30
		}
	} else if isFourWeek {
		validityDays = 28
	}

	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil || price < 0 {
		price = 0.0
	}

	var sessionCount *int
	var sessionsPerWeek *int
	if !isUnlimited {
		count, err := strconv.Atoi(r.FormValue("session_count"))
		if err == nil && count > 0 {
			sessionCount = &count
		} else {
			if isFourWeek {
				defaultCount := 4
				sessionCount = &defaultCount
			} else {
				defaultCount := 10
				sessionCount = &defaultCount
			}
		}
		if isFourWeek {
			cadence := *sessionCount / 4
			if cadence < 1 {
				cadence = 1
			}
			sessionsPerWeek = &cadence
		}
	}

	tpl := &models.PackageTemplate{
		ID:              uuid.New(),
		Title:           title,
		Description:     description,
		PlanType:        planType,
		SessionCount:    sessionCount,
		SessionsPerWeek: sessionsPerWeek,
		ValidityDays:    validityDays,
		Price:           price,
		IsActive:        true,
	}

	if err := tpl.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
		return
	}

	if err := a.store.CreatePackageTemplate(tpl); err != nil {
		http.Error(w, fmt.Sprintf("Failed to create plan: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/packages", http.StatusSeeOther)
}

func (a *AppHandler) HandleUpdatePackageTemplate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/packages/templates/")
	idStr = strings.TrimSuffix(idStr, "/edit")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	existing, err := a.store.GetPackageTemplateByID(id)
	if err != nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	description := strings.TrimSpace(r.FormValue("description"))
	validityDaysStr := r.FormValue("validity_days")
	priceStr := r.FormValue("price")
	planTypeStr := r.FormValue("plan_type")
	if planTypeStr == "" {
		planTypeStr = r.FormValue("edit_plan_type")
	}
	isUnlimited := r.FormValue("is_unlimited") == "1" || r.FormValue("is_unlimited") == "true" || r.FormValue("is_unlimited") == "on" || planTypeStr == "unlimited"
	isFourWeek := planTypeStr == "four_week"
	isActive := r.FormValue("is_active") == "1" || r.FormValue("is_active") == "true" || r.FormValue("is_active") == "on"

	validityDays, err := strconv.Atoi(validityDaysStr)
	if err == nil && validityDays > 0 {
		existing.ValidityDays = validityDays
	}
	if isFourWeek {
		existing.ValidityDays = 28
	}

	price, err := strconv.ParseFloat(priceStr, 64)
	if err == nil && price >= 0 {
		existing.Price = price
	}

	if title != "" {
		existing.Title = title
	}
	existing.Description = description
	existing.IsActive = isActive

	if isUnlimited {
		existing.PlanType = models.PlanTypeUnlimited
		existing.SessionCount = nil
		existing.SessionsPerWeek = nil
	} else if isFourWeek {
		existing.PlanType = models.PlanTypeFourWeek
		existing.ValidityDays = 28
		count, err := strconv.Atoi(r.FormValue("session_count"))
		if err == nil && count > 0 {
			existing.SessionCount = &count
			cadence := count / 4
			if cadence < 1 {
				cadence = 1
			}
			existing.SessionsPerWeek = &cadence
		}
	} else {
		existing.PlanType = models.PlanTypeStandard
		existing.SessionsPerWeek = nil
		count, err := strconv.Atoi(r.FormValue("session_count"))
		if err == nil && count > 0 {
			existing.SessionCount = &count
		}
	}

	if err := existing.Validate(); err != nil {
		http.Error(w, fmt.Sprintf("Validation error: %v", err), http.StatusBadRequest)
		return
	}

	if err := a.store.UpdatePackageTemplate(existing); err != nil {
		http.Error(w, fmt.Sprintf("Failed to update plan: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/packages", http.StatusSeeOther)
}

func (a *AppHandler) HandleTogglePackageTemplateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := strings.TrimPrefix(r.URL.Path, "/packages/templates/")
	idStr = strings.TrimSuffix(idStr, "/toggle")
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid template ID", http.StatusBadRequest)
		return
	}

	tpl, err := a.store.GetPackageTemplateByID(id)
	if err != nil {
		http.Error(w, "Template not found", http.StatusNotFound)
		return
	}

	newStatus := !tpl.IsActive
	if explicitStatus := r.FormValue("is_active"); explicitStatus != "" {
		newStatus = explicitStatus == "1" || explicitStatus == "true"
	}

	if err := a.store.TogglePackageTemplateStatus(id, newStatus); err != nil {
		http.Error(w, fmt.Sprintf("Failed to toggle status: %v", err), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/packages", http.StatusSeeOther)
}

// AssignPackageFromForm parses membership assignment parameters from form values and records the package in store.
func (a *AppHandler) AssignPackageFromForm(r *http.Request, studentID uuid.UUID) (*models.StudentPackage, error) {
	templateIDStr := strings.TrimSpace(r.FormValue("template_id"))
	if templateIDStr == "" {
		return nil, fmt.Errorf("membership template is required")
	}
	templateID, err := uuid.Parse(templateIDStr)
	if err != nil {
		return nil, fmt.Errorf("invalid template ID: %w", err)
	}

	templates, err := a.store.GetPackageTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve templates: %w", err)
	}
	var selectedTpl *models.PackageTemplate
	for _, t := range templates {
		if t.ID == templateID {
			selectedTpl = t
			break
		}
	}

	if selectedTpl == nil {
		return nil, fmt.Errorf("invalid template")
	}

	now := time.Now()
	validityDays := selectedTpl.ValidityDays
	if overrideDaysStr := strings.TrimSpace(r.FormValue("override_validity_days")); overrideDaysStr != "" {
		if days, err := strconv.Atoi(overrideDaysStr); err == nil && days > 0 {
			validityDays = days
		}
	}

	expiry := now.AddDate(0, 0, validityDays)
	if customExpStr := strings.TrimSpace(r.FormValue("custom_expiry_date")); customExpStr != "" {
		if parsedDate, err := time.Parse("2006-01-02", customExpStr); err == nil {
			expiry = parsedDate
		}
	}

	var totalSess, remSess *int
	isUnlimited := selectedTpl.IsUnlimited()
	overrideType := r.FormValue("override_type")
	overrideSessionsStr := strings.TrimSpace(r.FormValue("override_sessions"))

	if overrideType == "unlimited" {
		isUnlimited = true
	} else if overrideType == "sessions" || overrideSessionsStr != "" {
		isUnlimited = false
	}

	if !isUnlimited {
		if selectedTpl.SessionCount != nil {
			c := *selectedTpl.SessionCount
			totalSess = &c
			remSess = &c
		}
		if overrideSessionsStr != "" {
			if count, err := strconv.Atoi(overrideSessionsStr); err == nil && count > 0 {
				totalSess = &count
				remSess = &count
			}
		}
	}

	paymentStatus := strings.TrimSpace(r.FormValue("payment_status"))
	if paymentStatus == "" {
		paymentStatus = "paid"
	}

	var customPrice *float64
	priceVal := r.FormValue("custom_price")
	if customPriceStr := strings.TrimSpace(priceVal); customPriceStr != "" {
		if cp, err := strconv.ParseFloat(customPriceStr, 64); err == nil && cp >= 0 {
			customPrice = &cp
		}
	}

	notes := strings.TrimSpace(r.FormValue("notes"))
	if notes == "" {
		notes = strings.TrimSpace(r.FormValue("package_notes"))
	}

	var sessionsPerWeek *int
	if selectedTpl.IsFourWeek() {
		if totalSess != nil && *totalSess > 0 {
			cadence := *totalSess / 4
			if cadence < 1 {
				cadence = 1
			}
			sessionsPerWeek = &cadence
		} else if selectedTpl.SessionsPerWeek != nil {
			sessionsPerWeek = selectedTpl.SessionsPerWeek
		}
	}

	sp := &models.StudentPackage{
		ID:                uuid.New(),
		StudentID:         studentID,
		TemplateID:        templateID,
		TemplateTitle:     selectedTpl.Title,
		PlanType:          selectedTpl.PlanType,
		TotalSessions:     totalSess,
		RemainingSessions: remSess,
		SessionsPerWeek:   sessionsPerWeek,
		CustomPrice:       customPrice,
		Notes:             notes,
		PurchaseDate:      now,
		ExpiryDate:        expiry,
		PaymentStatus:     paymentStatus,
	}

	if err := a.store.AssignPackage(sp); err != nil {
		return nil, err
	}

	return sp, nil
}

func (a *AppHandler) HandleAssignPackage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	studentID, _ := uuid.Parse(r.FormValue("student_id"))
	if studentID == uuid.Nil {
		http.Error(w, "Student ID is required", http.StatusBadRequest)
		return
	}

	if _, err := a.AssignPackageFromForm(r, studentID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	redirectURL := r.FormValue("redirect_url")
	if redirectURL == "" {
		redirectURL = "/packages"
	}
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}
