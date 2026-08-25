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
	"fmt"
	"math"
	"net/url"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func setupTestDB(t *testing.T) (*sql.DB, *WebmentionsConfig) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	c := &WebmentionsConfig{DbPath: dbPath}

	db, err := InitDB(c.DbPath)
	if err != nil {
		t.Fatalf("Failed to initialize test database: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
	})
	return db, c
}

func TestServerAndRevalidatorConcurrency(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "concurrency_test.db")
	db, err := InitDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			for j := 0; j < 10; j++ {
				wr := &WebmentionRequest{
					Source: url.URL{Scheme: "https", Host: fmt.Sprintf("source-%d-%d.com", workerID, j)},
					Target: url.URL{Scheme: "https", Host: "target.com"},
					Code:   "",
				}

				err := SaveWebmention(ctx, db, wr)
				if err != nil {
					t.Errorf("Server worker failed to save: %v", err)
				}

				time.Sleep(5 * time.Millisecond)
			}
		}(i)
	}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			mentions, err := GetAllWebmentions(ctx, db)
			if err != nil {
				t.Errorf("Revalidator failed to fetch mentions: %v", err)
				return
			}

			// Simulate dead links
			for idx, m := range mentions {
				if idx%3 == 0 {
					err := DeleteWebmention(ctx, db, m.Source, m.Target)
					if err != nil {
						t.Errorf("Revalidator failed to delete mention: %v", err)
					}
				}
			}
		}()
	}

	wg.Wait()
}

func TestDatabase_WebmentionLifecycle(t *testing.T) {
	t.Parallel()
	db, _ := setupTestDB(t)
	ctx := context.Background()

	targetStr := "https://target.com/post/1"
	sourceStr := "https://source.com/post/3"

	targetURL, _ := parseAndValidateURL(targetStr)
	sourceURL, _ := parseAndValidateURL(sourceStr)

	// Test public Webmentions first
	wr := &WebmentionRequest{
		Source: *sourceURL,
		Target: *targetURL,
		Code:   "",
	}

	if err := SaveWebmention(ctx, db, wr); err != nil {
		t.Fatalf("Failed to save public Webmention: %v", err)
	}

	// Fetch public Webmentions
	mentions, err := GetWebmentionsByTarget(ctx, db, targetStr, false)
	if err != nil {
		t.Fatalf("Failed to fetch public Webmentions: %v", err)
	}

	if len(mentions) != 1 {
		t.Errorf("Expected 1 public Webmention, got %d", len(mentions))
	}

	// Test update to private Webmention
	wr.Code = "private"
	if err := SaveWebmention(ctx, db, wr); err != nil {
		t.Fatalf("Failed to update Webmention to private: %v", err)
	}

	mentions, err = GetWebmentionsByTarget(ctx, db, targetStr, false)
	if err != nil {
		t.Fatalf("Failed to fetch updated Webmention: %v", err)
	}
	if len(mentions) != 0 {
		t.Errorf("Expected 0 public Webmentions after update, got %d", len(mentions))
	}

	mentions, err = GetWebmentionsByTarget(ctx, db, targetStr, true)
	if err != nil {
		t.Fatalf("Failed to fetch private Webmention: %v", err)
	}
	if len(mentions) != 1 {
		t.Errorf("Expected 1 private Webmention, got %d", len(mentions))
	}
	if !mentions[0].IsPrivate {
		t.Errorf("Expected private Webmention to be private, got public")
	}

	if err := DeleteWebmention(ctx, db, sourceStr, targetStr); err != nil {
		t.Fatalf("Failed to delete Webmention: %v", err)
	}

	mentionsPublic, _ := GetWebmentionsByTarget(ctx, db, targetStr, false)
	mentionsPrivate, _ := GetWebmentionsByTarget(ctx, db, targetStr, true)
	if len(mentionsPublic) != 0 || len(mentionsPrivate) != 0 {
		t.Error("expected database to be empty after deletion")
	}
}

func TestDatabase_TokenLifecycle(t *testing.T) {
	t.Parallel()
	db, _ := setupTestDB(t)
	ctx := context.Background()

	realm := "abcd"
	initial_token := &TokenResponse{
		AccessToken: "fancy-secret-token-ooooo",
		ExpiresIn:   3600,
		ReceivedAt:  time.Now().UTC(),
	}

	if err := StoreToken(ctx, db, realm, initial_token); err != nil {
		t.Fatalf("Failed to store inital token: %v", err)
	}

	token, err := GetToken(ctx, db, realm)
	if err != nil {
		t.Fatalf("Failed to fetch token: %v", err)
	}

	if token.AccessToken != initial_token.AccessToken {
		t.Errorf("Expected token to %q, got %q", initial_token.AccessToken, token.AccessToken)
	}

	if diff := math.Abs(float64(token.ExpiresIn) - 3600); diff >= 5 {
		t.Errorf("expected remaining expiration near 3600s, got %d", token.ExpiresIn)
	}

	updatedToken := &TokenResponse{
		AccessToken: "fancy-secret-token-ooooo",
		ExpiresIn:   7200,
		ReceivedAt:  time.Now().UTC(),
	}

	if err := StoreToken(ctx, db, realm, updatedToken); err != nil {
		t.Fatalf("Failed to update token: %v", err)
	}

	newToken, err := GetToken(ctx, db, realm)

	if err != nil {
		t.Fatalf("Failed to fetch updated token: %v", err)
	}

	if newToken.AccessToken != updatedToken.AccessToken {
		t.Errorf("Expected token to %q, got %q", updatedToken.AccessToken, newToken.AccessToken)
	}

	if diff := math.Abs(float64(newToken.ExpiresIn) - 7200); diff >= 5 {
		t.Errorf("expected remaining expiration near 7200s, got %d", newToken.ExpiresIn)
	}
}
