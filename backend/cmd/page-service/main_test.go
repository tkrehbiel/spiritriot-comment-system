package main

import (
	"context"
	"html/template"
	"os"
	"testing"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func init() {
	os.Setenv("DYNAMO_COMMENT_TABLE", "test_comments")
	os.Setenv("DYNAMO_USER_TABLE", "test_users")
	os.Setenv("HTTP_ALLOWED_REFERRERS", "http://localhost:1313")
	os.Setenv("HTML_CSS", "comments.css")
	os.Setenv("HTML_TITLE", "Comments")
}

type MockPageDynamo struct {
	mock.Mock
}

func (m *MockPageDynamo) GetItem(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	args := m.Called(ctx, input)
	if result, ok := args.Get(0).(*dynamodb.GetItemOutput); ok {
		return result, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPageDynamo) PutItem(ctx context.Context, input *dynamodb.PutItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	args := m.Called(ctx, input)
	if result, ok := args.Get(0).(*dynamodb.PutItemOutput); ok {
		return result, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockPageDynamo) Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	args := m.Called(ctx, params)
	if result, ok := args.Get(0).(*dynamodb.QueryOutput); ok {
		return result, args.Error(1)
	}
	return nil, args.Error(1)
}

type MockPageSNS struct {
	mock.Mock
}

func (m *MockPageSNS) Publish(ctx context.Context, input *sns.PublishInput, opts ...func(*sns.Options)) (*sns.PublishOutput, error) {
	args := m.Called(ctx, input, opts)
	if result, ok := args.Get(0).(*sns.PublishOutput); ok {
		return result, args.Error(1)
	}
	return nil, args.Error(1)
}

func TestLambdaHandlerWeb_Page_GET(t *testing.T) {
	os.Setenv("HTML_TITLE", "Title")
	os.Setenv("HTML_CSS", "style.css")
	defer func() {
		os.Unsetenv("HTML_TITLE")
		os.Unsetenv("HTML_CSS")
	}()

	mockDb := new(MockPageDynamo)
	mockSns := new(MockPageSNS)

	dynamoClient = mockDb
	snsClient = mockSns

	// Mock comments database query
	mockDb.On("Query", mock.Anything, mock.Anything).Return(&dynamodb.QueryOutput{
		Items: nil,
		Count: 0,
	}, nil)

	req := events.APIGatewayProxyRequest{
		HTTPMethod: "GET",
		QueryStringParameters: map[string]string{
			"title":  "Post Title",
			"origin": "http://localhost:1313/test-page",
		},
		Headers: map[string]string{
			"Cookie": "name=Alice; email=alice@example.com",
		},
	}

	resp, err := lambdaHandlerWeb(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, resp.Body, "RE: <a href=\"http://localhost:1313/test-page\">Post Title</a>")
	assert.Contains(t, resp.Body, "No comments yet.")
}

func TestLambdaHandlerWeb_Page_POST(t *testing.T) {
	os.Setenv("HTML_TITLE", "Title")
	os.Setenv("HTML_CSS", "style.css")
	os.Setenv("DYNAMO_USER_TABLE", "users")
	os.Setenv("DYNAMO_COMMENT_TABLE", "comments")
	os.Setenv("NOTIFICATION_TOPIC_ARN", "arn:sns")
	os.Setenv("NOTIFICATION_HEADER", "Header")
	os.Setenv("HTTP_ALLOWED_REFERRERS", "localhost,example.com")
	defer func() {
		os.Unsetenv("HTML_TITLE")
		os.Unsetenv("HTML_CSS")
		os.Unsetenv("DYNAMO_USER_TABLE")
		os.Unsetenv("DYNAMO_COMMENT_TABLE")
		os.Unsetenv("NOTIFICATION_TOPIC_ARN")
		os.Unsetenv("NOTIFICATION_HEADER")
		os.Unsetenv("HTTP_ALLOWED_REFERRERS")
	}()

	mockDb := new(MockPageDynamo)
	mockSns := new(MockPageSNS)

	dynamoClient = mockDb
	snsClient = mockSns

	// Mock DB GetItem (no user), PutItem (user & comment)
	mockDb.On("GetItem", mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{Item: nil}, nil)
	mockDb.On("PutItem", mock.Anything, mock.Anything).Return(&dynamodb.PutItemOutput{}, nil)
	mockSns.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(&sns.PublishOutput{}, nil)

	// Mock query for recent comments after post
	mockDb.On("Query", mock.Anything, mock.Anything).Return(&dynamodb.QueryOutput{
		Items: nil,
		Count: 0,
	}, nil)

	req := events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Body:       "title=Title&origin=http%3A%2F%2Flocalhost%3A1313%2Fpage&page=%2Fpage&name=Alice&email=alice%40example.com&comment=Great!",
		Headers: map[string]string{
			"referer": "http://localhost:1313/page",
		},
		RequestContext: events.APIGatewayProxyRequestContext{
			Identity: events.APIGatewayRequestIdentity{
				SourceIP:  "127.0.0.1",
				UserAgent: "Agent",
			},
		},
	}

	resp, err := lambdaHandlerWeb(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, resp.Body, "No comments yet.")
}

func TestLambdaHandlerWeb_Page_POST_InvalidReferrer(t *testing.T) {
	os.Setenv("HTML_TITLE", "Title")
	os.Setenv("HTML_CSS", "style.css")
	os.Setenv("DYNAMO_USER_TABLE", "users")
	os.Setenv("DYNAMO_COMMENT_TABLE", "comments")
	os.Setenv("HTTP_ALLOWED_REFERRERS", "localhost,example.com")
	defer func() {
		os.Unsetenv("HTML_TITLE")
		os.Unsetenv("HTML_CSS")
		os.Unsetenv("DYNAMO_USER_TABLE")
		os.Unsetenv("DYNAMO_COMMENT_TABLE")
		os.Unsetenv("HTTP_ALLOWED_REFERRERS")
	}()

	mockDb := new(MockPageDynamo)
	mockSns := new(MockPageSNS)
	dynamoClient = mockDb
	snsClient = mockSns

	// Mock query for recent comments (SaveComment should not be called)
	mockDb.On("Query", mock.Anything, mock.Anything).Return(&dynamodb.QueryOutput{
		Items: nil,
		Count: 0,
	}, nil)

	req := events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		Body:       "title=Title&origin=http%3A%2F%2Flocalhost%3A1313%2Fpage&page=%2Fpage&name=Alice&email=alice%40example.com&comment=Great!",
		Headers: map[string]string{
			"referer": "http://unallowed.com/page",
		},
		RequestContext: events.APIGatewayProxyRequestContext{
			Identity: events.APIGatewayRequestIdentity{
				SourceIP:  "127.0.0.1",
				UserAgent: "Agent",
			},
		},
	}

	resp, err := lambdaHandlerWeb(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	// Body should indicate comment was rejected
	assert.Contains(t, resp.Body, "comment rejected")
}

func TestLambdaHandlerWeb_Page_POST_InvalidOrigin(t *testing.T) {
	os.Setenv("HTML_TITLE", "Title")
	os.Setenv("HTML_CSS", "style.css")
	os.Setenv("DYNAMO_USER_TABLE", "users")
	os.Setenv("DYNAMO_COMMENT_TABLE", "comments")
	os.Setenv("HTTP_ALLOWED_REFERRERS", "localhost,example.com")
	defer func() {
		os.Unsetenv("HTML_TITLE")
		os.Unsetenv("HTML_CSS")
		os.Unsetenv("DYNAMO_USER_TABLE")
		os.Unsetenv("DYNAMO_COMMENT_TABLE")
		os.Unsetenv("HTTP_ALLOWED_REFERRERS")
	}()

	mockDb := new(MockPageDynamo)
	mockSns := new(MockPageSNS)
	dynamoClient = mockDb
	snsClient = mockSns

	// Mock query for recent comments
	mockDb.On("Query", mock.Anything, mock.Anything).Return(&dynamodb.QueryOutput{
		Items: nil,
		Count: 0,
	}, nil)

	req := events.APIGatewayProxyRequest{
		HTTPMethod: "POST",
		// Origin is not allowed
		Body:       "title=Title&origin=http%3A%2F%2Funallowed.com%2Fpage&page=%2Fpage&name=Alice&email=alice%40example.com&comment=Great!",
		Headers: map[string]string{
			"referer": "http://localhost:1313/page",
		},
		RequestContext: events.APIGatewayProxyRequestContext{
			Identity: events.APIGatewayRequestIdentity{
				SourceIP:  "127.0.0.1",
				UserAgent: "Agent",
			},
		},
	}

	resp, err := lambdaHandlerWeb(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, resp.Body, "comment rejected")
}

func TestRenderTemplate_Error(t *testing.T) {
	tmpl := template.New("incomplete")
	res := renderTemplate(tmpl, CommentPageData{})
	assert.Contains(t, res, "Error rendering template")
}
