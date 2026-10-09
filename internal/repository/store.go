package repository

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
)

var (
	ErrNotFound      = errors.New("record not found")
	ErrAlreadyInRoster = errors.New("student already checked in for this session")
	ErrCoachHasRecords = errors.New("cannot delete coach with existing training classes, evaluations, or incident logs; deactivate coach instead")
)

type SessionFilter struct {
	CoachID      *uuid.UUID
	StudentID    *uuid.UUID
	LocationID   *uuid.UUID
	TrainingType string
	Date         string // "YYYY-MM-DD"
	StartDate    string // "YYYY-MM-DD"
	EndDate      string // "YYYY-MM-DD"
	EntryType    string // "class", "event", "duty", "open_session"
}

type RepositoryStore interface {
	// Locations
	GetAllLocations() ([]*models.Location, error)
	GetLocationByID(id uuid.UUID) (*models.Location, error)
	CreateLocation(loc *models.Location) error
	UpdateLocation(loc *models.Location) error
	DeleteLocation(id uuid.UUID) error

	// Training Categories
	GetAllTrainingCategories() ([]*models.TrainingCategory, error)
	GetTrainingCategoryByID(id uuid.UUID) (*models.TrainingCategory, error)
	GetTrainingCategoryByName(name string) (*models.TrainingCategory, error)
	CreateTrainingCategory(cat *models.TrainingCategory) error
	UpdateTrainingCategory(cat *models.TrainingCategory) error
	DeleteTrainingCategory(id uuid.UUID) error

	// Students
	GetAllStudents() ([]*models.Student, error)
	GetStudentByID(id uuid.UUID) (*models.Student, error)
	SearchStudents(query string) ([]*models.Student, error)
	CreateStudent(s *models.Student) error
	UpdateStudent(s *models.Student) error
	DeleteStudent(studentID uuid.UUID) error

	// Coaches
	GetAllCoaches() ([]*models.Coach, error)
	GetCoachByID(id uuid.UUID) (*models.Coach, error)
	CreateCoach(c *models.Coach) error
	UpdateCoach(c *models.Coach) error
	ToggleCoachActive(coachID uuid.UUID, isActive bool) error
	DeleteCoach(coachID uuid.UUID) error

	// Packages
	GetPackageTemplates() ([]*models.PackageTemplate, error)
	GetPackageTemplateByID(id uuid.UUID) (*models.PackageTemplate, error)
	CreatePackageTemplate(tpl *models.PackageTemplate) error
	UpdatePackageTemplate(tpl *models.PackageTemplate) error
	TogglePackageTemplateStatus(id uuid.UUID, isActive bool) error
	GetStudentPackages(studentID uuid.UUID) ([]*models.StudentPackage, error)
	GetStudentPackageByID(id uuid.UUID) (*models.StudentPackage, error)
	AssignPackage(pkg *models.StudentPackage) error
	UpdateStudentPackage(pkg *models.StudentPackage) error
	RevokeStudentPackage(id uuid.UUID) error

	// Sessions & Floor Attendance
	GetAllSessions() ([]*models.TrainingSession, error)
	GetSessions(filter SessionFilter) ([]*models.TrainingSession, error)
	GetSessionByID(id uuid.UUID) (*models.TrainingSession, error)
	CreateSession(sess *models.TrainingSession) error
	UpdateSession(sess *models.TrainingSession) error
	DeleteSession(sessionID uuid.UUID) error
	CancelSession(sessionID uuid.UUID, reason string, refundCredits bool) error
	GetSessionAttendances(sessionID uuid.UUID) ([]*models.Attendance, error)
	GetStudentAttendances(studentID uuid.UUID) ([]*models.Attendance, error)
	CheckInStudent(sessionID, studentID uuid.UUID, packageID *uuid.UUID, sessionRate *float64) (*models.Attendance, error)
	CheckInAttendee(sessionID uuid.UUID, attendeeType string, attendeeID *uuid.UUID, attendeeName, attendeeRole string, packageID *uuid.UUID, sessionRate *float64) (*models.Attendance, error)
	RemoveAttendance(sessionID, studentID uuid.UUID) error

	// Batch Optimizations
	GetAllAttendances() ([]*models.Attendance, error)
	GetSessionAttendanceCounts() (map[uuid.UUID]int, error)
	GetLatestEvaluations() (map[uuid.UUID]*models.StudentEvaluation, error)
	GetAllStudentPackagesGrouped() (map[uuid.UUID][]*models.StudentPackage, error)

	// Evaluations
	GetLatestEvaluation(studentID uuid.UUID) (*models.StudentEvaluation, error)
	GetStudentEvaluations(studentID uuid.UUID) ([]*models.StudentEvaluation, error)
	CreateEvaluation(eval *models.StudentEvaluation) error

	// Auth & Users
	GetUserByEmail(email string) (*models.User, error)
	GetUserByUsername(username string) (*models.User, error)
	GetUserByIdentifier(identifier string) (*models.User, error)
	GetUserByID(id uuid.UUID) (*models.User, error)
	GetUserByStudentID(studentID uuid.UUID) (*models.User, error)
	GetUserByCoachID(coachID uuid.UUID) (*models.User, error)
	GetUsersByRole(role models.UserRole) ([]*models.User, error)
	CreateUser(user *models.User) error
	UpdateUser(user *models.User) error
	ToggleUserActive(userID uuid.UUID, isActive bool) error
	DeleteUser(userID uuid.UUID) error
	UpdateUserLastLogin(id uuid.UUID) error
	CreateSessionToken(token string, userID uuid.UUID, expiresAt time.Time) error
	GetUserBySessionToken(token string) (*models.User, error)
	DeleteSessionToken(token string) error

	// Password Reset Tokens
	CreatePasswordResetToken(token *models.PasswordResetToken) error
	GetPasswordResetToken(token string) (*models.PasswordResetToken, error)
	MarkPasswordResetTokenUsed(token string) error

	// Safety Incidents
	CreateSafetyIncident(inc *models.SafetyIncident) error
	GetSafetyIncidents(resolved *bool) ([]*models.SafetyIncident, error)
	ResolveSafetyIncident(incidentID uuid.UUID, adminEmail string) error
	SetStudentSafetyFlag(studentID uuid.UUID, hasSafetyFlag bool) error

	// Belt Promotion
	PromoteStudent(studentID uuid.UUID, newBelt models.BeltRank) error

	// Audit Logs
	CreateAuditLog(entry *models.AuditLog) error
	GetAuditLogs(filter models.AuditLogFilter) ([]*models.AuditLog, int, error)
	GetAuditTelemetry() (*models.AuditTelemetry, error)

	// Notification Settings & User Preferences
	GetNotificationSettings() (*models.NotificationSettings, error)
	UpdateNotificationSettings(settings *models.NotificationSettings) error
	GetUserNotificationPreferences(userID uuid.UUID) (*models.UserNotificationPreferences, error)
	UpdateUserNotificationPreferences(prefs *models.UserNotificationPreferences) error

	// Notifications
	CreateNotification(n *models.Notification) error
	GetNotifications(filter models.NotificationFilter) ([]*models.Notification, int, error)
	GetUnreadNotificationCount(userID *uuid.UUID, role *models.UserRole) (int, error)
	MarkNotificationRead(id uuid.UUID) error
	MarkAllNotificationsRead(userID *uuid.UUID, role *models.UserRole) error
}

