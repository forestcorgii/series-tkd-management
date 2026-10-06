package services_test

import (
	"testing"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
	"series-tkd-management/internal/services"
)

func TestAuditService_Log(t *testing.T) {
	store := repository.NewMemoryStore()
	svc := services.NewAuditService(store)

	t.Run("nil entry returns error", func(t *testing.T) {
		err := svc.Log(nil)
		if err == nil {
			t.Fatal("expected error for nil entry, got nil")
		}
	})

	t.Run("empty action returns error", func(t *testing.T) {
		err := svc.Log(&models.AuditLog{
			Action:   "   ",
			Category: models.AuditCategoryAuth,
		})
		if err == nil {
			t.Fatal("expected error for empty action, got nil")
		}
	})

	t.Run("successful log with defaults filled", func(t *testing.T) {
		actorID := uuid.New()
		entry := &models.AuditLog{
			UserID:      &actorID,
			ActorName:   " Coach Ken ",
			ActorEmail:  "ken@seriestkd.com",
			ActorRole:   models.RoleCoach,
			Action:      "SESSION_CHECKIN",
			Category:    models.AuditCategoryAttendance,
			TargetType:  "Student",
			TargetName:  "Juan Dela Cruz",
			Description: "Checked in student to Sparring Class",
			IPAddress:   "192.168.1.100",
		}

		err := svc.Log(entry)
		if err != nil {
			t.Fatalf("unexpected error logging entry: %v", err)
		}

		if entry.ID == uuid.Nil {
			t.Error("expected entry ID to be populated with a valid UUID")
		}
		if entry.CreatedAt.IsZero() {
			t.Error("expected entry CreatedAt to be set")
		}
		if entry.ActorName != "Coach Ken" {
			t.Errorf("expected trimmed ActorName 'Coach Ken', got '%s'", entry.ActorName)
		}
	})
}

func TestAuditService_GetLogsAndFiltering(t *testing.T) {
	store := repository.NewMemoryStore()
	svc := services.NewAuditService(store)

	// Ingest sample records
	_ = svc.Log(&models.AuditLog{
		Action:      "AUTH_LOGIN",
		Category:    models.AuditCategoryAuth,
		ActorName:   "Admin Jane",
		ActorEmail:  "admin@seriestkd.com",
		ActorRole:   models.RoleAdmin,
		Description: "Admin authenticated successfully",
		IPAddress:   "127.0.0.1",
	})
	_ = svc.Log(&models.AuditLog{
		Action:      "STUDENT_CREATE",
		Category:    models.AuditCategoryStudents,
		ActorName:   "Manager Mike",
		ActorEmail:  "manager@seriestkd.com",
		ActorRole:   models.RoleOperationManager,
		TargetName:  "Carlos Yulo",
		Description: "Registered new student Carlos Yulo",
		IPAddress:   "192.168.1.50",
	})
	_ = svc.Log(&models.AuditLog{
		Action:      "COACH_DEACTIVATE",
		Category:    models.AuditCategoryCoaches,
		ActorName:   "Manager Mike",
		ActorEmail:  "manager@seriestkd.com",
		ActorRole:   models.RoleOperationManager,
		TargetName:  "Coach Lee",
		Description: "Deactivated coach account",
		IPAddress:   "192.168.1.50",
	})

	t.Run("filter by category", func(t *testing.T) {
		logs, total, err := svc.GetLogs(models.AuditLogFilter{
			Category: "AUTH",
		})
		if err != nil {
			t.Fatalf("GetLogs failed: %v", err)
		}
		if total != 1 {
			t.Fatalf("expected 1 log for AUTH category, got %d", total)
		}
		if logs[0].Action != "AUTH_LOGIN" {
			t.Errorf("expected AUTH_LOGIN, got %s", logs[0].Action)
		}
	})

	t.Run("filter by role", func(t *testing.T) {
		logs, total, err := svc.GetLogs(models.AuditLogFilter{
			Role: "ADMIN",
		})
		if err != nil {
			t.Fatalf("GetLogs failed: %v", err)
		}
		if total != 1 {
			t.Fatalf("expected 1 log for ADMIN role, got %d", total)
		}
		if logs[0].ActorRole != models.RoleAdmin {
			t.Errorf("expected RoleAdmin, got %s", logs[0].ActorRole)
		}
	})

	t.Run("filter by search text", func(t *testing.T) {
		logs, total, err := svc.GetLogs(models.AuditLogFilter{
			Search: "carlos",
		})
		if err != nil {
			t.Fatalf("GetLogs failed: %v", err)
		}
		if total != 1 {
			t.Fatalf("expected 1 log matching 'carlos', got %d", total)
		}
		if logs[0].TargetName != "Carlos Yulo" {
			t.Errorf("expected TargetName 'Carlos Yulo', got '%s'", logs[0].TargetName)
		}
	})

	t.Run("pagination limit clamp", func(t *testing.T) {
		logs, total, err := svc.GetLogs(models.AuditLogFilter{
			Limit: 1,
		})
		if err != nil {
			t.Fatalf("GetLogs failed: %v", err)
		}
		if total != 3 {
			t.Errorf("expected total 3, got %d", total)
		}
		if len(logs) != 1 {
			t.Errorf("expected 1 log returned due to limit 1, got %d", len(logs))
		}
	})
}

func TestAuditService_GetTelemetry(t *testing.T) {
	store := repository.NewMemoryStore()
	svc := services.NewAuditService(store)

	mgrID := uuid.New()
	adminID := uuid.New()

	_ = svc.Log(&models.AuditLog{
		UserID:      &mgrID,
		ActorName:   "Manager",
		ActorEmail:  "manager@seriestkd.com",
		Action:      "AUTH_LOGIN",
		Category:    models.AuditCategoryAuth,
		Description: "Manager login",
	})
	_ = svc.Log(&models.AuditLog{
		UserID:      &mgrID,
		ActorName:   "Manager",
		ActorEmail:  "manager@seriestkd.com",
		Action:      "SETTINGS_UPDATE",
		Category:    models.AuditCategorySettings,
		Description: "Updated branch configuration",
	})
	_ = svc.Log(&models.AuditLog{
		UserID:      &adminID,
		ActorName:   "Admin",
		ActorEmail:  "admin@seriestkd.com",
		Action:      "COACH_DEACTIVATE",
		Category:    models.AuditCategoryCoaches,
		Description: "Deactivated coach account",
	})

	telemetry, err := svc.GetTelemetry()
	if err != nil {
		t.Fatalf("GetTelemetry failed: %v", err)
	}

	if telemetry.TotalLogs != 3 {
		t.Errorf("expected TotalLogs 3, got %d", telemetry.TotalLogs)
	}
	if telemetry.TodayLogs != 3 {
		t.Errorf("expected TodayLogs 3, got %d", telemetry.TodayLogs)
	}
	if telemetry.ActiveUsers != 2 {
		t.Errorf("expected 2 ActiveUsers, got %d", telemetry.ActiveUsers)
	}
	if telemetry.SecurityActions < 2 {
		t.Errorf("expected at least 2 SecurityActions (AUTH_LOGIN and COACH_DEACTIVATE), got %d", telemetry.SecurityActions)
	}
}
