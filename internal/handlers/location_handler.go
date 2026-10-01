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
	"series-tkd-management/internal/repository"
)

type LocationListItem struct {
	Location        *models.Location
	SessionsCount   int
	AttendanceCount int
}

type LocationsPageData struct {
	Locations       []LocationListItem
	TotalLocations  int
	TotalSessions   int
	TotalAttendance int
	CurrentUser     *models.User
	SuccessNotice   string
	ErrorMessage    string
}

func (a *AppHandler) HandleLocations(w http.ResponseWriter, r *http.Request) {
	locations, err := a.store.GetAllLocations()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	sessions, _ := a.store.GetAllSessions()
	// Map session ID to location ID
	sessionLocationMap := make(map[uuid.UUID]uuid.UUID)
	sessionCounts := make(map[uuid.UUID]int)
	for _, s := range sessions {
		if s.LocationID != nil {
			sessionLocationMap[s.ID] = *s.LocationID
			sessionCounts[*s.LocationID]++
		}
	}

	attendanceCounts := make(map[uuid.UUID]int)
	totalAtt := 0
	for _, s := range sessions {
		atts, _ := a.store.GetSessionAttendances(s.ID)
		totalAtt += len(atts)
		for _, att := range atts {
			if att.LocationID != nil {
				attendanceCounts[*att.LocationID]++
			} else if locID, ok := sessionLocationMap[s.ID]; ok {
				attendanceCounts[locID]++
			}
		}
	}

	items := make([]LocationListItem, 0, len(locations))
	for _, loc := range locations {
		items = append(items, LocationListItem{
			Location:        loc,
			SessionsCount:   sessionCounts[loc.ID],
			AttendanceCount: attendanceCounts[loc.ID],
		})
	}

	user := GetUserFromContext(r.Context())
	data := LocationsPageData{
		Locations:       items,
		TotalLocations:  len(locations),
		TotalSessions:   len(sessions),
		TotalAttendance: totalAtt,
		CurrentUser:     user,
		SuccessNotice:   r.URL.Query().Get("success"),
		ErrorMessage:    r.URL.Query().Get("error"),
	}

	a.RenderPage(w, "locations.html", data)
}

func (a *AppHandler) HandleCreateLocation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	pin := strings.TrimSpace(r.FormValue("pin"))

	if name == "" {
		http.Redirect(w, r, "/locations?error="+url.QueryEscape("Location name is required"), http.StatusSeeOther)
		return
	}

	var fixedRate *float64
	rateStr := strings.TrimSpace(r.FormValue("fixed_rate"))
	if rateStr == "" {
		rateStr = strings.TrimSpace(r.FormValue("fix_rate"))
	}
	if rateStr != "" {
		if val, err := strconv.ParseFloat(rateStr, 64); err == nil && val >= 0 {
			fixedRate = &val
		}
	}

	loc := &models.Location{
		ID:        uuid.New(),
		Name:      name,
		Pin:       pin,
		FixedRate: fixedRate,
		CreatedAt: time.Now(),
	}

	if err := a.store.CreateLocation(loc); err != nil {
		http.Redirect(w, r, "/locations?error="+url.QueryEscape(fmt.Sprintf("Failed to create location: %v", err)), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/locations?success="+url.QueryEscape(fmt.Sprintf("Location \"%s\" added successfully", loc.Name)), http.StatusSeeOther)
}

func (a *AppHandler) HandleUpdateLocation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		path := strings.TrimPrefix(r.URL.Path, "/locations/")
		path = strings.TrimSuffix(path, "/edit")
		path = strings.TrimSuffix(path, "/delete")
		idStr = path
	}
	locID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid location ID", http.StatusBadRequest)
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	pin := strings.TrimSpace(r.FormValue("pin"))

	if name == "" {
		http.Redirect(w, r, "/locations?error="+url.QueryEscape("Location name is required"), http.StatusSeeOther)
		return
	}

	var fixedRate *float64
	rateStr := strings.TrimSpace(r.FormValue("fixed_rate"))
	if rateStr == "" {
		rateStr = strings.TrimSpace(r.FormValue("fix_rate"))
	}
	if rateStr != "" {
		if val, err := strconv.ParseFloat(rateStr, 64); err == nil && val >= 0 {
			fixedRate = &val
		}
	}

	loc := &models.Location{
		ID:        locID,
		Name:      name,
		Pin:       pin,
		FixedRate: fixedRate,
	}

	if err := a.store.UpdateLocation(loc); err != nil {
		if err == repository.ErrNotFound {
			http.Error(w, "Location not found", http.StatusNotFound)
			return
		}
		http.Redirect(w, r, "/locations?error="+url.QueryEscape(fmt.Sprintf("Failed to update location: %v", err)), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/locations?success="+url.QueryEscape(fmt.Sprintf("Location \"%s\" updated successfully", loc.Name)), http.StatusSeeOther)
}

func (a *AppHandler) HandleDeleteLocation(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		path := strings.TrimPrefix(r.URL.Path, "/locations/")
		path = strings.TrimSuffix(path, "/delete")
		path = strings.TrimSuffix(path, "/edit")
		idStr = path
	}
	locID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid location ID", http.StatusBadRequest)
		return
	}

	if err := a.store.DeleteLocation(locID); err != nil {
		if err == repository.ErrNotFound {
			http.Error(w, "Location not found", http.StatusNotFound)
			return
		}
		http.Redirect(w, r, "/locations?error="+url.QueryEscape(fmt.Sprintf("Failed to delete location: %v", err)), http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/locations?success="+url.QueryEscape("Location deleted successfully"), http.StatusSeeOther)
}
