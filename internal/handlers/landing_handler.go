package handlers

import (
	"html/template"
	"net/http"
	"sort"
	"strings"

	"series-tkd-management/internal/models"
)

// LandingProgram represents a martial arts discipline highlighted on the landing page.
type LandingProgram struct {
	ID          string
	Title       string
	KoreanTitle string
	Tagline     string
	Badge       string
	BadgeClass  string
	Description string
	FocusPoints []string
	IconSVG     template.HTML
}

// LandingScheduleItem represents a plotted class slot on the landing schedule preview.
type LandingScheduleItem struct {
	SessionID      string
	Title          string
	FormattedDate  string
	DayOfWeek      string
	FormattedTime  string
	TrainingType   string
	TypeColorClass string
	LocationID     string
	LocationName   string
	LocationPin    string
	CoachName      string
	IsClass        bool
}

// LandingPageViewData is the data context passed to landing.html.
type LandingPageViewData struct {
	CurrentUser      *models.User
	Locations        []*models.Location
	ScheduleItems    []LandingScheduleItem
	Programs         []LandingProgram
	ActiveLocations  int
	WeeklyClasses    int
	CoachesCount     int
	SelectedLocation string
	SelectedType     string
}

// LandingSchedulePartialData is the data context for landing_schedule_grid.html.
type LandingSchedulePartialData struct {
	ScheduleItems    []LandingScheduleItem
	SelectedLocation string
	SelectedType     string
	TotalCount       int
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
	coaches, _ := a.store.GetAllCoaches()
	scheduleItems := a.getLandingScheduleItems("", "")

	data := LandingPageViewData{
		CurrentUser:      user,
		Locations:        locations,
		ScheduleItems:    scheduleItems,
		Programs:         getLandingPrograms(),
		ActiveLocations:  len(locations),
		WeeklyClasses:    len(scheduleItems),
		CoachesCount:     len(coaches),
		SelectedLocation: "all",
		SelectedType:     "all",
	}

	a.RenderPage(w, "landing.html", data)
}

// HandleLandingSchedule serves the HTMX partial for filtering the schedule grid.
func (a *AppHandler) HandleLandingSchedule(w http.ResponseWriter, r *http.Request) {
	loc := strings.TrimSpace(r.URL.Query().Get("location"))
	trainType := strings.TrimSpace(r.URL.Query().Get("type"))

	items := a.getLandingScheduleItems(loc, trainType)

	data := LandingSchedulePartialData{
		ScheduleItems:    items,
		SelectedLocation: loc,
		SelectedType:     trainType,
		TotalCount:       len(items),
	}

	a.RenderPartial(w, "landing_schedule_grid.html", data)
}

// getLandingScheduleItems filters, extracts, and sorts upcoming sessions for public preview.
func (a *AppHandler) getLandingScheduleItems(locationFilter, typeFilter string) []LandingScheduleItem {
	allSessions, err := a.store.GetAllSessions()
	if err != nil {
		return nil
	}

	var items []LandingScheduleItem
	for _, s := range allSessions {
		if s.IsCancelled {
			continue
		}

		// Filter by location if specified
		if locationFilter != "" && locationFilter != "all" {
			locMatch := false
			if s.LocationID != nil && s.LocationID.String() == locationFilter {
				locMatch = true
			} else if strings.EqualFold(s.LocationName, locationFilter) {
				locMatch = true
			}
			if !locMatch {
				continue
			}
		}

		// Filter by training type if specified
		if typeFilter != "" && typeFilter != "all" {
			if !strings.EqualFold(string(s.TrainingType), typeFilter) {
				continue
			}
		}

		// Theme category badge
		typeColor := "bg-red-500/10 text-red-700 dark:text-red-400 border-red-500/30"
		switch strings.ToLower(string(s.TrainingType)) {
		case "sparring":
			typeColor = "bg-red-500/10 text-red-700 dark:text-red-400 border-red-500/30"
		case "poomsae":
			typeColor = "bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/30"
		case "conditioning":
			typeColor = "bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/30"
		case "promotion prep":
			typeColor = "bg-purple-500/10 text-purple-700 dark:text-purple-400 border-purple-500/30"
		default:
			typeColor = "bg-slate-500/10 text-slate-700 dark:text-slate-300 border-slate-500/30"
		}

		coachName := s.CoachName
		if coachName == "" {
			coachName = "Lead Instructor"
		}

		locationName := s.LocationName
		if locationName == "" {
			locationName = "Main Dojang"
		}

		locID := ""
		if s.LocationID != nil {
			locID = s.LocationID.String()
		}

		dateStr := s.SessionDate.Format("Jan 02, 2006")
		dayStr := s.SessionDate.Format("Monday")
		timeStr := s.StartTime
		if s.EndTime != "" {
			timeStr += " – " + s.EndTime
		}

		title := s.Title
		if title == "" {
			title = string(s.TrainingType) + " Session"
		}

		items = append(items, LandingScheduleItem{
			SessionID:      s.ID.String(),
			Title:          title,
			FormattedDate:  dateStr,
			DayOfWeek:      dayStr,
			FormattedTime:  timeStr,
			TrainingType:   string(s.TrainingType),
			TypeColorClass: typeColor,
			LocationID:     locID,
			LocationName:   locationName,
			LocationPin:    s.LocationPin,
			CoachName:      coachName,
			IsClass:        s.EntryTypeVal() == "class" || s.EntryTypeVal() == "",
		})
	}

	// Sort chronologically
	sort.Slice(items, func(i, j int) bool {
		if items[i].FormattedDate != items[j].FormattedDate {
			return items[i].FormattedDate < items[j].FormattedDate
		}
		return items[i].FormattedTime < items[j].FormattedTime
	})

	return items
}

