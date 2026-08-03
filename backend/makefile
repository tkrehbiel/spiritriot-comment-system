include env.makefile

clean:
	@if [ -f "function.zip" ]; then rm "function.zip"; echo "Zip removed."; else echo "Zip does not exist."; fi

build-fetch:
	GOOS=linux GOARCH=amd64 go build -o bootstrap -tags lambda.norpc ./cmd/fetch-service

deploy-fetch: clean build-fetch
	zip -r function.zip bootstrap
	AWS_PAGER="" aws lambda update-function-code \
		--function-name ${READ_COMMENTS_LAMBDA} \
		--zip-file fileb://function.zip
	rm bootstrap
	rm function.zip

build-page:
	GOOS=linux GOARCH=amd64 go build -o bootstrap -tags lambda.norpc ./cmd/page-service

deploy-page: clean build-page
	zip -r function.zip bootstrap
	AWS_PAGER="" aws lambda update-function-code \
		--function-name ${COMMENT_PAGE_LAMBDA} \
		--zip-file fileb://function.zip
	rm bootstrap
	rm function.zip

build-submit:
	GOOS=linux GOARCH=amd64 go build -o bootstrap -tags lambda.norpc ./cmd/submit-service

deploy-submit: clean build-submit
	zip -r function.zip bootstrap
	AWS_PAGER="" aws lambda update-function-code \
		--function-name ${POST_COMMENTS_LAMBDA} \
		--zip-file fileb://function.zip
	rm bootstrap
	rm function.zip

test:
	go test ./...

build: build-fetch build-page build-submit minify-css minify-js

deploy: deploy-fetch deploy-page deploy-submit deploy-s3-assets
