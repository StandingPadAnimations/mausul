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
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

type VerificationResult int

const (
	StatusKeep VerificationResult = iota
	StatusDelete
	StatusError
)

func IsAllowedTarget(c *WebmentionsConfig, u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	return c.AllowedTargets[host]
}

func IsPrivate(wr *WebmentionRequest) bool {
	return wr.Code != ""
}

func parseAndValidateURL(rawURL string) (*url.URL, error) {
	u, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return nil, errors.New("malformed URL")
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, errors.New("URL scheme must be http or https")
	}

	if u.Host == "" {
		return nil, errors.New("URL host must be specified")
	}
	return u, nil
}

func isPrivateIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	addr.Unmap()
	return addr.IsPrivate() ||
		addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() ||
		addr.IsUnspecified()
}

func safeHTTPClient(c *WebmentionsConfig) *http.Client {
	dialer := &net.Dialer{
		Timeout: 5 * time.Second,
	}

	transport := &http.Transport{
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}

			ips, err := net.LookupIP(host)
			if err != nil {
				return nil, err
			}

			for _, ip := range ips {
				if isPrivateIP(ip) {
					return nil, fmt.Errorf("SSRF blocked: %s is a private IP", ip.String())
				}
			}
			return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
		},
		ResponseHeaderTimeout: 5 * time.Second,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   c.MaxTimeout,
	}
}

func FetchSourceHTML(c *WebmentionsConfig, sourceURL string) (io.ReadCloser, int, http.Header, error) {
	client := safeHTTPClient(c)
	req, err := http.NewRequest(http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	return resp.Body, resp.StatusCode, resp.Header, nil
}

func FetchSourceHTMLPrivate(c *WebmentionsConfig, sourceURL, token string) (io.ReadCloser, int, http.Header, error) {
	client := safeHTTPClient(c)
	req, err := http.NewRequest(http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, 0, nil, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, nil, err
	}
	return resp.Body, resp.StatusCode, resp.Header, nil
}

func ContainsTargetLink(r io.Reader, target string) (bool, error) {
	tokenizer := html.NewTokenizer(r)
	targetNorm := strings.TrimRight(target, "/")
	for {
		tt := tokenizer.Next()
		switch tt {
		case html.ErrorToken:
			if errors.Is(tokenizer.Err(), io.EOF) {
				return false, nil
			}
			return false, tokenizer.Err()
		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()
			switch token.Data {
			case "a", "link", "area":
				for _, attr := range token.Attr {
					if attr.Key == "href" {
						trimmed := strings.TrimRight(attr.Val, "/")
						if trimmed == targetNorm {
							return true, nil
						}
					}
				}
			case "video", "source", "img", "iframe", "embed", "audio", "track", "input", "script":
				for _, attr := range token.Attr {
					if attr.Key == "src" {
						trimmed := strings.TrimRight(attr.Val, "/")
						if trimmed == targetNorm {
							return true, nil
						}
					}
				}
			}
		}
	}
}

func VerifyWebmention(c *WebmentionsConfig, wr *WebmentionRequest) (VerificationResult, *TokenResponse, error) {
	// Even if its private, we need to make
	// one GET request anyway to get the token
	// endpoint
	body, statusCode, header, err := FetchSourceHTML(c, wr.Source.String())
	if err != nil {
		return StatusError, nil, err
	}

	if statusCode == http.StatusGone {
		body.Close()
		return StatusDelete, nil, nil
	}

	// If unauthorized and the Webmention is private
	// TODO: Use wr.Realm to reuse tokens
	var tokenResponse *TokenResponse
	if statusCode == http.StatusUnauthorized && IsPrivate(wr) {
		body.Close()

		linkHeader := header.Get("Link")
		tokenEndpoint, err := ParseTokenEndpoint(linkHeader, &wr.Source)
		if err != nil {
			return StatusError, nil, fmt.Errorf("failed to discover token endpoint: %w", err)
		}

		client := safeHTTPClient(c)
		tokenResponse, err = ExchangeCodeForToken(client, tokenEndpoint, wr.Code)
		if err != nil {
			return StatusError, nil, fmt.Errorf("token exchange failed: %w", err)
		}
		body, statusCode, header, err = FetchSourceHTMLPrivate(c, wr.Source.String(), tokenResponse.AccessToken)
		if err != nil {
			return StatusError, nil, err
		}
	}
	defer body.Close()

	if statusCode != http.StatusOK {
		return StatusError, nil, fmt.Errorf("Source returned status %d", statusCode)
	}

	limitedReader := io.LimitReader(body, c.MaxFetchSizeBytes)
	hasLink, err := ContainsTargetLink(limitedReader, wr.Target.String())
	if err != nil {
		return StatusError, nil, err
	}

	if hasLink {
		return StatusKeep, tokenResponse, nil
	}

	// Page exists, target link not present on page
	return StatusDelete, nil, nil
}
