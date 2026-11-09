package handlers

import (
	"encoding/json"
	"fmt"
	"html"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/services"
)

// API endpoint returning JSON notifications list & unread count
func (a *AppHandler) HandleGetNotificationsAPI(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	filter := models.NotificationFilter{
		UserID: &user.ID,
		Role:   &user.Role,
		Limit:  30,
	}

	notifs, total, err := a.notifSvc.GetNotifications(filter)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	unreadCount, _ := a.notifSvc.GetUnreadCount(&user.ID, &user.Role)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"notifications": notifs,
		"total":         total,
		"unread_count":  unreadCount,
	})
}

// HTMX partial rendering the notification bell badge count
func (a *AppHandler) HandleNotificationBadge(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		w.WriteHeader(http.StatusOK)
		return
	}

	unreadCount, _ := a.notifSvc.GetUnreadCount(&user.ID, &user.Role)
	if unreadCount <= 0 {
		w.Header().Set("Content-Type", "text/html")
		w.Write([]byte(`<span id="notification-badge" class="hidden"></span>`))
		return
	}

	badgeText := fmt.Sprintf("%d", unreadCount)
	if unreadCount > 99 {
		badgeText = "99+"
	}

	html := fmt.Sprintf(`<span id="notification-badge" class="absolute -top-1 -right-1 flex h-4 min-w-[16px] px-1 items-center justify-center rounded-full bg-[#990303] text-[10px] font-bold text-white shadow-sm animate-pulse">%s</span>`, badgeText)
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// HTMX partial rendering the full interactive notification bell dropdown
func (a *AppHandler) HandleNotificationDropdown(w http.ResponseWriter, r *http.Request) {
	user := GetUserFromContext(r.Context())
	if user == nil {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	filter := models.NotificationFilter{
		UserID: &user.ID,
		Role:   &user.Role,
		Limit:  15,
	}

	notifs, _, _ := a.notifSvc.GetNotifications(filter)
	unreadCount, _ := a.notifSvc.GetUnreadCount(&user.ID, &user.Role)

	var sb strings.Builder
	sb.WriteString(`<div id="notification-dropdown-menu" class="w-80 sm:w-96 rounded-2xl bg-white dark:bg-neutral-900 border border-[#E5E3D8] dark:border-neutral-800 shadow-2xl overflow-hidden animate-fade-in text-xs">`)

	// Header
	sb.WriteString(`<div class="flex items-center justify-between p-3.5 border-b border-[#E5E3D8] dark:border-neutral-800 bg-[#FFFDF4]/80 dark:bg-neutral-900">`)
	sb.WriteString(`  <div class="flex items-center gap-2">`)
	sb.WriteString(`    <span class="font-display font-bold uppercase tracking-wider text-slate-900 dark:text-white text-sm">Notifications</span>`)
	if unreadCount > 0 {
		sb.WriteString(fmt.Sprintf(`    <span class="px-2 py-0.5 rounded-full bg-[#990303]/10 text-[#990303] dark:bg-red-950/60 dark:text-red-400 font-bold text-[10px]">%d unread</span>`, unreadCount))
	}
	sb.WriteString(`  </div>`)
	if unreadCount > 0 {
		sb.WriteString(`  <button type="button" hx-post="/api/notifications/mark-all-read" hx-target="#notification-dropdown-menu" hx-swap="innerHTML" class="text-[11px] font-semibold text-[#990303] hover:underline dark:text-red-400">Mark all read</button>`)
	}
	sb.WriteString(`</div>`)

	// Body / Items
	sb.WriteString(`<div class="max-h-80 overflow-y-auto divide-y divide-[#E5E3D8]/50 dark:divide-neutral-800/60">`)
	if len(notifs) == 0 {
		sb.WriteString(`<div class="p-8 text-center text-slate-400 dark:text-slate-500 italic">`)
		sb.WriteString(`  <span class="text-2xl block mb-1">🔕</span>`)
		sb.WriteString(`  No notifications yet. You're all caught up!`)
		sb.WriteString(`</div>`)
	} else {
		for _, n := range notifs {
			unreadBg := ""
			unreadDot := ""
			if !n.IsRead {
				unreadBg = "bg-red-50/40 dark:bg-neutral-800/40"
				unreadDot = `<span class="w-2 h-2 rounded-full bg-[#990303] flex-shrink-0 mt-1"></span>`
			}

			var channelBadges []string
			for _, ch := range n.DispatchedChannels() {
				switch ch {
				case models.ChannelTelegram:
					channelBadges = append(channelBadges, `<span class="px-1.5 py-0.5 rounded bg-sky-100 text-sky-800 dark:bg-sky-950 dark:text-sky-300 font-bold text-[9px]">TELEGRAM</span>`)
				case models.ChannelEmail:
					channelBadges = append(channelBadges, `<span class="px-1.5 py-0.5 rounded bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-300 font-bold text-[9px]">EMAIL</span>`)
				default:
					channelBadges = append(channelBadges, `<span class="px-1.5 py-0.5 rounded bg-slate-100 text-slate-700 dark:bg-neutral-800 dark:text-slate-300 font-bold text-[9px]">IN-APP</span>`)
				}
			}
			channelBadge := strings.Join(channelBadges, " ")

			sb.WriteString(fmt.Sprintf(`<div class="p-3 transition hover:bg-[#F4F1E4]/50 dark:hover:bg-neutral-800/60 flex items-start gap-2.5 %s" id="notif-item-%s">`, unreadBg, n.ID.String()))
			sb.WriteString(fmt.Sprintf(`  <div class="text-base flex-shrink-0 mt-0.5">%s</div>`, n.Icon()))
			sb.WriteString(`  <div class="flex-1 min-w-0">`)
			sb.WriteString(`    <div class="flex items-center justify-between gap-1 mb-0.5">`)
			sb.WriteString(fmt.Sprintf(`      <p class="font-bold text-slate-900 dark:text-white truncate text-xs">%s</p>`, template.HTMLEscapeString(n.Title)))
			sb.WriteString(fmt.Sprintf(`      <span class="text-[10px] text-slate-400 whitespace-nowrap">%s</span>`, n.CreatedAt.Format("03:04 PM")))
			sb.WriteString(`    </div>`)
			sb.WriteString(fmt.Sprintf(`    <p class="text-slate-600 dark:text-slate-300 text-[11px] leading-relaxed break-words">%s</p>`, template.HTMLEscapeString(n.Message)))
			sb.WriteString(`    <div class="flex items-center justify-between mt-1.5">`)
			sb.WriteString(fmt.Sprintf(`      <div class="flex items-center gap-1.5">%s</div>`, channelBadge))
			if !n.IsRead {
				sb.WriteString(fmt.Sprintf(`      <button type="button" hx-post="/api/notifications/%s/read" hx-target="#notif-item-%s" hx-swap="outerHTML" class="text-[10px] text-slate-400 hover:text-slate-700 dark:hover:text-slate-200">Dismiss</button>`, n.ID.String(), n.ID.String()))
			}
			sb.WriteString(`    </div>`)
			sb.WriteString(`  </div>`)
			if unreadDot != "" {
				sb.WriteString(fmt.Sprintf(`  %s`, unreadDot))
			}
			sb.WriteString(`</div>`)
		}
	}
	sb.WriteString(`</div>`)

	// Footer
	sb.WriteString(`<div class="p-2 border-t border-[#E5E3D8] dark:border-neutral-800 bg-[#FFFDF4]/40 dark:bg-neutral-900/40 text-center">`)
	sb.WriteString(`  <span class="text-[10px] text-slate-400">Telegram &amp; Email alerts configured</span>`)
	sb.WriteString(`</div>`)
	sb.WriteString(`</div>`)

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(sb.String()))
}

