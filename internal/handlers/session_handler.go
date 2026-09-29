package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
	"series-tkd-management/internal/services"
)

type SessionListItem struct {
	Session         *models.TrainingSession
	AttendanceCount int
}

type CalendarSession struct {
	Session         *models.TrainingSession
	AttendanceCount int
	StartHour       int
	StartMinute     int
	EndHour         int
	EndMinute       int
	StartMinutes    int // total minutes from midnight
	EndMinutes      int // total minutes from midnight
	DurationMinutes int // total duration in minutes
	TopOffset       int // pixels from top of grid
	CardHeight      int // height in pixels
	ColumnIndex     int // column index for overlapping sessions
	TotalColumns    int // total parallel columns in overlapping group
	LeftPercent     int // left position percentage
	WidthPercent    int // width percentage
	TimeDisplay     string
	CategoryBadge   string
	StatusLabel     string
	StatusBadge     string
}

type CalendarDay struct {
	Date               time.Time
	DateString         string // "2006-01-02"
	DayName            string // "Monday"
	DayShort           string // "Mon"
	DayNumber          int    // 28
	MonthShort         string // "Sep"
	IsToday            bool
	IsSelected         bool
	Sessions           []*CalendarSession
	HourlySessions     map[int][]*CalendarSession
	ContinuingSessions map[int][]*CalendarSession
}

func (d *CalendarDay) GetSessionsForHour(hour int) []*CalendarSession {
	if d == nil || d.HourlySessions == nil {
		return nil
	}
	return d.HourlySessions[hour]
}

func (d *CalendarDay) GetContinuingSessionsForHour(hour int) []*CalendarSession {
	if d == nil || d.ContinuingSessions == nil {
		return nil
	}
	return d.ContinuingSessions[hour]
}

func (d *CalendarDay) HasSessionsForHour(hour int) bool {
	return len(d.GetSessionsForHour(hour)) > 0
}

func (d *CalendarDay) HasContinuingForHour(hour int) bool {
	return len(d.GetContinuingSessionsForHour(hour)) > 0
}

// IsTopHalfFree returns true if the interval [hour:00, hour:30] has no active session
func (d *CalendarDay) IsTopHalfFree(hour int) bool {
	if d == nil {
		return true
	}
	topStart := hour * 60
	topEnd := hour*60 + 30
	for _, s := range d.Sessions {
		if !s.Session.IsCancelled && s.StartMinutes < topEnd && s.EndMinutes > topStart {
			return false
		}
	}
	return true
}

// IsBottomHalfFree returns true if the interval [hour:30, (hour+1):00] has no active session
func (d *CalendarDay) IsBottomHalfFree(hour int) bool {
	if d == nil {
		return true
	}
	botStart := hour*60 + 30
	botEnd := (hour + 1) * 60
	for _, s := range d.Sessions {
		if !s.Session.IsCancelled && s.StartMinutes < botEnd && s.EndMinutes > botStart {
			return false
		}
	}
	return true
}

func (d *CalendarDay) IsHourCompletelyFree(hour int) bool {
	return d.IsTopHalfFree(hour) && d.IsBottomHalfFree(hour)
}

type CalendarHour struct {
	Hour      int    // 8..21
	TimeLabel string // "08:00 AM"
	Time24    string // "08:00"
}

type CalendarWeek struct {
	StartDate        time.Time
	EndDate          time.Time
	StartDateStr     string // "2026-09-28"
	EndDateStr       string // "2026-10-04"
	PrevWeekStr      string // "2026-09-21"
	NextWeekStr      string // "2026-10-05"
	TodayStr         string // "2026-09-28"
	Title            string // "Sep 28 – Oct 04, 2026"
	MonthYear        string // "September 2026"
	Days             []*CalendarDay
	Hours            []CalendarHour
	TotalSessions    int
	MinHour          int
	MaxHour          int
	HourHeight       int
	GridHeight       int
	NowOffset        int
	ShowNowLine      bool
	CurrentTimeLabel string
}

