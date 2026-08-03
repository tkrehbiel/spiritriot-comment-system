package writeComments

import (
	"testing"

	"spiritriot-comment-services/internal/common"

	"github.com/stretchr/testify/assert"
)

func TestValidateComment(t *testing.T) {
	t.Run("Valid indieauth comment", func(t *testing.T) {
		data := common.CommentEntryData{
			Referrer:       "http://example.com",
			ClientIP:       "127.0.0.1",
			UserAgent:      "agent",
			PostOrigin:     "origin",
			Page:           "page",
			Name:           "name",
			Comment:        "comment",
			IndieAuthToken: "token",
		}
		err := validateComment(data)
		assert.NoError(t, err)
	})

	t.Run("Valid email comment", func(t *testing.T) {
		data := common.CommentEntryData{
			Referrer:   "http://example.com",
			ClientIP:   "127.0.0.1",
			UserAgent:  "agent",
			PostOrigin: "origin",
			Page:       "page",
			Name:       "name",
			Comment:    "comment",
			Email:      "test@example.com",
		}
		err := validateComment(data)
		assert.NoError(t, err)
	})

	t.Run("Invalid comment - multiple errors", func(t *testing.T) {
		data := common.CommentEntryData{
			Honeypot: "not-empty",
		}
		err := validateComment(data)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "referrer missing")
		assert.Contains(t, err.Error(), "clientIP missing")
		assert.Contains(t, err.Error(), "user agent missing")
		assert.Contains(t, err.Error(), "origin missing")
		assert.Contains(t, err.Error(), "page missing")
		assert.Contains(t, err.Error(), "name missing")
		assert.Contains(t, err.Error(), "email missing")
		assert.Contains(t, err.Error(), "comment missing")
		assert.Contains(t, err.Error(), "honeypot filled")
	})
}