type SessionTokenRecord struct {
	Token     string
	UserID    uuid.UUID
	ExpiresAt time.Time
}

type MemoryStore struct {
	mu                  sync.RWMutex
	students            map[uuid.UUID]*models.Student
	coaches             map[uuid.UUID]*models.Coach
	packageTemplates    map[uuid.UUID]*models.PackageTemplate
	studentPackages     map[uuid.UUID]*models.StudentPackage
	sessions            map[uuid.UUID]*models.TrainingSession
	attendances         map[uuid.UUID]*models.Attendance
	evaluations         map[uuid.UUID]*models.StudentEvaluation
	users               map[uuid.UUID]*models.User
	usersByEmail        map[string]uuid.UUID
	usersByUsername     map[string]uuid.UUID
	sessionTokens       map[string]SessionTokenRecord
	passwordResetTokens map[string]*models.PasswordResetToken
	safetyIncidents     map[uuid.UUID]*models.SafetyIncident
	locations           map[uuid.UUID]*models.Location
	trainingCategories  map[uuid.UUID]*models.TrainingCategory
	auditLogs           []*models.AuditLog
	notificationSettings *models.NotificationSettings
	userPreferences     map[uuid.UUID]*models.UserNotificationPreferences
	notifications       []*models.Notification
}

func NewMemoryStore() *MemoryStore {
	m := &MemoryStore{
		students:            make(map[uuid.UUID]*models.Student),
		coaches:             make(map[uuid.UUID]*models.Coach),
		packageTemplates:    make(map[uuid.UUID]*models.PackageTemplate),
		studentPackages:     make(map[uuid.UUID]*models.StudentPackage),
		sessions:            make(map[uuid.UUID]*models.TrainingSession),
		attendances:         make(map[uuid.UUID]*models.Attendance),
		evaluations:         make(map[uuid.UUID]*models.StudentEvaluation),
		users:               make(map[uuid.UUID]*models.User),
		usersByEmail:        make(map[string]uuid.UUID),
		usersByUsername:     make(map[string]uuid.UUID),
		sessionTokens:       make(map[string]SessionTokenRecord),
		passwordResetTokens: make(map[string]*models.PasswordResetToken),
		safetyIncidents:     make(map[uuid.UUID]*models.SafetyIncident),
		locations:           make(map[uuid.UUID]*models.Location),
		trainingCategories:  make(map[uuid.UUID]*models.TrainingCategory),
		auditLogs:           make([]*models.AuditLog, 0),
		notificationSettings: models.DefaultNotificationSettings(),
		userPreferences:     make(map[uuid.UUID]*models.UserNotificationPreferences),
		notifications:       make([]*models.Notification, 0),
	}
	m.seedData()
	return m
}

func (m *MemoryStore) GetAllStudents() ([]*models.Student, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*models.Student, 0, len(m.students))
	for _, s := range m.students {
		result = append(result, s)
	}
	return result, nil
}

func (m *MemoryStore) GetStudentByID(id uuid.UUID) (*models.Student, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.students[id]
	if !ok {
		return nil, ErrNotFound
	}
	return s, nil
}

func (m *MemoryStore) SearchStudents(query string) ([]*models.Student, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	q := strings.ToLower(strings.TrimSpace(query))
	result := []*models.Student{}
	for _, s := range m.students {
		if q == "" || strings.Contains(strings.ToLower(s.FullName), q) || strings.Contains(strings.ToLower(s.Phone), q) || strings.Contains(strings.ToLower(string(s.CurrentBelt)), q) {
			result = append(result, s)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].FullName) < strings.ToLower(result[j].FullName)
	})
	return result, nil
}

func (m *MemoryStore) CreateStudent(s *models.Student) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	s.CreatedAt = time.Now()
	m.students[s.ID] = s
	return nil
}

func (m *MemoryStore) UpdateStudent(s *models.Student) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.students[s.ID]; !ok {
		return ErrNotFound
	}
	m.students[s.ID] = s
	return nil
}

func (m *MemoryStore) DeleteStudent(studentID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.students[studentID]
	if !exists {
		return ErrNotFound
	}

	// Purge associated user accounts and sessions
	for uID, user := range m.users {
		if user.StudentID != nil && *user.StudentID == studentID {
			for token, rec := range m.sessionTokens {
				if rec.UserID == user.ID {
					delete(m.sessionTokens, token)
				}
			}
			for pToken, pRec := range m.passwordResetTokens {
				if pRec.UserID == user.ID {
					delete(m.passwordResetTokens, pToken)
				}
			}
			delete(m.usersByEmail, user.Email)
			delete(m.usersByEmail, strings.ToLower(user.Email))
			if user.Username != "" {
				delete(m.usersByUsername, strings.ToLower(user.Username))
			}
			delete(m.users, uID)
		}
	}

	// Cascade delete safety incidents
	for id, inc := range m.safetyIncidents {
		if inc.StudentID == studentID {
			delete(m.safetyIncidents, id)
		}
	}

	// Cascade delete evaluations
	for id, eval := range m.evaluations {
		if eval.StudentID == studentID {
			delete(m.evaluations, id)
		}
	}

	// Cascade delete attendance records
	for id, att := range m.attendances {
		if att.StudentID == studentID {
			delete(m.attendances, id)
		}
	}

	// Cascade delete student packages
	for id, pkg := range m.studentPackages {
		if pkg.StudentID == studentID {
			delete(m.studentPackages, id)
		}
	}

	// Delete student
	delete(m.students, studentID)
	return nil
}

func (m *MemoryStore) GetAllCoaches() ([]*models.Coach, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*models.Coach, 0, len(m.coaches))
	for _, c := range m.coaches {
		result = append(result, c)
	}
	return result, nil
}

func (m *MemoryStore) GetCoachByID(id uuid.UUID) (*models.Coach, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.coaches[id]
	if !ok {
		return nil, ErrNotFound
	}
	return c, nil
}

