package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

var (
	ErrInvalidCredentials     = errors.New("invalid email or password")
	ErrUserInactive           = errors.New("user account is inactive")
	ErrAccountPendingApproval = models.ErrAccountPendingApproval
	ErrEmailAlreadyExists     = errors.New("a user with this email already exists")
	ErrUsernameAlreadyExists  = models.ErrUsernameAlreadyExists
	ErrInvalidResetToken      = models.ErrInvalidResetToken
)

type AuthService struct {
	store repository.RepositoryStore
}

func NewAuthService(store repository.RepositoryStore) *AuthService {
	return &AuthService{
		store: store,
	}
}

func (s *AuthService) GenerateSecureToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate secure random token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

func (s *AuthService) Login(identifier, password string) (*models.User, string, error) {
	norm := strings.ToLower(strings.TrimSpace(identifier))
	if norm == "" {
		return nil, "", ErrInvalidCredentials
	}

	user, err := s.store.GetUserByIdentifier(norm)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, "", ErrInvalidCredentials
		}
		return nil, "", err
	}

	if !user.CheckPassword(password) {
		return nil, "", ErrInvalidCredentials
	}

	if !user.IsActive {
		if user.LastLoginAt == nil {
			return nil, "", ErrAccountPendingApproval
		}
		return nil, "", ErrUserInactive
	}

	if user.IsCoach() {
		var coach *models.Coach
		if user.CoachID != nil {
			coach, _ = s.store.GetCoachByID(*user.CoachID)
		} else {
			coaches, _ := s.store.GetAllCoaches()
			for _, c := range coaches {
				if strings.EqualFold(c.Email, user.Email) {
					coach = c
					break
				}
			}
		}
		if coach != nil && !coach.IsActive {
			if user.LastLoginAt == nil {
				return nil, "", ErrAccountPendingApproval
			}
			return nil, "", ErrUserInactive
		}
	}

	if user.IsStudent() && user.StudentID != nil {
		if student, err := s.store.GetStudentByID(*user.StudentID); err == nil && student != nil && !student.IsActive {
			if user.LastLoginAt == nil {
				return nil, "", ErrAccountPendingApproval
			}
			return nil, "", ErrUserInactive
		}
	}

	token, err := s.GenerateSecureToken()
	if err != nil {
		return nil, "", err
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7-day session
	if err := s.store.CreateSessionToken(token, user.ID, expiresAt); err != nil {
		return nil, "", fmt.Errorf("failed to persist session: %w", err)
	}

	_ = s.store.UpdateUserLastLogin(user.ID)

	return user, token, nil
}

// CreateSessionForUser generates and persists a new 7-day session token for the given user.
func (s *AuthService) CreateSessionForUser(userID uuid.UUID) (string, error) {
	token, err := s.GenerateSecureToken()
	if err != nil {
		return "", err
	}
	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	if err := s.store.CreateSessionToken(token, userID, expiresAt); err != nil {
		return "", err
	}
	return token, nil
}

func (s *AuthService) ValidateSession(token string) (*models.User, error) {
	if token == "" {
		return nil, repository.ErrNotFound
	}
	user, err := s.store.GetUserBySessionToken(token)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrUserInactive
	}
	if user.IsCoach() {
		var coach *models.Coach
		if user.CoachID != nil {
			var coachErr error
			coach, coachErr = s.store.GetCoachByID(*user.CoachID)
			if coachErr != nil && !errors.Is(coachErr, repository.ErrNotFound) {
				return nil, coachErr
			}
		} else {
			coaches, coachErr := s.store.GetAllCoaches()
			if coachErr != nil {
				return nil, coachErr
			}
			for _, c := range coaches {
				if strings.EqualFold(c.Email, user.Email) {
					coach = c
					break
				}
			}
		}
		if coach != nil && !coach.IsActive {
			return nil, ErrUserInactive
		}
	}
	if user.IsStudent() && user.StudentID != nil {
		student, stErr := s.store.GetStudentByID(*user.StudentID)
		if stErr != nil && !errors.Is(stErr, repository.ErrNotFound) {
			return nil, stErr
		}
		if student != nil && !student.IsActive {
			return nil, ErrUserInactive
		}
	}
	return user, nil
}

func (s *AuthService) Logout(token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSessionToken(token)
}

