package models

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
)

type NotificationChannel string

const (
	ChannelInApp NotificationChannel = "IN_APP"
	ChannelPush  NotificationChannel = "PUSH"
	ChannelSMS   NotificationChannel = "SMS"
	ChannelEmail NotificationChannel = "EMAIL"
)

type NotificationEventType string

const (
	EventStudentAdmitted      NotificationEventType = "STUDENT_ADMITTED"
	EventNewClassOpened       NotificationEventType = "NEW_CLASS_OPENED"
	EventStaffRegistered      NotificationEventType = "STAFF_REGISTERED"
	EventPromotionEligible    NotificationEventType = "PROMOTION_ELIGIBLE"
	EventStudentInjured       NotificationEventType = "STUDENT_INJURED"
	EventNewEvaluation        NotificationEventType = "NEW_EVALUATION"
	EventMembershipAwarded    NotificationEventType = "MEMBERSHIP_AWARDED"
	EventMembershipRevoked    NotificationEventType = "MEMBERSHIP_REVOKED"
	EventSafetyResolved       NotificationEventType = "SAFETY_RESOLVED"
	EventClassCancelled       NotificationEventType = "CLASS_CANCELLED"
	EventPassExpiring         NotificationEventType = "PASS_EXPIRING"
)

type NotificationStatus string

const (
	StatusSent      NotificationStatus = "SENT"
	StatusSimulated NotificationStatus = "SIMULATED"
	StatusFailed    NotificationStatus = "FAILED"
	StatusDisabled  NotificationStatus = "DISABLED"
)

