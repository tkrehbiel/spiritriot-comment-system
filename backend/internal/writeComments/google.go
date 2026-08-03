package writeComments

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

type GoogleClaims struct {
	Issuer        string `json:"iss"`
	Subject       string `json:"sub"`
	Audience      string `json:"aud"`
	Expiry        int64  `json:"exp"`
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

type JWK struct {
	Kid string `json:"kid"`
	N   string `json:"n"`
	E   string `json:"e"`
}

type JWKS struct {
	Keys []JWK `json:"keys"`
}

const defaultCertsURL = "https://www.googleapis.com/oauth2/v3/certs"

var (
	certsCache     map[string]*rsa.PublicKey
	certsCacheTime time.Time
	certsMutex     sync.RWMutex
)

func decodeBase64URL(s string) ([]byte, error) {
	// Pad string if necessary
	if l := len(s) % 4; l > 0 {
		s += strings.Repeat("=", 4-l)
	}
	return base64.URLEncoding.DecodeString(s)
}

func parseJWK(nStr, eStr string) (*rsa.PublicKey, error) {
	nBytes, err := decodeBase64URL(nStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode modulus: %w", err)
	}
	eBytes, err := decodeBase64URL(eStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode exponent: %w", err)
	}

	var e int
	if len(eBytes) < 4 {
		padded := make([]byte, 4)
		copy(padded[4-len(eBytes):], eBytes)
		e = int(big.NewInt(0).SetBytes(padded).Uint64())
	} else {
		e = int(big.NewInt(0).SetBytes(eBytes).Uint64())
	}

	return &rsa.PublicKey{
		N: big.NewInt(0).SetBytes(nBytes),
		E: e,
	}, nil
}

func fetchGoogleCerts(certsURL string) (map[string]*rsa.PublicKey, error) {
	resp, err := http.Get(certsURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch certs: status %d", resp.StatusCode)
	}

	var jwks JWKS
	if err := json.NewDecoder(resp.Body).Decode(&jwks); err != nil {
		return nil, err
	}

	keys := make(map[string]*rsa.PublicKey)
	for _, key := range jwks.Keys {
		pubKey, err := parseJWK(key.N, key.E)
		if err != nil {
			continue
		}
		keys[key.Kid] = pubKey
	}

	return keys, nil
}

func getGooglePublicKey(kid string, certsURL string) (*rsa.PublicKey, error) {
	if certsURL == "" {
		certsURL = defaultCertsURL
	}

	certsMutex.RLock()
	pubKey, exists := certsCache[kid]
	cacheAge := time.Since(certsCacheTime)
	certsMutex.RUnlock()

	if exists && cacheAge < 1*time.Hour {
		return pubKey, nil
	}

	certsMutex.Lock()
	defer certsMutex.Unlock()

	if pubKey, exists = certsCache[kid]; exists && time.Since(certsCacheTime) < 1*time.Hour {
		return pubKey, nil
	}

	newCerts, err := fetchGoogleCerts(certsURL)
	if err != nil {
		if exists {
			return pubKey, nil
		}
		return nil, fmt.Errorf("failed to fetch Google public keys: %w", err)
	}

	certsCache = newCerts
	certsCacheTime = time.Now()

	pubKey, exists = certsCache[kid]
	if !exists {
		return nil, fmt.Errorf("public key not found for kid: %s", kid)
	}

	return pubKey, nil
}

// VerifyGoogleToken verifies the Google ID token and returns the claims
func VerifyGoogleToken(tokenString string, clientID string, certsURL string) (*GoogleClaims, error) {
	tokenParts := strings.Split(tokenString, ".")
	if len(tokenParts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}

	headerBytes, err := decodeBase64URL(tokenParts[0])
	if err != nil {
		return nil, fmt.Errorf("failed to decode header: %w", err)
	}

	var header struct {
		Kid string `json:"kid"`
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("failed to unmarshal header: %w", err)
	}

	if header.Alg != "RS256" {
		return nil, fmt.Errorf("unexpected signing algorithm: %s", header.Alg)
	}

	pubKey, err := getGooglePublicKey(header.Kid, certsURL)
	if err != nil {
		return nil, err
	}

	message := tokenParts[0] + "." + tokenParts[1]
	signatureBytes, err := decodeBase64URL(tokenParts[2])
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}

	h := sha256.New()
	h.Write([]byte(message))
	hashed := h.Sum(nil)

	err = rsa.VerifyPKCS1v15(pubKey, crypto.SHA256, hashed, signatureBytes)
	if err != nil {
		return nil, fmt.Errorf("invalid signature: %w", err)
	}

	payloadBytes, err := decodeBase64URL(tokenParts[1])
	if err != nil {
		return nil, fmt.Errorf("failed to decode payload: %w", err)
	}

	var claims GoogleClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal claims: %w", err)
	}

	if claims.Issuer != "accounts.google.com" && claims.Issuer != "https://accounts.google.com" {
		return nil, fmt.Errorf("invalid issuer: %s", claims.Issuer)
	}

	if claims.Audience != clientID {
		return nil, fmt.Errorf("invalid audience: %s", claims.Audience)
	}

	if time.Now().Unix() > claims.Expiry {
		return nil, fmt.Errorf("token has expired")
	}

	if !claims.EmailVerified {
		return nil, fmt.Errorf("email is not verified")
	}

	return &claims, nil
}
