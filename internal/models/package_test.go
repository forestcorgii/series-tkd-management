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
