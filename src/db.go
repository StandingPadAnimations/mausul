// Copyright (C) 2026 Maryam Stellamaris <maryam@standingpad.org>
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
	"errors"
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
	IsPrivate bool
}

func InitDB(dbPath string) (*sql.DB, error) {
	params := url.Values{}
	params.Add("_pragma", "busy_timeout(5000)")
	params.Add("_pragma", "journal_mode(WAL)")
	params.Add("_pragma", "synchronous(NORMAL)")
	params.Add("_pragma", "foreign_keys(ON)")
	params.Add("_txlock", "immediate")
	dsn := fmt.Sprintf("file:%s?%s", dbPath, params.Encode())
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	ctx := context.Background()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to run database migrations: %w", err)
	}
	return db, nil
}

func SaveWebmention(ctx context.Context, db *sql.DB, wr *WebmentionRequest) error {
	query := `
	INSERT INTO webmentions (source, target, created_at, is_private)
	VALUES (?, ?, CURRENT_TIMESTAMP, ?)
	ON CONFLICT (source, target) DO UPDATE SET 
			is_private = excluded.is_private;
	`
	_, err := db.ExecContext(ctx, query, wr.Source.String(), wr.Target.String(), IsPrivate(wr))
	return err
}

func DeleteWebmention(ctx context.Context, db *sql.DB, source, target string) error {
	query := `DELETE FROM webmentions WHERE source = ? AND target = ?;`
	_, err := db.ExecContext(ctx, query, source, target)
	return err
}

func GetWebmentionsByTarget(ctx context.Context, db *sql.DB, target string, private bool) ([]*Webmention, error) {
	query := `
		SELECT id, source, target, created_at, is_private
		FROM webmentions
		WHERE target = ?
		  AND is_private = ?
		ORDER BY created_at ASC;
	`
	rows, err := db.QueryContext(ctx, query, target, private)
	if err != nil {
		return nil, fmt.Errorf("failed to query webmentions: %w", err)
	}
	defer rows.Close()

	var results []*Webmention
	for rows.Next() {
		var webmention Webmention
		if err := rows.Scan(&webmention.ID, &webmention.Source, &webmention.Target, &webmention.CreatedAt, &webmention.IsPrivate); err != nil {
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
		SELECT id, source, target, created_at, is_private
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
		webmention := &Webmention{}
		if err := rows.Scan(&webmention.ID, &webmention.Source, &webmention.Target, &webmention.CreatedAt, &webmention.IsPrivate); err != nil {
			return nil, fmt.Errorf("failed to scan webmention: %w", err)
		}
		results = append(results, webmention)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("failed to fetch all webmentions: %w", err)
	}
	return results, nil
}

func StoreToken(ctx context.Context, db *sql.DB, realm string, tokenResponse *TokenResponse) error {
	query := `
		INSERT INTO tokens (realm, access_token, created_at, updated_at, expires_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT (realm) DO UPDATE SET
				access_token = excluded.access_token,
				expires_at = excluded.expires_at,
				updated_at = excluded.updated_at;
	`
	var expiresAt any
	if tokenResponse.ExpiresIn > 0 {
		expiresAt = tokenResponse.ReceivedAt.Add(time.Duration(tokenResponse.ExpiresIn) * time.Second)
	}
	_, err := db.ExecContext(ctx, query, realm, tokenResponse.AccessToken, tokenResponse.ReceivedAt, tokenResponse.ReceivedAt, expiresAt)
	if err != nil {
		return fmt.Errorf("failed to store token: %w", err)
	}
	return nil
}

func GetToken(ctx context.Context, db *sql.DB, realm string) (*TokenResponse, error) {
	query := `
		SELECT access_token, expires_at, updated_at
		FROM tokens
		WHERE realm = ?
	`
	var tokenResponse TokenResponse
	var expiresAt sql.NullTime
	var updatedAt sql.NullTime

	err := db.QueryRowContext(ctx, query, realm).Scan(&tokenResponse.AccessToken, &expiresAt, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("no token found for realm")
		}
		return nil, fmt.Errorf("failed to get token: %w", err)
	}

	if updatedAt.Valid {
		tokenResponse.ReceivedAt = updatedAt.Time
	}

	if expiresAt.Valid {
		remaining := time.Until(expiresAt.Time).Seconds()

		// If the token is 15 seconds from expiration,
		// treat it as expired. Due to several factors
		// (such as network issues), we can't assume
		// all non-zero values are non-expired. 15 seconds
		// is a reasonable buffer.
		if remaining <= 15 {
			return nil, fmt.Errorf("token expired")
		}
		tokenResponse.ExpiresIn = int64(remaining)
	}
	return &tokenResponse, nil
}
