# Comment System Deployment Plan

This document outlines the step-by-step instructions and order of operations required to deploy the updated authenticated comment system to your live blog.

## Repositories Involved

1. **`spiritriot-comment-system`**: Contains the backend Go lambda services, web assets (JS/CSS), and the DynamoDB migration script.
2. **Your Hugo Blog Repository**: Contains the Hugo templates/theme layouts for the blog frontend.

---

## Step-by-Step Order of Operations

### Step 1: Pre-Deployment Verification
Before making any commits or starting deployment, run Go unit tests locally to confirm everything compiles and passes:
```bash
cd backend
go test ./...
```

### Step 2: Commit & Push `spiritriot-comment-system` Code
Check in all verified changes to git first to keep source control clean and synchronized.

### Step 3: Database Table Setup
Create the DynamoDB tables in production AWS if not already created (specifically the Mastodon clients table if migrating to dynamic registrations):
```bash
aws dynamodb create-table \
    --table-name spiritriot_mastodon_clients \
    --attribute-definitions \
        AttributeName=instance_host,AttributeType=S \
    --key-schema \
        AttributeName=instance_host,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST
```

### Step 4: Configure Lambda Environment Variables
Add or update environment variables on the AWS Lambda functions in the AWS Console. 
For a complete listing of required variables and descriptions, refer to [CONFIG.md](CONFIG.md).

Verify that all required environment variables have been explicitly defined (e.g. `DYNAMO_COMMENT_TABLE`, `DYNAMO_USER_TABLE`, `DYNAMO_MASTODON_CLIENTS_TABLE`, `JWT_SECRET`, `GOOGLE_CLIENT_ID`, `MASTODON_CLIENT_NAME`, `WEBSITE_URL`, `API_DOMAIN`, `HTTP_ALLOWED_REFERRERS`, and `HTML_CSS`).

### Step 5: Run Database Migration
Run the migration script against production DynamoDB tables to link existing historical comments to `user_id` values if necessary:
```bash
python3 tools/db-migration/migrate_db.py
```
*Note: Make sure your current terminal session has access to production AWS credentials (without `AWS_ENDPOINT_URL` set to LocalStack).*

### Step 6: Deploy Backend Lambda Services
Compile and upload the Go binaries to AWS Lambda:
```bash
cd backend
make deploy
```

### Step 7: Deploy Frontend Web Assets (S3 & CloudFront)
Upload the minified frontend scripts to the S3 assets bucket:
```bash
make deploy-s3-assets
```
Invalidate the CloudFront CDN cache to force browsers to fetch the updated files immediately:
```bash
aws cloudfront create-invalidation --distribution-id <DISTRIBUTION_ID> --paths "/js/comments.js" "/css/comments.css"
```

### Step 8: Configure Blog Repository Site Parameters
Add the required site parameters in your Hugo configuration (e.g. `hugo.toml`) as specified in [CONFIG.md](CONFIG.md):
```toml
[params.spiritriot]
  commentPageURL = "https://comments.yourdomain.com"
  apiURL = "https://api.yourdomain.com"
```

### Step 9: Commit & Push Hugo Blog Changes
Commit the template changes to deploy the new blog layouts:
```bash
git add layouts/partials/spiritriotcomment.html
git commit -m "Update comment form templates to support spiritriot decoupled config"
git push origin master
```

### Step 10: Post-Deployment Validation
1. Visit the live blog once the build pipeline finishes.
2. Verify that the new comment form renders correctly.
3. Test signing in via Google, Mastodon, or IndieAuth and posting a test comment to confirm end-to-end functionality.
