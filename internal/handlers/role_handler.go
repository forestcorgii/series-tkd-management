package handlers

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
)

type RolesPageData struct {
	CurrentUser    *models.User
	Roles          []*models.StaffRole
	AllPermissions []models.PermissionInfo
	TotalRoles     int
	CustomRoles    int
	TotalStaff     int
	SuccessNotice  string
	ErrorMessage   string
}

// HandleRoles renders the Roles & Permissions management view or returns JSON for API consumers.
func (a *AppHandler) HandleRoles(w http.ResponseWriter, r *http.Request) {
	currentUser := GetUserFromContext(r.Context())
	isJSON := strings.Contains(r.Header.Get("Accept"), "application/json") ||
		strings.Contains(r.Header.Get("Content-Type"), "application/json") ||
		r.URL.Query().Get("format") == "json" ||
		strings.HasPrefix(r.URL.Path, "/api/")

	roles, err := a.store.GetRoles()
	if err != nil {
		if isJSON {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "failed to retrieve roles"})
			return
		}
		http.Error(w, "Failed to load roles", http.StatusInternalServerError)
		return
	}

	allUsers, _ := a.store.GetAllUsers()
	staffCount := 0
	for _, u := range allUsers {
		if u.IsStaff() {
			staffCount++
		}
	}

	customCount := 0
	for _, role := range roles {
		if !role.IsSystem {
			customCount++
		}
	}

	if isJSON {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"roles":        roles,
			"permissions":  models.GetAllPermissions(),
			"total_roles":  len(roles),
			"custom_roles": customCount,
			"total_staff":  staffCount,
		})
		return
	}

	data := RolesPageData{
		CurrentUser:    currentUser,
		Roles:          roles,
		AllPermissions: models.GetAllPermissions(),
		TotalRoles:     len(roles),
		CustomRoles:    customCount,
		TotalStaff:     staffCount,
		SuccessNotice:  r.URL.Query().Get("success"),
		ErrorMessage:   r.URL.Query().Get("error"),
	}

	a.RenderPage(w, "roles.html", data)
}

