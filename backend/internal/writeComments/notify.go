package writeComments

import (
	"context"
	"spiritriot-comment-services/internal/common"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

// sendCommentNotification sends a comment received message to an SNS topic
func sendCommentNotification(ctx context.Context, snsClient snsService, data common.CommentEntryData) error {
	// Retrieve the SNS topic ARN from environment variables
	topicArn := common.GetEnvVar("NOTIFICATION_TOPIC_ARN", "")

	// Construct the message using environment variable and inputs
	message := fmt.Sprintf("%s\n\nPost: %s\nAuthor: %s\nComment:\n%s",
		common.GetEnvVar("NOTIFICATION_HEADER", ""), data.PostOrigin, data.Name, data.Comment)

	// Prepare the PublishInput parameters
	params := &sns.PublishInput{
		Message:  aws.String(message),
		TopicArn: aws.String(topicArn),
	}

	// Send the message to the SNS topic
	_, err := snsClient.Publish(ctx, params)
	if err != nil {
		return fmt.Errorf("failed to publish message to SNS: %v", err)
	}

	return nil
}
