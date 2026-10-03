package repository_test

import (
	"path/filepath"
	"testing"
	"time"

	"series-tkd-management/internal/models"
	"series-tkd-management/internal/repository"
)

func TestStore_TrainingCategories(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_categories.db")
	sqlStore, _, err := repository.InitDatabase(dbPath)
	if err != nil {
		t.Fatalf("failed to init test database: %v", err)
	}
	defer func() {
		if closer, ok := sqlStore.(interface{ Close() error }); ok {
			_ = closer.Close()
		}
	}()

	stores := map[string]repository.RepositoryStore{
		"MemoryStore": repository.NewMemoryStore(),
		"SQLStore":    sqlStore,
	}

	for name, store := range stores {
		t.Run(name, func(t *testing.T) {
			// 1. Check default seeded categories
			cats, err := store.GetAllTrainingCategories()
			if err != nil {
				t.Fatalf("GetAllTrainingCategories failed: %v", err)
			}
			if len(cats) < 4 {
				t.Fatalf("expected at least 4 default categories, got %d", len(cats))
			}

			// 2. Create new category
			newCat := &models.TrainingCategory{
				Name:  "Cadet Sparring",
				Color: "#059669",
			}
			if err := store.CreateTrainingCategory(newCat); err != nil {
				t.Fatalf("CreateTrainingCategory failed: %v", err)
			}

			// Duplicate name should fail
			dupCat := &models.TrainingCategory{
				Name:  "cadet sparring",
				Color: "#10b981",
			}
			if err := store.CreateTrainingCategory(dupCat); err == nil {
				t.Errorf("expected error creating duplicate category, got nil")
			}

			// 3. Get by ID
			fetched, err := store.GetTrainingCategoryByID(newCat.ID)
			if err != nil {
				t.Fatalf("GetTrainingCategoryByID failed: %v", err)
			}
			if fetched.Name != "Cadet Sparring" || fetched.Color != "#059669" {
				t.Errorf("unexpected category details: %+v", fetched)
			}

			// 4. Get by Name
			byName, err := store.GetTrainingCategoryByName("cadet sparring")
			if err != nil {
				t.Fatalf("GetTrainingCategoryByName failed: %v", err)
			}
			if byName.ID != newCat.ID {
				t.Errorf("expected category ID %v, got %v", newCat.ID, byName.ID)
			}

			// 5. Update category
			fetched.Name = "Elite Cadet Sparring"
			fetched.Color = "#047857"
			if err := store.UpdateTrainingCategory(fetched); err != nil {
				t.Fatalf("UpdateTrainingCategory failed: %v", err)
			}

			updated, _ := store.GetTrainingCategoryByID(newCat.ID)
			if updated.Name != "Elite Cadet Sparring" || updated.Color != "#047857" {
				t.Errorf("expected updated name and color, got %+v", updated)
			}

			// 6. Assigned session blocks deletion
			sess := &models.TrainingSession{
				SessionDate:  time.Now(),
				StartTime:    "10:00",
				EndTime:      "11:00",
				TrainingType: models.TrainingType(fetched.Name),
			}
			if err := store.CreateSession(sess); err != nil {
				t.Fatalf("CreateSession failed: %v", err)
			}

			if err := store.DeleteTrainingCategory(newCat.ID); err == nil {
				t.Fatalf("expected error deleting category in use by session, got nil")
			}

			// Clean up session
			_ = store.DeleteSession(sess.ID)

			// Now deletion should succeed
			if err := store.DeleteTrainingCategory(newCat.ID); err != nil {
				t.Fatalf("DeleteTrainingCategory failed: %v", err)
			}

			if _, err := store.GetTrainingCategoryByID(newCat.ID); err == nil {
				t.Errorf("expected error getting deleted category, got nil")
			}
		})
	}
}

