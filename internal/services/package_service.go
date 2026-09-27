package services

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"series-tkd-management/internal/models"
)

var (
	ErrNoValidPackageForStudent = errors.New("student has no valid active package with remaining credits")
)

type PackageService struct{}

func NewPackageService() *PackageService {
	return &PackageService{}
}

// CheckWeeklyQuota returns an error if a 4-week cadence plan has reached its limit in the current 7-day cycle.
func (ps *PackageService) CheckWeeklyQuota(pkg *models.StudentPackage, attendances []*models.Attendance, atTime time.Time) error {
	if !pkg.IsFourWeek() {
		return nil
	}
	weekNum, cycleStart, cycleEnd := pkg.CurrentCycleWindow(atTime)
	count := 0
	for _, att := range attendances {
		if att.StudentPackageID != nil && *att.StudentPackageID == pkg.ID {
			if !att.CheckedInAt.Before(cycleStart) && att.CheckedInAt.Before(cycleEnd) {
				count++
			}
		}
	}
	limit := pkg.WeeklyCadence()
	if count >= limit {
		return fmt.Errorf("weekly limit of %d class(es) reached for Week %d (resets on %s)", limit, weekNum, cycleEnd.Format("Jan 02"))
	}
	return nil
}

// FindOldestValidPackage finds the oldest valid package for the student.
// If attendances are provided, it respects weekly quota limits on 4-week plans.
func (ps *PackageService) FindOldestValidPackage(packages []*models.StudentPackage, attendances []*models.Attendance, atTime time.Time) (*models.StudentPackage, error) {
	validPackages := []*models.StudentPackage{}
	var lastWeeklyErr error

	for _, pkg := range packages {
		if pkg.IsValidAt(atTime) {
			if err := ps.CheckWeeklyQuota(pkg, attendances, atTime); err != nil {
				lastWeeklyErr = err
				continue
			}
			validPackages = append(validPackages, pkg)
		}
	}

	if len(validPackages) == 0 {
		if lastWeeklyErr != nil {
			return nil, lastWeeklyErr
		}
		return nil, ErrNoValidPackageForStudent
	}

	// Sort by purchase date ascending (oldest first)
	sort.Slice(validPackages, func(i, j int) bool {
		return validPackages[i].PurchaseDate.Before(validPackages[j].PurchaseDate)
	})

	return validPackages[0], nil
}

// ProcessCheckInDeduction finds an eligible package and deducts 1 session.
func (ps *PackageService) ProcessCheckInDeduction(packages []*models.StudentPackage, attendances []*models.Attendance, atTime time.Time) (*models.StudentPackage, error) {
	pkg, err := ps.FindOldestValidPackage(packages, attendances, atTime)
	if err != nil {
		return nil, err
	}

	if err := pkg.DeductSession(atTime); err != nil {
		return nil, err
	}

	return pkg, nil
}
