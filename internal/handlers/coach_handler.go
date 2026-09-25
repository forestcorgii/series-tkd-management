package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/services"
)

type CoachListItem struct {
	Coach   *models.Coach
	Payroll services.CoachPayrollSummary
}

type CoachesPageData struct {
	Coaches   []CoachListItem
	StartDate string
	EndDate   string
}

func (a *AppHandler) HandleCoaches(w http.ResponseWriter, r *http.Request) {
	coaches, err := a.store.GetAllCoaches()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessions, _ := a.store.GetAllSessions()

	now := time.Now()
	start := now.AddDate(0, -1, 0)
	end := now

	items := make([]CoachListItem, 0, len(coaches))
	for _, c := range coaches {
		summary := a.payrollSvc.CalculateCoachPayroll(c, sessions, nil, start, end)
		items = append(items, CoachListItem{
			Coach:   c,
			Payroll: summary,
		})
	}

	data := CoachesPageData{
		Coaches:   items,
		StartDate: start.Format("2006-01-02"),
		EndDate:   end.Format("2006-01-02"),
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

	http.Redirect(w, r, "/coaches", http.StatusSeeOther)
}
