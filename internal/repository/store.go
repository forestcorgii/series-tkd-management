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
}

type RepositoryStore interface {
	// Locations
	GetAllLocations() ([]*models.Location, error)
	GetLocationByID(id uuid.UUID) (*models.Location, error)
	CreateLocation(loc *models.Location) error
	UpdateLocation(loc *models.Location) error
	DeleteLocation(id uuid.UUID) error

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
	AssignPackage(pkg *models.StudentPackage) error
	UpdateStudentPackage(pkg *models.StudentPackage) error

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
	RemoveAttendance(sessionID, studentID uuid.UUID) error

	// Evaluations
	GetLatestEvaluation(studentID uuid.UUID) (*models.StudentEvaluation, error)
	GetStudentEvaluations(studentID uuid.UUID) ([]*models.StudentEvaluation, error)
	CreateEvaluation(eval *models.StudentEvaluation) error

	// Auth & Users
	GetUserByEmail(email string) (*models.User, error)
	GetUserByUsername(username string) (*models.User, error)
	GetUserByIdentifier(identifier string) (*models.User, error)
	GetUserByID(id uuid.UUID) (*models.User, error)
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

func (m *MemoryStore) UpdateStudentPackage(pkg *models.StudentPackage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.studentPackages[pkg.ID]; !ok {
		return ErrNotFound
	}
	m.studentPackages[pkg.ID] = pkg
	return nil
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
		if filter.CoachID != nil && (s.CoachID == nil || *s.CoachID != *filter.CoachID) {
			continue
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

func (m *MemoryStore) CheckInStudent(sessionID, studentID uuid.UUID, packageID *uuid.UUID, sessionRate *float64) (*models.Attendance, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already checked in
	for _, a := range m.attendances {
		if a.SessionID == sessionID && a.StudentID == studentID {
			return nil, ErrAlreadyInRoster
		}
	}

	att := &models.Attendance{
		ID:               uuid.New(),
		SessionID:        sessionID,
		StudentID:        studentID,
		StudentPackageID: packageID,
		SessionRate:      sessionRate,
		CheckedInAt:      time.Now(),
	}

	if sess, ok := m.sessions[sessionID]; ok && sess.LocationID != nil {
		att.LocationID = sess.LocationID
		if loc, okLoc := m.locations[*sess.LocationID]; okLoc {
			att.LocationName = loc.Name
			att.LocationPin = loc.Pin
		}
	}

	if st, ok := m.students[studentID]; ok {
		att.StudentName = st.FullName
		att.StudentBelt = st.CurrentBelt
	}
	if packageID != nil {
		if sp, ok := m.studentPackages[*packageID]; ok {
			if tpl, ok2 := m.packageTemplates[sp.TemplateID]; ok2 {
				att.PackageTitle = tpl.Title
			}
		}
	}

	m.attendances[att.ID] = att
	return att, nil
}

func (m *MemoryStore) RemoveAttendance(sessionID, studentID uuid.UUID) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var targetID uuid.UUID
	var targetPkgID *uuid.UUID
	found := false
	for id, a := range m.attendances {
		if a.SessionID == sessionID && a.StudentID == studentID {
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

