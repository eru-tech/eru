# Eru Events - Kafka Development Makefile

.PHONY: help kafka-up kafka-down kafka-logs kafka-clean test test-verbose build clean

# Default target
help:
	@echo "🚀 Eru Events - Kafka Development Commands"
	@echo "=========================================="
	@echo ""
	@echo "Kafka Management:"
	@echo "  kafka-up       Start Kafka services (Zookeeper, Kafka, Kafka UI)"
	@echo "  kafka-down     Stop Kafka services"
	@echo "  kafka-logs     View Kafka service logs"
	@echo "  kafka-clean    Stop services and remove volumes"
	@echo ""
	@echo "Testing:"
	@echo "  test           Run unit tests"
	@echo "  test-verbose   Run unit tests with verbose output"
	@echo "  test-kafka     Run Kafka integration test"
	@echo ""
	@echo "Building:"
	@echo "  build          Build the Go modules"
	@echo "  clean          Clean build artifacts"
	@echo ""
	@echo "URLs:"
	@echo "  Kafka UI: http://localhost:8080"
	@echo "  Kafka Broker: localhost:9092"

# Detect Docker Compose command
DOCKER_COMPOSE := $(shell command -v docker-compose 2> /dev/null)
ifndef DOCKER_COMPOSE
	DOCKER_COMPOSE := docker compose
endif

# Kafka Management
kafka-up:
	@echo "🚀 Starting Kafka services..."
	$(DOCKER_COMPOSE) up -d
	@echo "✅ Services started!"
	@echo "📊 Kafka UI: http://localhost:8080"

kafka-down:
	@echo "🛑 Stopping Kafka services..."
	$(DOCKER_COMPOSE) down
	@echo "✅ Services stopped!"

kafka-logs:
	@echo "📋 Viewing Kafka logs (Ctrl+C to exit)..."
	$(DOCKER_COMPOSE) logs -f kafka

kafka-clean:
	@echo "🧹 Cleaning up Kafka services and volumes..."
	$(DOCKER_COMPOSE) down -v
	docker system prune -f
	@echo "✅ Cleanup complete!"

# Testing
test:
	@echo "🧪 Running unit tests..."
	go test ./events

test-verbose:
	@echo "🧪 Running unit tests (verbose)..."
	go test ./events -v

test-kafka:
	@echo "🧪 Running Kafka integration test..."
	go run test-kafka.go

# Building
build:
	@echo "🔨 Building Go modules..."
	go mod tidy
	go build ./events
	go build test-kafka.go
	@echo "✅ Build complete!"

clean:
	@echo "🧹 Cleaning build artifacts..."
	rm -f test-kafka
	go clean
	@echo "✅ Clean complete!"

# Development workflow
dev: kafka-up build test-kafka

# Full test suite
test-all: kafka-up test test-kafka

# Production build
prod-build:
	@echo "🚀 Production build..."
	CGO_ENABLED=1 go build -ldflags="-s -w" ./events
	@echo "✅ Production build complete!"