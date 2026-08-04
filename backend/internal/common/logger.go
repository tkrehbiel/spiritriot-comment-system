package common

import (
	"context"
	"log"

	"github.com/aws/aws-lambda-go/lambdacontext"
)

// LogWithCtx writes a log message prefixed with the AWS Lambda Request ID if present.
func LogWithCtx(ctx context.Context, format string, v ...interface{}) {
	reqID := "local"
	if lc, ok := lambdacontext.FromContext(ctx); ok {
		reqID = lc.AwsRequestID
	}
	log.Printf("["+reqID+"] "+format, v...)
}
