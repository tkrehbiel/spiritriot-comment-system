package main

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"endgameviable-comment-services/internal/common"
	"endgameviable-comment-services/internal/writeComments"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

func getHost(request events.APIGatewayProxyRequest) string {
	if h, ok := request.Headers["Host"]; ok {
		return h
	}
	if h, ok := request.Headers["host"]; ok {
		return h
	}
	return "api.endgameviable.com"
}

func handleMastodonPrompt(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	headers := common.GetCORSHeaders("GET, OPTIONS")
	headers["Content-Type"] = "text/html"
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body: `<!DOCTYPE html>
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
</html>`,
	}, nil
}

func handleMastodonInit(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	form, err := url.ParseQuery(request.Body)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "invalid request body"}, nil
	}
	instanceRaw := form.Get("instance")
	host := writeComments.ParseInstanceHost(instanceRaw)
	if host == "" {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "invalid instance"}, nil
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		log.Printf("aws load config failed: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Internal server error"}, nil
	}

	dynamoClient := dynamodb.NewFromConfig(cfg)
	client, err := writeComments.GetMastodonClient(ctx, dynamoClient, host)
	if err != nil {
		log.Printf("GetMastodonClient failed: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Internal server error"}, nil
	}

	apiOrigin := "https://" + getHost(request)
	redirectURI := apiOrigin + "/auth/mastodon/callback"

	if client.ClientID == "" {
		clientID, clientSecret, err := writeComments.RegisterMastodonApp(ctx, host, redirectURI)
		if err != nil {
			log.Printf("RegisterMastodonApp failed for host %s: %v", host, err)
			return events.APIGatewayProxyResponse{StatusCode: 400, Body: fmt.Sprintf("failed to register app on %s: %v", host, err)}, nil
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

	headers := common.GetCORSHeaders("POST, OPTIONS")
	headers["Location"] = authURL
	return events.APIGatewayProxyResponse{
		StatusCode: 302,
		Headers:    headers,
	}, nil
}

func handleMastodonCallback(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	code := request.QueryStringParameters["code"]
	host := request.QueryStringParameters["state"]

	if code == "" || host == "" {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "missing code or state parameters"}, nil
	}

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		log.Printf("aws load config failed: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "Internal server error"}, nil
	}

	dynamoClient := dynamodb.NewFromConfig(cfg)
	client, err := writeComments.GetMastodonClient(ctx, dynamoClient, host)
	if err != nil || client.ClientID == "" {
		log.Printf("GetMastodonClient callback failed: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "oauth client details not found"}, nil
	}

	apiOrigin := "https://" + getHost(request)
	redirectURI := apiOrigin + "/auth/mastodon/callback"

	accessToken, err := writeComments.GetMastodonAccessToken(ctx, host, client.ClientID, client.ClientSecret, code, redirectURI)
	if err != nil {
		log.Printf("GetMastodonAccessToken failed: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "failed to exchange oauth code"}, nil
	}

	username, _, profileURL, err := writeComments.VerifyMastodonCredentials(ctx, host, accessToken)
	if err != nil {
		log.Printf("VerifyMastodonCredentials failed: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "failed to verify credentials"}, nil
	}

	jwtSecret := common.GetEnvVar("JWT_SECRET", "local-development-secret-key-12345")
	token, err := writeComments.SignMastodonToken(profileURL, jwtSecret)
	if err != nil {
		log.Printf("SignMastodonToken failed: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "signing failed"}, nil
	}

	finalName := fmt.Sprintf("@%s@%s", username, strings.ToLower(host))

	headers := common.GetCORSHeaders("GET, OPTIONS")
	headers["Content-Type"] = "text/html"
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body: fmt.Sprintf(`<!DOCTYPE html>
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
</html>`, token, profileURL, finalName),
	}, nil
}

func handleIndieAuthPrompt(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	headers := common.GetCORSHeaders("GET, OPTIONS")
	headers["Content-Type"] = "text/html"
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body: `<!DOCTYPE html>
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
</html>`,
	}, nil
}

func handleIndieAuthInit(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	form, err := url.ParseQuery(request.Body)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "invalid request body"}, nil
	}
	me := form.Get("me")
	if me == "" {
		me = request.QueryStringParameters["me"]
	}
	if me == "" {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "missing me parameter"}, nil
	}

	authEndpoint, err := writeComments.DiscoverEndpoints(ctx, me)
	if err != nil {
		log.Printf("indieauth discovery failed: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: fmt.Sprintf("discovery failed: %v (Make sure you entered a valid profile URL starting with http:// or https://)", err)}, nil
	}

	state := "random-state-123"
	apiOrigin := "https://" + getHost(request)
	redirectURI := apiOrigin + "/auth/indieauth/callback"
	clientID := apiOrigin

	authURL := fmt.Sprintf("%s?response_type=code&client_id=%s&redirect_uri=%s&state=%s&me=%s",
		authEndpoint,
		url.QueryEscape(clientID),
		url.QueryEscape(redirectURI),
		url.QueryEscape(state),
		url.QueryEscape(me),
	)

	headers := common.GetCORSHeaders("POST, OPTIONS")
	headers["Location"] = authURL
	return events.APIGatewayProxyResponse{
		StatusCode: 302,
		Headers:    headers,
	}, nil
}

func handleIndieAuthCallback(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	code := request.QueryStringParameters["code"]
	me := request.QueryStringParameters["me"]

	if code == "" || me == "" {
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: "missing code or me parameters"}, nil
	}

	authEndpoint, err := writeComments.DiscoverEndpoints(ctx, me)
	if err != nil {
		log.Printf("indieauth callback discovery failed: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: fmt.Sprintf("discovery failed: %v", err)}, nil
	}

	apiOrigin := "https://" + getHost(request)
	redirectURI := apiOrigin + "/auth/indieauth/callback"
	clientID := apiOrigin

	verifiedMe, err := writeComments.VerifyIndieAuthCode(ctx, authEndpoint, code, me, clientID, redirectURI)
	if err != nil {
		log.Printf("code verification failed: %v", err)
		return events.APIGatewayProxyResponse{StatusCode: 400, Body: fmt.Sprintf("verification failed: %v", err)}, nil
	}

	jwtSecret := common.GetEnvVar("JWT_SECRET", "local-development-secret-key-12345")
	token, err := writeComments.SignIndieAuthToken(verifiedMe, jwtSecret)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: "signing failed"}, nil
	}

	displayName := verifiedMe

	headers := common.GetCORSHeaders("GET, OPTIONS")
	headers["Content-Type"] = "text/html"
	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers:    headers,
		Body: fmt.Sprintf(`<!DOCTYPE html>
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
</html>`, token, verifiedMe, displayName),
	}, nil
}
