package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

type RecipientInfo struct {
	UserID   *uuid.UUID
	Role     models.UserRole
	Type     string // "COACH", "ADMIN", "STUDENT", "GUARDIAN", "OPERATION_MANAGER"
	Name     string
	Email    string
	Phone    string
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

		if settings.IsChannelEnabled(models.ChannelPush) && (userPrefs == nil || userPrefs.ShouldReceive(models.ChannelPush)) {
			dispatchedChannels = append(dispatchedChannels, models.ChannelPush)
			log.Printf("🔔 [PUSH NOTIFICATION] To: %s (%s) | %s: %s", r.Name, r.Role, title, message)
		}

		if settings.IsChannelEnabled(models.ChannelSMS) && (userPrefs == nil || userPrefs.ShouldReceive(models.ChannelSMS)) && strings.TrimSpace(r.Phone) != "" {
			dispatchedChannels = append(dispatchedChannels, models.ChannelSMS)
			log.Printf("📱 [SMS NOTIFICATION] To: %s (%s) | Phone: %s | %s", r.Name, r.Type, r.Phone, message)
		}

		if settings.IsChannelEnabled(models.ChannelEmail) && (userPrefs == nil || userPrefs.ShouldReceive(models.ChannelEmail)) && strings.TrimSpace(r.Email) != "" {
			dispatchedChannels = append(dispatchedChannels, models.ChannelEmail)
			log.Printf("✉️ [EMAIL NOTIFICATION] To: %s <%s> | %s: %s", r.Name, r.Email, title, message)
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
		message = fmt.Sprintf("%s self checked-in for %s class (%s).", student.FullName, sess.TrainingType, sess.SessionDate.Format("Jan 02"))
	} else {
		message = fmt.Sprintf("%s was admitted to %s class (%s) by %s.", student.FullName, sess.TrainingType, sess.SessionDate.Format("Jan 02"), actorName)
	}

	recipients := s.getCoachAndAdminRecipients(sess.CoachID)
	metadata := fmt.Sprintf(`{"student_id":"%s","session_id":"%s","was_self_admit":%t}`, student.ID, sess.ID, wasSelfAdmit)
	s.dispatch(models.EventStudentAdmitted, recipients, title, message, metadata)
}

// 2. Notify coach and admin: when new class has been opened
func (s *NotificationService) NotifyNewClassOpened(sess *models.TrainingSession, creatorName string) {
	if sess == nil {
		return
	}

	title := fmt.Sprintf("📅 New Class Opened: %s", sess.TrainingType)
	message := fmt.Sprintf("A new %s class was scheduled for %s (%s - %s) by %s.",
		sess.TrainingType, sess.SessionDate.Format("Monday, Jan 02"), sess.StartTime, sess.EndTime, creatorName)

	recipients := s.getCoachAndAdminRecipients(sess.CoachID)
	metadata := fmt.Sprintf(`{"session_id":"%s","training_type":"%s"}`, sess.ID, sess.TrainingType)
	s.dispatch(models.EventNewClassOpened, recipients, title, message, metadata)
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
