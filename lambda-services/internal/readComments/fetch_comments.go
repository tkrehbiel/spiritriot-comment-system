package readComments

import (
	"context"
	"log"
	"sort"
	"time"

	"endgameviable-comment-services/internal/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// CommentItem is what is queried and displayed
type CommentItem struct {
	Date    string `json:"date" dynamodbav:"date"`
	Author  string `json:"author" dynamodbav:"author"`
	Content string `json:"content" dynamodbav:"content"`
}

// Query DynamoDB table for comments on a given page
func Query(ctx context.Context, svc *dynamodb.Client, page string) ([]CommentItem, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String("endgameviable_comments"),
		IndexName:              aws.String("page-index"),
		KeyConditionExpression: aws.String("page = :id"),
		ProjectionExpression:   aws.String("#dt, #au, #co"),
		ExpressionAttributeNames: map[string]string{
			"#dt": "date", // reserved word
			"#au": "author",
			"#co": "content", // reserved word
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":id": &types.AttributeValueMemberS{Value: page},
		},
	}

	log.Println("querying dynamo table")
	result, err := svc.Query(ctx, input)
	if err != nil {
		log.Printf("failed to scan items: %v", err)
		return []CommentItem{}, err
	}

	log.Printf("returned item count: %d", result.Count)
	log.Println(result.Items)

	items := []CommentItem{}
	for _, item := range result.Items {
		var myItem CommentItem
		err = attributevalue.UnmarshalMap(item, &myItem)
		if err != nil {
			log.Printf("failed to unmarshal record: %v", err)
			return items, err
		}
		items = append(items, myItem)
	}

	log.Println("sorting comments")
	sort.Slice(items, func(i, j int) bool {
		idate, _ := time.Parse(common.CommentDateFormat, items[i].Date)
		jdate, _ := time.Parse(common.CommentDateFormat, items[j].Date)
		return idate.Before(jdate)
	})

	return items, nil
}
