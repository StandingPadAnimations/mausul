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
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type clientRecord struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type IPRateLimiter struct {
	mu      sync.Mutex
	clients map[string]*clientRecord
	r       rate.Limit
	b       int
}

func newIPRateLimiter(r rate.Limit, b int) *IPRateLimiter {
	i := &IPRateLimiter{
		clients: make(map[string]*clientRecord),
		r:       r,
		b:       b,
	}
	go i.cleanupStaleClients(5 * time.Minute)
	return i
}

func (i *IPRateLimiter) getClient(ip string) *rate.Limiter {
	i.mu.Lock()
	defer i.mu.Unlock()
	c, exists := i.clients[ip]
	if !exists {
		limiter := rate.NewLimiter(i.r, i.b)
		i.clients[ip] = &clientRecord{limiter: limiter, lastSeen: time.Now()}
		return limiter
	}
	c.lastSeen = time.Now()
	return c.limiter
}

func (i *IPRateLimiter) cleanupStaleClients(interval time.Duration) {
	for {
		time.Sleep(interval)
		i.mu.Lock()
		for ip, c := range i.clients {
			if time.Since(c.lastSeen) > interval {
				delete(i.clients, ip)
			}
		}
		i.mu.Unlock()
	}
}

func (i *IPRateLimiter) limitMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		limiter := i.getClient(ip)
		if !limiter.Allow() {
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

func extractIP(r *http.Request) string {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		clientIP := strings.TrimSpace(parts[0])
		if clientIP != "" {
			return clientIP
		}
	}
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}
