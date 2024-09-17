package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"endgameviable-comment-services/internal/readComments"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

var baseHeaders = map[string]string{
	"Content-Type":                 "application/json",
	"Access-Control-Allow-Headers": "Content-Type,X-Amz-Date,Authorization,X-Api-Key,X-Amz-Security-Token",
	"Access-Control-Allow-Methods": "GET, OPTIONS",
	"Access-Control-Allow-Origin":  "*",
}

func lambdaHandlerWeb(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	page := request.QueryStringParameters["page"]
	if page == "" {
		return errorResponse("invalid page"), nil
	}

	log.Println("loading config")
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		return errorResponse(fmt.Sprintf("error loading aws config: %v", err)), nil
	}

	svc := dynamodb.NewFromConfig(cfg)

	log.Printf("fetching comments for %s", page)
	comments, err := readComments.Query(ctx, svc, page)
	if err != nil {
		return errorResponse(fmt.Sprintf("error getting comments: %v", err)), nil
	}

	log.Println(comments)
	responseBody, err := json.Marshal(comments)
	if err != nil {
		return errorResponse(fmt.Sprintf("error marshalling comments: %v", err)), nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    baseHeaders,
		Body:       string(responseBody),
	}, nil
}

func errorResponse(err string) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: 500,
		Headers:    baseHeaders,
		Body:       err,
	}
}

func main() {
	lambda.Start(lambdaHandlerWeb)
}
