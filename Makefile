.PHONY: test lint tidy build clean

.DEFAULT_GOAL := test

test:
	go test --count=1  -bench=. -v ./...

lint:
	@golangci-lint run --fix --verbose

tidy:
	go mod tidy

build:
	go build ./...

clean:
	go clean