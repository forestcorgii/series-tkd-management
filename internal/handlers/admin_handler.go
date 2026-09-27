package handlers

import (
	"encoding/json"
	"fmt"
	"html"
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

func isJSONRequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	contentType := r.Header.Get("Content-Type")
	return strings.Contains(accept, "application/json") ||
		strings.Contains(contentType, "application/json") ||
		r.URL.Query().Get("format") == "json" ||
		strings.HasPrefix(r.URL.Path, "/api/")
}

func isHTMXRequest(r *http.Request) bool {
	return r.Header.Get("HX-Request") == "true"
}

func renderHTMXBanner(w http.ResponseWriter, statusCode int, isSuccess bool, message string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)
	if isSuccess {
		fmt.Fprintf(w, `<div class="p-3 rounded-xl bg-emerald-500/10 border border-emerald-500/30 text-emerald-700 dark:text-emerald-300 text-xs font-semibold flex items-center justify-between shadow-sm animate-fade-in"><div class="flex items-center gap-2"><span>✅</span><span>%s</span></div><button onclick="this.parentElement.remove()" class="text-emerald-700 dark:text-emerald-400 hover:opacity-75 text-sm">&times;</button></div>`, html.EscapeString(message))
	} else {
		fmt.Fprintf(w, `<div class="p-3 rounded-xl bg-rose-500/10 border border-rose-500/30 text-rose-700 dark:text-rose-300 text-xs font-semibold flex items-center justify-between shadow-sm animate-fade-in"><div class="flex items-center gap-2"><span>⚠️</span><span>%s</span></div><button onclick="this.parentElement.remove()" class="text-rose-700 dark:text-rose-400 hover:opacity-75 text-sm">&times;</button></div>`, html.EscapeString(message))
	}
}

func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(data)
}

