package services

import (
	"errors"
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

func (ps *PackageService) FindOldestValidPackage(packages []*models.StudentPackage, atTime time.Time) (*models.StudentPackage, error) {
	validPackages := []*models.StudentPackage{}
	for _, pkg := range packages {
		if pkg.IsValidAt(atTime) {
			validPackages = append(validPackages, pkg)
		}
	}

	if len(validPackages) == 0 {
		return nil, ErrNoValidPackageForStudent
	}

	// Sort by purchase date ascending (oldest first)
	sort.Slice(validPackages, func(i, j int) bool {
		return validPackages[i].PurchaseDate.Before(validPackages[j].PurchaseDate)
	})

	return validPackages[0], nil
}

func (ps *PackageService) ProcessCheckInDeduction(packages []*models.StudentPackage, atTime time.Time) (*models.StudentPackage, error) {
	pkg, err := ps.FindOldestValidPackage(packages, atTime)
	if err != nil {
		return nil, err
	}

	if err := pkg.DeductSession(atTime); err != nil {
		return nil, err
	}

	return pkg, nil
}
