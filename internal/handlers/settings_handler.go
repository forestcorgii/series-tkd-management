package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
)

type CategoryListItem struct {
	Category      *models.TrainingCategory
	SessionsCount int
}

type SettingsViewData struct {
	Categories           []CategoryListItem
	TotalCategories      int
	TotalSessions        int
	NotificationSettings *models.NotificationSettings
	ActiveTab            string
	CurrentUser          *models.User
	SuccessNotice        string
	ErrorMessage         string
}

func sanitizeColorHex(c string) string {
	c = strings.TrimSpace(c)
	if c == "" {
		return "#990303"
	}
	if !strings.HasPrefix(c, "#") {
		c = "#" + c
	}
	return c
}

func (a *AppHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	categories, err := a.store.GetAllTrainingCategories()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessions, _ := a.store.GetAllSessions()
	sessionCounts := make(map[string]int)
	for _, s := range sessions {
		sessionCounts[strings.ToLower(string(s.TrainingType))]++
	}

	items := make([]CategoryListItem, 0, len(categories))
	for _, cat := range categories {
		items = append(items, CategoryListItem{
			Category:      cat,
			SessionsCount: sessionCounts[strings.ToLower(cat.Name)],
		})
	}

	notifSettings, _ := a.notifSvc.GetSettings()
	if notifSettings == nil {
		notifSettings = models.DefaultNotificationSettings()
	}

	activeTab := r.URL.Query().Get("tab")
	if activeTab == "" {
		activeTab = "categories"
	}

	user := GetUserFromContext(r.Context())
	data := SettingsViewData{
		Categories:           items,
		TotalCategories:      len(categories),
		TotalSessions:        len(sessions),
		NotificationSettings: notifSettings,
		ActiveTab:            activeTab,
		CurrentUser:          user,
		SuccessNotice:        r.URL.Query().Get("success"),
		ErrorMessage:         r.URL.Query().Get("error"),
	}

	a.RenderPage(w, "settings.html", data)
}

func (a *AppHandler) HandleCreateCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	color := sanitizeColorHex(r.FormValue("color"))

	if name == "" {
		http.Redirect(w, r, "/settings?error="+url.QueryEscape("Category name is required"), http.StatusSeeOther)
		return
	}

	cat := &models.TrainingCategory{
		ID:        uuid.New(),
		Name:      name,
		Color:     color,
		CreatedAt: time.Now(),
	}

	if err := a.store.CreateTrainingCategory(cat); err != nil {
		http.Redirect(w, r, "/settings?error="+url.QueryEscape(fmt.Sprintf("Failed to create category: %v", err)), http.StatusSeeOther)
		return
	}

	a.LogAction(r, "CATEGORY_CREATE", models.AuditCategorySettings, "Category", cat.ID.String(), cat.Name, "Created training category: "+cat.Name)

	http.Redirect(w, r, "/settings?success="+url.QueryEscape(fmt.Sprintf("Category \"%s\" created successfully", cat.Name)), http.StatusSeeOther)
}

func (a *AppHandler) HandleUpdateCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		path := strings.TrimPrefix(r.URL.Path, "/settings/categories/")
		path = strings.TrimSuffix(path, "/edit")
		path = strings.TrimSuffix(path, "/delete")
		idStr = path
	}
	catID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	color := sanitizeColorHex(r.FormValue("color"))

	if name == "" {
		http.Redirect(w, r, "/settings?error="+url.QueryEscape("Category name is required"), http.StatusSeeOther)
		return
	}

	cat := &models.TrainingCategory{
		ID:    catID,
		Name:  name,
		Color: color,
	}

	if err := a.store.UpdateTrainingCategory(cat); err != nil {
		http.Redirect(w, r, "/settings?error="+url.QueryEscape(fmt.Sprintf("Failed to update category: %v", err)), http.StatusSeeOther)
		return
	}

	a.LogAction(r, "CATEGORY_UPDATE", models.AuditCategorySettings, "Category", cat.ID.String(), cat.Name, "Updated training category: "+cat.Name)

	http.Redirect(w, r, "/settings?success="+url.QueryEscape(fmt.Sprintf("Category \"%s\" updated successfully", cat.Name)), http.StatusSeeOther)
}

