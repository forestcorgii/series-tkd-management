package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"

	"github.com/google/uuid"
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
	CurrentUser        *models.User
	Calendar           CalendarWeek
	Locations          []*models.Location
	SelectedLocation   *models.Location
	SelectedLocationID string
	SelectedDate       string
}

// GuestReserveFormData is the data context passed to guest_reservation_form.html.
type GuestReserveFormData struct {
	SessionID         string
	SessionTitle      string
	TrainingType      string
	FormattedDate     string
	FormattedTime     string
	CoachName         string
	LocationName      string
	GuestName         string
	ContactNumber     string
	ErrorMessage      string
	IsStudent         bool
	StudentName       string
	StudentBelt       string
	IsAlreadyReserved bool
}

// GuestReservationSuccessData is the data context for guest_reservation_success.html.
type GuestReservationSuccessData struct {
	Session       *models.TrainingSession
	Attendance    *models.Attendance
	FormattedDate string
	FormattedTime string
}

// HandleLandingPage serves the public landing page at GET /.
// If an authenticated staff user visits without the preview parameter, they are redirected to their workspace.
// Students remain on the landing page as their default calendar view.
func (a *AppHandler) HandleLandingPage(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user != nil && r.URL.Query().Get("preview") != "1" && !user.IsStudent() {
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

	if user != nil && user.IsStudent() && user.StudentID != nil {
		markStudentReservations(&calWeek, a.store, *user.StudentID)
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

	user := GetUserFromContext(r.Context())
	if user != nil && user.IsStudent() && user.StudentID != nil {
		markStudentReservations(&calWeek, a.store, *user.StudentID)
	}

	data := LandingSchedulePartialData{
		CurrentUser:        user,
		Calendar:           calWeek,
		Locations:          locations,
		SelectedLocation:   selectedLoc,
		SelectedLocationID: selectedLocID,
		SelectedDate:       dateParam,
	}

	a.RenderPartial(w, "landing_calendar_grid.html", data)
}

func markStudentReservations(calWeek *CalendarWeek, store repository.RepositoryStore, studentID uuid.UUID) {
	attendances, _ := store.GetStudentAttendances(studentID)
	if len(attendances) == 0 {
		return
	}
	reservedMap := make(map[uuid.UUID]bool, len(attendances))
	for _, att := range attendances {
		reservedMap[att.SessionID] = true
	}
	for _, day := range calWeek.Days {
		for _, cs := range day.Sessions {
			if cs.Session != nil && reservedMap[cs.Session.ID] {
				cs.IsUserReserved = true
			}
		}
	}
}

// HandleGuestReserveForm renders the modal form for guest or student slot reservation at GET /landing/reserve-form.
// For guests, it renders the form requiring full name and contact number.
// For authenticated students, the input form is omitted since identity is known from the session context.
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
	timeStr := sess.TimeRange()

	data := GuestReserveFormData{
		SessionID:     sess.ID.String(),
		SessionTitle:  title,
		TrainingType:  string(sess.TrainingType),
		FormattedDate: sess.SessionDate.Format("Monday, Jan 02, 2006"),
		FormattedTime: timeStr,
		CoachName:     sess.CoachName,
		LocationName:  sess.LocationName,
	}

	user := GetUserFromContext(r.Context())
	if user != nil && user.IsStudent() {
		data.IsStudent = true
		if user.StudentID != nil {
			if st, err := a.store.GetStudentByID(*user.StudentID); err == nil && st != nil {
				data.StudentName = st.FullName
				data.StudentBelt = string(st.CurrentBelt)
			}
			attendances, _ := a.store.GetStudentAttendances(*user.StudentID)
			for _, att := range attendances {
				if att.SessionID == sessionID {
					data.IsAlreadyReserved = true
					break
				}
			}
		}
		if data.StudentName == "" {
			if user.DisplayName != "" {
				data.StudentName = user.DisplayName
			} else {
				data.StudentName = user.Email
			}
		}
	}

	// Check location capacity
	if sess.LocationID != nil && !data.IsAlreadyReserved {
		if loc, err := a.store.GetLocationByID(*sess.LocationID); err == nil && loc != nil && loc.HasMaxTrainees() {
			attendances, _ := a.store.GetSessionAttendances(sess.ID)
			if len(attendances) >= loc.MaxTraineesVal() {
				data.ErrorMessage = fmt.Sprintf("This training session has reached full capacity (%d trainees maximum).", loc.MaxTraineesVal())
			}
		}
	}

	a.RenderPartial(w, "guest_reservation_form.html", data)
}

// HandleGuestReservation processes slot reservation at POST /landing/reserve.
// If an authenticated student reserves, their student ID is linked and form inputs are not required.
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
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		a.renderGuestReserveError(w, nil, "", "", "Invalid session selected.")
		return
	}

	sess, err := a.store.GetSessionByID(sessionID)
	if err != nil {
		a.renderGuestReserveError(w, nil, "", "", "Training session not found.")
		return
	}

	user := GetUserFromContext(r.Context())
	isStudent := user != nil && user.IsStudent()

	// Check capacity
	if sess.LocationID != nil {
		if loc, err := a.store.GetLocationByID(*sess.LocationID); err == nil && loc != nil && loc.HasMaxTrainees() {
			attendances, _ := a.store.GetSessionAttendances(sess.ID)
			if len(attendances) >= loc.MaxTraineesVal() {
				errMsg := fmt.Sprintf("This training session has reached full capacity (%d trainees maximum).", loc.MaxTraineesVal())
				if isStudent {
					a.renderStudentReserveError(w, sess, user, errMsg)
				} else {
					a.renderGuestReserveError(w, sess, r.FormValue("guest_name"), r.FormValue("contact_number"), errMsg)
				}
				return
			}
		}
	}

	var att *models.Attendance
	if isStudent {
		if user.StudentID != nil {
			var st *models.Student
			st, _ = a.store.GetStudentByID(*user.StudentID)
			att, err = a.store.CheckInStudent(sessionID, *user.StudentID, nil, nil)
			if err != nil {
				errMsg := err.Error()
				if errors.Is(err, repository.ErrAlreadyInRoster) {
					errMsg = "You have already reserved a slot for this training session."
				}
				a.renderStudentReserveError(w, sess, user, errMsg)
				return
			}
			if st != nil && att.ContactNumber == "" {
				att.ContactNumber = st.Phone
			}
			if a.notifSvc != nil && st != nil {
				a.notifSvc.NotifyStudentAdmitted(sess, st, true, st.FullName)
			}
		} else {
			name := user.DisplayName
			if name == "" {
				name = user.Email
			}
			att, err = a.store.ReserveSessionSlot(sessionID, name, "Student Account")
			if err != nil {
				errMsg := err.Error()
				if errors.Is(err, repository.ErrAlreadyInRoster) {
					errMsg = "You have already reserved a slot for this training session."
				}
				a.renderStudentReserveError(w, sess, user, errMsg)
				return
			}
		}
	} else {
		guestName := strings.TrimSpace(r.FormValue("guest_name"))
		contactNumber := strings.TrimSpace(r.FormValue("contact_number"))

		if guestName == "" {
			a.renderGuestReserveError(w, sess, guestName, contactNumber, "Please enter your full name.")
			return
		}
		if contactNumber == "" {
			a.renderGuestReserveError(w, sess, guestName, contactNumber, "Please enter your contact number.")
			return
		}

		att, err = a.store.ReserveSessionSlot(sessionID, guestName, contactNumber)
		if err != nil {
			errMsg := err.Error()
			if errors.Is(err, repository.ErrAlreadyInRoster) {
				errMsg = "You have already reserved a slot for this training session."
			}
			a.renderGuestReserveError(w, sess, guestName, contactNumber, errMsg)
			return
		}
	}

	dateStr := sess.SessionDate.Format("Monday, Jan 02, 2006")
	timeStr := sess.TimeRange()

	data := GuestReservationSuccessData{
		Session:       sess,
		Attendance:    att,
		FormattedDate: dateStr,
		FormattedTime: timeStr,
	}

	a.RenderPartial(w, "guest_reservation_success.html", data)
}

