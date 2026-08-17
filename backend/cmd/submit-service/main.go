package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"spiritriot-comment-services/internal/common"
	"spiritriot-comment-services/internal/writeComments"

	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

type CommentData struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	Honeypot       string `json:"website"`
	Comment        string `json:"comment"`
	Page           string `json:"page"`
	Origin         string `json:"origin"`
	Private        bool   `json:"private"`
	GoogleToken    string `json:"google_token,omitempty"`
	MigrateAccount bool   `json:"migrate_account,omitempty"`
	IndieAuthToken string `json:"indieauth_token,omitempty"`
	MastodonToken  string `json:"mastodon_token,omitempty"`
}



type dynamoService interface {
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
}

type snsService interface {
	Publish(context.Context, *sns.PublishInput, ...func(*sns.Options)) (*sns.PublishOutput, error)
}

var (
	dynamoClient dynamoService
	snsClient    snsService
)

func lambdaHandlerWeb(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Received submit-service request: Path=%s, Method=%s, ClientIP=%s", request.Path, request.HTTPMethod, request.RequestContext.Identity.SourceIP)

	if request.HTTPMethod == "OPTIONS" {
		return events.APIGatewayProxyResponse{
			StatusCode: 200,
			Headers:    common.GetCORSHeaders("GET, POST, OPTIONS"),
		}, nil
	}

	// Route auth endpoints
	path := request.Path
	if strings.HasPrefix(path, "/default") {
		path = strings.TrimPrefix(path, "/default")
	}
	path = strings.TrimSuffix(path, "/")

	switch path {
	case "/config":
		return handleConfig(ctx, request)
	case "/auth/mastodon/prompt":
		return handleMastodonPrompt(ctx, request)
	case "/auth/mastodon/init":
		return handleMastodonInit(ctx, request)
	case "/auth/mastodon/callback":
		return handleMastodonCallback(ctx, request)
	case "/auth/indieauth/prompt":
		return handleIndieAuthPrompt(ctx, request)
	case "/auth/indieauth/init":
		return handleIndieAuthInit(ctx, request)
	case "/auth/indieauth/callback":
		return handleIndieAuthCallback(ctx, request)
	}

	var form CommentData
	err := json.Unmarshal([]byte(request.Body), &form)
	if err != nil {
		return standardResponse(403, fmt.Sprintf("error unmarshaling json body: %v", err)), nil
	}

	data := common.CommentEntryData{
		Name:           form.Name,
		Email:          form.Email,
		Honeypot:       form.Honeypot,
		Comment:        form.Comment,
		Page:           form.Page,
		PostOrigin:     form.Origin,
		UserAgent:      request.RequestContext.Identity.UserAgent,
		ClientIP:       request.RequestContext.Identity.SourceIP,
		Referrer:       request.Headers["referer"],
		Private:        form.Private,
		GoogleToken:    form.GoogleToken,
		MigrateAccount: form.MigrateAccount,
		IndieAuthToken: form.IndieAuthToken,
		MastodonToken:  form.MastodonToken,
	}

	allowedReferrers := common.GetEnvVar("HTTP_ALLOWED_REFERRERS", "")
	if !common.ValidateReferrer(data.Referrer, allowedReferrers) {
		log.Printf("Referrer check failed: Referer=%q (Allowed: %q)", data.Referrer, allowedReferrers)
		return standardResponse(403, "comment rejected"), nil
	}
	if !common.ValidateReferrer(data.PostOrigin, allowedReferrers) {
		log.Printf("PostOrigin check failed: PostOrigin=%q (Allowed: %q)", data.PostOrigin, allowedReferrers)
		return standardResponse(403, "comment rejected"), nil
	}

	if dynamoClient == nil || snsClient == nil {
		log.Println("loading config")
		cfg, err := common.LoadAWSConfig(ctx)
		if err != nil {
			return standardResponse(500, fmt.Sprintf("error loading aws config: %v", err)), nil
		}
		dynamoClient = dynamodb.NewFromConfig(cfg)
		snsClient = sns.NewFromConfig(cfg)
	}

	if err := writeComments.SaveComment(ctx, dynamoClient, snsClient, data); err != nil {
		log.Printf("comment rejected: %v", err)
		if err.Error() == "linking_consent_required" {
			return standardResponse(409, "linking_consent_required"), nil
		}
		return standardResponse(403, "comment rejected"), nil
	}

	return standardResponse(200, "comment accepted"), nil
}

func standardResponse(statusCode int, body string) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers:    common.GetCORSHeaders("GET, POST, OPTIONS"),
		Body:       body,
	}
}

func handleConfig(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	googleClientID := common.GetEnvVar("GOOGLE_CLIENT_ID", "")
	configData := map[string]string{
		"googleClientId": googleClientID,
	}
	bodyBytes, err := json.Marshal(configData)
	if err != nil {
		return standardResponse(500, fmt.Sprintf("error marshaling config: %v", err)), nil
	}
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    common.GetCORSHeaders("GET, POST, OPTIONS"),
		Body:       string(bodyBytes),
	}, nil
}

func main() {
	lambda.Start(lambdaHandlerWeb)
}
