package main

import (
	"errors"
	"net/url"
)

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
