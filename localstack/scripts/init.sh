#!/bin/bash
echo "Initializing DynamoDB tables..."

# 1. Create endgameviable_comments table with page-index-v3 index
awslocal dynamodb create-table \
    --table-name endgameviable_comments \
    --attribute-definitions \
        AttributeName=id,AttributeType=S \
        AttributeName=page,AttributeType=S \
    --key-schema \
        AttributeName=id,KeyType=HASH \
    --global-secondary-indexes '[
        {
            "IndexName": "page-index-v3",
            "KeySchema": [
                {
                    "AttributeName": "page",
                    "KeyType": "HASH"
                }
            ],
            "Projection": {
                "ProjectionType": "ALL"
            }
        }
    ]' \
    --billing-mode PAY_PER_REQUEST

# 2. Create endgameviable_users table
awslocal dynamodb create-table \
    --table-name endgameviable_users \
    --attribute-definitions \
        AttributeName=author,AttributeType=S \
    --key-schema \
        AttributeName=author,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST

# 3. Create endgameviable_mastodon_clients table
awslocal dynamodb create-table \
    --table-name endgameviable_mastodon_clients \
    --attribute-definitions \
        AttributeName=instance_host,AttributeType=S \
    --key-schema \
        AttributeName=instance_host,KeyType=HASH \
    --billing-mode PAY_PER_REQUEST

# 4. Create SNS topic
awslocal sns create-topic --name spiritriot-new-comments

echo "DynamoDB bootstrap completed."
