package syncapi

import (
	"net/http"
	"slices"
	"strings"
)

type CORSConfig struct {
	AllowedOrigins []string
}

type corsPolicy struct {
	allowAllOrigins bool
	allowedOrigins  map[string]struct{}
	allowedMethods  []string
	allowedHeaders  []string
	maxAgeSeconds   int
}

func buildCORSPolicy(cfg *CORSConfig) *corsPolicy {
	if cfg == nil || len(cfg.AllowedOrigins) == 0 {
		return nil
	}

	policy := &corsPolicy{
		allowedOrigins: map[string]struct{}{},
		allowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		allowedHeaders: []string{"Authorization", "Content-Type"},
		maxAgeSeconds:  600,
	}

	for _, origin := range cfg.AllowedOrigins {
		trimmed := strings.TrimSpace(origin)
		if trimmed == "" {
			continue
		}
		if trimmed == "*" {
			policy.allowAllOrigins = true
			continue
		}
		policy.allowedOrigins[trimmed] = struct{}{}
	}

	if !policy.allowAllOrigins && len(policy.allowedOrigins) == 0 {
		return nil
	}

	return policy
}

func (p *corsPolicy) isOriginAllowed(origin string) bool {
	if p == nil {
		return false
	}
	origin = strings.TrimSpace(origin)
	if origin == "" {
		return false
	}
	if p.allowAllOrigins {
		return true
	}
	_, ok := p.allowedOrigins[origin]
	return ok
}

func (p *corsPolicy) isMethodAllowed(method string) bool {
	if p == nil {
		return false
	}
	method = strings.ToUpper(strings.TrimSpace(method))
	if method == "" {
		return false
	}
	return slices.Contains(p.allowedMethods, method)
}

func (p *corsPolicy) allowOriginHeaderValue(origin string) string {
	if p.allowAllOrigins {
		return "*"
	}
	return origin
}

func appendVaryHeader(w http.ResponseWriter, value string) {
	existing := w.Header().Values("Vary")
	for _, item := range existing {
		if strings.EqualFold(strings.TrimSpace(item), value) {
			return
		}
	}
	w.Header().Add("Vary", value)
}