func (s *AuthService) RegisterUser(email, password string, role models.UserRole, studentID, coachID *uuid.UUID, username ...string) (*models.User, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	if normEmail == "" || password == "" {
		return nil, errors.New("email and password cannot be empty")
	}

	// Check if email already exists
	if existing, _ := s.store.GetUserByEmail(normEmail); existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	var normUsername string
	if len(username) > 0 && username[0] != "" {
		normUsername = strings.ToLower(strings.TrimSpace(username[0]))
		if existing, _ := s.store.GetUserByUsername(normUsername); existing != nil {
			return nil, ErrUsernameAlreadyExists
		}
	}

	u := &models.User{
		ID:        uuid.New(),
		Email:     normEmail,
		Username:  normUsername,
		Role:      role,
		StudentID: studentID,
		CoachID:   coachID,
		IsActive:  true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := u.SetPassword(password); err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.store.CreateUser(u); err != nil {
		return nil, fmt.Errorf("failed to create user record: %w", err)
	}

	return u, nil
}

// RegisterPendingCoach registers a new coach applicant with IsActive = false pending manager approval.
func (s *AuthService) RegisterPendingCoach(fullName, email, username, password, phone, beltRank string, specialties []string, firstAid bool) (*models.Coach, *models.User, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	trimmedName := strings.TrimSpace(fullName)
	trimmedPhone := strings.TrimSpace(phone)
	trimmedRank := strings.TrimSpace(beltRank)

	if trimmedName == "" {
		return nil, nil, errors.New("full name is required")
	}
	if normEmail == "" || !strings.Contains(normEmail, "@") {
		return nil, nil, errors.New("a valid email address is required")
	}
	if len(password) < 6 {
		return nil, nil, errors.New("password must be at least 6 characters long")
	}
	if trimmedPhone == "" {
		return nil, nil, errors.New("phone number is required for coach registration")
	}
	if trimmedRank == "" {
		trimmedRank = "1st Dan Black Belt"
	}

	// Check if email already exists in users
	if existing, _ := s.store.GetUserByEmail(normEmail); existing != nil {
		return nil, nil, ErrEmailAlreadyExists
	}
	// Check if email exists in coaches
	if coaches, err := s.store.GetAllCoaches(); err == nil {
		for _, c := range coaches {
			if strings.EqualFold(c.Email, normEmail) {
				return nil, nil, ErrEmailAlreadyExists
			}
		}
	}

	var normUsername string
	if username != "" {
		normUsername = strings.ToLower(strings.TrimSpace(username))
		if existing, _ := s.store.GetUserByUsername(normUsername); existing != nil {
			return nil, nil, ErrUsernameAlreadyExists
		}
	}

	coach := &models.Coach{
		ID:                uuid.New(),
		FullName:          trimmedName,
		Email:             normEmail,
		Phone:             trimmedPhone,
		BeltRank:          trimmedRank,
		Specialties:       specialties,
		FirstAidCertified: firstAid,
		RatePerSession:    0.0,
		IsActive:          false, // Pending manager approval
		CreatedAt:         time.Now(),
	}

	if err := s.store.CreateCoach(coach); err != nil {
		return nil, nil, fmt.Errorf("failed to save coach profile: %w", err)
	}

	user := &models.User{
		ID:          uuid.New(),
		Email:       normEmail,
		Username:    normUsername,
		Role:        models.RoleStaff,
		DisplayName: trimmedName,
		CoachID:     &coach.ID,
		IsActive:    false, // Pending manager approval
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := user.SetPassword(password); err != nil {
		_ = s.store.DeleteCoach(coach.ID)
		return nil, nil, fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.store.CreateUser(user); err != nil {
		_ = s.store.DeleteCoach(coach.ID)
		return nil, nil, fmt.Errorf("failed to create user account: %w", err)
	}

	if chRole, err := s.store.GetRoleBySlug("coach"); err == nil && chRole != nil {
		_ = s.store.AssignUserRoles(user.ID, []uuid.UUID{chRole.ID})
	}

	return coach, user, nil
}

// RegisterPendingAdmin registers a new administrator applicant with IsActive = false pending manager approval.
func (s *AuthService) RegisterPendingAdmin(fullName, email, username, password string) (*models.User, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	trimmedName := strings.TrimSpace(fullName)

	if trimmedName == "" {
		return nil, errors.New("full name is required")
	}
	if normEmail == "" || !strings.Contains(normEmail, "@") {
		return nil, errors.New("a valid email address is required")
	}
	if len(password) < 6 {
		return nil, errors.New("password must be at least 6 characters long")
	}

	// Check if email already exists
	if existing, _ := s.store.GetUserByEmail(normEmail); existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	var normUsername string
	if username != "" {
		normUsername = strings.ToLower(strings.TrimSpace(username))
		if existing, _ := s.store.GetUserByUsername(normUsername); existing != nil {
			return nil, ErrUsernameAlreadyExists
		}
	}

	user := &models.User{
		ID:          uuid.New(),
		Email:       normEmail,
		Username:    normUsername,
		Role:        models.RoleStaff,
		DisplayName: trimmedName,
		IsActive:    false, // Pending manager approval
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := user.SetPassword(password); err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.store.CreateUser(user); err != nil {
		return nil, fmt.Errorf("failed to create user record: %w", err)
	}

	if admRole, err := s.store.GetRoleBySlug("admin"); err == nil && admRole != nil {
		_ = s.store.AssignUserRoles(user.ID, []uuid.UUID{admRole.ID})
	}

	return user, nil
}

func (s *AuthService) RequestPasswordReset(identifier string) (*models.PasswordResetToken, error) {
	norm := strings.ToLower(strings.TrimSpace(identifier))
	if norm == "" {
		return nil, errors.New("email or username cannot be empty")
	}

	user, err := s.store.GetUserByIdentifier(norm)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			// Anti-enumeration: return nil without error
			return nil, nil
		}
		return nil, err
	}

	if !user.IsActive {
		return nil, ErrUserInactive
	}

	rawToken, err := s.GenerateSecureToken()
	if err != nil {
		return nil, err
	}

	token := &models.PasswordResetToken{
		ID:        uuid.New(),
		UserID:    user.ID,
		Token:     rawToken,
		ExpiresAt: time.Now().Add(1 * time.Hour), // 1 hour validity
		CreatedAt: time.Now(),
	}

	if err := s.store.CreatePasswordResetToken(token); err != nil {
		return nil, fmt.Errorf("failed to save password reset token: %w", err)
	}

	log.Printf("🔑 [PASSWORD RESET] Reset token generated for user %s (%s)", user.Email, user.Username)

	return token, nil
}

func (s *AuthService) ValidatePasswordResetToken(rawToken string) (*models.PasswordResetToken, *models.User, error) {
	tokenStr := strings.TrimSpace(rawToken)
	if tokenStr == "" {
		return nil, nil, ErrInvalidResetToken
	}

	resetToken, err := s.store.GetPasswordResetToken(tokenStr)
	if err != nil || !resetToken.IsValid() {
		return nil, nil, ErrInvalidResetToken
	}

	user, err := s.store.GetUserByID(resetToken.UserID)
	if err != nil {
		return nil, nil, ErrInvalidResetToken
	}

	return resetToken, user, nil
}

func (s *AuthService) ResetPassword(rawToken, newPassword string) (*models.User, error) {
	tokenStr := strings.TrimSpace(rawToken)
	if tokenStr == "" {
		return nil, ErrInvalidResetToken
	}
	if len(newPassword) < 6 {
		return nil, errors.New("password must be at least 6 characters")
	}

	resetToken, user, err := s.ValidatePasswordResetToken(tokenStr)
	if err != nil {
		return nil, err
	}
	if !user.IsActive {
		return nil, ErrUserInactive
	}

	// Mark token used first to prevent concurrent reuse
	if err := s.store.MarkPasswordResetTokenUsed(resetToken.Token); err != nil {
		return nil, fmt.Errorf("failed to mark token as used: %w", err)
	}

	if err := user.SetPassword(newPassword); err != nil {
		return nil, fmt.Errorf("failed to hash new password: %w", err)
	}

	if err := s.store.UpdateUser(user); err != nil {
		return nil, fmt.Errorf("failed to update user password: %w", err)
	}

	_ = s.store.DeleteUserSessions(user.ID)

	log.Printf("✅ [PASSWORD RESET] Password successfully reset for user %s (%s)", user.Email, user.Username)
	return user, nil
}

// StudentRegistrationInput holds the fields submitted on the public student registration page.
type StudentRegistrationInput struct {
	FullName          string
	DOB               time.Time
	Gender            string
	Phone             string
	CurrentBelt       models.BeltRank
	Email             string
	Username          string
	Password          string
	EmergencyName     string
	EmergencyPhone    string
	EmergencyRelation string
	MedicalNotes      string
	WaiverAccepted    bool
	WaiverSignedName  string
}

// RegisterPendingStudent registers a new student applicant with IsActive = false pending manager/admin approval.
func (s *AuthService) RegisterPendingStudent(in StudentRegistrationInput) (*models.Student, *models.User, error) {
	normEmail := strings.ToLower(strings.TrimSpace(in.Email))
	trimmedName := strings.TrimSpace(in.FullName)
	trimmedPhone := strings.TrimSpace(in.Phone)
	trimmedEmName := strings.TrimSpace(in.EmergencyName)
	trimmedEmPhone := strings.TrimSpace(in.EmergencyPhone)
	trimmedEmRel := strings.TrimSpace(in.EmergencyRelation)
	trimmedSig := strings.TrimSpace(in.WaiverSignedName)

	if trimmedName == "" {
		return nil, nil, errors.New("full name is required")
	}
	if normEmail == "" || !strings.Contains(normEmail, "@") {
		return nil, nil, errors.New("a valid email address is required")
	}
	if len(in.Password) < 6 {
		return nil, nil, errors.New("password must be at least 6 characters long")
	}
	if trimmedPhone == "" {
		return nil, nil, errors.New("contact number is required")
	}
	if trimmedEmName == "" || trimmedEmPhone == "" || trimmedEmRel == "" {
		return nil, nil, errors.New("emergency contact name, phone, and relation are required")
	}
	if !in.WaiverAccepted {
		return nil, nil, errors.New("you must read and agree to the liability waiver")
	}
	if trimmedSig == "" {
		return nil, nil, errors.New("typed waiver signature (student or parent/guardian name) is required")
	}

	if existing, _ := s.store.GetUserByEmail(normEmail); existing != nil {
		return nil, nil, ErrEmailAlreadyExists
	}

	var normUsername string
	if in.Username != "" {
		normUsername = strings.ToLower(strings.TrimSpace(in.Username))
		if existing, _ := s.store.GetUserByUsername(normUsername); existing != nil {
			return nil, nil, ErrUsernameAlreadyExists
		}
	}

	dob := in.DOB
	if dob.IsZero() {
		dob = time.Now().AddDate(-10, 0, 0)
	}
	belt := in.CurrentBelt
	if strings.TrimSpace(string(belt)) == "" {
		belt = models.BeltWhite
	}

	now := time.Now()
	student := &models.Student{
		ID:                uuid.New(),
		FullName:          trimmedName,
		DOB:               dob,
		Gender:            strings.TrimSpace(in.Gender),
		Phone:             trimmedPhone,
		CurrentBelt:       belt,
		LastPromotionDate: now,
		EmergencyName:     trimmedEmName,
		EmergencyPhone:    trimmedEmPhone,
		EmergencyRelation: trimmedEmRel,
		MedicalNotes:      strings.TrimSpace(in.MedicalNotes),
		IsActive:          false, // Pending manager/admin approval
		WaiverAccepted:    true,
		WaiverSignedName:  trimmedSig,
		WaiverSignedAt:    &now,
		CreatedAt:         now,
	}

	if err := s.store.CreateStudent(student); err != nil {
		return nil, nil, fmt.Errorf("failed to create student profile: %w", err)
	}

	user := &models.User{
		ID:          uuid.New(),
		Email:       normEmail,
		Username:    normUsername,
		Role:        models.RoleStudent,
		StudentID:   &student.ID,
		DisplayName: trimmedName,
		IsActive:    false, // Pending manager/admin approval
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if err := user.SetPassword(in.Password); err != nil {
		_ = s.store.DeleteStudent(student.ID)
		return nil, nil, fmt.Errorf("failed to hash password: %w", err)
	}

	if err := s.store.CreateUser(user); err != nil {
		_ = s.store.DeleteStudent(student.ID)
		return nil, nil, fmt.Errorf("failed to create user record: %w", err)
	}

	return student, user, nil
}
