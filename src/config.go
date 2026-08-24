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
	AllowedTargets       map[string]bool
	UserAgent            string
	MaxTimeout           time.Duration
	MaxFetchSizeBytes    int64
	DbPath               string
	AllowPrivateMentions bool
}

func newConfig() (*WebmentionsConfig, error) {
	config := &WebmentionsConfig{
		AllowedTargets:       map[string]bool{},
		UserAgent:            "Mausul Webmention-Receiver Bot/1.0",
		MaxTimeout:           10 * time.Second,
		MaxFetchSizeBytes:    1024 * 1024, // 1MB
		AllowPrivateMentions: false,
	}

	allowedTargets := strings.TrimSpace(os.Getenv("MAUSUL_ALLOWED_TARGETS"))

	if allowedTargets != "" {
		for _, target := range strings.Split(allowedTargets, ",") {
			if target := strings.TrimSpace(target); target == "" {
				continue
			}
			config.AllowedTargets[strings.ToLower(target)] = true
		}
	}

	if ua := os.Getenv("MAUSUL_USER_AGENT"); ua != "" {
		config.UserAgent = os.Getenv("MAUSUL_USER_AGENT")
	}

	if envTimeout := os.Getenv("MAUSUL_MAX_TIMEOUT"); envTimeout != "" {
		maxTimeout, err := time.ParseDuration(envTimeout)
		if err != nil {
			return nil, err
		}
		config.MaxTimeout = maxTimeout
	}

	if envFetchSizeBytes := os.Getenv("MAUSUL_MAX_FETCH_SIZE_BYTES"); envFetchSizeBytes != "" {
		maxFetchSizeBytes, err := strconv.Atoi(envFetchSizeBytes)
		if err != nil {
			return nil, err
		}
		config.MaxFetchSizeBytes = int64(maxFetchSizeBytes)
	}

	if envAllowPrivateMentions := os.Getenv("MAUSUL_ALLOW_PRIVATE_MENTIONS"); envAllowPrivateMentions != "" {
		val, err := strconv.ParseBool(envAllowPrivateMentions)
		if err != nil {
			return nil, err
		}
		config.AllowPrivateMentions = val
	}

	dbPath := os.Getenv("MAUSUL_DB_PATH")
	if dbPath == "" {
		return nil, errors.New("MAUSUL_DB_PATH environment variable must be set")
	}
	config.DbPath = dbPath
	return config, nil
}
