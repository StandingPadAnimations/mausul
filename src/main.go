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
	"html/template"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/coreos/go-systemd/v22/activation"
	"golang.org/x/time/rate"
)

type App struct {
	db *sql.DB
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

	if !IsAllowedTarget(targetURL) {
		http.Error(w, fmt.Sprintf("Target URL %s is not allowed.", targetURL.String()), http.StatusBadRequest)
		return
	}

	mentions, err := GetWebmentionsByTarget(r.Context(), a.db, targetURL.String())
	if err != nil {
		http.Error(w, "Failed to retrieve webmentions", http.StatusInternalServerError)
		return
	}

	// Prepare data for the template matching Python's netloc and date extraction
	var viewItems []WebmentionView
	for _, m := range mentions {
		displayLabel := m.Source
		if parsedSource, err := url.Parse(m.Source); err == nil && parsedSource.Host != "" {
			displayLabel = parsedSource.Host
		}

		// "2006-01-02" formats as YYYY-MM-DD
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

	if !IsAllowedTarget(targetURL) {
		http.Error(w, fmt.Sprintf("Target domain'%s' is not allowed", targetURL.Host), http.StatusBadRequest)
		return
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		log.Printf("[worker] verifying mention: %s -> %s", sourceURL.String(), targetURL.String())
		valid, err := VerifyWebmention(sourceURL, targetURL)
		if err != nil {
			log.Printf("[worker] failed to verify mention: %s", err)
			return
		}
		if valid {
			log.Printf("[worker] mention verified: %s -> %s", sourceURL.String(), targetURL.String())
			if err := SaveWebmention(ctx, a.db, sourceURL.String(), targetURL.String()); err != nil {
				log.Printf("[worker] error saving to database: %v", err)
			}
		} else {
			log.Printf("[worker] mention invalid or removed, deleting: %s -> %s", sourceURL.String(), targetURL.String())
			if err := DeleteWebmention(ctx, a.db, sourceURL.String(), targetURL.String()); err != nil {
				log.Printf("[worker] error deleting from database: %v", err)
			}
		}
	}()

	w.WriteHeader(http.StatusAccepted)

	// Simulate sending a webmention request
	// This is a placeholder for actual webmention sending logic
	fmt.Printf("Sending webmention from %s to %s\n", sourceURL, targetURL)
}

func main() {
	listeners, err := activation.Listeners()
	if err != nil {
		panic(err)
	}
	if len(listeners) != 1 {
		panic("Unexpected number of socket activation fds")
	}
	db, err := InitDB("webmentions.db")
	if err != nil {
		log.Fatalf("failed to initialize database: %v", err)
	}
	defer db.Close()
	app := &App{db: db}

	http.HandleFunc("/", homeHandler)

	webmentionLimiter := newIPRateLimiter(rate.Every(10*time.Minute), 3)
	http.HandleFunc("/webmention", webmentionLimiter.limitMiddleware(app.webmentionHandler))

	getwebmentionsLimiter := newIPRateLimiter(rate.Every(60*time.Minute), 3)
	http.HandleFunc("/get_webmentions", getwebmentionsLimiter.limitMiddleware(app.getWebmentionsHandler))

	log.Println("Server is running on port :8000")
	http.Serve(listeners[0], nil)
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, `<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <title>Webmention Receiver Test</title>
    <link rel="webmention" href="/webmention">
    <link rel="me" href="https://github.com/StandingPadAnimations">
</head>
<body>
    <h1>Webmention Receiver</h1>
    <p><a href="https://github.com/StandingPadAnimations" rel="me">GitHub Profile</a></p>
</body>
</html>`)
}
