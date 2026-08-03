-include env.makefile

GOOGLE_CLIENT_ID ?= mock-google-client-id

ASSETS = ./frontend/client-assets
HUGO = ./hugo/sample-blog/static

DOCKER := $(shell which docker 2>/dev/null || echo "/Applications/Docker.app/Contents/Resources/bin/docker")

# Minification targets
${ASSETS}/css/comments.min.css: ${ASSETS}/css/comments.css
	minify ${ASSETS}/css/comments.css > ${ASSETS}/css/comments.min.css

${ASSETS}/js/comments.min.js: ${ASSETS}/js/comments.js
	minify ${ASSETS}/js/comments.js > ${ASSETS}/js/comments.min.js

# deploy assets to s3 bucket
deploy-s3-assets: ${ASSETS}/css/comments.min.css ${ASSETS}/js/comments.min.js
	aws s3 sync ${ASSETS}/css/ ${S3_BUCKET}/css/
	aws s3 sync ${ASSETS}/js/ ${S3_BUCKET}/js/
	@echo "now go invalidate the cache in CloudFront"

# startup a local hugo server for testing
web:
	cp ${ASSETS}/css/comments.css ${HUGO}/css/
	cp ${ASSETS}/js/comments.js ${HUGO}/js/
	hugo server --source ./hugo/sample-blog

# --- LocalStack & Dev Environment Targets ---

start:
	$(DOCKER) compose -f ./localstack/docker-compose.yml up -d

stop:
	$(DOCKER) compose -f ./localstack/docker-compose.yml down

api:
	cd ./backend && \
	HTTP_ALLOWED_REFERRERS="http://localhost:1313" \
	GOOGLE_CLIENT_ID="$(GOOGLE_CLIENT_ID)" \
	JWT_SECRET="local-development-secret-key-12345" \
	DYNAMO_COMMENT_TABLE="endgameviable_comments" \
	DYNAMO_USER_TABLE="endgameviable_users" \
	DYNAMO_MASTODON_CLIENTS_TABLE="endgameviable_mastodon_clients" \
	MASTODON_CLIENT_NAME="Endgame Viable Comments" \
	NOTIFICATION_TOPIC_ARN="arn:aws:sns:us-east-1:000000000000:spiritriot-new-comments" \
	NOTIFICATION_HEADER="New Comment Received" \
	AWS_ENDPOINT_URL="http://localhost:4566" \
	AWS_REGION="us-east-1" \
	AWS_ACCESS_KEY_ID="mock" \
	AWS_SECRET_ACCESS_KEY="mock" \
	go run ./cmd/dev-server/main.go

migrate-local:
	DYNAMO_COMMENT_TABLE="endgameviable_comments" \
	DYNAMO_USER_TABLE="endgameviable_users" \
	AWS_ENDPOINT_URL="http://localhost:4566" \
	AWS_REGION="us-east-1" \
	AWS_ACCESS_KEY_ID="mock" \
	AWS_SECRET_ACCESS_KEY="mock" \
	python3 ./tools/db-migration/migrate_db.py

dev-local: start
	@echo "Waiting for LocalStack/DynamoDB to be ready..."
	@until curl -s http://localhost:4566 > /dev/null; do sleep 1; done
	@echo "Checking if DynamoDB tables are bootstrapped..."
	@until aws dynamodb list-tables --endpoint-url http://localhost:4566 --region us-east-1 --output text 2>/dev/null | grep -q "endgameviable_comments"; do sleep 1; done
	@echo "Bootstrapped! Copying frontend assets to endgameviable-hugo..."
	mkdir -p ../endgameviable-hugo/static/js ../endgameviable-hugo/static/css
	cp ./frontend/client-assets/js/comments.js ../endgameviable-hugo/static/js/
	cp ./frontend/client-assets/css/comments.css ../endgameviable-hugo/static/css/
	@echo "Starting Go API Dev Server and Hugo blog concurrently (press Ctrl+C to stop both)..."
	(trap 'kill 0' SIGINT; \
	 $(MAKE) api & \
	 cd ../endgameviable-hugo && hugo server --bind 127.0.0.1 --port 1313)

# Scan local DynamoDB tables
show-comments-local:
	AWS_PAGER="" aws dynamodb scan --table-name endgameviable_comments --endpoint-url http://localhost:4566 --region us-east-1

show-users-local:
	AWS_PAGER="" aws dynamodb scan --table-name endgameviable_users --endpoint-url http://localhost:4566 --region us-east-1

show-mastodon-clients-local:
	AWS_PAGER="" aws dynamodb scan --table-name endgameviable_mastodon_clients --endpoint-url http://localhost:4566 --region us-east-1

empty-db-local:
	@echo "Emptying DynamoDB tables..."
	@python3 -c "import boto3; \
	db = boto3.resource('dynamodb', endpoint_url='http://localhost:4566', region_name='us-east-1', aws_access_key_id='mock', aws_secret_access_key='mock'); \
	t_comments = db.Table('endgameviable_comments'); \
	t_users = db.Table('endgameviable_users'); \
	t_clients = db.Table('endgameviable_mastodon_clients'); \
	[t_comments.delete_item(Key={'id': i['id']}) for i in t_comments.scan().get('Items', [])]; \
	[t_users.delete_item(Key={'author': i['author']}) for i in t_users.scan().get('Items', [])]; \
	[t_clients.delete_item(Key={'instance_host': i['instance_host']}) for i in t_clients.scan().get('Items', [])]"
	@echo "Database tables emptied successfully."

.PHONY: start stop api migrate-local dev-local show-comments-local show-users-local show-mastodon-clients-local empty-db-local web deploy-s3-assets