func (m *MemoryStore) CreateCoach(c *models.Coach) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	c.CreatedAt = time.Now()
	m.coaches[c.ID] = c
	return nil
}

func (m *MemoryStore) UpdateCoach(c *models.Coach) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.coaches[c.ID]; !ok {
		return ErrNotFound
	}
	m.coaches[c.ID] = c
	return nil
}

func (m *MemoryStore) ToggleCoachActive(coachID uuid.UUID, isActive bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	coach, exists := m.coaches[coachID]
	if !exists {
		return ErrNotFound
	}
	coach.IsActive = isActive

	// Update associated user account and purge sessions if deactivating
	for _, user := range m.users {
		if (user.CoachID != nil && *user.CoachID == coachID) || (user.Role == models.RoleCoach && strings.EqualFold(user.Email, coach.Email)) {
			user.IsActive = isActive
			user.UpdatedAt = time.Now()
			if !isActive {
				for token, rec := range m.sessionTokens {
					if rec.UserID == user.ID {
						delete(m.sessionTokens, token)
					}
				}
			}
		}
	}
	return nil
}

func (m *MemoryStore) DeleteCoach(coachID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	coach, exists := m.coaches[coachID]
	if !exists {
		return ErrNotFound
	}

	// Purge associated user accounts and sessions
	for uID, user := range m.users {
		if (user.CoachID != nil && *user.CoachID == coachID) || (user.Role == models.RoleCoach && strings.EqualFold(user.Email, coach.Email)) {
			for token, rec := range m.sessionTokens {
				if rec.UserID == user.ID {
					delete(m.sessionTokens, token)
				}
			}
			delete(m.usersByEmail, user.Email)
			delete(m.users, uID)
		}
	}

	// Cascade delete safety incidents
	for id, inc := range m.safetyIncidents {
		if inc.CoachID != nil && *inc.CoachID == coachID {
			delete(m.safetyIncidents, id)
		}
	}

	// Cascade delete evaluations
	for id, eval := range m.evaluations {
		if eval.CoachID == coachID {
			delete(m.evaluations, id)
		}
	}

	// Cascade delete training sessions and their attendances
	for sID, sess := range m.sessions {
		if sess.AdminID != nil && *sess.AdminID == coachID {
			sess.AdminID = nil
		}
		if sess.CoachID != nil && *sess.CoachID == coachID {
			delete(m.attendances, sID)
			delete(m.sessions, sID)
		}
	}

	delete(m.coaches, coachID)
	return nil
}


func (m *MemoryStore) GetPackageTemplates() ([]*models.PackageTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*models.PackageTemplate, 0, len(m.packageTemplates))
	for _, pt := range m.packageTemplates {
		result = append(result, pt)
	}
	return result, nil
}

func (m *MemoryStore) GetPackageTemplateByID(id uuid.UUID) (*models.PackageTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tpl, ok := m.packageTemplates[id]
	if !ok {
		return nil, ErrNotFound
	}
	return tpl, nil
}

func (m *MemoryStore) CreatePackageTemplate(tpl *models.PackageTemplate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if tpl.ID == uuid.Nil {
		tpl.ID = uuid.New()
	}
	m.packageTemplates[tpl.ID] = tpl
	return nil
}

func (m *MemoryStore) UpdatePackageTemplate(tpl *models.PackageTemplate) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.packageTemplates[tpl.ID]; !ok {
		return ErrNotFound
	}
	m.packageTemplates[tpl.ID] = tpl
	return nil
}

func (m *MemoryStore) TogglePackageTemplateStatus(id uuid.UUID, isActive bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tpl, ok := m.packageTemplates[id]
	if !ok {
		return ErrNotFound
	}
	tpl.IsActive = isActive
	return nil
}

func (m *MemoryStore) GetStudentPackages(studentID uuid.UUID) ([]*models.StudentPackage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := []*models.StudentPackage{}
	for _, sp := range m.studentPackages {
		if sp.StudentID == studentID {
			if tpl, ok := m.packageTemplates[sp.TemplateID]; ok {
				sp.TemplateTitle = tpl.Title
			}
			result = append(result, sp)
		}
	}
	return result, nil
}

func (m *MemoryStore) AssignPackage(pkg *models.StudentPackage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pkg.ID == uuid.Nil {
		pkg.ID = uuid.New()
	}
	pkg.CreatedAt = time.Now()
	m.studentPackages[pkg.ID] = pkg
	return nil
}

func (m *MemoryStore) GetStudentPackageByID(id uuid.UUID) (*models.StudentPackage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sp, ok := m.studentPackages[id]
	if !ok {
		return nil, ErrNotFound
	}
	if tpl, ok := m.packageTemplates[sp.TemplateID]; ok {
		sp.TemplateTitle = tpl.Title
	}
	return sp, nil
}

func (m *MemoryStore) UpdateStudentPackage(pkg *models.StudentPackage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.studentPackages[pkg.ID]; !ok {
		return ErrNotFound
	}
	m.studentPackages[pkg.ID] = pkg
	return nil
}

func (m *MemoryStore) RevokeStudentPackage(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sp, ok := m.studentPackages[id]
	if !ok {
		return ErrNotFound
	}
	sp.PaymentStatus = "revoked"
	return nil
}

func (m *MemoryStore) GetAllStudentPackagesGrouped() (map[uuid.UUID][]*models.StudentPackage, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[uuid.UUID][]*models.StudentPackage)
	for _, sp := range m.studentPackages {
		if tpl, ok := m.packageTemplates[sp.TemplateID]; ok {
			sp.TemplateTitle = tpl.Title
		}
		result[sp.StudentID] = append(result[sp.StudentID], sp)
	}
	return result, nil
}

func (m *MemoryStore) GetAllSessions() ([]*models.TrainingSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*models.TrainingSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		if s.CoachID != nil {
			if coach, ok := m.coaches[*s.CoachID]; ok {
				s.CoachName = coach.FullName
			} else {
				s.CoachName = ""
			}
		} else {
			s.CoachName = ""
		}
		result = append(result, s)
	}
	return result, nil
}

func (m *MemoryStore) GetSessionByID(id uuid.UUID) (*models.TrainingSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.sessions[id]
	if !ok {
		return nil, ErrNotFound
	}
	if s.CoachID != nil {
		if coach, ok := m.coaches[*s.CoachID]; ok {
			s.CoachName = coach.FullName
		} else {
			s.CoachName = ""
		}
	} else {
		s.CoachName = ""
	}
	if s.AdminID != nil {
		if u, ok := m.users[*s.AdminID]; ok {
			if u.DisplayName != "" {
				s.AdminName = u.DisplayName
			} else {
				s.AdminName = u.Email
			}
		} else if c, ok := m.coaches[*s.AdminID]; ok {
			s.AdminName = c.FullName
		}
	}
	return s, nil
}

