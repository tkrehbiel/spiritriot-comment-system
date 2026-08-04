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
	go build -o dev-server-bin ./cmd/dev-server/main.go && \
	HTTP_ALLOWED_REFERRERS="http://localhost:1313" \
	WEBSITE_URL="http://localhost:1313" \
	GOOGLE_CLIENT_ID="$(GOOGLE_CLIENT_ID)" \
	JWT_SECRET="local-development-secret-key-12345" \
	AKISMET_API_KEY="$(AKISMET_API_KEY)" \
	DYNAMO_COMMENT_TABLE="spiritriot_comments" \
	DYNAMO_USER_TABLE="spiritriot_users" \
	DYNAMO_MASTODON_CLIENTS_TABLE="spiritriot_mastodon_clients" \
	MASTODON_CLIENT_NAME="SpiritRiot Comments" \
	NOTIFICATION_TOPIC_ARN="arn:aws:sns:us-east-1:000000000000:spiritriot-new-comments" \
	NOTIFICATION_HEADER="New Comment Received" \
	AWS_ENDPOINT_URL="http://localhost:4566" \
	AWS_REGION="us-east-1" \
	AWS_ACCESS_KEY_ID="mock" \
	AWS_SECRET_ACCESS_KEY="mock" \
	./dev-server-bin

migrate-local:
	DYNAMO_COMMENT_TABLE="spiritriot_comments" \
	DYNAMO_USER_TABLE="spiritriot_users" \
	AWS_ENDPOINT_URL="http://localhost:4566" \
	AWS_REGION="us-east-1" \
	AWS_ACCESS_KEY_ID="mock" \
	AWS_SECRET_ACCESS_KEY="mock" \
	python3 ./tools/db-migration/migrate_db.py

dev-local: start
	@echo "Waiting for LocalStack/DynamoDB to be ready..."
	@until curl -s http://localhost:4566 > /dev/null; do sleep 1; done
	@echo "Checking if DynamoDB tables are bootstrapped..."
	@until aws dynamodb list-tables --endpoint-url http://localhost:4566 --region us-east-1 --output text 2>/dev/null | grep -q "spiritriot_comments"; do sleep 1; done
	@echo "Bootstrapped! Copying frontend assets to local sample blog..."
	mkdir -p ./hugo/sample-blog/static/js ./hugo/sample-blog/static/css
	cp ./frontend/client-assets/js/comments.js ./hugo/sample-blog/static/js/
	cp ./frontend/client-assets/css/comments.css ./hugo/sample-blog/static/css/
	@echo "Starting Go API Dev Server and Hugo blog concurrently (press Ctrl+C to stop both)..."
	(trap 'kill 0' SIGINT; \
	 $(MAKE) api & \
	 hugo server --source ./hugo/sample-blog --bind 127.0.0.1 --port 1313)

# Scan local DynamoDB tables
show-comments-local:
	AWS_PAGER="" aws dynamodb scan --table-name spiritriot_comments --endpoint-url http://localhost:4566 --region us-east-1

show-users-local:
	AWS_PAGER="" aws dynamodb scan --table-name spiritriot_users --endpoint-url http://localhost:4566 --region us-east-1

show-mastodon-clients-local:
	AWS_PAGER="" aws dynamodb scan --table-name spiritriot_mastodon_clients --endpoint-url http://localhost:4566 --region us-east-1

empty-db-local:
	@echo "Emptying DynamoDB tables..."
	@python3 -c "import boto3; \
	db = boto3.resource('dynamodb', endpoint_url='http://localhost:4566', region_name='us-east-1', aws_access_key_id='mock', aws_secret_access_key='mock'); \
	t_comments = db.Table('spiritriot_comments'); \
	t_users = db.Table('spiritriot_users'); \
	t_clients = db.Table('spiritriot_mastodon_clients'); \
	[t_comments.delete_item(Key={'id': i['id']}) for i in t_comments.scan().get('Items', [])]; \
	[t_users.delete_item(Key={'author': i['author']}) for i in t_users.scan().get('Items', [])]; \
	[t_clients.delete_item(Key={'instance_host': i['instance_host']}) for i in t_clients.scan().get('Items', [])]"
	@echo "Database tables emptied successfully."

.PHONY: start stop api migrate-local dev-local show-comments-local show-users-local show-mastodon-clients-local empty-db-local web
