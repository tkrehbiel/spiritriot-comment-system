package writeComments

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"spiritriot-comment-services/internal/common"
)

// CheckAkismet contacts the Akismet API to verify if the comment is spam.
// It returns true if the comment is spam, false if ham, and logs any errors.
func CheckAkismet(ctx context.Context, data common.CommentEntryData) (bool, error) {
	apiKey := os.Getenv("AKISMET_API_KEY")
	if apiKey == "" {
		common.LogWithCtx(ctx, "Akismet API key not configured, bypassing spam check")
		return false, nil
	}

	websiteURL := os.Getenv("WEBSITE_URL")
	if websiteURL == "" {
		websiteURL = "https://endgameviable.com"
	}

	// Akismet comment-check API endpoint (support test URL override)
	apiURL := os.Getenv("AKISMET_API_URL")
	if apiURL == "" {
		apiURL = "https://" + apiKey + ".rest.akismet.com/1.1/comment-check"
	}

	formValues := url.Values{}
	formValues.Set("blog", websiteURL)
	formValues.Set("user_ip", data.ClientIP)
	formValues.Set("user_agent", data.UserAgent)
	formValues.Set("referrer", data.Referrer)
	
	// Construct the permalink
	permalink := websiteURL + data.Page
	if strings.HasPrefix(data.Page, "http://") || strings.HasPrefix(data.Page, "https://") {
		permalink = data.Page
	}
	formValues.Set("permalink", permalink)
	
	formValues.Set("comment_type", "comment")
	formValues.Set("comment_author", data.Name)
	if data.Email != "" {
		formValues.Set("comment_author_email", data.Email)
	}
	formValues.Set("comment_content", data.Comment)

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(formValues.Encode()))
	if err != nil {
		return false, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "SpiritRiot Comment System/1.0 | Akismet Go Client/1.0")

	// 5-second timeout client
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}

	response := strings.TrimSpace(string(bodyBytes))
	common.LogWithCtx(ctx, "Akismet API check response: %s", response)

	if response == "true" {
		return true, nil
	}

	return false, nil
}