func (m *MemoryStore) GetSessions(filter SessionFilter) ([]*models.TrainingSession, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*models.TrainingSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		if filter.CoachID != nil {
			matchesCoach := s.CoachID != nil && *s.CoachID == *filter.CoachID
			if !matchesCoach && s.HasAssignedStaffID(filter.CoachID.String()) {
				matchesCoach = true
			}
			if !matchesCoach {
				continue
			}
		}
		if filter.LocationID != nil {
			if s.LocationID == nil || *s.LocationID != *filter.LocationID {
				continue
			}
		}
		if filter.StudentID != nil {
			hasAtt := false
			for _, att := range m.attendances {
				if att.SessionID == s.ID && att.StudentID == *filter.StudentID {
					hasAtt = true
					break
				}
			}
			if !hasAtt {
				continue
			}
		}
		if filter.TrainingType != "" && string(s.TrainingType) != filter.TrainingType {
			continue
		}
		if filter.EntryType != "" {
			if filter.EntryType == "class" {
				if !s.IsClass() {
					continue
				}
			} else if string(s.EntryType) != filter.EntryType {
				continue
			}
		}
		if filter.Date != "" {
			dateStr := s.SessionDate.Format("2006-01-02")
			if dateStr != filter.Date {
				continue
			}
		} else {
			dateStr := s.SessionDate.Format("2006-01-02")
			if filter.StartDate != "" && dateStr < filter.StartDate {
				continue
			}
			if filter.EndDate != "" && dateStr > filter.EndDate {
				continue
			}
		}
		if s.CoachID != nil {
			if coach, ok := m.coaches[*s.CoachID]; ok {
				s.CoachName = coach.FullName
			} else {
				s.CoachName = ""
			}
		} else {
			s.CoachName = ""
		}
		if s.LocationID != nil {
			if loc, ok := m.locations[*s.LocationID]; ok {
				s.LocationName = loc.Name
				s.LocationPin = loc.Pin
			}
		}
		if s.AdminID != nil {
			if u, ok := m.users[*s.AdminID]; ok {
				if u.DisplayName != "" {
					s.AdminName = u.DisplayName
				} else {
					s.AdminName = u.Email
				}
			} else if c, ok := m.coaches[*s.AdminID]; ok {
				s.AdminName = c.FullName
			}
		}
		result = append(result, s)
	}
	return result, nil
}

func (m *MemoryStore) CancelSession(sessionID uuid.UUID, reason string, refundCredits bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess, ok := m.sessions[sessionID]
	if !ok {
		return ErrNotFound
	}
	if sess.IsCancelled {
		return errors.New("class is already cancelled")
	}

	if refundCredits {
		// Refund any student package sessions that were deducted
		for _, att := range m.attendances {
			if att.SessionID == sessionID && att.StudentPackageID != nil {
				if pkg, exists := m.studentPackages[*att.StudentPackageID]; exists && pkg.RemainingSessions != nil {
					*pkg.RemainingSessions++
				}
			}
		}
		// Clear attendances for this session
		for id, att := range m.attendances {
			if att.SessionID == sessionID {
				delete(m.attendances, id)
			}
		}
	}

	now := time.Now()
	sess.IsCancelled = true
	sess.CancelledAt = &now
	sess.CancellationReason = reason
	return nil
}

func (m *MemoryStore) CreateSession(sess *models.TrainingSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sess.ID == uuid.Nil {
		sess.ID = uuid.New()
	}
	sess.CreatedAt = time.Now()
	m.sessions[sess.ID] = sess
	return nil
}

func (m *MemoryStore) UpdateSession(sess *models.TrainingSession) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.sessions[sess.ID]
	if !ok {
		return ErrNotFound
	}
	existing.SessionDate = sess.SessionDate
	existing.StartTime = sess.StartTime
	existing.EndTime = sess.EndTime
	existing.CoachID = sess.CoachID
	if sess.CoachID == nil {
		existing.CoachName = ""
	}
	existing.AdminID = sess.AdminID
	existing.LocationID = sess.LocationID
	existing.TrainingType = sess.TrainingType
	existing.Notes = sess.Notes
	existing.SessionRate = sess.SessionRate
	existing.EntryType = sess.EntryType
	existing.Title = sess.Title
	existing.AssignedStaff = sess.AssignedStaff
	return nil
}

func (m *MemoryStore) DeleteSession(sessionID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.sessions[sessionID]; !ok {
		return ErrNotFound
	}
	// Refund credits for any attendances in this session
	for _, att := range m.attendances {
		if att.SessionID == sessionID && att.StudentPackageID != nil {
			if pkg, exists := m.studentPackages[*att.StudentPackageID]; exists && pkg.RemainingSessions != nil {
				*pkg.RemainingSessions++
			}
		}
	}
	// Clear attendances for this session
	for id, att := range m.attendances {
		if att.SessionID == sessionID {
			delete(m.attendances, id)
		}
	}
	delete(m.sessions, sessionID)
	return nil
}

