package writeComments

import (
	"context"
	"fmt"
	"log"
	"time"

	"endgameviable-comment-services/internal/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/google/uuid"
)

const commentTableVar = "DYNAMO_COMMENT_TABLE"

type CommentApiRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Honeypot string `json:"website"`
	Comment  string `json:"comment"`
	Page     string `json:"page"`
	Origin   string `json:"origin"`
	Date     string `json:"date"`
}

type CommentSaveItem struct {
	ID      string `json:"id" dynamodbav:"id"`
	Date    string `json:"date" dynamodbav:"date"`
	Author  string `json:"author" dynamodbav:"author"`
	Content string `json:"content" dynamodbav:"content"`
	Page    string `json:"page" dynamodbav:"page"`
	Source  string `json:"source" dynamodbav:"source"`
}

type UserSaveItem struct {
	Author   string `json:"author" dynamodbav:"author"`
	AuthorID string `json:"author_id" dynamodbav:"author_id"`
}

func SaveComment(ctx context.Context, dynamoService *dynamodb.Client, snsClient *sns.Client, data common.CommentEntryData) error {
	log.Println(data)

	log.Println("validating form")
	if err := validateComment(data); err != nil {
		log.Printf("form validation failed: %v", err)
		return fmt.Errorf("invalid comment data: %v", err)
	}

	account, err := getUser(ctx, dynamoService, data.Name)
	if err != nil {
		log.Printf("error fetching user account: %v", err)
	}

	accountID := fmt.Sprintf("email:%s", data.Email)
	if account.Author != "" {
		if account.AuthorID != accountID {
			return fmt.Errorf("email doesn't match")
		}
	} else {
		log.Println("saving user to dynamodb")
		if err := putUser(ctx, dynamoService, UserAccount{
			Author:   data.Name,
			AuthorID: accountID,
		}); err != nil {
			log.Printf("error saving user account: %v", err)
		}
	}

	log.Println("saving comment to dynamodb")
	if err := putItem(ctx, dynamoService, data); err != nil {
		return fmt.Errorf("error saving comment: %v", err)
	}

	if err := sendCommentNotification(ctx, snsClient, data); err != nil {
		log.Printf("error sending notification: %v", err)
	}

	return nil
}

// putItem saves a comment to a dynamo table
func putItem(ctx context.Context, svc *dynamodb.Client, data common.CommentEntryData) error {
	commentTableName := common.GetEnvVar(commentTableVar, "")

	item := CommentSaveItem{
		ID:      uuid.NewString(),
		Date:    time.Now().UTC().Format(common.CommentDateFormat),
		Page:    data.Page,
		Author:  data.Name,
		Content: data.Comment,
		Source:  "form2",
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("failed to marshal item: %v", err)
	}

	input := &dynamodb.PutItemInput{
		TableName: aws.String(commentTableName),
		Item:      av,
	}

	_, err = svc.PutItem(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to put item: %v", err)
	}

	return nil
}
