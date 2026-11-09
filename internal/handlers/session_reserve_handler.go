package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

// SessionReservePageData represents the complete view state for the dedicated
// session reservation page (GET /reserve/{id}).
type SessionReservePageData struct {
	CurrentUser        *models.User
	Session            *models.TrainingSession
	Location           *models.Location
	FormattedDate      string
	FormattedTime      string
	FormattedTitle     string
	IsStudent          bool
	StudentName        string
	StudentBelt        string
	IsAlreadyReserved  bool
	IsCancelled        bool
	IsConcluded        bool
	IsFull             bool
	MaxCapacity        int
	CurrentTrainees    int
	RemainingSlots     int
	HasCapacityLimit   bool
	GuestName          string
	ContactNumber      string
	ErrorMessage       string
	SuccessMessage     string
	IsSuccess          bool
	ReservedAttendance *models.Attendance
	IsStaff            bool
	StaffRoleName      string
}

// HandleSessionReservePage serves the dedicated public self-reservation page for a single session at GET /reserve/{id}.
// Guests, students, and staff can access this page directly via shared links.
func (a *AppHandler) HandleSessionReservePage(w http.ResponseWriter, r *http.Request) {
	sessionIDStr := strings.TrimSpace(r.PathValue("id"))
	if sessionIDStr == "" {
		sessionIDStr = strings.TrimSpace(r.URL.Query().Get("id"))
	}
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		http.Error(w, "Invalid training session ID", http.StatusBadRequest)
		return
	}

	sess, err := a.store.GetSessionByID(sessionID)
	if err != nil || sess == nil {
		http.Error(w, "Training session not found or no longer available", http.StatusNotFound)
		return
	}

	data := a.buildSessionReserveData(r, sess)
	a.RenderPage(w, "session_reserve.html", data)
}

// HandleSessionReserveSubmit processes self-reservation submissions at POST /reserve/{id}.
// Supports both HTMX AJAX requests and standard form submissions.
func (a *AppHandler) HandleSessionReserveSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionIDStr := strings.TrimSpace(r.PathValue("id"))
	if sessionIDStr == "" {
		sessionIDStr = strings.TrimSpace(r.FormValue("session_id"))
	}
	sessionID, err := uuid.Parse(sessionIDStr)
	if err != nil {
		http.Error(w, "Invalid training session ID", http.StatusBadRequest)
		return
	}

	sess, err := a.store.GetSessionByID(sessionID)
	if err != nil || sess == nil {
		http.Error(w, "Training session not found or no longer available", http.StatusNotFound)
		return
	}

	data := a.buildSessionReserveData(r, sess)

	// Validate session status
	if sess.IsCancelled {
		data.ErrorMessage = "This training session has been cancelled and cannot accept reservations."
		a.renderReserveResponse(w, r, data)
		return
	}
	if sess.IsDone() || sess.SessionDate.Before(time.Now().Add(-24*time.Hour)) {
		data.ErrorMessage = "This training session has already concluded."
		a.renderReserveResponse(w, r, data)
		return
	}

	// Validate capacity limit
	if data.HasCapacityLimit && data.IsFull && !data.IsAlreadyReserved {
		data.ErrorMessage = fmt.Sprintf("This training session has reached maximum capacity (%d trainees).", data.MaxCapacity)
		a.renderReserveResponse(w, r, data)
		return
	}

	user := GetUserFromContext(r.Context())
	isStudent := user != nil && user.IsStudent()

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
					data.IsAlreadyReserved = true
				}
				data.ErrorMessage = errMsg
				a.renderReserveResponse(w, r, data)
				return
			}
			if st != nil && att.ContactNumber == "" {
				att.ContactNumber = st.Phone
			}
			if a.notifSvc != nil && st != nil {
				a.notifSvc.NotifySessionSlotReserved(sess, st.FullName, false, st.Phone)
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
					data.IsAlreadyReserved = true
				}
				data.ErrorMessage = errMsg
				a.renderReserveResponse(w, r, data)
				return
			}
			if a.notifSvc != nil {
				a.notifSvc.NotifySessionSlotReserved(sess, name, false, user.Email)
			}
		}
	} else {
		guestName := strings.TrimSpace(r.FormValue("guest_name"))
		contactNumber := strings.TrimSpace(r.FormValue("contact_number"))
		data.GuestName = guestName
		data.ContactNumber = contactNumber

		if guestName == "" {
			data.ErrorMessage = "Please enter your full name."
			a.renderReserveResponse(w, r, data)
			return
		}
		if contactNumber == "" {
			data.ErrorMessage = "Please enter your contact number."
			a.renderReserveResponse(w, r, data)
			return
		}

		att, err = a.store.ReserveSessionSlot(sessionID, guestName, contactNumber)
		if err != nil {
			errMsg := err.Error()
			if errors.Is(err, repository.ErrAlreadyInRoster) {
				errMsg = "You have already reserved a slot for this training session."
				data.IsAlreadyReserved = true
			}
			data.ErrorMessage = errMsg
			a.renderReserveResponse(w, r, data)
			return
		}
		if a.notifSvc != nil {
			a.notifSvc.NotifySessionSlotReserved(sess, guestName, true, contactNumber)
		}
	}

	// Recompute attendance count and remaining slots
	allAtt, _ := a.store.GetSessionAttendances(sess.ID)
	data.CurrentTrainees = len(allAtt)
	if data.HasCapacityLimit {
		data.RemainingSlots = data.MaxCapacity - data.CurrentTrainees
		if data.RemainingSlots < 0 {
			data.RemainingSlots = 0
		}
	}

	data.IsSuccess = true
	data.IsAlreadyReserved = true
	data.ReservedAttendance = att
	data.SuccessMessage = fmt.Sprintf("Your training slot has been confirmed! Please arrive 10–15 minutes before %s.", data.FormattedTime)

	a.renderReserveResponse(w, r, data)
}