func (m *MemoryStore) GetSessionAttendances(sessionID uuid.UUID) ([]*models.Attendance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := []*models.Attendance{}
	for _, a := range m.attendances {
		if a.SessionID == sessionID {
			if st, ok := m.students[a.StudentID]; ok {
				a.StudentName = st.FullName
				a.StudentBelt = st.CurrentBelt
			}
			if a.StudentPackageID != nil {
				if sp, ok := m.studentPackages[*a.StudentPackageID]; ok {
					if tpl, ok2 := m.packageTemplates[sp.TemplateID]; ok2 {
						a.PackageTitle = tpl.Title
					}
				}
			}
			if a.LocationID != nil {
				if loc, ok := m.locations[*a.LocationID]; ok {
					a.LocationName = loc.Name
					a.LocationPin = loc.Pin
				}
			} else if sess, ok := m.sessions[a.SessionID]; ok && sess.LocationID != nil {
				a.LocationID = sess.LocationID
				if loc, ok := m.locations[*sess.LocationID]; ok {
					a.LocationName = loc.Name
					a.LocationPin = loc.Pin
				}
			}
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *MemoryStore) GetStudentAttendances(studentID uuid.UUID) ([]*models.Attendance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := []*models.Attendance{}
	for _, a := range m.attendances {
		if a.StudentID == studentID {
			if a.LocationID != nil {
				if loc, ok := m.locations[*a.LocationID]; ok {
					a.LocationName = loc.Name
					a.LocationPin = loc.Pin
				}
			} else if sess, ok := m.sessions[a.SessionID]; ok && sess.LocationID != nil {
				a.LocationID = sess.LocationID
				if loc, ok := m.locations[*sess.LocationID]; ok {
					a.LocationName = loc.Name
					a.LocationPin = loc.Pin
				}
			}
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *MemoryStore) CheckInAttendee(sessionID uuid.UUID, attendeeType string, attendeeID *uuid.UUID, attendeeName, attendeeRole string, packageID *uuid.UUID, sessionRate *float64) (*models.Attendance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if attendeeType == "" {
		attendeeType = "student"
	}

	// Check if already checked in
	for _, a := range m.attendances {
		if a.SessionID == sessionID {
			if attendeeType == "student" && attendeeID != nil && a.StudentID == *attendeeID {
				return nil, ErrAlreadyInRoster
			}
			if attendeeID != nil && a.StudentID == *attendeeID {
				return nil, ErrAlreadyInRoster
			}
			if attendeeName != "" && a.AttendeeName == attendeeName {
				return nil, ErrAlreadyInRoster
			}
		}
	}

	att := &models.Attendance{
		ID:               uuid.New(),
		SessionID:        sessionID,
		StudentPackageID: packageID,
		SessionRate:      sessionRate,
		CheckedInAt:      time.Now(),
		AttendeeType:     attendeeType,
		AttendeeName:     attendeeName,
		AttendeeRole:     attendeeRole,
	}

	if attendeeID != nil {
		att.StudentID = *attendeeID
	}

	if sess, ok := m.sessions[sessionID]; ok && sess.LocationID != nil {
		att.LocationID = sess.LocationID
		if loc, okLoc := m.locations[*sess.LocationID]; okLoc {
			att.LocationName = loc.Name
			att.LocationPin = loc.Pin
		}
	}

	if attendeeType == "student" && attendeeID != nil {
		if st, ok := m.students[*attendeeID]; ok {
			att.StudentName = st.FullName
			att.StudentBelt = st.CurrentBelt
			if att.AttendeeName == "" {
				att.AttendeeName = st.FullName
			}
			if att.AttendeeRole == "" {
				att.AttendeeRole = string(st.CurrentBelt)
			}
		}
		if packageID != nil {
			if sp, ok := m.studentPackages[*packageID]; ok {
				if tpl, ok2 := m.packageTemplates[sp.TemplateID]; ok2 {
					att.PackageTitle = tpl.Title
				}
			}
		}
	} else if attendeeType == "coach" && attendeeID != nil {
		if c, ok := m.coaches[*attendeeID]; ok {
			if att.AttendeeName == "" {
				att.AttendeeName = c.FullName
			}
			if att.AttendeeRole == "" {
				att.AttendeeRole = "Coach"
			}
		}
	} else if attendeeType == "admin" && attendeeID != nil {
		if u, ok := m.users[*attendeeID]; ok {
			if att.AttendeeName == "" {
				if u.DisplayName != "" {
					att.AttendeeName = u.DisplayName
				} else {
					att.AttendeeName = u.Email
				}
			}
			if att.AttendeeRole == "" {
				att.AttendeeRole = string(u.Role)
			}
		}
	}

	m.attendances[att.ID] = att
	return att, nil
}

func (m *MemoryStore) CheckInStudent(sessionID, studentID uuid.UUID, packageID *uuid.UUID, sessionRate *float64) (*models.Attendance, error) {
	return m.CheckInAttendee(sessionID, "student", &studentID, "", "", packageID, sessionRate)
}

func (m *MemoryStore) RemoveAttendance(sessionID, studentID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var targetID uuid.UUID
	var targetPkgID *uuid.UUID
	found := false
	for id, a := range m.attendances {
		if a.SessionID == sessionID && (a.StudentID == studentID || id == studentID) {
			targetID = id
			targetPkgID = a.StudentPackageID
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}

	// Refund deducted package session if applicable
	if targetPkgID != nil {
		if pkg, exists := m.studentPackages[*targetPkgID]; exists && pkg.RemainingSessions != nil {
			*pkg.RemainingSessions++
		}
	}

	delete(m.attendances, targetID)
	return nil
}

func (m *MemoryStore) GetAllAttendances() ([]*models.Attendance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*models.Attendance, 0, len(m.attendances))
	for _, a := range m.attendances {
		if st, ok := m.students[a.StudentID]; ok {
			a.StudentName = st.FullName
			a.StudentBelt = st.CurrentBelt
		}
		if a.StudentPackageID != nil {
			if sp, ok := m.studentPackages[*a.StudentPackageID]; ok {
				if tpl, ok2 := m.packageTemplates[sp.TemplateID]; ok2 {
					a.PackageTitle = tpl.Title
				}
			}
		}
		if a.LocationID != nil {
			if loc, ok := m.locations[*a.LocationID]; ok {
				a.LocationName = loc.Name
				a.LocationPin = loc.Pin
			}
		} else if sess, ok := m.sessions[a.SessionID]; ok && sess.LocationID != nil {
			a.LocationID = sess.LocationID
			if loc, ok := m.locations[*sess.LocationID]; ok {
				a.LocationName = loc.Name
				a.LocationPin = loc.Pin
			}
		}
		result = append(result, a)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CheckedInAt.After(result[j].CheckedInAt)
	})
	return result, nil
}

func (m *MemoryStore) GetSessionAttendanceCounts() (map[uuid.UUID]int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	counts := make(map[uuid.UUID]int)
	for _, a := range m.attendances {
		counts[a.SessionID]++
	}
	return counts, nil
}

func (m *MemoryStore) GetLatestEvaluation(studentID uuid.UUID) (*models.StudentEvaluation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var latest *models.StudentEvaluation
	for _, e := range m.evaluations {
		if e.StudentID == studentID {
			if latest == nil || e.EvaluationDate.After(latest.EvaluationDate) {
				if c, ok := m.coaches[e.CoachID]; ok {
					e.CoachName = c.FullName
				}
				latest = e
			}
		}
	}
	return latest, nil
}

func (m *MemoryStore) GetLatestEvaluations() (map[uuid.UUID]*models.StudentEvaluation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[uuid.UUID]*models.StudentEvaluation)
	for _, e := range m.evaluations {
		curr, exists := result[e.StudentID]
		if !exists || e.EvaluationDate.After(curr.EvaluationDate) ||
			(e.EvaluationDate.Equal(curr.EvaluationDate) && e.CreatedAt.After(curr.CreatedAt)) {
			if c, ok := m.coaches[e.CoachID]; ok {
				e.CoachName = c.FullName
			}
			result[e.StudentID] = e
		}
	}
	return result, nil
}

func (m *MemoryStore) GetStudentEvaluations(studentID uuid.UUID) ([]*models.StudentEvaluation, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := []*models.StudentEvaluation{}
	for _, e := range m.evaluations {
		if e.StudentID == studentID {
			if c, ok := m.coaches[e.CoachID]; ok {
				e.CoachName = c.FullName
			}
			result = append(result, e)
		}
	}
	return result, nil
}

func (m *MemoryStore) CreateEvaluation(eval *models.StudentEvaluation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if eval.ID == uuid.Nil {
		eval.ID = uuid.New()
	}
	eval.CreatedAt = time.Now()
	eval.SyncLegacyFields()
	m.evaluations[eval.ID] = eval
	return nil
}

func (m *MemoryStore) GetUserByEmail(email string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normEmail := strings.ToLower(strings.TrimSpace(email))
	id, ok := m.usersByEmail[normEmail]
	if !ok {
		return nil, ErrNotFound
	}
	u, ok := m.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (m *MemoryStore) GetUserByUsername(username string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	normUsername := strings.ToLower(strings.TrimSpace(username))
	if normUsername == "" {
		return nil, ErrNotFound
	}
	id, ok := m.usersByUsername[normUsername]
	if !ok {
		return nil, ErrNotFound
	}
	u, ok := m.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (m *MemoryStore) GetUserByIdentifier(identifier string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	norm := strings.ToLower(strings.TrimSpace(identifier))
	if norm == "" {
		return nil, ErrNotFound
	}

	// Try email first
	if id, ok := m.usersByEmail[norm]; ok {
		if u, exists := m.users[id]; exists {
			return u, nil
		}
	}

	// Try username
	if id, ok := m.usersByUsername[norm]; ok {
		if u, exists := m.users[id]; exists {
			return u, nil
		}
	}

	return nil, ErrNotFound
}

func (m *MemoryStore) GetUserByID(id uuid.UUID) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (m *MemoryStore) GetUserByStudentID(studentID uuid.UUID) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, u := range m.users {
		if u.StudentID != nil && *u.StudentID == studentID {
			return u, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MemoryStore) GetUserByCoachID(coachID uuid.UUID) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, u := range m.users {
		if u.CoachID != nil && *u.CoachID == coachID {
			return u, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MemoryStore) GetUsersByRole(role models.UserRole) ([]*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*models.User
	for _, u := range m.users {
		if u.Role == role {
			result = append(result, u)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})
	return result, nil
}

func (m *MemoryStore) CreateUser(u *models.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	normEmail := strings.ToLower(strings.TrimSpace(u.Email))
	u.Email = normEmail

	normUsername := strings.ToLower(strings.TrimSpace(u.Username))
	u.Username = normUsername

	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	m.users[u.ID] = u
	m.usersByEmail[normEmail] = u.ID
	if normUsername != "" {
		m.usersByUsername[normUsername] = u.ID
	}
	return nil
}

func (m *MemoryStore) UpdateUser(u *models.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.users[u.ID]
	if !ok {
		return ErrNotFound
	}

	normUsername := strings.ToLower(strings.TrimSpace(u.Username))
	if existing.Username != normUsername {
		if existing.Username != "" {
			delete(m.usersByUsername, existing.Username)
		}
		if normUsername != "" {
			m.usersByUsername[normUsername] = u.ID
		}
		existing.Username = normUsername
	}

	existing.PasswordHash = u.PasswordHash
	existing.IsActive = u.IsActive
	existing.DisplayName = u.DisplayName
	existing.ProfilePictureURL = u.ProfilePictureURL
	existing.UpdatedAt = time.Now()
	return nil
}

func (m *MemoryStore) ToggleUserActive(userID uuid.UUID, isActive bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.users[userID]
	if !ok {
		return ErrNotFound
	}
	existing.IsActive = isActive
	existing.UpdatedAt = time.Now()
	if !isActive {
		for token, rec := range m.sessionTokens {
			if rec.UserID == userID {
				delete(m.sessionTokens, token)
			}
		}
	}
	return nil
}

func (m *MemoryStore) DeleteUser(userID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	user, exists := m.users[userID]
	if !exists {
		return ErrNotFound
	}

	for token, rec := range m.sessionTokens {
		if rec.UserID == userID {
			delete(m.sessionTokens, token)
		}
	}
	for _, sess := range m.sessions {
		if sess.AdminID != nil && *sess.AdminID == userID {
			sess.AdminID = nil
			sess.AdminName = ""
		}
	}
	delete(m.usersByEmail, user.Email)
	delete(m.users, userID)
	return nil
}

func (m *MemoryStore) UpdateUserLastLogin(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	u, ok := m.users[id]
	if !ok {
		return ErrNotFound
	}
	now := time.Now()
	u.LastLoginAt = &now
	u.UpdatedAt = now
	return nil
}

func (m *MemoryStore) CreateSessionToken(token string, userID uuid.UUID, expiresAt time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.sessionTokens[token] = SessionTokenRecord{
		Token:     token,
		UserID:    userID,
		ExpiresAt: expiresAt,
	}
	return nil
}

func (m *MemoryStore) GetUserBySessionToken(token string) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rec, ok := m.sessionTokens[token]
	if !ok {
		return nil, ErrNotFound
	}
	if time.Now().After(rec.ExpiresAt) {
		return nil, ErrNotFound
	}

	u, ok := m.users[rec.UserID]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (m *MemoryStore) DeleteSessionToken(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessionTokens, token)
	return nil
}

func (m *MemoryStore) CreatePasswordResetToken(t *models.PasswordResetToken) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	if t.CreatedAt.IsZero() {
		t.CreatedAt = time.Now()
	}
	m.passwordResetTokens[t.Token] = t
	return nil
}

func (m *MemoryStore) GetPasswordResetToken(token string) (*models.PasswordResetToken, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.passwordResetTokens[token]
	if !ok {
		return nil, ErrNotFound
	}
	return t, nil
}

func (m *MemoryStore) MarkPasswordResetTokenUsed(token string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.passwordResetTokens[token]
	if !ok {
		return ErrNotFound
	}
	now := time.Now()
	t.UsedAt = &now
	return nil
}

func (m *MemoryStore) CreateSafetyIncident(inc *models.SafetyIncident) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if inc.ID == uuid.Nil {
		inc.ID = uuid.New()
	}
	inc.CreatedAt = time.Now()
	m.safetyIncidents[inc.ID] = inc

	// Auto-set safety flag on student
	if st, ok := m.students[inc.StudentID]; ok {
		st.HasSafetyFlag = true
		inc.StudentName = st.FullName
	}
	if inc.CoachID != nil {
		if coach, ok := m.coaches[*inc.CoachID]; ok {
			inc.CoachName = coach.FullName
		}
	}
	return nil
}

func (m *MemoryStore) GetSafetyIncidents(resolved *bool) ([]*models.SafetyIncident, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := []*models.SafetyIncident{}
	for _, inc := range m.safetyIncidents {
		if resolved != nil && inc.Resolved != *resolved {
			continue
		}
		if st, ok := m.students[inc.StudentID]; ok {
			inc.StudentName = st.FullName
		}
		if inc.CoachID != nil {
			if coach, ok := m.coaches[*inc.CoachID]; ok {
				inc.CoachName = coach.FullName
			}
		}
		result = append(result, inc)
	}
	return result, nil
}

func (m *MemoryStore) ResolveSafetyIncident(incidentID uuid.UUID, adminEmail string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	inc, ok := m.safetyIncidents[incidentID]
	if !ok {
		return ErrNotFound
	}
	inc.Resolved = true
	inc.ResolvedBy = adminEmail
	now := time.Now()
	inc.ResolvedAt = &now

	// Check if this student has any remaining unresolved incidents
	hasUnresolved := false
	for _, other := range m.safetyIncidents {
		if other.StudentID == inc.StudentID && !other.Resolved {
			hasUnresolved = true
			break
		}
	}
	if !hasUnresolved {
		if st, ok := m.students[inc.StudentID]; ok {
			st.HasSafetyFlag = false
		}
	}
	return nil
}

func (m *MemoryStore) SetStudentSafetyFlag(studentID uuid.UUID, hasSafetyFlag bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	st, ok := m.students[studentID]
	if !ok {
		return ErrNotFound
	}
	st.HasSafetyFlag = hasSafetyFlag
	return nil
}

func (m *MemoryStore) PromoteStudent(studentID uuid.UUID, newBelt models.BeltRank) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	st, ok := m.students[studentID]
	if !ok {
		return ErrNotFound
	}
	st.CurrentBelt = newBelt
	st.LastPromotionDate = time.Now()
	return nil
}

func (m *MemoryStore) GetAllLocations() ([]*models.Location, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*models.Location, 0, len(m.locations))
	for _, l := range m.locations {
		result = append(result, l)
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result, nil
}

func (m *MemoryStore) GetLocationByID(id uuid.UUID) (*models.Location, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	l, ok := m.locations[id]
	if !ok {
		return nil, ErrNotFound
	}
	return l, nil
}

func (m *MemoryStore) CreateLocation(loc *models.Location) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if loc.ID == uuid.Nil {
		loc.ID = uuid.New()
	}
	if loc.CreatedAt.IsZero() {
		loc.CreatedAt = time.Now()
	}
	m.locations[loc.ID] = loc
	return nil
}

func (m *MemoryStore) UpdateLocation(loc *models.Location) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.locations[loc.ID]
	if !ok {
		return ErrNotFound
	}
	existing.Name = loc.Name
	existing.Pin = loc.Pin
	existing.FixedRate = loc.FixedRate
	return nil
}

