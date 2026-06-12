.PHONY: test build run

test:
	go test ./...

build:
	go build -o bin/agentdb233-server ./cmd/agentdb233-server

run:
	go run ./cmd/agentdb233-server serve
