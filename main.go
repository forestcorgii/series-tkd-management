package main

import (
	"log"
	"os"

	"series-tkd-management/internal/repository"
	"series-tkd-management/internal/server"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	store, driver, err := repository.InitDatabase(databaseURL)
	if err != nil {
		log.Fatalf("Fatal: Failed to initialize %s database: %v", driver, err)
	}
	log.Printf("📦 Database connected using driver: %s", driver)

	server.BootstrapManagers(store)

	handler, err := server.NewHandler(store)
	if err != nil {
		log.Fatalf("Fatal: Failed to initialize AppHandler: %v", err)
	}

	var closeFn func() error
	if closer, ok := store.(interface{ Close() error }); ok {
		closeFn = closer.Close
	}
	if err := server.ListenAndServe(os.Getenv("PORT"), handler, closeFn); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
