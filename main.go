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

	// Ensure bootstrap Operation Manager exists
	managerEmail := os.Getenv("MANAGER_EMAIL")
	if managerEmail == "" {
		managerEmail = "manager@seriestkd.com"
	}
	managerPass := os.Getenv("MANAGER_PASSWORD")
	if managerPass == "" {
		managerPass = "manager123"
	}
	if existing, err := store.GetUserByEmail(managerEmail); err != nil || existing == nil {
		managerUser := &models.User{
			ID:          uuid.New(),
			Email:       managerEmail,
			Role:        models.RoleOperationManager,
			IsActive:    true,
			DisplayName: "Operation Manager",
		}
		if err := managerUser.SetPassword(managerPass); err == nil {
			_ = store.CreateUser(managerUser)
			log.Printf("👑 Operation Manager account provisioned: %s", managerEmail)
		}
	}

	// Ensure bootstrap administrator exists
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
			DisplayName: "Dojang Administrator",
		}
		if err := adminUser.SetPassword(adminPass); err == nil {
			_ = store.CreateUser(adminUser)
			log.Printf("🛡️  Admin account provisioned: %s", adminEmail)
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
	mux.HandleFunc("POST /api/auth/register", app.RequireRole(models.RoleOperationManager)(app.HandleAPIRegister))

	// Role Portals
	mux.HandleFunc("GET /portal/student", app.RequireRole(models.RoleStudent, models.RoleCoach, models.RoleOperationManager)(app.HandleStudentPortal))
	mux.HandleFunc("GET /portal/coach", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleCoachPortal))
	mux.HandleFunc("GET /portal/admin", app.RequireRole(models.RoleOperationManager)(app.HandleAdminPortal))

	// Section 4 Role Guarded APIs
	mux.HandleFunc("GET /api/student/readiness", app.RequireRole(models.RoleStudent, models.RoleCoach, models.RoleOperationManager)(app.HandleAPIStudentReadiness))
	mux.HandleFunc("GET /api/coach/sessions/live", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleAPICoachLiveSession))
	mux.HandleFunc("POST /api/coach/check-in", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleAPICoachCheckIn))
	mux.HandleFunc("POST /api/coach/evaluate", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleAPICoachEvaluate))
	mux.HandleFunc("POST /api/safety/flag", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleAPISafetyFlag))
	mux.HandleFunc("POST /api/safety/resolve", app.RequireRole(models.RoleOperationManager)(app.HandleAPISafetyResolve))
	mux.HandleFunc("PATCH /api/safety/resolve", app.RequireRole(models.RoleOperationManager)(app.HandleAPISafetyResolve))
	mux.HandleFunc("POST /api/admin/schedule", app.RequireRole(models.RoleOperationManager)(app.HandleAPIAdminSchedule))
	mux.HandleFunc("PUT /api/admin/schedule", app.RequireRole(models.RoleOperationManager)(app.HandleAPIAdminSchedule))
	mux.HandleFunc("POST /api/admin/promote", app.RequireRole(models.RoleOperationManager)(app.HandleAPIAdminPromote))

	// Dashboard (Only Operation Manager has full control over executive dashboard)
	mux.HandleFunc("GET /", app.RequireRole(models.RoleOperationManager)(app.HandleDashboard))

	// Students & Ability Radar (Coach & Operation Manager; Students can view their own profile via /students/{id})
	mux.HandleFunc("GET /students", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleStudents))
	mux.HandleFunc("GET /students/{id}", app.RequireRole(models.RoleStudent, models.RoleCoach, models.RoleOperationManager)(app.HandleStudentDetail))
	mux.HandleFunc("POST /students", app.RequireRole(models.RoleOperationManager)(app.HandleCreateStudent))
	mux.HandleFunc("POST /students/{id}/evaluations", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleCreateEvaluation))

	// Coaches & Staff Directory / Payroll (Only Operation Manager)
	mux.HandleFunc("GET /coaches", app.RequireRole(models.RoleOperationManager)(app.HandleCoaches))
	mux.HandleFunc("POST /coaches", app.RequireRole(models.RoleOperationManager)(app.HandleCreateCoach))

	// Packages & Billing Passes (Admin can view and assign; Operation Manager has full control)
	mux.HandleFunc("GET /packages", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandlePackages))
	mux.HandleFunc("POST /packages/assign", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleAssignPackage))
	mux.HandleFunc("POST /packages/templates", app.RequireRole(models.RoleOperationManager)(app.HandleCreatePackageTemplate))
	mux.HandleFunc("POST /packages/templates/{id}", app.RequireRole(models.RoleOperationManager)(app.HandleUpdatePackageTemplate))
	mux.HandleFunc("POST /packages/templates/{id}/toggle", app.RequireRole(models.RoleOperationManager)(app.HandleTogglePackageTemplateStatus))

	// Training Sessions & Live Floor Tablet Check-In (Coach and Operation Manager)
	mux.HandleFunc("GET /sessions", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleSessions))
	mux.HandleFunc("POST /sessions", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleCreateSession))
	mux.HandleFunc("GET /sessions/{id}/live", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleLiveSession))
	mux.HandleFunc("POST /sessions/{id}/search-student", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleSearchStudent))
	mux.HandleFunc("POST /sessions/{id}/checkin/{student_id}", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleCheckIn))

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
