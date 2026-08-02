# Comment System Deployment Plan

This document outlines the step-by-step instructions and order of operations required to deploy the updated authenticated comment system to the live blog.

## Repositories Involved

1. **`spiritriot-comment-system`**: Contains the backend Go lambda services, web assets (JS/CSS), and the DynamoDB migration script.
2. **`endgameviable-hugo`**: Contains the Hugo templates/theme layouts for the blog frontend.

---

## Step-by-Step Order of Operations

### Step 1: Pre-Deployment Verification
Before making any commits or starting deployment, run Go unit tests locally to confirm everything compiles and passes:
```bash
cd lambda-services
go test ./...
```

### Step 2: Commit & Push `spiritriot-comment-system` Code
Check in all verified changes to git first to keep source control clean and synchronized:
```bash
git add .
git commit -m "Revert identity column to author_id and clean up migrate_db.py"
git push origin main
```

### Step 3: Database Table Setup
Create the `endgameviable_mastodon_clients` table in production DynamoDB using the AWS CLI:
```bash
aws dynamodb create-table \
    --table-name endgameviable_mastodon_clients \
    --attribute-definitions \
        AttributeName=instance_host,AttributeType=S \
    --key-schema \
        AttributeName=instance_host,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST
```

### Step 4: Configure Lambda Environment Variables
Add or update the following environment variables on the AWS Lambda functions in the AWS Console.

#### Function: `spiritriot-post-comment` (Submit Service)
| Environment Variable | Recommended Value / Action | Source / Where to Get It |
| :--- | :--- | :--- |
| **`DYNAMO_USER_TABLE`** | `endgameviable_users` | Name of your existing users table. |
| **`DYNAMO_MASTODON_CLIENTS_TABLE`** | `endgameviable_mastodon_clients` | Name of the table created in Step 3. |
| **`GOOGLE_CLIENT_ID`** | `<your-google-oauth-client-id>` | Retrieve from the **Google Cloud Console** under: *APIs & Services* -> *Credentials* -> *OAuth 2.0 Client IDs*. |
| **`JWT_SECRET`** | `<secure-random-string>` | Generate a new cryptographically secure random string on your terminal (e.g., run `openssl rand -base64 32`). |
| **`MASTODON_CLIENT_NAME`** | `Endgame Viable Comments` | The custom application name that will appear on Mastodon when visitors authorize their account. |

#### Function: `spiritriot-comment-page` (Page Service)
| Environment Variable | Recommended Value / Action | Source / Where to Get It |
| :--- | :--- | :--- |
| **`HTTP_ALLOWED_REFERRERS`** | `https://endgameviable.com` | Your live blog production domain. |
| **`HTML_CSS`** | `https://assets.endgameviable.com/css/comments.css` | Public URL of the deployed comments stylesheet. |

### Step 5: Run Database Migration
Run the migration script against production DynamoDB tables to link existing historical comments to `user_id` values:
```bash
python3 migration/migrate_db.py
```
*Note: Make sure your current terminal session has access to production AWS credentials (without `AWS_ENDPOINT_URL` set to LocalStack).*

### Step 6: Deploy Backend Lambda Services
Compile and upload the Go binaries to AWS Lambda:
```bash
cd lambda-services
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

### Step 8: Commit & Push `endgameviable-hugo` Git Changes
Commit the template changes to deploy the new blog layouts:
```bash
cd ../endgameviable-hugo
git add layouts/partials/spiritriotcomment.html themes/endgame/layouts/partials/assets.html themes/endgame2026/layouts/partials/assets.html
git commit -m "Update comment form templates to support authenticated comments"
git push origin master
```
*Note: Pushing to `endgameviable-hugo` triggers your AWS Amplify pipeline to build and publish the blog site.*

### Step 9: Post-Deployment Validation
1. Visit the live blog once the AWS Amplify build finishes.
2. Verify that the new comment form renders correctly.
3. Test signing in via Google, Mastodon, or IndieAuth and posting a test comment to confirm end-to-end functionality.
