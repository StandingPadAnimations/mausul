package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed migrations/*.sql
var migrationFS embed.FS

type migrationFile struct {
	version int
	path    string
}

func migrate(ctx context.Context, db *sql.DB) error {
	var currentVersion int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&currentVersion); err != nil {
		return fmt.Errorf("failed to read user_version: %w", err)
	}

	if currentVersion == 0 {
		var exists int
		err := db.QueryRowContext(ctx, `
				SELECT COUNT(*) FROM sqlite_master
				WHERE type = 'table' AND name = 'webmentions'
			`).Scan(&exists)
		if err != nil {
			return fmt.Errorf("failed to check existing schema: %w", err)
		}
		if exists > 0 {
			// We didn't set this originally, but
			// schema version 1 is what the initial
			// schema was
			currentVersion = 1
			if _, err := db.ExecContext(ctx, `PRAGMA user_version = 1`); err != nil {
				return fmt.Errorf("failed to baseline user_version: %w", err)
			}
		}
	}

	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		return fmt.Errorf("failed to read embedded migrations directory: %w", err)
	}

	var plan []migrationFile
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}

		parts := strings.SplitN(entry.Name(), "_", 2)
		if len(parts) < 2 {
			continue
		}

		v, err := strconv.Atoi(parts[0])
		if err != nil {
			return fmt.Errorf("failed to parse migration filename prefix: %w", err)
		}

		plan = append(plan, migrationFile{
			version: v,
			path:    filepath.Join("migrations", entry.Name()),
		})
	}

	sort.Slice(plan, func(i, j int) bool {
		return plan[i].version < plan[j].version
	})

	for _, m := range plan {
		if m.version <= currentVersion {
			continue
		}

		content, err := migrationFS.ReadFile(m.path)
		if err != nil {
			return fmt.Errorf("failed to read migration file: %w", err)
		}

		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("failed to begin migration transaction: %w", err)
		}

		if _, err := tx.ExecContext(ctx, string(content)); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to execute migration: %w", err)
		}

		setVerQuery := fmt.Sprintf("PRAGMA user_version = %d", m.version)
		if _, err := tx.ExecContext(ctx, setVerQuery); err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to set user_version: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to commit migration transaction: %w", err)
		}
	}
	return nil
}
