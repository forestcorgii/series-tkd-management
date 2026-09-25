package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"

	"series-tkd-management/internal/repository"
	"series-tkd-management/internal/services"
)

type AppHandler struct {
	store        repository.RepositoryStore
	promotionSvc *services.PromotionService
	payrollSvc   *services.PayrollService
	packageSvc   *services.PackageService
	templates    *template.Template
}

func NewAppHandler(store repository.RepositoryStore) (*AppHandler, error) {
	promSvc := services.NewPromotionService()
	paySvc := services.NewPayrollService()
	pkgSvc := services.NewPackageService()

	app := &AppHandler{
		store:        store,
		promotionSvc: promSvc,
		payrollSvc:   paySvc,
		packageSvc:   pkgSvc,
	}

	tmpl, err := app.parseTemplates()
	if err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}
	app.templates = tmpl

	return app, nil
}

func (a *AppHandler) parseTemplates() (*template.Template, error) {
	funcMap := template.FuncMap{
		"formatDate": func(t interface{}) string {
			switch v := t.(type) {
			case string:
				return v
			}
			return fmt.Sprintf("%v", t)
		},
		"sub": func(a, b int) int {
			return a - b
		},
		"add": func(a, b int) int {
			return a + b
		},
		"dict": func(values ...interface{}) (map[string]interface{}, error) {
			if len(values)%2 != 0 {
				return nil, fmt.Errorf("invalid dict call")
			}
			dict := make(map[string]interface{}, len(values)/2)
			for i := 0; i < len(values); i += 2 {
				key, ok := values[i].(string)
				if !ok {
					return nil, fmt.Errorf("dict keys must be strings")
				}
				dict[key] = values[i+1]
			}
			return dict, nil
		},
		"safeHTML": func(s string) template.HTML {
			return template.HTML(s)
		},
	}

	baseDir := "web/templates"
	candidates := []string{"web/templates", "../web/templates", "../../web/templates"}
	for _, c := range candidates {
		if fi, err := os.Stat(c); err == nil && fi.IsDir() {
			baseDir = c
			break
		}
	}

	tmpl := template.New("").Funcs(funcMap)
	// Glob layout, pages, partials
	parsed, err := tmpl.ParseGlob(filepath.Join(baseDir, "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse layout templates in %s: %w", baseDir, err)
	}
	parsed, err = parsed.ParseGlob(filepath.Join(baseDir, "pages", "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse page templates: %w", err)
	}
	parsed, err = parsed.ParseGlob(filepath.Join(baseDir, "partials", "*.html"))
	if err != nil {
		return nil, fmt.Errorf("failed to parse partial templates: %w", err)
	}

	return parsed, nil
}

func (a *AppHandler) RenderPage(w http.ResponseWriter, pageName string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := a.templates.ExecuteTemplate(w, pageName, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
	}
}

func (a *AppHandler) RenderPartial(w http.ResponseWriter, partialName string, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := a.templates.ExecuteTemplate(w, partialName, data)
	if err != nil {
		http.Error(w, fmt.Sprintf("Partial template error: %v", err), http.StatusInternalServerError)
	}
}