// Mark single notification as read
func (a *AppHandler) HandleMarkNotificationRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.PathValue("id")
	if idStr == "" {
		idStr = strings.TrimPrefix(r.URL.Path, "/api/notifications/")
		idStr = strings.TrimSuffix(idStr, "/read")
	}

	notifID, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid notification ID", http.StatusBadRequest)
		return
	}

	_ = a.notifSvc.MarkAsRead(notifID)

	w.Header().Set("HX-Trigger", "notificationUpdated")
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(""))
}

// Mark all notifications read
func (a *AppHandler) HandleMarkAllNotificationsRead(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := GetUserFromContext(r.Context())
	if user != nil {
		_ = a.notifSvc.MarkAllAsRead(&user.ID, &user.Role)
	}

	w.Header().Set("HX-Trigger", "notificationUpdated")
	// Re-render the dropdown content cleanly
	a.HandleNotificationDropdown(w, r)
}

// HandleTestTelegram tests Telegram Bot connectivity and sends an immediate test ping
func (a *AppHandler) HandleTestTelegram(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := GetUserFromContext(r.Context())
	if user != nil && !user.IsOperationManager() {
		http.Error(w, "Forbidden: Only Operation Managers can test telegram settings", http.StatusForbidden)
		return
	}

	botToken := strings.TrimSpace(r.FormValue("telegram_bot_token"))
	targetChat := strings.TrimSpace(r.FormValue("telegram_public_chat_id"))
	if targetChat == "" {
		targetChat = strings.TrimSpace(r.FormValue("telegram_staff_chat_id"))
	}

	if botToken == "" {
		settings, _ := a.notifSvc.GetSettings()
		if settings != nil {
			botToken = settings.TelegramBotToken
			if targetChat == "" {
				targetChat = settings.TelegramPublicChatID
				if targetChat == "" {
					targetChat = settings.TelegramStaffChatID
				}
			}
		}
	}

	botUsername, err := a.notifSvc.SendTestTelegram(botToken, targetChat)
	w.Header().Set("Content-Type", "text/html")
	if err != nil {
		w.Write([]byte(fmt.Sprintf(`<div class="p-2.5 rounded-lg bg-rose-50 dark:bg-rose-950/40 border border-rose-200 dark:border-rose-900 text-rose-700 dark:text-rose-300 text-xs font-semibold flex items-center gap-2"><span>✗</span> <span>Connection failed: %s</span></div>`, html.EscapeString(err.Error()))))
		return
	}

	targetNotice := ""
	if targetChat != "" {
		targetNotice = fmt.Sprintf(" and test message delivered to %s", html.EscapeString(targetChat))
	}
	w.Write([]byte(fmt.Sprintf(`<div class="p-2.5 rounded-lg bg-emerald-50 dark:bg-emerald-950/40 border border-emerald-200 dark:border-emerald-900 text-emerald-700 dark:text-emerald-300 text-xs font-semibold flex items-center gap-2"><span>✓</span> <span>Connected to bot <b>@%s</b>%s!</span></div>`, html.EscapeString(botUsername), targetNotice)))
}

