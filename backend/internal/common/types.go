package common

import (
	"context"
	"log"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
)

const CommentDateFormat = "2006-01-02T15:04:05Z" // standard comment ISO8601 UTC format

// CommentEntryData contains the comment fields gathered by a form to submit
type CommentEntryData struct {
	PostOrigin     string
	PostTitle      string
	Page           string
	Honeypot       string
	Name           string
	Email          string
	Website        string
	Comment        string
	Referrer       string
	ClientIP       string
	UserAgent      string
	Private        bool
	GoogleToken    string
	MigrateAccount bool
	UserID         string
	Verified       bool
	IndieAuthToken string
	ProfileURL     string
	MastodonToken  string
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

// LoadAWSConfig loads the default AWS SDK configuration
func LoadAWSConfig(ctx context.Context) (aws.Config, error) {
	if os.Getenv("AWS_LAMBDA_FUNCTION_NAME") == "" {
		if os.Getenv("AWS_ENDPOINT_URL") == "" {
			log.Println("Local environment detected and AWS_ENDPOINT_URL is empty. Defaulting to LocalStack at http://localhost:4566 to prevent production tampering.")
			os.Setenv("AWS_ENDPOINT_URL", "http://localhost:4566")
		}
	}
	return config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
}
