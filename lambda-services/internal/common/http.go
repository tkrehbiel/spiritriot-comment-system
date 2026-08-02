package common

import "strings"

func ValidateReferrer(referrer string, allowedReferrers string) bool {
	if referrer == "" {
		return false
	}
	for _, allowed := range strings.Split(allowedReferrers, ",") {
		if strings.Contains(referrer, allowed) {
			return true
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
