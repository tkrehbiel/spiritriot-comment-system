package writeComments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDiscoverEndpoints(t *testing.T) {
	ctx := context.TODO()

	t.Run("Discovery via HTTP Link Header", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Add("Link", `<http://auth.example.com/oauth>; rel="authorization_endpoint"`)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html></html>"))
		}))
		defer server.Close()

		endpoint, err := DiscoverEndpoints(ctx, server.URL)
		assert.NoError(t, err)
		assert.Equal(t, "http://auth.example.com/oauth", endpoint)
	})

	t.Run("Discovery via HTML Link Tag", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<html>
<head>
    <link rel="authorization_endpoint" href="http://auth.example.com/oauth">
</head>
</html>`))
		}))
		defer server.Close()

		endpoint, err := DiscoverEndpoints(ctx, server.URL)
		assert.NoError(t, err)
		assert.Equal(t, "http://auth.example.com/oauth", endpoint)
	})

	t.Run("Discovery via HTML Link Tag (Reverse attributes)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<html>
<head>
    <link href="http://auth.example.com/oauth-reverse" rel="auth-endpoint">
</head>
</html>`))
		}))
		defer server.Close()

		endpoint, err := DiscoverEndpoints(ctx, server.URL)
		assert.NoError(t, err)
		assert.Equal(t, "http://auth.example.com/oauth-reverse", endpoint)
	})

	t.Run("Discovery fails when endpoint missing", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<html><body>No tags here</body></html>`))
		}))
		defer server.Close()

		_, err := DiscoverEndpoints(ctx, server.URL)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "authorization endpoint not found")
	})
}

func TestVerifyIndieAuthCode(t *testing.T) {
	ctx := context.TODO()

	t.Run("Successful Code Exchange (JSON)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "POST", r.Method)
			assert.Equal(t, "application/x-www-form-urlencoded", r.Header.Get("Content-Type"))

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"me": "https://mastodon.social/@alice"}`))
		}))
		defer server.Close()

		me, err := VerifyIndieAuthCode(ctx, server.URL, "auth-code", "https://mastodon.social/@alice", "client-id", "redirect-uri")
		assert.NoError(t, err)
		assert.Equal(t, "https://mastodon.social/@alice", me)
	})

	t.Run("Successful Code Exchange (Form encoded)", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("me=https%3A%2F%2Fmastodon.social%2F%40bob"))
		}))
		defer server.Close()

		me, err := VerifyIndieAuthCode(ctx, server.URL, "auth-code", "https://mastodon.social/@bob", "client-id", "redirect-uri")
		assert.NoError(t, err)
		assert.Equal(t, "https://mastodon.social/@bob", me)
	})

	t.Run("Failed Exchange returns error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("invalid code"))
		}))
		defer server.Close()

		_, err := VerifyIndieAuthCode(ctx, server.URL, "bad-code", "me", "client-id", "redirect-uri")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "server returned status 400")
	})
}

func TestIndieAuthLocalJWT(t *testing.T) {
	secret := "my-secret-key-123"
	identity := "https://mastodon.social/@alice"

	t.Run("Sign and verify token successfully", func(t *testing.T) {
		token, err := SignIndieAuthToken(identity, secret)
		assert.NoError(t, err)
		assert.NotEmpty(t, token)

		verifiedId, err := VerifyIndieAuthToken(token, secret)
		assert.NoError(t, err)
		assert.Equal(t, identity, verifiedId)
	})

	t.Run("Verification fails with wrong secret", func(t *testing.T) {
		token, err := SignIndieAuthToken(identity, secret)
		assert.NoError(t, err)

		_, err = VerifyIndieAuthToken(token, "wrong-secret-key")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "signature verification failed")
	})

	t.Run("Verification fails on malformed token", func(t *testing.T) {
		_, err := VerifyIndieAuthToken("not.a.jwt", secret)
		assert.Error(t, err)
	})

	t.Run("Verification fails on expired token", func(t *testing.T) {
		// Construct an expired token manually
		header := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
		claims := localJWTClaims{
			Sub: identity,
			Exp: time.Now().Add(-1 * time.Hour).Unix(),
			Iss: "spiritriot",
		}
		claimsBytes, _ := json.Marshal(claims)
		payload := base64RawURLEncode(claimsBytes)
		message := header + "." + payload

		// Sign it
		signature := hmacSha256(message, secret)
		token := message + "." + signature

		_, err := VerifyIndieAuthToken(token, secret)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "token expired")
	})
}

// Helpers
func base64RawURLEncode(data []byte) string {
	// Simple RawURLEncoding
	return base64.RawURLEncoding.EncodeToString(data)
}

func hmacSha256(message, secret string) string {
	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(h.Sum(nil))
}

func TestDiscoverEndpoints_Errors(t *testing.T) {
	ctx := context.TODO()

	t.Run("Invalid Scheme", func(t *testing.T) {
		_, err := DiscoverEndpoints(ctx, "ftp://example.com")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid profile URL")
	})

	t.Run("HTTP Non-200 Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		_, err := DiscoverEndpoints(ctx, server.URL)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "profile page returned status 500")
	})
}

func TestVerifyIndieAuthCode_ParseError(t *testing.T) {
	ctx := context.TODO()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("not-me-and-not-query-encoded"))
	}))
	defer server.Close()

	_, err := VerifyIndieAuthCode(ctx, server.URL, "code", "me", "client-id", "redirect-uri")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse verification response")
}

func TestVerifyIndieAuthToken_InvalidIssuer(t *testing.T) {
	secret := "my-secret-key-123"
	header := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"
	claims := localJWTClaims{
		Sub: "https://mastodon.social/@alice",
		Exp: time.Now().Add(1 * time.Hour).Unix(),
		Iss: "fake-issuer",
	}
	claimsBytes, _ := json.Marshal(claims)
	payload := base64RawURLEncode(claimsBytes)
	message := header + "." + payload
	signature := hmacSha256(message, secret)
	token := message + "." + signature

	_, err := VerifyIndieAuthToken(token, secret)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid issuer")
}
