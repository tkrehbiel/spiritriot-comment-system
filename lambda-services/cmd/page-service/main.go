package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"

	"endgameviable-comment-services/internal/common"
	"endgameviable-comment-services/internal/readComments"
	"endgameviable-comment-services/internal/writeComments"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sns"
)

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
<form method="POST" action="#comment-form">
	<label for="name">Name:</label>
	<input type="text" id="comment-author" name="name" value="{{ .Name }}" required>

	<label for="email">Email:</label>
	<input type="text" id="comment-email" name="email" value="{{ .Email }}" required>

	<label for="comment">Comment (plain text please):</label>
	<textarea id="comment-content" name="comment" rows="4" required></textarea>

	<div style="display:none;">
		<input type="text" id="website" name="website" value="">
		<input type="text" id="page" name="page" value="{{ .Page }}">
		<input type="text" id="origin" name="origin" value="{{ .PostOrigin }}">
		<input type="text" id="title" name="title" value="{{ .PostTitle }}">
	</div>

	<input type="submit" value="Submit">
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

// CommentPageData is the data model for the page template
type CommentPageData struct {
	common.CommentEntryData
	Comments  []readComments.CommentItem
	Response  string
	PageTitle string
	CSS       string
}

func lambdaHandlerWeb(ctx context.Context, request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Println(request)

	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}

	dynamoClient := dynamodb.NewFromConfig(cfg)
	snsClient := sns.NewFromConfig(cfg)

	var data CommentPageData

	data.PageTitle = common.GetEnvVar("HTML_TITLE", "No-Javascript Comments")
	data.CSS = common.GetEnvVar("HTML_CSS", "")

	if request.Headers["Cookie"] != "" {
		headers := http.Header{}
		headers.Add("Cookie", request.Headers["Cookie"])
		mockRequest := http.Request{Header: headers}
		name, err := mockRequest.Cookie("name")
		if err == nil {
			data.Name = name.Value
		}
		email, err := mockRequest.Cookie("email")
		if err == nil {
			data.Email = email.Value
		}
	}

	if request.HTTPMethod == "GET" {
		log.Println("processing GET")
		data.PostTitle = request.QueryStringParameters["title"]
		data.PostOrigin = request.QueryStringParameters["origin"]
		url, err := url.Parse(data.PostOrigin)
		if err == nil {
			data.Page = url.Path
		}
	} else if request.HTTPMethod == "POST" {
		log.Println("processing POST")
		values, _ := url.ParseQuery(request.Body)
		data.PostTitle = values.Get("title")
		data.PostOrigin = values.Get("origin")
		data.Page = values.Get("page")
		data.Name = values.Get("name")
		data.Email = values.Get("email")
		data.Comment = values.Get("comment")
		data.Honeypot = values.Get("website")
		data.ClientIP = request.RequestContext.Identity.SourceIP
		data.UserAgent = request.RequestContext.Identity.UserAgent
		data.Referrer = request.Headers["referer"]
		if !common.ValidateReferrer(data.Referrer, common.GetEnvVar("HTTP_ALLOWED_REFERRERS", "")) {
			log.Printf("referrer missing or not allowed")
		}
		if err := writeComments.SaveComment(ctx, dynamoClient, snsClient, data.CommentEntryData); err != nil {
			log.Printf("error posting comment: %v", err)
			data.Response = err.Error()
		}
	}

	if data.PostTitle == "" {
		data.PostTitle = data.PostOrigin
	}

	log.Println(data)
	comments, err := readComments.Query(ctx, dynamoClient, data.Page)
	if err == nil {
		data.Comments = comments
	}

	t := template.Must(template.New("webpage").Parse(htmlTemplate))
	html := renderTemplate(t, data)

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
		Headers: map[string]string{
			"Content-Type": "text/html",
		},
		MultiValueHeaders: map[string][]string{
			"Set-Cookie": {
				fmt.Sprintf("name=%s; Path=/; Max-Age=%d; Secure; HttpOnly", data.Name, CookieAge),
				fmt.Sprintf("email=%s; Path=/; Max-Age=%d; Secure; HttpOnly", data.Email, CookieAge),
			},
		},
		Body: html,
	}, nil
}

func renderTemplate(t *template.Template, data CommentPageData) string {
	var renderedHTML string
	sb := &strings.Builder{}
	err := t.Execute(sb, data)
	if err != nil {
		renderedHTML = fmt.Sprintf("Error rendering template: %s", err)
	} else {
		renderedHTML = sb.String()
	}
	return renderedHTML
}

func main() {
	lambda.Start(lambdaHandlerWeb)
}
