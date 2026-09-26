package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrUserInactive       = errors.New("user account is inactive")
	ErrEmailAlreadyExists = errors.New("a user with this email already exists")
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

func (s *AuthService) Login(email, password string) (*models.User, string, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	user, err := s.store.GetUserByEmail(normEmail)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, "", ErrInvalidCredentials
		}
		return nil, "", err
	}

	if !user.IsActive {
		return nil, "", ErrUserInactive
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
	return user, nil
}

func (s *AuthService) Logout(token string) error {
	if token == "" {
		return nil
	}
	return s.store.DeleteSessionToken(token)
}

func (s *AuthService) RegisterUser(email, password string, role models.UserRole, studentID, coachID *uuid.UUID) (*models.User, error) {
	normEmail := strings.ToLower(strings.TrimSpace(email))
	if normEmail == "" || password == "" {
		return nil, errors.New("email and password cannot be empty")
	}

	// Check if already exists
	if existing, _ := s.store.GetUserByEmail(normEmail); existing != nil {
		return nil, ErrEmailAlreadyExists
	}

	u := &models.User{
		ID:        uuid.New(),
		Email:     normEmail,
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
