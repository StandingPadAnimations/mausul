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
	"html/template"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/coreos/go-systemd/v22/activation"
	"golang.org/x/time/rate"
)

type App struct {
	db          *sql.DB
	c           *WebmentionsConfig
	idleWatcher *IdleWatcher
	workersWg   sync.WaitGroup
}

type WebmentionRequest struct {
	Target url.URL
	Source url.URL
	Code   string
	Realm  string
}

type WebmentionView struct {
	Source       string
	DisplayLabel string
	DateStr      string
}

var mentionsTmpl = template.Must(template.New("mentions").Parse(`
{{- if not . -}}
<p class="webmentions_empty">No webmentions yet.</p>
{{- else -}}
<ul class="webmentions_container">
    {{- range . }}
    <li class="webmention">
        <a href="{{ .Source }}" rel="nofollow noopener noreferrer" class="webmention_link">{{ .DisplayLabel }}</a>
        <time class="webmention_date" datetime="{{ .DateStr }}">{{ .DateStr }}</time>
    </li>
    {{- end }}
</ul>
{{- end -}}
`))

func (a *App) getWebmentionsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rawTarget := r.URL.Query().Get("target")
	if rawTarget == "" {
		http.Error(w, "Query parameter 'target' is required", http.StatusBadRequest)
		return
	}

	targetURL, err := parseAndValidateURL(rawTarget)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid target URL: %v", err), http.StatusBadRequest)
		return
	}

	if !IsAllowedTarget(a.c, targetURL) {
		http.Error(w, fmt.Sprintf("Target URL %s is not allowed.", targetURL.String()), http.StatusBadRequest)
		return
	}

	// NO PRIVATE WEBMENTIONS
	mentions, err := GetWebmentionsByTarget(r.Context(), a.db, targetURL.String(), false)
	if err != nil {
		http.Error(w, "Failed to retrieve webmentions", http.StatusInternalServerError)
		return
	}

	var viewItems []WebmentionView
	for _, m := range mentions {
		displayLabel := m.Source
		if parsedSource, err := url.Parse(m.Source); err == nil && parsedSource.Host != "" {
			displayLabel = parsedSource.Host
		}

		dateStr := m.CreatedAt.Format("2006-01-02")

		viewItems = append(viewItems, WebmentionView{
			Source:       m.Source,
			DisplayLabel: displayLabel,
			DateStr:      dateStr,
		})
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := mentionsTmpl.Execute(w, viewItems); err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
		return
	}
}

func (a *App) webmentionHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	rawSource := r.FormValue("source")
	rawTarget := r.FormValue("target")

	if rawSource == "" || rawTarget == "" {
		http.Error(w, "Source and target URLs are required", http.StatusBadRequest)
		return
	}

	sourceURL, err := parseAndValidateURL(rawSource)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid source URL: %s", err), http.StatusBadRequest)
		return
	}

	targetURL, err := parseAndValidateURL(rawTarget)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid target URL: %s", err), http.StatusBadRequest)
		return
	}

	if sourceURL.String() == targetURL.String() {
		http.Error(w, "Source and target URLs cannot be the same", http.StatusBadRequest)
		return
	}

	if !IsAllowedTarget(a.c, targetURL) {
		http.Error(w, fmt.Sprintf("Target domain'%s' is not allowed", targetURL.Host), http.StatusBadRequest)
		return
	}

	wr := WebmentionRequest{
		Source: *sourceURL,
		Target: *targetURL,
		Code:   r.FormValue("code"),
		Realm:  r.FormValue("realm"),
	}

	if !a.c.AllowPrivateMentions && wr.Code != "" {
		http.Error(w, "Private mentions are not allowed", http.StatusBadRequest)
		return
	}

	a.workersWg.Add(1)
	go func() {
		defer a.workersWg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		log.Printf("[worker] verifying mention: %s -> %s", sourceURL.String(), targetURL.String())
		status, tokenResponse, err := VerifyWebmention(a.c, &wr)
		if err != nil {
			log.Printf("[worker] failed to verify mention: %s", err)
			return
		}
		a.idleWatcher.RecordActivity()

		if status == StatusKeep {
			log.Printf("[worker] mention verified: %s -> %s", sourceURL.String(), targetURL.String())
			if err := SaveWebmention(ctx, a.db, &wr); err != nil {
				log.Printf("[worker] error saving to database: %v", err)
			}
		} else {
			log.Printf("[worker] mention invalid or removed, deleting: %s -> %s", sourceURL.String(), targetURL.String())
			if err := DeleteWebmention(ctx, a.db, sourceURL.String(), targetURL.String()); err != nil {
				log.Printf("[worker] error deleting from database: %v", err)
			}
		}

		if wr.Realm != "" && tokenResponse != nil {
			log.Printf("[worker] saving token: %s -> %s", sourceURL.String(), targetURL.String())
			if err := StoreToken(ctx, a.db, wr.Realm, tokenResponse); err != nil {
				log.Printf("[worker] error saving token: %v", err)
			}
		}
		a.idleWatcher.RecordActivity()
	}()
	w.WriteHeader(http.StatusAccepted)
}

func serve(c *WebmentionsConfig) {
	db, err := InitDB(c.DbPath)
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer db.Close()
	idleWatcher := newIdleWatcher(5 * time.Minute)
	app := &App{db: db, c: c, idleWatcher: idleWatcher}

	mux := http.NewServeMux()
	webmentionLimiter := newIPRateLimiter(rate.Every(6*time.Second), 3)
	mux.HandleFunc("/webmention", webmentionLimiter.limitMiddleware(app.webmentionHandler))

	getwebmentionsLimiter := newIPRateLimiter(rate.Every(1*time.Second), 10)
	mux.HandleFunc("/get_webmentions", getwebmentionsLimiter.limitMiddleware(app.getWebmentionsHandler))

	server := &http.Server{
		Handler: idleWatcher.Middleware(mux),
	}
	go idleWatcher.StartWatchdog(context.Background(), server)

	listeners, err := activation.Listeners()
	if err != nil {
		panic(err)
	}
	if len(listeners) == 0 {
		panic("No socket activation fds found")
	}
	log.Println("Server started on socket activation fd")
	var wg sync.WaitGroup
	for _, l := range listeners {
		defer l.Close()
		wg.Add(1)
		go func(listener net.Listener) {
			defer wg.Done()
			if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
				log.Printf("server failed: %v", err)
			}
		}(l)
	}

	wg.Wait()

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
		status, _, err := VerifyWebmention(c, &wr)
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
