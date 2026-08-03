package main

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockDynamoDB struct {
	mock.Mock
}

func (m *MockDynamoDB) GetItem(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	args := m.Called(ctx, input)
	if result, ok := args.Get(0).(*dynamodb.GetItemOutput); ok {
		return result, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockDynamoDB) PutItem(ctx context.Context, input *dynamodb.PutItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	args := m.Called(ctx, input)
	if result, ok := args.Get(0).(*dynamodb.PutItemOutput); ok {
		return result, args.Error(1)
	}
	return nil, args.Error(1)
}

type MockSNS struct {
	mock.Mock
}

func (m *MockSNS) Publish(ctx context.Context, input *sns.PublishInput, opts ...func(*sns.Options)) (*sns.PublishOutput, error) {
	args := m.Called(ctx, input, opts)
	if result, ok := args.Get(0).(*sns.PublishOutput); ok {
		return result, args.Error(1)
	}
	return nil, args.Error(1)
}

func encodeBase64URL(b []byte) string {
	s := base64.URLEncoding.EncodeToString(b)
	return strings.TrimRight(s, "=")
}

func createTestToken(t *testing.T, key *rsa.PrivateKey, kid string, claims interface{}) string {
	t.Helper()

	header := struct {
		Kid string `json:"kid"`
		Alg string `json:"alg"`
	}{
		Kid: kid,
		Alg: "RS256",
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("failed to marshal header: %v", err)
	}

	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("failed to marshal claims: %v", err)
	}

	headerB64 := encodeBase64URL(headerBytes)
	claimsB64 := encodeBase64URL(claimsBytes)

	message := headerB64 + "." + claimsB64

	h := sha256.New()
	h.Write([]byte(message))
	hashed := h.Sum(nil)

	sigBytes, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hashed)
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	sigB64 := encodeBase64URL(sigBytes)

	return message + "." + sigB64
}

func TestLambdaHandlerWeb_Submit_Success(t *testing.T) {
	os.Setenv("HTTP_ALLOWED_REFERRERS", "localhost,example.com")
	os.Setenv("DYNAMO_USER_TABLE", "users")
	os.Setenv("DYNAMO_COMMENT_TABLE", "comments")
	os.Setenv("NOTIFICATION_TOPIC_ARN", "arn:sns")
	os.Setenv("NOTIFICATION_HEADER", "header")
	defer func() {
		os.Unsetenv("HTTP_ALLOWED_REFERRERS")
		os.Unsetenv("DYNAMO_USER_TABLE")
		os.Unsetenv("DYNAMO_COMMENT_TABLE")
		os.Unsetenv("NOTIFICATION_TOPIC_ARN")
		os.Unsetenv("NOTIFICATION_HEADER")
	}()

	mockDb := new(MockDynamoDB)
	mockSns := new(MockSNS)

	dynamoClient = mockDb
	snsClient = mockSns

	mockDb.On("GetItem", mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{Item: nil}, nil)
	mockDb.On("PutItem", mock.Anything, mock.Anything).Return(&dynamodb.PutItemOutput{}, nil)
	mockSns.On("Publish", mock.Anything, mock.Anything, mock.Anything).Return(&sns.PublishOutput{}, nil)

	bodyBytes, _ := json.Marshal(CommentData{
		Name:    "Alice",
		Email:   "alice@example.com",
		Comment: "Great post!",
		Page:    "/test-page",
		Origin:  "http://localhost:1313/test-page",
	})

	req := events.APIGatewayProxyRequest{
		Body: string(bodyBytes),
		Headers: map[string]string{
			"referer": "http://localhost:1313/test-page",
		},
		RequestContext: events.APIGatewayProxyRequestContext{
			Identity: events.APIGatewayRequestIdentity{
				SourceIP:  "127.0.0.1",
				UserAgent: "Mozilla/5.0",
			},
		},
	}

	resp, err := lambdaHandlerWeb(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)
	assert.Contains(t, resp.Body, "comment accepted")
}

func TestLambdaHandlerWeb_Submit_InvalidJSON(t *testing.T) {
	req := events.APIGatewayProxyRequest{
		Body: "{invalid json",
	}
	resp, err := lambdaHandlerWeb(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)
	assert.Contains(t, resp.Body, "error unmarshaling json")
}

func TestLambdaHandlerWeb_Submit_InvalidReferrer(t *testing.T) {
	os.Setenv("HTTP_ALLOWED_REFERRERS", "localhost,example.com")
	defer os.Unsetenv("HTTP_ALLOWED_REFERRERS")

	bodyBytes, _ := json.Marshal(CommentData{
		Name:    "Alice",
		Email:   "alice@example.com",
		Comment: "Great post!",
		Page:    "/test-page",
		Origin:  "http://localhost:1313/test-page",
	})

	req := events.APIGatewayProxyRequest{
		Body: string(bodyBytes),
		Headers: map[string]string{
			"referer": "http://unallowed.com/test-page",
		},
	}

	resp, err := lambdaHandlerWeb(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)
	assert.Contains(t, resp.Body, "comment rejected")
}

