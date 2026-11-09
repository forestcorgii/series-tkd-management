package services

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"net"
	"net/smtp"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

type RecipientInfo struct {
	UserID *uuid.UUID
	Role   models.UserRole
	Type   string // "COACH", "ADMIN", "STUDENT", "GUARDIAN", "OPERATION_MANAGER"
	Name   string
	Email  string
	Phone  string
}

type NotificationService struct {
	store        repository.RepositoryStore
	dedupMu      sync.Mutex
	recentEvents map[string]time.Time
}

func NewNotificationService(store repository.RepositoryStore) *NotificationService {
	return &NotificationService{
		store:        store,
		recentEvents: make(map[string]time.Time),
	}
}

// Settings management
func (s *NotificationService) GetSettings() (*models.NotificationSettings, error) {
	return s.store.GetNotificationSettings()
}

func (s *NotificationService) UpdateSettings(settings *models.NotificationSettings) error {
	if settings == nil {
		return errors.New("notification settings cannot be nil")
	}
	return s.store.UpdateNotificationSettings(settings)
}

// SendTestTelegram tests the connection to Telegram API and verifies the bot username
func (s *NotificationService) SendTestTelegram(botToken, chatID string) (string, error) {
	return TestTelegramConnection(botToken, chatID)
}

// User preferences management
func (s *NotificationService) GetUserPreferences(userID uuid.UUID) (*models.UserNotificationPreferences, error) {
	return s.store.GetUserNotificationPreferences(userID)
}

func (s *NotificationService) UpdateUserPreferences(prefs *models.UserNotificationPreferences) error {
	if prefs == nil {
		return errors.New("user notification preferences cannot be nil")
	}
	return s.store.UpdateUserNotificationPreferences(prefs)
}

// Query & read operations
func (s *NotificationService) GetNotifications(filter models.NotificationFilter) ([]*models.Notification, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	} else if filter.Limit > 100 {
		filter.Limit = 100
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	return s.store.GetNotifications(filter)
}

func (s *NotificationService) GetUnreadCount(userID *uuid.UUID, role *models.UserRole) (int, error) {
	return s.store.GetUnreadNotificationCount(userID, role)
}

func (s *NotificationService) MarkAsRead(id uuid.UUID) error {
	return s.store.MarkNotificationRead(id)
}

func (s *NotificationService) MarkAllAsRead(userID *uuid.UUID, role *models.UserRole) error {
	return s.store.MarkAllNotificationsRead(userID, role)
}

// deduplicateRecipients ensures no recipient appears twice (by UserID or by email/phone)
func deduplicateRecipients(recipients []RecipientInfo) []RecipientInfo {
	seenUsers := make(map[uuid.UUID]bool)
	seenEmails := make(map[string]bool)
	seenPhones := make(map[string]bool)
	var deduped []RecipientInfo

	for _, r := range recipients {
		if r.UserID != nil && *r.UserID != uuid.Nil {
			if seenUsers[*r.UserID] {
				continue
			}
			seenUsers[*r.UserID] = true
		} else if strings.TrimSpace(r.Email) != "" {
			emailKey := strings.ToLower(strings.TrimSpace(r.Email))
			if seenEmails[emailKey] {
				continue
			}
			seenEmails[emailKey] = true
		} else if strings.TrimSpace(r.Phone) != "" {
			phoneKey := strings.TrimSpace(r.Phone)
			if seenPhones[phoneKey] {
				continue
			}
			seenPhones[phoneKey] = true
		}
		deduped = append(deduped, r)
	}
	return deduped
}

// Internal multi-channel dispatcher - creates EXACTLY ONE notification record per recipient
func (s *NotificationService) dispatch(event models.NotificationEventType, recipients []RecipientInfo, title, message, metadata string) {
	settings, err := s.store.GetNotificationSettings()
	if err != nil || settings == nil {
		settings = models.DefaultNotificationSettings()
	}

	if !settings.IsEventEnabled(event) {
		return
	}

	recipients = deduplicateRecipients(recipients)
	now := time.Now()

	// Purge stale debounce keys older than 60s
	s.dedupMu.Lock()
	for k, t := range s.recentEvents {
		if now.Sub(t) > 60*time.Second {
			delete(s.recentEvents, k)
		}
	}
	s.dedupMu.Unlock()

	for _, r := range recipients {
		// In-flight debounce check: prevent duplicate notifications for same recipient and event within 5s
		userKey := "none"
		if r.UserID != nil {
			userKey = r.UserID.String()
		}
		dedupKey := fmt.Sprintf("%s:%s:%s:%s:%s", event, userKey, r.Type, r.Email, title)

		s.dedupMu.Lock()
		if lastTime, exists := s.recentEvents[dedupKey]; exists && now.Sub(lastTime) < 5*time.Second {
			s.dedupMu.Unlock()
			continue
		}
		s.recentEvents[dedupKey] = now
		s.dedupMu.Unlock()

		var userPrefs *models.UserNotificationPreferences
		if r.UserID != nil && *r.UserID != uuid.Nil {
			prefs, err := s.store.GetUserNotificationPreferences(*r.UserID)
			if err == nil && prefs != nil {
				userPrefs = prefs
			}
		}
		if userPrefs == nil && r.UserID != nil {
			userPrefs = models.DefaultUserPreferences(*r.UserID)
		}

		// Calculate active outward delivery channels
		var dispatchedChannels []models.NotificationChannel
		dispatchedChannels = append(dispatchedChannels, models.ChannelInApp)

		if settings.IsChannelEnabled(models.ChannelEmail) && (userPrefs == nil || userPrefs.ShouldReceive(models.ChannelEmail)) && strings.TrimSpace(r.Email) != "" {
			dispatchedChannels = append(dispatchedChannels, models.ChannelEmail)
			log.Printf("✉️ [EMAIL NOTIFICATION] To: %s <%s> | %s: %s", r.Name, r.Email, title, message)
			go s.sendEmailNotification(r, title, message, event, metadata, settings)
		}

		if settings.IsChannelEnabled(models.ChannelTelegram) && settings.HasTelegramConfig() {
			dispatchedChannels = append(dispatchedChannels, models.ChannelTelegram)
		}

		// Attach channels list into metadata JSON so UI can display multi-channel badges without separate DB rows
		channelsJSON, _ := json.Marshal(dispatchedChannels)
		metaWithChannels := metadata
		if metaWithChannels == "" || metaWithChannels == "{}" {
			metaWithChannels = fmt.Sprintf(`{"channels":%s}`, string(channelsJSON))
		} else if strings.HasPrefix(metaWithChannels, "{") && strings.HasSuffix(metaWithChannels, "}") {
			metaWithChannels = fmt.Sprintf(`{"channels":%s,%s`, string(channelsJSON), metaWithChannels[1:])
		}

		// All system notification records are stored under ChannelInApp
		primaryChan := models.ChannelInApp

		// Create EXACTLY ONE Notification entry in the repository for this recipient!
		notif := &models.Notification{
			ID:               uuid.New(),
			UserID:           r.UserID,
			RecipientRole:    r.Role,
			RecipientType:    r.Type,
			RecipientName:    r.Name,
			RecipientContact: r.Email,
			Channel:          primaryChan,
			EventType:        event,
			Title:            title,
			Message:          message,
			Metadata:         metaWithChannels,
			Status:           models.StatusSent,
			IsRead:           false,
			CreatedAt:        now,
		}
		_ = s.store.CreateNotification(notif)
	}

	// Dispatch outward broadcast to Telegram Bot (curated by branch location or global)
	if settings.IsChannelEnabled(models.ChannelTelegram) && settings.HasTelegramConfig() {
		go s.sendTelegramNotification(event, title, message, metadata, settings)
	}
}

