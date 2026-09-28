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
	ErrInvalidCredentials    = errors.New("invalid email or password")
	ErrUserInactive          = errors.New("user account is inactive")
	ErrEmailAlreadyExists    = errors.New("a user with this email already exists")
	ErrUsernameAlreadyExists = models.ErrUsernameAlreadyExists
	ErrInvalidResetToken     = models.ErrInvalidResetToken
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

	if !user.IsActive {
		return nil, "", ErrUserInactive
	}

	if user.Role == models.RoleCoach {
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
			return nil, "", ErrUserInactive
		}
	}

	if !user.CheckPassword(password) {
		return nil, "", ErrInvalidCredentials
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
	if user.Role == models.RoleCoach {
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

	log.Printf("🔑 [PASSWORD RESET] Token generated for user %s (%s): /reset-password?token=%s", user.Email, user.Username, rawToken)

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
	if tokenStr == "" || len(newPassword) < 6 {
		return nil, errors.New("password must be at least 6 characters")
	}

	resetToken, user, err := s.ValidatePasswordResetToken(tokenStr)
	if err != nil {
		return nil, err
	}

	if err := user.SetPassword(newPassword); err != nil {
		return nil, fmt.Errorf("failed to hash new password: %w", err)
	}

	if err := s.store.UpdateUser(user); err != nil {
		return nil, fmt.Errorf("failed to update user password: %w", err)
	}

	if err := s.store.MarkPasswordResetTokenUsed(resetToken.Token); err != nil {
		return nil, fmt.Errorf("failed to mark token as used: %w", err)
	}

	log.Printf("✅ [PASSWORD RESET] Password successfully reset for user %s (%s)", user.Email, user.Username)
	return user, nil
}
