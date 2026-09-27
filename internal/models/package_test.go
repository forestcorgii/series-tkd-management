package models

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPackageTemplate_Validate(t *testing.T) {
	ten := 10
	zero := 0
	negative := -5

	tests := []struct {
		name    string
		tpl     PackageTemplate
		wantErr error
	}{
		{
			name: "Valid Fixed Session Template",
			tpl: PackageTemplate{
				Title:        "10-Class Sparring Pass",
				SessionCount: &ten,
				ValidityDays: 90,
				Price:        150.00,
			},
			wantErr: nil,
		},
		{
			name: "Valid Unlimited Template",
			tpl: PackageTemplate{
				Title:        "Monthly Unlimited Pass",
				SessionCount: nil,
				ValidityDays: 30,
				Price:        200.00,
			},
			wantErr: nil,
		},
		{
			name: "Empty Title",
			tpl: PackageTemplate{
				Title:        "",
				SessionCount: &ten,
				ValidityDays: 90,
				Price:        150.00,
			},
			wantErr: ErrInvalidTitle,
		},
		{
			name: "Zero Validity Days",
			tpl: PackageTemplate{
				Title:        "Invalid Validity",
				SessionCount: &ten,
				ValidityDays: 0,
				Price:        150.00,
			},
			wantErr: ErrInvalidValidity,
		},
		{
			name: "Negative Price",
			tpl: PackageTemplate{
				Title:        "Invalid Price",
				SessionCount: &ten,
				ValidityDays: 90,
				Price:        -20.00,
			},
			wantErr: ErrInvalidPrice,
		},
		{
			name: "Zero Session Count",
			tpl: PackageTemplate{
				Title:        "Zero Session",
				SessionCount: &zero,
				ValidityDays: 90,
				Price:        100.00,
			},
			wantErr: ErrInvalidSessionCount,
		},
		{
			name: "Negative Session Count",
			tpl: PackageTemplate{
				Title:        "Negative Session",
				SessionCount: &negative,
				ValidityDays: 90,
				Price:        100.00,
			},
			wantErr: ErrInvalidSessionCount,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tpl.Validate()
			if err != tt.wantErr {
				t.Fatalf("expected error %v, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestStudentPackage_ValidityAndDeduction(t *testing.T) {
	now := time.Now()
	five := 5

	sp := &StudentPackage{
		ID:                uuid.New(),
		StudentID:         uuid.New(),
		TemplateID:        uuid.New(),
		TotalSessions:     &five,
		RemainingSessions: &five,
		PurchaseDate:      now.AddDate(0, 0, -10),
		ExpiryDate:        now.AddDate(0, 0, 20),
		PaymentStatus:     "paid",
	}

	if !sp.IsValidAt(now) {
		t.Fatal("expected package to be valid at now")
	}

	// Deduct session
	if err := sp.DeductSession(now); err != nil {
		t.Fatalf("unexpected error deducting session: %v", err)
	}

	if *sp.RemainingSessions != 4 {
		t.Fatalf("expected 4 remaining sessions, got %d", *sp.RemainingSessions)
	}

	// Test unpaid status
	sp.PaymentStatus = "unpaid"
	if sp.IsValidAt(now) {
		t.Fatal("expected unpaid package to be invalid")
	}

	// Test expired
	sp.PaymentStatus = "paid"
	expiredTime := now.AddDate(0, 0, 30)
	if sp.IsValidAt(expiredTime) {
		t.Fatal("expected expired package to be invalid")
	}
	if err := sp.DeductSession(expiredTime); err != ErrPackageExpired {
		t.Fatalf("expected ErrPackageExpired, got %v", err)
	}
}

func TestPackageTemplate_FourWeekPlan(t *testing.T) {
	four := 4
	eight := 8
	twelve := 12

	t.Run("Valid 4-week 4-session plan defaults to 28 days and 1/week cadence", func(t *testing.T) {
		tpl := PackageTemplate{
			Title:        "4-Week Foundations Pass",
			PlanType:     PlanTypeFourWeek,
			SessionCount: &four,
			ValidityDays: 0, // Should default to 28
			Price:        100.00,
		}
		if err := tpl.Validate(); err != nil {
			t.Fatalf("unexpected validation error: %v", err)
		}
		if tpl.ValidityDays != 28 {
			t.Fatalf("expected validity days 28, got %d", tpl.ValidityDays)
		}
		if !tpl.IsFourWeek() {
			t.Fatal("expected IsFourWeek to be true")
		}
		if tpl.WeeklyCadence() != 1 {
			t.Fatalf("expected weekly cadence 1, got %d", tpl.WeeklyCadence())
		}
	})

	t.Run("Valid 4-week 8-session plan has 2/week cadence", func(t *testing.T) {
		tpl := PackageTemplate{
			Title:        "4-Week Competitor Pass",
			PlanType:     PlanTypeFourWeek,
			SessionCount: &eight,
			ValidityDays: 28,
			Price:        180.00,
		}
		if err := tpl.Validate(); err != nil {
			t.Fatalf("unexpected validation error: %v", err)
		}
		if tpl.WeeklyCadence() != 2 {
			t.Fatalf("expected weekly cadence 2, got %d", tpl.WeeklyCadence())
		}
	})

	t.Run("Valid 4-week 12-session plan has 3/week cadence", func(t *testing.T) {
		tpl := PackageTemplate{
			Title:        "4-Week Intensive Pass",
			PlanType:     PlanTypeFourWeek,
			SessionCount: &twelve,
			ValidityDays: 28,
			Price:        250.00,
		}
		if tpl.WeeklyCadence() != 3 {
			t.Fatalf("expected weekly cadence 3, got %d", tpl.WeeklyCadence())
		}
	})

	t.Run("Four-week plan requires positive session count", func(t *testing.T) {
		tpl := PackageTemplate{
			Title:        "Invalid 4-Week Pass",
			PlanType:     PlanTypeFourWeek,
			SessionCount: nil,
			ValidityDays: 28,
			Price:        100.00,
		}
		if err := tpl.Validate(); err != ErrInvalidSessionCount {
			t.Fatalf("expected ErrInvalidSessionCount, got %v", err)
		}
	})
}

func TestStudentPackage_FourWeekCycleWindow(t *testing.T) {
	startDate := time.Date(2026, 10, 1, 10, 0, 0, 0, time.UTC)
	four := 4

	sp := &StudentPackage{
		ID:            uuid.New(),
		PlanType:      PlanTypeFourWeek,
		TotalSessions: &four,
		PurchaseDate:  startDate,
		ExpiryDate:    startDate.AddDate(0, 0, 28),
		PaymentStatus: "paid",
	}

	// Day 3 (within Week 1: Days 1-7)
	weekNum, start, end := sp.CurrentCycleWindow(startDate.AddDate(0, 0, 2))
	if weekNum != 1 {
		t.Fatalf("expected week 1, got %d", weekNum)
	}
	if !start.Equal(startDate) {
		t.Fatalf("expected cycle start %v, got %v", startDate, start)
	}
	if !end.Equal(startDate.AddDate(0, 0, 7)) {
		t.Fatalf("expected cycle end %v, got %v", startDate.AddDate(0, 0, 7), end)
	}

	// Day 8 (within Week 2: Days 8-14)
	weekNum, start, end = sp.CurrentCycleWindow(startDate.AddDate(0, 0, 7))
	if weekNum != 2 {
		t.Fatalf("expected week 2, got %d", weekNum)
	}
	if !start.Equal(startDate.AddDate(0, 0, 7)) {
		t.Fatalf("expected cycle start %v, got %v", startDate.AddDate(0, 0, 7), start)
	}

	// Day 25 (within Week 4: Days 22-28)
	weekNum, _, _ = sp.CurrentCycleWindow(startDate.AddDate(0, 0, 24))
	if weekNum != 4 {
		t.Fatalf("expected week 4, got %d", weekNum)
	}
}
