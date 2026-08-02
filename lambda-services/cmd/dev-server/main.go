package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"endgameviable-comment-services/internal/common"
	"endgameviable-comment-services/internal/readComments"
	"endgameviable-comment-services/internal/writeComments"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

type CommentData struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	Honeypot       string `json:"website"`
	Comment        string `json:"comment"`
	Page           string `json:"page"`
	Origin         string `json:"origin"`
	Private        bool   `json:"private"`
	GoogleToken    string `json:"google_token,omitempty"`
	MigrateAccount bool   `json:"migrate_account,omitempty"`
	IndieAuthToken string `json:"indieauth_token,omitempty"`
	MastodonToken  string `json:"mastodon_token,omitempty"`
}

func main() {
	port := "8080"
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	http.HandleFunc("/comment", handleComment)
	http.HandleFunc("/comments", handleComments)
	http.HandleFunc("/auth/indieauth", handleIndieAuthInit)
	http.HandleFunc("/auth/indieauth/prompt", handleIndieAuthPrompt)
	http.HandleFunc("/auth/indieauth/init", handleIndieAuthInit)
	http.HandleFunc("/auth/indieauth/callback", handleIndieAuthCallback)
	http.HandleFunc("/auth/mastodon/prompt", handleMastodonPrompt)
	http.HandleFunc("/auth/mastodon/init", handleMastodonInit)
	http.HandleFunc("/auth/mastodon/callback", handleMastodonCallback)

	log.Printf("Starting SpiritRiot Local Dev Server on port %s...", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func setupCORS(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Amz-Date, Authorization, X-Api-Key, X-Amz-Security-Token")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return true
	}
	return false
}

func handleComment(w http.ResponseWriter, r *http.Request) {
	if setupCORS(w, r) {
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading request body: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	var form CommentData
	err = json.Unmarshal(body, &form)
	if err != nil {
		log.Printf("Error unmarshaling JSON: %v", err)
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.RemoteAddr
	}

	data := common.CommentEntryData{
		Name:           form.Name,
		Email:          form.Email,
		Honeypot:       form.Honeypot,
		Comment:        form.Comment,
		Page:           form.Page,
		PostOrigin:     form.Origin,
		UserAgent:      r.UserAgent(),
		ClientIP:       clientIP,
		Referrer:       r.Referer(),
		Private:        form.Private,
		GoogleToken:    form.GoogleToken,
		MigrateAccount: form.MigrateAccount,
		IndieAuthToken: form.IndieAuthToken,
		MastodonToken:  form.MastodonToken,
	}

	// Validate referrer against the allowed referrers list
	allowedReferrers := os.Getenv("HTTP_ALLOWED_REFERRERS")
	if !common.ValidateReferrer(data.Referrer, allowedReferrers) {
		log.Printf("Referrer not allowed: %s (Allowed: %s)", data.Referrer, allowedReferrers)
		http.Error(w, "comment rejected", http.StatusForbidden)
		return
	}

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		log.Printf("Error loading AWS config: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	dynamoClient := dynamodb.NewFromConfig(cfg)
	snsClient := sns.NewFromConfig(cfg)

	err = writeComments.SaveComment(ctx, dynamoClient, snsClient, data)
	if err != nil {
		log.Printf("Comment rejected by SaveComment: %v", err)
		if err.Error() == "linking_consent_required" {
			http.Error(w, "linking_consent_required", http.StatusConflict)
			return
		}
		http.Error(w, "comment rejected", http.StatusForbidden)
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("comment accepted"))
}

func handleComments(w http.ResponseWriter, r *http.Request) {
	if setupCORS(w, r) {
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	page := r.URL.Query().Get("page")
	if page == "" {
		http.Error(w, "invalid page", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		log.Printf("Error loading AWS config: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	svc := dynamodb.NewFromConfig(cfg)

	comments, err := readComments.Query(ctx, svc, page)
	if err != nil {
		log.Printf("Error querying comments: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	responseBody, err := json.Marshal(comments)
	if err != nil {
		log.Printf("Error marshalling comments: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(responseBody)
}

func handleIndieAuthInit(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)

	me := r.FormValue("me")
	if me == "" {
		me = r.URL.Query().Get("me")
	}
	if me == "" {
		http.Error(w, "missing me parameter", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	authEndpoint, err := writeComments.DiscoverEndpoints(ctx, me)
	if err != nil {
		log.Printf("indieauth discovery failed: %v", err)
		http.Error(w, fmt.Sprintf("discovery failed: %v (Make sure you entered a valid profile URL starting with http:// or https://)", err), http.StatusBadRequest)
		return
	}

	state := "random-state-123"
	apiOrigin := "http://" + r.Host
	if r.TLS != nil {
		apiOrigin = "https://" + r.Host
	}

	redirectURI := apiOrigin + "/auth/indieauth/callback"
	clientID := apiOrigin

	authURL := fmt.Sprintf("%s?response_type=code&client_id=%s&redirect_uri=%s&state=%s&me=%s",
		authEndpoint,
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(state),
		url.QueryEscape(me),
	)

	http.Redirect(w, r, authURL, http.StatusFound)
}

func handleIndieAuthCallback(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)

	code := r.URL.Query().Get("code")
	me := r.URL.Query().Get("me")

	if code == "" || me == "" {
		http.Error(w, "missing code or me parameters", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	authEndpoint, err := writeComments.DiscoverEndpoints(ctx, me)
	if err != nil {
		log.Printf("indieauth callback discovery failed: %v", err)
		http.Error(w, fmt.Sprintf("discovery failed: %v", err), http.StatusBadRequest)
		return
	}

	apiOrigin := "http://" + r.Host
	if r.TLS != nil {
		apiOrigin = "https://" + r.Host
	}

	redirectURI := apiOrigin + "/auth/indieauth/callback"
	clientID := apiOrigin

	verifiedMe, err := writeComments.VerifyIndieAuthCode(ctx, authEndpoint, code, me, clientID, redirectURI)
	if err != nil {
		log.Printf("code verification failed: %v", err)
		http.Error(w, fmt.Sprintf("verification failed: %v", err), http.StatusBadRequest)
		return
	}

	jwtSecret := common.GetEnvVar("JWT_SECRET", "local-development-secret-key-12345")
	token, err := writeComments.SignIndieAuthToken(verifiedMe, jwtSecret)
	if err != nil {
		http.Error(w, "signing failed", http.StatusInternalServerError)
		return
	}

	displayName := verifiedMe

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Authentication Successful</title></head>
<body>
    <p>Authentication successful! Closing window...</p>
    <script>
        if (window.opener) {
            window.opener.postMessage({
                type: "spiritriot-auth-success",
                provider: "indieauth",
                token: "%s",
                identity: "%s",
                name: "%s"
            }, "*");
        }
        window.close();
    </script>
</body>
</html>`, token, verifiedMe, displayName)
}



func handleMastodonPrompt(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Sign in with Mastodon</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background-color: #f3f4f6;
            color: #1f2937;
            margin: 0;
            padding: 2rem;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            box-sizing: border-box;
        }
        @media (prefers-color-scheme: dark) {
            body {
                background-color: #111827;
                color: #f9fafb;
            }
        }
        .card {
            background: white;
            border-radius: 12px;
            padding: 2rem;
            width: 100%;
            max-width: 400px;
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
            box-sizing: border-box;
        }
        @media (prefers-color-scheme: dark) {
            .card {
                background: #1f2937;
            }
        }
        h1 {
            font-size: 1.25rem;
            margin-top: 0;
            margin-bottom: 1.5rem;
            text-align: center;
        }
        label {
            display: block;
            font-size: 0.875rem;
            font-weight: 600;
            margin-bottom: 0.5rem;
        }
        input[type="text"] {
            width: 100%;
            padding: 0.75rem;
            border: 1px solid #d1d5db;
            border-radius: 8px;
            box-sizing: border-box;
            font-size: 0.95rem;
            margin-bottom: 1.25rem;
            background: white;
            color: #1f2937;
        }
        @media (prefers-color-scheme: dark) {
            input[type="text"] {
                border-color: #4b5563;
                background: #111827;
                color: #f9fafb;
            }
        }
        input[type="text"]:focus {
            outline: none;
            border-color: #6366f1;
            box-shadow: 0 0 0 3px rgba(99, 102, 241, 0.15);
        }
        button {
            width: 100%;
            padding: 0.75rem;
            background-color: #6366f1;
            color: white;
            border: none;
            border-radius: 8px;
            font-size: 0.95rem;
            font-weight: 600;
            cursor: pointer;
            transition: background-color 0.2s;
        }
        button:hover {
            background-color: #4f46e5;
        }
    </style>
</head>
<body>
    <div class="card">
        <h1>Sign in with Mastodon</h1>
        <form action="/auth/mastodon/init" method="POST">
            <label for="instance">Mastodon Handle or Instance:</label>
            <input type="text" id="instance" name="instance" placeholder="e.g. @user@mastodon.social or mastodon.social" required autofocus>
            <button type="submit">Proceed to Instance</button>
        </form>
    </div>
</body>
</html>`))
}

func handleMastodonInit(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	instanceRaw := r.FormValue("instance")
	host := writeComments.ParseInstanceHost(instanceRaw)
	if host == "" {
		http.Error(w, "invalid instance", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		log.Printf("aws load config failed: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	dynamoClient := dynamodb.NewFromConfig(cfg)
	client, err := writeComments.GetMastodonClient(ctx, dynamoClient, host)
	if err != nil {
		log.Printf("GetMastodonClient failed: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	apiOrigin := "http://" + r.Host
	if r.TLS != nil {
		apiOrigin = "https://" + r.Host
	}
	redirectURI := apiOrigin + "/auth/mastodon/callback"

	if client.ClientID == "" {
		clientID, clientSecret, err := writeComments.RegisterMastodonApp(ctx, host, redirectURI)
		if err != nil {
			log.Printf("RegisterMastodonApp failed for host %s: %v", host, err)
			http.Error(w, fmt.Sprintf("failed to register app on %s: %v", host, err), http.StatusBadRequest)
			return
		}
		client = writeComments.MastodonClient{
			InstanceHost: host,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			CreatedAt:    time.Now().Format(time.RFC3339),
		}
		err = writeComments.PutMastodonClient(ctx, dynamoClient, client)
		if err != nil {
			log.Printf("PutMastodonClient failed: %v", err)
		}
	}

	authURL := fmt.Sprintf("https://%s/oauth/authorize?client_id=%s&response_type=code&redirect_uri=%s&scope=read:accounts&state=%s",
		host,
		url.QueryEscape(client.ClientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(host),
	)

	http.Redirect(w, r, authURL, http.StatusFound)
}

func handleMastodonCallback(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)

	code := r.URL.Query().Get("code")
	host := r.URL.Query().Get("state")

	if code == "" || host == "" {
		http.Error(w, "missing code or state parameters", http.StatusBadRequest)
		return
	}

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		log.Printf("aws load config failed: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	dynamoClient := dynamodb.NewFromConfig(cfg)
	client, err := writeComments.GetMastodonClient(ctx, dynamoClient, host)
	if err != nil || client.ClientID == "" {
		log.Printf("GetMastodonClient callback failed: %v", err)
		http.Error(w, "oauth client details not found", http.StatusBadRequest)
		return
	}

	apiOrigin := "http://" + r.Host
	if r.TLS != nil {
		apiOrigin = "https://" + r.Host
	}
	redirectURI := apiOrigin + "/auth/mastodon/callback"

	accessToken, err := writeComments.GetMastodonAccessToken(ctx, host, client.ClientID, client.ClientSecret, code, redirectURI)
	if err != nil {
		log.Printf("GetMastodonAccessToken failed: %v", err)
		http.Error(w, "failed to exchange oauth code", http.StatusBadRequest)
		return
	}

	username, _, profileURL, err := writeComments.VerifyMastodonCredentials(ctx, host, accessToken)
	if err != nil {
		log.Printf("VerifyMastodonCredentials failed: %v", err)
		http.Error(w, "failed to verify credentials", http.StatusBadRequest)
		return
	}

	jwtSecret := common.GetEnvVar("JWT_SECRET", "local-development-secret-key-12345")
	token, err := writeComments.SignMastodonToken(profileURL, jwtSecret)
	if err != nil {
		log.Printf("SignMastodonToken failed: %v", err)
		http.Error(w, "signing failed", http.StatusInternalServerError)
		return
	}

	finalName := fmt.Sprintf("@%s@%s", username, strings.ToLower(host))

	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html>
<head><title>Authentication Successful</title></head>
<body>
    <p>Authentication successful! Closing window...</p>
    <script>
        if (window.opener) {
            window.opener.postMessage({
                type: "spiritriot-auth-success",
                provider: "mastodon",
                token: "%s",
                identity: "%s",
                name: "%s"
            }, "*");
        }
        window.close();
    </script>
</body>
</html>`, token, profileURL, finalName)
}

func handleIndieAuthPrompt(w http.ResponseWriter, r *http.Request) {
	setupCORS(w, r)
	w.Header().Set("Content-Type", "text/html")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Sign in with IndieAuth</title>
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <style>
        body {
            font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
            background-color: #f3f4f6;
            color: #1f2937;
            margin: 0;
            padding: 2rem;
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            box-sizing: border-box;
        }
        @media (prefers-color-scheme: dark) {
            body {
                background-color: #111827;
                color: #f9fafb;
            }
        }
        .card {
            background: white;
            border-radius: 12px;
            padding: 2rem;
            width: 100%;
            max-width: 400px;
            box-shadow: 0 4px 6px -1px rgba(0, 0, 0, 0.1), 0 2px 4px -1px rgba(0, 0, 0, 0.06);
            box-sizing: border-box;
        }
        @media (prefers-color-scheme: dark) {
            .card {
                background: #1f2937;
            }
        }
        h1 {
            font-size: 1.25rem;
            margin-top: 0;
            margin-bottom: 1.5rem;
            text-align: center;
        }
        label {
            display: block;
            font-size: 0.875rem;
            font-weight: 600;
            margin-bottom: 0.5rem;
        }
        input[type="text"] {
            width: 100%;
            padding: 0.75rem;
            border: 1px solid #d1d5db;
            border-radius: 8px;
            box-sizing: border-box;
            font-size: 0.95rem;
            margin-bottom: 1.25rem;
            background: white;
            color: #1f2937;
        }
        @media (prefers-color-scheme: dark) {
            input[type="text"] {
                border-color: #4b5563;
                background: #111827;
                color: #f9fafb;
            }
        }
        input[type="text"]:focus {
            outline: none;
            border-color: #eab308;
            box-shadow: 0 0 0 3px rgba(234, 179, 8, 0.15);
        }
        button {
            width: 100%;
            padding: 0.75rem;
            background-color: #eab308;
            color: black;
            border: none;
            border-radius: 8px;
            font-size: 0.95rem;
            font-weight: 600;
            cursor: pointer;
            transition: background-color 0.2s;
        }
        button:hover {
            background-color: #ca8a04;
        }
    </style>
</head>
<body>
    <div class="card">
        <h1>Sign in with IndieAuth</h1>
        <form action="/auth/indieauth/init" method="POST">
            <label for="me">IndieAuth URL:</label>
            <input type="text" id="me" name="me" placeholder="e.g. https://example.com" required autofocus>
            <button type="submit">Proceed to Login</button>
        </form>
    </div>
</body>
</html>`))
}
