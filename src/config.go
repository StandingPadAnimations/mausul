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
