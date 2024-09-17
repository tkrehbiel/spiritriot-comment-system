package writeComments

import (
	"context"
	"endgameviable-comment-services/internal/common"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const userTableVar = "DYNAMO_USER_TABLE"

type UserAccount struct {
	Author   string `json:"author" dynamodbav:"author"`
	AuthorID string `json:"author_id" dynamodbav:"author_id"`
}

type dynamoService interface {
	GetItem(context.Context, *dynamodb.GetItemInput, ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(context.Context, *dynamodb.PutItemInput, ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
}

func getUser(ctx context.Context, svc dynamoService, username string) (UserAccount, error) {
	userTableName := common.GetEnvVar(userTableVar, "")

	var account UserAccount

	params := &dynamodb.GetItemInput{
		TableName: aws.String(userTableName),
		Key: map[string]types.AttributeValue{
			"author": &types.AttributeValueMemberS{Value: username},
		},
	}

	result, err := svc.GetItem(ctx, params)
	if err != nil {
		return account, fmt.Errorf("failed to get item: %v", err)
	}

	if result.Item == nil {
		// return empty account if not found
		return account, nil
	}

	err = attributevalue.UnmarshalMap(result.Item, &account)
	if err != nil {
		return account, fmt.Errorf("failed to unmarshal result to User struct: %v", err)
	}

	return account, nil
}

func putUser(ctx context.Context, svc dynamoService, account UserAccount) error {
	userTableName := common.GetEnvVar(userTableVar, "")

	av, err := attributevalue.MarshalMap(account)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %v", err)
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(userTableName),
		Item:      av,
	}

	_, err = svc.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to put item: %v", err)
	}

	return nil
}
