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
	"testing"
	"time"

	"golang.org/x/time/rate"
)

func TestRateLimiter_BurstAndReject(t *testing.T) {
	t.Parallel()

	limiter := newIPRateLimiter(rate.Every(1*time.Second), 3)
	handler := limiter.limitMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/webmention", nil)
	req.RemoteAddr = "192.168.1.50:12345"

	for i := 0; i < 3; i++ {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK {
			t.Errorf("Expecting request %d to pass, got: %d", i+1, recorder.Code)
		}
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	if recorder.Code != http.StatusTooManyRequests {
		t.Errorf("Expecting request 4th to fail, got: %d", recorder.Code)
	}
}

func TestRateLimiter_XForwardedForExtraction(t *testing.T) {
	t.Parallel()

	limiter := newIPRateLimiter(rate.Every(1*time.Second), 3)
	handler := limiter.limitMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/webmention", nil)
		req.RemoteAddr = "127.0.0.1:8060"
		req.Header.Set("X-Forwarded-For", "203.0.113.5, 198.51.100.1")

		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Errorf("Expecting request %d to pass, got: %d", i+1, rec.Code)
		}
	}

	reqFail := httptest.NewRequest(http.MethodGet, "/webmention", nil)
	reqFail.RemoteAddr = "127.0.0.1:8060"
	reqFail.Header.Set("X-Forwarded-For", "203.0.113.5, 198.51.100.1")

	recFail := httptest.NewRecorder()
	handler.ServeHTTP(recFail, reqFail)
	if recFail.Code != http.StatusTooManyRequests {
		t.Errorf("Expecting request 4th to fail with 429, got: %d", recFail.Code)
	}

	reqNew := httptest.NewRequest(http.MethodGet, "/webmention", nil)
	reqNew.RemoteAddr = "127.0.0.1:8060" // Simulating reverse proxy
	reqNew.Header.Set("X-Forwarded-For", "198.51.100.22")

	recNew := httptest.NewRecorder()
	handler.ServeHTTP(recNew, reqNew)
	if recNew.Code != http.StatusOK {
		t.Errorf("Expecting request from new IP to pass, got: %d", recNew.Code)
	}
}
