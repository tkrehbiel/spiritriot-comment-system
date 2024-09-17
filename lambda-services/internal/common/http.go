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
