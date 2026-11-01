package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

// LandingPageViewData is the data context passed to landing.html.
type LandingPageViewData struct {
	CurrentUser        *models.User
	Locations          []*models.Location
	SelectedLocation   *models.Location
	SelectedLocationID string
	Calendar           CalendarWeek
	SelectedDate       string
}

// LandingSchedulePartialData is the data context for landing_calendar_grid.html.
type LandingSchedulePartialData struct {
	Calendar           CalendarWeek
	Locations          []*models.Location
	SelectedLocation   *models.Location
	SelectedLocationID string
	SelectedDate       string
}

// GuestReserveFormData is the data context passed to guest_reservation_form.html.
type GuestReserveFormData struct {
	SessionID     string
	SessionTitle  string
	TrainingType  string
	FormattedDate string
	FormattedTime string
	CoachName     string
	LocationName  string
	GuestName     string
	ContactNumber string
	ErrorMessage  string
}

// GuestReservationSuccessData is the data context for guest_reservation_success.html.
type GuestReservationSuccessData struct {
	Session       *models.TrainingSession
	Attendance    *models.Attendance
	FormattedDate string
	FormattedTime string
}

// HandleLandingPage serves the public landing page at GET /.
// If an authenticated user visits without the preview parameter, they are redirected to their workspace.
func (a *AppHandler) HandleLandingPage(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user != nil && r.URL.Query().Get("preview") != "1" {
		if user.IsStudent() {
			http.Redirect(w, r, "/portal/student", http.StatusSeeOther)
			return
		}
		if user.IsCoach() || user.IsAdmin() {
			http.Redirect(w, r, "/sessions", http.StatusSeeOther)
			return
		}
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
		return
	}

	locations, _ := a.store.GetAllLocations()

	// Determine selected location: check query param first, then fallback to default location
	locQuery := strings.TrimSpace(r.URL.Query().Get("location"))
	var selectedLoc *models.Location
	if locQuery != "" && locQuery != "all" {
		if locID, err := uuid.Parse(locQuery); err == nil {
			selectedLoc, _ = a.store.GetLocationByID(locID)
		}
	}
	if selectedLoc == nil && locQuery != "all" {
		selectedLoc, _ = a.store.GetDefaultLocation()
		if selectedLoc == nil && len(locations) > 0 {
			selectedLoc = locations[0]
		}
	}

	selectedLocID := ""
	if selectedLoc != nil {
		selectedLocID = selectedLoc.ID.String()
	} else if locQuery != "" {
		selectedLocID = locQuery
	}

	dateParam := strings.TrimSpace(r.URL.Query().Get("date"))
	if dateParam == "" {
		dateParam = strings.TrimSpace(r.URL.Query().Get("week_date"))
	}

	calWeek, _, err := a.BuildCalendarWeek(dateParam, selectedLocID, "", "", "", "", true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := LandingPageViewData{
		CurrentUser:        user,
		Locations:          locations,
		SelectedLocation:   selectedLoc,
		SelectedLocationID: selectedLocID,
		Calendar:           calWeek,
		SelectedDate:       dateParam,
	}

	a.RenderPage(w, "landing.html", data)
}

// HandleLandingSchedule serves the HTMX partial for filtering the schedule grid by location/week.
func (a *AppHandler) HandleLandingSchedule(w http.ResponseWriter, r *http.Request) {
	locations, _ := a.store.GetAllLocations()
	locQuery := strings.TrimSpace(r.URL.Query().Get("location"))
	var selectedLoc *models.Location
	if locQuery != "" && locQuery != "all" {
		if locID, err := uuid.Parse(locQuery); err == nil {
			selectedLoc, _ = a.store.GetLocationByID(locID)
		}
	}
	if selectedLoc == nil && locQuery != "all" {
		selectedLoc, _ = a.store.GetDefaultLocation()
		if selectedLoc == nil && len(locations) > 0 {
			selectedLoc = locations[0]
		}
	}

	selectedLocID := ""
	if selectedLoc != nil {
		selectedLocID = selectedLoc.ID.String()
	} else if locQuery != "" {
		selectedLocID = locQuery
	}

	dateParam := strings.TrimSpace(r.URL.Query().Get("date"))
	if dateParam == "" {
		dateParam = strings.TrimSpace(r.URL.Query().Get("week_date"))
	}

	calWeek, _, err := a.BuildCalendarWeek(dateParam, selectedLocID, "", "", "", "", true)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	data := LandingSchedulePartialData{
		Calendar:           calWeek,
		Locations:          locations,
		SelectedLocation:   selectedLoc,
		SelectedLocationID: selectedLocID,
		SelectedDate:       dateParam,
	}

	a.RenderPartial(w, "landing_calendar_grid.html", data)
}

// HandleGuestReserveForm renders the modal form for guest slot reservation at GET /landing/reserve-form.
func (a *AppHandler) HandleGuestReserveForm(w http.ResponseWriter, r *http.Request) {
	sessionIDStr := strings.TrimSpace(r.URL.Query().Get("session_id"))
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	sess, err := a.store.GetSessionByID(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	title := sess.Title
	if title == "" {
		title = string(sess.TrainingType) + " Session"
	}
	timeStr := sess.StartTime
	if sess.EndTime != "" {
		timeStr += " – " + sess.EndTime
	}

	data := GuestReserveFormData{
		SessionID:     sess.ID.String(),
		SessionTitle:  title,
		TrainingType:  string(sess.TrainingType),
		FormattedDate: sess.SessionDate.Format("Monday, Jan 02, 2006"),
		FormattedTime: timeStr,
		CoachName:     sess.CoachName,
		LocationName:  sess.LocationName,
	}

	a.RenderPartial(w, "guest_reservation_form.html", data)
}

// HandleGuestReservation processes guest slot reservation at POST /landing/reserve.
func (a *AppHandler) HandleGuestReservation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	sessionIDStr := strings.TrimSpace(r.FormValue("session_id"))
	guestName := strings.TrimSpace(r.FormValue("guest_name"))
	contactNumber := strings.TrimSpace(r.FormValue("contact_number"))

	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		a.renderGuestReserveError(w, nil, guestName, contactNumber, "Invalid session selected.")
		return
	}

	sess, err := a.store.GetSessionByID(sessionID)
	if err != nil {
		a.renderGuestReserveError(w, nil, guestName, contactNumber, "Training session not found.")
		return
	}

	if guestName == "" {
		a.renderGuestReserveError(w, sess, guestName, contactNumber, "Please enter your full name.")
		return
	}

	if contactNumber == "" {
		a.renderGuestReserveError(w, sess, guestName, contactNumber, "Please enter your contact number.")
		return
	}

	att, err := a.store.ReserveSessionSlot(sessionID, guestName, contactNumber)
	if err != nil {
		errMsg := err.Error()
		if errors.Is(err, repository.ErrAlreadyInRoster) {
			errMsg = "You have already reserved a slot for this training session."
		}
		a.renderGuestReserveError(w, sess, guestName, contactNumber, errMsg)
		return
	}

	dateStr := sess.SessionDate.Format("Monday, Jan 02, 2006")
	timeStr := sess.StartTime
	if sess.EndTime != "" {
		timeStr += " – " + sess.EndTime
	}

	data := GuestReservationSuccessData{
		Session:       sess,
		Attendance:    att,
		FormattedDate: dateStr,
		FormattedTime: timeStr,
	}

	a.RenderPartial(w, "guest_reservation_success.html", data)
}

func (a *AppHandler) renderGuestReserveError(w http.ResponseWriter, sess *models.TrainingSession, guestName, contactNumber, errMsg string) {
	data := GuestReserveFormData{
		GuestName:     guestName,
		ContactNumber: contactNumber,
		ErrorMessage:  errMsg,
	}
	if sess != nil {
		data.SessionID = sess.ID.String()
		title := sess.Title
		if title == "" {
			title = string(sess.TrainingType) + " Session"
		}
		data.SessionTitle = title
		data.TrainingType = string(sess.TrainingType)
		data.FormattedDate = sess.SessionDate.Format("Monday, Jan 02, 2006")
		timeStr := sess.StartTime
		if sess.EndTime != "" {
			timeStr += " – " + sess.EndTime
		}
		data.FormattedTime = timeStr
		data.CoachName = sess.CoachName
		data.LocationName = sess.LocationName
	}
	a.RenderPartial(w, "guest_reservation_form.html", data)
}
