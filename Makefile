# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=ip-allocator-api
BINARY_PATH=./cmd/api

# Build the application
build:
	$(GOBUILD) -o $(BINARY_NAME) -v $(BINARY_PATH)

# Run the application
run:
	$(GOBUILD) -o $(BINARY_NAME) -v $(BINARY_PATH)
	./$(BINARY_NAME)

# Clean build files
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)

# Run tests
test:
	$(GOTEST) -v ./...

# Download dependencies
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# Run with hot reload (requires air)
dev:
	air

# Install air for hot reload
install-air:
	$(GOGET) -u github.com/cosmtrek/air

# Docker build
docker-build:
	docker build -t $(BINARY_NAME) .

# Docker run
docker-run:
	docker-compose up -d

# Docker stop
docker-stop:
	docker-compose down

# Docker clean
docker-clean:
	docker-compose down -v
	docker rmi $(BINARY_NAME) 2>/dev/null || true

# Format code
fmt:
	$(GOCMD) fmt ./...

# Lint code (requires golangci-lint)
lint:
	golangci-lint run

# Security check (requires gosec)
security:
	gosec ./...

# Generate API documentation
docs:
	swag init -g cmd/api/main.go

# Initialize project (first time setup)
init:
	$(GOMOD) tidy
	mkdir -p logs
	mkdir -p scripts

.PHONY: build run clean test deps dev install-air docker-build docker-run docker-stop docker-clean fmt lint security docs init
