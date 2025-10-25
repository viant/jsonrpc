package common

import (
	"net"
	"net/http"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// ClientHost returns the browser-visible host, considering proxies.
// It looks at Forwarded, X-Forwarded-Host, then falls back to r.Host.
func ClientHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	// RFC 7239 Forwarded: host=; proto=
	if fwd := r.Header.Get("Forwarded"); fwd != "" {
		// naive parse; take first host= token
		parts := strings.Split(fwd, ";")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(strings.ToLower(p), "host=") {
				v := strings.TrimPrefix(p, "host=")
				v = strings.Trim(v, "\"")
				if v != "" {
					return stripPort(v)
				}
			}
		}
	}
	if xfh := r.Header.Get("X-Forwarded-Host"); xfh != "" {
		v := strings.TrimSpace(strings.Split(xfh, ",")[0])
		if v != "" {
			return stripPort(v)
		}
	}
	return stripPort(r.Host)
}

// TopDomain returns eTLD+1 for a host (e.g., app.example.co.uk -> example.co.uk).
func TopDomain(host string) (string, error) {
	if host == "" || isIP(host) || isLocalhost(host) {
		return "", nil
	}
	// Remove potential port suffix
	host = stripPort(host)
	e, err := publicsuffix.EffectiveTLDPlusOne(host)
	if err != nil {
		return "", err
	}
	// Avoid returning public suffix itself
	if e == host || e == "" {
		return "", nil
	}
	return e, nil
}

func isIP(h string) bool { return net.ParseIP(stripPort(h)) != nil }
func isLocalhost(h string) bool {
	h = strings.ToLower(stripPort(h))
	return h == "localhost" || strings.HasSuffix(h, ".localhost")
}
func stripPort(h string) string {
	if i := strings.IndexByte(h, ':'); i > -1 {
		return h[:i]
	}
	return h
}
