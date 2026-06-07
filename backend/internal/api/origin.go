package api

import (
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/brdweb/homelab-control/internal/config"
)

type originPolicy struct {
	allowAll bool
	allowed  map[string]struct{}
}

func newOriginPolicy(allowedOrigins []string) originPolicy {
	policy := originPolicy{allowed: make(map[string]struct{}, len(allowedOrigins))}
	for _, origin := range allowedOrigins {
		if origin == "*" {
			policy.allowAll = true
			continue
		}
		policy.allowed[origin] = struct{}{}
	}
	return policy
}

func (p originPolicy) allowedOrigin(r *http.Request) (string, bool) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return "", true
	}

	normalized, err := config.NormalizeAllowedOrigin(origin)
	if err != nil {
		return "", false
	}
	if sameRequestHost(r, normalized) {
		return origin, true
	}
	if p.allowAll {
		return origin, true
	}
	_, ok := p.allowed[normalized]
	if !ok {
		return "", false
	}
	return origin, true
}

func sameRequestHost(r *http.Request, origin string) bool {
	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return normalizeHost(originURL.Host) == normalizeHost(r.Host)
}

func normalizeHost(host string) string {
	host = strings.ToLower(strings.TrimSpace(host))
	if h, p, err := net.SplitHostPort(host); err == nil {
		if p == "80" || p == "443" {
			return strings.ToLower(h)
		}
	}
	return host
}
