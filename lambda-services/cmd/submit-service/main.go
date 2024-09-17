package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"endgameviable-comment-services/internal/common"
	"endgameviable-comment-services/internal/writeComments"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

type CommentData struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Honeypot string `json:"website"`
	Comment  string `json:"comment"`
	Page     string `json:"page"`
	Origin   string `json:"origin"`
}

var baseHeaders = map[string]string{
	"Content-Type":                 "application/json",
	"Access-Control-Allow-Headers": "Content-Type,X-Amz-Date,Authorization,X-Api-Key,X-Amz-Security-Token",
	"Access-Control-Allow-Methods": "GET, OPTIONS",
	"Access-Control-Allow-Origin":  "*", // TODO: get from env var
}

func lambdaHandlerWeb(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("%v+", request)

	var form CommentData
	err := json.Unmarshal([]byte(request.Body), &form)
	if err != nil {
		return standardResponse(403, fmt.Sprintf("error unmarshaling json body: %v", err)), nil
	}

	data := common.CommentEntryData{
		Name:       form.Name,
		Email:      form.Email,
		Honeypot:   form.Honeypot,
		Comment:    form.Comment,
		Page:       form.Page,
		PostOrigin: form.Origin,
		UserAgent:  request.RequestContext.Identity.UserAgent,
		ClientIP:   request.RequestContext.Identity.SourceIP,
		Referrer:   request.Headers["referer"],
	}

	if !common.ValidateReferrer(data.Referrer, common.GetEnvVar("HTTP_ALLOWED_REFERRERS", "")) {
		log.Printf("referrer missing or not allowed")
		return standardResponse(403, "comment rejected"), nil
	}

	log.Println("loading config")
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		return standardResponse(500, fmt.Sprintf("error loading aws config: %v", err)), nil
	}

	dynamoClient := dynamodb.NewFromConfig(cfg)
	snsClient := sns.NewFromConfig(cfg)

	if err := writeComments.SaveComment(ctx, dynamoClient, snsClient, data); err != nil {
		log.Printf("comment rejected: %v", err)
		return standardResponse(403, "comment rejected"), nil
	}

	return standardResponse(200, "comment accepted"), nil
}

func standardResponse(statusCode int, body string) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers:    baseHeaders,
		Body:       body,
	}
}

func main() {
	lambda.Start(lambdaHandlerWeb)
}
