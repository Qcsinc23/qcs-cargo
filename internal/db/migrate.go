package db

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// RunMigrations executes all *.sql files in dir in lexicographic order.
// Each file may contain "-- +goose Up" and "-- +goose Down"; only the Up block is executed.
func RunMigrations(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(dir, name)
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		up := extractGooseUp(string(raw))
		if up == "" {
			continue
		}
		tx, err := DB().Begin()
		if err != nil {
			return err
		}
		for _, stmt := range splitStatements(up) {
			stmt = strings.TrimSpace(stmt)
			if stmt == "" {
				continue
			}
			if _, err := tx.Exec(stmt); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func splitStatements(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ";") {
		if t := strings.TrimSpace(part); t != "" {
			out = append(out, t+";")
		}
	}
	return out
}

func extractGooseUp(content string) string {
	lines := strings.Split(content, "\n")
	var out []string
	inUp := false
	for _, line := range lines {
		if strings.TrimSpace(line) == "-- +goose Up" {
			inUp = true
			continue
		}
		if inUp && strings.TrimSpace(line) == "-- +goose Down" {
			break
		}
		if inUp {
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
