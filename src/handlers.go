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
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"time"
)

type WebmentionView struct {
	Source       string
	DisplayLabel string
	DateStr      string
}

type WebmentionRequest struct {
	Target url.URL
	Source url.URL
	Code   string
	Realm  string
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
		status, tokenResponse, err := VerifyWebmention(a.c, &wr, a.db)
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
