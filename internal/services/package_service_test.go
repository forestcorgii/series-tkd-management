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

		usedPkg, err := svc.ProcessCheckInDeduction([]*models.StudentPackage{pkgNew, pkgOld}, nil, now)
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

		_, err := svc.ProcessCheckInDeduction([]*models.StudentPackage{expiredPkg}, nil, now)
		if err == nil {
			t.Errorf("expected error when package is expired/empty, got nil")
		}
	})

	t.Run("4-Week 4-session plan enforces 1 session per week", func(t *testing.T) {
		four := 4
		startDate := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

		pkg4 := &models.StudentPackage{
			ID:                uuid.New(),
			StudentID:         studentID,
			PlanType:          models.PlanTypeFourWeek,
			TotalSessions:     &four,
			RemainingSessions: &four,
			PurchaseDate:      startDate,
			ExpiryDate:        startDate.AddDate(0, 0, 28),
			PaymentStatus:     "paid",
		}

		// First check-in on Day 2: Should succeed
		day2 := startDate.AddDate(0, 0, 1)
		used, err := svc.ProcessCheckInDeduction([]*models.StudentPackage{pkg4}, nil, day2)
		if err != nil {
			t.Fatalf("expected first check-in to succeed: %v", err)
		}
		if *used.RemainingSessions != 3 {
			t.Fatalf("expected 3 sessions left, got %d", *used.RemainingSessions)
		}

		// Record the attendance from Day 2
		atts := []*models.Attendance{
			{
				ID:               uuid.New(),
				StudentID:        studentID,
				StudentPackageID: &pkg4.ID,
				CheckedInAt:      day2,
			},
		}

		// Second check-in on Day 4 (same Week 1): Should fail with weekly quota error
		day4 := startDate.AddDate(0, 0, 3)
		_, err = svc.ProcessCheckInDeduction([]*models.StudentPackage{pkg4}, atts, day4)
		if err == nil {
			t.Fatal("expected second check-in in Week 1 to be rejected, but succeeded")
		}

		// Third check-in on Day 9 (Week 2): Should succeed!
		day9 := startDate.AddDate(0, 0, 8)
		used2, err := svc.ProcessCheckInDeduction([]*models.StudentPackage{pkg4}, atts, day9)
		if err != nil {
			t.Fatalf("expected check-in in Week 2 to succeed: %v", err)
		}
		if *used2.RemainingSessions != 2 {
			t.Fatalf("expected 2 sessions left, got %d", *used2.RemainingSessions)
		}
	})

	t.Run("4-Week 8-session plan enforces 2 sessions per week", func(t *testing.T) {
		eight := 8
		startDate := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

		pkg8 := &models.StudentPackage{
			ID:                uuid.New(),
			StudentID:         studentID,
			PlanType:          models.PlanTypeFourWeek,
			TotalSessions:     &eight,
			RemainingSessions: &eight,
			PurchaseDate:      startDate,
			ExpiryDate:        startDate.AddDate(0, 0, 28),
			PaymentStatus:     "paid",
		}

		// Two check-ins in Week 1
		atts := []*models.Attendance{
			{
				ID:               uuid.New(),
				StudentID:        studentID,
				StudentPackageID: &pkg8.ID,
				CheckedInAt:      startDate.AddDate(0, 0, 1),
			},
			{
				ID:               uuid.New(),
				StudentID:        studentID,
				StudentPackageID: &pkg8.ID,
				CheckedInAt:      startDate.AddDate(0, 0, 3),
			},
		}

		// Third check-in on Day 5 (same Week 1): Should fail
		day5 := startDate.AddDate(0, 0, 4)
		_, err := svc.ProcessCheckInDeduction([]*models.StudentPackage{pkg8}, atts, day5)
		if err == nil {
			t.Fatal("expected 3rd check-in in Week 1 to fail for 8-session plan")
		}
	})
}
