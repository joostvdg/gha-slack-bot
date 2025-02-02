# Makefile for Go application

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=gha-slack-bot
BINARY_UNIX=$(BINARY_NAME)_unix

# All target
all: test build

format:
	gofmt -s -w -l main.go
	#gofmt -s -w -l **/*.*

# Build the project
build:
	$(GOBUILD) -o $(BINARY_NAME) -v

# Run the tests
test:
	$(GOTEST) -v ./...

# Clean the build files
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

# Install dependencies
deps:
	$(GOGET) -v ./...

# Cross compile for Linux
build-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -o $(BINARY_UNIX) -v

# Run the application
run:
	$(GOBUILD) -o $(BINARY_NAME) -v
	./$(BINARY_NAME)

.PHONY: all build clean test deps build-linux run

fly-logs:
	fly logs -a gha-slack-bot