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
	"strings"
	"time"

	"endgameviable-comment-services/internal/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const mastodonClientsTableVar = "DYNAMO_MASTODON_CLIENTS_TABLE"

type MastodonClient struct {
	InstanceHost string `dynamodbav:"instance_host"`
	ClientID     string `dynamodbav:"client_id"`
	ClientSecret string `dynamodbav:"client_secret"`
	CreatedAt    string `dynamodbav:"created_at"`
}

type mastodonJWTClaims struct {
	Sub string `json:"sub"`
	Exp int64  `json:"exp"`
	Iss string `json:"iss"`
}

func SignMastodonToken(identity string, secret string) (string, error) {
	if secret == "" {
		return "", fmt.Errorf("JWT secret is empty")
	}

	header := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"

	claims := mastodonJWTClaims{
		Sub: identity,
		Exp: time.Now().Add(30 * 24 * time.Hour).Unix(), // 30 days
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

func VerifyMastodonToken(tokenString string, secret string) (string, error) {
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

	var claims mastodonJWTClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return "", fmt.Errorf("failed to parse claims")
	}

	if time.Now().Unix() > claims.Exp {
		return "", fmt.Errorf("token has expired")
	}

	return claims.Sub, nil
}

func GetMastodonClient(ctx context.Context, svc dynamoService, instanceHost string) (MastodonClient, error) {
	tableName := common.GetEnvVar(mastodonClientsTableVar, "endgameviable_mastodon_clients")

	var client MastodonClient

	params := &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"instance_host": &types.AttributeValueMemberS{Value: instanceHost},
		},
	}

	result, err := svc.GetItem(ctx, params)
	if err != nil {
		return client, fmt.Errorf("failed to get mastodon client: %v", err)
	}

	if result.Item == nil {
		return client, nil
	}

	err = attributevalue.UnmarshalMap(result.Item, &client)
	if err != nil {
		return client, fmt.Errorf("failed to unmarshal mastodon client: %v", err)
	}

	return client, nil
}

func PutMastodonClient(ctx context.Context, svc dynamoService, client MastodonClient) error {
	tableName := common.GetEnvVar(mastodonClientsTableVar, "endgameviable_mastodon_clients")

	av, err := attributevalue.MarshalMap(client)
	if err != nil {
		return fmt.Errorf("failed to marshal mastodon client: %v", err)
	}

	params := &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      av,
	}

	_, err = svc.PutItem(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to put mastodon client: %v", err)
	}

	return nil
}

type RegistrationResponse struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
}

func getMastodonURL(instanceHost string, path string) string {
	scheme := "https"
	if strings.HasPrefix(instanceHost, "localhost:") || strings.HasPrefix(instanceHost, "127.0.0.1:") || strings.Contains(instanceHost, ".local") {
		scheme = "http"
	}
	return fmt.Sprintf("%s://%s%s", scheme, instanceHost, path)
}

func RegisterMastodonApp(ctx context.Context, instanceHost string, redirectURI string) (string, string, error) {
	registerURL := getMastodonURL(instanceHost, "/api/v1/apps")

	data := url.Values{}
	clientName := common.GetEnvVar("MASTODON_CLIENT_NAME", "SpiritRiot Comments")
	data.Set("client_name", clientName)
	data.Set("redirect_uris", redirectURI)
	data.Set("scopes", "read:accounts")
	data.Set("website", "https://endgameviable.com")

	req, err := http.NewRequestWithContext(ctx, "POST", registerURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("failed to create dynamic registration request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("dynamic registration request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", fmt.Errorf("registration returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var regResp RegistrationResponse
	if err := json.NewDecoder(resp.Body).Decode(&regResp); err != nil {
		return "", "", fmt.Errorf("failed to decode registration response: %w", err)
	}

	return regResp.ClientID, regResp.ClientSecret, nil
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
}

func GetMastodonAccessToken(ctx context.Context, instanceHost string, clientID string, clientSecret string, code string, redirectURI string) (string, error) {
	tokenURL := getMastodonURL(instanceHost, "/oauth/token")

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("code", code)
	data.Set("grant_type", "authorization_code")
	data.Set("redirect_uri", redirectURI)
	data.Set("scope", "read:accounts")

	req, err := http.NewRequestWithContext(ctx, "POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return "", fmt.Errorf("failed to create token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token exchange returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var tokResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	return tokResp.AccessToken, nil
}

type AccountResponse struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	URL         string `json:"url"`
}

func VerifyMastodonCredentials(ctx context.Context, instanceHost string, accessToken string) (string, string, string, error) {
	verifyURL := getMastodonURL(instanceHost, "/api/v1/accounts/verify_credentials")

	req, err := http.NewRequestWithContext(ctx, "GET", verifyURL, nil)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create credentials verification request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", "", fmt.Errorf("credentials verification request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", "", "", fmt.Errorf("credentials verification returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var accResp AccountResponse
	if err := json.NewDecoder(resp.Body).Decode(&accResp); err != nil {
		return "", "", "", fmt.Errorf("failed to decode account response: %w", err)
	}

	return accResp.Username, accResp.DisplayName, accResp.URL, nil
}

func ParseInstanceHost(input string) string {
	input = strings.TrimSpace(input)
	if input == "" {
		return ""
	}
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		if u, err := url.Parse(input); err == nil {
			return u.Host
		}
	}
	if strings.HasPrefix(input, "@") {
		input = input[1:]
	}
	if strings.Contains(input, "@") {
		parts := strings.Split(input, "@")
		if len(parts) > 1 && parts[1] != "" {
			input = parts[1]
		}
	}
	return input
}
