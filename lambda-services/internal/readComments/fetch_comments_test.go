package readComments

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockDynamoQueryClient struct {
	mock.Mock
}

func (m *MockDynamoQueryClient) Query(ctx context.Context, params *dynamodb.QueryInput, optFns ...func(*dynamodb.Options)) (*dynamodb.QueryOutput, error) {
	args := m.Called(ctx, params)
	if result, ok := args.Get(0).(*dynamodb.QueryOutput); ok {
		return result, args.Error(1)
	}
	return nil, args.Error(1)
}

func TestQuery_Success(t *testing.T) {
	ctx := context.TODO()
	mockSvc := new(MockDynamoQueryClient)

	commentsList := []CommentItem{
		{
			Date:       "2026-08-01T12:00:00Z",
			Author:     "Alice",
			Content:    "Hello!",
			UserID:     "user-1",
			Verified:   true,
			ProfileURL: "https://example.com/alice",
		},
		{
			Date:       "2026-08-02T12:00:00Z",
			Author:     "Bob",
			Content:    "Hi!",
			UserID:     "user-2",
			Verified:   false,
		},
	}

	var items []map[string]types.AttributeValue
	for _, c := range commentsList {
		av, _ := attributevalue.MarshalMap(c)
		items = append(items, av)
	}

	mockSvc.On("Query", ctx, mock.Anything).Return(&dynamodb.QueryOutput{
		Items: items,
		Count: int32(len(items)),
	}, nil)

	results, err := Query(ctx, mockSvc, "/test-page")
	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "Alice", results[0].Author)
	assert.Equal(t, "Bob", results[1].Author)

	mockSvc.AssertExpectations(t)
}

func TestQuery_Error(t *testing.T) {
	ctx := context.TODO()
	mockSvc := new(MockDynamoQueryClient)

	mockSvc.On("Query", ctx, mock.Anything).Return(nil, errors.New("db error"))

	results, err := Query(ctx, mockSvc, "/test-page")
	assert.Error(t, err)
	assert.Empty(t, results)

	mockSvc.AssertExpectations(t)
}

func TestQuery_UnmarshalError(t *testing.T) {
	ctx := context.TODO()
	mockSvc := new(MockDynamoQueryClient)

	invalidItem := map[string]types.AttributeValue{
		"author":   &types.AttributeValueMemberS{Value: "Alice"},
		"verified": &types.AttributeValueMemberS{Value: "invalid-bool-type"},
	}

	mockSvc.On("Query", ctx, mock.Anything).Return(&dynamodb.QueryOutput{
		Items: []map[string]types.AttributeValue{invalidItem},
		Count: 1,
	}, nil)

	results, err := Query(ctx, mockSvc, "/test-page")
	assert.Error(t, err)
	assert.Empty(t, results)

	mockSvc.AssertExpectations(t)
}