func (a *AppHandler) renderStudentReserveError(w http.ResponseWriter, sess *models.TrainingSession, user *models.User, errMsg string) {
	data := GuestReserveFormData{
		ErrorMessage: errMsg,
		IsStudent:    true,
	}
	if user != nil && user.StudentID != nil {
		if st, err := a.store.GetStudentByID(*user.StudentID); err == nil && st != nil {
			data.StudentName = st.FullName
			data.StudentBelt = string(st.CurrentBelt)
		}
		attendances, _ := a.store.GetStudentAttendances(*user.StudentID)
		if sess != nil {
			for _, att := range attendances {
				if att.SessionID == sess.ID {
					data.IsAlreadyReserved = true
					break
				}
			}
		}
	}
	if data.StudentName == "" && user != nil {
		if user.DisplayName != "" {
			data.StudentName = user.DisplayName
		} else {
			data.StudentName = user.Email
		}
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
		timeStr := sess.TimeRange()
		data.FormattedTime = timeStr
		data.CoachName = sess.CoachName
		data.LocationName = sess.LocationName
	}
	a.RenderPartial(w, "guest_reservation_form.html", data)
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
		timeStr := sess.TimeRange()
		data.FormattedTime = timeStr
		data.CoachName = sess.CoachName
		data.LocationName = sess.LocationName
	}
	a.RenderPartial(w, "guest_reservation_form.html", data)
}
