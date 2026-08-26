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
)

type LinkHeaderParseMode int

const (
	ModeNone LinkHeaderParseMode = iota
	ModeLink
	ModeAttrKey
	ModeAttrVal
	ModeQuoted
)

type LinkHeader struct {
	Href  string
	Attrs map[string]string
}

func parseLinkHeader(linkHeader string) ([]*LinkHeader, error) {
	if linkHeader == "" {
		return []*LinkHeader{}, nil
	}
	links := []*LinkHeader{
		{Attrs: make(map[string]string)},
	}
	mode := ModeNone

	lastAttr := ""
	currentBuffer := ""
	for _, char := range linkHeader {
		quoteMode := mode == ModeQuoted
		header := links[len(links)-1]
		switch {
		case char == '<' && !quoteMode:
			mode = ModeLink
		case char == '>' && !quoteMode:
			header.Href = strings.TrimSpace(currentBuffer)
			currentBuffer = ""
			mode = ModeNone
		case char == ';' && !quoteMode:
			if mode == ModeAttrVal {
				header.Attrs[lastAttr] = strings.TrimSpace(currentBuffer)
			}
			currentBuffer = ""
			mode = ModeAttrKey
		case char == '=' && !quoteMode && mode == ModeAttrKey:
			lastAttr = strings.TrimSpace(currentBuffer)
			currentBuffer = ""
			mode = ModeAttrVal
		case char == ',' && !quoteMode && mode != ModeLink:
			if mode == ModeAttrVal {
				header.Attrs[lastAttr] = strings.TrimSpace(currentBuffer)
			}
			links = append(links, &LinkHeader{Attrs: make(map[string]string)})
			currentBuffer = ""
			mode = ModeNone
		case char == '"' && (mode == ModeAttrVal || mode == ModeQuoted):
			if mode == ModeAttrVal {
				mode = ModeQuoted
			} else {
				mode = ModeAttrVal
			}
		default:
			currentBuffer += string(char)
		}
	}

	if mode == ModeAttrVal {
		links[len(links)-1].Attrs[lastAttr] = strings.TrimSpace(currentBuffer)
	}

	filteredLinks := []*LinkHeader{}
	for _, link := range links {
		if link.Href != "" {
			filteredLinks = append(filteredLinks, link)
		}
	}
	return filteredLinks, nil
}