func (a *AppHandler) HandleDeleteCategory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		path := strings.TrimPrefix(r.URL.Path, "/settings/categories/")
		path = strings.TrimSuffix(path, "/delete")
		idStr = path
	}
	catID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid category ID", http.StatusBadRequest)
		return
	}

	if err := a.store.DeleteTrainingCategory(catID); err != nil {
		http.Redirect(w, r, "/settings?error="+url.QueryEscape(fmt.Sprintf("Cannot delete category: %v", err)), http.StatusSeeOther)
		return
	}

	a.LogAction(r, "CATEGORY_DELETE", models.AuditCategorySettings, "Category", catID.String(), "", "Deleted training category: "+catID.String())

	http.Redirect(w, r, "/settings?success="+url.QueryEscape("Training category deleted successfully"), http.StatusSeeOther)
}

func (a *AppHandler) HandleUpdateNotificationSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	settings, err := a.notifSvc.GetSettings()
	if err != nil || settings == nil {
		settings = models.DefaultNotificationSettings()
	}

	// Helper for checkbox presence: checkboxes only send "on" when checked.
	// If absent in form data, they evaluate to false (allowing email and SMS to be disabled!)
	isChecked := func(fieldName string) bool {
		val := strings.TrimSpace(r.FormValue(fieldName))
		return val == "on" || val == "true" || val == "1"
	}

	// Master Channel Switches (SMS and Email can be disabled!)
	settings.PushEnabled = isChecked("push_enabled")
	settings.SMSEnabled = isChecked("sms_enabled")
	settings.EmailEnabled = isChecked("email_enabled")

	// Staff / Coach & Admin event triggers
	settings.NotifyAdminStudentAdmitted = isChecked("notify_admin_student_admitted")
	settings.NotifyAdminNewClassOpened = isChecked("notify_admin_new_class_opened")
	settings.NotifyAdminStaffRegistered = isChecked("notify_admin_staff_registered")
	settings.NotifyAdminStudentInjured = isChecked("notify_admin_student_injured")
	settings.NotifyAdminClassCancelled = isChecked("notify_admin_class_cancelled")

	// Student / Guardian event triggers
	settings.NotifyStudentPromotionEligible = isChecked("notify_student_promotion_eligible")
	settings.NotifyStudentInjured = isChecked("notify_student_injured")
	settings.NotifyStudentEvaluationLogged = isChecked("notify_student_evaluation_logged")
	settings.NotifyStudentMembershipChanged = isChecked("notify_student_membership_changed")
	settings.NotifyStudentSafetyResolved = isChecked("notify_student_safety_resolved")
	settings.NotifyStudentClassCancelled = isChecked("notify_student_class_cancelled")
	settings.NotifyStudentPassExpiring = isChecked("notify_student_pass_expiring")

	// Provider & Gateway Configurations
	settings.SMTPHost = strings.TrimSpace(r.FormValue("smtp_host"))
	if portStr := strings.TrimSpace(r.FormValue("smtp_port")); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			settings.SMTPPort = p
		}
	}
	settings.SMTPUser = strings.TrimSpace(r.FormValue("smtp_user"))
	if pw := r.FormValue("smtp_password"); pw != "" {
		settings.SMTPPassword = pw
	}
	settings.SMTPFrom = strings.TrimSpace(r.FormValue("smtp_from"))

	settings.SMSProvider = strings.TrimSpace(r.FormValue("sms_provider"))
	if settings.SMSProvider == "" {
		settings.SMSProvider = "simulated"
	}
	if key := r.FormValue("sms_api_key"); key != "" {
		settings.SMSApiKey = strings.TrimSpace(key)
	}
	settings.SMSFromNumber = strings.TrimSpace(r.FormValue("sms_from_number"))

	settings.WebPushPublicKey = strings.TrimSpace(r.FormValue("web_push_public_key"))
	if priv := r.FormValue("web_push_private_key"); priv != "" {
		settings.WebPushPrivateKey = strings.TrimSpace(priv)
	}
	settings.WebPushSubject = strings.TrimSpace(r.FormValue("web_push_subject"))

	if err := a.notifSvc.UpdateSettings(settings); err != nil {
		http.Redirect(w, r, "/settings?tab=notifications&error="+url.QueryEscape("Failed to save notification settings: "+err.Error()), http.StatusSeeOther)
		return
	}

	a.LogAction(r, "NOTIFICATIONS_SETTINGS_UPDATE", models.AuditCategorySettings, "Settings", settings.ID.String(), "Notification Settings", "Updated push, SMS, and email notification channel settings")

	http.Redirect(w, r, "/settings?tab=notifications&success="+url.QueryEscape("Notification settings updated successfully"), http.StatusSeeOther)
}
