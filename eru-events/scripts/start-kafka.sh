#!/bin/bash

# Eru Events - Kafka Local Development Setup Script
# This script helps you quickly start and test Kafka locally

set -e

echo "🚀 Eru Events - Kafka Local Development Setup"
echo "============================================="

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo "❌ Docker is not running. Please start Docker first."
    exit 1
fi

# Check if Docker Compose is available (try both old and new syntax)
DOCKER_COMPOSE_CMD=""
if command -v docker-compose &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker-compose"
elif docker compose version &> /dev/null; then
    DOCKER_COMPOSE_CMD="docker compose"
else
    echo "❌ Docker Compose is not available. Please install Docker Compose."
    echo "   Found docker at: $(which docker 2>/dev/null || echo 'not found')"
    echo "   Checked for docker-compose at: $(which docker-compose 2>/dev/null || echo 'not found')"
    exit 1
fi

echo "✅ Using Docker Compose command: $DOCKER_COMPOSE_CMD"

# Function to wait for service to be ready
wait_for_service() {
    local service_name=$1
    local max_attempts=30
    local attempt=1
    
    echo "⏳ Waiting for $service_name to be ready..."
    
    while [ $attempt -le $max_attempts ]; do
        if $DOCKER_COMPOSE_CMD ps | grep -q "$service_name.*Up"; then
            echo "✅ $service_name is ready!"
            return 0
        fi
        
        echo "   Attempt $attempt/$max_attempts - waiting for $service_name..."
        sleep 5
        ((attempt++))
    done
    
    echo "❌ $service_name failed to start within expected time"
    return 1
}

# Start services
echo "🐳 Starting Kafka services..."
$DOCKER_COMPOSE_CMD up -d

# Wait for services to be ready
wait_for_service "zookeeper"
wait_for_service "kafka"
wait_for_service "kafka-ui"

echo ""
echo "🎉 Kafka services are now running!"
echo ""
echo "📋 Service URLs:"
echo "   • Kafka Broker: localhost:9092"
echo "   • Kafka UI: http://localhost:8080"
echo "   • Zookeeper: localhost:2181"
echo ""
echo "🧪 To run tests:"
echo "   go run test-kafka.go"
echo ""
echo "📊 To view logs:"
echo "   $DOCKER_COMPOSE_CMD logs -f kafka"
echo ""
echo "🛑 To stop services:"
echo "   $DOCKER_COMPOSE_CMD down"
echo ""
echo "Happy coding! 🚀"