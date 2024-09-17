package common

import (
	"log"
	"os"
)

const CommentDateFormat = "2006-01-02T15:04:05Z" // standard comment ISO8601 UTC format

// CommentEntryData contains the comment fields gathered by a form to submit
type CommentEntryData struct {
	PostOrigin string
	PostTitle  string
	Page       string
	Honeypot   string
	Name       string
	Email      string
	Website    string
	Comment    string
	Referrer   string
	ClientIP   string
	UserAgent  string
}

func GetEnvVar(name string, def string) string {
	val := os.Getenv(name)
	if val == "" {
		if def == "" {
			log.Fatalf("fatal error: %s env var not defined", name)
		}
		val = def
	}
	return val
}
