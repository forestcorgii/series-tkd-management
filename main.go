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
	mux.HandleFunc("GET /forgot-password", app.HandleForgotPasswordPage)
	mux.HandleFunc("POST /forgot-password", app.HandleForgotPasswordSubmit)
	mux.HandleFunc("GET /reset-password", app.HandleResetPasswordPage)
	mux.HandleFunc("POST /reset-password", app.HandleResetPasswordSubmit)
	mux.HandleFunc("GET /register", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /logout", app.HandleLogout)
	mux.HandleFunc("POST /logout", app.HandleLogout)
	mux.HandleFunc("POST /api/auth/login", app.HandleAPILogin)
	mux.HandleFunc("GET /api/auth/me", app.HandleAPIMe)
	mux.HandleFunc("POST /api/auth/logout", app.HandleAPILogout)
	mux.HandleFunc("POST /api/auth/register", app.RequireRole(models.RoleOperationManager)(app.HandleAPIRegister))
	mux.HandleFunc("POST /api/auth/forgot-password", app.HandleAPIForgotPassword)
	mux.HandleFunc("POST /api/auth/reset-password", app.HandleAPIResetPassword)

	// Role Portals
	mux.HandleFunc("GET /portal/student", app.RequireRole(models.RoleStudent, models.RoleCoach, models.RoleOperationManager)(app.HandleStudentPortal))
	mux.HandleFunc("GET /portal/coach", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleCoachPortal))
	mux.HandleFunc("GET /portal/admin", app.RequireRole(models.RoleOperationManager)(app.HandleAdminPortal))

	// User Profile (All Authenticated Roles)
	mux.HandleFunc("GET /profile", app.RequireAuth(app.HandleProfile))
	mux.HandleFunc("POST /profile", app.RequireAuth(app.HandleProfile))

	// Section 4 Role Guarded APIs
	mux.HandleFunc("GET /api/student/readiness", app.RequireRole(models.RoleStudent, models.RoleCoach, models.RoleOperationManager)(app.HandleAPIStudentReadiness))
	mux.HandleFunc("POST /api/student/check-in", app.RequireRole(models.RoleStudent, models.RoleCoach, models.RoleOperationManager)(app.HandleAPIStudentCheckIn))
	mux.HandleFunc("GET /api/coach/sessions/live", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleAPICoachLiveSession))
	mux.HandleFunc("POST /api/coach/check-in", app.RequireRole(models.RoleStudent, models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleAPICoachCheckIn))
	mux.HandleFunc("POST /api/coach/evaluate", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleAPICoachEvaluate))
	mux.HandleFunc("POST /api/safety/flag", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleAPISafetyFlag))
	mux.HandleFunc("POST /api/safety/resolve", app.RequireRole(models.RoleOperationManager)(app.HandleAPISafetyResolve))
	mux.HandleFunc("PATCH /api/safety/resolve", app.RequireRole(models.RoleOperationManager)(app.HandleAPISafetyResolve))
	mux.HandleFunc("POST /api/admin/schedule", app.RequireRole(models.RoleOperationManager)(app.HandleAPIAdminSchedule))
	mux.HandleFunc("PUT /api/admin/schedule", app.RequireRole(models.RoleOperationManager)(app.HandleAPIAdminSchedule))
	mux.HandleFunc("POST /api/admin/promote", app.RequireRole(models.RoleOperationManager)(app.HandleAPIAdminPromote))

	// Dashboard (Only Operation Manager has full control over executive dashboard)
	mux.HandleFunc("GET /", app.RequireRole(models.RoleOperationManager)(app.HandleDashboard))

	// Students & Ability Radar (Coach, Admin & Operation Manager; Students can view their own profile via /students/{id})
	mux.HandleFunc("GET /students", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleStudents))
	mux.HandleFunc("GET /students/{id}", app.RequireRole(models.RoleStudent, models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleStudentDetail))
	mux.HandleFunc("POST /students", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleCreateStudent))
	mux.HandleFunc("POST /students/{id}", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleUpdateStudent))
	mux.HandleFunc("POST /students/{id}/edit", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleUpdateStudent))
	mux.HandleFunc("POST /students/{id}/delete", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleDeleteStudent))
	mux.HandleFunc("DELETE /students/{id}", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleDeleteStudent))
	mux.HandleFunc("DELETE /api/students/{id}", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleDeleteStudent))
	mux.HandleFunc("POST /students/{id}/evaluations", app.RequireRole(models.RoleCoach, models.RoleOperationManager)(app.HandleCreateEvaluation))

	// Coaches & Staff Directory / Payroll (Only Operation Manager)
	mux.HandleFunc("GET /coaches", app.RequireRole(models.RoleOperationManager)(app.HandleCoaches))
	mux.HandleFunc("POST /coaches", app.RequireRole(models.RoleOperationManager)(app.HandleCreateCoach))
	mux.HandleFunc("POST /coaches/{id}/toggle", app.RequireRole(models.RoleOperationManager)(app.HandleToggleCoachStatus))
	mux.HandleFunc("POST /coaches/{id}/delete", app.RequireRole(models.RoleOperationManager)(app.HandleDeleteCoach))
	mux.HandleFunc("DELETE /coaches/{id}", app.RequireRole(models.RoleOperationManager)(app.HandleDeleteCoach))


	// Administrators Management (Only Operation Manager)
	mux.HandleFunc("GET /admin", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admins", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /admin/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/admins", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /admins", app.RequireRole(models.RoleOperationManager)(app.HandleAdmins))
	mux.HandleFunc("GET /admins/", app.RequireRole(models.RoleOperationManager)(app.HandleAdmins))
	mux.HandleFunc("GET /api/admins", app.RequireRole(models.RoleOperationManager)(app.HandleAdmins))
	mux.HandleFunc("POST /admins", app.RequireRole(models.RoleOperationManager)(app.HandleCreateAdmin))
	mux.HandleFunc("POST /api/admins", app.RequireRole(models.RoleOperationManager)(app.HandleCreateAdmin))
	mux.HandleFunc("POST /admins/{id}/toggle", app.RequireRole(models.RoleOperationManager)(app.HandleToggleAdminStatus))
	mux.HandleFunc("PATCH /admins/{id}/toggle", app.RequireRole(models.RoleOperationManager)(app.HandleToggleAdminStatus))
	mux.HandleFunc("POST /api/admins/{id}/toggle", app.RequireRole(models.RoleOperationManager)(app.HandleToggleAdminStatus))
	mux.HandleFunc("PATCH /api/admins/{id}/toggle", app.RequireRole(models.RoleOperationManager)(app.HandleToggleAdminStatus))
	mux.HandleFunc("POST /admins/{id}/reset-password", app.RequireRole(models.RoleOperationManager)(app.HandleResetAdminPassword))
	mux.HandleFunc("PUT /admins/{id}/reset-password", app.RequireRole(models.RoleOperationManager)(app.HandleResetAdminPassword))
	mux.HandleFunc("POST /api/admins/{id}/reset-password", app.RequireRole(models.RoleOperationManager)(app.HandleResetAdminPassword))
	mux.HandleFunc("PUT /api/admins/{id}/reset-password", app.RequireRole(models.RoleOperationManager)(app.HandleResetAdminPassword))

	// Packages & Billing Passes (Admin can view and assign; Operation Manager has full control)
	mux.HandleFunc("GET /packages", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandlePackages))
	mux.HandleFunc("POST /packages/assign", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleAssignPackage))
	mux.HandleFunc("POST /packages/templates", app.RequireRole(models.RoleOperationManager)(app.HandleCreatePackageTemplate))
	mux.HandleFunc("POST /packages/templates/{id}", app.RequireRole(models.RoleOperationManager)(app.HandleUpdatePackageTemplate))
	mux.HandleFunc("POST /packages/templates/{id}/toggle", app.RequireRole(models.RoleOperationManager)(app.HandleTogglePackageTemplateStatus))

	// Locations Management (Admin and Operation Manager)
	mux.HandleFunc("GET /location", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/locations", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /location/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/locations", http.StatusSeeOther)
	})
	mux.HandleFunc("GET /locations", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleLocations))
	mux.HandleFunc("GET /locations/", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleLocations))
	mux.HandleFunc("POST /locations", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleCreateLocation))
	mux.HandleFunc("POST /locations/{id}", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleUpdateLocation))
	mux.HandleFunc("POST /locations/{id}/edit", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleUpdateLocation))
	mux.HandleFunc("PUT /locations/{id}", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleUpdateLocation))
	mux.HandleFunc("POST /locations/{id}/delete", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleDeleteLocation))
	mux.HandleFunc("DELETE /locations/{id}", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleDeleteLocation))
	mux.HandleFunc("DELETE /api/locations/{id}", app.RequireRole(models.RoleAdmin, models.RoleOperationManager)(app.HandleDeleteLocation))

	// Training Sessions & Live Floor Tablet Check-In (Coach, Admin, and Operation Manager)
	mux.HandleFunc("GET /sessions", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleSessions))
	mux.HandleFunc("POST /sessions", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleCreateSession))
	mux.HandleFunc("POST /sessions/{id}/edit", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleUpdateSession))
	mux.HandleFunc("PUT /sessions/{id}", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleUpdateSession))
	mux.HandleFunc("POST /sessions/{id}/delete", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleDeleteSession))
	mux.HandleFunc("DELETE /sessions/{id}", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleDeleteSession))
	mux.HandleFunc("POST /sessions/{id}/cancel", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleCancelSession))
	mux.HandleFunc("GET /sessions/{id}/live", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleLiveSession))
	mux.HandleFunc("POST /sessions/{id}/search-student", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleSearchStudent))
	mux.HandleFunc("POST /sessions/{id}/checkin/{student_id}", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleCheckIn))
	mux.HandleFunc("POST /sessions/{id}/attendance/{student_id}/remove", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleRemoveAttendance))
	mux.HandleFunc("DELETE /sessions/{id}/attendance/{student_id}", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleRemoveAttendance))
	mux.HandleFunc("POST /sessions/{id}/remove/{student_id}", app.RequireRole(models.RoleCoach, models.RoleAdmin, models.RoleOperationManager)(app.HandleRemoveAttendance))

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
