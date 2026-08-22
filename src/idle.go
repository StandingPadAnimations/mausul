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
	"log"
	"net/http"
	"sync/atomic"
	"time"
)

type IdleWatcher struct {
	lastActivity atomic.Int64
	timeout      time.Duration
}

func newIdleWatcher(timeout time.Duration) *IdleWatcher {
	w := &IdleWatcher{
		timeout: timeout,
	}
	w.RecordActivity()
	return w
}

func (w *IdleWatcher) RecordActivity() {
	w.lastActivity.Store(time.Now().UnixNano())
}

func (w *IdleWatcher) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, r *http.Request) {
		w.RecordActivity()
		next.ServeHTTP(rw, r)
	})
}

func (w *IdleWatcher) StartWatchdog(ctx context.Context, server *http.Server) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			last := time.Unix(0, w.lastActivity.Load())
			if time.Since(last) > w.timeout {
				log.Printf("[idle] no requests for %v, shutting down...", w.timeout)

				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				if err := server.Shutdown(shutdownCtx); err != nil {
					log.Printf("[idle] failed to shutdown server: %v", err)
				}
				cancel()
				return
			}
		}
	}
}