// HandleAdmins renders the administrators directory and telemetry view for Operation Managers, with JSON support
func (a *AppHandler) HandleAdmins(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		if isJSONRequest(r) {
			writeJSONResponse(w, http.StatusUnauthorized, map[string]interface{}{
				"error":   "unauthorized",
				"message": "authentication required",
			})
			return
		}
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	admins, err := a.store.GetUsersByRole(models.RoleAdmin)
	if err != nil {
		if isJSONRequest(r) {
			writeJSONResponse(w, http.StatusInternalServerError, map[string]interface{}{
				"error": fmt.Sprintf("Failed to load administrators: %v", err),
			})
			return
		}
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

	if isJSONRequest(r) {
		writeJSONResponse(w, http.StatusOK, map[string]interface{}{
			"status":   "success",
			"total":    total,
			"active":   active,
			"inactive": inactive,
			"admins":   admins,
		})
		return
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

// HandleCreateAdmin provisions a new administrator account (RoleAdmin), supporting Form, HTMX, and JSON payloads
func (a *AppHandler) HandleCreateAdmin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		if isJSONRequest(r) {
			writeJSONResponse(w, http.StatusMethodNotAllowed, map[string]interface{}{
				"error": "Method not allowed",
			})
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	respondError := func(statusCode int, msg string) {
		if isJSONRequest(r) {
			writeJSONResponse(w, statusCode, map[string]interface{}{
				"status": "error",
				"error":  msg,
			})
			return
		}
		if isHTMXRequest(r) {
			renderHTMXBanner(w, statusCode, false, msg)
			return
		}
		http.Redirect(w, r, "/admins?error="+url.QueryEscape(msg), http.StatusSeeOther)
	}

	var fullName, email, password string
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			FullName string `json:"full_name"`
			Email    string `json:"email"`
			Password string `json:"password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(http.StatusBadRequest, "Invalid JSON payload: "+err.Error())
			return
		}
		fullName = strings.TrimSpace(req.FullName)
		email = strings.ToLower(strings.TrimSpace(req.Email))
		password = strings.TrimSpace(req.Password)
	} else {
		fullName = strings.TrimSpace(r.FormValue("full_name"))
		email = strings.ToLower(strings.TrimSpace(r.FormValue("email")))
		password = strings.TrimSpace(r.FormValue("password"))
	}

	if fullName == "" || email == "" {
		respondError(http.StatusBadRequest, "Full name and email address are required.")
		return
	}

	if password == "" {
		password = "admin123"
	} else if len(password) < 6 {
		respondError(http.StatusBadRequest, "Initial password must be at least 6 characters long.")
		return
	}

	// Verify email uniqueness
	if existing, _ := a.store.GetUserByEmail(email); existing != nil {
		respondError(http.StatusConflict, "A user account with this email address already exists.")
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
		respondError(http.StatusInternalServerError, "Failed to hash password: "+err.Error())
		return
	}

	if err := a.store.CreateUser(newAdmin); err != nil {
		respondError(http.StatusInternalServerError, "Failed to create administrator record: "+err.Error())
		return
	}

	successMsg := fmt.Sprintf("Administrator account for '%s' provisioned successfully.", fullName)
	if isJSONRequest(r) {
		writeJSONResponse(w, http.StatusCreated, map[string]interface{}{
			"status":  "created",
			"admin":   newAdmin,
			"message": successMsg,
		})
		return
	}
	if isHTMXRequest(r) {
		renderHTMXBanner(w, http.StatusOK, true, successMsg)
		return
	}

	http.Redirect(w, r, "/admins?success="+url.QueryEscape(successMsg), http.StatusSeeOther)
}

// HandleToggleAdminStatus toggles active/inactive state of an administrator account with Form, HTMX, and JSON support
func (a *AppHandler) HandleToggleAdminStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPatch {
		if isJSONRequest(r) {
			writeJSONResponse(w, http.StatusMethodNotAllowed, map[string]interface{}{
				"error": "Method not allowed",
			})
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	respondError := func(statusCode int, msg string) {
		if isJSONRequest(r) {
			writeJSONResponse(w, statusCode, map[string]interface{}{
				"status": "error",
				"error":  msg,
			})
			return
		}
		if isHTMXRequest(r) {
			renderHTMXBanner(w, statusCode, false, msg)
			return
		}
		http.Redirect(w, r, "/admins?error="+url.QueryEscape(msg), http.StatusSeeOther)
	}

	idStr := r.PathValue("id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		respondError(http.StatusBadRequest, "Invalid administrator ID.")
		return
	}

	targetUser, err := a.store.GetUserByID(targetID)
	if err != nil || targetUser == nil {
		respondError(http.StatusNotFound, "Administrator record not found.")
		return
	}

	if targetUser.Role != models.RoleAdmin {
		respondError(http.StatusBadRequest, "Only front-desk administrator accounts can be modified from this view.")
		return
	}

	newStatus := !targetUser.IsActive
	if err := a.store.ToggleUserActive(targetID, newStatus); err != nil {
		respondError(http.StatusInternalServerError, "Failed to update status: "+err.Error())
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

	msg := fmt.Sprintf("Administrator account for '%s' has been %s.", name, statusMsg)
	if isJSONRequest(r) {
		writeJSONResponse(w, http.StatusOK, map[string]interface{}{
			"status":    "success",
			"id":        targetID,
			"is_active": newStatus,
			"message":   msg,
		})
		return
	}
	if isHTMXRequest(r) {
		renderHTMXBanner(w, http.StatusOK, true, msg)
		return
	}

	http.Redirect(w, r, "/admins?success="+url.QueryEscape(msg), http.StatusSeeOther)
}

// HandleResetAdminPassword resets password for front-desk administrator with Form, HTMX, and JSON support
func (a *AppHandler) HandleResetAdminPassword(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodPatch {
		if isJSONRequest(r) {
			writeJSONResponse(w, http.StatusMethodNotAllowed, map[string]interface{}{
				"error": "Method not allowed",
			})
			return
		}
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	respondError := func(statusCode int, msg string) {
		if isJSONRequest(r) {
			writeJSONResponse(w, statusCode, map[string]interface{}{
				"status": "error",
				"error":  msg,
			})
			return
		}
		if isHTMXRequest(r) {
			renderHTMXBanner(w, statusCode, false, msg)
			return
		}
		http.Redirect(w, r, "/admins?error="+url.QueryEscape(msg), http.StatusSeeOther)
	}

	idStr := r.PathValue("id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		respondError(http.StatusBadRequest, "Invalid administrator ID.")
		return
	}

	var newPassword, confirmPassword string
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			NewPassword     string `json:"new_password"`
			ConfirmPassword string `json:"confirm_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			respondError(http.StatusBadRequest, "Invalid JSON payload: "+err.Error())
			return
		}
		newPassword = strings.TrimSpace(req.NewPassword)
		confirmPassword = strings.TrimSpace(req.ConfirmPassword)
	} else {
		newPassword = strings.TrimSpace(r.FormValue("new_password"))
		confirmPassword = strings.TrimSpace(r.FormValue("confirm_password"))
	}

	if len(newPassword) < 6 {
		respondError(http.StatusBadRequest, "New password must be at least 6 characters long.")
		return
	}

	if confirmPassword != "" && newPassword != confirmPassword {
		respondError(http.StatusBadRequest, "Password confirmation does not match.")
		return
	}

	targetUser, err := a.store.GetUserByID(targetID)
	if err != nil || targetUser == nil {
		respondError(http.StatusNotFound, "Administrator record not found.")
		return
	}

	if targetUser.Role != models.RoleAdmin {
		respondError(http.StatusBadRequest, "Only front-desk administrator accounts can be modified from this view.")
		return
	}

	if err := targetUser.SetPassword(newPassword); err != nil {
		respondError(http.StatusInternalServerError, "Failed to hash new password: "+err.Error())
		return
	}

	if err := a.store.UpdateUser(targetUser); err != nil {
		respondError(http.StatusInternalServerError, "Failed to update password: "+err.Error())
		return
	}

	name := targetUser.DisplayName
	if name == "" {
		name = targetUser.Email
	}

	msg := fmt.Sprintf("Password for '%s' reset successfully.", name)
	if isJSONRequest(r) {
		writeJSONResponse(w, http.StatusOK, map[string]interface{}{
			"status":  "success",
			"id":      targetID,
			"message": msg,
		})
		return
	}
	if isHTMXRequest(r) {
		renderHTMXBanner(w, http.StatusOK, true, msg)
		return
	}

	http.Redirect(w, r, "/admins?success="+url.QueryEscape(msg), http.StatusSeeOther)
}
