package repository_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

func TestAuditStore_SQLiteAndMemory(t *testing.T) {
	// 1. Test MemoryStore
	memStore := repository.NewMemoryStore()
	testAuditStoreContract(t, memStore, "MemoryStore")

	// 2. Test SQLStore (in-memory SQLite)
	sqlStore, driver, err := repository.InitDatabase("file:memdb_audit?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("failed to initialize sqlite store: %v", err)
	}
	if driver != "sqlite" {
		t.Fatalf("expected sqlite driver, got %s", driver)
	}

	testAuditStoreContract(t, sqlStore, "SQLStore")
}

func testAuditStoreContract(t *testing.T, store repository.RepositoryStore, storeName string) {
	t.Helper()

	actorID := uuid.New()
	entry := &models.AuditLog{
		ID:          uuid.New(),
		UserID:      &actorID,
		ActorName:   "Admin Jane",
		ActorEmail:  "admin@seriestkd.com",
		ActorRole:   models.RoleAdmin,
		Action:      "STUDENT_CREATE",
		Category:    models.AuditCategoryStudents,
		TargetType:  "Student",
		TargetID:    uuid.New().String(),
		TargetName:  "Alex Morgan",
		Description: "Created student profile",
		IPAddress:   "127.0.0.1",
		UserAgent:   "Go-Test-Agent",
		Metadata:    `{"belt":"White"}`,
		CreatedAt:   time.Now(),
	}

	if err := store.CreateAuditLog(entry); err != nil {
		t.Fatalf("[%s] CreateAuditLog failed: %v", storeName, err)
	}

	logs, total, err := store.GetAuditLogs(models.AuditLogFilter{
		Category: "STUDENTS",
	})
	if err != nil {
		t.Fatalf("[%s] GetAuditLogs failed: %v", storeName, err)
	}
	if total < 1 || len(logs) < 1 {
		t.Fatalf("[%s] expected at least 1 log, got total=%d, len=%d", storeName, total, len(logs))
	}
	if logs[0].Action != "STUDENT_CREATE" {
		t.Errorf("[%s] expected action 'STUDENT_CREATE', got '%s'", storeName, logs[0].Action)
	}

	telemetry, err := store.GetAuditTelemetry()
	if err != nil {
		t.Fatalf("[%s] GetAuditTelemetry failed: %v", storeName, err)
	}
	if telemetry.TotalLogs < 1 {
		t.Errorf("[%s] expected TotalLogs >= 1, got %d", storeName, telemetry.TotalLogs)
	}
}
