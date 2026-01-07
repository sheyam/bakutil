# Makefile for backup utility

.PHONY: build clean test lint run dev help

# Build variables
BINARY_NAME=backup-util
VERSION?=dev
BUILD_TIME=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

LDFLAGS=-ldflags "-X 'main.version=$(VERSION)' -X 'main.buildTime=$(BUILD_TIME)' -X 'main.gitCommit=$(GIT_COMMIT)'"

## help: Show this help message
help:
	@echo "Available targets:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ':' | sed -e 's/^/ /'

## build: Build the application
build:
	@echo "Building $(BINARY_NAME)..."
	go build $(LDFLAGS) -o $(BINARY_NAME) main.go

## dev: Build for development (no optimizations)
dev:
	@echo "Building $(BINARY_NAME) for development..."
	go build -o $(BINARY_NAME) main.go

## test: Run all tests
test:
	@echo "Running tests..."
	go test -v ./...

## test-race: Run tests with race detector
test-race:
	@echo "Running tests with race detector..."
	go test -race -v ./...

## lint: Run linter
lint:
	@echo "Running linter..."
	golangci-lint run

## clean: Clean build artifacts
clean:
	@echo "Cleaning..."
	rm -f $(BINARY_NAME)
	rm -rf build/

## build-all: Build for all platforms
build-all: clean
	@echo "Building for all platforms..."
	@mkdir -p build
	
	# Linux
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o build/$(BINARY_NAME)-linux-amd64 main.go
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o build/$(BINARY_NAME)-linux-arm64 main.go
	
	# macOS
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o build/$(BINARY_NAME)-darwin-amd64 main.go
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o build/$(BINARY_NAME)-darwin-arm64 main.go
	
	# Windows
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o build/$(BINARY_NAME)-windows-amd64.exe main.go

## run: Build and run the application
run: build
	./$(BINARY_NAME)

## install: Install the application
install:
	go install $(LDFLAGS) .

## mod: Download and tidy modules
mod:
	go mod download
	go mod tidy