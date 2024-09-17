package writeComments

import (
	"endgameviable-comment-services/internal/common"
	"fmt"
)

// validateForm verifies that comment data is okay
func validateComment(data common.CommentEntryData) error {
	var errors common.MultiError
	if data.Referrer == "" {
		errors.Add(fmt.Errorf("referrer missing"))
	}
	if data.ClientIP == "" {
		errors.Add(fmt.Errorf("clientIP missing"))
	}
	if data.UserAgent == "" {
		errors.Add(fmt.Errorf("user agent missing"))
	}
	if data.PostOrigin == "" {
		errors.Add(fmt.Errorf("origin missing"))
	}
	if data.Page == "" {
		errors.Add(fmt.Errorf("page missing"))
	}
	if data.Name == "" {
		errors.Add(fmt.Errorf("name missing"))
	}
	if data.Email == "" {
		errors.Add(fmt.Errorf("email missing"))
	}
	if data.Comment == "" {
		errors.Add(fmt.Errorf("comment missing"))
	}
	if data.Honeypot != "" {
		errors.Add(fmt.Errorf("honeypot filled"))
	}

	// TODO: validate origin same as referrer
	// TODO: check string lengths
	// TODO: Akismet? only if not already invalid

	return errors.Get()
}
