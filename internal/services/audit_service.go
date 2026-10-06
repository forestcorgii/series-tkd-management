package services

import (
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

type AuditService struct {
	store repository.RepositoryStore
}

func NewAuditService(store repository.RepositoryStore) *AuditService {
	return &AuditService{store: store}
}

func (s *AuditService) Log(entry *models.AuditLog) error {
	if entry == nil {
		return errors.New("audit log entry cannot be nil")
	}

	entry.Action = strings.TrimSpace(entry.Action)
	if entry.Action == "" {
		return errors.New("audit log action is required")
	}

	entry.Category = models.AuditCategory(strings.TrimSpace(string(entry.Category)))
	if entry.Category == "" {
		entry.Category = models.AuditCategorySettings
	}

	entry.ActorName = strings.TrimSpace(entry.ActorName)
	entry.ActorEmail = strings.TrimSpace(entry.ActorEmail)
	entry.Description = strings.TrimSpace(entry.Description)
	entry.TargetType = strings.TrimSpace(entry.TargetType)
	entry.TargetName = strings.TrimSpace(entry.TargetName)
	entry.IPAddress = strings.TrimSpace(entry.IPAddress)
	entry.UserAgent = strings.TrimSpace(entry.UserAgent)

	if entry.ID == uuid.Nil {
		entry.ID = uuid.New()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}

	return s.store.CreateAuditLog(entry)
}

func (s *AuditService) GetLogs(filter models.AuditLogFilter) ([]*models.AuditLog, int, error) {
	if filter.Limit <= 0 {
		filter.Limit = 50
	} else if filter.Limit > 200 {
		filter.Limit = 200
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}

	return s.store.GetAuditLogs(filter)
}

func (s *AuditService) GetTelemetry() (*models.AuditTelemetry, error) {
	return s.store.GetAuditTelemetry()
}