// -------------------------------------------------------------
// High-Level Domain Event Notifications
// -------------------------------------------------------------

// 1. Notify coach and admin: when student self admit or was admitted
func (s *NotificationService) NotifyStudentAdmitted(sess *models.TrainingSession, student *models.Student, wasSelfAdmit bool, actorName string) {
	if sess == nil || student == nil {
		return
	}

	title := fmt.Sprintf("🥋 Student Admitted: %s", student.FullName)
	var message string
	if wasSelfAdmit {
		message = fmt.Sprintf("%s self checked-in for %s class (%s).", student.FullName, sess.DisplayTitle(), sess.SessionDate.Format("Jan 02"))
	} else {
		message = fmt.Sprintf("%s was admitted to %s class (%s) by %s.", student.FullName, sess.DisplayTitle(), sess.SessionDate.Format("Jan 02"), actorName)
	}

	var recipients []RecipientInfo
	if wasSelfAdmit {
		recipients = s.getClassStaffRecipients(sess)
	} else {
		recipients = s.getCoachAndAdminRecipients(sess.CoachID)
	}
	metadata := fmt.Sprintf(`{"student_id":"%s","session_id":"%s","was_self_admit":%t,"training_type":"%s","session_date":"%s"}`,
		student.ID, sess.ID, wasSelfAdmit, sess.DisplayTitle(), sess.SessionDate.Format("2006-01-02"))
	s.dispatch(models.EventStudentAdmitted, recipients, title, message, metadata)
}

// 2. Notify coach, admin, and managers: when new class has been opened
func (s *NotificationService) NotifyNewClassOpened(sess *models.TrainingSession, creatorName string) {
	if sess == nil {
		return
	}

	coachStatus := "Unassigned / Open"
	if strings.TrimSpace(sess.CoachName) != "" {
		coachStatus = sess.CoachName
	} else if sess.CoachID != nil && *sess.CoachID != uuid.Nil {
		if c, err := s.store.GetCoachByID(*sess.CoachID); err == nil && c != nil {
			coachStatus = c.FullName
		}
	}

	adminStatus := "Unassigned"
	if strings.TrimSpace(sess.AdminName) != "" {
		adminStatus = sess.AdminName
	} else if sess.AdminID != nil && *sess.AdminID != uuid.Nil {
		if a, err := s.store.GetUserByID(*sess.AdminID); err == nil && a != nil {
			if a.DisplayName != "" {
				adminStatus = a.DisplayName
			} else {
				adminStatus = a.Email
			}
		}
	}

	title := fmt.Sprintf("📅 New Class Opened: %s", sess.DisplayTitle())
	message := fmt.Sprintf("A new %s class was scheduled for %s (%s). Coach: %s. Admin: %s. Scheduled by %s.",
		sess.DisplayTitle(), sess.SessionDate.Format("Monday, Jan 02"), sess.TimeRange(), coachStatus, adminStatus, creatorName)

	recipients := s.getClassStaffRecipients(sess)
	metadata := fmt.Sprintf(`{"session_id":"%s","training_type":"%s","session_date":"%s","time_range":"%s","coach":"%s","admin":"%s"}`,
		sess.ID, sess.DisplayTitle(), sess.SessionDate.Format("2006-01-02"), sess.TimeRange(), coachStatus, adminStatus)
	s.dispatch(models.EventNewClassOpened, recipients, title, message, metadata)
}

// NotifySessionSlotReserved notifies coach, admin, and managers when a guest or student self-reserves a slot
func (s *NotificationService) NotifySessionSlotReserved(sess *models.TrainingSession, attendeeName string, isGuest bool, contactInfo string) {
	if sess == nil {
		return
	}

	attendeeType := "Student"
	icon := "🥋"
	if isGuest {
		attendeeType = "Guest"
		icon = "🎟️"
	}

	title := fmt.Sprintf("%s New %s Reservation: %s", icon, attendeeType, attendeeName)
	contactDetail := ""
	if strings.TrimSpace(contactInfo) != "" {
		contactDetail = fmt.Sprintf(" (Contact: %s)", contactInfo)
	}
	message := fmt.Sprintf("%s%s reserved a spot for %s on %s (%s).",
		attendeeName, contactDetail, sess.DisplayTitle(), sess.SessionDate.Format("Monday, Jan 02"), sess.TimeRange())

	recipients := s.getClassStaffRecipients(sess)
	metadata := fmt.Sprintf(`{"session_id":"%s","training_type":"%s","attendee_name":"%s","attendee_type":"%s","is_guest":%t,"contact_info":"%s","session_date":"%s","time_range":"%s"}`,
		sess.ID, sess.DisplayTitle(), attendeeName, attendeeType, isGuest, contactInfo, sess.SessionDate.Format("2006-01-02"), sess.TimeRange())
	s.dispatch(models.EventSlotReserved, recipients, title, message, metadata)
}

// 3. Notify manager: when staff (coach/admin) registers pending approval
func (s *NotificationService) NotifyStaffRegistered(user *models.User) {
	if user == nil {
		return
	}

	title := fmt.Sprintf("👤 New %s Registration: %s", user.Role, user.DisplayName)
	message := fmt.Sprintf("%s (%s, %s) submitted registration and is awaiting manager verification.", user.DisplayName, user.Role, user.Email)

	recipients := s.getManagerRecipients()
	metadata := fmt.Sprintf(`{"user_id":"%s","role":"%s"}`, user.ID, user.Role)
	s.dispatch(models.EventStaffRegistered, recipients, title, message, metadata)
}

// NotifyStudentRegistered notifies staff when a new student is enrolled
func (s *NotificationService) NotifyStudentRegistered(student *models.Student, creatorName string, pkgTitle string) {
	if student == nil {
		return
	}

	title := fmt.Sprintf("👋 New Student Enrolled: %s", student.FullName)
	pkgDetail := "No package assigned"
	if strings.TrimSpace(pkgTitle) != "" {
		pkgDetail = pkgTitle
	}

	message := fmt.Sprintf("%s (%s Belt, Age %d) was enrolled by %s with %s.",
		student.FullName, string(student.CurrentBelt), student.Age(), creatorName, pkgDetail)

	recipients := s.getManagerRecipients()
	recipients = append(recipients, s.getCoachAndAdminRecipients(nil)...)

	metadata := fmt.Sprintf(`{"student_id":"%s","name":"%s","belt":"%s","age":%d,"package":"%s","emergency_name":"%s","emergency_phone":"%s","emergency_relation":"%s"}`,
		student.ID, student.FullName, string(student.CurrentBelt), student.Age(), pkgDetail,
		student.EmergencyName, student.EmergencyPhone, student.EmergencyRelation)

	s.dispatch(models.EventStudentRegistered, recipients, title, message, metadata)
}

// 4. Notify students / guardian: when eligible for promotions
func (s *NotificationService) NotifyPromotionEligible(student *models.Student) {
	if student == nil {
		return
	}

	title := "🟢 Belt Promotion Test Eligible!"
	message := fmt.Sprintf("Congratulations %s! You have satisfied all attendance and ability benchmarks and are now eligible for %s promotion testing.",
		student.FullName, student.NextBelt())

	recipients := s.getStudentAndGuardianRecipients(student)
	// Also alert floor coaches & managers
	recipients = append(recipients, s.getCoachAndAdminRecipients(nil)...)

	metadata := fmt.Sprintf(`{"student_id":"%s","next_belt":"%s"}`, student.ID, student.NextBelt())
	s.dispatch(models.EventPromotionEligible, recipients, title, message, metadata)
}

