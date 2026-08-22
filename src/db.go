// Copyright (C) 2026 Maryam Stellamaris
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
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
	params := url.Values{}
	params.Add("_pragma", "busy_timeout(5000)")
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "synchronous(NORMAL)")
	params.Add("_pragma", "foreign_keys(ON)")
	params.Add("_txlock", "immediate")
	dsn := fmt.Sprintf("%s?%s", dbPath, params.Encode())
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
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

func GetAllWebmentions(ctx context.Context, db *sql.DB) ([]*Webmention, error) {
	query := `
		SELECT id, source, target, created_at
		FROM webmentions
		ORDER BY created_at ASC;
	`
	rows, err := db.QueryContext(ctx, query)
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
