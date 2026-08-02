package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	"endgameviable-comment-services/internal/common"
	"endgameviable-comment-services/internal/readComments"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)



type dynamoQueryAPI interface {
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

var dynamoClient dynamoQueryAPI

func lambdaHandlerWeb(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	page := request.QueryStringParameters["page"]
	if page == "" {
		return errorResponse("invalid page"), nil
	}

	if dynamoClient == nil {
		log.Println("loading config")
		cfg, err := common.LoadAWSConfig(ctx)
		if err != nil {
			return errorResponse(fmt.Sprintf("error loading aws config: %v", err)), nil
		}
		dynamoClient = dynamodb.NewFromConfig(cfg)
	}

	log.Printf("fetching comments for %s", page)
	comments, err := readComments.Query(ctx, dynamoClient, page)
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
		Headers:    common.GetCORSHeaders("GET, OPTIONS"),
		Body:       string(responseBody),
	}, nil
}

func errorResponse(err string) events.APIGatewayProxyResponse {
	return events.APIGatewayProxyResponse{
		StatusCode: 500,
		Headers:    common.GetCORSHeaders("GET, OPTIONS"),
		Body:       err,
	}
}

func main() {
	lambda.Start(lambdaHandlerWeb)
}