// HandleEmailTest tests SMTP email delivery settings with an immediate test message
func (a *AppHandler) HandleEmailTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	user := GetUserFromContext(r.Context())
	if user == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
		return
	}

	var req struct {
		Host      string `json:"smtp_host"`
		Port      int    `json:"smtp_port"`
		User      string `json:"smtp_user"`
		Password  string `json:"smtp_password"`
		From      string `json:"smtp_from"`
		Recipient string `json:"recipient_email"`
	}

	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": "Invalid JSON payload: " + err.Error()})
			return
		}
	} else {
		_ = r.ParseForm()
		req.Host = r.FormValue("smtp_host")
		if portStr := r.FormValue("smtp_port"); portStr != "" {
			req.Port, _ = strconv.Atoi(portStr)
		}
		req.User = r.FormValue("smtp_user")
		req.Password = r.FormValue("smtp_password")
		req.From = r.FormValue("smtp_from")
		req.Recipient = r.FormValue("recipient_email")
	}

	if strings.TrimSpace(req.Recipient) == "" {
		req.Recipient = user.Email
	}

	params := services.TestEmailParams{
		Host:      req.Host,
		Port:      req.Port,
		User:      req.User,
		Password:  req.Password,
		From:      req.From,
		Recipient: req.Recipient,
	}

	err := a.notifSvc.SendTestEmail(params)
	if err != nil {
		if strings.Contains(r.Header.Get("Accept"), "text/html") && !strings.Contains(contentType, "application/json") {
			http.Redirect(w, r, "/settings?tab=notifications&error="+url.QueryEscape("Test email failed: "+err.Error()), http.StatusSeeOther)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	a.LogAction(r, "NOTIFICATIONS_TEST_EMAIL", models.AuditCategorySettings, "Settings", "", "Test Email", fmt.Sprintf("Sent test email to %s", req.Recipient))

	if strings.Contains(r.Header.Get("Accept"), "text/html") && !strings.Contains(contentType, "application/json") {
		http.Redirect(w, r, "/settings?tab=notifications&success="+url.QueryEscape(fmt.Sprintf("Test email dispatched successfully to %s", req.Recipient)), http.StatusSeeOther)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":    "ok",
		"message":   fmt.Sprintf("Test email sent successfully to %s", req.Recipient),
		"recipient": req.Recipient,
	})
}