func (m *MemoryStore) DeleteLocation(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.locations[id]; !ok {
		return ErrNotFound
	}
	delete(m.locations, id)
	return nil
}

func (m *MemoryStore) GetAllTrainingCategories() ([]*models.TrainingCategory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*models.TrainingCategory, 0, len(m.trainingCategories))
	for _, c := range m.trainingCategories {
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool {
		return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name)
	})
	return result, nil
}

func (m *MemoryStore) GetTrainingCategoryByID(id uuid.UUID) (*models.TrainingCategory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	c, ok := m.trainingCategories[id]
	if !ok {
		return nil, ErrNotFound
	}
	return c, nil
}

func (m *MemoryStore) GetTrainingCategoryByName(name string) (*models.TrainingCategory, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cleanName := strings.ToLower(strings.TrimSpace(name))
	for _, c := range m.trainingCategories {
		if strings.ToLower(c.Name) == cleanName {
			return c, nil
		}
	}
	return nil, ErrNotFound
}

func (m *MemoryStore) CreateTrainingCategory(cat *models.TrainingCategory) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cleanName := strings.ToLower(strings.TrimSpace(cat.Name))
	for _, existing := range m.trainingCategories {
		if strings.ToLower(existing.Name) == cleanName {
			return errors.New("training category with this name already exists")
		}
	}

	if cat.ID == uuid.Nil {
		cat.ID = uuid.New()
	}
	if cat.CreatedAt.IsZero() {
		cat.CreatedAt = time.Now()
	}
	if strings.TrimSpace(cat.Color) == "" {
		cat.Color = "#990303"
	}
	m.trainingCategories[cat.ID] = cat
	return nil
}