// HandleCreateRole provisions a new custom staff role.
func (a *AppHandler) HandleCreateRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	isJSON := strings.Contains(r.Header.Get("Accept"), "application/json") ||
		strings.Contains(r.Header.Get("Content-Type"), "application/json") ||
		strings.HasPrefix(r.URL.Path, "/api/")

	var name, slug, description string
	var permissions []string

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			Name        string   `json:"name"`
			Slug        string   `json:"slug"`
			Description string   `json:"description"`
			Permissions []string `json:"permissions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request payload"})
			return
		}
		name = req.Name
		slug = req.Slug
		description = req.Description
		permissions = req.Permissions
	} else {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/roles?error="+url.QueryEscape("Invalid form submission"), http.StatusSeeOther)
			return
		}
		name = r.FormValue("name")
		slug = r.FormValue("slug")
		description = r.FormValue("description")
		permissions = r.Form["permissions"]
	}

	name = strings.TrimSpace(name)
	slug = strings.TrimSpace(slug)
	if slug == "" && name != "" {
		slug = strings.ToLower(strings.ReplaceAll(name, " ", "-"))
	}

	role := &models.StaffRole{
		Name:        name,
		Slug:        slug,
		Description: description,
		IsSystem:    false,
		Permissions: permissions,
	}

	if err := a.store.CreateRole(role); err != nil {
		if isJSON {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		http.Redirect(w, r, "/roles?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	a.LogAction(r, "ROLE_CREATE", models.AuditCategoryGovernance, "Role", role.ID.String(), role.Name, "Created custom staff role: "+role.Name)

	if isJSON {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(role)
		return
	}

	http.Redirect(w, r, "/roles?success="+url.QueryEscape("Role '"+role.Name+"' created successfully."), http.StatusSeeOther)
}

// HandleUpdateRole updates role details and assigned permissions.
func (a *AppHandler) HandleUpdateRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	roleID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid role ID", http.StatusBadRequest)
		return
	}

	existing, err := a.store.GetRoleByID(roleID)
	if err != nil {
		http.Error(w, "Role not found", http.StatusNotFound)
		return
	}

	isJSON := strings.Contains(r.Header.Get("Accept"), "application/json") ||
		strings.Contains(r.Header.Get("Content-Type"), "application/json") ||
		strings.HasPrefix(r.URL.Path, "/api/")

	var name, slug, description string
	var permissions []string

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			Name        string   `json:"name"`
			Slug        string   `json:"slug"`
			Description string   `json:"description"`
			Permissions []string `json:"permissions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "invalid request payload"})
			return
		}
		name = req.Name
		slug = req.Slug
		description = req.Description
		permissions = req.Permissions
	} else {
		if err := r.ParseForm(); err != nil {
			http.Redirect(w, r, "/roles?error="+url.QueryEscape("Invalid form submission"), http.StatusSeeOther)
			return
		}
		name = r.FormValue("name")
		slug = r.FormValue("slug")
		description = r.FormValue("description")
		permissions = r.Form["permissions"]
	}

	if name != "" {
		existing.Name = strings.TrimSpace(name)
	}
	if slug != "" {
		existing.Slug = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(slug), " ", "-"))
	}
	existing.Description = description
	existing.Permissions = permissions

	if err := a.store.UpdateRole(existing); err != nil {
		if isJSON {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		http.Redirect(w, r, "/roles?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	a.LogAction(r, "ROLE_UPDATE", models.AuditCategoryGovernance, "Role", existing.ID.String(), existing.Name, "Updated role: "+existing.Name+" ("+existing.Slug+")")

	if isJSON {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(existing)
		return
	}

	http.Redirect(w, r, "/roles?success="+url.QueryEscape("Role '"+existing.Name+"' updated successfully."), http.StatusSeeOther)
}

// HandleDeleteRole deletes an unused custom staff role.
func (a *AppHandler) HandleDeleteRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	roleID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid role ID", http.StatusBadRequest)
		return
	}

	role, err := a.store.GetRoleByID(roleID)
	if err != nil {
		http.Error(w, "Role not found", http.StatusNotFound)
		return
	}

	isJSON := strings.Contains(r.Header.Get("Accept"), "application/json") ||
		strings.Contains(r.Header.Get("Content-Type"), "application/json") ||
		strings.HasPrefix(r.URL.Path, "/api/")

	if err := a.store.DeleteRole(roleID); err != nil {
		if isJSON {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		http.Redirect(w, r, "/roles?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	a.LogAction(r, "ROLE_DELETE", models.AuditCategoryGovernance, "Role", roleID.String(), role.Name, "Deleted custom staff role: "+role.Name)

	if isJSON {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "deleted", "message": "role deleted successfully"})
		return
	}

	http.Redirect(w, r, "/roles?success="+url.QueryEscape("Role '"+role.Name+"' deleted successfully."), http.StatusSeeOther)
}

// HandleAssignStaffRole reassigns roles for a staff user.
func (a *AppHandler) HandleAssignStaffRole(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userIDStr := r.PathValue("id")
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	targetUser, err := a.store.GetUserByID(userID)
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	isJSON := strings.Contains(r.Header.Get("Accept"), "application/json") ||
		strings.Contains(r.Header.Get("Content-Type"), "application/json") ||
		strings.HasPrefix(r.URL.Path, "/api/")

	var roleIDs []uuid.UUID
	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			RoleIDs []string `json:"role_ids"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			for _, ridStr := range req.RoleIDs {
				if rid, err := uuid.Parse(ridStr); err == nil {
					roleIDs = append(roleIDs, rid)
				}
			}
		}
	} else {
		_ = r.ParseForm()
		for _, ridStr := range r.Form["role_id"] {
			if rid, err := uuid.Parse(ridStr); err == nil {
				roleIDs = append(roleIDs, rid)
			}
		}
		// Also accept single role_id
		if len(roleIDs) == 0 {
			if rid, err := uuid.Parse(r.FormValue("role_id")); err == nil {
				roleIDs = append(roleIDs, rid)
			}
		}
	}

	if len(roleIDs) == 0 {
		if isJSON {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "at least one role must be selected"})
			return
		}
		http.Redirect(w, r, "/admins?error="+url.QueryEscape("At least one role must be selected."), http.StatusSeeOther)
		return
	}

	if err := a.store.AssignUserRoles(userID, roleIDs); err != nil {
		if isJSON {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
		http.Redirect(w, r, "/admins?error="+url.QueryEscape(err.Error()), http.StatusSeeOther)
		return
	}

	a.LogAction(r, "STAFF_ROLE_ASSIGN", models.AuditCategoryGovernance, "User", targetUser.ID.String(), targetUser.DisplayName, "Reassigned staff roles for: "+targetUser.DisplayName)

	if isJSON {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "message": "roles updated"})
		return
	}

	redirectTo := r.FormValue("redirect_to")
	if redirectTo == "" {
		redirectTo = "/admins"
	}
	http.Redirect(w, r, redirectTo+"?success="+url.QueryEscape("Staff role assignments updated."), http.StatusSeeOther)
}