// 5. Notify students / guardian and coaches: when student got injured
func (s *NotificationService) NotifyStudentInjured(incident *models.SafetyIncident, student *models.Student, coachName string) {
	if incident == nil || student == nil {
		return
	}

	// 1. Notify Student & Guardian
	studentTitle := fmt.Sprintf("🩹 Health & Safety Notice: %s", student.FullName)
	studentMsg := fmt.Sprintf("A safety incident (%s) was logged on the dojang floor today. Please consult your physician or coach before next session.", incident.IncidentType)
	studentRecipients := s.getStudentAndGuardianRecipients(student)
	metadata := fmt.Sprintf(`{"incident_id":"%s","student_id":"%s"}`, incident.ID, student.ID)
	s.dispatch(models.EventStudentInjured, studentRecipients, studentTitle, studentMsg, metadata)

	// 2. Notify Coaches and Admins
	staffTitle := fmt.Sprintf("🩹 Medical Hold Logged: %s", student.FullName)
	staffMsg := fmt.Sprintf("Incident: %s for %s (reported by Coach %s). Emergency contact: %s (%s). Floor safety hold is active.",
		incident.IncidentType, student.FullName, coachName, student.EmergencyName, student.EmergencyPhone)
	staffRecipients := s.getCoachAndAdminRecipients(incident.CoachID)
	s.dispatch(models.EventStudentInjured, staffRecipients, staffTitle, staffMsg, metadata)
}

// 6. Notify students / guardian: when newly evaluated
func (s *NotificationService) NotifyNewlyEvaluated(eval *models.StudentEvaluation, student *models.Student, coachName string) {
	if eval == nil || student == nil {
		return
	}

	title := "📊 Athletic Ability Evaluation Recorded"
	message := fmt.Sprintf("Coach %s submitted a 6-pillar athletic ability assessment for %s (Average Score: %.1f/10). View your updated radar chart in Practitioner Portal.",
		coachName, student.FullName, eval.AverageScore())

	recipients := s.getStudentAndGuardianRecipients(student)
	metadata := fmt.Sprintf(`{"evaluation_id":"%s","student_id":"%s"}`, eval.ID, student.ID)
	s.dispatch(models.EventNewEvaluation, recipients, title, message, metadata)
}

// 7. Notify students / guardian: when membership was awarded / revoked
func (s *NotificationService) NotifyMembershipAwarded(pkg *models.StudentPackage, student *models.Student, tplTitle string) {
	if pkg == nil || student == nil {
		return
	}

	title := "💳 Membership Pass Activated"
	message := fmt.Sprintf("A new %s package has been added to %s's account. Valid until %s.",
		tplTitle, student.FullName, pkg.ExpiryDate.Format("Jan 02, 2006"))

	recipients := s.getStudentAndGuardianRecipients(student)
	metadata := fmt.Sprintf(`{"package_id":"%s","student_id":"%s"}`, pkg.ID, student.ID)
	s.dispatch(models.EventMembershipAwarded, recipients, title, message, metadata)
}

func (s *NotificationService) NotifyMembershipRevoked(pkg *models.StudentPackage, student *models.Student) {
	if pkg == nil || student == nil {
		return
	}

	title := "⚠️ Membership Pass Revoked"
	message := fmt.Sprintf("Membership pass #%s for %s was revoked by management. Remaining session credits are no longer active.",
		pkg.ID.String()[:8], student.FullName)

	recipients := s.getStudentAndGuardianRecipients(student)
	metadata := fmt.Sprintf(`{"package_id":"%s","student_id":"%s"}`, pkg.ID, student.ID)
	s.dispatch(models.EventMembershipRevoked, recipients, title, message, metadata)
}

// 8. Others & Etc: Safety resolved, class cancelled, pass expiring
func (s *NotificationService) NotifySafetyResolved(incident *models.SafetyIncident, student *models.Student, clearedBy string) {
	if incident == nil || student == nil {
		return
	}

	title := fmt.Sprintf("🛡️ Medical Clearance: %s", student.FullName)
	message := fmt.Sprintf("Safety incident (%s) for %s was cleared by %s. Medical hold lifted; cleared for active mat training.",
		incident.IncidentType, student.FullName, clearedBy)

	recipients := s.getStudentAndGuardianRecipients(student)
	recipients = append(recipients, s.getCoachAndAdminRecipients(incident.CoachID)...)

	metadata := fmt.Sprintf(`{"incident_id":"%s","student_id":"%s"}`, incident.ID, student.ID)
	s.dispatch(models.EventSafetyResolved, recipients, title, message, metadata)
}

func (s *NotificationService) NotifyClassCancelled(sess *models.TrainingSession, reason string) {
	if sess == nil {
		return
	}

	title := fmt.Sprintf("🛑 Class Cancelled: %s", sess.TrainingType)
	message := fmt.Sprintf("The %s session scheduled for %s has been cancelled. Reason: %s. Any deducted pass credits have been refunded.",
		sess.TrainingType, sess.SessionDate.Format("Monday, Jan 02"), reason)

	// Notify assigned coach and floor admins
	recipients := s.getCoachAndAdminRecipients(sess.CoachID)

	// Also notify enrolled students if attendance exists
	if attendances, err := s.store.GetSessionAttendances(sess.ID); err == nil {
		for _, att := range attendances {
			if st, err := s.store.GetStudentByID(att.StudentID); err == nil && st != nil {
				recipients = append(recipients, s.getStudentAndGuardianRecipients(st)...)
			}
		}
	}

	metadata := fmt.Sprintf(`{"session_id":"%s","reason":"%s"}`, sess.ID, reason)
	s.dispatch(models.EventClassCancelled, recipients, title, message, metadata)
}

func (s *NotificationService) NotifyPassExpiring(student *models.Student, pkg *models.StudentPackage, reason string) {
	if student == nil || pkg == nil {
		return
	}

	title := fmt.Sprintf("⏳ Membership Pass Notice: %s", student.FullName)
	message := fmt.Sprintf("Your membership pass is expiring soon (%s). Please visit the front desk to renew your pass credits.", reason)

	recipients := s.getStudentAndGuardianRecipients(student)
	metadata := fmt.Sprintf(`{"package_id":"%s","student_id":"%s"}`, pkg.ID, student.ID)
	s.dispatch(models.EventPassExpiring, recipients, title, message, metadata)
}

// -------------------------------------------------------------
// Recipient Resolution Helpers
// -------------------------------------------------------------

