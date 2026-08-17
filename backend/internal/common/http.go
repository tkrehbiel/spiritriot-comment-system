package common

import (
	"net"
	"net/url"
	"strings"
)

func ValidateReferrer(referrer string, allowedReferrers string) bool {
	if referrer == "" {
		return false
	}
	var refURL *url.URL
	var err error
	if !strings.Contains(referrer, "://") {
		refURL, err = url.Parse("http://" + referrer)
	} else {
		refURL, err = url.Parse(referrer)
	}
	if err != nil {
		return false
	}
	refHost := strings.ToLower(refURL.Host)
	if refHost == "" {
		return false
	}

	// Split refHost into host and port
	refHostNoPort := refHost
	if host, _, err := net.SplitHostPort(refHost); err == nil {
		refHostNoPort = host
	}

	for _, allowed := range strings.Split(allowedReferrers, ",") {
		allowed = strings.TrimSpace(strings.ToLower(allowed))
		if allowed == "" {
			continue
		}

		var allowedHost string
		if strings.HasPrefix(allowed, "http://") || strings.HasPrefix(allowed, "https://") {
			allowedURL, err := url.Parse(allowed)
			if err == nil {
				allowedHost = allowedURL.Host
			} else {
				allowedHost = allowed
			}
		} else {
			allowedHost = allowed
		}

		// Split allowedHost into host and port if present
		allowedHostNoPort := allowedHost
		hasAllowedPort := false
		if host, _, err := net.SplitHostPort(allowedHost); err == nil {
			allowedHostNoPort = host
			hasAllowedPort = true
		}

		if hasAllowedPort {
			// If allowed host specifies a port, match the exact refHost (including port)
			if refHost == allowedHost {
				return true
			}
		} else {
			// If allowed host does not specify a port, match the host part only
			if refHostNoPort == allowedHostNoPort {
				return true
			}
			// Subdomain match
			if strings.HasSuffix(refHostNoPort, "."+allowedHostNoPort) {
				return true
			}
		}
	}
	return false
}

// GetCORSHeaders returns standard CORS response headers
func GetCORSHeaders(methods string) map[string]string {
	return map[string]string{
		"Access-Control-Allow-Headers": "Content-Type,X-Amz-Date,Authorization,X-Api-Key,X-Amz-Security-Token",
		"Access-Control-Allow-Methods": methods,
		"Access-Control-Allow-Origin":  "*",
		"Content-Type":                 "application/json",
	}
}

