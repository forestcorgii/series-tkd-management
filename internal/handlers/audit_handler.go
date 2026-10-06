package handlers

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"
	"strings"

	"series-tkd-management/internal/models"
)

type AuditLogsViewData struct {
	CurrentUser *models.User
	Logs        []*models.AuditLog
	Telemetry   *models.AuditTelemetry
	Filter      models.AuditLogFilter
	CurrentPage int
	TotalPages  int
	TotalLogs   int
	HasPrev     bool
	HasNext     bool
	PrevPage    int
	NextPage    int
}

func (a *AppHandler) HandleAuditLogs(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil || !user.IsOperationManager() {
		if strings.HasPrefix(r.URL.Path, "/api/") || r.Header.Get("HX-Request") == "true" || strings.Contains(r.Header.Get("Accept"), "application/json") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusForbidden)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "forbidden",
				"message": "Activity and audit logs are exclusively restricted to Operation Managers",
			})
			return
		}
		if user == nil {
			http.Redirect(w, r, "/login?redirect=/logs", http.StatusSeeOther)
		} else {
			http.Redirect(w, r, user.RoleDashboardURL(), http.StatusSeeOther)
		}
		return
	}

	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if q == "" {
		q = strings.TrimSpace(r.URL.Query().Get("search"))
	}
	category := strings.TrimSpace(r.URL.Query().Get("category"))
	role := strings.TrimSpace(r.URL.Query().Get("role"))
	action := strings.TrimSpace(r.URL.Query().Get("action"))
	startDate := strings.TrimSpace(r.URL.Query().Get("start_date"))
	endDate := strings.TrimSpace(r.URL.Query().Get("end_date"))

	page := 1
	if p, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && p > 0 {
		page = p
	}

	limit := 25
	if l, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && l > 0 && l <= 100 {
		limit = l
	}

	offset := (page - 1) * limit

	filter := models.AuditLogFilter{
		Search:    q,
		Category:  category,
		Role:      role,
		Action:    action,
		StartDate: startDate,
		EndDate:   endDate,
		Limit:     limit,
		Offset:    offset,
	}

	logs, total, err := a.auditSvc.GetLogs(filter)
	if err != nil {
		http.Error(w, "Failed to retrieve audit logs: "+err.Error(), http.StatusInternalServerError)
		return
	}

	telemetry, err := a.auditSvc.GetTelemetry()
	if err != nil {
		telemetry = &models.AuditTelemetry{}
	}

	totalPages := int(math.Ceil(float64(total) / float64(limit)))
	if totalPages < 1 {
		totalPages = 1
	}

	viewData := AuditLogsViewData{
		CurrentUser: user,
		Logs:        logs,
		Telemetry:   telemetry,
		Filter:      filter,
		CurrentPage: page,
		TotalPages:  totalPages,
		TotalLogs:   total,
		HasPrev:     page > 1,
		HasNext:     page < totalPages,
		PrevPage:    page - 1,
		NextPage:    page + 1,
	}

	if strings.HasPrefix(r.URL.Path, "/api/") || (r.Header.Get("Accept") == "application/json" && r.Header.Get("HX-Request") != "true") {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"logs":         logs,
			"total":        total,
			"page":         page,
			"total_pages":  totalPages,
			"telemetry":    telemetry,
			"filter":       filter,
		})
		return
	}

	// Partial swapping for HTMX dynamic filter & search
	if r.Header.Get("HX-Request") == "true" && (r.Header.Get("HX-Target") == "audit-log-rows" || r.URL.Query().Get("partial") == "rows") {
		a.RenderPartial(w, "audit_log_rows.html", viewData)
		return
	}

	a.RenderPage(w, "audit_logs.html", viewData)
}

func (a *AppHandler) HandleAuditTelemetry(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil || !user.IsOperationManager() {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "forbidden",
		})
		return
	}

	telemetry, err := a.auditSvc.GetTelemetry()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(telemetry)
}
