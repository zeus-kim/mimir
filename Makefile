# mimir-mcp Makefile

BINARY_NAME=mimir-mcp
VERSION?=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS=-ldflags "-X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME) -X main.Commit=$(COMMIT)"

GO=go
GOFLAGS=-trimpath
CGO_ENABLED=1

.PHONY: all build clean test test-coverage lint fmt vet install run dev docker help

all: clean lint test build

## Build
build: ## Build the binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/mimir-mcp

build-linux: ## Build for Linux
	@echo "Building for Linux..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 $(GO) build $(GOFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/mimir-mcp

build-darwin: ## Build for macOS
	@echo "Building for macOS..."
	@mkdir -p bin
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/mimir-mcp
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=$(CGO_ENABLED) $(GO) build $(GOFLAGS) $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/mimir-mcp

build-all: build-linux build-darwin ## Build for all platforms

## Development
run: build ## Build and run
	./bin/$(BINARY_NAME)

dev: ## Run with hot reload (requires air)
	@which air > /dev/null || (echo "Installing air..." && go install github.com/air-verse/air@latest)
	air

install: build ## Install to GOPATH/bin
	@cp bin/$(BINARY_NAME) $(GOPATH)/bin/

## Testing
test: ## Run tests
	@echo "Running tests..."
	$(GO) test -race -v ./...

test-short: ## Run tests (short mode)
	$(GO) test -short -v ./...

test-coverage: ## Run tests with coverage
	@echo "Running tests with coverage..."
	@mkdir -p coverage
	$(GO) test -race -coverprofile=coverage/coverage.out -covermode=atomic ./...
	$(GO) tool cover -html=coverage/coverage.out -o coverage/coverage.html
	@echo "Coverage report: coverage/coverage.html"

test-integration: ## Run integration tests
	$(GO) test -race -v -tags=integration ./...

bench: ## Run benchmarks
	$(GO) test -bench=. -benchmem ./...

## Code Quality
lint: ## Run linter
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run ./...

fmt: ## Format code
	$(GO) fmt ./...
	@which goimports > /dev/null || go install golang.org/x/tools/cmd/goimports@latest
	goimports -w .

vet: ## Run go vet
	$(GO) vet ./...

## Dependencies
deps: ## Download dependencies
	$(GO) mod download

deps-update: ## Update dependencies
	$(GO) get -u ./...
	$(GO) mod tidy

deps-verify: ## Verify dependencies
	$(GO) mod verify

## Docker
docker: ## Build Docker image
	docker build -t $(BINARY_NAME):$(VERSION) .
	docker tag $(BINARY_NAME):$(VERSION) $(BINARY_NAME):latest

docker-run: docker ## Run Docker container
	docker run -p 8080:8080 -v ~/.mimir:/data $(BINARY_NAME):latest

docker-push: docker ## Push Docker image
	docker push $(BINARY_NAME):$(VERSION)
	docker push $(BINARY_NAME):latest

## Cleanup
clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -rf bin/
	@rm -rf coverage/
	@rm -rf dist/
	$(GO) clean -cache

## Release
release: clean test build-all ## Build release artifacts
	@mkdir -p dist
	@cp bin/$(BINARY_NAME)-* dist/
	@cd dist && for f in $(BINARY_NAME)-*; do tar -czf $$f.tar.gz $$f && rm $$f; done
	@echo "Release artifacts in dist/"

## Utilities
version: ## Show version
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build Time: $(BUILD_TIME)"

check: fmt vet lint test ## Run all checks

## Help
help: ## Show this help
	@echo "mimir-mcp - Vertical Search Engine MCP Server"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'
