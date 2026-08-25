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
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestWebmentionHandler_PrivateMentionsGuard(t *testing.T) {
	t.Parallel()

	app := &App{
		c: &WebmentionsConfig{
			AllowedTargets: map[string]bool{
				"example.com": true,
			},
			AllowPrivateMentions: false,
		},
	}

	formData := url.Values{
		"source": {"https://source.com/post"},
		"target": {"https://example.com/post"},
		"code":   {"abc123"},
	}
	req := httptest.NewRequest(http.MethodPost, "/webmention", strings.NewReader(formData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	recorder := httptest.NewRecorder()
	app.webmentionHandler(recorder, req)

	if recorder.Code != http.StatusBadRequest {
		t.Errorf("expected %d but got %d", http.StatusForbidden, recorder.Code)
	}

	expectedError := "Private mentions are not allowed"
	if !strings.Contains(recorder.Body.String(), expectedError) {
		t.Errorf("expected error %s but got %s", expectedError, recorder.Body.String())
	}
}