type SessionsPageData struct {
	CurrentUser        *models.User
	Sessions           []*models.TrainingSession
	Coaches            []*models.Coach
	Students           []*models.Student
	FilterCoachID      string
	FilterStudentID    string
	FilterCategory     string
	FilterDay          string
	Calendar           CalendarWeek
	SelectedDate       string
	ViewMode           string // "grid" (default) or "agenda"
	Page               int
	TotalPages         int
	TotalMatchingCount int
	DisplayStart       int
	DisplayEnd         int
	HasPrevPage        bool
	HasNextPage        bool
	PrevPage           int
	NextPage           int
	PageNumbers        []int
}

type LiveCheckInPageData struct {
	CurrentUser      *models.User
	Session          *models.TrainingSession
	Attendances      []*models.Attendance
	Students         []*models.Student
	ReadinessMap     map[string]services.PromotionReadiness
	PackageStatusMap map[string]string
}

type StudentSearchResultItem struct {
	SessionID        uuid.UUID
	Student          *models.Student
	Readiness        services.PromotionReadiness
	ActivePackage    *models.StudentPackage
	IsAlreadyChecked bool
	WarningMessage   string
}

func parseTimeHM(timeStr string) (int, int) {
	timeStr = strings.TrimSpace(timeStr)
	for _, layout := range []string{"15:04", "15:04:05", "3:04 PM", "3:04PM", "03:04 PM", "03:04PM"} {
		if t, err := time.Parse(layout, timeStr); err == nil {
			return t.Hour(), t.Minute()
		}
	}
	parts := strings.Split(timeStr, ":")
	if len(parts) >= 2 {
		h, _ := strconv.Atoi(parts[0])
		mStr := parts[1]
		if len(mStr) > 2 {
			mStr = mStr[:2]
		}
		m, _ := strconv.Atoi(mStr)
		return h, m
	}
	return 0, 0
}

func formatHourLabel(h int) string {
	period := "AM"
	displayH := h
	if h >= 12 {
		period = "PM"
	}
	if h > 12 {
		displayH = h - 12
	} else if h == 0 {
		displayH = 12
	}
	return fmt.Sprintf("%02d:00 %s", displayH, period)
}

