# Makefile for build-agent-smith
.PHONY: test lint vet fmt build run smoke

test:
	go test -race -count=1 -coverprofile=coverage.out ./...

vet:
	go vet ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .

build:
	go build -o bin/agent ./cmd/agent

run: build
	./bin/agent -manifest configs/repos.json

smoke:
	./scripts/test-audit-trigger.sh
