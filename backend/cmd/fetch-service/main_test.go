package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"spiritriot-comment-services/internal/readComments"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func init() {
	os.Setenv("DYNAMO_COMMENT_TABLE", "test_comments")
}

type MockDynamoQuery struct {
	mock.Mock
}

func (m *MockDynamoQuery) Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	args := m.Called(ctx, params)
	if result, ok := args.Get(0).(*dynamodb.QueryOutput); ok {
		return result, args.Error(1)
	}
	return nil, args.Error(1)
}

func TestLambdaHandlerWeb_Fetch_Success(t *testing.T) {
	mockDb := new(MockDynamoQuery)
	dynamoClient = mockDb

	avMap := map[string]types.AttributeValue{
		"author":  &types.AttributeValueMemberS{Value: "Alice"},
		"content": &types.AttributeValueMemberS{Value: "Great!"},
		"date":    &types.AttributeValueMemberS{Value: "2026-08-01T12:00:00Z"},
	}

	mockDb.On("Query", mock.Anything, mock.Anything).Return(&dynamodb.QueryOutput{
		Items: []map[string]types.AttributeValue{avMap},
		Count: 1,
	}, nil)

	req := events.APIGatewayProxyRequest{
		QueryStringParameters: map[string]string{
			"page": "/test",
		},
	}

	resp, err := lambdaHandlerWeb(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, resp.Body, "Alice")

	var result []readComments.CommentItem
	err = json.Unmarshal([]byte(resp.Body), &result)
	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Alice", result[0].Author)
}

func TestLambdaHandlerWeb_Fetch_MissingPage(t *testing.T) {
	req := events.APIGatewayProxyRequest{
		QueryStringParameters: map[string]string{},
	}

	resp, err := lambdaHandlerWeb(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
	assert.Contains(t, resp.Body, "invalid page")
}

func TestLambdaHandlerWeb_Fetch_DBError(t *testing.T) {
	mockDb := new(MockDynamoQuery)
	dynamoClient = mockDb

	mockDb.On("Query", mock.Anything, mock.Anything).Return(nil, errors.New("query fail"))

	req := events.APIGatewayProxyRequest{
		QueryStringParameters: map[string]string{
			"page": "/test",
		},
	}

	resp, err := lambdaHandlerWeb(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, 500, resp.StatusCode)
	assert.Contains(t, resp.Body, "error getting comments")
}
