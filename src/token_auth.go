package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
}

var linkHeaderRegex = regexp.MustCompile(`<([^>]+)>;\s*rel=["']?([^"';]+)["']?`)

func ParseTokenEndpoint(linkHeader string, baseURL *url.URL) (string, error) {
	if linkHeader == "" {
		return "", fmt.Errorf("no Link header found")
	}

	for _, link := range strings.Split(linkHeader, ",") {
		matches := linkHeaderRegex.FindStringSubmatch(strings.TrimSpace(link))
		if len(matches) == 3 {
			rawURL := matches[1]
			rels := strings.Fields(matches[2])

			for _, rel := range rels {
				if rel == "token_endpoint" {
					resolved, err := baseURL.Parse(rawURL)
					if err != nil {
						return "", fmt.Errorf("failed to parse token endpoint URL: %w", err)
					}
					return resolved.String(), nil
				}
			}
		}
	}

	return "", fmt.Errorf("rel=\"token_endpoint\" not found in Link header")
}

func ExchangeCodeForToken(client *http.Client, tokenEndpoint string, code string) (*TokenResponse, error) {
	data := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
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

	if tokenResp.AccessToken == "" {
		return nil, fmt.Errorf("token endpoint returned empty access_token")
	}

	return &tokenResp, nil
}
