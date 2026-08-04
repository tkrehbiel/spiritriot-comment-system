clean:
	@rm -f bootstrap function.zip

build-fetch:
	GOOS=linux GOARCH=amd64 go build -o bootstrap -tags lambda.norpc ./cmd/fetch-service

build-page:
	GOOS=linux GOARCH=amd64 go build -o bootstrap -tags lambda.norpc ./cmd/page-service

build-submit:
	GOOS=linux GOARCH=amd64 go build -o bootstrap -tags lambda.norpc ./cmd/submit-service

test:
	go test ./...

build: build-fetch build-page build-submit
