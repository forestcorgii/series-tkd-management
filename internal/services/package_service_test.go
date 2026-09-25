package services_test

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/services"
)

func TestPackageService_ProcessCheckInDeduction(t *testing.T) {
	svc := services.NewPackageService()
	studentID := uuid.New()
	now := time.Now()
	rem10 := 10
	rem1 := 1

	t.Run("Deducts session from oldest valid package", func(t *testing.T) {
		pkgOld := &models.StudentPackage{
			ID:                uuid.New(),
			StudentID:         studentID,
			RemainingSessions: &rem1,
			PurchaseDate:      now.AddDate(0, 0, -30),
			ExpiryDate:        now.AddDate(0, 0, 30),
			PaymentStatus:     "paid",
		}
		pkgNew := &models.StudentPackage{
			ID:                uuid.New(),
			StudentID:         studentID,
			RemainingSessions: &rem10,
			PurchaseDate:      now.AddDate(0, 0, -5),
			ExpiryDate:        now.AddDate(0, 0, 60),
			PaymentStatus:     "paid",
		}

		usedPkg, err := svc.ProcessCheckInDeduction([]*models.StudentPackage{pkgNew, pkgOld}, now)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if usedPkg.ID != pkgOld.ID {
			t.Errorf("expected oldest package to be used")
		}
		if *pkgOld.RemainingSessions != 0 {
			t.Errorf("expected remaining sessions to be 0, got %d", *pkgOld.RemainingSessions)
		}
	})

	t.Run("Returns error when no valid packages exist", func(t *testing.T) {
		rem0 := 0
		expiredPkg := &models.StudentPackage{
			ID:                uuid.New(),
			StudentID:         studentID,
			RemainingSessions: &rem0,
			PurchaseDate:      now.AddDate(0, 0, -30),
			ExpiryDate:        now.AddDate(0, 0, -1),
			PaymentStatus:     "paid",
		}

		_, err := svc.ProcessCheckInDeduction([]*models.StudentPackage{expiredPkg}, now)
		if err == nil {
			t.Errorf("expected error when package is expired/empty, got nil")
		}
	})
}