// getClassStaffRecipients resolves staff recipients for class creation and reservation events according to:
// 1. Operation Managers always receive all notifications.
// 2. Coach: If specified on the session, notify that coach; if unassigned, notify all active coaches.
// 3. Admin: If specified on the session, notify that admin; if unassigned, notify all active admins.
// 4. If neither coach nor admin is specified, all active coaches, admins, and managers are notified.
func (s *NotificationService) getClassStaffRecipients(sess *models.TrainingSession) []RecipientInfo {
	var list []RecipientInfo
	if sess == nil {
		return list
	}

	// 1. Operation Managers always receive all notifications
	if managers, err := s.store.GetUsersByRole(models.RoleOperationManager); err == nil {
		for _, u := range managers {
			if !u.IsActive {
				continue
			}
			list = append(list, RecipientInfo{
				UserID: &u.ID,
				Role:   models.RoleOperationManager,
				Type:   "OPERATION_MANAGER",
				Name:   u.DisplayName,
				Email:  u.Email,
			})
		}
	}

	// 2. Coach resolution: specific coach if assigned, otherwise all active coaches
	if sess.CoachID != nil && *sess.CoachID != uuid.Nil {
		if coach, err := s.store.GetCoachByID(*sess.CoachID); err == nil && coach != nil {
			var coachUserID *uuid.UUID
			if users, err := s.store.GetUsersByRole(models.RoleCoach); err == nil {
				for _, u := range users {
					if u.CoachID != nil && *u.CoachID == coach.ID {
						coachUserID = &u.ID
						break
					}
				}
			}
			list = append(list, RecipientInfo{
				UserID: coachUserID,
				Role:   models.RoleCoach,
				Type:   "COACH",
				Name:   coach.FullName,
				Email:  coach.Email,
				Phone:  coach.Phone,
			})
		}
	} else {
		// Coach unassigned -> notify all active coaches
		if coaches, err := s.store.GetAllCoaches(); err == nil {
			coachUserMap := make(map[uuid.UUID]uuid.UUID)
			if coachUsers, err := s.store.GetUsersByRole(models.RoleCoach); err == nil {
				for _, u := range coachUsers {
					if u.CoachID != nil && *u.CoachID != uuid.Nil {
						coachUserMap[*u.CoachID] = u.ID
					}
				}
			}
			for _, coach := range coaches {
				if !coach.IsActive {
					continue
				}
				var uID *uuid.UUID
				if id, ok := coachUserMap[coach.ID]; ok {
					uID = &id
				}
				list = append(list, RecipientInfo{
					UserID: uID,
					Role:   models.RoleCoach,
					Type:   "COACH",
					Name:   coach.FullName,
					Email:  coach.Email,
					Phone:  coach.Phone,
				})
			}
		}
		// Also include any coach users from users table who may not be in coaches table
		if coachUsers, err := s.store.GetUsersByRole(models.RoleCoach); err == nil {
			for _, u := range coachUsers {
				if !u.IsActive {
					continue
				}
				list = append(list, RecipientInfo{
					UserID: &u.ID,
					Role:   models.RoleCoach,
					Type:   "COACH",
					Name:   u.DisplayName,
					Email:  u.Email,
				})
			}
		}
	}

	// 3. Admin resolution: specific admin if assigned, otherwise all active admins
	if sess.AdminID != nil && *sess.AdminID != uuid.Nil {
		if adminUser, err := s.store.GetUserByID(*sess.AdminID); err == nil && adminUser != nil {
			list = append(list, RecipientInfo{
				UserID: &adminUser.ID,
				Role:   adminUser.Role,
				Type:   "ADMIN",
				Name:   adminUser.DisplayName,
				Email:  adminUser.Email,
			})
		}
	} else {
		// Admin unassigned -> notify all active admins
		if admins, err := s.store.GetUsersByRole(models.RoleAdmin); err == nil {
			for _, u := range admins {
				if !u.IsActive {
					continue
				}
				list = append(list, RecipientInfo{
					UserID: &u.ID,
					Role:   models.RoleAdmin,
					Type:   "ADMIN",
					Name:   u.DisplayName,
					Email:  u.Email,
				})
			}
		}
	}

	return deduplicateRecipients(list)
}

func (s *NotificationService) getCoachAndAdminRecipients(leadCoachID *uuid.UUID) []RecipientInfo {
	var list []RecipientInfo
	seenUsers := make(map[uuid.UUID]bool)
	seenEmails := make(map[string]bool)

	// Lead coach if specified
	if leadCoachID != nil && *leadCoachID != uuid.Nil {
		if coach, err := s.store.GetCoachByID(*leadCoachID); err == nil && coach != nil {
			var coachUserID *uuid.UUID
			if users, err := s.store.GetUsersByRole(models.RoleCoach); err == nil {
				for _, u := range users {
					if u.CoachID != nil && *u.CoachID == coach.ID {
						coachUserID = &u.ID
						break
					}
				}
			}
			if coachUserID != nil {
				seenUsers[*coachUserID] = true
			}
			if coach.Email != "" {
				seenEmails[strings.ToLower(strings.TrimSpace(coach.Email))] = true
			}
			list = append(list, RecipientInfo{
				UserID: coachUserID,
				Role:   models.RoleCoach,
				Type:   "COACH",
				Name:   coach.FullName,
				Email:  coach.Email,
				Phone:  coach.Phone,
			})
		}
	}

	// All active Admins and Operation Managers
	if admins, err := s.store.GetUsersByRole(models.RoleAdmin); err == nil {
		for _, u := range admins {
			emailLower := strings.ToLower(strings.TrimSpace(u.Email))
			if !u.IsActive || seenUsers[u.ID] || (emailLower != "" && seenEmails[emailLower]) {
				continue
			}
			seenUsers[u.ID] = true
			if emailLower != "" {
				seenEmails[emailLower] = true
			}
			list = append(list, RecipientInfo{
				UserID: &u.ID,
				Role:   models.RoleAdmin,
				Type:   "ADMIN",
				Name:   u.DisplayName,
				Email:  u.Email,
			})
		}
	}

	if managers, err := s.store.GetUsersByRole(models.RoleOperationManager); err == nil {
		for _, u := range managers {
			emailLower := strings.ToLower(strings.TrimSpace(u.Email))
			if !u.IsActive || seenUsers[u.ID] || (emailLower != "" && seenEmails[emailLower]) {
				continue
			}
			seenUsers[u.ID] = true
			if emailLower != "" {
				seenEmails[emailLower] = true
			}
			list = append(list, RecipientInfo{
				UserID: &u.ID,
				Role:   models.RoleOperationManager,
				Type:   "OPERATION_MANAGER",
				Name:   u.DisplayName,
				Email:  u.Email,
			})
		}
	}

	return list
}

func (s *NotificationService) getManagerRecipients() []RecipientInfo {
	var list []RecipientInfo
	seenUsers := make(map[uuid.UUID]bool)
	seenEmails := make(map[string]bool)

	if managers, err := s.store.GetUsersByRole(models.RoleOperationManager); err == nil {
		for _, u := range managers {
			emailLower := strings.ToLower(strings.TrimSpace(u.Email))
			if !u.IsActive || seenUsers[u.ID] || (emailLower != "" && seenEmails[emailLower]) {
				continue
			}
			seenUsers[u.ID] = true
			if emailLower != "" {
				seenEmails[emailLower] = true
			}
			list = append(list, RecipientInfo{
				UserID: &u.ID,
				Role:   models.RoleOperationManager,
				Type:   "OPERATION_MANAGER",
				Name:   u.DisplayName,
				Email:  u.Email,
			})
		}
	}
	return list
}

func (s *NotificationService) getStudentAndGuardianRecipients(student *models.Student) []RecipientInfo {
	var list []RecipientInfo
	if student == nil {
		return list
	}

	// 1. Linked user account for student
	var studentUserID *uuid.UUID
	var studentEmail string
	if users, err := s.store.GetUsersByRole(models.RoleStudent); err == nil {
		for _, u := range users {
			if u.StudentID != nil && *u.StudentID == student.ID {
				studentUserID = &u.ID
				studentEmail = u.Email
				break
			}
		}
	}

	// Student Practitioner
	list = append(list, RecipientInfo{
		UserID: studentUserID,
		Role:   models.RoleStudent,
		Type:   "STUDENT",
		Name:   student.FullName,
		Email:  studentEmail,
		Phone:  student.Phone,
	})

	// Guardian / Emergency Contact
	emergencyPhone := strings.TrimSpace(student.EmergencyPhone)
	emergencyName := strings.TrimSpace(student.EmergencyName)
	if emergencyPhone != "" || emergencyName != "" {
		// UserID is set to nil and Role to "" so guardian alert does NOT duplicate in the student's personal account
		list = append(list, RecipientInfo{
			UserID: nil,
			Role:   "",
			Type:   "GUARDIAN",
			Name:   fmt.Sprintf("%s (Guardian for %s)", emergencyName, student.FullName),
			Email:  "", // Emergency contact is notified via SMS/phone directly
			Phone:  emergencyPhone,
		})
	}

	return list
}

