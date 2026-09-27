package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
)

type AdminsPageData struct {
	CurrentUser    *models.User
	Admins         []*models.User
	TotalAdmins    int
	ActiveAdmins   int
	InactiveAdmins int
	SuccessNotice  string
	ErrorMessage   string
}

// HandleAdmins renders the administrators directory and telemetry view for Operation Managers
func (a *AppHandler) HandleAdmins(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	admins, err := a.store.GetUsersByRole(models.RoleAdmin)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load administrators: %v", err), http.StatusInternalServerError)
		return
	}

	total := len(admins)
	active := 0
	inactive := 0
	for _, adm := range admins {
		if adm.IsActive {
			active++
		} else {
			inactive++
		}
	}

	data := AdminsPageData{
		CurrentUser:    user,
		Admins:         admins,
		TotalAdmins:    total,
		ActiveAdmins:   active,
		InactiveAdmins: inactive,
		SuccessNotice:  r.URL.Query().Get("success"),
		ErrorMessage:   r.URL.Query().Get("error"),
	}

	a.RenderPage(w, "admins.html", data)
}

// HandleCreateAdmin provisions a new administrator account (RoleAdmin)
func (a *AppHandler) HandleCreateAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fullName := strings.TrimSpace(r.FormValue("full_name"))
	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	password := strings.TrimSpace(r.FormValue("password"))

	if fullName == "" || email == "" {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("Full name and email address are required."), http.StatusSeeOther)
		return
	}

	if password == "" {
		password = "admin123"
	} else if len(password) < 6 {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("Initial password must be at least 6 characters long."), http.StatusSeeOther)
		return
	}

	// Verify email uniqueness
	if existing, _ := a.store.GetUserByEmail(email); existing != nil {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("A user account with this email address already exists."), http.StatusSeeOther)
		return
	}

	newAdmin := &models.User{
		ID:          uuid.New(),
		Email:       email,
		Role:        models.RoleAdmin,
		DisplayName: fullName,
		IsActive:    true,
	}

	if err := newAdmin.SetPassword(password); err != nil {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("Failed to hash password: "+err.Error()), http.StatusSeeOther)
		return
	}

	if err := a.store.CreateUser(newAdmin); err != nil {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("Failed to create administrator record: "+err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/admins?success="+url.QueryEscape(fmt.Sprintf("Administrator account for '%s' provisioned successfully.", fullName)), http.StatusSeeOther)
}

// HandleToggleAdminStatus toggles active/inactive state of an administrator account
func (a *AppHandler) HandleToggleAdminStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("Invalid administrator ID."), http.StatusSeeOther)
		return
	}

	targetUser, err := a.store.GetUserByID(targetID)
	if err != nil || targetUser == nil {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("Administrator record not found."), http.StatusSeeOther)
		return
	}

	if targetUser.Role != models.RoleAdmin {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("Only front-desk administrator accounts can be modified from this view."), http.StatusSeeOther)
		return
	}

	newStatus := !targetUser.IsActive
	if err := a.store.ToggleUserActive(targetID, newStatus); err != nil {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("Failed to update status: "+err.Error()), http.StatusSeeOther)
		return
	}

	statusMsg := "activated"
	if !newStatus {
		statusMsg = "deactivated"
	}

	name := targetUser.DisplayName
	if name == "" {
		name = targetUser.Email
	}

	http.Redirect(w, r, "/admins?success="+url.QueryEscape(fmt.Sprintf("Administrator account for '%s' has been %s.", name, statusMsg)), http.StatusSeeOther)
}

// HandleResetAdminPassword resets the password for a front-desk administrator
func (a *AppHandler) HandleResetAdminPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("Invalid administrator ID."), http.StatusSeeOther)
		return
	}

	newPassword := strings.TrimSpace(r.FormValue("new_password"))
	confirmPassword := strings.TrimSpace(r.FormValue("confirm_password"))

	if len(newPassword) < 6 {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("New password must be at least 6 characters long."), http.StatusSeeOther)
		return
	}

	if confirmPassword != "" && newPassword != confirmPassword {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("Password confirmation does not match."), http.StatusSeeOther)
		return
	}

	targetUser, err := a.store.GetUserByID(targetID)
	if err != nil || targetUser == nil {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("Administrator record not found."), http.StatusSeeOther)
		return
	}

	if targetUser.Role != models.RoleAdmin {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("Only front-desk administrator accounts can be modified from this view."), http.StatusSeeOther)
		return
	}

	if err := targetUser.SetPassword(newPassword); err != nil {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("Failed to hash new password: "+err.Error()), http.StatusSeeOther)
		return
	}

	if err := a.store.UpdateUser(targetUser); err != nil {
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("Failed to update password: "+err.Error()), http.StatusSeeOther)
		return
	}

	name := targetUser.DisplayName
	if name == "" {
		name = targetUser.Email
	}

	http.Redirect(w, r, "/admins?success="+url.QueryEscape(fmt.Sprintf("Password for '%s' reset successfully.", name)), http.StatusSeeOther)
}
