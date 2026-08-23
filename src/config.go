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
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type WebmentionsConfig struct {
	AllowedTargets    map[string]bool
	UserAgent         string
	MaxTimeout        time.Duration
	MaxFetchSizeBytes int64
	DbPath            string
}

func newConfig() (*WebmentionsConfig, error) {
	config := &WebmentionsConfig{
		AllowedTargets:    map[string]bool{},
		UserAgent:         "Maryam's Webmention-Receiver/1.0",
		MaxTimeout:        10 * time.Second,
		MaxFetchSizeBytes: 1024 * 1024, // 1MB
	}

	allowedTargets := strings.TrimSpace(os.Getenv("WEBMENTIONS_ALLOWED_TARGETS"))

	if allowedTargets != "" {
		for _, target := range strings.Split(allowedTargets, ",") {
			if target == "" {
				continue
			}
			config.AllowedTargets[strings.ToLower(target)] = true
		}
	}
	config.UserAgent = os.Getenv("WEBMENTIONS_USER_AGENT")

	maxTimeout, err := time.ParseDuration(os.Getenv("WEBMENTIONS_MAX_TIMEOUT"))
	if err != nil {
		return nil, err
	}
	config.MaxTimeout = maxTimeout

	maxFetchSizeBytes, err := strconv.Atoi(os.Getenv("WEBMENTIONS_MAX_FETCH_SIZE_BYTES"))
	if err != nil {
		return nil, err
	}
	config.MaxFetchSizeBytes = int64(maxFetchSizeBytes)

	dbPath := os.Getenv("WEBMENTIONS_DB_PATH")
	if dbPath == "" {
		return nil, errors.New("WEBMENTIONS_DB_PATH environment variable must be set")
	}
	config.DbPath = dbPath
	return config, nil
}