func determineDeepLinkURL(event models.NotificationEventType, metadata string) string {
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(metadata), &meta); err == nil {
		if event == models.EventNewClassOpened {
			if dateStr, ok := meta["session_date"].(string); ok && dateStr != "" {
				return "/sessions?date=" + dateStr
			}
		}
		if sessID, ok := meta["session_id"].(string); ok && sessID != "" {
			return "/sessions/" + sessID + "/live"
		}
		if studentID, ok := meta["student_id"].(string); ok && studentID != "" {
			return "/students/" + studentID
		}
	}
	switch event {
	case models.EventStudentAdmitted, models.EventNewClassOpened, models.EventSlotReserved, models.EventClassCancelled:
		return "/sessions"
	case models.EventStaffRegistered:
		return "/admins"
	case models.EventPromotionEligible, models.EventStudentInjured, models.EventNewEvaluation, models.EventStudentRegistered:
		return "/students"
	case models.EventMembershipAwarded, models.EventMembershipRevoked, models.EventPassExpiring:
		return "/packages"
	default:
		return "/dashboard"
	}
}

func (s *NotificationService) sendTelegramNotification(event models.NotificationEventType, title, message, metadata string, settings *models.NotificationSettings) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("⚠️ Recovered from panic in sendTelegramNotification: %v", r)
		}
	}()

	if settings == nil || !settings.TelegramEnabled || strings.TrimSpace(settings.TelegramBotToken) == "" {
		return
	}

	botToken := strings.TrimSpace(settings.TelegramBotToken)
	var meta map[string]interface{}
	_ = json.Unmarshal([]byte(metadata), &meta)

	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("APP_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("HOST")), "/")
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}

	// Helper to resolve session and location for location-curated destinations
	var targetSession *models.TrainingSession
	var targetLocation *models.Location
	if sessIDStr, ok := meta["session_id"].(string); ok && sessIDStr != "" {
		if sessID, err := uuid.Parse(sessIDStr); err == nil {
			if sess, err := s.store.GetSessionByID(sessID); err == nil && sess != nil {
				targetSession = sess
				if sess.LocationID != nil && *sess.LocationID != uuid.Nil {
					if loc, err := s.store.GetLocationByID(*sess.LocationID); err == nil && loc != nil {
						targetLocation = loc
					}
				}
			}
		}
	}

	switch event {
	case models.EventNewClassOpened:
		if !settings.NotifyTelegramNewClass {
			return
		}
		// Location-curated destination or global fallback
		targetChat := strings.TrimSpace(settings.TelegramPublicChatID)
		locName := "Main Dojang"
		if targetLocation != nil {
			if strings.TrimSpace(targetLocation.TelegramChannelID) != "" {
				targetChat = strings.TrimSpace(targetLocation.TelegramChannelID)
			}
			if targetLocation.Name != "" {
				locName = targetLocation.Name
			}
		}
		if targetChat == "" {
			return
		}

		sessionDate := ""
		timeRange := ""
		coach := "Instructor"
		progTitle := "Training Session"
		if meta != nil {
			if d, ok := meta["session_date"].(string); ok {
				sessionDate = d
			}
			if tr, ok := meta["time_range"].(string); ok {
				timeRange = tr
			}
			if c, ok := meta["coach"].(string); ok {
				coach = c
			}
			if t, ok := meta["training_type"].(string); ok {
				progTitle = t
			}
		}
		if targetSession != nil {
			progTitle = targetSession.DisplayTitle()
			sessionDate = targetSession.SessionDate.Format("Monday, Jan 02, 2006")
			timeRange = targetSession.TimeRange()
		}

		htmlText := fmt.Sprintf("🥋 <b>NEW CLASS SCHEDULED</b> 📅\n\n"+
			"A new class is now open for reservations:\n\n"+
			"<b>Program:</b> %s\n"+
			"<b>Branch:</b> 📍 %s\n"+
			"<b>Date:</b> %s\n"+
			"<b>Time:</b> %s\n"+
			"<b>Instructor:</b> %s\n\n"+
			"👉 <i>Reserve early on STMS before slots fill up!</i>",
			progTitle, locName, sessionDate, timeRange, coach)

		var buttons [][]TelegramInlineButton
		if targetSession != nil {
			buttons = append(buttons, []TelegramInlineButton{
				{Text: "🥋 Reserve a Spot", URL: fmt.Sprintf("%s/reserve/%s", baseURL, targetSession.ID.String())},
				{Text: "📅 View Timetable", URL: fmt.Sprintf("%s/sessions?date=%s", baseURL, targetSession.SessionDate.Format("2006-01-02"))},
			})
		}

		if err := SendTelegramMessage(botToken, targetChat, htmlText, buttons); err != nil {
			log.Printf("⚠️ Telegram delivery error to %s: %v", targetChat, err)
		}

	case models.EventClassCancelled:
		if !settings.NotifyTelegramClassCancel {
			return
		}
		targetChat := strings.TrimSpace(settings.TelegramPublicChatID)
		locName := ""
		if targetLocation != nil {
			if strings.TrimSpace(targetLocation.TelegramChannelID) != "" {
				targetChat = strings.TrimSpace(targetLocation.TelegramChannelID)
			}
			locName = " (" + targetLocation.Name + ")"
		}
		if targetChat == "" {
			return
		}

		progTitle := "Class"
		reason := "Facility maintenance / scheduling update"
		if targetSession != nil {
			progTitle = targetSession.DisplayTitle()
		}
		if r, ok := meta["reason"].(string); ok && r != "" {
			reason = r
		}

		htmlText := fmt.Sprintf("⚠️ <b>CLASS CANCELLATION NOTICE</b> ⚠️\n\n"+
			"Please note that the following class has been cancelled%s:\n\n"+
			"<b>Class:</b> %s\n"+
			"<b>Reason:</b> %s\n\n"+
			"<i>Any pass deductions have been credited back. Please check the timetable for alternative training slots.</i>",
			locName, progTitle, reason)

		buttons := [][]TelegramInlineButton{
			{{Text: "📅 Check Alternative Classes", URL: baseURL + "/sessions"}},
		}
		if err := SendTelegramMessage(botToken, targetChat, htmlText, buttons); err != nil {
			log.Printf("⚠️ Telegram delivery error to %s: %v", targetChat, err)
		}

	case models.EventSlotReserved:
		if !settings.NotifyTelegramSlotReserved {
			return
		}
		// Route to Branch Staff Chat or fallback to Global Staff Chat
		targetChat := strings.TrimSpace(settings.TelegramStaffChatID)
		locName := ""
		if targetLocation != nil {
			if strings.TrimSpace(targetLocation.TelegramStaffChatID) != "" {
				targetChat = strings.TrimSpace(targetLocation.TelegramStaffChatID)
			}
			locName = " | 📍 " + targetLocation.Name
		}
		if targetChat == "" {
			targetChat = strings.TrimSpace(settings.TelegramPublicChatID)
		}
		if targetChat == "" {
			return
		}

		attendeeName := "Student"
		attendeeType := "Member"
		contactInfo := "Not provided"
		progTitle := "Training Session"
		timeStr := ""
		if meta != nil {
			if n, ok := meta["attendee_name"].(string); ok && n != "" {
				attendeeName = n
			}
			if t, ok := meta["attendee_type"].(string); ok && t != "" {
				attendeeType = t
			}
			if c, ok := meta["contact_info"].(string); ok && c != "" {
				contactInfo = c
			}
			if tr, ok := meta["time_range"].(string); ok {
				timeStr = tr
			}
		}
		if targetSession != nil {
			progTitle = targetSession.DisplayTitle()
			if timeStr == "" {
				timeStr = fmt.Sprintf("%s (%s)", targetSession.SessionDate.Format("Monday, Jan 02"), targetSession.TimeRange())
			}
		}

		htmlText := fmt.Sprintf("🥋 <b>NEW CLASS RESERVATION</b> 🎟️\n\n"+
			"A new slot was just reserved online:\n\n"+
			"<b>Attendee:</b> %s (%s)\n"+
			"<b>Contact:</b> %s\n"+
			"<b>Class:</b> %s%s\n"+
			"<b>Schedule:</b> %s",
			attendeeName, attendeeType, contactInfo, progTitle, locName, timeStr)

		var buttons [][]TelegramInlineButton
		if targetSession != nil {
			buttons = append(buttons, []TelegramInlineButton{
				{Text: "🥋 Open Live Session", URL: fmt.Sprintf("%s/sessions/%s/live", baseURL, targetSession.ID.String())},
			})
		}
		if err := SendTelegramMessage(botToken, targetChat, htmlText, buttons); err != nil {
			log.Printf("⚠️ Telegram delivery error to %s: %v", targetChat, err)
		}

	case models.EventStudentRegistered:
		if !settings.NotifyTelegramNewStudent {
			return
		}
		targetChat := strings.TrimSpace(settings.TelegramStaffChatID)
		if targetChat == "" {
			targetChat = strings.TrimSpace(settings.TelegramPublicChatID)
		}
		if targetChat == "" {
			return
		}

		name := "New Student"
		belt := "White Belt"
		age := 0
		pkg := "None"
		emerg := ""
		studentID := ""
		if meta != nil {
			if n, ok := meta["name"].(string); ok {
				name = n
			}
			if b, ok := meta["belt"].(string); ok {
				belt = b
			}
			if a, ok := meta["age"].(float64); ok {
				age = int(a)
			}
			if p, ok := meta["package"].(string); ok {
				pkg = p
			}
			if ep, ok := meta["emergency_phone"].(string); ok && ep != "" {
				emerg = ep
				if en, ok := meta["emergency_name"].(string); ok && en != "" {
					emerg = fmt.Sprintf("%s (%s)", en, ep)
				}
			}
			if id, ok := meta["student_id"].(string); ok {
				studentID = id
			}
		}

		ageStr := ""
		if age > 0 {
			ageStr = fmt.Sprintf(" (Age %d)", age)
		}
		emergStr := ""
		if emerg != "" {
			emergStr = fmt.Sprintf("\n<b>Emergency Contact:</b> %s", emerg)
		}

		htmlText := fmt.Sprintf("👋 <b>NEW STUDENT ENROLLED</b> 🥋\n\n"+
			"A new student has joined Series Taekwondo:\n\n"+
			"<b>Name:</b> %s%s\n"+
			"<b>Belt:</b> %s\n"+
			"<b>Pass / Package:</b> %s%s",
			name, ageStr, belt, pkg, emergStr)

		var buttons [][]TelegramInlineButton
		if studentID != "" {
			buttons = append(buttons, []TelegramInlineButton{
				{Text: "📋 View Student Dossier", URL: fmt.Sprintf("%s/students/%s", baseURL, studentID)},
			})
		}
		if err := SendTelegramMessage(botToken, targetChat, htmlText, buttons); err != nil {
			log.Printf("⚠️ Telegram delivery error to %s: %v", targetChat, err)
		}

	case models.EventStaffRegistered:
		if !settings.NotifyTelegramStaffRegistered {
			return
		}
		targetChat := strings.TrimSpace(settings.TelegramStaffChatID)
		if targetChat == "" {
			targetChat = strings.TrimSpace(settings.TelegramPublicChatID)
		}
		if targetChat == "" {
			return
		}

		htmlText := fmt.Sprintf("👤 <b>NEW STAFF REGISTRATION</b> 🥋\n\n"+
			"%s\n\n"+
			"👉 <i>Please review credentials and access rights in the Staff Directory.</i>",
			message)

		buttons := [][]TelegramInlineButton{
			{{Text: "👥 Open Staff Directory", URL: baseURL + "/admins"}},
		}
		if err := SendTelegramMessage(botToken, targetChat, htmlText, buttons); err != nil {
			log.Printf("⚠️ Telegram delivery error to %s: %v", targetChat, err)
		}

	case models.EventPromotionEligible:
		targetChat := strings.TrimSpace(settings.TelegramPublicChatID)
		if targetChat == "" {
			return
		}
		htmlText := fmt.Sprintf("🎉 <b>BELT ADVANCEMENT MILESTONE!</b> 🥋🏅\n\n%s", message)
		if err := SendTelegramMessage(botToken, targetChat, htmlText, nil); err != nil {
			log.Printf("⚠️ Telegram delivery error to %s: %v", targetChat, err)
		}
	}
}

