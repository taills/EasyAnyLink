.PHONY: all build test proto clean install-tools certs docker help

# Variables
BINARY_SERVER=bin/server
BINARY_AGENT=bin/agent
PROTO_DIR=common/proto
GO_FILES=$(shell find . -name '*.go' -type f -not -path "./vendor/*")
PROTO_FILES=$(shell find $(PROTO_DIR) -name '*.proto')

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
GOCLEAN=$(GOCMD) clean

# Build flags
LDFLAGS=-ldflags "-s -w"
BUILD_TIME=$(shell date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# Default target
all: proto build

## help: Display this help message
help:
	@echo "EasyAnyLink Build System"
	@echo "========================"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

## install-tools: Install required development tools
install-tools:
	@echo "Installing development tools..."
	@which protoc > /dev/null || (echo "Please install protoc from https://grpc.io/docs/protoc-installation/" && exit 1)
	$(GOGET) google.golang.org/protobuf/cmd/protoc-gen-go@latest
	$(GOGET) google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@echo "✓ Development tools installed"

## proto: Generate Go code from Protocol Buffer definitions
proto:
	@echo "Generating Protocol Buffer code..."
	@mkdir -p $(PROTO_DIR)
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		$(PROTO_FILES)
	@echo "✓ Protocol Buffer code generated"

## build: Build server and agent binaries
build: build-server build-agent

## build-server: Build server binary
build-server:
	@echo "Building server..."
	@mkdir -p bin
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_SERVER) \
		-ldflags "-X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)" \
		./cmd/server
	@echo "✓ Server built: $(BINARY_SERVER)"

## build-agent: Build agent binary
build-agent:
	@echo "Building agent..."
	@mkdir -p bin
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_AGENT) \
		-ldflags "-X main.Version=$(VERSION) -X main.GitCommit=$(GIT_COMMIT) -X main.BuildTime=$(BUILD_TIME)" \
		./cmd/agent
	@echo "✓ Agent built: $(BINARY_AGENT)"

## test: Run all tests
test:
	@echo "Running tests..."
	$(GOTEST) -v -race -coverprofile=coverage.out ./...
	@echo "✓ Tests completed"

## test-coverage: Run tests with coverage report
test-coverage: test
	@echo "Generating coverage report..."
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "✓ Coverage report: coverage.html"

## test-integration: Run integration tests (requires root)
test-integration:
	@echo "Running integration tests..."
	@echo "Note: This requires root privileges for TUN interface"
	sudo $(GOTEST) -tags=integration -v ./agent/... ./server/...

## certs: Generate development certificates
certs:
	@echo "Generating certificates..."
	@./scripts/generate_certs.sh

## run-server: Run server with example config
run-server: build-server
	@echo "Starting server..."
	@mkdir -p logs
	./$(BINARY_SERVER) -config config/server.example.json

## run-agent-client: Run agent in client mode (requires root)
run-agent-client: build-agent
	@echo "Starting agent in client mode..."
	@mkdir -p logs
	sudo ./$(BINARY_AGENT) -config config/agent-client.example.json

## run-agent-gateway: Run agent in gateway mode (requires root)
run-agent-gateway: build-agent
	@echo "Starting agent in gateway mode..."
	@mkdir -p logs
	sudo ./$(BINARY_AGENT) -config config/agent-gateway.example.json

## docker: Build Docker images
docker: docker-server docker-agent

## docker-server: Build server Docker image
docker-server:
	@echo "Building server Docker image..."
	docker build -t easyanylink/server:latest -f Dockerfile.server .
	@echo "✓ Server image built: easyanylink/server:latest"

## docker-agent: Build agent Docker image
docker-agent:
	@echo "Building agent Docker image..."
	docker build -t easyanylink/agent:latest -f Dockerfile.agent .
	@echo "✓ Agent image built: easyanylink/agent:latest"

## clean: Remove build artifacts
clean:
	@echo "Cleaning build artifacts..."
	$(GOCLEAN)
	rm -rf bin/
	rm -rf web/dist/
	rm -f coverage.out coverage.html
	rm -f $(PROTO_DIR)/*.pb.go
	@echo "✓ Clean completed"

## deps: Download and tidy dependencies
deps:
	@echo "Downloading dependencies..."
	$(GOMOD) download
	$(GOMOD) tidy
	@echo "✓ Dependencies updated"

## fmt: Format Go code
fmt:
	@echo "Formatting code..."
	@gofmt -s -w $(GO_FILES)
	@echo "✓ Code formatted"

## lint: Run linters
lint:
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "Please install golangci-lint" && exit 1)
	golangci-lint run ./...
	@echo "✓ Linting completed"

## vet: Run go vet
vet:
	@echo "Running go vet..."
	$(GOCMD) vet ./...
	@echo "✓ Vet completed"

## db-init: Initialize database schema
db-init:
	@echo "Initializing database..."
	@which mysql > /dev/null || (echo "Please install mysql client" && exit 1)
	mysql -u root -p < scripts/init_db.sql
	@echo "✓ Database initialized"

## cross-compile: Build for multiple platforms
cross-compile:
	@echo "Cross-compiling for multiple platforms..."
	@mkdir -p bin
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/server-linux-amd64 ./cmd/server
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/agent-linux-amd64 ./cmd/agent
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/server-darwin-amd64 ./cmd/server
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/agent-darwin-amd64 ./cmd/agent
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/server-darwin-arm64 ./cmd/server
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o bin/agent-darwin-arm64 ./cmd/agent
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o bin/agent-windows-amd64.exe ./cmd/agent
	@echo "✓ Cross-compilation completed"
