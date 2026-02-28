package main

import (
	"log"
	"os"
	"strconv"
	"strings"

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

	direction := strings.ToLower(strings.TrimSpace(os.Getenv("MIGRATION_DIRECTION")))
	if direction == "" {
		direction = "up"
	}

	switch direction {
	case "up":
		if err := db.RunMigrations(dir); err != nil {
			log.Fatalf("migrate up: %v", err)
		}
		log.Println("Migrations up complete.")
	case "down":
		steps := 0
		if raw := strings.TrimSpace(os.Getenv("MIGRATION_STEPS")); raw != "" {
			v, err := strconv.Atoi(raw)
			if err != nil {
				log.Fatalf("invalid MIGRATION_STEPS %q: %v", raw, err)
			}
			steps = v
		}
		if err := db.RunMigrationsDown(dir, steps); err != nil {
			log.Fatalf("migrate down: %v", err)
		}
		log.Printf("Migrations down complete (steps=%d, 0 means all).", steps)
	default:
		log.Fatalf("unsupported MIGRATION_DIRECTION %q (use 'up' or 'down')", direction)
	}
}