func (s *NotificationService) sendEmailNotification(recipient RecipientInfo, title, message string, event models.NotificationEventType, metadata string, settings *models.NotificationSettings) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("⚠️ Recovered from panic in sendEmailNotification: %v", r)
		}
	}()

	if settings == nil {
		return
	}

	host := strings.TrimSpace(settings.SMTPHost)
	if host == "" {
		return
	}

	to := strings.TrimSpace(recipient.Email)
	if to == "" || !strings.Contains(to, "@") {
		return
	}

	port := settings.SMTPPort
	if port <= 0 {
		port = 587
	}

	from := strings.TrimSpace(settings.SMTPFrom)
	if from == "" {
		from = "notifications@seriestkd.com"
	}

	user := strings.TrimSpace(settings.SMTPUser)
	password := strings.TrimSpace(settings.SMTPPassword)
	if strings.Contains(strings.ToLower(host), "gmail.com") {
		password = strings.ReplaceAll(password, " ", "")
	}

	// Simulated / mock delivery for testing or development
	if strings.ToLower(host) == "simulated" || strings.ToLower(host) == "mock" || strings.HasSuffix(strings.ToLower(host), ".test") {
		log.Printf("✉️ [SIMULATED EMAIL DISPATCH] To: %s | Host: %s:%d | Event: %s | Title: %s", to, host, port, event, title)
		return
	}

	// Determine deep link URL
	deepLink := determineDeepLinkURL(event, metadata)
	baseURL := strings.TrimRight(strings.TrimSpace(os.Getenv("APP_BASE_URL")), "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(strings.TrimSpace(os.Getenv("HOST")), "/")
	}
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	fullLink := baseURL + deepLink

	// Build Subject, HTML and Text body
	subject, bodyHTML, bodyText := buildNotificationEmail(recipient, title, message, event, metadata, fullLink)

	if err := sendSMTPMail(host, port, user, password, from, to, subject, bodyHTML, bodyText); err != nil {
		log.Printf("❌ [EMAIL DELIVERY FAILED] To: %s | Error: %v", to, err)
	} else {
		log.Printf("✅ [EMAIL DELIVERED] To: %s | Subject: %s", to, subject)
	}
}