func (m *MemoryStore) UpdateTrainingCategory(cat *models.TrainingCategory) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.trainingCategories[cat.ID]
	if !ok {
		return ErrNotFound
	}

	cleanName := strings.ToLower(strings.TrimSpace(cat.Name))
	for id, other := range m.trainingCategories {
		if id != cat.ID && strings.ToLower(other.Name) == cleanName {
			return errors.New("training category with this name already exists")
		}
	}

	oldName := existing.Name
	existing.Name = strings.TrimSpace(cat.Name)
	if strings.TrimSpace(cat.Color) != "" {
		existing.Color = strings.TrimSpace(cat.Color)
	}

	// Update any existing sessions if name changed
	if oldName != existing.Name {
		for _, s := range m.sessions {
			if string(s.TrainingType) == oldName {
				s.TrainingType = models.TrainingType(existing.Name)
			}
		}
	}

	return nil
}

func (m *MemoryStore) DeleteTrainingCategory(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.trainingCategories[id]
	if !ok {
		return ErrNotFound
	}

	// Check if any sessions are using this category
	for _, s := range m.sessions {
		if string(s.TrainingType) == existing.Name {
			return errors.New("cannot delete training category that is assigned to existing sessions")
		}
	}

	delete(m.trainingCategories, id)
	return nil
}

// Audit Logs
func (m *MemoryStore) CreateAuditLog(entry *models.AuditLog) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	// Prepend for newest-first order
	m.auditLogs = append([]*models.AuditLog{entry}, m.auditLogs...)
	return nil
}

