# Spiritriot Comment System

Spiritriot is a custom, homegrown serverless comment system built for static blogs (specifically designed for Hugo sites). It consists of Go-based AWS Lambda services as the backend, DynamoDB as the storage engine, and a lightweight client-side JavaScript widget for rendering comments dynamically.

---

## Directory Structure

```text
spiritriot-comment-system/
├── makefile                # Root makefile for managing assets and local testing
├── env.makefile            # Environment variables mapping S3 buckets and Lambda names
├── lambda-services/        # Go Lambda backend implementation
│   ├── cmd/                # Entrypoints for individual Lambdas
│   │   ├── fetch-service/  # API endpoint to fetch dynamic comments for a page
│   │   ├── page-service/   # No-JS dynamic comment entry page
│   │   └── submit-service/ # API endpoint to submit a comment (includes spam checks)
│   ├── internal/           # Shared database queries, common models, and validation logic
│   └── makefile            # Backend Go compile and deploy configurations
├── web-assets/             # Frontend client assets
│   ├── css/                # Comments styling
│   └── js/                 # Client script (comments.js)
└── hugo-sample/            # A local Hugo sample site for integration testing
```

---

## Configuration

Both the root directory and the `lambda-services` directory rely on an `env.makefile` to store deployment targets (Lambda function names, S3 bucket names, etc.).

If not already configured, copy the samples and customize them:
```bash
cp env.makefile.sample env.makefile
cp lambda-services/env.makefile.sample lambda-services/env.makefile
```

---

## Local Testing

To test comments locally with a sample Hugo project:
```bash
# This minifies/copies local CSS/JS to the static directory and starts a Hugo dev server
make web
```

---

## Backend Deployment (AWS Lambda)

The backend runs as three Go-based AWS Lambda functions. Compilation and deployments are configured inside the `lambda-services` directory.

> [!NOTE]
> All build commands target `GOOS=linux GOARCH=amd64` using bootstrap tags suitable for Amazon Linux 2023 (`provided.al2023` runtime).

### Prerequisites
Make sure your terminal is authenticated with an AWS profile (using the AWS CLI) that has IAM permissions to update lambda function code (e.g. `aws lambda update-function-code`).

### 1. Build and Run Tests
Verify that all Go code compiles and tests pass:
```bash
# Runs existing Go unit tests
make -C lambda-services test
```

### 2. Deploy Individual Lambdas
If you've modified a specific service, you can deploy it directly:

- **Fetch Service** (fetches live comments for posts):
  ```bash
  make -C lambda-services deploy-fetch
  ```
- **Page Service** (No-JS fallback webpage):
  ```bash
  make -C lambda-services deploy-page
  ```
- **Submit Service** (handles comment validation and writes to DynamoDB):
  ```bash
  make -C lambda-services deploy-submit
  ```

### 3. Deploy All Backend Services
To build and deploy all three Lambda functions at once:
```bash
make -C lambda-services deploy-fetch deploy-page deploy-submit
```

---

## Frontend Deployment (Amazon S3 & CloudFront)

The frontend JavaScript and CSS assets are served from Amazon S3 (and usually cached by CloudFront).

To minify and upload your updated client-side assets to S3:
```bash
make deploy-s3-assets
```

*Note: After uploading, you must manually invalidate the CloudFront cache for your CDN domain (e.g., assets.endgameviable.com) to make the new styles and scripts live immediately.*