func (a *AppHandler) HandleSessions(w http.ResponseWriter, r *http.Request) {
	coachIDStr := strings.TrimSpace(r.URL.Query().Get("coach_id"))
	studentIDStr := strings.TrimSpace(r.URL.Query().Get("student_id"))
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	viewMode := strings.TrimSpace(r.URL.Query().Get("view"))
	if viewMode == "" {
		viewMode = "grid"
	}

	dateParam := strings.TrimSpace(r.URL.Query().Get("date"))
	if dateParam == "" {
		dateParam = strings.TrimSpace(r.URL.Query().Get("day"))
	}
	if dateParam == "" {
		dateParam = strings.TrimSpace(r.URL.Query().Get("week_date"))
	}

	today := time.Now()
	todayStr := today.Format("2006-01-02")
	refDate := today

	// Pre-filter by coach, student, and category
	baseFilter := repository.SessionFilter{
		TrainingType: category,
	}
	if coachIDStr != "" {
		if cID, err := uuid.Parse(coachIDStr); err == nil {
			baseFilter.CoachID = &cID
		}
	}
	if studentIDStr != "" {
		if sID, err := uuid.Parse(studentIDStr); err == nil {
			baseFilter.StudentID = &sID
		}
	}

	allMatching, err := a.store.GetSessions(baseFilter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if dateParam != "" {
		if t, err := time.Parse("2006-01-02", dateParam); err == nil {
			refDate = t
		}
	} else if len(allMatching) > 0 {
		// If no date specified, check if today's week has sessions.
		// If not, but matching sessions exist, center on the first matching session's week.
		todayWeekday := today.Weekday()
		todayOffset := (int(todayWeekday) + 6) % 7
		currWeekStart := time.Date(today.Year(), today.Month(), today.Day()-todayOffset, 0, 0, 0, 0, today.Location())
		currWeekEnd := currWeekStart.AddDate(0, 0, 6)
		currStartStr := currWeekStart.Format("2006-01-02")
		currEndStr := currWeekEnd.Format("2006-01-02")

		hasCurrentWeekSession := false
		for _, s := range allMatching {
			sDate := s.SessionDate.Format("2006-01-02")
			if sDate >= currStartStr && sDate <= currEndStr {
				hasCurrentWeekSession = true
				break
			}
		}
		if !hasCurrentWeekSession {
			refDate = allMatching[0].SessionDate
		}
	}

	// Compute Monday-to-Sunday week boundary for refDate
	weekday := refDate.Weekday()
	offset := (int(weekday) + 6) % 7 // Monday = 0, ..., Sunday = 6
	weekStart := time.Date(refDate.Year(), refDate.Month(), refDate.Day()-offset, 0, 0, 0, 0, refDate.Location())
	weekEnd := weekStart.AddDate(0, 0, 6)
	startDateStr := weekStart.Format("2006-01-02")
	endDateStr := weekEnd.Format("2006-01-02")
	prevWeekStr := weekStart.AddDate(0, 0, -7).Format("2006-01-02")
	nextWeekStr := weekStart.AddDate(0, 0, 7).Format("2006-01-02")

	// Filter sessions within the selected week
	var sessions []*models.TrainingSession
	for _, s := range allMatching {
		sDate := s.SessionDate.Format("2006-01-02")
		if sDate >= startDateStr && sDate <= endDateStr {
			sessions = append(sessions, s)
		}
	}
	coaches, _ := a.store.GetAllCoaches()

	// Initialize 7 days (Monday through Sunday)
	days := make([]*CalendarDay, 7)
	daysMap := make(map[string]*CalendarDay)
	for i := 0; i < 7; i++ {
		d := weekStart.AddDate(0, 0, i)
		dStr := d.Format("2006-01-02")
		calDay := &CalendarDay{
			Date:               d,
			DateString:         dStr,
			DayName:            d.Format("Monday"),
			DayShort:           d.Format("Mon"),
			DayNumber:          d.Day(),
			MonthShort:         d.Format("Jan"),
			IsToday:            dStr == todayStr,
			IsSelected:         dStr == dateParam || (dateParam == "" && dStr == todayStr),
			Sessions:           make([]*CalendarSession, 0),
			HourlySessions:     make(map[int][]*CalendarSession),
			ContinuingSessions: make(map[int][]*CalendarSession),
		}
		days[i] = calDay
		daysMap[dStr] = calDay
	}

	minHour := 8
	maxHour := 21

	// Map sessions into days and hours
	for _, s := range sessions {
		startH, startM := parseTimeHM(s.StartTime)
		endH, endM := parseTimeHM(s.EndTime)

		if startH < minHour && startH >= 0 {
			minHour = startH
		}
		if endH > maxHour && endH <= 23 {
			maxHour = endH
		}

		atts, _ := a.store.GetSessionAttendances(s.ID)
		attCount := len(atts)

		catBadge := "bg-rose-50 text-rose-700 border-rose-300 dark:bg-rose-950/40 dark:text-rose-300 dark:border-rose-800"
		switch s.TrainingType {
		case models.TrainingPoomsae:
			catBadge = "bg-indigo-50 text-indigo-700 border-indigo-300 dark:bg-indigo-950/40 dark:text-indigo-300 dark:border-indigo-800"
		case models.TrainingConditioning:
			catBadge = "bg-amber-50 text-amber-700 border-amber-300 dark:bg-amber-950/40 dark:text-amber-300 dark:border-amber-800"
		case models.TrainingPromotionPrep:
			catBadge = "bg-purple-50 text-purple-700 border-purple-300 dark:bg-purple-950/40 dark:text-purple-300 dark:border-purple-800"
		}

		statusLabel := "UPCOMING"
		statusBadge := "bg-slate-100 text-slate-700 border-slate-300 dark:bg-slate-800 dark:text-slate-300 dark:border-slate-700"
		if s.IsCancelled {
			statusLabel = "CANCELLED"
			statusBadge = "bg-rose-100 text-rose-700 border-rose-300 dark:bg-rose-950 dark:text-rose-300 dark:border-rose-800"
		} else if s.IsDone() {
			statusLabel = "CLOSED"
			statusBadge = "bg-slate-200 text-slate-600 border-slate-300 dark:bg-slate-800/80 dark:text-slate-400 dark:border-slate-700"
		} else if s.SessionDate.Format("2006-01-02") == todayStr && s.IsOpen() {
			statusLabel = "LIVE"
			statusBadge = "bg-emerald-500 text-white font-bold animate-pulse shadow-sm"
		}

		timeDisp := s.StartTime
		if s.EndTime != "" {
			timeDisp = fmt.Sprintf("%s - %s", s.StartTime, s.EndTime)
		}

		startTotal := startH*60 + startM
		endTotal := endH*60 + endM
		if endTotal <= startTotal {
			endTotal = startTotal + 120
		}
		durationTotal := endTotal - startTotal
		if durationTotal <= 0 {
			durationTotal = 120
		}

		calSess := &CalendarSession{
			Session:         s,
			AttendanceCount: attCount,
			StartHour:       startH,
			StartMinute:     startM,
			EndHour:         endH,
			EndMinute:       endM,
			StartMinutes:    startTotal,
			EndMinutes:      endTotal,
			DurationMinutes: durationTotal,
			TimeDisplay:     timeDisp,
			CategoryBadge:   catBadge,
			StatusLabel:     statusLabel,
			StatusBadge:     statusBadge,
		}

		sessDateStr := s.SessionDate.Format("2006-01-02")
		if dayObj, ok := daysMap[sessDateStr]; ok {
			dayObj.Sessions = append(dayObj.Sessions, calSess)
			dayObj.HourlySessions[startH] = append(dayObj.HourlySessions[startH], calSess)

			// Mark subsequent hours that this session spans through as continuing
			if !s.IsCancelled {
				for h := startH + 1; h <= 23; h++ {
					if endTotal > h*60 {
						dayObj.ContinuingSessions[h] = append(dayObj.ContinuingSessions[h], calSess)
					}
				}
			}
		}
	}

	hourHeight := 60 // 1 minute = 1 pixel
	totalHours := maxHour - minHour + 1
	gridHeight := totalHours * hourHeight

	// Calculate TopOffset, CardHeight, and side-by-side overlap columns for each day
	for _, dayObj := range days {
		sort.Slice(dayObj.Sessions, func(i, j int) bool {
			if dayObj.Sessions[i].StartMinutes == dayObj.Sessions[j].StartMinutes {
				return dayObj.Sessions[i].EndMinutes > dayObj.Sessions[j].EndMinutes
			}
			return dayObj.Sessions[i].StartMinutes < dayObj.Sessions[j].StartMinutes
		})

		for _, s := range dayObj.Sessions {
			startDiff := s.StartMinutes - minHour*60
			s.TopOffset = startDiff * hourHeight / 60
			s.CardHeight = (s.DurationMinutes * hourHeight / 60) - 2
			if s.CardHeight < 28 {
				s.CardHeight = 28
			}
		}

		// Calculate columns for side-by-side overlaps
		var colEndTimes []int
		for _, s := range dayObj.Sessions {
			placed := false
			for col, endTime := range colEndTimes {
				if s.StartMinutes >= endTime {
					s.ColumnIndex = col
					colEndTimes[col] = s.EndMinutes
					placed = true
					break
				}
			}
			if !placed {
				s.ColumnIndex = len(colEndTimes)
				colEndTimes = append(colEndTimes, s.EndMinutes)
			}
		}

		for _, s := range dayObj.Sessions {
			maxCol := s.ColumnIndex
			for _, other := range dayObj.Sessions {
				if other != s && s.StartMinutes < other.EndMinutes && s.EndMinutes > other.StartMinutes {
					if other.ColumnIndex > maxCol {
						maxCol = other.ColumnIndex
					}
				}
			}
			s.TotalColumns = maxCol + 1
		}

		for _, s := range dayObj.Sessions {
			for _, other := range dayObj.Sessions {
				if other != s && s.StartMinutes < other.EndMinutes && s.EndMinutes > other.StartMinutes {
					if other.TotalColumns > s.TotalColumns {
						s.TotalColumns = other.TotalColumns
					}
				}
			}
			if s.TotalColumns < 1 {
				s.TotalColumns = 1
			}
			s.LeftPercent = (s.ColumnIndex * 100) / s.TotalColumns
			s.WidthPercent = 100 / s.TotalColumns
		}
	}

	now := time.Now()
	nowTotalMin := now.Hour()*60 + now.Minute()
	minTotalMin := minHour * 60
	maxTotalMin := (maxHour + 1) * 60
	showNowLine := false
	nowOffset := 0
	if nowTotalMin >= minTotalMin && nowTotalMin <= maxTotalMin {
		showNowLine = true
		nowOffset = (nowTotalMin - minTotalMin) * hourHeight / 60
	}
	currentTimeLabel := now.Format("3:04 PM")

	var hours []CalendarHour
	for h := minHour; h <= maxHour; h++ {
		hours = append(hours, CalendarHour{
			Hour:      h,
			TimeLabel: formatHourLabel(h),
			Time24:    fmt.Sprintf("%02d:00", h),
		})
	}

	weekTitle := fmt.Sprintf("%s – %s", weekStart.Format("Jan 02"), weekEnd.Format("Jan 02, 2006"))
	calWeek := CalendarWeek{
		StartDate:        weekStart,
		EndDate:          weekEnd,
		StartDateStr:     startDateStr,
		EndDateStr:       endDateStr,
		PrevWeekStr:      prevWeekStr,
		NextWeekStr:      nextWeekStr,
		TodayStr:         todayStr,
		Title:            weekTitle,
		MonthYear:        weekStart.Format("January 2006"),
		Days:             days,
		Hours:            hours,
		TotalSessions:    len(sessions),
		MinHour:          minHour,
		MaxHour:          maxHour,
		HourHeight:       hourHeight,
		GridHeight:       gridHeight,
		NowOffset:        nowOffset,
		ShowNowLine:      showNowLine,
		CurrentTimeLabel: currentTimeLabel,
	}

	students, _ := a.store.GetAllStudents()
	user := GetUserFromContext(r.Context())

	// Sort Class Rosters & Quick Operations items by the more recent (newest first)
	rosterSessions := make([]*models.TrainingSession, len(allMatching))
	copy(rosterSessions, allMatching)
	sort.Slice(rosterSessions, func(i, j int) bool {
		dateI := rosterSessions[i].SessionDate.Format("2006-01-02")
		dateJ := rosterSessions[j].SessionDate.Format("2006-01-02")
		if dateI != dateJ {
			return dateI > dateJ // more recent date first
		}
		if rosterSessions[i].StartTime != rosterSessions[j].StartTime {
			return rosterSessions[i].StartTime > rosterSessions[j].StartTime // later time first
		}
		return rosterSessions[i].CreatedAt.After(rosterSessions[j].CreatedAt)
	})

	// Pagination for Class Rosters & Quick Operations (10 per page)
	const rosterPageSize = 10
	pageStr := strings.TrimSpace(r.URL.Query().Get("page"))
	page := 1
	if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
		page = p
	}

	totalMatchingCount := len(rosterSessions)
	totalPages := (totalMatchingCount + rosterPageSize - 1) / rosterPageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page > totalPages {
		page = totalPages
	}

	startIndex := (page - 1) * rosterPageSize
	endIndex := startIndex + rosterPageSize
	if endIndex > totalMatchingCount {
		endIndex = totalMatchingCount
	}

	var pagedSessions []*models.TrainingSession
	if startIndex < totalMatchingCount {
		pagedSessions = rosterSessions[startIndex:endIndex]
	} else {
		pagedSessions = []*models.TrainingSession{}
	}

	displayStart := 0
	displayEnd := 0
	if totalMatchingCount > 0 {
		displayStart = startIndex + 1
		displayEnd = endIndex
	}

	var pageNumbers []int
	startPage := page - 2
	if startPage < 1 {
		startPage = 1
	}
	endPage := startPage + 4
	if endPage > totalPages {
		endPage = totalPages
		startPage = endPage - 4
		if startPage < 1 {
			startPage = 1
		}
	}
	for i := startPage; i <= endPage; i++ {
		pageNumbers = append(pageNumbers, i)
	}

	data := SessionsPageData{
		CurrentUser:        user,
		Sessions:           pagedSessions,
		Coaches:            coaches,
		Students:           students,
		FilterCoachID:      coachIDStr,
		FilterStudentID:    studentIDStr,
		FilterCategory:     category,
		FilterDay:          dateParam,
		Calendar:           calWeek,
		SelectedDate:       dateParam,
		ViewMode:           viewMode,
		Page:               page,
		TotalPages:         totalPages,
		TotalMatchingCount: totalMatchingCount,
		DisplayStart:       displayStart,
		DisplayEnd:         displayEnd,
		HasPrevPage:        page > 1,
		HasNextPage:        page < totalPages,
		PrevPage:           page - 1,
		NextPage:           page + 1,
		PageNumbers:        pageNumbers,
	}

	if r.Header.Get("HX-Request") == "true" {
		a.RenderPartial(w, "session_cards.html", data)
		return
	}

	a.RenderPage(w, "sessions.html", data)
}