func (a *AppHandler) buildSessionReserveData(r *http.Request, sess *models.TrainingSession) SessionReservePageData {
	user := GetUserFromContext(r.Context())

	title := sess.Title
	if title == "" {
		title = string(sess.TrainingType) + " Session"
	}
	dateStr := sess.SessionDate.Format("Monday, January 02, 2006")
	timeStr := sess.TimeRange()

	var loc *models.Location
	if sess.LocationID != nil {
		loc, _ = a.store.GetLocationByID(*sess.LocationID)
	}

	attendances, _ := a.store.GetSessionAttendances(sess.ID)
	currentCount := len(attendances)
	maxCap := 0
	hasCap := false
	isFull := false
	remSlots := 0

	if loc != nil && loc.HasMaxTrainees() {
		hasCap = true
		maxCap = loc.MaxTraineesVal()
		if currentCount >= maxCap {
			isFull = true
			remSlots = 0
		} else {
			remSlots = maxCap - currentCount
		}
	}

	data := SessionReservePageData{
		CurrentUser:      user,
		Session:          sess,
		Location:         loc,
		FormattedDate:    dateStr,
		FormattedTime:    timeStr,
		FormattedTitle:   title,
		IsCancelled:      sess.IsCancelled,
		IsConcluded:      sess.IsDone() || sess.SessionDate.Before(time.Now().Add(-24*time.Hour)),
		IsFull:           isFull,
		MaxCapacity:      maxCap,
		CurrentTrainees:  currentCount,
		RemainingSlots:   remSlots,
		HasCapacityLimit: hasCap,
	}

	if user != nil {
		if user.IsStudent() {
			data.IsStudent = true
			if user.StudentID != nil {
				if st, err := a.store.GetStudentByID(*user.StudentID); err == nil && st != nil {
					data.StudentName = st.FullName
					data.StudentBelt = string(st.CurrentBelt)
				}
				for _, att := range attendances {
					if att.StudentID == *user.StudentID {
						data.IsAlreadyReserved = true
						data.ReservedAttendance = att
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
		} else if user.IsStaff() {
			data.IsStaff = true
			data.StaffRoleName = user.PrimaryRoleName()
		}
	}

	return data
}

func (a *AppHandler) renderReserveResponse(w http.ResponseWriter, r *http.Request, data SessionReservePageData) {
	if r.Header.Get("HX-Request") == "true" && r.Header.Get("HX-Boosted") != "true" {
		a.RenderPartial(w, "session_reserve_card.html", data)
		return
	}
	a.RenderPage(w, "session_reserve.html", data)
}
