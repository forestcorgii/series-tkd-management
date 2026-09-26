package main

import (
	"log"
	"net/http"
	"os"

	"github.com/google/uuid"

	"series-tkd-management/internal/handlers"
	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

func main() {
	// 1. Initialize Repository Store
	// Production uses PostgreSQL via DATABASE_URL; local development uses SQLite (series_tkd.db)
	databaseURL := os.Getenv("DATABASE_URL")
	store, driver, err := repository.InitDatabase(databaseURL)
	if err != nil {
		log.Fatalf("Fatal: Failed to initialize %s database: %v", driver, err)
	}
	log.Printf("📦 Database connected using driver: %s", driver)

	// Ensure bootstrap administrator exists (configurable via ADMIN_EMAIL and ADMIN_PASSWORD env vars)
	adminEmail := os.Getenv("ADMIN_EMAIL")
	if adminEmail == "" {
		adminEmail = "admin@seriestkd.com"
	}
	adminPass := os.Getenv("ADMIN_PASSWORD")
	if adminPass == "" {
		adminPass = "admin123"
	}
	if existing, err := store.GetUserByEmail(adminEmail); err != nil || existing == nil {
		adminUser := &models.User{
			ID:          uuid.New(),
			Email:       adminEmail,
			Role:        models.RoleAdmin,
			IsActive:    true,
			DisplayName: "Master Administrator",
		}
		if err := adminUser.SetPassword(adminPass); err == nil {
			_ = store.CreateUser(adminUser)
			log.Printf("🛡️  Master Admin account provisioned: %s", adminEmail)
		}
	}

	// 2. Initialize App Handlers & Templates
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		log.Fatalf("Fatal: Failed to initialize AppHandler: %v", err)
	}

	// 3. Configure HTTP Routes
	mux := http.NewServeMux()

	// Authentication (Web & REST API)
	mux.HandleFunc("GET /login", app.HandleLoginPage)
	mux.HandleFunc("POST /login", app.HandleLoginSubmit)
	mux.HandleFunc("GET /register", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /logout", app.HandleLogout)
	mux.HandleFunc("POST /logout", app.HandleLogout)
	mux.HandleFunc("POST /api/auth/login", app.HandleAPILogin)
	mux.HandleFunc("GET /api/auth/me", app.HandleAPIMe)
	mux.HandleFunc("POST /api/auth/logout", app.HandleAPILogout)
	mux.HandleFunc("POST /api/auth/register", app.RequireRole(models.RoleAdmin)(app.HandleAPIRegister))

	// Role Portals
	mux.HandleFunc("GET /portal/student", app.RequireRole(models.RoleStudent, models.RoleAdmin)(app.HandleStudentPortal))
	mux.HandleFunc("GET /portal/coach", app.RequireRole(models.RoleCoach, models.RoleAdmin)(app.HandleCoachPortal))
	mux.HandleFunc("GET /portal/admin", app.RequireRole(models.RoleAdmin)(app.HandleAdminPortal))

	// Section 4 Role Guarded APIs
	mux.HandleFunc("GET /api/student/readiness", app.RequireRole(models.RoleStudent, models.RoleAdmin)(app.HandleAPIStudentReadiness))
	mux.HandleFunc("GET /api/coach/sessions/live", app.RequireRole(models.RoleCoach, models.RoleAdmin)(app.HandleAPICoachLiveSession))
	mux.HandleFunc("POST /api/coach/check-in", app.RequireRole(models.RoleCoach, models.RoleAdmin)(app.HandleAPICoachCheckIn))
	mux.HandleFunc("POST /api/coach/evaluate", app.RequireRole(models.RoleCoach, models.RoleAdmin)(app.HandleAPICoachEvaluate))
	mux.HandleFunc("POST /api/safety/flag", app.RequireRole(models.RoleCoach, models.RoleAdmin)(app.HandleAPISafetyFlag))
	mux.HandleFunc("POST /api/safety/resolve", app.RequireRole(models.RoleAdmin)(app.HandleAPISafetyResolve))
	mux.HandleFunc("PATCH /api/safety/resolve", app.RequireRole(models.RoleAdmin)(app.HandleAPISafetyResolve))
	mux.HandleFunc("POST /api/admin/schedule", app.RequireRole(models.RoleAdmin)(app.HandleAPIAdminSchedule))
	mux.HandleFunc("PUT /api/admin/schedule", app.RequireRole(models.RoleAdmin)(app.HandleAPIAdminSchedule))
	mux.HandleFunc("POST /api/admin/promote", app.RequireRole(models.RoleAdmin)(app.HandleAPIAdminPromote))

	// Dashboard (Requires Auth - redirects unauthenticated users to /login)
	mux.HandleFunc("GET /", app.RequireAuth(app.HandleDashboard))

	// Students & Ability Radar (Requires Auth)
	mux.HandleFunc("GET /students", app.RequireAuth(app.HandleStudents))
	mux.HandleFunc("GET /students/{id}", app.RequireAuth(app.HandleStudentDetail))
	mux.HandleFunc("POST /students", app.RequireRole(models.RoleAdmin)(app.HandleCreateStudent))
	mux.HandleFunc("POST /students/{id}/evaluations", app.RequireRole(models.RoleCoach, models.RoleAdmin)(app.HandleCreateEvaluation))

	// Coaches & Staff Directory / Payroll (Requires Coach or Admin to view, Admin to create)
	mux.HandleFunc("GET /coaches", app.RequireRole(models.RoleCoach, models.RoleAdmin)(app.HandleCoaches))
	mux.HandleFunc("POST /coaches", app.RequireRole(models.RoleAdmin)(app.HandleCreateCoach))

	// Packages & Billing Passes (Requires Auth, Admin to modify)
	mux.HandleFunc("GET /packages", app.RequireAuth(app.HandlePackages))
	mux.HandleFunc("POST /packages/assign", app.RequireRole(models.RoleAdmin)(app.HandleAssignPackage))
	mux.HandleFunc("POST /packages/templates", app.RequireRole(models.RoleAdmin)(app.HandleCreatePackageTemplate))
	mux.HandleFunc("POST /packages/templates/{id}", app.RequireRole(models.RoleAdmin)(app.HandleUpdatePackageTemplate))
	mux.HandleFunc("POST /packages/templates/{id}/toggle", app.RequireRole(models.RoleAdmin)(app.HandleTogglePackageTemplateStatus))

	// Training Sessions & Live Floor Tablet Check-In (Requires Coach or Admin)
	mux.HandleFunc("GET /sessions", app.RequireAuth(app.HandleSessions))
	mux.HandleFunc("POST /sessions", app.RequireRole(models.RoleCoach, models.RoleAdmin)(app.HandleCreateSession))
	mux.HandleFunc("GET /sessions/{id}/live", app.RequireRole(models.RoleCoach, models.RoleAdmin)(app.HandleLiveSession))
	mux.HandleFunc("POST /sessions/{id}/search-student", app.RequireRole(models.RoleCoach, models.RoleAdmin)(app.HandleSearchStudent))
	mux.HandleFunc("POST /sessions/{id}/checkin/{student_id}", app.RequireRole(models.RoleCoach, models.RoleAdmin)(app.HandleCheckIn))

	// Static assets if needed
	fs := http.FileServer(http.Dir("web/static"))
	mux.Handle("GET /static/", http.StripPrefix("/static/", fs))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("=========================================================")
	log.Printf("🥋 Series Taekwondo Management System (STMS) Server Started")
	log.Printf("📍 Listening on http://localhost:%s", port)
	log.Printf("🛡️  RBAC Engine Online: Student, Coach & Admin Portals Activated")
	log.Printf("=========================================================")

	handler := app.AuthMiddleware(mux)
	if err := http.ListenAndServe(":"+port, handler); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
