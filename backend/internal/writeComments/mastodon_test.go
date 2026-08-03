package writeComments

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestMastodonToken_SignAndVerify(t *testing.T) {
	secret := "test-secret-12345"
	identity := "https://mastodon.social/@alice"

	token, err := SignMastodonToken(identity, secret)
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	verified, err := VerifyMastodonToken(token, secret)
	assert.NoError(t, err)
	assert.Equal(t, identity, verified)

	_, err = VerifyMastodonToken(token, "wrong-secret")
	assert.Error(t, err)
}

func TestParseInstanceHost(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"@alice@mastodon.social", "mastodon.social"},
		{"mastodon.social", "mastodon.social"},
		{"https://fosstodon.org/@bob", "fosstodon.org"},
		{"http://local.server:4566", "local.server:4566"},
		{"", ""},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, ParseInstanceHost(tc.input))
	}
}

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

func TestGetAndPutMastodonClient(t *testing.T) {
	os.Setenv("DYNAMO_MASTODON_CLIENTS_TABLE", "mastodon_clients")
	ctx := context.Background()
	mockDb := new(MockDynamoDB)

	mockDb.On("GetItem", ctx, mock.Anything).Return(&dynamodb.GetItemOutput{Item: nil}, nil)
	mockDb.On("PutItem", ctx, mock.Anything).Return(&dynamodb.PutItemOutput{}, nil)

	client, err := GetMastodonClient(ctx, mockDb, "mastodon.social")
	assert.NoError(t, err)
	assert.Empty(t, client.ClientID)

	err = PutMastodonClient(ctx, mockDb, MastodonClient{
		InstanceHost: "mastodon.social",
		ClientID:     "123",
		ClientSecret: "456",
	})
	assert.NoError(t, err)

	mockDb.AssertExpectations(t)
}

func TestVerifyMastodonCredentials(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer valid-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(AccountResponse{
			Username:    "alice",
			DisplayName: "Alice Smith",
			URL:         "https://mastodon.social/@alice",
		})
	}))
	defer server.Close()

	u := server.URL[7:] // strip http://

	username, displayName, profileURL, err := VerifyMastodonCredentials(context.Background(), u, "valid-token")
	assert.NoError(t, err)
	assert.Equal(t, "alice", username)
	assert.Equal(t, "Alice Smith", displayName)
	assert.Equal(t, "https://mastodon.social/@alice", profileURL)
}

func TestRegisterMastodonApp(t *testing.T) {
	os.Setenv("MASTODON_CLIENT_NAME", "Test Client Name")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/api/v1/apps", r.URL.Path)

		err := r.ParseForm()
		assert.NoError(t, err)
		assert.Equal(t, "Test Client Name", r.FormValue("client_name"))
		assert.Equal(t, "https://example.com/callback", r.FormValue("redirect_uris"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(RegistrationResponse{
			ClientID:     "client-id-123",
			ClientSecret: "client-secret-456",
		})
	}))
	defer server.Close()

	u := server.URL[7:] // strip http://

	clientID, clientSecret, err := RegisterMastodonApp(context.Background(), u, "https://example.com/callback")
	assert.NoError(t, err)
	assert.Equal(t, "client-id-123", clientID)
	assert.Equal(t, "client-secret-456", clientSecret)
}

func TestGetMastodonAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/oauth/token", r.URL.Path)

		err := r.ParseForm()
		assert.NoError(t, err)
		assert.Equal(t, "client-id-123", r.FormValue("client_id"))
		assert.Equal(t, "client-secret-456", r.FormValue("client_secret"))
		assert.Equal(t, "auth-code-789", r.FormValue("code"))
		assert.Equal(t, "authorization_code", r.FormValue("grant_type"))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(TokenResponse{
			AccessToken: "access-token-abc",
		})
	}))
	defer server.Close()

	u := server.URL[7:] // strip http://

	token, err := GetMastodonAccessToken(context.Background(), u, "client-id-123", "client-secret-456", "auth-code-789", "https://example.com/callback")
	assert.NoError(t, err)
	assert.Equal(t, "access-token-abc", token)
}

func TestGetMastodonClient_Errors(t *testing.T) {
	ctx := context.Background()
	mockDb := new(MockDynamoDB)

	mockDb.On("GetItem", ctx, mock.Anything).Return(nil, errors.New("db error"))

	_, err := GetMastodonClient(ctx, mockDb, "mastodon.social")
	assert.Error(t, err)
}

func TestVerifyMastodonCredentials_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte("unauthorized"))
	}))
	defer server.Close()

	u := server.URL[7:]

	_, _, _, err := VerifyMastodonCredentials(context.Background(), u, "invalid-token")
	assert.Error(t, err)
}
