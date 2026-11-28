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
	HasSMTPPassword      bool
	HasTelegramBotToken  bool
	WaiverSettings       *models.WaiverSettings
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
	if len(c) != 4 && len(c) != 7 {
		return "#990303"
	}
	for i := 1; i < len(c); i++ {
		ch := c[i]
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return "#990303"
		}
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

	user := GetUserFromContext(r.Context())
	var notifSettings *models.NotificationSettings
	var hasSMTPPassword, hasTelegramBotToken bool
	if user != nil && (user.IsOperationManager() || user.IsAdmin()) {
		loadedNotif, _ := a.notifSvc.GetSettings()
		if loadedNotif == nil {
			loadedNotif = models.DefaultNotificationSettings()
		}
		// Clone without raw secret fields so credentials never leak into HTML DOM
		sanitized := *loadedNotif
		hasSMTPPassword = sanitized.SMTPPassword != ""
		hasTelegramBotToken = sanitized.TelegramBotToken != ""
		sanitized.SMTPPassword = ""
		sanitized.TelegramBotToken = ""
		notifSettings = &sanitized
	} else {
		notifSettings = models.DefaultNotificationSettings()
	}

	waiverSettings, _ := a.store.GetWaiverSettings()
	if waiverSettings == nil {
		waiverSettings = models.DefaultWaiverSettings()
	}

	activeTab := r.URL.Query().Get("tab")
	if activeTab == "" {
		activeTab = "categories"
	}

	data := SettingsViewData{
		Categories:           items,
		TotalCategories:      len(categories),
		TotalSessions:        len(sessions),
		NotificationSettings: notifSettings,
		HasSMTPPassword:      hasSMTPPassword,
		HasTelegramBotToken:  hasTelegramBotToken,
		WaiverSettings:       waiverSettings,
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

	// Master Channel Switches
	settings.EmailEnabled = isChecked("email_enabled")
	settings.TelegramEnabled = isChecked("telegram_enabled")

	// Telegram Bot Configurations & Toggles
	if token := strings.TrimSpace(r.FormValue("telegram_bot_token")); token != "" {
		settings.TelegramBotToken = token
	}
	settings.TelegramPublicChatID = strings.TrimSpace(r.FormValue("telegram_public_chat_id"))
	settings.TelegramStaffChatID = strings.TrimSpace(r.FormValue("telegram_staff_chat_id"))
	settings.NotifyTelegramNewClass = isChecked("notify_telegram_new_class")
	settings.NotifyTelegramNewStudent = isChecked("notify_telegram_new_student")
	settings.NotifyTelegramSlotReserved = isChecked("notify_telegram_slot_reserved")
	settings.NotifyTelegramStaffRegistered = isChecked("notify_telegram_staff_registered")
	settings.NotifyTelegramClassCancel = isChecked("notify_telegram_class_cancel")

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

	// SMTP Email Provider Configurations
	settings.SMTPHost = strings.TrimSpace(r.FormValue("smtp_host"))
	if portStr := strings.TrimSpace(r.FormValue("smtp_port")); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			settings.SMTPPort = p
		}
	}
	settings.SMTPUser = strings.TrimSpace(r.FormValue("smtp_user"))
	if pw := strings.TrimSpace(r.FormValue("smtp_password")); pw != "" {
		if strings.Contains(strings.ToLower(settings.SMTPHost), "gmail.com") {
			pw = strings.ReplaceAll(pw, " ", "")
		}
		settings.SMTPPassword = pw
	}
	settings.SMTPFrom = strings.TrimSpace(r.FormValue("smtp_from"))
	fromName := strings.TrimSpace(r.FormValue("smtp_from_name"))
	if fromName == "" {
		fromName = strings.TrimSpace(r.FormValue("smtp_display_name"))
	}
	settings.SMTPFromName = fromName

	if err := a.notifSvc.UpdateSettings(settings); err != nil {
		http.Redirect(w, r, "/settings?tab=notifications&error="+url.QueryEscape("Failed to save notification settings: "+err.Error()), http.StatusSeeOther)
		return
	}

	a.LogAction(r, "NOTIFICATIONS_SETTINGS_UPDATE", models.AuditCategorySettings, "Settings", settings.ID.String(), "Notification Settings", "Updated Telegram and email notification channel settings")

	http.Redirect(w, r, "/settings?tab=notifications&success="+url.QueryEscape("Notification settings updated successfully"), http.StatusSeeOther)
}

func (a *AppHandler) HandleUpdateWaiverSettings(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	title := strings.TrimSpace(r.FormValue("waiver_title"))
	content := strings.TrimSpace(r.FormValue("waiver_content"))

	if title == "" {
		http.Redirect(w, r, "/settings?tab=waiver&error="+url.QueryEscape("Waiver title is required"), http.StatusSeeOther)
		return
	}
	if content == "" {
		http.Redirect(w, r, "/settings?tab=waiver&error="+url.QueryEscape("Waiver content cannot be empty"), http.StatusSeeOther)
		return
	}

	waiver, err := a.store.GetWaiverSettings()
	if err != nil || waiver == nil {
		waiver = models.DefaultWaiverSettings()
	}

	updatedBy := "Operations Manager"
	if user := GetUserFromContext(r.Context()); user != nil {
		if strings.TrimSpace(user.DisplayName) != "" {
			updatedBy = user.DisplayName
		} else if strings.TrimSpace(user.Email) != "" {
			updatedBy = user.Email
		}
	}

	waiver.Title = title
	waiver.Content = content
	waiver.UpdatedBy = updatedBy

	if err := a.store.UpdateWaiverSettings(waiver); err != nil {
		http.Redirect(w, r, "/settings?tab=waiver&error="+url.QueryEscape("Failed to save waiver settings: "+err.Error()), http.StatusSeeOther)
		return
	}

	a.LogAction(r, "WAIVER_SETTINGS_UPDATE", models.AuditCategorySettings, "Settings", waiver.ID.String(), waiver.Title, "Updated student registration waiver content")

	http.Redirect(w, r, "/settings?tab=waiver&success="+url.QueryEscape("Student registration waiver updated successfully"), http.StatusSeeOther)
}