func TestLambdaHandlerWeb_Submit_SaveError(t *testing.T) {
	os.Setenv("HTTP_ALLOWED_REFERRERS", "localhost,example.com")
	os.Setenv("DYNAMO_USER_TABLE", "users")
	os.Setenv("DYNAMO_COMMENT_TABLE", "comments")
	defer func() {
		os.Unsetenv("HTTP_ALLOWED_REFERRERS")
		os.Unsetenv("DYNAMO_USER_TABLE")
		os.Unsetenv("DYNAMO_COMMENT_TABLE")
	}()

	mockDb := new(MockDynamoDB)
	mockSns := new(MockSNS)

	dynamoClient = mockDb
	snsClient = mockSns

	mockDb.On("GetItem", mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{Item: nil}, nil)
	mockDb.On("PutItem", mock.Anything, mock.Anything).Return(nil, errors.New("db save fail"))

	bodyBytes, _ := json.Marshal(CommentData{
		Name:    "Alice",
		Email:   "alice@example.com",
		Comment: "Great post!",
		Page:    "/test-page",
		Origin:  "http://localhost:1313/test-page",
	})

	req := events.APIGatewayProxyRequest{
		Body: string(bodyBytes),
		Headers: map[string]string{
			"referer": "http://localhost:1313/test-page",
		},
		RequestContext: events.APIGatewayProxyRequestContext{
			Identity: events.APIGatewayRequestIdentity{
				SourceIP:  "127.0.0.1",
				UserAgent: "Mozilla/5.0",
			},
		},
	}

	resp, err := lambdaHandlerWeb(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, 403, resp.StatusCode)
	assert.Contains(t, resp.Body, "comment rejected")
}

func TestLambdaHandlerWeb_Submit_LinkingConsentRequired(t *testing.T) {
	// Setup keys and mock certs server
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	assert.NoError(t, err)

	kid := "google-key-id"
	nStr := encodeBase64URL(privateKey.N.Bytes())
	eBytes := big.NewInt(int64(privateKey.E)).Bytes()
	eStr := encodeBase64URL(eBytes)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		jwks := struct {
			Keys []struct {
				Kid string `json:"kid"`
				N   string `json:"n"`
				E   string `json:"e"`
			} `json:"keys"`
		}{
			Keys: []struct {
				Kid string `json:"kid"`
				N   string `json:"n"`
				E   string `json:"e"`
			}{
				{Kid: kid, N: nStr, E: eStr},
			},
		}
		json.NewEncoder(w).Encode(jwks)
	}))
	defer mockServer.Close()

	os.Setenv("GOOGLE_CLIENT_ID", "my-client-id")
	os.Setenv("GOOGLE_CERTS_URL", mockServer.URL)
	os.Setenv("HTTP_ALLOWED_REFERRERS", "localhost,example.com")
	os.Setenv("DYNAMO_USER_TABLE", "users")
	os.Setenv("DYNAMO_COMMENT_TABLE", "comments")
	defer func() {
		os.Unsetenv("GOOGLE_CLIENT_ID")
		os.Unsetenv("GOOGLE_CERTS_URL")
		os.Unsetenv("HTTP_ALLOWED_REFERRERS")
		os.Unsetenv("DYNAMO_USER_TABLE")
		os.Unsetenv("DYNAMO_COMMENT_TABLE")
	}()

	mockDb := new(MockDynamoDB)
	mockSns := new(MockSNS)

	dynamoClient = mockDb
	snsClient = mockSns

	// Mock existing user claimed by email
	existingUser := struct {
		Author   string `json:"author" dynamodbav:"author"`
		UserID   string `json:"user_id" dynamodbav:"user_id"`
		AuthorID string `json:"author_id" dynamodbav:"author_id"`
	}{
		Author:   "GoogleUser",
		UserID:   "google-user-uuid",
		AuthorID: "email:googleuser@example.com",
	}
	item, _ := attributevalue.MarshalMap(existingUser)
	mockDb.On("GetItem", mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{Item: item}, nil)

	// Create a valid Google JWT token
	claims := struct {
		Issuer        string `json:"iss"`
		Subject       string `json:"sub"`
		Audience      string `json:"aud"`
		Expiry        int64  `json:"exp"`
		Email         string `json:"email"`
		EmailVerified bool   `json:"email_verified"`
		Name          string `json:"name"`
	}{
		Issuer:        "https://accounts.google.com",
		Subject:       "google-sub-123",
		Audience:      "my-client-id",
		Expiry:        time.Now().Add(1 * time.Hour).Unix(),
		Email:         "googleuser@example.com",
		EmailVerified: true,
		Name:          "GoogleUser",
	}

	token := createTestToken(t, privateKey, kid, claims)

	bodyBytes, _ := json.Marshal(CommentData{
		Name:        "GoogleUser",
		Email:       "googleuser@example.com",
		Comment:     "Verified comment!",
		Page:        "/test",
		Origin:      "http://localhost:1313/test",
		GoogleToken: token,
	})

	req := events.APIGatewayProxyRequest{
		Body: string(bodyBytes),
		Headers: map[string]string{
			"referer": "http://localhost:1313/test",
		},
		RequestContext: events.APIGatewayProxyRequestContext{
			Identity: events.APIGatewayRequestIdentity{
				SourceIP:  "127.0.0.1",
				UserAgent: "Mozilla/5.0",
			},
		},
	}

	resp, err := lambdaHandlerWeb(context.TODO(), req)
	assert.NoError(t, err)
	assert.Equal(t, 409, resp.StatusCode)
	assert.Contains(t, resp.Body, "linking_consent_required")
}
