package readComments

import (
	"context"
	"log"
	"sort"
	"time"

	"spiritriot-comment-services/internal/common"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// CommentItem is what is queried and displayed
type CommentItem struct {
	Date       string `json:"date" dynamodbav:"date"`
	Author     string `json:"author" dynamodbav:"author"`
	Content    string `json:"content" dynamodbav:"content"`
	UserID     string `json:"user_id" dynamodbav:"user_id"`
	Verified   bool   `json:"verified" dynamodbav:"verified"`
	ProfileURL string `json:"profile_url,omitempty" dynamodbav:"profile_url,omitempty"`
}

type DynamoQueryAPI interface {
	Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error)
}

// Query DynamoDB table for comments on a given page
func Query(ctx context.Context, svc DynamoQueryAPI, page string) ([]CommentItem, error) {
	input := &dynamodb.QueryInput{
		TableName:              aws.String(common.GetEnvVar("DYNAMO_COMMENT_TABLE", "")),
		IndexName:              aws.String("page-index-v3"),
		KeyConditionExpression: aws.String("page = :id"),
		FilterExpression:       aws.String("(attribute_not_exists(imported) OR imported = :false_val) AND (attribute_not_exists(#pv) OR #pv = :false_val) AND (attribute_not_exists(#md) OR #md = :false_val)"),
		ProjectionExpression:   aws.String("#dt, #au, #co, #ui, #ve, #pu"),
		ExpressionAttributeNames: map[string]string{
			"#dt": "date",    // reserved word
			"#au": "author",
			"#co": "content", // reserved word
			"#pv": "private", // reserved word
			"#ui": "user_id",
			"#ve": "verified",
			"#pu": "profile_url",
			"#md": "moderate",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":id":        &types.AttributeValueMemberS{Value: page},
			":false_val": &types.AttributeValueMemberBOOL{Value: false},
		},
	}

	log.Println("querying dynamo table")
	result, err := svc.Query(ctx, input)
	if err != nil {
		log.Printf("failed to scan items: %v", err)
		return []CommentItem{}, err
	}

	log.Printf("returned item count: %d", result.Count)

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
