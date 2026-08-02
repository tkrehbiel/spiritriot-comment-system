package writeComments

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var (
	indieAuthHTTPClient = &http.Client{Timeout: 10 * time.Second}
	reAuth              = regexp.MustCompile(`(?i)<link\s+[^>]*rel=["'](?:authorization_endpoint|auth-endpoint)["'][^>]*href=["']([^"']+)["']`)
	reAuthReverse       = regexp.MustCompile(`(?i)<link\s+[^>]*href=["']([^"']+)["'][^>]*rel=["'](?:authorization_endpoint|auth-endpoint)["']`)
)

// DiscoverEndpoints fetches the profile page and finds the IndieAuth authorization endpoint.
func DiscoverEndpoints(ctx context.Context, profileURL string) (string, error) {
	u, err := url.Parse(profileURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return "", fmt.Errorf("invalid profile URL")
	}

	req, err := http.NewRequestWithContext(ctx, "GET", profileURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (SpiritRiot Comment System)")

	resp, err := indieAuthHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("profile page returned status %d", resp.StatusCode)
	}

	// 1. Check Link Headers
	for _, linkHeader := range resp.Header["Link"] {
		endpoint := parseLinkHeader(linkHeader, "authorization_endpoint")
		if endpoint != "" {
			return resolveURL(profileURL, endpoint), nil
		}
	}

	// 2. Read first 128KB of response body to search for HTML tags
	limitReader := io.LimitReader(resp.Body, 128*1024)
	bodyBytes, err := io.ReadAll(limitReader)
	if err != nil {
		return "", err
	}
	bodyStr := string(bodyBytes)

	matches := reAuth.FindStringSubmatch(bodyStr)
	if len(matches) > 1 {
		return resolveURL(profileURL, matches[1]), nil
	}

	matchesReverse := reAuthReverse.FindStringSubmatch(bodyStr)
	if len(matchesReverse) > 1 {
		return resolveURL(profileURL, matchesReverse[1]), nil
	}

	return "", fmt.Errorf("authorization endpoint not found on profile page")
}

func parseLinkHeader(header, relTarget string) string {
	parts := strings.Split(header, ",")
	for _, part := range parts {
		if !strings.Contains(part, fmt.Sprintf(`rel="%s"`, relTarget)) && !strings.Contains(part, fmt.Sprintf(`rel='%s'`, relTarget)) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start != -1 && end != -1 && end > start {
			return part[start+1 : end]
		}
	}
	return ""
}

func resolveURL(base, ref string) string {
	baseURL, err := url.Parse(base)
	if err != nil {
		return ref
	}
	refURL, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	return baseURL.ResolveReference(refURL).String()
}

// VerifyIndieAuthCode exchanges the authorization code with the endpoint.
func VerifyIndieAuthCode(ctx context.Context, endpoint string, code string, me string, clientID string, redirectURI string) (string, error) {
	data := url.Values{}
	data.Set("grant_type", "authorization_code")
	data.Set("code", code)
	data.Set("client_id", clientID)
	data.Set("redirect_uri", redirectURI)
	data.Set("me", me)

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (SpiritRiot Comment System)")

	resp, err := indieAuthHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Me string `json:"me"`
	}
	if err := json.Unmarshal(bodyBytes, &result); err == nil && result.Me != "" {
		return normalizeURL(result.Me), nil
	}

	values, err := url.ParseQuery(string(bodyBytes))
	if err == nil && values.Get("me") != "" {
		return normalizeURL(values.Get("me")), nil
	}

	return "", fmt.Errorf("failed to parse verification response: %s", string(bodyBytes))
}

func normalizeURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return raw
	}
	return fmt.Sprintf("%s://%s%s", u.Scheme, u.Host, strings.TrimSuffix(u.Path, "/"))
}

type localJWTClaims struct {
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
	Iss string `json:"iss"`
}

// SignIndieAuthToken signs a local JWT token with the secret key using HS256.
func SignIndieAuthToken(identity string, secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("JWT secret is empty")
	}

	header := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9" // Base64Url of {"alg":"HS256","typ":"JWT"}

	claims := localJWTClaims{
		Sub: identity,
		Exp: time.Now().Add(30 * 24 * time.Hour).Unix(), // 30 day expiration
		Iss: "spiritriot",
	}

	claimsBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	payload := base64.RawURLEncoding.EncodeToString(claimsBytes)
	message := header + "." + payload

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	signature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	return message + "." + signature, nil
}

// VerifyIndieAuthToken verifies the local JWT token signature and returns the identity.
func VerifyIndieAuthToken(tokenString string, secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("JWT secret is empty")
	}

	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid token format")
	}

	message := parts[0] + "." + parts[1]

	h := hmac.New(sha256.New, []byte(secret))
	h.Write([]byte(message))
	expectedSignature := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

	if !hmac.Equal([]byte(parts[2]), []byte(expectedSignature)) {
		return "", fmt.Errorf("signature verification failed")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", fmt.Errorf("failed to decode payload")
	}

	var claims localJWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return "", fmt.Errorf("failed to parse claims")
	}

	if time.Now().Unix() > claims.Exp {
		return "", fmt.Errorf("token expired")
	}

	if claims.Iss != "spiritriot" {
		return "", fmt.Errorf("invalid issuer")
	}

	return claims.Sub, nil
}
