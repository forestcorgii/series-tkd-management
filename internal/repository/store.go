package repository

import (
	"errors"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
)

var (
	ErrNotFound      = errors.New("record not found")
	ErrAlreadyInRoster = errors.New("student already checked in for this session")
)

type RepositoryStore interface {
	// Students
	GetAllStudents() ([]*models.Student, error)
	GetStudentByID(id uuid.UUID) (*models.Student, error)
	SearchStudents(query string) ([]*models.Student, error)
	CreateStudent(s *models.Student) error
	UpdateStudent(s *models.Student) error

	// Coaches
	GetAllCoaches() ([]*models.Coach, error)
	GetCoachByID(id uuid.UUID) (*models.Coach, error)
	CreateCoach(c *models.Coach) error

	// Packages
	GetPackageTemplates() ([]*models.PackageTemplate, error)
	GetStudentPackages(studentID uuid.UUID) ([]*models.StudentPackage, error)
	AssignPackage(pkg *models.StudentPackage) error
	UpdateStudentPackage(pkg *models.StudentPackage) error

	// Sessions & Floor Attendance
	GetAllSessions() ([]*models.TrainingSession, error)
	GetSessionByID(id uuid.UUID) (*models.TrainingSession, error)
	CreateSession(sess *models.TrainingSession) error
	GetSessionAttendances(sessionID uuid.UUID) ([]*models.Attendance, error)
	GetStudentAttendances(studentID uuid.UUID) ([]*models.Attendance, error)
	CheckInStudent(sessionID, studentID uuid.UUID, packageID *uuid.UUID) (*models.Attendance, error)

	// Evaluations
	GetLatestEvaluation(studentID uuid.UUID) (*models.StudentEvaluation, error)
	GetStudentEvaluations(studentID uuid.UUID) ([]*models.StudentEvaluation, error)
	CreateEvaluation(eval *models.StudentEvaluation) error

	// Auth & Users
	GetUserByEmail(email string) (*models.User, error)
	GetUserByID(id uuid.UUID) (*models.User, error)
	CreateUser(user *models.User) error
	UpdateUserLastLogin(id uuid.UUID) error
	CreateSessionToken(token string, userID uuid.UUID, expiresAt time.Time) error
	GetUserBySessionToken(token string) (*models.User, error)
	DeleteSessionToken(token string) error

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
	mu               sync.RWMutex
	students         map[uuid.UUID]*models.Student
	coaches          map[uuid.UUID]*models.Coach
	packageTemplates map[uuid.UUID]*models.PackageTemplate
	studentPackages  map[uuid.UUID]*models.StudentPackage
	sessions         map[uuid.UUID]*models.TrainingSession
	attendances      map[uuid.UUID]*models.Attendance
	evaluations      map[uuid.UUID]*models.StudentEvaluation
	users            map[uuid.UUID]*models.User
	usersByEmail     map[string]uuid.UUID
	sessionTokens    map[string]SessionTokenRecord
	safetyIncidents  map[uuid.UUID]*models.SafetyIncident
}

func NewMemoryStore() *MemoryStore {
	m := &MemoryStore{
		students:         make(map[uuid.UUID]*models.Student),
		coaches:          make(map[uuid.UUID]*models.Coach),
		packageTemplates: make(map[uuid.UUID]*models.PackageTemplate),
		studentPackages:  make(map[uuid.UUID]*models.StudentPackage),
		sessions:         make(map[uuid.UUID]*models.TrainingSession),
		attendances:      make(map[uuid.UUID]*models.Attendance),
		evaluations:      make(map[uuid.UUID]*models.StudentEvaluation),
		users:            make(map[uuid.UUID]*models.User),
		usersByEmail:     make(map[string]uuid.UUID),
		sessionTokens:    make(map[string]SessionTokenRecord),
		safetyIncidents:  make(map[uuid.UUID]*models.SafetyIncident),
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

func (m *MemoryStore) GetPackageTemplates() ([]*models.PackageTemplate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*models.PackageTemplate, 0, len(m.packageTemplates))
	for _, pt := range m.packageTemplates {
		result = append(result, pt)
	}
	return result, nil
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
		if coach, ok := m.coaches[s.CoachID]; ok {
			s.CoachName = coach.FullName
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
	if coach, ok := m.coaches[s.CoachID]; ok {
		s.CoachName = coach.FullName
	}
	return s, nil
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
			result = append(result, a)
		}
	}
	return result, nil
}

func (m *MemoryStore) CheckInStudent(sessionID, studentID uuid.UUID, packageID *uuid.UUID) (*models.Attendance, error) {
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
		CheckedInAt:      time.Now(),
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

func (m *MemoryStore) GetUserByID(id uuid.UUID) (*models.User, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	u, ok := m.users[id]
	if !ok {
		return nil, ErrNotFound
	}
	return u, nil
}

func (m *MemoryStore) CreateUser(u *models.User) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	normEmail := strings.ToLower(strings.TrimSpace(u.Email))
	u.Email = normEmail
	now := time.Now()
	u.CreatedAt = now
	u.UpdatedAt = now
	m.users[u.ID] = u
	m.usersByEmail[normEmail] = u.ID
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
