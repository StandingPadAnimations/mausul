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
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	ReceivedAt  time.Time
}

func ParseTokenEndpoint(linkHeader string, baseURL *url.URL) (string, error) {
	if linkHeader == "" {
		return "", fmt.Errorf("no Link header found")
	}

	links, err := parseLinkHeader(linkHeader)
	if err != nil {
		return "", err
	}

	for _, link := range links {
		relVal, ok := link.Attrs["rel"]
		if !ok {
			continue
		}
		if relVal == "token_endpoint" {
			parsedHref, err := url.Parse(link.Href)
			if err != nil {
				return "", err
			}
			finalURL := baseURL.ResolveReference(parsedHref)
			return finalURL.String(), nil
		}
	}
	return "", fmt.Errorf("no token_endpoint link found")
}

func ExchangeCodeForToken(client *http.Client, tokenEndpoint string, code string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type": {"authorization_code"},
		"code":       {code},
	}

	req, err := http.NewRequest(http.MethodPost, tokenEndpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token endpoint returned status %d", resp.StatusCode)
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}
	tokenResp.ReceivedAt = time.Now().UTC()

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("token endpoint returned empty access_token")
	}

	return &tokenResp, nil
}
