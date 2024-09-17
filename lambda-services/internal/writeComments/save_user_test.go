package writeComments

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Define a mock DynamoDB client
type MockDynamoDBClient struct {
	mock.Mock
}

func (m *MockDynamoDBClient) GetItem(ctx context.Context, input *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	args := m.Called(ctx, input, opts)
	if result, ok := args.Get(0).(*dynamodb.GetItemOutput); ok {
		return result, args.Error(1)
	}
	return nil, args.Error(1)
}

func (m *MockDynamoDBClient) PutItem(ctx context.Context, input *dynamodb.PutItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	args := m.Called(ctx, input, opts)
	if result, ok := args.Get(0).(*dynamodb.PutItemOutput); ok {
		return result, args.Error(1)
	}
	return nil, args.Error(1)
}

func TestGetUser_Found(t *testing.T) {
	os.Setenv("DYNAMO_USER_TABLE", "table")

	mockSvc := new(MockDynamoDBClient)

	ctx := context.TODO()

	expectedAccount := UserAccount{Author: "testUser"}
	item, _ := attributevalue.MarshalMap(expectedAccount)

	mockSvc.On("GetItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{
		Item: item,
	}, nil)

	result, err := getUser(ctx, mockSvc, "testUser")

	assert.NoError(t, err)
	assert.Equal(t, expectedAccount, result)
	mockSvc.AssertExpectations(t)
}

func TestGetUser_NotFound(t *testing.T) {
	os.Setenv("DYNAMO_USER_TABLE", "table")

	mockSvc := new(MockDynamoDBClient)

	ctx := context.TODO()

	mockSvc.On("GetItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.GetItemOutput{
		Item: nil,
	}, nil)

	result, err := getUser(ctx, mockSvc, "nonExistentUser")

	assert.NoError(t, err)
	assert.Equal(t, UserAccount{}, result)
	mockSvc.AssertExpectations(t)
}

func TestGetUser_GetItemError(t *testing.T) {
	os.Setenv("DYNAMO_USER_TABLE", "table")

	mockSvc := new(MockDynamoDBClient)

	ctx := context.TODO()

	mockSvc.On("GetItem", ctx, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("dynamodb error"))

	result, err := getUser(ctx, mockSvc, "testUser")

	assert.Error(t, err)
	assert.EqualError(t, err, "failed to get item: dynamodb error")
	assert.Equal(t, UserAccount{}, result)
	mockSvc.AssertExpectations(t)
}

func TestPutUser_Success(t *testing.T) {
	os.Setenv("DYNAMO_USER_TABLE", "table")

	mockSvc := new(MockDynamoDBClient)

	ctx := context.TODO()
	account := UserAccount{Author: "testUser", AuthorID: "email:test@example.com"}

	mockSvc.On("PutItem", ctx, mock.Anything, mock.Anything).Return(&dynamodb.PutItemOutput{}, nil)

	err := putUser(ctx, mockSvc, account)

	assert.NoError(t, err)
	mockSvc.AssertCalled(t, "PutItem", ctx, mock.Anything, mock.Anything)
	mockSvc.AssertExpectations(t)
}

func TestPutUser_PutItemError(t *testing.T) {
	os.Setenv("DYNAMO_USER_TABLE", "table")

	mockSvc := new(MockDynamoDBClient)

	ctx := context.TODO()
	account := UserAccount{Author: "testUser", AuthorID: "email:test@example.com"}

	mockSvc.On("PutItem", ctx, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("DynamoDB error"))

	err := putUser(ctx, mockSvc, account)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to put item")
	mockSvc.AssertCalled(t, "PutItem", ctx, mock.Anything, mock.Anything)
	mockSvc.AssertExpectations(t)
}
