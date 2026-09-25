package handlers_test

import (
	"testing"

	"series-tkd-management/internal/handlers"
	"series-tkd-management/internal/repository"
)

func TestAppHandler_ParseTemplates(t *testing.T) {
	store := repository.NewMemoryStore()
	_, err := handlers.NewAppHandler(store)
	if err != nil {
		t.Fatalf("failed to initialize AppHandler: %v", err)
	}
}