func (a *AppHandler) HandleCreateSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	coachID, _ := uuid.Parse(r.FormValue("coach_id"))
	dateStr := r.FormValue("session_date")
	sessDate, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		sessDate = time.Now()
	}

	sess := &models.TrainingSession{
		SessionDate:  sessDate,
		StartTime:    r.FormValue("start_time"),
		EndTime:      r.FormValue("end_time"),
		CoachID:      coachID,
		TrainingType: models.TrainingType(r.FormValue("training_type")),
		Notes:        r.FormValue("notes"),
	}

	if err := a.store.CreateSession(sess); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	http.Redirect(w, r, "/sessions/"+sess.ID.String()+"/live", http.StatusSeeOther)
}

func (a *AppHandler) HandleLiveSession(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/sessions/")
	idStr = strings.TrimSuffix(idStr, "/live")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	session, err := a.store.GetSessionByID(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}

	attendances, _ := a.store.GetSessionAttendances(sessionID)
	allStudents, _ := a.store.GetAllStudents()
	allSessions, _ := a.store.GetAllSessions()

	allSessionsMap := make(map[string]*models.TrainingSession)
	for _, s := range allSessions {
		allSessionsMap[s.ID.String()] = s
	}

	readinessMap := make(map[string]services.PromotionReadiness)
	pkgStatusMap := make(map[string]string)

	for _, st := range allStudents {
		atts, _ := a.store.GetStudentAttendances(st.ID)
		latestEval, _ := a.store.GetLatestEvaluation(st.ID)
		rStatus := a.promotionSvc.EvaluateReadiness(st, atts, allSessionsMap, latestEval)
		readinessMap[st.ID.String()] = rStatus

		pkgs, _ := a.store.GetStudentPackages(st.ID)
		oldestValid, err := a.packageSvc.FindOldestValidPackage(pkgs, atts, time.Now())
		if err != nil {
			pkgStatusMap[st.ID.String()] = "No Active Credits ⚠️"
		} else if oldestValid.IsFourWeek() {
			weekNum, _, _ := oldestValid.CurrentCycleWindow(time.Now())
			if oldestValid.RemainingSessions != nil {
				pkgStatusMap[st.ID.String()] = fmt.Sprintf("Wk %d: %d left (%dx/wk)", weekNum, *oldestValid.RemainingSessions, oldestValid.WeeklyCadence())
			} else {
				pkgStatusMap[st.ID.String()] = fmt.Sprintf("Wk %d 4-Week Pass", weekNum)
			}
		} else if oldestValid.RemainingSessions != nil {
			pkgStatusMap[st.ID.String()] = fmt.Sprintf("%d sessions left", *oldestValid.RemainingSessions)
		} else {
			pkgStatusMap[st.ID.String()] = "Unlimited Pass ♾️"
		}
	}

	user := GetUserFromContext(r.Context())
	data := LiveCheckInPageData{
		CurrentUser:      user,
		Session:          session,
		Attendances:      attendances,
		Students:         allStudents,
		ReadinessMap:     readinessMap,
		PackageStatusMap: pkgStatusMap,
	}

	a.RenderPage(w, "live_checkin.html", data)
}

