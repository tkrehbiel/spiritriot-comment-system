package writeComments

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"endgameviable-comment-services/internal/common"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockSNSService struct {
	mock.Mock
}

func (m *MockSNSService) Publish(ctx context.Context, input *sns.PublishInput, opts ...func(*sns.Options)) (*sns.PublishOutput, error) {
	args := m.Called(ctx, input, opts)
	if result, ok := args.Get(0).(*sns.PublishOutput); ok {
		return result, args.Error(1)
	}
	return nil, args.Error(1)
}

func TestSaveComment_Guest(t *testing.T) {
	os.Setenv("DYNAMO_USER_TABLE", "users")
	os.Setenv("DYNAMO_COMMENT_TABLE", "comments")
	os.Setenv("NOTIFICATION_TOPIC_ARN", "arn:sns")
	os.Setenv("NOTIFICATION_HEADER", "New Comment")

	ctx := context.TODO()
	data := common.CommentEntryData{
		Name:       "GuestUser",
		Email:      "guest@example.com",
		Comment:    "Hello!",
		Page:       "/test",
		Referrer:   "http://localhost:1313/",
		ClientIP:   "127.0.0.1",
		UserAgent:  "Mozilla/5.0",
		PostOrigin: "http://localhost:1313/",
	}

	t.Run("Guest New User Registration", func(t *testing.T) {
		mockDynamo := new(MockDynamoDBClient)
		mockSns := new(MockSNSService)

		// User not found initially
		mockDynamo.On("GetItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{Item: nil}, nil)
		// Registers new user
		mockDynamo.On("PutItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.PutItemOutput{}, nil).Once()
		// Saves comment
		mockDynamo.On("PutItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.PutItemOutput{}, nil).Once()
		// Sends notification
		mockSns.On("Publish", ctx, mock.Anything, mock.Anything).Return(&sns.PublishOutput{}, nil)

		err := SaveComment(ctx, mockDynamo, mockSns, data)
		assert.NoError(t, err)

		mockDynamo.AssertExpectations(t)
		mockSns.AssertExpectations(t)
	})

	t.Run("Guest Existing User Matches", func(t *testing.T) {
		mockDynamo := new(MockDynamoDBClient)
		mockSns := new(MockSNSService)

		existingUser := UserAccount{Author: "GuestUser", UserID: "guest-uuid", AuthorID: "email:guest@example.com"}
		item, _ := attributevalue.MarshalMap(existingUser)

		mockDynamo.On("GetItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{Item: item}, nil)
		mockDynamo.On("PutItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.PutItemOutput{}, nil) // For comment
		mockSns.On("Publish", ctx, mock.Anything, mock.Anything).Return(&sns.PublishOutput{}, nil)

		err := SaveComment(ctx, mockDynamo, mockSns, data)
		assert.NoError(t, err)

		mockDynamo.AssertExpectations(t)
	})

	t.Run("Guest Existing User Email Mismatch", func(t *testing.T) {
		mockDynamo := new(MockDynamoDBClient)
		mockSns := new(MockSNSService)

		existingUser := UserAccount{Author: "GuestUser", UserID: "guest-uuid", AuthorID: "email:other@example.com"}
		item, _ := attributevalue.MarshalMap(existingUser)

		mockDynamo.On("GetItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{Item: item}, nil)

		err := SaveComment(ctx, mockDynamo, mockSns, data)
		assert.Error(t, err)
		assert.EqualError(t, err, "email doesn't match")
	})
}

func TestSaveComment_Google(t *testing.T) {
	// Setup keys and mock certs server
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	kid := "google-key-id"
	nStr := encodeBase64URL(privateKey.N.Bytes())
	eBytes := big.NewInt(int64(privateKey.E)).Bytes()
	eStr := encodeBase64URL(eBytes)

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		jwks := JWKS{
			Keys: []JWK{
				{
					Kid: kid,
					N:   nStr,
					E:   eStr,
				},
			},
		}
		json.NewEncoder(w).Encode(jwks)
	}))
	defer mockServer.Close()

	os.Setenv("GOOGLE_CLIENT_ID", "my-client-id")
	os.Setenv("GOOGLE_CERTS_URL", mockServer.URL)
	os.Setenv("DYNAMO_USER_TABLE", "users")
	os.Setenv("DYNAMO_COMMENT_TABLE", "comments")
	os.Setenv("NOTIFICATION_TOPIC_ARN", "arn:sns")
	os.Setenv("NOTIFICATION_HEADER", "New Comment")

	ctx := context.TODO()

	googleClaims := GoogleClaims{
		Issuer:        "https://accounts.google.com",
		Subject:       "google-sub-123",
		Audience:      "my-client-id",
		Expiry:        time.Now().Add(1 * time.Hour).Unix(),
		Email:         "googleuser@example.com",
		EmailVerified: true,
		Name:          "GoogleUser",
	}

	token := createTestToken(t, privateKey, kid, googleClaims)

	t.Run("Google New User Registration", func(t *testing.T) {
		mockDynamo := new(MockDynamoDBClient)
		mockSns := new(MockSNSService)

		data := common.CommentEntryData{
			Name:        "GoogleUser",
			Email:       "googleuser@example.com",
			Comment:     "Verified comment!",
			Page:        "/test",
			Referrer:    "http://localhost:1313/",
			GoogleToken: token,
			ClientIP:    "127.0.0.1",
			UserAgent:   "Mozilla/5.0",
			PostOrigin:  "http://localhost:1313/",
		}

		mockDynamo.On("GetItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{Item: nil}, nil)
		mockDynamo.On("PutItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.PutItemOutput{}, nil).Twice() // One for user, one for comment
		mockSns.On("Publish", ctx, mock.Anything, mock.Anything).Return(&sns.PublishOutput{}, nil)

		err := SaveComment(ctx, mockDynamo, mockSns, data)
		assert.NoError(t, err)

		mockDynamo.AssertExpectations(t)
	})

	t.Run("Google Legacy Account Migration Mismatch (No Consent)", func(t *testing.T) {
		mockDynamo := new(MockDynamoDBClient)
		mockSns := new(MockSNSService)

		data := common.CommentEntryData{
			Name:        "GoogleUser",
			Email:       "googleuser@example.com",
			Comment:     "Verified comment!",
			Page:        "/test",
			Referrer:    "http://localhost:1313/",
			GoogleToken: token,
			ClientIP:    "127.0.0.1",
			UserAgent:   "Mozilla/5.0",
			PostOrigin:  "http://localhost:1313/",
			// MigrateAccount is false
		}

		// Existing user claimed by email
		existingUser := UserAccount{Author: "GoogleUser", UserID: "google-user-uuid", AuthorID: "email:googleuser@example.com"}
		item, _ := attributevalue.MarshalMap(existingUser)

		mockDynamo.On("GetItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{Item: item}, nil)

		err := SaveComment(ctx, mockDynamo, mockSns, data)
		assert.Error(t, err)
		assert.EqualError(t, err, "linking_consent_required")
	})

	t.Run("Google Legacy Account Migration (With Consent)", func(t *testing.T) {
		mockDynamo := new(MockDynamoDBClient)
		mockSns := new(MockSNSService)

		data := common.CommentEntryData{
			Name:           "GoogleUser",
			Email:          "googleuser@example.com",
			Comment:        "Verified comment!",
			Page:           "/test",
			Referrer:       "http://localhost:1313/",
			GoogleToken:    token,
			MigrateAccount: true,
			ClientIP:       "127.0.0.1",
			UserAgent:      "Mozilla/5.0",
			PostOrigin:     "http://localhost:1313/",
		}

		// Existing user claimed by email
		existingUser := UserAccount{Author: "GoogleUser", UserID: "google-user-uuid", AuthorID: "email:googleuser@example.com"}
		item, _ := attributevalue.MarshalMap(existingUser)

		mockDynamo.On("GetItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{Item: item}, nil)
		mockDynamo.On("PutItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.PutItemOutput{}, nil).Twice() // One user upgrade, one comment save
		mockSns.On("Publish", ctx, mock.Anything, mock.Anything).Return(&sns.PublishOutput{}, nil)

		err := SaveComment(ctx, mockDynamo, mockSns, data)
		assert.NoError(t, err)

		mockDynamo.AssertExpectations(t)
	})
}

func TestSaveComment_IndieAuth(t *testing.T) {
	os.Setenv("JWT_SECRET", "my-test-secret-12345")
	os.Setenv("DYNAMO_USER_TABLE", "users")
	os.Setenv("DYNAMO_COMMENT_TABLE", "comments")
	os.Setenv("NOTIFICATION_TOPIC_ARN", "arn:sns")
	os.Setenv("NOTIFICATION_HEADER", "New Comment")

	ctx := context.TODO()

	identity := "https://mastodon.social/@indieuser"
	token, err := SignIndieAuthToken(identity, "my-test-secret-12345")
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}

	t.Run("IndieAuth New User Registration", func(t *testing.T) {
		mockDynamo := new(MockDynamoDBClient)
		mockSns := new(MockSNSService)

		data := common.CommentEntryData{
			Name:           "IndieUser",
			Email:          "",
			Comment:        "Hello from the Fediverse!",
			Page:           "/test",
			Referrer:       "http://localhost:1313/",
			IndieAuthToken: token,
			ClientIP:       "127.0.0.1",
			UserAgent:      "Mozilla/5.0",
			PostOrigin:     "http://localhost:1313/",
		}

		mockDynamo.On("GetItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{Item: nil}, nil)
		mockDynamo.On("PutItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.PutItemOutput{}, nil).Twice()
		mockSns.On("Publish", ctx, mock.Anything, mock.Anything).Return(&sns.PublishOutput{}, nil)

		err := SaveComment(ctx, mockDynamo, mockSns, data)
		assert.NoError(t, err)

		mockDynamo.AssertExpectations(t)
		mockSns.AssertExpectations(t)

		// Assert that the registered user has the profile url
		var foundUserPut bool
		for _, call := range mockDynamo.Calls {
			if call.Method == "PutItem" {
				input := call.Arguments.Get(1).(*dynamodb.PutItemInput)
				if *input.TableName == "users" {
					var savedUser UserAccount
					err := attributevalue.UnmarshalMap(input.Item, &savedUser)
					assert.NoError(t, err)
					assert.Equal(t, "IndieUser", savedUser.Author)
					assert.Equal(t, identity, savedUser.ProfileURL)
					foundUserPut = true
				}
			}
		}
		assert.True(t, foundUserPut, "Should have registered the new user with ProfileURL")
	})

	t.Run("IndieAuth Existing User Matches", func(t *testing.T) {
		mockDynamo := new(MockDynamoDBClient)
		mockSns := new(MockSNSService)

		data := common.CommentEntryData{
			Name:           "IndieUser",
			Comment:        "Hello again!",
			Page:           "/test",
			Referrer:       "http://localhost:1313/",
			IndieAuthToken: token,
			ClientIP:       "127.0.0.1",
			UserAgent:      "Mozilla/5.0",
			PostOrigin:     "http://localhost:1313/",
		}

		existingUser := UserAccount{Author: "IndieUser", UserID: "indie-uuid", AuthorID: "indieauth:" + identity, ProfileURL: identity}
		item, _ := attributevalue.MarshalMap(existingUser)

		mockDynamo.On("GetItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{Item: item}, nil)
		mockDynamo.On("PutItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.PutItemOutput{}, nil)
		mockSns.On("Publish", ctx, mock.Anything, mock.Anything).Return(&sns.PublishOutput{}, nil)

		err := SaveComment(ctx, mockDynamo, mockSns, data)
		assert.NoError(t, err)

		mockDynamo.AssertExpectations(t)
	})
}

func TestSaveComment_ValidationFailure(t *testing.T) {
	ctx := context.TODO()
	mockDynamo := new(MockDynamoDBClient)
	mockSns := new(MockSNSService)

	data := common.CommentEntryData{}
	err := SaveComment(ctx, mockDynamo, mockSns, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid comment data")
}

func TestSaveComment_Mastodon(t *testing.T) {
	os.Setenv("JWT_SECRET", "my-test-secret-12345")
	os.Setenv("DYNAMO_USER_TABLE", "users")
	os.Setenv("DYNAMO_COMMENT_TABLE", "comments")
	os.Setenv("NOTIFICATION_TOPIC_ARN", "arn:sns")
	os.Setenv("NOTIFICATION_HEADER", "New Comment")

	ctx := context.TODO()
	identity := "https://mastodon.social/@alice"
	token, err := SignMastodonToken(identity, "my-test-secret-12345")
	assert.NoError(t, err)

	t.Run("Mastodon New User Registration", func(t *testing.T) {
		mockDynamo := new(MockDynamoDBClient)
		mockSns := new(MockSNSService)

		data := common.CommentEntryData{
			Name:          "Alice",
			Comment:       "Hello!",
			Page:          "/test",
			Referrer:      "http://localhost:1313/",
			MastodonToken: token,
			ClientIP:      "127.0.0.1",
			UserAgent:     "Mozilla/5.0",
			PostOrigin:    "http://localhost:1313/",
		}

		mockDynamo.On("GetItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{Item: nil}, nil)
		mockDynamo.On("PutItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.PutItemOutput{}, nil).Twice()
		mockSns.On("Publish", ctx, mock.Anything, mock.Anything).Return(&sns.PublishOutput{}, nil)

		err := SaveComment(ctx, mockDynamo, mockSns, data)
		assert.NoError(t, err)

		mockDynamo.AssertExpectations(t)
		mockSns.AssertExpectations(t)

		// Assert that the registered user has the profile url
		var foundUserPut bool
		for _, call := range mockDynamo.Calls {
			if call.Method == "PutItem" {
				input := call.Arguments.Get(1).(*dynamodb.PutItemInput)
				if *input.TableName == "users" {
					var savedUser UserAccount
					err := attributevalue.UnmarshalMap(input.Item, &savedUser)
					assert.NoError(t, err)
					assert.Equal(t, "Alice", savedUser.Author)
					assert.Equal(t, identity, savedUser.ProfileURL)
					foundUserPut = true
				}
			}
		}
		assert.True(t, foundUserPut, "Should have registered the new user with ProfileURL")
	})

	t.Run("Mastodon Existing User Mismatch (Username Taken)", func(t *testing.T) {
		mockDynamo := new(MockDynamoDBClient)
		mockSns := new(MockSNSService)

		data := common.CommentEntryData{
			Name:          "Alice",
			Comment:       "Hello!",
			Page:          "/test",
			Referrer:      "http://localhost:1313/",
			MastodonToken: token,
			ClientIP:      "127.0.0.1",
			UserAgent:     "Mozilla/5.0",
			PostOrigin:    "http://localhost:1313/",
		}

		existingUser := UserAccount{Author: "Alice", UserID: "alice-uuid", AuthorID: "google:other-google-sub"}
		item, _ := attributevalue.MarshalMap(existingUser)

		mockDynamo.On("GetItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{Item: item}, nil)

		err := SaveComment(ctx, mockDynamo, mockSns, data)
		assert.Error(t, err)
		assert.EqualError(t, err, "username taken")
	})
}
