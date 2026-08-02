package writeComments

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func encodeBase64URL(b []byte) string {
	s := base64.URLEncoding.EncodeToString(b)
	return strings.TrimRight(s, "=")
}

func createTestToken(t *testing.T, key *rsa.PrivateKey, kid string, claims GoogleClaims) string {
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

func TestVerifyGoogleToken(t *testing.T) {
	// Generate RSA key pair for testing
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	kid := "test-key-id-123"

	// Mock JWKS Server
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

	validClaims := GoogleClaims{
		Issuer:        "https://accounts.google.com",
		Subject:       "google-user-1",
		Audience:      "my-client-id",
		Expiry:        time.Now().Add(1 * time.Hour).Unix(),
		Email:         "test@example.com",
		EmailVerified: true,
		Name:          "Test User",
	}

	t.Run("Valid Token", func(t *testing.T) {
		token := createTestToken(t, privateKey, kid, validClaims)
		claims, err := VerifyGoogleToken(token, "my-client-id", mockServer.URL)
		if err != nil {
			t.Fatalf("expected valid token, got error: %v", err)
		}
		if claims.Subject != validClaims.Subject {
			t.Errorf("expected subject %s, got %s", validClaims.Subject, claims.Subject)
		}
	})

	t.Run("Expired Token", func(t *testing.T) {
		expiredClaims := validClaims
		expiredClaims.Expiry = time.Now().Add(-1 * time.Hour).Unix()
		token := createTestToken(t, privateKey, kid, expiredClaims)
		_, err := VerifyGoogleToken(token, "my-client-id", mockServer.URL)
		if err == nil || !strings.Contains(err.Error(), "token has expired") {
			t.Errorf("expected expired error, got: %v", err)
		}
	})

	t.Run("Invalid Audience", func(t *testing.T) {
		token := createTestToken(t, privateKey, kid, validClaims)
		_, err := VerifyGoogleToken(token, "different-client-id", mockServer.URL)
		if err == nil || !strings.Contains(err.Error(), "invalid audience") {
			t.Errorf("expected audience error, got: %v", err)
		}
	})

	t.Run("Invalid Issuer", func(t *testing.T) {
		invalidIssuerClaims := validClaims
		invalidIssuerClaims.Issuer = "https://fake.issuer.com"
		token := createTestToken(t, privateKey, kid, invalidIssuerClaims)
		_, err := VerifyGoogleToken(token, "my-client-id", mockServer.URL)
		if err == nil || !strings.Contains(err.Error(), "invalid issuer") {
			t.Errorf("expected issuer error, got: %v", err)
		}
	})

	t.Run("Email Not Verified", func(t *testing.T) {
		unverifiedClaims := validClaims
		unverifiedClaims.EmailVerified = false
		token := createTestToken(t, privateKey, kid, unverifiedClaims)
		_, err := VerifyGoogleToken(token, "my-client-id", mockServer.URL)
		if err == nil || !strings.Contains(err.Error(), "email is not verified") {
			t.Errorf("expected email verification error, got: %v", err)
		}
	})

	t.Run("Invalid Signature", func(t *testing.T) {
		token := createTestToken(t, privateKey, kid, validClaims)
		// Tamper with signature
		token = token + "tampered"
		_, err := VerifyGoogleToken(token, "my-client-id", mockServer.URL)
		if err == nil || !strings.Contains(err.Error(), "invalid signature") && !strings.Contains(err.Error(), "invalid token format") {
			t.Errorf("expected signature or format error, got: %v", err)
		}
	})

	t.Run("Invalid Token Format (Missing parts)", func(t *testing.T) {
		_, err := VerifyGoogleToken("part1.part2", "my-client-id", mockServer.URL)
		if err == nil || !strings.Contains(err.Error(), "invalid token format") {
			t.Errorf("expected format error, got: %v", err)
		}
	})

	t.Run("Unexpected Signing Algorithm", func(t *testing.T) {
		header := struct {
			Kid string `json:"kid"`
			Alg string `json:"alg"`
		}{
			Kid: kid,
			Alg: "HS256",
		}
		headerBytes, _ := json.Marshal(header)
		token := encodeBase64URL(headerBytes) + "." + encodeBase64URL([]byte(`{}`)) + "." + encodeBase64URL([]byte(`sig`))
		_, err := VerifyGoogleToken(token, "my-client-id", mockServer.URL)
		if err == nil || !strings.Contains(err.Error(), "unexpected signing algorithm") {
			t.Errorf("expected algorithm error, got: %v", err)
		}
	})
}
