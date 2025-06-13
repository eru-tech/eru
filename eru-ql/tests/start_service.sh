#!/bin/bash

# Script to start eru-ql service for testing

echo "🚀 Starting eru-ql service for testing..."

# Set environment variables for test databases
export POSTGRES_HOST=localhost
export POSTGRES_PORT=5432
export POSTGRES_DB=eru_test
export POSTGRES_USER=postgres
export POSTGRES_PASSWORD=password

export MYSQL_HOST=localhost
export MYSQL_PORT=3306
export MYSQL_DB=eru_test
export MYSQL_USER=root
export MYSQL_PASSWORD=password

export STORE_TYPE=POSTGRES
export ERUQLPORT=8087

echo "📊 Database Configuration:"
echo "  PostgreSQL: ${POSTGRES_USER}@${POSTGRES_HOST}:${POSTGRES_PORT}/${POSTGRES_DB}"
echo "  MySQL: ${MYSQL_USER}@${MYSQL_HOST}:${MYSQL_PORT}/${MYSQL_DB}"
echo "  Service Port: ${ERUQLPORT}"
echo ""

# Run environment setup if script exists
if [ -f "../../set_eruconfig_db.sh" ]; then
    echo "🔧 Running environment setup..."
    source ../../set_eruconfig_db.sh db.dev.erutech.io eruconfig_devapp
    echo ""
fi

echo "🏃 Starting eru-ql service..."
echo "Service will be available at: http://localhost:${ERUQLPORT}"
echo "Health check: http://localhost:${ERUQLPORT}/health"
echo "GraphQL endpoint: http://localhost:${ERUQLPORT}/graphql"
echo ""
echo "Press Ctrl+C to stop the service"
echo ""

# Start the service
go run main.go