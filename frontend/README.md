# Spiritriot Frontend Assets

This directory contains client-side frontend assets for the comment system.

## Directory Structure

* **`client-assets/`**: Raw assets loaded by browser client widgets.
  * **`js/comments.js`**: Core client script that dynamically fetches and renders comments, handles user login/authentication flows, and submits comments to the API.
  * **`css/comments.css`**: CSS stylesheet for styling the comment threads, form boxes, buttons, and login providers.

## Deployment Pipeline

During production deployment, the assets are minified and synced to an S3 bucket (and distributed via CloudFront) using the root `Makefile` targets:

```bash
# Minify JS/CSS assets and sync them to production S3
make deploy-s3-assets
```