func buildNotificationEmail(recipient RecipientInfo, title, message string, event models.NotificationEventType, metadata string, actionURL string) (string, string, string) {
	subject := fmt.Sprintf("🥋 Series TKD: %s", title)
	buttonText := "Open Dashboard"

	var meta map[string]interface{}
	_ = json.Unmarshal([]byte(metadata), &meta)

	switch event {
	case models.EventNewClassOpened:
		trainingType, _ := meta["training_type"].(string)
		if trainingType == "" {
			trainingType = "Training Class"
		}
		sessionDate, _ := meta["session_date"].(string)
		timeRange, _ := meta["time_range"].(string)
		subject = fmt.Sprintf("📅 New Class: %s on %s", trainingType, sessionDate)
		if timeRange != "" {
			subject = fmt.Sprintf("📅 New Class: %s on %s (%s)", trainingType, sessionDate, timeRange)
		}
		buttonText = "View Class Timetable"
	case models.EventSlotReserved:
		attendeeName, _ := meta["attendee_name"].(string)
		attendeeType, _ := meta["attendee_type"].(string)
		if attendeeType == "" {
			attendeeType = "Reservation"
		}
		subject = fmt.Sprintf("🎟️ %s Slot Reserved: %s", attendeeType, attendeeName)
		buttonText = "View Live Roster"
	case models.EventStudentAdmitted:
		subject = title
		buttonText = "View Live Roster"
	}

	recipientGreeting := recipient.Name
	if recipientGreeting == "" {
		recipientGreeting = "Staff Member"
	}

	// Plain text fallback
	bodyText := fmt.Sprintf(`SERIES TAEKWONDO MANAGEMENT SYSTEM
========================================

Hello %s,

%s

Details:
%s

Action link: %s

Train with passion, lead with purpose.
Series Taekwondo Team
`, recipientGreeting, title, message, actionURL)

	// HTML details rows
	var detailsRows strings.Builder
	detailsRows.WriteString(fmt.Sprintf(`<tr><td style="padding: 6px 12px; font-weight: 600; color: #475569; width: 140px;">Notification:</td><td style="padding: 6px 12px; color: #0f172a;">%s</td></tr>`, html.EscapeString(title)))

	if trainingType, ok := meta["training_type"].(string); ok && trainingType != "" {
		detailsRows.WriteString(fmt.Sprintf(`<tr><td style="padding: 6px 12px; font-weight: 600; color: #475569;">Category:</td><td style="padding: 6px 12px; color: #0f172a; font-weight: 600;">%s</td></tr>`, html.EscapeString(trainingType)))
	}
	if sessionDate, ok := meta["session_date"].(string); ok && sessionDate != "" {
		detailsRows.WriteString(fmt.Sprintf(`<tr><td style="padding: 6px 12px; font-weight: 600; color: #475569;">Date:</td><td style="padding: 6px 12px; color: #0f172a;">%s</td></tr>`, html.EscapeString(sessionDate)))
	}
	if timeRange, ok := meta["time_range"].(string); ok && timeRange != "" {
		detailsRows.WriteString(fmt.Sprintf(`<tr><td style="padding: 6px 12px; font-weight: 600; color: #475569;">Time:</td><td style="padding: 6px 12px; color: #0f172a;">%s</td></tr>`, html.EscapeString(timeRange)))
	}
	if coach, ok := meta["coach"].(string); ok && coach != "" {
		detailsRows.WriteString(fmt.Sprintf(`<tr><td style="padding: 6px 12px; font-weight: 600; color: #475569;">Coach:</td><td style="padding: 6px 12px; color: #0f172a;">%s</td></tr>`, html.EscapeString(coach)))
	}
	if admin, ok := meta["admin"].(string); ok && admin != "" {
		detailsRows.WriteString(fmt.Sprintf(`<tr><td style="padding: 6px 12px; font-weight: 600; color: #475569;">Supervising Admin:</td><td style="padding: 6px 12px; color: #0f172a;">%s</td></tr>`, html.EscapeString(admin)))
	}
	if attendeeName, ok := meta["attendee_name"].(string); ok && attendeeName != "" {
		attType, _ := meta["attendee_type"].(string)
		label := "Attendee:"
		if attType != "" {
			label = attType + ":"
		}
		detailsRows.WriteString(fmt.Sprintf(`<tr><td style="padding: 6px 12px; font-weight: 600; color: #475569;">%s</td><td style="padding: 6px 12px; color: #0f172a; font-weight: 600;">%s</td></tr>`, html.EscapeString(label), html.EscapeString(attendeeName)))
	}
	if contactInfo, ok := meta["contact_info"].(string); ok && contactInfo != "" {
		detailsRows.WriteString(fmt.Sprintf(`<tr><td style="padding: 6px 12px; font-weight: 600; color: #475569;">Contact Info:</td><td style="padding: 6px 12px; color: #0f172a;">%s</td></tr>`, html.EscapeString(contactInfo)))
	}

	bodyHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>%s</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background-color: #f8fafc; margin: 0; padding: 24px; color: #1e293b;">
  <div style="max-width: 580px; margin: 0 auto; background-color: #ffffff; border-radius: 16px; border: 1px solid #e2e8f0; overflow: hidden; box-shadow: 0 4px 6px -1px rgba(0,0,0,0.05);">
    <div style="background-color: #990303; padding: 24px; text-align: center; color: #ffffff;">
      <h1 style="margin: 0; font-size: 20px; font-weight: 800; letter-spacing: 1px;">SERIES TAEKWONDO</h1>
      <p style="margin: 4px 0 0 0; font-size: 12px; opacity: 0.9; text-transform: uppercase; letter-spacing: 0.5px;">Operations &amp; Class Notification</p>
    </div>
    <div style="padding: 24px;">
      <h2 style="font-size: 16px; font-weight: 700; color: #0f172a; margin-top: 0;">Hello %s,</h2>
      <p style="font-size: 14px; line-height: 1.5; color: #475569;">
        %s
      </p>
      <div style="background-color: #f8fafc; border-radius: 8px; border: 1px solid #e2e8f0; margin: 20px 0; overflow: hidden;">
        <table style="width: 100%%; border-collapse: collapse; font-size: 13px; text-align: left;">
          <tbody>
            %s
          </tbody>
        </table>
      </div>
      <div style="text-align: center; margin: 24px 0;">
        <a href="%s" style="display: inline-block; background-color: #990303; color: #ffffff; text-decoration: none; font-weight: 600; font-size: 14px; padding: 12px 24px; border-radius: 8px; box-shadow: 0 2px 4px rgba(153, 3, 3, 0.2);">
          %s &rarr;
        </a>
      </div>
      <p style="font-size: 12px; color: #64748b; margin-top: 24px; border-top: 1px solid #f1f5f9; padding-top: 16px;">
        Train with passion, lead with purpose.<br>
        &copy; Series Taekwondo Management System
      </p>
    </div>
  </div>
</body>
</html>`, html.EscapeString(subject), html.EscapeString(recipientGreeting), html.EscapeString(message), detailsRows.String(), html.EscapeString(actionURL), html.EscapeString(buttonText))

	return subject, bodyHTML, bodyText
}

// TestEmailParams holds the parameters for dispatching a test email
type TestEmailParams struct {
	Host      string
	Port      int
	User      string
	Password  string
	From      string
	Recipient string
}

// SendTestEmail sends an immediate diagnostic test email verifying SMTP configuration
func (s *NotificationService) SendTestEmail(params TestEmailParams) error {
	settings, err := s.GetSettings()
	if err != nil || settings == nil {
		settings = models.DefaultNotificationSettings()
	}

	host := strings.TrimSpace(params.Host)
	if host == "" {
		host = strings.TrimSpace(settings.SMTPHost)
	}
	if host == "" {
		return errors.New("SMTP host is not configured; please provide a valid SMTP server hostname")
	}

	port := params.Port
	if port <= 0 {
		port = settings.SMTPPort
	}
	if port <= 0 {
		port = 587
	}

	from := strings.TrimSpace(params.From)
	if from == "" {
		from = strings.TrimSpace(settings.SMTPFrom)
	}
	if from == "" {
		from = "notifications@seriestkd.com"
	}

	user := strings.TrimSpace(params.User)
	if user == "" {
		user = strings.TrimSpace(settings.SMTPUser)
	}

	password := strings.TrimSpace(params.Password)
	if password == "" {
		password = strings.TrimSpace(settings.SMTPPassword)
	}
	if strings.Contains(strings.ToLower(host), "gmail.com") {
		password = strings.ReplaceAll(password, " ", "")
	}

	recipient := strings.TrimSpace(params.Recipient)
	if recipient == "" {
		return errors.New("recipient email address is required")
	}
	if !strings.Contains(recipient, "@") {
		return errors.New("recipient must be a valid email address containing '@'")
	}

	// Support simulated or mock delivery for tests or local sandbox modes
	if strings.ToLower(host) == "simulated" || strings.ToLower(host) == "mock" || strings.HasSuffix(strings.ToLower(host), ".test") {
		log.Printf("✉️ [SIMULATED EMAIL TEST] To: %s | Host: %s:%d | From: %s | User: %s", recipient, host, port, from, user)
		return nil
	}

	subject := "🥋 Series Taekwondo: Test Email Notification"
	timestamp := time.Now().Format("2006-01-02 15:04:05 MST")

	bodyText := fmt.Sprintf(`🥋 SERIES TAEKWONDO MANAGEMENT SYSTEM
====================================

Hello,

This is a test notification confirming that your SMTP email settings are correctly configured and operational!

Server Host: %s:%d
From Address: %s
Dispatched To: %s
Timestamp: %s

Train with passion, lead with purpose.
Series Taekwondo Team
`, host, port, from, recipient, timestamp)

	bodyHTML := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>Series Taekwondo - Test Email</title>
</head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background-color: #f8fafc; margin: 0; padding: 24px; color: #1e293b;">
  <div style="max-width: 560px; margin: 0 auto; background-color: #ffffff; border-radius: 16px; border: 1px solid #e2e8f0; overflow: hidden; box-shadow: 0 4px 6px -1px rgba(0,0,0,0.05);">
    <div style="background-color: #990303; padding: 24px; text-align: center; color: #ffffff;">
      <h1 style="margin: 0; font-size: 20px; font-weight: 800; letter-spacing: 1px;">SERIES TAEKWONDO</h1>
      <p style="margin: 4px 0 0 0; font-size: 12px; opacity: 0.9;">SMTP Notification Delivery Test</p>
    </div>
    <div style="padding: 24px;">
      <h2 style="font-size: 16px; font-weight: 700; color: #0f172a; margin-top: 0;">Test Email Successful!</h2>
      <p style="font-size: 14px; line-height: 1.5; color: #475569;">
        This test notification confirms that outward SMTP email delivery from your Series Taekwondo Management System is operational.
      </p>
      <div style="background-color: #f1f5f9; border-radius: 8px; padding: 16px; margin: 20px 0; font-size: 13px; line-height: 1.6;">
        <div><strong>SMTP Host:</strong> %s:%d</div>
        <div><strong>From Address:</strong> %s</div>
        <div><strong>Recipient:</strong> %s</div>
        <div><strong>Dispatched At:</strong> %s</div>
      </div>
      <p style="font-size: 12px; color: #64748b; margin-bottom: 0;">
        Train with passion, lead with purpose.<br>
        &copy; Series Taekwondo
      </p>
    </div>
  </div>
</body>
</html>`, host, port, from, recipient, timestamp)

	return sendSMTPMail(host, port, user, password, from, recipient, subject, bodyHTML, bodyText)
}