func (a *AppHandler) HandleSearchStudent(w http.ResponseWriter, r *http.Request) {
	idStr := strings.TrimPrefix(r.URL.Path, "/sessions/")
	idStr = strings.TrimSuffix(idStr, "/search-student")
	sessionID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	query := r.FormValue("query")
	students, _ := a.store.SearchStudents(query)
	existingAttendances, _ := a.store.GetSessionAttendances(sessionID)

	checkedMap := make(map[string]bool)
	for _, att := range existingAttendances {
		checkedMap[att.StudentID.String()] = true
	}

	allSessions, _ := a.store.GetAllSessions()
	allSessionsMap := make(map[string]*models.TrainingSession)
	for _, s := range allSessions {
		allSessionsMap[s.ID.String()] = s
	}

	results := []StudentSearchResultItem{}
	for _, st := range students {
		atts, _ := a.store.GetStudentAttendances(st.ID)
		latestEval, _ := a.store.GetLatestEvaluation(st.ID)
		readiness := a.promotionSvc.EvaluateReadiness(st, atts, allSessionsMap, latestEval)

		pkgs, _ := a.store.GetStudentPackages(st.ID)
		validPkg, pkgErr := a.packageSvc.FindOldestValidPackage(pkgs, atts, time.Now())

		warning := ""
		if pkgErr != nil {
			warning = pkgErr.Error()
		}

		results = append(results, StudentSearchResultItem{
			SessionID:        sessionID,
			Student:          st,
			Readiness:        readiness,
			ActivePackage:    validPkg,
			IsAlreadyChecked: checkedMap[st.ID.String()],
			WarningMessage:   warning,
		})
	}

	a.RenderPartial(w, "search_results.html", results)
}

