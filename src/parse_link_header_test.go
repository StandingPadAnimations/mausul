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
	"reflect"
	"testing"
)

func TestParseLinkHeader(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []*LinkHeader
	}{
		{
			name:     "Empty string",
			input:    "",
			expected: []*LinkHeader{},
		},
		{
			name:  "Single link no attributes",
			input: "<https://example.com/page/1>",
			expected: []*LinkHeader{
				{Href: "https://example.com/page/1", Attrs: map[string]string{}},
			},
		},
		{
			name:  "Single link with one attribute",
			input: `<https://example.com/page/2>; rel="next"`,
			expected: []*LinkHeader{
				{Href: "https://example.com/page/2", Attrs: map[string]string{"rel": "next"}},
			},
		},
		{
			name:  "Single link with one attribute and special characters in quotes",
			input: `<https://example.com/page/2>; rel="> next on this website, among the posts"`,
			expected: []*LinkHeader{
				{Href: "https://example.com/page/2", Attrs: map[string]string{"rel": "> next on this website, among the posts"}},
			},
		},
		{
			name:  "Single link with multiple attributes",
			input: `<https://example.com/page/2>; rel="next"; title="Next Page"`,
			expected: []*LinkHeader{
				{Href: "https://example.com/page/2", Attrs: map[string]string{"rel": "next", "title": "Next Page"}},
			},
		},
		{
			name:  "Multiple links",
			input: `<https://example.com/page/1>; rel="prev", <https://example.com/page/3>; rel="next"`,
			expected: []*LinkHeader{
				{Href: "https://example.com/page/1", Attrs: map[string]string{"rel": "prev"}},
				{Href: "https://example.com/page/3", Attrs: map[string]string{"rel": "next"}},
			},
		},
		{
			name:  "Comma inside URL (Should not split)",
			input: `<https://api.example.com/users?id=1,2,3>; rel="next"`,
			expected: []*LinkHeader{
				{Href: "https://api.example.com/users?id=1,2,3", Attrs: map[string]string{"rel": "next"}},
			},
		},
		{
			name:  "Semicolon and comma inside quotes (Should not split)",
			input: `<https://example.com>; title="Hello; World, How are you?"`,
			expected: []*LinkHeader{
				{Href: "https://example.com", Attrs: map[string]string{"title": "Hello; World, How are you?"}},
			},
		},
		{
			name:  "Trailing comma with empty space",
			input: `<https://example.com/page/1>; rel="next", `,
			expected: []*LinkHeader{
				{Href: "https://example.com/page/1", Attrs: map[string]string{"rel": "next"}},
			},
		},
		{
			name:  "Messy whitespace",
			input: `   <https://example.com>   ;   rel   =   "next"   `,
			expected: []*LinkHeader{
				{Href: "https://example.com", Attrs: map[string]string{"rel": "next"}},
			},
		},
		{
			name:  "Unquoted attribute values",
			input: `<https://example.com>; rel=next`,
			expected: []*LinkHeader{
				{Href: "https://example.com", Attrs: map[string]string{"rel": "next"}},
			},
		},
		{
			name:     "Malformed: Completely missing brackets",
			input:    `just some random string`,
			expected: []*LinkHeader{}, // Href never gets set, so it's filtered out
		},
		{
			name:     "Malformed: Missing closing bracket",
			input:    `<https://example.com/page/1; rel="next"`,
			expected: []*LinkHeader{}, // The '>' is never hit, so Href is never finalized
		},
		{
			name: "Malformed: Missing opening bracket",
			// It tolerant-parses whatever is in the buffer when it hits '>'
			input: `https://example.com/page/1>; rel="next"`,
			expected: []*LinkHeader{
				{Href: "https://example.com/page/1", Attrs: map[string]string{"rel": "next"}},
			},
		},
		{
			name:  "Malformed: Unclosed quote on attribute",
			input: `<https://example.com/page/1>; rel="next`,
			// Because the quote never closes, the state machine finishes in ModeQuoted
			// and safely discards the incomplete attribute.
			expected: []*LinkHeader{
				{Href: "https://example.com/page/1", Attrs: map[string]string{}},
			},
		},
		{
			name:  "Malformed: Garbage text instead of attributes",
			input: `<https://example.com/page/1> just absolute garbage, <https://example.com/page/2>`,
			// The state machine ignores text that doesn't follow the HTTP header rules
			// and seamlessly picks up the next valid link.
			expected: []*LinkHeader{
				{Href: "https://example.com/page/1", Attrs: map[string]string{}},
				{Href: "https://example.com/page/2", Attrs: map[string]string{}},
			},
		},
		{
			name:  "Malformed: Missing equal sign in attribute",
			input: `<https://example.com>; rel"next"`,
			// It never transitions to ModeAttrVal, so it treats it as junk
			expected: []*LinkHeader{
				{Href: "https://example.com", Attrs: map[string]string{}},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseLinkHeader(tc.input)
			if err != nil {
				t.Fatalf("parseLinkHeader() returned unexpected error: %v", err)
			}

			// reflect.DeepEqual is the standard way to compare complex nested
			// structures (like slices of structs containing maps) in Go.
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("\nGot:  %+v\nWant: %+v", got, tc.expected)
			}
		})
	}
}
