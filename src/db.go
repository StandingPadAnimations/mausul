package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Webmention struct {
	ID        int
	Source    string
	Target    string
	CreatedAt time.Time
}

func InitDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode=WAL;",
		"PRAGMA busy_timeout=5000;",
		"PRAGMA synchronous=NORMAL;",
		"PRAGMA foreign_keys=ON;",
	}

	for _, pragma := range pragmas {
		if _, err := db.Exec(pragma); err != nil {
			return nil, fmt.Errorf("failed to set pragma %q: %w", pragma, err)
		}
	}

	schema := `
	CREATE TABLE IF NOT EXISTS webmentions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		source TEXT NOT NULL,
		target TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(source, target)
	);
	`

	if _, err := db.Exec(schema); err != nil {
		return nil, fmt.Errorf("failed to create table: %w", err)
	}

	return db, nil
}

func SaveWebmention(ctx context.Context, db *sql.DB, source, target string) error {
	query := `
	INSERT INTO webmentions (source, target, created_at)
	VALUES (?, ?, CURRENT_TIMESTAMP)
	ON CONFLICT (source, target) DO UPDATE SET 
			created_at = CURRENT_TIMESTAMP;
	`
	_, err := db.ExecContext(ctx, query, source, target)
	return err
}

func DeleteWebmention(ctx context.Context, db *sql.DB, source, target string) error {
	query := `DELETE FROM webmentions WHERE source = ? AND target = ?;`
	_, err := db.ExecContext(ctx, query, source, target)
	return err
}

func GetWebmentionsByTarget(ctx context.Context, db *sql.DB, target string) ([]*Webmention, error) {
	query := `
		SELECT id, source, target, created_at
		FROM webmentions
		WHERE target = ?
		ORDER BY created_at ASC;
	`
	rows, err := db.QueryContext(ctx, query, target)
	if err != nil {
		return nil, fmt.Errorf("failed to query webmentions: %w", err)
	}
	defer rows.Close()

	var results []*Webmention
	for rows.Next() {
		var webmention Webmention
		if err := rows.Scan(&webmention.ID, &webmention.Source, &webmention.Target, &webmention.CreatedAt); err != nil {
			return nil, fmt.Errorf("failed to scan webmention: %w", err)
		}
		results = append(results, &webmention)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to fetch all webmentions: %w", err)
	}
	return results, nil
}
