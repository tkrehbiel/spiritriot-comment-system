package writeComments

import (
	"context"
	"fmt"
	"log"
	"os"
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
	ID         string `json:"id" dynamodbav:"id"`
	Date       string `json:"date" dynamodbav:"date"`
	Author     string `json:"author" dynamodbav:"author"`
	UserID     string `json:"user_id" dynamodbav:"user_id"`
	Content    string `json:"content" dynamodbav:"content"`
	Page       string `json:"page" dynamodbav:"page"`
	Source     string `json:"source" dynamodbav:"source"`
	Private    bool   `json:"private,omitempty" dynamodbav:"private,omitempty"`
	Verified   bool   `json:"verified" dynamodbav:"verified"`
	ProfileURL string `json:"profile_url,omitempty" dynamodbav:"profile_url,omitempty"`
}

type snsService interface {
	Publish(context.Context, *sns.PublishInput, ...func(*sns.Options)) (*sns.PublishOutput, error)
}

func SaveComment(ctx context.Context, dynamoService dynamoService, snsClient snsService, data common.CommentEntryData) error {
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

	var accountID string
	var userID string

	if data.GoogleToken != "" {
		googleClientID := common.GetEnvVar("GOOGLE_CLIENT_ID", "")
		certsURL := os.Getenv("GOOGLE_CERTS_URL") // Can be set in tests
		claims, err := VerifyGoogleToken(data.GoogleToken, googleClientID, certsURL)
		if err != nil {
			return fmt.Errorf("google token verification failed: %w", err)
		}

		accountID = fmt.Sprintf("google:%s", claims.Subject)

		if account.Author != "" {
			if account.AuthorID == accountID {
				userID = account.UserID
			} else if account.AuthorID == fmt.Sprintf("email:%s", claims.Email) {
				// User email matches existing legacy guest account email - check migration consent
				if data.MigrateAccount {
					log.Printf("Migrating account for user %s to Google ID", data.Name)
					account.AuthorID = accountID
					if err := putUser(ctx, dynamoService, account); err != nil {
						return fmt.Errorf("failed to upgrade user account: %w", err)
					}
					userID = account.UserID
				} else {
					return fmt.Errorf("linking_consent_required")
				}
			} else {
				return fmt.Errorf("username taken")
			}
		} else {
			userID = uuid.NewString()
			log.Println("saving new Google-verified user to dynamodb")
			if err := putUser(ctx, dynamoService, UserAccount{
				Author:   data.Name,
				UserID:   userID,
				AuthorID: accountID,
			}); err != nil {
				log.Printf("error saving user account: %v", err)
			}
		}
		data.Verified = true
	} else if data.MastodonToken != "" {
		jwtSecret := common.GetEnvVar("JWT_SECRET", "local-development-secret-key-12345")
		verifiedIdentity, err := VerifyMastodonToken(data.MastodonToken, jwtSecret)
		if err != nil {
			return fmt.Errorf("mastodon token verification failed: %w", err)
		}

		accountID = fmt.Sprintf("mastodon:%s", verifiedIdentity)

		if account.Author != "" {
			if account.AuthorID == accountID {
				userID = account.UserID
			} else {
				return fmt.Errorf("username taken")
			}
		} else {
			userID = uuid.NewString()
			log.Println("saving new Mastodon-verified user to dynamodb")
			if err := putUser(ctx, dynamoService, UserAccount{
				Author:     data.Name,
				UserID:     userID,
				AuthorID:   accountID,
				ProfileURL: verifiedIdentity,
			}); err != nil {
				log.Printf("error saving user account: %v", err)
			}
		}
		data.Verified = true
		data.ProfileURL = verifiedIdentity
	} else if data.IndieAuthToken != "" {
		jwtSecret := common.GetEnvVar("JWT_SECRET", "local-development-secret-key-12345")
		verifiedIdentity, err := VerifyIndieAuthToken(data.IndieAuthToken, jwtSecret)
		if err != nil {
			return fmt.Errorf("indieauth token verification failed: %w", err)
		}

		accountID = fmt.Sprintf("indieauth:%s", verifiedIdentity)

		if account.Author != "" {
			if account.AuthorID == accountID {
				userID = account.UserID
			} else {
				return fmt.Errorf("username taken")
			}
		} else {
			userID = uuid.NewString()
			log.Println("saving new IndieAuth-verified user to dynamodb")
			if err := putUser(ctx, dynamoService, UserAccount{
				Author:     data.Name,
				UserID:     userID,
				AuthorID:   accountID,
				ProfileURL: verifiedIdentity,
			}); err != nil {
				log.Printf("error saving user account: %v", err)
			}
		}
		data.Verified = true
		data.ProfileURL = verifiedIdentity
	} else {
		// Guest submission
		accountID = fmt.Sprintf("email:%s", data.Email)
		if account.Author != "" {
			if account.AuthorID != accountID {
				return fmt.Errorf("email doesn't match")
			}
			userID = account.UserID
		} else {
			userID = uuid.NewString()
			log.Println("saving legacy user to dynamodb")
			if err := putUser(ctx, dynamoService, UserAccount{
				Author:   data.Name,
				UserID:   userID,
				AuthorID: accountID,
			}); err != nil {
				log.Printf("error saving user account: %v", err)
			}
		}
		data.Verified = false
	}

	data.UserID = userID

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
func putItem(ctx context.Context, svc dynamoService, data common.CommentEntryData) error {
	commentTableName := common.GetEnvVar(commentTableVar, "")

	item := CommentSaveItem{
		ID:         uuid.NewString(),
		Date:       time.Now().UTC().Format(common.CommentDateFormat),
		Page:       data.Page,
		Author:     data.Name,
		UserID:     data.UserID,
		Content:    data.Comment,
		Source:     "form2",
		Private:    data.Private,
		Verified:   data.Verified,
		ProfileURL: data.ProfileURL,
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
