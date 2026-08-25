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
	"strings"
	"testing"
	"time"
)

func TestIsAllowedTarget(t *testing.T) {
	t.Parallel()
	c := &WebmentionsConfig{
		AllowedTargets: map[string]bool{
			"standingpad.org": true,
			"example.com":     true,
		},
	}

	tests := []struct {
		name     string
		rawURL   string
		expected bool
	}{
		{"Allowed exact match", "https://standingpad.org/post/1", true},
		{"Disallowed domain", "https://attacker.com/post/1", false},
		{"Allowed mixed case", "https://ExAmPlE.CoM/path", true},
		{"Unmapped subdomain", "https://api.standingpad.org", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			u, err := parseAndValidateURL(tc.rawURL)
			if err != nil {
				t.Fatalf("setup failed, invalid url: %v", err)
			}

			result := IsAllowedTarget(c, u)
			if result != tc.expected {
				t.Errorf("expected %v but got %v", tc.expected, result)
			}
		})
	}
}

func TestIsPrivateWebmention(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		request  WebmentionRequest
		expected bool
	}{
		{"Public Mention", WebmentionRequest{Code: ""}, false},
		{"Private Mention", WebmentionRequest{Code: "private"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := IsPrivate(&tc.request)
			if result != tc.expected {
				t.Errorf("expected %v but got %v", tc.expected, result)
			}
		})
	}
}

func TestContainsTargetLink(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		htmlBody string
		target   string
		expected bool
	}{
		{
			name:     "Standard anchor tag",
			htmlBody: `<html><body><a href="https://example.com/target">Link</a></body></html>`,
			target:   "https://example.com/target",
			expected: true,
		},
		{
			name:     "Anchor tag with trailing slash normalization",
			htmlBody: `<html><body><a href="https://example.com/target/">Link</a></body></html>`,
			target:   "https://example.com/target",
			expected: true,
		},
		{
			name:     "Target with trailing slash, HTML without",
			htmlBody: `<html><body><a href="https://example.com/target">Link</a></body></html>`,
			target:   "https://example.com/target/",
			expected: true,
		},
		{
			name:     "Video source tag",
			htmlBody: `<html><body><video><source src="https://example.com/video.mp4"></video></body></html>`,
			target:   "https://example.com/video.mp4",
			expected: true,
		},
		{
			name:     "Image tag",
			htmlBody: `<html><body><img src="https://example.com/image.png" alt="test"></body></html>`,
			target:   "https://example.com/image.png",
			expected: true,
		},
		{
			name:     "Target missing from page",
			htmlBody: `<html><body><a href="https://different.com/link">Link</a></body></html>`,
			target:   "https://example.com/target",
			expected: false,
		},
		{
			name:     "Malformed HTML still extracts valid link",
			htmlBody: `<div><p>Unclosed tags <a href="https://example.com/target">Link`,
			target:   "https://example.com/target",
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// strings.NewReader satisfies the io.Reader interface ContainsTargetLink expects
			reader := strings.NewReader(tc.htmlBody)

			result, err := ContainsTargetLink(reader, tc.target)
			if err != nil {
				t.Fatalf("unexpected error parsing HTML: %v", err)
			}

			if result != tc.expected {
				t.Errorf("expected %v but got %v", tc.expected, result)
			}
		})
	}
}

func TestSSRFDialer(t *testing.T) {
	t.Parallel()

	c := &WebmentionsConfig{MaxTimeout: 2 * time.Second}
	client := safeHTTPClient(c)

	targets := []string{
		"http://127.0.0.1",
		"http://localhost",
		"http://[::1]",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1",
		"http://192.168.1.100",
		"http://[::ffff:7f00:1]",
	}

	for _, target := range targets {
		t.Run(target, func(t *testing.T) {
			t.Parallel()

			_, err := client.Get(target)
			if err == nil {
				t.Fatalf("Expected SSRF block for %s, but request succeeded", target)
			}

			if !strings.Contains(err.Error(), "SSRF blocked") {
				t.Errorf("Expected SSRF blocked error, got: %v", err)
			}
		})
	}
}