func getLandingPrograms() []LandingProgram {
	return []LandingProgram{
		{
			ID:          "sparring",
			Title:       "Sparring (Kyorugi)",
			KoreanTitle: "겨루기",
			Tagline:     "Olympic Combat, Speed & Tactical Ring IQ",
			Badge:       "Olympic & Competitive",
			BadgeClass:  "bg-red-500/10 text-red-700 dark:text-red-400 border-red-500/30",
			Description: "High-octane competitive sparring training emphasizing speed, reaction timing, footwork angles, electronic sensor scoring, and ring strategy under World Taekwondo rules.",
			FocusPoints: []string{
				"Footwork angles, ring cut-offs & distance control",
				"Electronic sensor impact delivery & scoring combos",
				"High-speed counter-kicking tactical setups",
				"Cadet, Junior & Senior tournament preparation",
			},
		},
		{
			ID:          "poomsae",
			Title:       "Forms (Poomsae)",
			KoreanTitle: "품새",
			Tagline:     "Mastery of Precision, Balance & Power Delivery",
			Badge:       "Traditional & Technical",
			BadgeClass:  "bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/30",
			Description: "Comprehensive WT Kukkiwon curriculum from Taegeuk 1-Jang through advanced Black Belt Dan patterns. Honing stance stability, breath control, and explosive technical snaps.",
			FocusPoints: []string{
				"Official Kukkiwon Dan & Kup syllabus lineage",
				"Biomechanical stance depth & balance transitions",
				"Speed, acceleration, and dynamic power pauses",
				"Belt promotion & technical championship readiness",
			},
		},
		{
			ID:          "cadets",
			Title:       "Cadets & Kids Fundamentals",
			KoreanTitle: "어린이 태권도",
			Tagline:     "Confidence, Respect & Motor Agility",
			Badge:       "Ages 4–12 Youth Development",
			BadgeClass:  "bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/30",
			Description: "Dynamic, values-centered training designed for young martial artists. Cultivates dojang respect, athletic coordination, focus, anti-bullying confidence, and teamwork.",
			FocusPoints: []string{
				"Traditional bowing, etiquette & respect for elders",
				"Speed ladders, agility & motor coordination skills",
				"Anti-bullying assertiveness, voice & confidence",
				"Fun, rewarding belt milestone progression",
			},
		},
		{
			ID:          "conditioning",
			Title:       "Athletic Conditioning & Promotion Prep",
			KoreanTitle: "체력단련",
			Tagline:     "Peak Conditioning & Test Benchmark Readiness",
			Badge:       "Fitness & Promotion Prep",
			BadgeClass:  "bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-500/30",
			Description: "High-intensity conditioning aligned directly with the 6-factor ability evaluation: Flexibility, Stamina, Power, Technique, Sparring IQ, and Discipline.",
			FocusPoints: []string{
				"High-intensity interval circuits (HIIT) & core power",
				"Dynamic flexibility, hip mobility & kick range",
				"Belt test stamina simulation & benchmark drills",
				"6-Factor athletic radar metric optimization",
			},
		},
	}
}