// NotificationSettings controls global channels and per-event notification toggles
type NotificationSettings struct {
	ID        uuid.UUID `json:"id"`
	PushEnabled  bool   `json:"push_enabled"`
	SMSEnabled   bool   `json:"sms_enabled"`
	EmailEnabled bool   `json:"email_enabled"`

	// Staff / Coach & Admin event triggers
	NotifyAdminStudentAdmitted bool `json:"notify_admin_student_admitted"`
	NotifyAdminNewClassOpened  bool `json:"notify_admin_new_class_opened"`
	NotifyAdminStaffRegistered bool `json:"notify_admin_staff_registered"`
	NotifyAdminStudentInjured  bool `json:"notify_admin_student_injured"`
	NotifyAdminClassCancelled  bool `json:"notify_admin_class_cancelled"`

	// Student / Guardian event triggers
	NotifyStudentPromotionEligible   bool `json:"notify_student_promotion_eligible"`
	NotifyStudentInjured             bool `json:"notify_student_injured"`
	NotifyStudentEvaluationLogged    bool `json:"notify_student_evaluation_logged"`
	NotifyStudentMembershipChanged   bool `json:"notify_student_membership_changed"`
	NotifyStudentSafetyResolved      bool `json:"notify_student_safety_resolved"`
	NotifyStudentClassCancelled      bool `json:"notify_student_class_cancelled"`
	NotifyStudentPassExpiring        bool `json:"notify_student_pass_expiring"`

	// Provider configurations
	SMTPHost          string    `json:"smtp_host"`
	SMTPPort          int       `json:"smtp_port"`
	SMTPUser          string    `json:"smtp_user"`
	SMTPPassword      string    `json:"smtp_password"`
	SMTPFrom          string    `json:"smtp_from"`
	SMSProvider       string    `json:"sms_provider"`
	SMSApiKey         string    `json:"sms_api_key"`
	SMSFromNumber     string    `json:"sms_from_number"`
	WebPushPublicKey  string    `json:"web_push_public_key"`
	WebPushPrivateKey string    `json:"web_push_private_key"`
	WebPushSubject     string    `json:"web_push_subject"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func DefaultNotificationSettings() *NotificationSettings {
	return &NotificationSettings{
		ID:           uuid.New(),
		PushEnabled:  true,
		SMSEnabled:   true,
		EmailEnabled: true,

		// Staff / Coach & Admin event toggles (default enabled)
		NotifyAdminStudentAdmitted: true,
		NotifyAdminNewClassOpened:  true,
		NotifyAdminStaffRegistered: true,
		NotifyAdminStudentInjured:  true,
		NotifyAdminClassCancelled:  true,

		// Student / Guardian event toggles (default enabled)
		NotifyStudentPromotionEligible: true,
		NotifyStudentInjured:           true,
		NotifyStudentEvaluationLogged:  true,
		NotifyStudentMembershipChanged: true,
		NotifyStudentSafetyResolved:    true,
		NotifyStudentClassCancelled:    true,
		NotifyStudentPassExpiring:      true,

		// Default provider placeholders
		SMTPPort:    587,
		SMSProvider: "simulated",
		UpdatedAt:   time.Now(),
	}
}

func (s *NotificationSettings) IsChannelEnabled(channel NotificationChannel) bool {
	if s == nil {
		return false
	}
	switch channel {
	case ChannelInApp:
		return true // In-app is always enabled as operational inbox
	case ChannelPush:
		return s.PushEnabled
	case ChannelSMS:
		return s.SMSEnabled
	case ChannelEmail:
		return s.EmailEnabled
	default:
		return false
	}
}

func (s *NotificationSettings) IsEventEnabled(eventType NotificationEventType) bool {
	if s == nil {
		return false
	}
	switch eventType {
	case EventStudentAdmitted:
		return s.NotifyAdminStudentAdmitted
	case EventNewClassOpened:
		return s.NotifyAdminNewClassOpened
	case EventStaffRegistered:
		return s.NotifyAdminStaffRegistered
	case EventPromotionEligible:
		return s.NotifyStudentPromotionEligible
	case EventStudentInjured:
		return s.NotifyStudentInjured || s.NotifyAdminStudentInjured
	case EventNewEvaluation:
		return s.NotifyStudentEvaluationLogged
	case EventMembershipAwarded, EventMembershipRevoked:
		return s.NotifyStudentMembershipChanged
	case EventSafetyResolved:
		return s.NotifyStudentSafetyResolved
	case EventClassCancelled:
		return s.NotifyStudentClassCancelled || s.NotifyAdminClassCancelled
	case EventPassExpiring:
		return s.NotifyStudentPassExpiring
	default:
		return true
	}
}

// UserNotificationPreferences allows individual practitioners or staff to configure channels
type UserNotificationPreferences struct {
	UserID       uuid.UUID `json:"user_id"`
	EmailEnabled bool      `json:"email_enabled"`
	SMSEnabled   bool      `json:"sms_enabled"`
	PushEnabled  bool      `json:"push_enabled"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func DefaultUserPreferences(userID uuid.UUID) *UserNotificationPreferences {
	return &UserNotificationPreferences{
		UserID:       userID,
		EmailEnabled: true,
		SMSEnabled:   true,
		PushEnabled:  true,
		UpdatedAt:    time.Now(),
	}
}

func (p *UserNotificationPreferences) ShouldReceive(channel NotificationChannel) bool {
	if p == nil {
		return true
	}
	switch channel {
	case ChannelInApp:
		return true
	case ChannelEmail:
		return p.EmailEnabled
	case ChannelSMS:
		return p.SMSEnabled
	case ChannelPush:
		return p.PushEnabled
	default:
		return true
	}
}

// Notification represents an in-app or dispatched message
type Notification struct {
	ID               uuid.UUID             `json:"id"`
	UserID           *uuid.UUID            `json:"user_id,omitempty"`
	RecipientRole    UserRole              `json:"recipient_role,omitempty"`
	RecipientType    string                `json:"recipient_type"` // "COACH", "ADMIN", "STUDENT", "GUARDIAN"
	RecipientName    string                `json:"recipient_name"`
	RecipientContact string                `json:"recipient_contact"` // Email address or phone number
	Channel          NotificationChannel   `json:"channel"`
	EventType        NotificationEventType `json:"event_type"`
	Title            string                `json:"title"`
	Message          string                `json:"message"`
	Metadata         string                `json:"metadata,omitempty"`
	Status           NotificationStatus    `json:"status"`
	IsRead           bool                  `json:"is_read"`
	CreatedAt        time.Time             `json:"created_at"`
}

func (n *Notification) MarkAsRead() {
	if n != nil {
		n.IsRead = true
	}
}

func (n *Notification) DispatchedChannels() []NotificationChannel {
	if n == nil {
		return nil
	}
	if strings.Contains(n.Metadata, `"channels"`) {
		var meta struct {
			Channels []NotificationChannel `json:"channels"`
		}
		if err := json.Unmarshal([]byte(n.Metadata), &meta); err == nil && len(meta.Channels) > 0 {
			return meta.Channels
		}
	}
	if n.Channel != "" {
		return []NotificationChannel{n.Channel}
	}
	return []NotificationChannel{ChannelInApp}
}

func (n *Notification) Icon() string {
	switch n.EventType {
	case EventStudentAdmitted:
		return "🥋"
	case EventNewClassOpened:
		return "📅"
	case EventStaffRegistered:
		return "👤"
	case EventPromotionEligible:
		return "🟢"
	case EventStudentInjured:
		return "🩹"
	case EventNewEvaluation:
		return "📊"
	case EventMembershipAwarded:
		return "💳"
	case EventMembershipRevoked:
		return "⚠️"
	case EventSafetyResolved:
		return "🛡️"
	case EventClassCancelled:
		return "🛑"
	case EventPassExpiring:
		return "⏳"
	default:
		return "🔔"
	}
}

func (n *Notification) BadgeClass() string {
	switch n.EventType {
	case EventPromotionEligible, EventSafetyResolved:
		return "bg-emerald-100 text-emerald-800 dark:bg-emerald-950 dark:text-emerald-300 border-emerald-300 dark:border-emerald-800"
	case EventStudentInjured, EventMembershipRevoked, EventClassCancelled:
		return "bg-rose-100 text-rose-800 dark:bg-rose-950 dark:text-rose-300 border-rose-300 dark:border-rose-800"
	case EventPassExpiring:
		return "bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-300 border-amber-300 dark:border-amber-800"
	default:
		return "bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-300 border-blue-300 dark:border-blue-800"
	}
}

type NotificationFilter struct {
	UserID     *uuid.UUID
	Role       *UserRole
	Channel    *NotificationChannel
	EventType  *NotificationEventType
	UnreadOnly bool
	Limit      int
	Offset     int
}
