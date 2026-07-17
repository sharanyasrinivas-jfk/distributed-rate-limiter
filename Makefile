.PHONY: run test build docker-build lint

run:          ## Run gateway locally
	go run ./cmd/gateway

test:         ## Run all tests with coverage and the race detector
	go test ./... -cover -race

build:        ## Build the binary
	go build -o bin/gateway ./cmd/gateway

docker-build: ## Build Docker image
	docker build -t rate-limiter-gateway:local -f deployments/Dockerfile .

lint:         ## Run linter
	golangci-lint run
