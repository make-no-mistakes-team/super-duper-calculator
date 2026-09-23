package main

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// PUBLIC_ORIGIN is configured by the operator, never inferred from forwarded
// headers. An empty value uses the direct request's transport and Host.
func parsePublicOrigin(value string) (*url.URL, error) {
	if value == "" {
		return nil, nil
	}
	origin, err := parseOrigin(value)
	if err != nil {
		return nil, fmt.Errorf("invalid PUBLIC_ORIGIN: %w", err)
	}
	return origin, nil
}

func parseOrigin(value string) (*url.URL, error) {
	u, err := url.Parse(value)
	if err != nil {
		return nil, err
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" ||
		u.User != nil || u.Path != "" || strings.ContainsAny(value, "?#") {
		return nil, fmt.Errorf("expected an HTTP(S) origin without credentials, path, query or fragment")
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return nil, fmt.Errorf("invalid origin port")
		}
	} else if strings.HasSuffix(u.Host, ":") {
		return nil, fmt.Errorf("empty origin port")
	}
	return u, nil
}

func originAuthority(u *url.URL) string {
	port := u.Port()
	if port == "" {
		if u.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	return net.JoinHostPort(strings.ToLower(u.Hostname()), port)
}

func (a api) sameOrigin(r *http.Request) bool {
	values := r.Header.Values("Origin")
	if len(values) == 0 {
		// Older clients and command-line tools may omit Origin. Modern browser
		// metadata must not explicitly identify a cross-origin mutation.
		site := r.Header.Get("Sec-Fetch-Site")
		return site == "" || site == "same-origin" || site == "none"
	}
	if len(values) != 1 {
		return false
	}
	actual, err := parseOrigin(values[0])
	if err != nil {
		return false
	}
	expected := a.publicOrigin
	if expected == nil {
		scheme := "http"
		if r.TLS != nil {
			scheme = "https"
		}
		expected, err = parseOrigin(scheme + "://" + r.Host)
		if err != nil {
			return false
		}
	}
	return actual.Scheme == expected.Scheme && originAuthority(actual) == originAuthority(expected)
}
