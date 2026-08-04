package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"spiritriot-comment-services/internal/common"
	"spiritriot-comment-services/internal/readComments"
	"spiritriot-comment-services/internal/writeComments"

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
	http.HandleFunc("/comment-page", handleCommentPage)
	http.HandleFunc("/css/comments.css", func(w http.ResponseWriter, r *http.Request) {
		content, err := os.ReadFile("../frontend/client-assets/css/comments.css")
		if err != nil {
			log.Printf("Error reading CSS file: %v", err)
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		w.Write(content)
	})
	http.HandleFunc("/config", handleConfig)
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

func handleConfig(w http.ResponseWriter, r *http.Request) {
	if setupCORS(w, r) {
		return
	}

	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	googleClientID := common.GetEnvVar("GOOGLE_CLIENT_ID", "")
	configData := map[string]string{
		"googleClientId": googleClientID,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(configData)
}

func handleComment(w http.ResponseWriter, r *http.Request) {
	log.Printf("Received submit-service request: Path=%s, Method=%s, ClientIP=%s", r.URL.Path, r.Method, r.RemoteAddr)

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

	jwtSecret := common.GetEnvVar("JWT_SECRET", "")
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

	jwtSecret := common.GetEnvVar("JWT_SECRET", "")
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

const htmlTemplate = `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<meta name="robots" content="noindex, nofollow">
<title>{{ .PageTitle }}</title>
{{ with .CSS }}<link rel="stylesheet" href="{{ . }}">{{ end }}
</head>
<body>

<main>
<div class="u-wrapper">
<div class="u-padding">

<h1>{{ .PageTitle }}</h1>
<h2>RE: <a href="{{ .PostOrigin }}">{{ .PostTitle }}</a></h2>

<p><i>This is a tiny serverless web page for viewing
and entering blog comments without Javascript.
It's on a separate page on a different domain
due to the fickle nature of technology
and the static nature of my blog. Basically,
it allows dynamic page generation.</i></p>

<div id="commentsection">

<div id="comments">
	{{ with .Comments }}
	<h3>Recent Comments</h3>
		{{ range . }}
		<p class="comment">
			<span class="author">{{ .Author }}</span>
			<span class="datetime">{{ .Date }}</span>
			{{ .Content }}
		</p>
		{{ end }}
	{{ else }}
	<p>No comments yet.</p>
	{{ end }}
</div>

{{ with .Response }}
<div id="comment-response"><p>{{ . }}</p></div>
{{ end }}

<div id="comment-form">
{{ with .CommentEntryData }}
<form method="POST" action="#comment-form" class="spiritriot-form">
	<div class="spiritriot-form-grid">
		<div class="spiritriot-field">
			<label for="comment-author">Name:</label>
			<input type="text" id="comment-author" name="name" class="spiritriot-input" value="{{ .Name }}" required>
		</div>
		<div class="spiritriot-field">
			<label for="comment-email">Email:</label>
			<input type="text" id="comment-email" name="email" class="spiritriot-input" value="{{ .Email }}" required>
		</div>
	</div>

	<div class="spiritriot-field">
		<label for="comment-content">Comment (plain text please):</label>
		<textarea id="comment-content" name="comment" class="spiritriot-textarea" rows="4" required></textarea>
	</div>

	<div style="display:none;">
		<input type="text" id="website" name="website" value="">
		<input type="text" id="page" name="page" value="{{ .Page }}">
		<input type="text" id="origin" name="origin" value="{{ .PostOrigin }}">
		<input type="text" id="title" name="title" value="{{ .PostTitle }}">
		{{ if .Private }}<input type="hidden" name="private" value="true">{{ end }}
	</div>

	<button type="submit" class="spiritriot-button-primary">Submit</button>
</form>
{{ end }}
</div>

</div>

</div>
</div>
</main>

</body>
</html>`

const CookieAge = 60 * 60 * 24 * 90 // 90 days

type CommentPageData struct {
	common.CommentEntryData
	Comments []readComments.CommentItem
	Response string
	PageTitle string
	CSS string
}

func handleCommentPage(w http.ResponseWriter, r *http.Request) {
	if setupCORS(w, r) {
		return
	}

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		log.Printf("Error loading AWS config: %v", err)
		http.Error(w, "AWS config error", http.StatusInternalServerError)
		return
	}
	dynamoClient := dynamodb.NewFromConfig(cfg)
	snsClient := sns.NewFromConfig(cfg)

	var data CommentPageData
	data.PageTitle = os.Getenv("HTML_TITLE")
	if data.PageTitle == "" {
		data.PageTitle = "Local Comments"
	}
	data.CSS = os.Getenv("HTML_CSS")
	if data.CSS == "" {
		data.CSS = "http://localhost:8080/css/comments.css"
	}

	if cookieName, err := r.Cookie("name"); err == nil {
		data.Name = cookieName.Value
	}
	if cookieEmail, err := r.Cookie("email"); err == nil {
		data.Email = cookieEmail.Value
	}

	if r.Method == "GET" {
		log.Println("processing GET in local page-service")
		data.PostTitle = r.URL.Query().Get("title")
		data.PostOrigin = r.URL.Query().Get("origin")
		data.Private = r.URL.Query().Get("private") == "true"
		urlParsed, err := url.Parse(data.PostOrigin)
		if err == nil {
			data.Page = urlParsed.Path
		}
	} else if r.Method == "POST" {
		log.Println("processing POST in local page-service")
		err := r.ParseForm()
		if err != nil {
			log.Printf("Error parsing form: %v", err)
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		data.PostTitle = r.FormValue("title")
		data.PostOrigin = r.FormValue("origin")
		data.Page = r.FormValue("page")
		data.Name = r.FormValue("name")
		data.Email = r.FormValue("email")
		data.Comment = r.FormValue("comment")
		data.Honeypot = r.FormValue("website")
		data.Private = r.FormValue("private") == "true"

		clientIP := r.Header.Get("X-Forwarded-For")
		if clientIP == "" {
			clientIP = r.RemoteAddr
		}
		data.ClientIP = clientIP
		data.UserAgent = r.UserAgent()
		data.Referrer = r.Referer()

		allowedReferrers := os.Getenv("HTTP_ALLOWED_REFERRERS")
		if !common.ValidateReferrer(data.Referrer, allowedReferrers) {
			log.Printf("Referrer not allowed: %s (Allowed: %s)", data.Referrer, allowedReferrers)
		}

		err = writeComments.SaveComment(ctx, dynamoClient, snsClient, data.CommentEntryData)
		if err != nil {
			log.Printf("error posting comment: %v", err)
			data.Response = err.Error()
		} else {
			data.Response = "Comment submitted successfully."
		}
	}

	if data.PostTitle == "" {
		data.PostTitle = data.PostOrigin
	}

	comments, err := readComments.Query(ctx, dynamoClient, data.Page)
	if err == nil {
		data.Comments = comments
	}

	http.SetCookie(w, &http.Cookie{
		Name: "name",
		Value: data.Name,
		Path: "/",
		MaxAge: CookieAge,
	})
	http.SetCookie(w, &http.Cookie{
		Name: "email",
		Value: data.Email,
		Path: "/",
		MaxAge: CookieAge,
	})

	t := template.Must(template.New("webpage").Parse(htmlTemplate))
	w.Header().Set("Content-Type", "text/html")

	sb := &strings.Builder{}
	err = t.Execute(sb, data)
	if err != nil {
		w.Write([]byte(fmt.Sprintf("Error rendering template: %s", err)))
	} else {
		w.Write([]byte(sb.String()))
	}
}

