package writeComments

import (
	"context"
	"fmt"
	"os"
	"time"

	"spiritriot-comment-services/internal/common"

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
	Moderate   bool   `json:"moderate,omitempty" dynamodbav:"moderate,omitempty"`
}

type snsService interface {
	Publish(context.Context, *sns.PublishInput, ...func(*sns.Options)) (*sns.PublishOutput, error)
}

func SaveComment(ctx context.Context, dynamoService dynamoService, snsClient snsService, data common.CommentEntryData) error {
	common.LogWithCtx(ctx, "Received comment submission: Author=%q, Email=%q, Page=%q, Origin=%q, Referer=%q, ClientIP=%q, UserAgent=%q, HoneypotLen=%d",
		data.Name, data.Email, data.Page, data.PostOrigin, data.Referrer, data.ClientIP, data.UserAgent, len(data.Honeypot))

	common.LogWithCtx(ctx, "validating form")
	if err := validateComment(data); err != nil {
		common.LogWithCtx(ctx, "form validation failed: %v", err)
		return fmt.Errorf("invalid comment data: %v", err)
	}

	// Flag for moderation if honeypot is filled OR if Akismet flags it as spam
	var moderate bool
	if data.Honeypot != "" {
		common.LogWithCtx(ctx, "Honeypot field was filled (%q). Flagging comment by %q for moderation.", data.Honeypot, data.Name)
		moderate = true
	} else {
		isSpam, err := CheckAkismet(ctx, data)
		if err != nil {
			common.LogWithCtx(ctx, "Akismet check failed for comment by %q: %v. Continuing without flagging.", data.Name, err)
		} else if isSpam {
			common.LogWithCtx(ctx, "Akismet flagged comment by %q as spam. Flagging for moderation.", data.Name)
			moderate = true
		} else {
			common.LogWithCtx(ctx, "Akismet verified comment by %q as ham (not spam).", data.Name)
		}
	}

	account, err := getUser(ctx, dynamoService, data.Name)
	if err != nil {
		common.LogWithCtx(ctx, "error fetching user account for %q: %v", data.Name, err)
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
					common.LogWithCtx(ctx, "Migrating account for user %s to Google ID", data.Name)
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
			common.LogWithCtx(ctx, "saving new Google-verified user to dynamodb")
			if err := putUser(ctx, dynamoService, UserAccount{
				Author:   data.Name,
				UserID:   userID,
				AuthorID: accountID,
			}); err != nil {
				common.LogWithCtx(ctx, "error saving user account: %v", err)
			}
		}
		data.Verified = true
	} else if data.MastodonToken != "" {
		jwtSecret := common.GetEnvVar("JWT_SECRET", "")
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
			common.LogWithCtx(ctx, "saving new Mastodon-verified user to dynamodb")
			if err := putUser(ctx, dynamoService, UserAccount{
				Author:     data.Name,
				UserID:     userID,
				AuthorID:   accountID,
				ProfileURL: verifiedIdentity,
			}); err != nil {
				common.LogWithCtx(ctx, "error saving user account: %v", err)
			}
		}
		data.Verified = true
		data.ProfileURL = verifiedIdentity
	} else if data.IndieAuthToken != "" {
		jwtSecret := common.GetEnvVar("JWT_SECRET", "")
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
			common.LogWithCtx(ctx, "saving new IndieAuth-verified user to dynamodb")
			if err := putUser(ctx, dynamoService, UserAccount{
				Author:     data.Name,
				UserID:     userID,
				AuthorID:   accountID,
				ProfileURL: verifiedIdentity,
			}); err != nil {
				common.LogWithCtx(ctx, "error saving user account: %v", err)
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
			common.LogWithCtx(ctx, "saving legacy user to dynamodb")
			if err := putUser(ctx, dynamoService, UserAccount{
				Author:   data.Name,
				UserID:   userID,
				AuthorID: accountID,
			}); err != nil {
				common.LogWithCtx(ctx, "error saving user account: %v", err)
			}
		}
		data.Verified = false
	}

	data.UserID = userID

	common.LogWithCtx(ctx, "saving comment to dynamodb: Author=%q, Moderate=%t", data.Name, moderate)
	if err := putItem(ctx, dynamoService, data, moderate); err != nil {
		return fmt.Errorf("error saving comment: %v", err)
	}

	// Skip sending notifications if the comment is flagged for moderation
	if !moderate {
		if err := sendCommentNotification(ctx, snsClient, data); err != nil {
			common.LogWithCtx(ctx, "error sending notification: %v", err)
		}
	} else {
		common.LogWithCtx(ctx, "Skipped SNS notification for moderated comment by %q", data.Name)
	}

	return nil
}

// putItem saves a comment to a dynamo table
func putItem(ctx context.Context, svc dynamoService, data common.CommentEntryData, moderate bool) error {
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
		Moderate:   moderate,
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
