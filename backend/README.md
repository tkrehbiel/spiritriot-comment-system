# Spiritriot Go Backend Services

This directory contains the Go-based backend implementation for the serverless Spiritriot comment system for static blogs, compiled as AWS Lambda functions.

## What's Up With That Name?

These days I tend to name my projects with random word generators. This time it came up with "spirit riot," which I found funny, evocative, unique, and completely random. It was originally going to be a Next.js blog platform, but I've mostly abandoned the blog parts.

## Directory Structure

* **`cmd/`**: Entrypoints for each individual microservice.
  * **`submit-service/`**: API endpoint to submit a comment (includes validation, Google/Mastodon/IndieAuth OAuth authentication checks, and writing to DynamoDB).
  * **`fetch-service/`**: API endpoint to query and fetch comments for a specific post.
  * **`page-service/`**: Serves a fallback HTML page enabling visitors to read and submit comments without Javascript enabled.
  * **`dev-server/`**: A local web server for development, mimicking Lambda triggers and proxying requests to LocalStack.
* **`internal/`**: Shared code, database layer queries, structures, and comment validation rules.

## Local Development & Compilation

To build and run tests for all Go backend components:

```bash
# Run Go unit tests
go test ./...

# Build all binaries
go build ./...
```

For environment configurations and run shortcuts, see the root monorepo `Makefile`.