func sendSMTPMail(host string, port int, user, password, from, to, subject, bodyHTML, bodyText string) error {
	addr := fmt.Sprintf("%s:%d", host, port)

	boundary := fmt.Sprintf("stms_boundary_%d", time.Now().UnixNano())

	header := make(map[string]string)
	header["From"] = from
	header["To"] = to
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = fmt.Sprintf("multipart/alternative; boundary=\"%s\"", boundary)
	header["Date"] = time.Now().Format(time.RFC1123Z)

	var msg strings.Builder
	for k, v := range header {
		msg.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	msg.WriteString("\r\n")

	// Plain text part
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	msg.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
	msg.WriteString(bodyText)
	msg.WriteString("\r\n\r\n")

	// HTML part
	msg.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msg.WriteString("Content-Type: text/html; charset=\"UTF-8\"\r\n")
	msg.WriteString("Content-Transfer-Encoding: 7bit\r\n\r\n")
	msg.WriteString(bodyHTML)
	msg.WriteString("\r\n\r\n")

	msg.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	rawMsg := []byte(msg.String())

	var client *smtp.Client
	if port == 465 {
		tlsConfig := &tls.Config{
			ServerName: host,
		}
		dialer := &net.Dialer{Timeout: 10 * time.Second}
		conn, err := tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server via TLS (%s): %w", addr, err)
		}
		c, err := smtp.NewClient(conn, host)
		if err != nil {
			_ = conn.Close()
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}
		client = c
	} else {
		conn, err := net.DialTimeout("tcp", addr, 10*time.Second)
		if err != nil {
			return fmt.Errorf("failed to connect to SMTP server (%s): %w", addr, err)
		}
		c, err := smtp.NewClient(conn, host)
		if err != nil {
			_ = conn.Close()
			return fmt.Errorf("failed to create SMTP client: %w", err)
		}
		client = c

		if ok, _ := client.Extension("STARTTLS"); ok {
			tlsConfig := &tls.Config{
				ServerName: host,
			}
			if err := client.StartTLS(tlsConfig); err != nil {
				_ = client.Close()
				return fmt.Errorf("failed to initiate STARTTLS: %w", err)
			}
		}
	}
	defer client.Close()

	if user != "" && password != "" {
		auth := smtp.PlainAuth("", user, password, host)
		if err := client.Auth(auth); err != nil {
			errStr := err.Error()
			if strings.Contains(errStr, "535") || strings.Contains(errStr, "BadCredentials") || strings.Contains(strings.ToLower(errStr), "username and password not accepted") {
				if strings.Contains(strings.ToLower(host), "gmail.com") {
					return fmt.Errorf("Gmail SMTP authentication failed (535 BadCredentials): Google rejected the credentials. Google accounts require 2-Step Verification and a 16-character Google App Password (not your normal Gmail password). Generate an App Password at https://myaccount.google.com/apppasswords")
				}
			}
			return fmt.Errorf("SMTP authentication failed: %w", err)
		}
	}

	if err := client.Mail(from); err != nil {
		return fmt.Errorf("SMTP MAIL FROM failed: %w", err)
	}

	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("SMTP RCPT TO failed: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA failed: %w", err)
	}

	if _, err := w.Write(rawMsg); err != nil {
		return fmt.Errorf("failed to write email body: %w", err)
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to complete email data transfer: %w", err)
	}

	_ = client.Quit()
	return nil
}
