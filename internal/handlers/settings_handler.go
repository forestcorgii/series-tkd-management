package handlers

import (
	"fmt"
	"net/http"
	"net/url"
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
	Categories      []CategoryListItem
	TotalCategories int
	TotalSessions   int
	CurrentUser     *models.User
	SuccessNotice   string
	ErrorMessage    string
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

	user := GetUserFromContext(r.Context())
	data := SettingsViewData{
		Categories:      items,
		TotalCategories: len(categories),
		TotalSessions:   len(sessions),
		CurrentUser:     user,
		SuccessNotice:   r.URL.Query().Get("success"),
		ErrorMessage:    r.URL.Query().Get("error"),
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

	http.Redirect(w, r, "/settings?success="+url.QueryEscape("Training category deleted successfully"), http.StatusSeeOther)
}
