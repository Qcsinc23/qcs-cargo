package main

import (
	"log"
	"os"

	"github.com/Qcsinc23/qcs-cargo/internal/db"
	_ "modernc.org/sqlite"
)

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "file:qcs.db?_journal_mode=WAL"
	}
	if err := db.Connect(dbURL); err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer db.Close()

	dir := os.Getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = "sql/migrations"
	}
	if err := db.RunMigrations(dir); err != nil {
		log.Fatalf("migrate: %v", err)
	}
	log.Println("Migrations complete.")
}
