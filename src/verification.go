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

var allowedTargetDomains = map[string]bool{
	"standingpad.org":               true,
	"www.standingpad.org":           true,
	"arch.otter-pythagorean.ts.net": true,
}

const maxFetchSizeBytes = 1024 * 1024 // 1MB
const maxTimeout = 10 * time.Second
const userAgent = "Maryam's Webmention-Receiver/1.0"

func IsAllowedTarget(u *url.URL) bool {
	host := strings.ToLower(u.Hostname())
	return allowedTargetDomains[host]
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

func safeHTTPClient() *http.Client {
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
		Timeout:   maxTimeout,
	}
}

func FetchSourceHTML(sourceURL string) (io.ReadCloser, int, error) {
	client := safeHTTPClient()
	req, err := http.NewRequest(http.MethodGet, sourceURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	return resp.Body, resp.StatusCode, nil
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
			if token.Data == "a" || token.Data == "link" {
				for _, attr := range token.Attr {
					if attr.Key == "href" {
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

func VerifyWebmention(sourceURL, targetURL *url.URL) (bool, error) {
	body, statusCode, err := FetchSourceHTML(sourceURL.String())
	if err != nil {
		return false, err
	}
	defer body.Close()

	if statusCode != http.StatusOK {
		return false, fmt.Errorf("Source returned status %d", statusCode)
	}

	limitedReader := io.LimitReader(body, maxFetchSizeBytes)
	return ContainsTargetLink(limitedReader, targetURL.String())
}
