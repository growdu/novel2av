.PHONY: build run-api run-cli test fmt vet

build:
	go build ./...

run-api:
	go run ./cmd/api

run-cli:
	go run ./cmd/cli --help

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...
