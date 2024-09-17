include env.makefile

ASSETS = ./web-assets

${ASSETS}/js/comments.min.css: ${ASSETS}/css/comments.css
	minify ${ASSETS}/css/comments.css > ${ASSETS}/css/comments.min.css

${ASSETS}/css/comments.min.js: ${ASSETS}/js/comments.js
	minify ${ASSETS}/js/comments.js > ${ASSETS}/js/comments.min.js

# deploy assets to s3 bucket
deploy-s3-assets: ${ASSETS}/js/comments.min.css ${ASSETS}/css/comments.min.js
	aws s3 sync ${ASSETS}/css/ ${S3_BUCKET}/css/
	aws s3 sync ${ASSETS}/js/ ${S3_BUCKET}/js/
	@echo "now go invalidate the cache in CloudFront"

HUGO = ./hugo-sample/static

# startup a local hugo server for testing
web:
	cp ${ASSETS}/css/comments.css ${HUGO}/css
	cp ${ASSETS}/js/comments.js ${HUGO}/js
	hugo server --source ./hugo-sample
