.PHONY: build run test clean docker-build docker-up docker-down

# Build the bot
build:
	go build -o bin/bot ./cmd/bot

# Run the bot locally
run: build
	./bin/bot

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -rf bin/

# Build Docker image
docker-build:
	docker-compose build

# Start with Docker Compose
docker-up:
	docker-compose up -d

# Stop Docker Compose
docker-down:
	docker-compose down

# View logs
docker-logs:
	docker-compose logs -f

# Download dependencies
deps:
	go mod download
	go mod tidy

