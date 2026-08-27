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
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type App struct {
	db          *sql.DB
	c           *WebmentionsConfig
	idleWatcher *IdleWatcher
	workersWg   sync.WaitGroup
}

const WebmentionRateLimit = 6 * time.Second
const WebmentionBurst = 3

const GetWebmentionsRateLimit = 1 * time.Second
const GetWebmentionsBurst = 10

func serve(c *WebmentionsConfig) {
	db, err := InitDB(c.DbPath)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer db.Close()
	idleWatcher := newIdleWatcher(5 * time.Minute)
	app := &App{db: db, c: c, idleWatcher: idleWatcher}

	mux := http.NewServeMux()
	webmentionLimiter := newIPRateLimiter(rate.Every(WebmentionRateLimit), WebmentionBurst)
	mux.HandleFunc("/webmention", webmentionLimiter.limitMiddleware(app.webmentionHandler))

	getWebmentionsLimiter := newIPRateLimiter(rate.Every(GetWebmentionsRateLimit), GetWebmentionsBurst)
	mux.HandleFunc("/get_webmentions", getWebmentionsLimiter.limitMiddleware(app.getWebmentionsHandler))

	// WARN: This must be secured in the reverse
	// proxy layer!
	getPrivateWebmentionsLimiter := newIPRateLimiter(rate.Every(GetWebmentionsRateLimit), GetWebmentionsBurst)
	mux.HandleFunc("/get_private_webmentions", getPrivateWebmentionsLimiter.limitMiddleware(app.getPrivateWebmentionsHandler))

	server := &http.Server{
		Handler: idleWatcher.Middleware(mux),
	}
	go idleWatcher.StartWatchdog(context.Background(), server)

	f := os.NewFile(3, "systemd socket")
	listener, err := net.FileListener(f)
	if err != nil {
		log.Fatalf("failed to create listener from systemd socket: %v", err)
	}

	log.Println("Server started on socket activation fd")
	if err := server.Serve(listener); err != http.ErrServerClosed {
		log.Fatalf("server failed: %v", err)
	}

	log.Println("Server HTTP listener closed, waiting for background workers to complete...")
	app.workersWg.Wait()
	log.Println("Server exited cleanly on idle")
}

func revalidate(c *WebmentionsConfig) {
	db, err := InitDB(c.DbPath)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer db.Close()
	mentions, err := GetAllWebmentions(context.Background(), db)
	if err != nil {
		log.Fatalf("failed to fetch webmentions: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for _, mention := range mentions {
		sourceURL, err := parseAndValidateURL(mention.Source)
		if err != nil {
			log.Printf("[revalidate] invalid URL, deleting from database: %v", err)
			if err := DeleteWebmention(ctx, db, mention.Source, mention.Target); err != nil {
				log.Printf("[revalidate] error deleting from database: %v", err)
			}
			continue
		}
		targetURL, err := parseAndValidateURL(mention.Target)
		if err != nil {
			log.Printf("[revalidate] invalid URL, deleting from database: %v", err)
			if err := DeleteWebmention(ctx, db, mention.Source, mention.Target); err != nil {
				log.Printf("[revalidate] error deleting from database: %v", err)
			}
			continue
		}

		// Private Webmentions cannot be revalidated,
		// since a code is required to get the token.
		//
		// Although we could have a realm that has an
		// unexpired token, it's easier just to treat
		// Private Webmentions as something that has
		// to be updated by the author.
		if mention.IsPrivate {
			continue
		}
		wr := WebmentionRequest{
			Source: *sourceURL,
			Target: *targetURL,
		}
		status, _, err := VerifyWebmention(c, &wr, db)
		if err != nil {
			log.Printf("[revalidate] fetch/verification error for %s -> %s: %v", mention.Source, mention.Target, err)
			continue
		}
		if status == StatusDelete {
			log.Printf("[revalidate] link removed, deleting: %s -> %s", mention.Source, mention.Target)
			if err := DeleteWebmention(ctx, db, sourceURL.String(), targetURL.String()); err != nil {
				log.Printf("[revalidate] error deleting from database: %v", err)
			}
		}
	}
}

func runMigrations(c *WebmentionsConfig) {
	db, err := InitDB(c.DbPath)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	log.Println("[migrate] checking and applying database migrations...")
	if err := migrate(ctx, db); err != nil {
		log.Fatalf("Failed to apply migrations: %v", err)
	}
	log.Println("[migrate] migrations applied successfully")
}

func main() {
	mode := "serve"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}

	c, err := newConfig()
	if err != nil {
		log.Fatalf("Failed to fetch config: %v", err)
	}

	switch mode {
	case "serve":
		serve(c)
	case "revalidate":
		revalidate(c)
	case "dump-config":
		fmt.Printf("Config loaded successfully:\n%+v\n", c)
		fmt.Printf("%#v\n", c.AllowedTargets)
	default:
		fmt.Fprintf(os.Stderr, "Usage: %s [serve|revalidate|dump-config]\n", os.Args[0])
		os.Exit(1)
	}
}