func (a *AppHandler) HandleCancelSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Path: /sessions/{id}/cancel
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	sessionID, err := uuid.Parse(pathParts[2])
	if err != nil {
		http.Error(w, "Invalid session ID", http.StatusBadRequest)
		return
	}

	_ = r.ParseForm()
	reason := strings.TrimSpace(r.FormValue("reason"))
	if reason == "" {
		reason = "Cancelled by staff"
	}

	refundVal := r.FormValue("refund_credits")
	refundCredits := refundVal == "true" || refundVal == "on" || refundVal == "1"

	if err := a.store.CancelSession(sessionID, reason, refundCredits); err != nil {
		http.Error(w, fmt.Sprintf("Failed to cancel session: %v", err), http.StatusInternalServerError)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}

	referer := r.Header.Get("Referer")
	if referer != "" {
		http.Redirect(w, r, referer, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/sessions", http.StatusSeeOther)
}

func (a *AppHandler) HandleCheckIn(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(r.URL.Path, "/")
	// /sessions/{sessionID}/checkin/{studentID}
	if len(pathParts) < 5 {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	sessionID, err1 := uuid.Parse(pathParts[2])
	studentID, err2 := uuid.Parse(pathParts[4])
	if err1 != nil || err2 != nil {
		http.Error(w, "Invalid parameters", http.StatusBadRequest)
		return
	}

	session, err := a.store.GetSessionByID(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}
	if session.IsCancelled {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`<div class="p-3 bg-rose-950 border border-rose-800 text-rose-300 rounded-lg text-sm">
			❌ Check-in rejected: This class has been cancelled.
		</div>`))
		return
	}

	pkgs, _ := a.store.GetStudentPackages(studentID)
	atts, _ := a.store.GetStudentAttendances(studentID)
	override := r.FormValue("override") == "true"

	usedPkg, err := a.packageSvc.ProcessCheckInDeduction(pkgs, atts, time.Now())
	var pkgID *uuid.UUID
	if err == nil && usedPkg != nil {
		pkgID = &usedPkg.ID
		_ = a.store.UpdateStudentPackage(usedPkg)
	} else if !override {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusPaymentRequired)
		w.Write([]byte(fmt.Sprintf(`<div class="p-3 bg-rose-950 border border-rose-800 text-rose-300 rounded-lg text-sm">
			❌ Check-in rejected: %v. <button hx-post="/sessions/%s/checkin/%s?override=true" hx-target="#attendance-roster" hx-swap="afterbegin" class="underline font-bold ml-2">Manual Override</button>
		</div>`, err, sessionID, studentID)))
		return
	} else if override {
		for _, p := range pkgs {
			if p.IsValidAt(time.Now()) {
				_ = p.DeductSession(time.Now())
				pkgID = &p.ID
				_ = a.store.UpdateStudentPackage(p)
				break
			}
		}
	}

	att, err := a.store.CheckInStudent(sessionID, studentID, pkgID)
	if err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusConflict)
		w.Write([]byte(fmt.Sprintf(`<div class="p-3 bg-amber-950 border border-amber-800 text-amber-300 rounded-lg text-sm">⚠️ %v</div>`, err)))
		return
	}

	st, _ := a.store.GetStudentByID(studentID)
	allSessions, _ := a.store.GetAllSessions()
	allSessionsMap := make(map[string]*models.TrainingSession)
	for _, s := range allSessions {
		allSessionsMap[s.ID.String()] = s
	}

	atts, _ = a.store.GetStudentAttendances(studentID)
	latestEval, _ := a.store.GetLatestEvaluation(studentID)
	readiness := a.promotionSvc.EvaluateReadiness(st, atts, allSessionsMap, latestEval)

	data := struct {
		Attendance *models.Attendance
		Readiness  services.PromotionReadiness
	}{
		Attendance: att,
		Readiness:  readiness,
	}

	w.Header().Set("HX-Trigger", "attendanceUpdated")
	a.RenderPartial(w, "checkin_row.html", data)
}