func (m *MemoryStore) GetAuditLogs(filter models.AuditLogFilter) ([]*models.AuditLog, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var filtered []*models.AuditLog
	searchLower := strings.ToLower(strings.TrimSpace(filter.Search))
	catFilter := strings.ToUpper(strings.TrimSpace(filter.Category))
	roleFilter := strings.ToUpper(strings.TrimSpace(filter.Role))
	actionFilter := strings.ToUpper(strings.TrimSpace(filter.Action))

	for _, entry := range m.auditLogs {
		if catFilter != "" && catFilter != "ALL" && string(entry.Category) != catFilter {
			continue
		}
		if roleFilter != "" && roleFilter != "ALL" && string(entry.ActorRole) != roleFilter {
			continue
		}
		if actionFilter != "" && !strings.Contains(strings.ToUpper(entry.Action), actionFilter) {
			continue
		}
		if filter.StartDate != "" {
			entryDate := entry.CreatedAt.Format("2006-01-02")
			if entryDate < filter.StartDate {
				continue
			}
		}
		if filter.EndDate != "" {
			entryDate := entry.CreatedAt.Format("2006-01-02")
			if entryDate > filter.EndDate {
				continue
			}
		}
		if searchLower != "" {
			match := strings.Contains(strings.ToLower(entry.ActorName), searchLower) ||
				strings.Contains(strings.ToLower(entry.ActorEmail), searchLower) ||
				strings.Contains(strings.ToLower(entry.Action), searchLower) ||
				strings.Contains(strings.ToLower(entry.TargetName), searchLower) ||
				strings.Contains(strings.ToLower(entry.Description), searchLower) ||
				strings.Contains(strings.ToLower(entry.IPAddress), searchLower)
			if !match {
				continue
			}
		}
		filtered = append(filtered, entry)
	}

	total := len(filtered)
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}
	if offset >= total {
		return []*models.AuditLog{}, total, nil
	}

	end := total
	if filter.Limit > 0 && offset+filter.Limit < end {
		end = offset + filter.Limit
	}

	page := make([]*models.AuditLog, end-offset)
	copy(page, filtered[offset:end])
	return page, total, nil
}

func (m *MemoryStore) GetAuditTelemetry() (*models.AuditTelemetry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	todayStr := time.Now().Format("2006-01-02")
	activeUsersMap := make(map[string]bool)
	var todayCount int
	var secCount int

	for _, entry := range m.auditLogs {
		if entry.CreatedAt.Format("2006-01-02") == todayStr {
			todayCount++
			if entry.ActorEmail != "" {
				activeUsersMap[entry.ActorEmail] = true
			} else if entry.UserID != nil {
				activeUsersMap[entry.UserID.String()] = true
			}
		}
		act := strings.ToUpper(entry.Action)
		if entry.Category == models.AuditCategoryAuth ||
			entry.Category == models.AuditCategorySafety ||
			strings.Contains(act, "DELETE") ||
			strings.Contains(act, "RESET") ||
			strings.Contains(act, "REVOKE") ||
			strings.Contains(act, "DEACTIVATE") {
			secCount++
		}
	}

	return &models.AuditTelemetry{
		TotalLogs:       len(m.auditLogs),
		TodayLogs:       todayCount,
		ActiveUsers:     len(activeUsersMap),
		SecurityActions: secCount,
	}, nil
}

func (m *MemoryStore) GetNotificationSettings() (*models.NotificationSettings, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.notificationSettings == nil {
		m.notificationSettings = models.DefaultNotificationSettings()
	}
	// Return a copy
	cp := *m.notificationSettings
	return &cp, nil
}

func (m *MemoryStore) UpdateNotificationSettings(settings *models.NotificationSettings) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if settings == nil {
		return errors.New("notification settings cannot be nil")
	}
	settings.UpdatedAt = time.Now()
	cp := *settings
	m.notificationSettings = &cp
	return nil
}

func (m *MemoryStore) GetUserNotificationPreferences(userID uuid.UUID) (*models.UserNotificationPreferences, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	prefs, ok := m.userPreferences[userID]
	if !ok {
		return models.DefaultUserPreferences(userID), nil
	}
	cp := *prefs
	return &cp, nil
}

func (m *MemoryStore) UpdateUserNotificationPreferences(prefs *models.UserNotificationPreferences) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if prefs == nil {
		return errors.New("user preferences cannot be nil")
	}
	prefs.UpdatedAt = time.Now()
	cp := *prefs
	m.userPreferences[prefs.UserID] = &cp
	return nil
}

func (m *MemoryStore) CreateNotification(n *models.Notification) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if n == nil {
		return errors.New("notification cannot be nil")
	}
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}

	cp := *n
	m.notifications = append([]*models.Notification{&cp}, m.notifications...)
	return nil
}

func (m *MemoryStore) GetNotifications(filter models.NotificationFilter) ([]*models.Notification, int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matched []*models.Notification
	for _, n := range m.notifications {
		if filter.UserID != nil {
			if n.UserID == nil {
				if filter.Role == nil || n.RecipientRole == "" || n.RecipientRole != *filter.Role {
					continue
				}
			} else if *n.UserID != *filter.UserID {
				continue
			}
		} else if filter.Role != nil && n.RecipientRole != "" && n.RecipientRole != *filter.Role {
			continue
		}
		if filter.Channel != nil && n.Channel != *filter.Channel {
			continue
		}
		if filter.EventType != nil && n.EventType != *filter.EventType {
			continue
		}
		if filter.UnreadOnly && n.IsRead {
			continue
		}
		cp := *n
		matched = append(matched, &cp)
	}

	total := len(matched)
	if filter.Offset >= total {
		return []*models.Notification{}, total, nil
	}
	end := total
	if filter.Limit > 0 && filter.Offset+filter.Limit < end {
		end = filter.Offset + filter.Limit
	}
	return matched[filter.Offset:end], total, nil
}

func (m *MemoryStore) GetUnreadNotificationCount(userID *uuid.UUID, role *models.UserRole) (int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	count := 0
	for _, n := range m.notifications {
		if n.IsRead {
			continue
		}
		if userID != nil {
			if n.UserID == nil {
				if role == nil || n.RecipientRole == "" || n.RecipientRole != *role {
					continue
				}
			} else if *n.UserID != *userID {
				continue
			}
		} else if role != nil && n.RecipientRole != "" && n.RecipientRole != *role {
			continue
		}
		count++
	}
	return count, nil
}

func (m *MemoryStore) MarkNotificationRead(id uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, n := range m.notifications {
		if n.ID == id {
			n.IsRead = true
			return nil
		}
	}
	return ErrNotFound
}

func (m *MemoryStore) MarkAllNotificationsRead(userID *uuid.UUID, role *models.UserRole) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, n := range m.notifications {
		if userID != nil && n.UserID != nil && *n.UserID != *userID {
			continue
		}
		if role != nil && n.RecipientRole != "" && n.RecipientRole != *role {
			continue
		}
		n.IsRead = true
	}
	return nil
}

