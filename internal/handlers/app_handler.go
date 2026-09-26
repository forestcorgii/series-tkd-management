package handlers

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"reflect"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
	"series-tkd-management/internal/services"
)

type AppHandler struct {
	store            repository.RepositoryStore
	promotionSvc     *services.PromotionService
	payrollSvc       *services.PayrollService
	packageSvc       *services.PackageService
	authSvc          *services.AuthService
	pageTemplates    map[string]*template.Template
	partialTemplates *template.Template
}

func NewAppHandler(store repository.RepositoryStore) (*AppHandler, error) {
	promSvc := services.NewPromotionService()
	paySvc := services.NewPayrollService()
	pkgSvc := services.NewPackageService()
	authSvc := services.NewAuthService(store)

	app := &AppHandler{
		store:        store,
		promotionSvc: promSvc,
		payrollSvc:   paySvc,
		packageSvc:   pkgSvc,
		authSvc:      authSvc,
	}

	if err := app.parseTemplates(); err != nil {
		return nil, fmt.Errorf("failed to parse templates: %w", err)
	}

	return app, nil
}

func (a *AppHandler) parseTemplates() error {
	funcMap := template.FuncMap{
		"currentUser": func(data interface{}) *models.User {
			if data == nil {
				return nil
			}
			val := reflect.ValueOf(data)
			if val.Kind() == reflect.Ptr {
				if val.IsNil() {
					return nil
				}
				val = val.Elem()
			}
			if val.Kind() == reflect.Struct {
				f := val.FieldByName("CurrentUser")
				if f.IsValid() && !f.IsNil() && f.Type() == reflect.TypeOf((*models.User)(nil)) {
					return f.Interface().(*models.User)
				}
			} else if val.Kind() == reflect.Map {
				m := val.MapIndex(reflect.ValueOf("CurrentUser"))
				if m.IsValid() {
					if u, ok := m.Interface().(*models.User); ok {
						return u
					}
				}
			}
			return nil
		},
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
		"mul": func(a float64, b float64) float64 {
			return a * b
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

	// 1. Base template containing layout.html and partials
	baseTmpl := template.New("").Funcs(funcMap)
	var err error
	baseTmpl, err = baseTmpl.ParseGlob(filepath.Join(baseDir, "*.html"))
	if err != nil {
		return fmt.Errorf("failed to parse layout templates in %s: %w", baseDir, err)
	}

	// Parse partials into base template so partials are available to all pages
	partialsPattern := filepath.Join(baseDir, "partials", "*.html")
	partialFiles, err := filepath.Glob(partialsPattern)
	if err != nil {
		return fmt.Errorf("failed to glob partials: %w", err)
	}
	if len(partialFiles) > 0 {
		baseTmpl, err = baseTmpl.ParseGlob(partialsPattern)
		if err != nil {
			return fmt.Errorf("failed to parse partial templates: %w", err)
		}
	}
	a.partialTemplates = baseTmpl

	// 2. Parse each page file with its own isolated clone of baseTmpl
	pagesPattern := filepath.Join(baseDir, "pages", "*.html")
	pageFiles, err := filepath.Glob(pagesPattern)
	if err != nil {
		return fmt.Errorf("failed to glob pages: %w", err)
	}

	a.pageTemplates = make(map[string]*template.Template, len(pageFiles))
	for _, pageFile := range pageFiles {
		clone, err := baseTmpl.Clone()
		if err != nil {
			return fmt.Errorf("failed to clone base template for %s: %w", pageFile, err)
		}
		pageTmpl, err := clone.ParseFiles(pageFile)
		if err != nil {
			return fmt.Errorf("failed to parse page template %s: %w", pageFile, err)
		}
		a.pageTemplates[filepath.Base(pageFile)] = pageTmpl
	}

	return nil
}

func (a *AppHandler) RenderPage(w http.ResponseWriter, pageName string, data interface{}) {
	tmpl, ok := a.pageTemplates[pageName]
	if !ok {
		http.Error(w, fmt.Sprintf("Template %s not found", pageName), http.StatusInternalServerError)
		return
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, pageName, data); err != nil {
		http.Error(w, fmt.Sprintf("Template error: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}

func (a *AppHandler) RenderPartial(w http.ResponseWriter, partialName string, data interface{}) {
	var buf bytes.Buffer
	if err := a.partialTemplates.ExecuteTemplate(&buf, partialName, data); err != nil {
		http.Error(w, fmt.Sprintf("Partial template error: %v", err), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	buf.WriteTo(w)
}