func (a *AppHandler) HandleRemoveAttendance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pathParts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// Supported formats:
	// /sessions/{sessionID}/remove/{studentID}
	// /sessions/{sessionID}/attendance/{studentID}
	// /sessions/{sessionID}/attendance/{studentID}/remove
	if len(pathParts) < 4 {
		http.Error(w, "Invalid path parameters", http.StatusBadRequest)
		return
	}

	sessionID, err1 := uuid.Parse(pathParts[1])
	var studentID uuid.UUID
	var err2 error
	if pathParts[2] == "remove" {
		studentID, err2 = uuid.Parse(pathParts[3])
	} else if pathParts[2] == "attendance" {
		studentID, err2 = uuid.Parse(pathParts[3])
	} else {
		studentID, err2 = uuid.Parse(pathParts[len(pathParts)-1])
	}

	if err1 != nil || err2 != nil {
		http.Error(w, "Invalid session or student ID", http.StatusBadRequest)
		return
	}

	session, err := a.store.GetSessionByID(sessionID)
	if err != nil {
		http.Error(w, "Session not found", http.StatusNotFound)
		return
	}
	_ = session

	if err := a.store.RemoveAttendance(sessionID, studentID); err != nil {
		if err == repository.ErrNotFound {
			http.Error(w, "Student is not checked in to this session", http.StatusNotFound)
			return
		}
		http.Error(w, fmt.Sprintf("Failed to remove attendance: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("HX-Trigger", "attendanceUpdated")

	if r.URL.Query().Get("from") == "search" {
		w.Header().Set("HX-Refresh", "true")
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Header.Get("HX-Request") == "true" {
		w.WriteHeader(http.StatusOK)
		return
	}

	referer := r.Header.Get("Referer")
	if referer != "" {
		http.Redirect(w, r, referer, http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/sessions/%s/live", sessionID), http.StatusSeeOther)
}

