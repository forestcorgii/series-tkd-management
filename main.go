package main

import (
	"log"
	"net/http"
	"os"

	"series-tkd-management/internal/handlers"
	"series-tkd-management/internal/repository"
)

func main() {
	// 1. Initialize Repository Store
	store := repository.NewMemoryStore()

	// 2. Initialize App Handlers & Templates
	app, err := handlers.NewAppHandler(store)
	if err != nil {
		log.Fatalf("Fatal: Failed to initialize AppHandler: %v", err)
	}

	// 3. Configure HTTP Routes
	mux := http.NewServeMux()

	// Dashboard
	mux.HandleFunc("GET /", app.HandleDashboard)

	// Students & Ability Radar
	mux.HandleFunc("GET /students", app.HandleStudents)
	mux.HandleFunc("GET /students/{id}", app.HandleStudentDetail)
	mux.HandleFunc("POST /students", app.HandleCreateStudent)
	mux.HandleFunc("POST /students/{id}/evaluations", app.HandleCreateEvaluation)

	// Coaches & Staff Directory / Payroll
	mux.HandleFunc("GET /coaches", app.HandleCoaches)
	mux.HandleFunc("POST /coaches", app.HandleCreateCoach)

	// Packages & Billing Passes
	mux.HandleFunc("GET /packages", app.HandlePackages)
	mux.HandleFunc("POST /packages/assign", app.HandleAssignPackage)

	// Training Sessions & Live Floor Tablet Check-In
	mux.HandleFunc("GET /sessions", app.HandleSessions)
	mux.HandleFunc("POST /sessions", app.HandleCreateSession)
	mux.HandleFunc("GET /sessions/{id}/live", app.HandleLiveSession)
	mux.HandleFunc("POST /sessions/{id}/search-student", app.HandleSearchStudent)
	mux.HandleFunc("POST /sessions/{id}/checkin/{student_id}", app.HandleCheckIn)

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
	log.Printf("=========================================================")

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
