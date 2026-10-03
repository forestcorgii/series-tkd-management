package handlers

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/services"
)

type CoachListItem struct {
	Coach             *models.Coach
	Payroll           services.CoachPayrollSummary
	IsPendingApproval bool
}

type CoachesPageData struct {
	Coaches          []CoachListItem
	TotalCoaches     int
	ActiveCoaches    int
	PendingCoaches   int
	InactiveCoaches  int
	CertifiedCoaches int
	StartDate        string
	EndDate          string
	CurrentUser      *models.User
	SuccessNotice    string
	ErrorMessage     string
}

func (a *AppHandler) HandleCoaches(w http.ResponseWriter, r *http.Request) {
	coaches, err := a.store.GetAllCoaches()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessions, _ := a.store.GetAllSessions()
	allAttendances, _ := a.store.GetAllAttendances()

	now := time.Now()
	start := now.AddDate(0, -1, 0)
	end := now

	total := len(coaches)
	active := 0
	pending := 0
	inactive := 0
	certified := 0
	items := make([]CoachListItem, 0, len(coaches))
	for _, c := range coaches {
		isPending := false
		if c.IsActive {
			active++
		} else {
			u, _ := a.store.GetUserByEmail(c.Email)
			if u != nil && u.LastLoginAt == nil {
				isPending = true
				pending++
			} else {
				inactive++
			}
		}
		if c.IsFirstAidValid() {
			certified++
		}
		summary := a.payrollSvc.CalculateCoachPayroll(c, sessions, allAttendances, start, end)
		items = append(items, CoachListItem{
			Coach:             c,
			Payroll:           summary,
			IsPendingApproval: isPending,
		})
	}

	user := GetUserFromContext(r.Context())
	data := CoachesPageData{
		Coaches:          items,
		TotalCoaches:     total,
		ActiveCoaches:    active,
		PendingCoaches:   pending,
		InactiveCoaches:  inactive,
		CertifiedCoaches: certified,
		StartDate:        start.Format("2006-01-02"),
		EndDate:          end.Format("2006-01-02"),
		CurrentUser:      user,
		SuccessNotice:    r.URL.Query().Get("success"),
		ErrorMessage:     r.URL.Query().Get("error"),
	}

	a.RenderPage(w, "coaches.html", data)
}

func (a *AppHandler) HandleCreateCoach(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rate, _ := strconv.ParseFloat(r.FormValue("rate_per_session"), 64)
	firstAidCertified := r.FormValue("first_aid_certified") == "on" || r.FormValue("first_aid_certified") == "true"

	var firstAidExpiry *time.Time
	if expiryStr := r.FormValue("first_aid_expiry"); expiryStr != "" {
		if t, err := time.Parse("2006-01-02", expiryStr); err == nil {
			firstAidExpiry = &t
		}
	}

	specialtiesStr := r.FormValue("specialties")
	specs := []string{}
	for _, s := range strings.Split(specialtiesStr, ",") {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			specs = append(specs, trimmed)
		}
	}

	c := &models.Coach{
		FullName:          r.FormValue("full_name"),
		Email:             r.FormValue("email"),
		Phone:             r.FormValue("phone"),
		BeltRank:          r.FormValue("belt_rank"),
		RatePerSession:    rate,
		FirstAidCertified: firstAidCertified,
		FirstAidExpiry:    firstAidExpiry,
		Specialties:       specs,
		IsActive:          true,
	}

	if err := a.store.CreateCoach(c); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	password := strings.TrimSpace(r.FormValue("password"))
	if c.Email != "" {
		if password == "" {
			password = "coach123"
		}
		_, _ = a.authSvc.RegisterUser(c.Email, password, models.RoleCoach, nil, &c.ID)
	}

	http.Redirect(w, r, "/coaches", http.StatusSeeOther)
}

// HandleToggleCoachStatus toggles active/inactive state of a coach and their login access
func (a *AppHandler) HandleToggleCoachStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		http.Redirect(w, r, "/coaches?error="+url.QueryEscape("Invalid coach ID."), http.StatusSeeOther)
		return
	}

	coach, err := a.store.GetCoachByID(targetID)
	if err != nil || coach == nil {
		http.Redirect(w, r, "/coaches?error="+url.QueryEscape("Coach record not found."), http.StatusSeeOther)
		return
	}

	newStatus := !coach.IsActive
	if err := a.store.ToggleCoachActive(targetID, newStatus); err != nil {
		http.Redirect(w, r, "/coaches?error="+url.QueryEscape("Failed to update coach status: "+err.Error()), http.StatusSeeOther)
		return
	}

	statusMsg := "activated and can now log in"
	u, _ := a.store.GetUserByEmail(coach.Email)
	if u != nil && u.LastLoginAt == nil && newStatus {
		statusMsg = "approved and activated. They can now log in and lead classes"
	} else if !newStatus {
		statusMsg = "deactivated. Their active sessions have been terminated and login access is blocked"
	}

	http.Redirect(w, r, "/coaches?success="+url.QueryEscape(fmt.Sprintf("Coach '%s' has been %s.", coach.FullName, statusMsg)), http.StatusSeeOther)
}

// HandleDeleteCoach permanently removes a coach profile and linked account if no historical classes exist
func (a *AppHandler) HandleDeleteCoach(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	targetID, err := uuid.Parse(idStr)
	if err != nil {
		http.Redirect(w, r, "/coaches?error="+url.QueryEscape("Invalid coach ID."), http.StatusSeeOther)
		return
	}

	coach, err := a.store.GetCoachByID(targetID)
	if err != nil || coach == nil {
		http.Redirect(w, r, "/coaches?error="+url.QueryEscape("Coach record not found."), http.StatusSeeOther)
		return
	}

	if err := a.store.DeleteCoach(targetID); err != nil {
		http.Redirect(w, r, "/coaches?error="+url.QueryEscape("Failed to delete coach: "+err.Error()), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/coaches?success="+url.QueryEscape(fmt.Sprintf("Coach '%s' and all associated records have been permanently deleted.", coach.FullName)), http.StatusSeeOther)
}

